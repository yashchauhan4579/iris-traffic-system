package streamer

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/irisdrone/magicbox-node/internal/decoder"
)

// CameraReader reads frames from an RTSP stream using the best available decoder
type CameraReader struct {
	cameraID  string
	rtspURL   string
	fps       int
	width     int
	height    int
	publisher *Publisher

	decoder decoder.Decoder
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex

	// Stats
	framesRead  uint64
	lastFrame   time.Time
	lastError   error
	isConnected bool
}

// CameraConfig holds camera configuration
type CameraConfig struct {
	CameraID string
	RTSPURL  string
	FPS      int
	Width    int
	Height   int
}

// NewCameraReader creates a new camera reader
func NewCameraReader(cfg CameraConfig, publisher *Publisher) *CameraReader {
	// Defaults
	if cfg.FPS <= 0 {
		cfg.FPS = 15
	}
	if cfg.Width <= 0 {
		cfg.Width = 1280
	}
	if cfg.Height <= 0 {
		cfg.Height = 720
	}

	return &CameraReader{
		cameraID:  cfg.CameraID,
		rtspURL:   cfg.RTSPURL,
		fps:       cfg.FPS,
		width:     cfg.Width,
		height:    cfg.Height,
		publisher: publisher,
	}
}

// Start begins reading frames from the RTSP stream
func (c *CameraReader) Start() error {
	c.mu.Lock()
	if c.ctx != nil {
		c.mu.Unlock()
		return fmt.Errorf("camera reader already started")
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.mu.Unlock()

	// Create decoder with auto-detection
	dec, err := decoder.New(decoder.DecoderConfig{
		CameraID:    c.cameraID,
		RTSPURL:     c.rtspURL,
		FPS:         c.fps,
		Width:       c.width,
		Height:      c.height,
		JPEGQuality: 75,
	})
	if err != nil {
		return fmt.Errorf("failed to create decoder: %w", err)
	}

	c.mu.Lock()
	c.decoder = dec
	c.mu.Unlock()

	// Start decoder with frame handler
	return dec.Start(c.ctx, c.handleFrame)
}

// handleFrame is called for each decoded frame
func (c *CameraReader) handleFrame(frame *decoder.Frame) {
	// Publish frame to NATS
	if err := c.publisher.PublishFrame(frame.CameraID, frame.Data, frame.Width, frame.Height); err != nil {
		log.Printf("⚠️ Failed to publish frame for %s: %v", c.cameraID, err)
	}

	c.mu.Lock()
	c.framesRead++
	c.lastFrame = time.Now()
	c.isConnected = true
	c.mu.Unlock()
}

// Stop stops the camera reader
func (c *CameraReader) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancel != nil {
		c.cancel()
	}
	if c.decoder != nil {
		c.decoder.Stop()
	}
	c.isConnected = false
	c.ctx = nil
	c.decoder = nil
}

// Stats returns current statistics
func (c *CameraReader) Stats() CameraStats {
	c.mu.Lock()
	defer c.mu.Unlock()

	stats := CameraStats{
		CameraID:    c.cameraID,
		FramesRead:  c.framesRead,
		LastFrame:   c.lastFrame,
		LastError:   c.lastError,
		IsConnected: c.isConnected,
		FPS:         c.fps,
	}

	// Get decoder-specific stats if available
	if c.decoder != nil {
		decStats := c.decoder.Stats()
		stats.Backend = string(decStats.Backend)
		stats.HardwareType = string(decStats.HardwareType)
		stats.CurrentFPS = decStats.FPS
		stats.IsConnected = decStats.IsConnected
		if decStats.LastError != nil {
			stats.LastError = decStats.LastError
		}
	}

	return stats
}

// CameraStats holds camera statistics
type CameraStats struct {
	CameraID     string
	FramesRead   uint64
	LastFrame    time.Time
	LastError    error
	IsConnected  bool
	FPS          int
	CurrentFPS   float64
	Backend      string
	HardwareType string
}
