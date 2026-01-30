// Package streamer handles video frame capture and distribution
package streamer

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/irisdrone/magicbox-node/internal/natsserver"
)

// FrameMessage is the message format published to NATS
type FrameMessage struct {
	Camera    string `json:"c"`  // Camera ID
	Seq       uint64 `json:"s"`  // Sequence number
	Timestamp int64  `json:"t"`  // Unix timestamp in milliseconds
	Width     int    `json:"w"`  // Frame width
	Height    int    `json:"h"`  // Frame height
	Frame     string `json:"f"`  // Base64 encoded JPEG
}

// Publisher publishes frames to NATS
type Publisher struct {
	nats          *natsserver.EmbeddedNATS
	seq           map[string]uint64
	fpsCount      map[string]int
	lastFPSUpdate time.Time
	mu            sync.Mutex
}

// NewPublisher creates a new frame publisher
func NewPublisher(nats *natsserver.EmbeddedNATS) *Publisher {
	p := &Publisher{
		nats:          nats,
		seq:           make(map[string]uint64),
		fpsCount:      make(map[string]int),
		lastFPSUpdate: time.Now(),
	}
	// Start FPS logging goroutine
	go p.logFPS()
	return p
}

// logFPS logs FPS every second
func (p *Publisher) logFPS() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		p.mu.Lock()
		for cameraID, count := range p.fpsCount {
			if count > 0 {
				log.Printf("📊 [PUBLISHER] %s: %d fps to local NATS", cameraID, count)
			}
			p.fpsCount[cameraID] = 0
		}
		p.mu.Unlock()
	}
}

// PublishFrame publishes a JPEG frame to NATS
func (p *Publisher) PublishFrame(cameraID string, jpegData []byte, width, height int) error {
	p.mu.Lock()
	p.seq[cameraID]++
	seq := p.seq[cameraID]
	p.fpsCount[cameraID]++
	p.mu.Unlock()

	msg := FrameMessage{
		Camera:    cameraID,
		Seq:       seq,
		Timestamp: time.Now().UnixMilli(),
		Width:     width,
		Height:    height,
		Frame:     base64.StdEncoding.EncodeToString(jpegData),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Publish to subject: frames.<camera_id>
	return p.nats.Publish("frames."+cameraID, data)
}

// PublishFrameRaw publishes raw bytes (for binary protocol if needed later)
func (p *Publisher) PublishFrameRaw(cameraID string, data []byte) error {
	return p.nats.Publish("frames."+cameraID+".raw", data)
}

// GetSequence returns the current sequence number for a camera
func (p *Publisher) GetSequence(cameraID string) uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.seq[cameraID]
}

