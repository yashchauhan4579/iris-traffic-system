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

// GStreamerDecoder decodes video using GStreamer pipelines
// Especially optimized for NVIDIA Jetson with nvv4l2decoder
type GStreamerDecoder struct {
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

// NewGStreamerDecoder creates a new GStreamer-based decoder
func NewGStreamerDecoder(cfg DecoderConfig, hwInfo *HardwareInfo) (*GStreamerDecoder, error) {
	if hwInfo.GStreamerPath == "" {
		return nil, fmt.Errorf("GStreamer (gst-launch-1.0) not found in PATH")
	}

	return &GStreamerDecoder{
		cfg:    cfg,
		hwInfo: hwInfo,
	}, nil
}

// Backend returns the backend type
func (d *GStreamerDecoder) Backend() BackendType {
	return BackendGStreamer
}

// Stats returns decoder statistics
func (d *GStreamerDecoder) Stats() DecoderStats {
	d.mu.Lock()
	defer d.mu.Unlock()
	return DecoderStats{
		CameraID:      d.cfg.CameraID,
		Backend:       BackendGStreamer,
		HardwareType:  d.hwInfo.Type,
		IsConnected:   d.isConnected,
		FramesDecoded: d.framesDecoded,
		LastFrame:     d.lastFrame,
		LastError:     d.lastError,
		FPS:           d.currentFPS,
	}
}

// Start begins decoding
func (d *GStreamerDecoder) Start(ctx context.Context, handler FrameHandler) error {
	childCtx, cancel := context.WithCancel(ctx)
	d.cancel = cancel

	go d.decodeLoop(childCtx, handler)
	return nil
}

// Stop stops the decoder
func (d *GStreamerDecoder) Stop() {
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

func (d *GStreamerDecoder) decodeLoop(ctx context.Context, handler FrameHandler) {
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
			log.Printf("⚠️ GStreamer decoder %s error: %v, reconnecting in 5s...", d.cfg.CameraID, err)
			time.Sleep(5 * time.Second)
		}
	}
}

func (d *GStreamerDecoder) connectAndDecode(ctx context.Context, handler FrameHandler) error {
	// Build GStreamer pipeline
	pipeline := d.buildGStreamerPipeline()

	d.mu.Lock()
	d.cmd = exec.CommandContext(ctx, "gst-launch-1.0", "-q", pipeline)
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

	// Start GStreamer
	if err := d.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start GStreamer: %w", err)
	}

	d.mu.Lock()
	d.isConnected = true
	d.mu.Unlock()

	log.Printf("🎥 GStreamer decoder %s connected (HW: %s): %s",
		d.cfg.CameraID, d.hwInfo.Type, d.cfg.RTSPURL)
	log.Printf("   Pipeline: %s", pipeline)

	// Log stderr in background
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if bytes.Contains([]byte(line), []byte("ERROR")) ||
				bytes.Contains([]byte(line), []byte("error")) {
				log.Printf("⚠️ GStreamer %s: %s", d.cfg.CameraID, line)
			}
		}
	}()

	// Read and parse JPEG frames
	return d.readJPEGFrames(ctx, stdout, handler)
}

func (d *GStreamerDecoder) buildGStreamerPipeline() string {
	decoderElement := d.hwInfo.GetGStreamerDecoderElement()

	// JPEG quality (0-100 for GStreamer jpegenc)
	jpegQuality := d.cfg.JPEGQuality
	if jpegQuality <= 0 {
		jpegQuality = 75
	}
	if jpegQuality > 100 {
		jpegQuality = 100
	}

	var pipeline string

	switch d.hwInfo.Type {
	case HWNVIDIAJetson:
		// Optimized pipeline for Jetson with nvv4l2decoder
		// nvv4l2decoder outputs to NVMM memory, need nvvidconv to convert
		pipeline = fmt.Sprintf(
			`rtspsrc location="%s" latency=100 protocols=tcp ! `+
				`rtph264depay ! h264parse ! %s ! `+
				`nvvidconv ! "video/x-raw,format=BGRx,width=%d,height=%d" ! `+
				`videorate ! "video/x-raw,framerate=%d/1" ! `+
				`videoconvert ! "video/x-raw,format=RGB" ! `+
				`jpegenc quality=%d ! `+
				`fdsink`,
			d.cfg.RTSPURL, decoderElement, d.cfg.Width, d.cfg.Height, d.cfg.FPS, jpegQuality)

	case HWNVIDIADesktop:
		// Desktop NVIDIA with nvdec
		pipeline = fmt.Sprintf(
			`rtspsrc location="%s" latency=100 protocols=tcp ! `+
				`rtph264depay ! h264parse ! %s ! `+
				`videoconvert ! videoscale ! "video/x-raw,width=%d,height=%d" ! `+
				`videorate ! "video/x-raw,framerate=%d/1" ! `+
				`jpegenc quality=%d ! `+
				`fdsink`,
			d.cfg.RTSPURL, decoderElement, d.cfg.Width, d.cfg.Height, d.cfg.FPS, jpegQuality)

	case HWIntelVAAPI, HWAMVAAPI:
		// VAAPI pipeline
		pipeline = fmt.Sprintf(
			`rtspsrc location="%s" latency=100 protocols=tcp ! `+
				`rtph264depay ! h264parse ! %s ! `+
				`vaapipostproc width=%d height=%d ! `+
				`videoconvert ! `+
				`videorate ! "video/x-raw,framerate=%d/1" ! `+
				`jpegenc quality=%d ! `+
				`fdsink`,
			d.cfg.RTSPURL, decoderElement, d.cfg.Width, d.cfg.Height, d.cfg.FPS, jpegQuality)

	default:
		// Software decode fallback
		pipeline = fmt.Sprintf(
			`rtspsrc location="%s" latency=100 protocols=tcp ! `+
				`rtph264depay ! h264parse ! avdec_h264 ! `+
				`videoconvert ! videoscale ! "video/x-raw,width=%d,height=%d" ! `+
				`videorate ! "video/x-raw,framerate=%d/1" ! `+
				`jpegenc quality=%d ! `+
				`fdsink`,
			d.cfg.RTSPURL, d.cfg.Width, d.cfg.Height, d.cfg.FPS, jpegQuality)
	}

	return pipeline
}

func (d *GStreamerDecoder) readJPEGFrames(ctx context.Context, reader io.Reader, handler FrameHandler) error {
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
			framesThisSecond = 0
		default:
		}

		b, err := bufReader.ReadByte()
		if err != nil {
			if err == io.EOF {
				return fmt.Errorf("GStreamer stream ended")
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

