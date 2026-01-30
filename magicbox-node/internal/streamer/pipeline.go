package streamer

import (
	"log"
	"sync"

	"github.com/irisdrone/magicbox-node/internal/config"
	"github.com/irisdrone/magicbox-node/internal/natsserver"
	"github.com/nats-io/nats.go"
)

// Pipeline manages all camera readers
type Pipeline struct {
	config    *config.Manager
	nats      *natsserver.EmbeddedNATS
	publisher *Publisher
	cameras   map[string]*CameraReader
	mu        sync.RWMutex
	running   bool
}

// NewPipeline creates a new streaming pipeline
func NewPipeline(cfg *config.Manager, nats *natsserver.EmbeddedNATS) *Pipeline {
	publisher := NewPublisher(nats)

	return &Pipeline{
		config:    cfg,
		nats:      nats,
		publisher: publisher,
		cameras:   make(map[string]*CameraReader),
	}
}

// Start starts the streaming pipeline
func (p *Pipeline) Start() {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.mu.Unlock()

	log.Println("🎥 Starting streaming pipeline...")

	// Start cameras from config
	p.syncCameras()

	// Subscribe to config updates
	p.nats.Subscribe("config.cameras", func(msg *nats.Msg) {
		log.Println("📋 Camera config update received")
		p.syncCameras()
	})
}

// Stop stops all camera readers
func (p *Pipeline) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.running = false

	for id, cam := range p.cameras {
		log.Printf("🛑 Stopping camera %s", id)
		cam.Stop()
	}
	p.cameras = make(map[string]*CameraReader)

	log.Println("🎥 Streaming pipeline stopped")
}

// syncCameras synchronizes running cameras with config
func (p *Pipeline) syncCameras() {
	cfg := p.config.Get()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Track which cameras should be running
	desired := make(map[string]bool)

	for _, cam := range cfg.Cameras {
		if !cam.Enabled {
			continue
		}

		desired[cam.DeviceID] = true

		// Start if not already running
		if _, exists := p.cameras[cam.DeviceID]; !exists {
			reader := NewCameraReader(CameraConfig{
				CameraID: cam.DeviceID,
				RTSPURL:  cam.RTSPUrl,
				FPS:      cam.FPS,
				Width:    1280, // Default, could be from config
				Height:   720,
			}, p.publisher)

			if err := reader.Start(); err != nil {
				log.Printf("⚠️ Failed to start camera %s: %v", cam.DeviceID, err)
				continue
			}

			p.cameras[cam.DeviceID] = reader
			log.Printf("▶️ Started camera %s", cam.DeviceID)
		}
	}

	// Stop cameras that shouldn't be running
	for id, cam := range p.cameras {
		if !desired[id] {
			log.Printf("⏹️ Stopping camera %s (no longer in config)", id)
			cam.Stop()
			delete(p.cameras, id)
		}
	}

	log.Printf("🎥 Pipeline: %d cameras active", len(p.cameras))
}

// GetStats returns statistics for all cameras
func (p *Pipeline) GetStats() []CameraStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make([]CameraStats, 0, len(p.cameras))
	for _, cam := range p.cameras {
		stats = append(stats, cam.Stats())
	}
	return stats
}

// GetCameraStats returns statistics for a specific camera
func (p *Pipeline) GetCameraStats(cameraID string) (CameraStats, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if cam, exists := p.cameras[cameraID]; exists {
		return cam.Stats(), true
	}
	return CameraStats{}, false
}

// IsRunning returns whether the pipeline is running
func (p *Pipeline) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// CameraCount returns the number of active cameras
func (p *Pipeline) CameraCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.cameras)
}

// RefreshCamera restarts a specific camera
func (p *Pipeline) RefreshCamera(cameraID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if cam, exists := p.cameras[cameraID]; exists {
		cam.Stop()
		delete(p.cameras, cameraID)
	}

	// Find camera config
	cfg := p.config.Get()
	for _, cam := range cfg.Cameras {
		if cam.DeviceID == cameraID && cam.Enabled {
			reader := NewCameraReader(CameraConfig{
				CameraID: cam.DeviceID,
				RTSPURL:  cam.RTSPUrl,
				FPS:      cam.FPS,
				Width:    1280,
				Height:   720,
			}, p.publisher)

			if err := reader.Start(); err != nil {
				return err
			}

			p.cameras[cameraID] = reader
			log.Printf("🔄 Refreshed camera %s", cameraID)
		}
	}

	return nil
}

