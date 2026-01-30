package queue

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EventType represents the type of event
type EventType string

const (
	EventTypeANPR      EventType = "anpr"
	EventTypeVCC       EventType = "vcc"
	EventTypeViolation EventType = "violation"
	EventTypeCrowd     EventType = "crowd"
	EventTypeAlert     EventType = "alert"
)

// EventStatus represents the processing status
type EventStatus string

const (
	StatusPending    EventStatus = "pending"
	StatusProcessing EventStatus = "processing"
	StatusSent       EventStatus = "sent"
	StatusFailed     EventStatus = "failed"
)

// Event represents a queued event
type Event struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	DeviceID  string                 `json:"deviceId"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Images    []string               `json:"images,omitempty"` // Paths to image files
	Status    EventStatus            `json:"status"`
	Retries   int                    `json:"retries"`
	Error     string                 `json:"error,omitempty"`
	CreatedAt time.Time              `json:"createdAt"`
	UpdatedAt time.Time              `json:"updatedAt"`
}

// QueueStats holds queue statistics
type QueueStats struct {
	Pending   int `json:"pending"`
	Failed    int `json:"failed"`
	Processed int `json:"processed"`
}

// EventSender interface for sending events
type EventSender interface {
	SendEvent(event *Event) error
}

// FileQueue implements a file-based event queue
type FileQueue struct {
	baseDir     string
	pendingDir  string
	sentDir     string
	failedDir   string
	sender      EventSender
	stopChan    chan struct{}
	wg          sync.WaitGroup
	mu          sync.RWMutex
	stats       QueueStats
	maxRetries  int
	retryDelay  time.Duration
	batchSize   int
	processRate time.Duration
}

// NewFileQueue creates a new file-based queue
func NewFileQueue(baseDir string) (*FileQueue, error) {
	q := &FileQueue{
		baseDir:     baseDir,
		pendingDir:  filepath.Join(baseDir, "pending"),
		sentDir:     filepath.Join(baseDir, "sent"),
		failedDir:   filepath.Join(baseDir, "failed"),
		stopChan:    make(chan struct{}),
		maxRetries:  5,
		retryDelay:  5 * time.Second,
		batchSize:   10,
		processRate: 1 * time.Second,
	}

	// Create directories
	for _, dir := range []string{q.pendingDir, q.sentDir, q.failedDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Count existing files
	q.updateStats()

	return q, nil
}

// SetSender sets the event sender
func (q *FileQueue) SetSender(sender EventSender) {
	q.sender = sender
}

// Enqueue adds an event to the queue
func (q *FileQueue) Enqueue(eventType EventType, deviceID string, data map[string]interface{}, images []string) (*Event, error) {
	event := &Event{
		ID:        uuid.New().String(),
		Type:      eventType,
		DeviceID:  deviceID,
		Timestamp: time.Now(),
		Data:      data,
		Images:    images,
		Status:    StatusPending,
		Retries:   0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := q.saveEvent(event, q.pendingDir); err != nil {
		return nil, err
	}

	q.mu.Lock()
	q.stats.Pending++
	q.mu.Unlock()

	log.Printf("📤 Event queued: %s (%s)", event.ID[:8], event.Type)
	return event, nil
}

// StartProcessor starts the background queue processor
func (q *FileQueue) StartProcessor() {
	q.wg.Add(1)
	go q.processLoop()
}

// Stop stops the queue processor
func (q *FileQueue) Stop() {
	close(q.stopChan)
	q.wg.Wait()
}

// GetStats returns current queue statistics
func (q *FileQueue) GetStats() QueueStats {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.stats
}

// GetPendingEvents returns all pending events
func (q *FileQueue) GetPendingEvents() ([]*Event, error) {
	return q.loadEventsFromDir(q.pendingDir)
}

// GetFailedEvents returns all failed events
func (q *FileQueue) GetFailedEvents() ([]*Event, error) {
	return q.loadEventsFromDir(q.failedDir)
}

// GetSentEvents returns recently sent events (limited)
func (q *FileQueue) GetSentEvents(limit int) ([]*Event, error) {
	events, err := q.loadEventsFromDir(q.sentDir)
	if err != nil {
		return nil, err
	}
	
	// Sort by timestamp descending
	sort.Slice(events, func(i, j int) bool {
		return events[i].UpdatedAt.After(events[j].UpdatedAt)
	})
	
	if len(events) > limit {
		events = events[:limit]
	}
	
	return events, nil
}

// RetryEvent moves a failed event back to pending
func (q *FileQueue) RetryEvent(eventID string) error {
	// Find in failed
	event, err := q.loadEvent(q.failedDir, eventID)
	if err != nil {
		return fmt.Errorf("event not found in failed queue: %w", err)
	}

	// Reset status
	event.Status = StatusPending
	event.Retries = 0
	event.Error = ""
	event.UpdatedAt = time.Now()

	// Move to pending
	if err := q.saveEvent(event, q.pendingDir); err != nil {
		return err
	}
	
	if err := q.deleteEvent(q.failedDir, eventID); err != nil {
		return err
	}

	q.mu.Lock()
	q.stats.Pending++
	q.stats.Failed--
	q.mu.Unlock()

	return nil
}

// RetryAllFailed moves all failed events back to pending
func (q *FileQueue) RetryAllFailed() (int, error) {
	events, err := q.GetFailedEvents()
	if err != nil {
		return 0, err
	}

	count := 0
	for _, event := range events {
		if err := q.RetryEvent(event.ID); err != nil {
			log.Printf("⚠️ Failed to retry event %s: %v", event.ID[:8], err)
			continue
		}
		count++
	}

	return count, nil
}

// ClearSent removes old sent events (cleanup)
func (q *FileQueue) ClearSent(olderThan time.Duration) (int, error) {
	events, err := q.loadEventsFromDir(q.sentDir)
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-olderThan)
	count := 0
	
	for _, event := range events {
		if event.UpdatedAt.Before(cutoff) {
			// Delete event and its images
			q.deleteEventWithImages(q.sentDir, event)
			count++
		}
	}

	q.mu.Lock()
	q.stats.Processed -= count
	q.mu.Unlock()

	return count, nil
}

// processLoop processes queued events
func (q *FileQueue) processLoop() {
	defer q.wg.Done()

	ticker := time.NewTicker(q.processRate)
	defer ticker.Stop()

	for {
		select {
		case <-q.stopChan:
			return
		case <-ticker.C:
			q.processBatch()
		}
	}
}

// processBatch processes a batch of pending events
func (q *FileQueue) processBatch() {
	if q.sender == nil {
		return
	}

	events, err := q.loadEventsFromDir(q.pendingDir)
	if err != nil {
		log.Printf("⚠️ Failed to load pending events: %v", err)
		return
	}

	// Sort by created time (oldest first)
	sort.Slice(events, func(i, j int) bool {
		return events[i].CreatedAt.Before(events[j].CreatedAt)
	})

	// Process batch
	processed := 0
	for _, event := range events {
		if processed >= q.batchSize {
			break
		}

		if err := q.processEvent(event); err != nil {
			log.Printf("⚠️ Event %s failed: %v", event.ID[:8], err)
		}
		processed++
	}
}

// processEvent attempts to send a single event
func (q *FileQueue) processEvent(event *Event) error {
	// Update status to processing
	event.Status = StatusProcessing
	event.UpdatedAt = time.Now()
	q.saveEvent(event, q.pendingDir)

	// Try to send
	err := q.sender.SendEvent(event)
	
	if err == nil {
		// Success - move to sent
		event.Status = StatusSent
		event.UpdatedAt = time.Now()
		
		if err := q.saveEvent(event, q.sentDir); err != nil {
			return err
		}
		if err := q.deleteEvent(q.pendingDir, event.ID); err != nil {
			return err
		}

		q.mu.Lock()
		q.stats.Pending--
		q.stats.Processed++
		q.mu.Unlock()

		log.Printf("✅ Event sent: %s (%s)", event.ID[:8], event.Type)
		return nil
	}

	// Failed
	event.Retries++
	event.Error = err.Error()
	event.UpdatedAt = time.Now()

	if event.Retries >= q.maxRetries {
		// Move to failed
		event.Status = StatusFailed
		if err := q.saveEvent(event, q.failedDir); err != nil {
			return err
		}
		if err := q.deleteEvent(q.pendingDir, event.ID); err != nil {
			return err
		}

		q.mu.Lock()
		q.stats.Pending--
		q.stats.Failed++
		q.mu.Unlock()

		log.Printf("❌ Event failed permanently: %s (%s)", event.ID[:8], event.Type)
	} else {
		// Keep in pending with incremented retry count
		event.Status = StatusPending
		q.saveEvent(event, q.pendingDir)
		log.Printf("🔄 Event retry %d/%d: %s", event.Retries, q.maxRetries, event.ID[:8])
	}

	return err
}

// saveEvent saves an event to a directory
func (q *FileQueue) saveEvent(event *Event, dir string) error {
	eventDir := filepath.Join(dir, event.ID)
	if err := os.MkdirAll(eventDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(eventDir, "event.json"), data, 0644)
}

// loadEvent loads an event from a directory
func (q *FileQueue) loadEvent(dir, eventID string) (*Event, error) {
	eventFile := filepath.Join(dir, eventID, "event.json")
	data, err := os.ReadFile(eventFile)
	if err != nil {
		return nil, err
	}

	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, err
	}

	return &event, nil
}

// loadEventsFromDir loads all events from a directory
func (q *FileQueue) loadEventsFromDir(dir string) ([]*Event, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var events []*Event
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		
		event, err := q.loadEvent(dir, entry.Name())
		if err != nil {
			log.Printf("⚠️ Failed to load event %s: %v", entry.Name(), err)
			continue
		}
		events = append(events, event)
	}

	return events, nil
}

// deleteEvent removes an event directory
func (q *FileQueue) deleteEvent(dir, eventID string) error {
	eventDir := filepath.Join(dir, eventID)
	return os.RemoveAll(eventDir)
}

// deleteEventWithImages removes an event and its images
func (q *FileQueue) deleteEventWithImages(dir string, event *Event) error {
	// Delete images
	for _, imgPath := range event.Images {
		os.Remove(imgPath)
	}
	
	return q.deleteEvent(dir, event.ID)
}

// updateStats recounts events from directories
func (q *FileQueue) updateStats() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.stats.Pending = q.countEventsInDir(q.pendingDir)
	q.stats.Failed = q.countEventsInDir(q.failedDir)
	q.stats.Processed = q.countEventsInDir(q.sentDir)
}

// countEventsInDir counts event directories
func (q *FileQueue) countEventsInDir(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			count++
		}
	}
	return count
}

// SaveImage saves an image and returns the path
func (q *FileQueue) SaveImage(eventID string, imageData []byte, filename string) (string, error) {
	imageDir := filepath.Join(q.pendingDir, eventID)
	if err := os.MkdirAll(imageDir, 0755); err != nil {
		return "", err
	}

	imagePath := filepath.Join(imageDir, filename)
	if err := os.WriteFile(imagePath, imageData, 0644); err != nil {
		return "", err
	}

	return imagePath, nil
}

