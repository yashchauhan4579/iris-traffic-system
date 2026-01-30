package decoder

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"io"
	"log"
	"os/exec"
	"sync"
	"time"
)

// FFmpegDecoder decodes video using FFmpeg with optional hardware acceleration
type FFmpegDecoder struct {
	cfg    DecoderConfig
	hwInfo *HardwareInfo

	cmd    *exec.Cmd
	cancel context.CancelFunc
	mu     sync.Mutex

	// Stats
	framesDecoded uint64
	lastFrame     time.Time
	lastError     error
	isConnected   bool
	currentFPS    float64
}

// NewFFmpegDecoder creates a new FFmpeg-based decoder
func NewFFmpegDecoder(cfg DecoderConfig, hwInfo *HardwareInfo) (*FFmpegDecoder, error) {
	if hwInfo.FFmpegPath == "" {
		return nil, fmt.Errorf("FFmpeg not found in PATH")
	}

	return &FFmpegDecoder{
		cfg:    cfg,
		hwInfo: hwInfo,
	}, nil
}

// Backend returns the backend type
func (d *FFmpegDecoder) Backend() BackendType {
	return BackendFFmpeg
}

// Stats returns decoder statistics
func (d *FFmpegDecoder) Stats() DecoderStats {
	d.mu.Lock()
	defer d.mu.Unlock()
	return DecoderStats{
		CameraID:      d.cfg.CameraID,
		Backend:       BackendFFmpeg,
		HardwareType:  d.hwInfo.Type,
		IsConnected:   d.isConnected,
		FramesDecoded: d.framesDecoded,
		LastFrame:     d.lastFrame,
		LastError:     d.lastError,
		FPS:           d.currentFPS,
	}
}

// Start begins decoding
func (d *FFmpegDecoder) Start(ctx context.Context, handler FrameHandler) error {
	childCtx, cancel := context.WithCancel(ctx)
	d.cancel = cancel

	go d.decodeLoop(childCtx, handler)
	return nil
}

// Stop stops the decoder
func (d *FFmpegDecoder) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.cancel != nil {
		d.cancel()
	}
	if d.cmd != nil && d.cmd.Process != nil {
		d.cmd.Process.Kill()
	}
	d.isConnected = false
}

func (d *FFmpegDecoder) decodeLoop(ctx context.Context, handler FrameHandler) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := d.connectAndDecode(ctx, handler)
		if err != nil {
			d.mu.Lock()
			d.lastError = err
			d.isConnected = false
			d.mu.Unlock()
			log.Printf("⚠️ FFmpeg decoder %s error: %v, reconnecting in 5s...", d.cfg.CameraID, err)
			time.Sleep(5 * time.Second)
		}
	}
}

func (d *FFmpegDecoder) connectAndDecode(ctx context.Context, handler FrameHandler) error {
	// Build FFmpeg command
	args := d.buildFFmpegArgs()

	d.mu.Lock()
	d.cmd = exec.CommandContext(ctx, "ffmpeg", args...)
	stdout, err := d.cmd.StdoutPipe()
	if err != nil {
		d.mu.Unlock()
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stderr, err := d.cmd.StderrPipe()
	if err != nil {
		d.mu.Unlock()
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}
	d.mu.Unlock()

	// Start FFmpeg
	if err := d.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	d.mu.Lock()
	d.isConnected = true
	d.mu.Unlock()

	log.Printf("🎥 FFmpeg decoder %s connected (HW: %s): %s", 
		d.cfg.CameraID, d.hwInfo.Type, d.cfg.RTSPURL)

	// Log stderr in background
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			// Only log errors, not info
			if bytes.Contains([]byte(line), []byte("error")) ||
				bytes.Contains([]byte(line), []byte("Error")) {
				log.Printf("⚠️ FFmpeg %s: %s", d.cfg.CameraID, line)
			}
		}
	}()

	// Read and parse JPEG frames
	return d.readJPEGFrames(ctx, stdout, handler)
}

func (d *FFmpegDecoder) buildFFmpegArgs() []string {
	args := []string{
		"-hide_banner",
		"-loglevel", "warning",
	}

	// Add hardware acceleration if available
	hwArgs := d.hwInfo.GetFFmpegHWAccelArgs()
	if len(hwArgs) > 0 {
		args = append(args, hwArgs...)
		log.Printf("🚀 Using FFmpeg hardware acceleration: %v", hwArgs)
	}

	// Input options
	args = append(args,
		"-rtsp_transport", "tcp",
		"-i", d.cfg.RTSPURL,
	)

	// Video filters
	vf := fmt.Sprintf("fps=%d", d.cfg.FPS)
	if d.cfg.Width > 0 && d.cfg.Height > 0 {
		vf += fmt.Sprintf(",scale=%d:%d", d.cfg.Width, d.cfg.Height)
	}
	args = append(args, "-vf", vf)

	// Output options - MJPEG to stdout
	jpegQuality := 31 - (d.cfg.JPEGQuality * 30 / 100) // Convert 1-100 to 31-1
	if jpegQuality < 1 {
		jpegQuality = 1
	}
	if jpegQuality > 31 {
		jpegQuality = 31
	}

	args = append(args,
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"-q:v", fmt.Sprintf("%d", jpegQuality),
		"-",
	)

	return args
}

func (d *FFmpegDecoder) readJPEGFrames(ctx context.Context, reader io.Reader, handler FrameHandler) error {
	bufReader := bufio.NewReader(reader)
	frameBuffer := &bytes.Buffer{}
	inFrame := false
	var sequence uint64

	// FPS tracking
	framesTicker := time.NewTicker(time.Second)
	defer framesTicker.Stop()
	framesThisSecond := 0

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-framesTicker.C:
			d.mu.Lock()
			d.currentFPS = float64(framesThisSecond)
			d.mu.Unlock()
			log.Printf("📊 [DECODER] %s: %d fps from RTSP", d.cfg.CameraID, framesThisSecond)
			framesThisSecond = 0
		default:
		}

		b, err := bufReader.ReadByte()
		if err != nil {
			if err == io.EOF {
				return fmt.Errorf("FFmpeg stream ended")
			}
			return fmt.Errorf("read error: %w", err)
		}

		frameBuffer.WriteByte(b)

		// Detect JPEG markers
		bufLen := frameBuffer.Len()
		if bufLen >= 2 {
			data := frameBuffer.Bytes()

			// SOI marker (Start of Image): 0xFF 0xD8
			if !inFrame && data[bufLen-2] == 0xFF && data[bufLen-1] == 0xD8 {
				inFrame = true
				frameBuffer.Reset()
				frameBuffer.Write([]byte{0xFF, 0xD8})
			}

			// EOI marker (End of Image): 0xFF 0xD9
			if inFrame && data[bufLen-2] == 0xFF && data[bufLen-1] == 0xD9 {
				// Complete frame received
				jpegData := make([]byte, frameBuffer.Len())
				copy(jpegData, frameBuffer.Bytes())

				// Get actual dimensions
				width, height := d.cfg.Width, d.cfg.Height
				if img, err := jpeg.Decode(bytes.NewReader(jpegData)); err == nil {
					bounds := img.Bounds()
					width = bounds.Dx()
					height = bounds.Dy()
				}

				// Create frame
				sequence++
				frame := &Frame{
					CameraID:  d.cfg.CameraID,
					Data:      jpegData,
					Width:     width,
					Height:    height,
					Timestamp: time.Now(),
					Sequence:  sequence,
				}

				// Call handler
				handler(frame)

				// Update stats
				d.mu.Lock()
				d.framesDecoded++
				d.lastFrame = time.Now()
				d.mu.Unlock()
				framesThisSecond++

				frameBuffer.Reset()
				inFrame = false
			}
		}

		// Prevent buffer overflow (10MB max)
		if frameBuffer.Len() > 10*1024*1024 {
			frameBuffer.Reset()
			inFrame = false
		}
	}
}

