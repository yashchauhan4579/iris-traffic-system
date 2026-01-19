// Package services provides business logic services
package services

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nats-io/nats.go"
)

// FrameMessage is the format used by MagicBox for publishing frames
type FrameMessage struct {
	Camera    string `json:"c"` // Camera ID
	Seq       uint64 `json:"s"` // Sequence number
	Timestamp int64  `json:"t"` // Unix timestamp in milliseconds
	Width     int    `json:"w"` // Frame width
	Height    int    `json:"h"` // Frame height
	Frame     string `json:"f"` // Base64 encoded JPEG
}

// FeedHub manages camera feed subscriptions and WebSocket connections
type FeedHub struct {
	natsConn *nats.Conn

	// WebSocket connections
	clients   map[*FeedClient]bool
	clientsMu sync.RWMutex

	// Camera subscriptions (cameraKey -> subscription)
	subscriptions   map[string]*cameraSubscription
	subscriptionsMu sync.RWMutex

	// Broadcast channels
	register   chan *FeedClient
	unregister chan *FeedClient

	// FPS tracking per camera
	fpsCount map[string]int
	fpsMu    sync.Mutex
	stopFPS  chan struct{}
}

// cameraSubscription tracks a camera feed subscription
type cameraSubscription struct {
	cameraKey   string // format: workerID.cameraID
	natsSub     *nats.Subscription
	detectSub   *nats.Subscription
	viewers     map[*FeedClient]bool
	viewersMu   sync.RWMutex
	lastFrame   []byte
	lastFrameAt time.Time
}

// FeedClient represents a WebSocket client viewing feeds
type FeedClient struct {
	hub        *FeedHub
	conn       *websocket.Conn
	send       chan []byte
	cameras    map[string]bool // cameras this client is viewing
	camerasMu  sync.RWMutex
	userID     string
	remoteAddr string
}

// FeedMessage is a message sent to/from clients
type FeedMessage struct {
	Type     string          `json:"type"`     // subscribe, unsubscribe, frame, detection
	Camera   string          `json:"camera"`   // workerID.cameraID
	Data     json.RawMessage `json:"data,omitempty"`
	Binary   bool            `json:"-"` // True if this is binary frame data
	RawBytes []byte          `json:"-"` // Raw binary data
}

// NewFeedHub creates a new feed hub
func NewFeedHub(natsConn *nats.Conn) *FeedHub {
	h := &FeedHub{
		natsConn:      natsConn,
		clients:       make(map[*FeedClient]bool),
		subscriptions: make(map[string]*cameraSubscription),
		register:      make(chan *FeedClient),
		unregister:    make(chan *FeedClient),
		fpsCount:      make(map[string]int),
		stopFPS:       make(chan struct{}),
	}
	// Start FPS logging goroutine
	go h.logFPS()
	return h
}

// logFPS logs FPS every second for frames broadcast to WebSocket clients
func (h *FeedHub) logFPS() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-h.stopFPS:
			return
		case <-ticker.C:
			h.fpsMu.Lock()
			for cameraKey, count := range h.fpsCount {
				if count > 0 {
					log.Printf("📊 [FEEDHUB] %s: %d fps to WebSocket clients", cameraKey, count)
				}
				h.fpsCount[cameraKey] = 0
			}
			h.fpsMu.Unlock()
		}
	}
}

// Register adds a client to the hub
func (h *FeedHub) Register(client *FeedClient) {
	h.register <- client
}

// Run starts the hub's main loop
func (h *FeedHub) Run() {
	log.Println("📺 Feed hub started")

	for {
		select {
		case client := <-h.register:
			h.clientsMu.Lock()
			h.clients[client] = true
			h.clientsMu.Unlock()
			log.Printf("📺 Client connected: %s", client.remoteAddr)

		case client := <-h.unregister:
			h.clientsMu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.clientsMu.Unlock()

			// Unsubscribe from all cameras
			client.camerasMu.Lock()
			for cameraKey := range client.cameras {
				h.unsubscribeClient(client, cameraKey)
			}
			client.camerasMu.Unlock()

			log.Printf("📺 Client disconnected: %s", client.remoteAddr)
		}
	}
}

// Subscribe subscribes a client to a camera feed
func (h *FeedHub) Subscribe(client *FeedClient, cameraKey string) error {
	// Parse workerID and cameraID
	workerID, cameraID, err := parseCameraKey(cameraKey)
	if err != nil {
		return err
	}

	h.subscriptionsMu.Lock()
	defer h.subscriptionsMu.Unlock()

	// Check if subscription exists
	sub, exists := h.subscriptions[cameraKey]
	if !exists {
		// Create new subscription
		sub = &cameraSubscription{
			cameraKey: cameraKey,
			viewers:   make(map[*FeedClient]bool),
		}

		// Subscribe to frames from NATS
		frameSubject := fmt.Sprintf("frames.%s.%s", workerID, cameraID)
		sub.natsSub, err = h.natsConn.Subscribe(frameSubject, func(msg *nats.Msg) {
			h.broadcastFrame(cameraKey, msg.Data)
		})
		if err != nil {
			return fmt.Errorf("failed to subscribe to frames: %w", err)
		}

		// Subscribe to detections from NATS
		detectSubject := fmt.Sprintf("detections.%s.%s", workerID, cameraID)
		sub.detectSub, err = h.natsConn.Subscribe(detectSubject, func(msg *nats.Msg) {
			h.broadcastDetection(cameraKey, msg.Data)
		})
		if err != nil {
			sub.natsSub.Unsubscribe()
			return fmt.Errorf("failed to subscribe to detections: %w", err)
		}

		h.subscriptions[cameraKey] = sub

		// Send command to MagicBox to start streaming
		h.sendStartStreamCommand(workerID, cameraID)

		log.Printf("📺 Created subscription for camera %s", cameraKey)
	}

	// Add client to viewers
	sub.viewersMu.Lock()
	sub.viewers[client] = true
	sub.viewersMu.Unlock()

	// Track on client
	client.camerasMu.Lock()
	client.cameras[cameraKey] = true
	client.camerasMu.Unlock()

	log.Printf("📺 Client %s subscribed to %s", client.remoteAddr, cameraKey)
	return nil
}

// Unsubscribe removes a client from a camera feed
func (h *FeedHub) Unsubscribe(client *FeedClient, cameraKey string) {
	h.unsubscribeClient(client, cameraKey)
}

func (h *FeedHub) unsubscribeClient(client *FeedClient, cameraKey string) {
	h.subscriptionsMu.Lock()
	defer h.subscriptionsMu.Unlock()

	sub, exists := h.subscriptions[cameraKey]
	if !exists {
		return
	}

	// Remove client from viewers
	sub.viewersMu.Lock()
	delete(sub.viewers, client)
	viewerCount := len(sub.viewers)
	sub.viewersMu.Unlock()

	// Remove from client tracking
	client.camerasMu.Lock()
	delete(client.cameras, cameraKey)
	client.camerasMu.Unlock()

	// If no more viewers, unsubscribe from NATS
	if viewerCount == 0 {
		if sub.natsSub != nil {
			sub.natsSub.Unsubscribe()
		}
		if sub.detectSub != nil {
			sub.detectSub.Unsubscribe()
		}
		delete(h.subscriptions, cameraKey)

		// Send command to MagicBox to stop streaming
		workerID, cameraID, _ := parseCameraKey(cameraKey)
		h.sendStopStreamCommand(workerID, cameraID)

		log.Printf("📺 Removed subscription for camera %s (no viewers)", cameraKey)
	}

	log.Printf("📺 Client %s unsubscribed from %s", client.remoteAddr, cameraKey)
}

// broadcastFrame sends a frame to all viewers of a camera
func (h *FeedHub) broadcastFrame(cameraKey string, frameData []byte) {
	h.subscriptionsMu.RLock()
	sub, exists := h.subscriptions[cameraKey]
	h.subscriptionsMu.RUnlock()

	if !exists {
		return
	}

	// Decode the JSON frame message from MagicBox
	var frameMsg FrameMessage
	if err := json.Unmarshal(frameData, &frameMsg); err != nil {
		log.Printf("⚠️ Failed to decode frame message: %v", err)
		return
	}

	// Decode base64 JPEG
	jpegData, err := base64.StdEncoding.DecodeString(frameMsg.Frame)
	if err != nil {
		log.Printf("⚠️ Failed to decode base64 frame: %v", err)
		return
	}

	// Update last frame
	sub.lastFrame = jpegData
	sub.lastFrameAt = time.Now()

	// Create message with camera prefix
	// Format: [1 byte type][camera key length][camera key][raw JPEG data]
	msg := make([]byte, 1+1+len(cameraKey)+len(jpegData))
	msg[0] = 0x01 // Frame type
	msg[1] = byte(len(cameraKey))
	copy(msg[2:2+len(cameraKey)], cameraKey)
	copy(msg[2+len(cameraKey):], jpegData)

	// Send to all viewers
	sub.viewersMu.RLock()
	viewerCount := len(sub.viewers)
	for client := range sub.viewers {
		select {
		case client.send <- msg:
		default:
			// Client buffer full, skip frame
		}
	}
	sub.viewersMu.RUnlock()

	// Track FPS if there are viewers
	if viewerCount > 0 {
		h.fpsMu.Lock()
		h.fpsCount[cameraKey]++
		h.fpsMu.Unlock()
	}
}

// broadcastDetection sends detection data to all viewers of a camera
func (h *FeedHub) broadcastDetection(cameraKey string, detectData []byte) {
	h.subscriptionsMu.RLock()
	sub, exists := h.subscriptions[cameraKey]
	h.subscriptionsMu.RUnlock()

	if !exists {
		return
	}

	// Create message
	msg := FeedMessage{
		Type:   "detection",
		Camera: cameraKey,
		Data:   detectData,
	}
	msgBytes, _ := json.Marshal(msg)

	// Send to all viewers
	sub.viewersMu.RLock()
	for client := range sub.viewers {
		select {
		case client.send <- msgBytes:
		default:
			// Client buffer full, skip
		}
	}
	sub.viewersMu.RUnlock()
}

// sendStartStreamCommand tells MagicBox to start streaming a camera
func (h *FeedHub) sendStartStreamCommand(workerID, cameraID string) {
	cmd := map[string]string{
		"action":   "start_stream",
		"cameraId": cameraID,
	}
	cmdBytes, _ := json.Marshal(cmd)

	subject := fmt.Sprintf("command.%s", workerID)
	if err := h.natsConn.Publish(subject, cmdBytes); err != nil {
		log.Printf("⚠️ Failed to send start_stream command: %v", err)
	} else {
		log.Printf("📤 Sent start_stream command to %s for camera %s", workerID, cameraID)
	}
}

// sendStopStreamCommand tells MagicBox to stop streaming a camera
func (h *FeedHub) sendStopStreamCommand(workerID, cameraID string) {
	cmd := map[string]string{
		"action":   "stop_stream",
		"cameraId": cameraID,
	}
	cmdBytes, _ := json.Marshal(cmd)

	subject := fmt.Sprintf("command.%s", workerID)
	if err := h.natsConn.Publish(subject, cmdBytes); err != nil {
		log.Printf("⚠️ Failed to send stop_stream command: %v", err)
	} else {
		log.Printf("📤 Sent stop_stream command to %s for camera %s", workerID, cameraID)
	}
}

// parseCameraKey splits workerID.cameraID
func parseCameraKey(key string) (workerID, cameraID string, err error) {
	for i, c := range key {
		if c == '.' {
			return key[:i], key[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("invalid camera key format: %s (expected workerID.cameraID)", key)
}

// Stats returns hub statistics
type HubStats struct {
	Clients       int      `json:"clients"`
	Subscriptions int      `json:"subscriptions"`
	ActiveCameras []string `json:"activeCameras"`
}

func (h *FeedHub) Stats() HubStats {
	h.clientsMu.RLock()
	clientCount := len(h.clients)
	h.clientsMu.RUnlock()

	h.subscriptionsMu.RLock()
	cameras := make([]string, 0, len(h.subscriptions))
	for key := range h.subscriptions {
		cameras = append(cameras, key)
	}
	h.subscriptionsMu.RUnlock()

	return HubStats{
		Clients:       clientCount,
		Subscriptions: len(cameras),
		ActiveCameras: cameras,
	}
}

