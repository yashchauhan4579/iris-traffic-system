// Package central handles communication with the central NATS server
package central

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/irisdrone/magicbox-node/internal/config"
	"github.com/irisdrone/magicbox-node/internal/natsserver"
	"github.com/nats-io/nats.go"
)

// CentralNATSPort is the fixed port for central NATS server
const CentralNATSPort = 4233

// Client manages connection to central NATS and forwarding
type Client struct {
	config      *config.Manager
	localNATS   *natsserver.EmbeddedNATS
	centralConn *nats.Conn
	workerID    string

	// Subscriptions
	eventSub     *nats.Subscription
	detectionSub *nats.Subscription
	commandSub   *nats.Subscription

	// Active streams (cameras being viewed remotely)
	activeStreams     map[string]*nats.Subscription // cameraID -> frame subscription
	activeDetections  map[string]*nats.Subscription // cameraID -> detection subscription
	activeStreamsMu   sync.RWMutex

	// Stats
	eventsForwarded     uint64
	framesForwarded     uint64
	detectionsForwarded uint64

	// FPS tracking per camera
	fpsCount   map[string]int
	fpsMu      sync.Mutex

	mu       sync.RWMutex
	running  bool
	stopChan chan struct{}
}

// NewClient creates a new central NATS client
func NewClient(cfg *config.Manager, localNATS *natsserver.EmbeddedNATS) *Client {
	c := &Client{
		config:           cfg,
		localNATS:        localNATS,
		activeStreams:    make(map[string]*nats.Subscription),
		activeDetections: make(map[string]*nats.Subscription),
		fpsCount:         make(map[string]int),
		stopChan:         make(chan struct{}),
	}
	// Start FPS logging goroutine
	go c.logFPS()
	return c
}

// logFPS logs FPS every second for frames forwarded to central
func (c *Client) logFPS() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.fpsMu.Lock()
			for cameraID, count := range c.fpsCount {
				if count > 0 {
					log.Printf("📊 [FORWARDER] %s: %d fps to central NATS", cameraID, count)
				}
				c.fpsCount[cameraID] = 0
			}
			c.fpsMu.Unlock()
		}
	}
}

// Start connects to central NATS and begins forwarding (with retry)
func (c *Client) Start() error {
	// Start connection loop in background - don't block startup
	go c.connectLoop()
	return nil
}

// connectLoop retries connection to central NATS until successful
func (c *Client) connectLoop() {
	for {
		select {
		case <-c.stopChan:
			return
		default:
		}

		cfg := c.config.Get()

		// Check if platform is configured
		if cfg.Platform.ServerURL == "" {
			log.Println("📡 Platform not configured, waiting...")
			time.Sleep(10 * time.Second)
			continue
		}

		if cfg.Platform.WorkerID == "" {
			log.Println("📡 Worker ID not set, waiting...")
			time.Sleep(10 * time.Second)
			continue
		}

		c.workerID = cfg.Platform.WorkerID

		// Derive central NATS URL from platform server URL
		centralNATSURL, err := deriveCentralNATSURL(cfg.Platform.ServerURL)
		if err != nil {
			log.Printf("⚠️ Failed to derive central NATS URL: %v", err)
			time.Sleep(10 * time.Second)
			continue
		}

		// Try to connect to central NATS
		log.Printf("📡 Connecting to central NATS: %s", centralNATSURL)
		c.centralConn, err = nats.Connect(
			centralNATSURL,
			nats.Name(fmt.Sprintf("magicbox-%s", c.workerID)),
			nats.ReconnectWait(2*time.Second),
			nats.MaxReconnects(-1), // Infinite reconnects after initial connection
			nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
				log.Printf("⚠️ Central NATS disconnected: %v", err)
			}),
			nats.ReconnectHandler(func(nc *nats.Conn) {
				log.Printf("✅ Central NATS reconnected")
				// Re-subscribe after reconnect
				c.subscribeToCommands()
			}),
		)
		if err != nil {
			log.Printf("⚠️ Failed to connect to central NATS: %v (retrying in 5s)", err)
			time.Sleep(5 * time.Second)
			continue
		}

		log.Printf("✅ Connected to central NATS: %s", centralNATSURL)

		// Start subscriptions
		if err := c.subscribeToCommands(); err != nil {
			log.Printf("⚠️ Failed to subscribe to commands: %v", err)
			c.centralConn.Close()
			time.Sleep(5 * time.Second)
			continue
		}

		if err := c.subscribeToLocalEvents(); err != nil {
			log.Printf("⚠️ Failed to subscribe to local events: %v", err)
		}

		if err := c.subscribeToLocalDetections(); err != nil {
			log.Printf("⚠️ Failed to subscribe to local detections: %v", err)
		}

		c.mu.Lock()
		c.running = true
		c.mu.Unlock()

		log.Println("📡 Central forwarder started")
		
		// Wait for disconnect or stop
		for {
			select {
			case <-c.stopChan:
				return
			default:
				if c.centralConn == nil || !c.centralConn.IsConnected() {
					log.Println("📡 Central NATS connection lost, reconnecting...")
					break
				}
				time.Sleep(1 * time.Second)
			}
		}
	}
}

// Stop disconnects from central NATS
func (c *Client) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return
	}

	close(c.stopChan)

	// Unsubscribe all
	if c.eventSub != nil {
		c.eventSub.Unsubscribe()
	}
	if c.detectionSub != nil {
		c.detectionSub.Unsubscribe()
	}
	if c.commandSub != nil {
		c.commandSub.Unsubscribe()
	}

	// Stop active streams
	c.activeStreamsMu.Lock()
	for camID, sub := range c.activeStreams {
		sub.Unsubscribe()
		delete(c.activeStreams, camID)
	}
	c.activeStreamsMu.Unlock()

	// Close central connection
	if c.centralConn != nil {
		c.centralConn.Close()
	}

	c.running = false
	log.Println("📡 Central forwarder stopped")
}

// subscribeToCommands listens for commands from central
func (c *Client) subscribeToCommands() error {
	subject := fmt.Sprintf("command.%s", c.workerID)

	var err error
	c.commandSub, err = c.centralConn.Subscribe(subject, func(msg *nats.Msg) {
		c.handleCommand(msg)
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to commands: %w", err)
	}

	log.Printf("📥 Listening for commands on: %s", subject)
	return nil
}

// Command represents a command from central
type Command struct {
	Action   string `json:"action"`   // start_stream, stop_stream
	CameraID string `json:"cameraId"` // Camera to start/stop
}

// handleCommand processes commands from central
func (c *Client) handleCommand(msg *nats.Msg) {
	var cmd Command
	if err := json.Unmarshal(msg.Data, &cmd); err != nil {
		log.Printf("⚠️ Invalid command: %v", err)
		return
	}

	log.Printf("📥 Command received: %s for camera %s", cmd.Action, cmd.CameraID)

	switch cmd.Action {
	case "start_stream":
		c.startStreamForward(cmd.CameraID)
	case "stop_stream":
		c.stopStreamForward(cmd.CameraID)
	default:
		log.Printf("⚠️ Unknown command: %s", cmd.Action)
	}
}

// startStreamForward begins forwarding frames for a camera to central
func (c *Client) startStreamForward(cameraID string) {
	c.activeStreamsMu.Lock()
	defer c.activeStreamsMu.Unlock()

	// Check if already streaming
	if _, exists := c.activeStreams[cameraID]; exists {
		log.Printf("📹 Already streaming camera %s", cameraID)
		return
	}

	// Subscribe to local frames for this camera
	localFrameSubject := fmt.Sprintf("frames.%s", cameraID)
	centralFrameSubject := fmt.Sprintf("frames.%s.%s", c.workerID, cameraID)

	frameSub, err := c.localNATS.Subscribe(localFrameSubject, func(msg *nats.Msg) {
		// Forward to central
		if err := c.centralConn.Publish(centralFrameSubject, msg.Data); err != nil {
			log.Printf("⚠️ Failed to forward frame: %v", err)
		} else {
			c.framesForwarded++
			c.fpsMu.Lock()
			c.fpsCount[cameraID]++
			c.fpsMu.Unlock()
		}
	})
	if err != nil {
		log.Printf("⚠️ Failed to subscribe to local frames: %v", err)
		return
	}
	c.activeStreams[cameraID] = frameSub

	// Also subscribe to detections for this camera (from analytics workers)
	localDetectSubject := fmt.Sprintf("detections.%s", cameraID)
	centralDetectSubject := fmt.Sprintf("detections.%s.%s", c.workerID, cameraID)

	detectSub, err := c.localNATS.Subscribe(localDetectSubject, func(msg *nats.Msg) {
		// Forward detections to central for UI overlay
		if err := c.centralConn.Publish(centralDetectSubject, msg.Data); err != nil {
			log.Printf("⚠️ Failed to forward detection: %v", err)
		} else {
			c.detectionsForwarded++
		}
	})
	if err != nil {
		log.Printf("⚠️ Failed to subscribe to local detections: %v", err)
		// Continue without detections - frames will still work
	} else {
		c.activeDetections[cameraID] = detectSub
	}

	log.Printf("📹 Started streaming camera %s to central (frames + detections)", cameraID)
}

// stopStreamForward stops forwarding frames for a camera
func (c *Client) stopStreamForward(cameraID string) {
	c.activeStreamsMu.Lock()
	defer c.activeStreamsMu.Unlock()

	// Unsubscribe from frames
	if frameSub, exists := c.activeStreams[cameraID]; exists {
		frameSub.Unsubscribe()
		delete(c.activeStreams, cameraID)
	}

	// Unsubscribe from detections
	if detectSub, exists := c.activeDetections[cameraID]; exists {
		detectSub.Unsubscribe()
		delete(c.activeDetections, cameraID)
	}

	log.Printf("📹 Stopped streaming camera %s to central", cameraID)
}

// subscribeToLocalEvents forwards all events to central
func (c *Client) subscribeToLocalEvents() error {
	// Subscribe to all local events
	var err error
	c.eventSub, err = c.localNATS.Subscribe("events.*", func(msg *nats.Msg) {
		// Forward to central with worker prefix
		centralSubject := fmt.Sprintf("events.%s", c.workerID)
		if err := c.centralConn.Publish(centralSubject, msg.Data); err != nil {
			log.Printf("⚠️ Failed to forward event: %v", err)
		} else {
			c.eventsForwarded++
		}
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to local events: %w", err)
	}

	log.Println("📤 Forwarding events to central")
	return nil
}

// subscribeToLocalDetections forwards detections when camera is being viewed
func (c *Client) subscribeToLocalDetections() error {
	// Subscribe to all local detections
	var err error
	c.detectionSub, err = c.localNATS.Subscribe("detections.*", func(msg *nats.Msg) {
		// Extract camera ID from subject (detections.{cameraID})
		cameraID := msg.Subject[len("detections."):]

		// Only forward if camera is being streamed
		c.activeStreamsMu.RLock()
		_, isStreaming := c.activeStreams[cameraID]
		c.activeStreamsMu.RUnlock()

		if isStreaming {
			centralSubject := fmt.Sprintf("detections.%s.%s", c.workerID, cameraID)
			if err := c.centralConn.Publish(centralSubject, msg.Data); err != nil {
				log.Printf("⚠️ Failed to forward detection: %v", err)
			} else {
				c.detectionsForwarded++
			}
		}
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to local detections: %w", err)
	}

	log.Println("📤 Forwarding detections (when viewing) to central")
	return nil
}

// Stats returns forwarding statistics
type Stats struct {
	Connected           bool     `json:"connected"`
	CentralURL          string   `json:"centralUrl"`
	EventsForwarded     uint64   `json:"eventsForwarded"`
	FramesForwarded     uint64   `json:"framesForwarded"`
	DetectionsForwarded uint64   `json:"detectionsForwarded"`
	ActiveStreams       []string `json:"activeStreams"`
}

// GetStats returns current stats
func (c *Client) GetStats() Stats {
	c.activeStreamsMu.RLock()
	streams := make([]string, 0, len(c.activeStreams))
	for camID := range c.activeStreams {
		streams = append(streams, camID)
	}
	c.activeStreamsMu.RUnlock()

	connected := c.centralConn != nil && c.centralConn.IsConnected()
	centralURL := ""
	if cfg := c.config.Get(); cfg.Platform.ServerURL != "" {
		centralURL, _ = deriveCentralNATSURL(cfg.Platform.ServerURL)
	}

	return Stats{
		Connected:           connected,
		CentralURL:          centralURL,
		EventsForwarded:     c.eventsForwarded,
		FramesForwarded:     c.framesForwarded,
		DetectionsForwarded: c.detectionsForwarded,
		ActiveStreams:       streams,
	}
}

// IsConnected returns true if connected to central NATS
func (c *Client) IsConnected() bool {
	return c.centralConn != nil && c.centralConn.IsConnected()
}

// deriveCentralNATSURL extracts host from platform serverUrl and returns NATS URL on fixed port
// Example: "http://central.example.com:3001" -> "nats://central.example.com:4233"
func deriveCentralNATSURL(serverURL string) (string, error) {
	parsed, err := url.Parse(serverURL)
	if err != nil {
		return "", fmt.Errorf("invalid server URL: %w", err)
	}

	host := parsed.Hostname()
	if host == "" {
		return "", fmt.Errorf("no host in server URL")
	}

	return fmt.Sprintf("nats://%s:%d", host, CentralNATSPort), nil
}

