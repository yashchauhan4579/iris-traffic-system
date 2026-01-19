package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/irisdrone/backend/services"
)

var (
	feedHub  *services.FeedHub
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024 * 1024, // 1MB for frames
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for now
		},
	}
)

// SetFeedHub sets the feed hub for the handlers
func SetFeedHub(hub *services.FeedHub) {
	feedHub = hub
}

// HandleFeedWebSocket handles WebSocket connections for camera feeds
func HandleFeedWebSocket(c *gin.Context) {
	if feedHub == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Feed hub not initialized"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("⚠️ WebSocket upgrade failed: %v", err)
		return
	}

	// Get user ID from context (if authenticated)
	userID := c.GetString("userID")
	if userID == "" {
		userID = "anonymous"
	}

	client := services.NewFeedClient(feedHub, conn, userID, c.ClientIP())

	feedHub.Register(client)

	// Start goroutines for reading and writing
	go client.WritePump()
	go client.ReadPump()
}

// GetFeedHubStats returns feed hub statistics
func GetFeedHubStats(c *gin.Context) {
	if feedHub == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled": false,
		})
		return
	}

	stats := feedHub.Stats()
	c.JSON(http.StatusOK, gin.H{
		"enabled":       true,
		"clients":       stats.Clients,
		"subscriptions": stats.Subscriptions,
		"activeCameras": stats.ActiveCameras,
	})
}

