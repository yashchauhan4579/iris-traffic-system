package decoder

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Frame represents a decoded video frame
type Frame struct {
	CameraID  string
	Data      []byte // JPEG encoded
	Width     int
	Height    int
	Timestamp time.Time
	Sequence  uint64
}

// FrameHandler is called for each decoded frame
type FrameHandler func(frame *Frame)

// DecoderConfig holds configuration for a decoder
type DecoderConfig struct {
	CameraID   string
	RTSPURL    string
	FPS        int
	Width      int
	Height     int
	JPEGQuality int // 1-100, default 75
}

// Decoder is the interface for video decoders
type Decoder interface {
	// Start begins decoding and calls handler for each frame
	Start(ctx context.Context, handler FrameHandler) error
	// Stop stops the decoder
	Stop()
	// Stats returns decoder statistics
	Stats() DecoderStats
	// Backend returns the backend type being used
	Backend() BackendType
}

// DecoderStats holds decoder statistics
type DecoderStats struct {
	CameraID      string
	Backend       BackendType
	HardwareType  HardwareType
	IsConnected   bool
	FramesDecoded uint64
	LastFrame     time.Time
	LastError     error
	FPS           float64
}

// decoderFactory creates decoders based on hardware
var globalHWInfo *HardwareInfo

// Init initializes the decoder package and detects hardware
func Init() *HardwareInfo {
	if globalHWInfo == nil {
		globalHWInfo = DetectHardware()
	}
	return globalHWInfo
}

// GetHardwareInfo returns the detected hardware info
func GetHardwareInfo() *HardwareInfo {
	if globalHWInfo == nil {
		Init()
	}
	return globalHWInfo
}

// New creates a new decoder using the best available backend
func New(cfg DecoderConfig) (Decoder, error) {
	hwInfo := GetHardwareInfo()

	// Apply defaults
	if cfg.FPS <= 0 {
		cfg.FPS = 15
	}
	if cfg.Width <= 0 {
		cfg.Width = 1280
	}
	if cfg.Height <= 0 {
		cfg.Height = 720
	}
	if cfg.JPEGQuality <= 0 {
		cfg.JPEGQuality = 75
	}

	log.Printf("🎬 Creating decoder for %s using %s backend (%s)", 
		cfg.CameraID, hwInfo.Backend, hwInfo.Type)

	switch hwInfo.Backend {
	case BackendGStreamer:
		return NewGStreamerDecoder(cfg, hwInfo)
	case BackendFFmpeg:
		return NewFFmpegDecoder(cfg, hwInfo)
	default:
		return NewFFmpegDecoder(cfg, hwInfo)
	}
}

// NewWithBackend creates a decoder with a specific backend (for testing/override)
func NewWithBackend(cfg DecoderConfig, backend BackendType) (Decoder, error) {
	hwInfo := GetHardwareInfo()

	// Apply defaults
	if cfg.FPS <= 0 {
		cfg.FPS = 15
	}
	if cfg.Width <= 0 {
		cfg.Width = 1280
	}
	if cfg.Height <= 0 {
		cfg.Height = 720
	}
	if cfg.JPEGQuality <= 0 {
		cfg.JPEGQuality = 75
	}

	switch backend {
	case BackendGStreamer:
		if hwInfo.GStreamerPath == "" {
			return nil, fmt.Errorf("GStreamer not available")
		}
		return NewGStreamerDecoder(cfg, hwInfo)
	case BackendFFmpeg:
		if hwInfo.FFmpegPath == "" {
			return nil, fmt.Errorf("FFmpeg not available")
		}
		return NewFFmpegDecoder(cfg, hwInfo)
	default:
		return nil, fmt.Errorf("unknown backend: %s", backend)
	}
}

