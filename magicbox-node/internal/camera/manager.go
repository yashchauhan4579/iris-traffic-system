package camera

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/irisdrone/magicbox-node/internal/config"
	"github.com/irisdrone/magicbox-node/internal/queue"
)

// FrameCallback is called when a frame is decoded
type FrameCallback func(frame *Frame)

// Frame represents a decoded video frame
type Frame struct {
	CameraID  string
	Timestamp time.Time
	Width     int
	Height    int
	Data      []byte // Raw frame data (RGB/BGR)
	Format    string // "rgb24", "bgr24", "nv12", etc.
}

// CameraStatus represents the status of a camera stream
type CameraStatus struct {
	DeviceID    string    `json:"deviceId"`
	Connected   bool      `json:"connected"`
	FPS         float64   `json:"fps"`
	Resolution  string    `json:"resolution"`
	Errors      int       `json:"errors"`
	LastFrame   time.Time `json:"lastFrame"`
	BytesRead   uint64    `json:"bytesRead"`
	FramesRead  uint64    `json:"framesRead"`
}

// Stream handles a single camera stream
type Stream struct {
	config     config.CameraConfig
	status     CameraStatus
	callbacks  []FrameCallback
	stopChan   chan struct{}
	mu         sync.RWMutex
	isRunning  bool
}

// Manager manages all camera streams
type Manager struct {
	configMgr *config.Manager
	queue     *queue.FileQueue
	streams   map[string]*Stream
	mu        sync.RWMutex
	wg        sync.WaitGroup
}

// NewManager creates a new camera manager
func NewManager(cfg *config.Manager, q *queue.FileQueue) *Manager {
	return &Manager{
		configMgr: cfg,
		queue:     q,
		streams:   make(map[string]*Stream),
	}
}

// Start starts all configured camera streams
func (m *Manager) Start() error {
	cfg := m.configMgr.Get()
	
	for _, camCfg := range cfg.Cameras {
		if camCfg.Enabled {
			if err := m.StartStream(camCfg); err != nil {
				log.Printf("⚠️ Failed to start camera %s: %v", camCfg.Name, err)
			}
		}
	}
	
	return nil
}

// Stop stops all camera streams
func (m *Manager) Stop() {
	m.mu.Lock()
	for id := range m.streams {
		m.stopStreamUnsafe(id)
	}
	m.mu.Unlock()
	m.wg.Wait()
}

// StartStream starts a single camera stream
func (m *Manager) StartStream(cfg config.CameraConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.streams[cfg.DeviceID]; exists {
		return fmt.Errorf("stream %s already running", cfg.DeviceID)
	}

	stream := &Stream{
		config:   cfg,
		stopChan: make(chan struct{}),
		status: CameraStatus{
			DeviceID:   cfg.DeviceID,
			Connected:  false,
			Resolution: cfg.Resolution,
		},
	}

	m.streams[cfg.DeviceID] = stream

	m.wg.Add(1)
	go m.runStream(stream)

	log.Printf("📹 Started camera stream: %s (%s)", cfg.Name, cfg.DeviceID)
	return nil
}

// StopStream stops a single camera stream
func (m *Manager) StopStream(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopStreamUnsafe(deviceID)
}

func (m *Manager) stopStreamUnsafe(deviceID string) error {
	stream, exists := m.streams[deviceID]
	if !exists {
		return fmt.Errorf("stream %s not found", deviceID)
	}

	close(stream.stopChan)
	delete(m.streams, deviceID)
	return nil
}

// GetStatus returns status of all streams
func (m *Manager) GetStatus() []CameraStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make([]CameraStatus, 0, len(m.streams))
	for _, stream := range m.streams {
		stream.mu.RLock()
		statuses = append(statuses, stream.status)
		stream.mu.RUnlock()
	}
	return statuses
}

// GetStreamStatus returns status of a specific stream
func (m *Manager) GetStreamStatus(deviceID string) (*CameraStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stream, exists := m.streams[deviceID]
	if !exists {
		return nil, fmt.Errorf("stream %s not found", deviceID)
	}

	stream.mu.RLock()
	status := stream.status
	stream.mu.RUnlock()

	return &status, nil
}

// RegisterCallback registers a callback for frame events
func (m *Manager) RegisterCallback(deviceID string, callback FrameCallback) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream, exists := m.streams[deviceID]
	if !exists {
		return fmt.Errorf("stream %s not found", deviceID)
	}

	stream.mu.Lock()
	stream.callbacks = append(stream.callbacks, callback)
	stream.mu.Unlock()

	return nil
}

// runStream runs the camera stream processing loop
func (m *Manager) runStream(stream *Stream) {
	defer m.wg.Done()

	stream.mu.Lock()
	stream.isRunning = true
	stream.mu.Unlock()

	log.Printf("🎬 Connecting to: %s", stream.config.RTSPUrl)

	// TODO: Implement actual RTSP connection using:
	// Option 1: FFmpeg via exec
	// Option 2: GStreamer via cgo
	// Option 3: go-rtsp library
	// Option 4: Custom RTSP implementation
	
	// For now, this is a placeholder that simulates a camera stream
	ticker := time.NewTicker(time.Second / time.Duration(stream.config.FPS))
	defer ticker.Stop()

	reconnectDelay := 5 * time.Second
	frameCount := uint64(0)

	for {
		select {
		case <-stream.stopChan:
			log.Printf("⏹️ Stream stopped: %s", stream.config.Name)
			return
			
		case <-ticker.C:
			// Simulate frame processing
			// In production, this would:
			// 1. Read frame from RTSP stream
			// 2. Decode H264/H265 to raw frame
			// 3. Call analytics callbacks
			// 4. Handle any events from analytics
			
			frameCount++
			
			stream.mu.Lock()
			stream.status.Connected = true
			stream.status.FramesRead = frameCount
			stream.status.LastFrame = time.Now()
			stream.status.FPS = float64(stream.config.FPS)
			stream.mu.Unlock()

			// Simulate occasional reconnection needs
			if frameCount%1000 == 0 {
				log.Printf("📊 Camera %s: %d frames processed", stream.config.Name, frameCount)
			}
		}

		// Reconnection logic placeholder
		_ = reconnectDelay
	}
}

// TestConnection tests connectivity to a camera
func TestConnection(rtspURL string) error {
	// TODO: Implement actual connection test
	// This would attempt to connect to the RTSP stream
	// and read a few frames to verify it's working
	
	log.Printf("🔍 Testing connection to: %s", rtspURL)
	
	// Simulate connection test
	time.Sleep(2 * time.Second)
	
	return nil
}

// GetSupportedCodecs returns supported video codecs
func GetSupportedCodecs() []string {
	return []string{
		"h264",
		"h265",
		"hevc",
	}
}

