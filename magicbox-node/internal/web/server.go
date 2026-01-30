package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/irisdrone/magicbox-node/internal/central"
	"github.com/irisdrone/magicbox-node/internal/config"
	"github.com/irisdrone/magicbox-node/internal/natsserver"
	"github.com/irisdrone/magicbox-node/internal/platform"
	"github.com/irisdrone/magicbox-node/internal/queue"
	"github.com/irisdrone/magicbox-node/internal/streamer"
	"github.com/irisdrone/magicbox-node/internal/wireguard"
)

// generateShortUUID generates a short random ID (16 hex chars)
func generateShortUUID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

//go:embed templates/*
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

// Server is the web UI server
type Server struct {
	config    *config.Manager
	platform  *platform.Client
	queue     *queue.FileQueue
	nats      *natsserver.EmbeddedNATS
	pipeline  *streamer.Pipeline
	central   *central.Client
	wireguard *wireguard.Manager
	port      int
	router    *gin.Engine
	server    *http.Server
}

// NewServer creates a new web server
func NewServer(cfg *config.Manager, plat *platform.Client, q *queue.FileQueue, nats *natsserver.EmbeddedNATS, pipeline *streamer.Pipeline, centralClient *central.Client, port int) *Server {
	gin.SetMode(gin.ReleaseMode)
	
	// Initialize WireGuard manager
	wgManager := wireguard.NewManager()
	
	// Check if WireGuard is installed, if not start installation in background
	if !wgManager.IsInstalled() {
		log.Println("⚠️ WireGuard not installed, starting installation...")
		wgManager.Install()
	} else {
		log.Println("✅ WireGuard is installed")
	}
	
	s := &Server{
		config:    cfg,
		platform:  plat,
		queue:     q,
		nats:      nats,
		pipeline:  pipeline,
		central:   centralClient,
		wireguard: wgManager,
		port:      port,
		router:    gin.New(),
	}

	// Connect queue to platform sender
	q.SetSender(plat)

	s.setupRoutes()
	
	// Auto-bring up WireGuard if configured and config file exists
	wgCfg := cfg.GetWireGuard()
	if wgCfg.Configured && wgCfg.Enabled {
		go func() {
			time.Sleep(2 * time.Second) // Wait for service initialization
			
			// Check if config file exists before trying to bring up
			if _, err := os.Stat("/etc/wireguard/wg-iris.conf"); os.IsNotExist(err) {
				log.Println("ℹ️ WireGuard config file not found, skipping auto-start (will be created on first setup)")
				return
			}
			
			if err := wgManager.Up(); err != nil {
				log.Printf("⚠️ Failed to bring up WireGuard: %v", err)
			} else {
				log.Println("✅ WireGuard interface brought up successfully")
			}
		}()
	}
	
	return s
}

// Start starts the web server
func (s *Server) Start() error {
	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Handler: s.router,
	}

	log.Printf("🌐 Web UI starting on http://0.0.0.0:%d", s.port)
	return s.server.ListenAndServe()
}

// Stop stops the web server
func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func (s *Server) setupRoutes() {
	// Middleware
	s.router.Use(gin.Recovery())
	s.router.Use(gin.Logger())

	// Parse embedded templates
	tmpl := template.Must(template.ParseFS(templatesFS, "templates/*.html"))
	s.router.SetHTMLTemplate(tmpl)

	// Serve static files
	staticSub, _ := fs.Sub(staticFS, "static")
	s.router.StaticFS("/static", http.FS(staticSub))

	// Pages
	s.router.GET("/", s.handleIndex)
	s.router.GET("/setup", s.handleSetup)
	s.router.GET("/dashboard", s.handleDashboard)
	s.router.GET("/queue", s.handleQueue)
	s.router.GET("/cameras", s.handleCameras)
	s.router.GET("/logs", s.handleLogs)

	// API endpoints
	api := s.router.Group("/api")
	{
		// Status
		api.GET("/status", s.handleAPIStatus)
		api.GET("/resources", s.handleAPIResources)
		
		// Registration
		api.POST("/register", s.handleAPIRegister)
		api.POST("/request-approval", s.handleAPIRequestApproval)
		api.GET("/approval-status", s.handleAPIApprovalStatus)
		api.POST("/disconnect", s.handleAPIDisconnect)
		
		// Config
		api.GET("/config", s.handleAPIGetConfig)
		api.PUT("/config", s.handleAPIUpdateConfig)
		api.PUT("/config/platform", s.handleAPIUpdatePlatformConfig)
		api.PUT("/config/network", s.handleAPIUpdateNetworkConfig)
		api.POST("/sync", s.handleAPISyncConfig)
		
		// Queue
		api.GET("/queue/stats", s.handleAPIQueueStats)
		api.GET("/queue/pending", s.handleAPIQueuePending)
		api.GET("/queue/failed", s.handleAPIQueueFailed)
		api.GET("/queue/sent", s.handleAPIQueueSent)
		api.POST("/queue/retry/:id", s.handleAPIQueueRetry)
		api.POST("/queue/retry-all", s.handleAPIQueueRetryAll)
		api.DELETE("/queue/clear-sent", s.handleAPIQueueClearSent)
		
		// Cameras
		api.GET("/cameras", s.handleAPIGetCameras)
		api.POST("/cameras", s.handleAPIAddCamera)
		api.DELETE("/cameras/:id", s.handleAPIDeleteCamera)
		api.POST("/cameras/test", s.handleAPITestCamera)
		api.POST("/cameras/sync", s.handleAPISyncCameras)
		api.POST("/cameras/:id/enable", s.handleAPIEnableCamera)
		api.POST("/cameras/:id/disable", s.handleAPIDisableCamera)

		// Streaming
		api.GET("/streaming/status", s.handleAPIStreamingStatus)
		api.GET("/streaming/cameras", s.handleAPIStreamingCameras)
		api.POST("/streaming/cameras/:id/restart", s.handleAPIRestartCamera)

		// NATS info
		api.GET("/nats/info", s.handleAPINATSInfo)

		// Central NATS info
		api.GET("/central/stats", s.handleAPICentralStats)

		// WireGuard VPN
		api.GET("/magicnetwork/status", s.handleAPIMagicNetworkStatus)
		api.POST("/magicnetwork/setup", s.handleAPIMagicNetworkSetup)
		api.POST("/magicnetwork/up", s.handleAPIMagicNetworkUp)
		api.POST("/magicnetwork/down", s.handleAPIMagicNetworkDown)
		api.POST("/magicnetwork/restart", s.handleAPIMagicNetworkRestart)
	}
}

// Page handlers
func (s *Server) handleIndex(c *gin.Context) {
	cfg := s.config.Get()
	
	// Redirect based on state
	if cfg.State == config.StateUnconfigured {
		c.Redirect(http.StatusFound, "/setup")
		return
	}
	
	c.Redirect(http.StatusFound, "/dashboard")
}

func (s *Server) handleSetup(c *gin.Context) {
	cfg := s.config.Get()
	c.HTML(http.StatusOK, "setup.html", gin.H{
		"config": cfg,
	})
}

func (s *Server) handleDashboard(c *gin.Context) {
	cfg := s.config.Get()
	
	// Redirect to setup only if node name is not set
	// Allow dashboard access even if not fully registered (state can be pending)
	if cfg.NodeName == "" {
		c.Redirect(http.StatusFound, "/setup")
		return
	}
	
	stats := s.queue.GetStats()
	
	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"config":     cfg,
		"queueStats": stats,
	})
}

func (s *Server) handleQueue(c *gin.Context) {
	cfg := s.config.Get()
	stats := s.queue.GetStats()
	pending, _ := s.queue.GetPendingEvents()
	failed, _ := s.queue.GetFailedEvents()
	sent, _ := s.queue.GetSentEvents(50)
	
	c.HTML(http.StatusOK, "queue.html", gin.H{
		"config":  cfg,
		"stats":   stats,
		"pending": pending,
		"failed":  failed,
		"sent":    sent,
	})
}

func (s *Server) handleCameras(c *gin.Context) {
	cfg := s.config.Get()
	
	c.HTML(http.StatusOK, "cameras.html", gin.H{
		"config":  cfg,
		"cameras": cfg.Cameras,
	})
}

func (s *Server) handleLogs(c *gin.Context) {
	cfg := s.config.Get()
	
	c.HTML(http.StatusOK, "logs.html", gin.H{
		"config": cfg,
	})
}

// API handlers
func (s *Server) handleAPIStatus(c *gin.Context) {
	cfg := s.config.Get()
	stats := s.queue.GetStats()

	// NATS info
	natsInfo := gin.H{"enabled": false}
	if s.nats != nil {
		natsInfo = gin.H{
			"enabled":     true,
			"address":     s.nats.Address(),
			"numClients":  s.nats.NumClients(),
		}
	}

	// Streaming info
	streamingInfo := gin.H{"enabled": false}
	if s.pipeline != nil {
		streamingInfo = gin.H{
			"enabled":       true,
			"running":       s.pipeline.IsRunning(),
			"activeCameras": s.pipeline.CameraCount(),
		}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"state":         cfg.State,
		"nodeName":      cfg.NodeName,
		"nodeModel":     cfg.NodeModel,
		"mac":           cfg.MAC,
		"platform":      cfg.Platform.ServerURL,
		"workerId":      cfg.Platform.WorkerID,
		"configVersion": cfg.ConfigVersion,
		"lastSync":      cfg.LastSync,
		"cameraCount":   len(cfg.Cameras),
		"queueStats":    stats,
		"nats":          natsInfo,
		"streaming":     streamingInfo,
	})
}

func (s *Server) handleAPIResources(c *gin.Context) {
	// Use platform client's resource getter
	c.JSON(http.StatusOK, gin.H{
		"resources": map[string]interface{}{
			"timestamp": time.Now(),
		},
	})
}

func (s *Server) handleAPIRegister(c *gin.Context) {
	var req struct {
		ServerURL string `json:"serverUrl" binding:"required"`
		Token     string `json:"token" binding:"required"`
		NodeName  string `json:"nodeName"` // Optional - uses current if not provided
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Get current config
	cfg := s.config.Get()
	
	// Use current node name if not provided
	if req.NodeName == "" {
		req.NodeName = cfg.NodeName
	}
	
	// Save server URL and token
	cfg.Platform.ServerURL = req.ServerURL
	cfg.Platform.Token = req.Token
	
	// Save platform config
	if err := s.config.SetPlatformConfig(cfg.Platform); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Proceed with registration
	if err := s.platform.RegisterWithToken(req.ServerURL, req.Token, req.NodeName); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"message":  "Registration successful",
		"workerId": s.config.Get().Platform.WorkerID,
	})
}

func (s *Server) handleAPIRequestApproval(c *gin.Context) {
	var req struct {
		ServerURL string `json:"serverUrl" binding:"required"`
		NodeName  string `json:"nodeName"` // Optional - uses current if not provided
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Get current config
	cfg := s.config.Get()
	
	// Use current node name if not provided
	if req.NodeName == "" {
		req.NodeName = cfg.NodeName
	}
	
	// Save server URL
	cfg.Platform.ServerURL = req.ServerURL
	if err := s.config.SetPlatformConfig(cfg.Platform); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// Proceed with approval request
	if err := s.platform.RequestApproval(req.ServerURL, req.NodeName); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"message":   "Approval request submitted",
		"requestId": s.config.Get().Platform.RequestID,
	})
}

func (s *Server) handleAPIApprovalStatus(c *gin.Context) {
	status, err := s.platform.CheckApprovalStatus()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, status)
}

func (s *Server) handleAPIDisconnect(c *gin.Context) {
	if err := s.platform.Disconnect(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Disconnected from platform",
	})
}

func (s *Server) handleAPIGetConfig(c *gin.Context) {
	cfg := s.config.Get()
	c.JSON(http.StatusOK, cfg)
}

func (s *Server) handleAPIUpdateConfig(c *gin.Context) {
	var req struct {
		NodeName string `json:"nodeName"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	cfg := s.config.Get()
	
	if req.NodeName != "" {
		if err := s.config.SetNodeName(req.NodeName); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	
	// If state is unconfigured and we're setting node name, change to pending
	// This allows access to dashboard even if not fully registered
	if cfg.State == config.StateUnconfigured {
		if err := s.config.SetState(config.StatePending); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleAPIUpdatePlatformConfig(c *gin.Context) {
	var req struct {
		ServerURL string `json:"serverUrl"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	cfg := s.config.Get()
	if req.ServerURL != "" {
		cfg.Platform.ServerURL = req.ServerURL
		if err := s.config.SetPlatformConfig(cfg.Platform); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Platform configuration updated"})
}

func (s *Server) handleAPIUpdateNetworkConfig(c *gin.Context) {
	var req struct {
		Mode     string `json:"mode" binding:"required"` // "direct" or "magicnetwork"
		ServerIP string `json:"serverIP,omitempty"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	cfg := s.config.Get()
	
	if req.Mode == "direct" {
		if req.ServerIP != "" {
			cfg.Platform.ServerIP = req.ServerIP
			if err := s.config.SetPlatformConfig(cfg.Platform); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
	} else if req.Mode == "magicnetwork" {
		// MagicNetwork configuration is handled separately via /magicnetwork/setup
		c.JSON(http.StatusBadRequest, gin.H{"error": "Use /magicnetwork/setup to configure MagicNetwork"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Network configuration updated"})
}

func (s *Server) handleAPISyncConfig(c *gin.Context) {
	workerCfg, err := s.platform.FetchConfig()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	s.config.SetCameras(workerCfg.Cameras)
	s.config.SetConfigVersion(workerCfg.ConfigVersion)
	s.config.UpdateLastSync()
	
	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"configVersion": workerCfg.ConfigVersion,
		"cameraCount":   len(workerCfg.Cameras),
	})
}

func (s *Server) handleAPIQueueStats(c *gin.Context) {
	stats := s.queue.GetStats()
	c.JSON(http.StatusOK, stats)
}

func (s *Server) handleAPIQueuePending(c *gin.Context) {
	events, err := s.queue.GetPendingEvents()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, events)
}

func (s *Server) handleAPIQueueFailed(c *gin.Context) {
	events, err := s.queue.GetFailedEvents()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, events)
}

func (s *Server) handleAPIQueueSent(c *gin.Context) {
	events, err := s.queue.GetSentEvents(100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, events)
}

func (s *Server) handleAPIQueueRetry(c *gin.Context) {
	eventID := c.Param("id")
	
	if err := s.queue.RetryEvent(eventID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleAPIQueueRetryAll(c *gin.Context) {
	count, err := s.queue.RetryAllFailed()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"count":   count,
	})
}

func (s *Server) handleAPIQueueClearSent(c *gin.Context) {
	count, err := s.queue.ClearSent(24 * time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"count":   count,
	})
}

func (s *Server) handleAPIGetCameras(c *gin.Context) {
	cfg := s.config.Get()
	c.JSON(http.StatusOK, cfg.Cameras)
}

func (s *Server) handleAPITestCamera(c *gin.Context) {
	var req struct {
		RTSPUrl string `json:"rtspUrl" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// TODO: Actually test the RTSP connection
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Connection test not implemented yet",
	})
}

func (s *Server) handleAPIAddCamera(c *gin.Context) {
	var req struct {
		Name    string `json:"name" binding:"required"`
		RTSPUrl string `json:"rtspUrl" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Generate a local device ID
	deviceID := fmt.Sprintf("cam_%s", generateShortUUID())
	
	// Create camera config
	cam := config.CameraConfig{
		DeviceID:   deviceID,
		Name:       req.Name,
		RTSPUrl:    req.RTSPUrl,
		Analytics:  []string{}, // Will be set by platform
		FPS:        15,
		Resolution: "1080p",
		Enabled:    false, // Not enabled until platform assigns analytics
	}
	
	// Add to config
	cfg := s.config.Get()
	
	// Check for duplicate RTSP URL
	for _, existing := range cfg.Cameras {
		if existing.RTSPUrl == req.RTSPUrl {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Camera with this RTSP URL already exists"})
			return
		}
	}
	
	cameras := append(cfg.Cameras, cam)
	if err := s.config.SetCameras(cameras); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save camera"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"device_id": deviceID,
	})
}

func (s *Server) handleAPIDeleteCamera(c *gin.Context) {
	deviceID := c.Param("id")
	
	cfg := s.config.Get()
	
	// Find and remove the camera
	found := false
	cameras := make([]config.CameraConfig, 0)
	for _, cam := range cfg.Cameras {
		if cam.DeviceID == deviceID {
			found = true
			continue // Skip this one
		}
		cameras = append(cameras, cam)
	}
	
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Camera not found"})
		return
	}
	
	if err := s.config.SetCameras(cameras); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete camera"})
		return
	}
	
	// Also delete from platform if connected
	if s.platform != nil && cfg.Platform.WorkerID != "" {
		go s.platform.DeleteCamera(deviceID)
	}
	
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleAPISyncCameras(c *gin.Context) {
	cfg := s.config.Get()
	
	if cfg.Platform.WorkerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Not connected to platform"})
		return
	}
	
	if s.platform == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Platform client not initialized"})
		return
	}
	
	// Sync cameras to platform
	result, err := s.platform.SyncCameras(cfg.Cameras)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"created": result.Created,
		"updated": result.Updated,
	})
}

func (s *Server) handleAPIEnableCamera(c *gin.Context) {
	cameraID := c.Param("id")
	s.setCameraEnabled(c, cameraID, true)
}

func (s *Server) handleAPIDisableCamera(c *gin.Context) {
	cameraID := c.Param("id")
	s.setCameraEnabled(c, cameraID, false)
}

func (s *Server) setCameraEnabled(c *gin.Context, cameraID string, enabled bool) {
	cfg := s.config.Get()

	found := false
	cameras := make([]config.CameraConfig, len(cfg.Cameras))
	for i, cam := range cfg.Cameras {
		cameras[i] = cam
		if cam.DeviceID == cameraID {
			cameras[i].Enabled = enabled
			found = true
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Camera not found"})
		return
	}

	if err := s.config.SetCameras(cameras); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update camera"})
		return
	}

	// Notify pipeline to sync cameras
	if s.pipeline != nil && s.nats != nil {
		s.nats.Publish("config.cameras", []byte("updated"))
	}

	action := "disabled"
	if enabled {
		action = "enabled"
	}
	log.Printf("🎥 Camera %s %s for streaming", cameraID, action)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"enabled": enabled,
	})
}

// Streaming handlers
func (s *Server) handleAPIStreamingStatus(c *gin.Context) {
	status := gin.H{
		"enabled": s.pipeline != nil,
		"running": false,
		"cameras": 0,
	}

	if s.pipeline != nil {
		status["running"] = s.pipeline.IsRunning()
		status["cameras"] = s.pipeline.CameraCount()
	}

	c.JSON(http.StatusOK, status)
}

func (s *Server) handleAPIStreamingCameras(c *gin.Context) {
	if s.pipeline == nil {
		c.JSON(http.StatusOK, []interface{}{})
		return
	}

	stats := s.pipeline.GetStats()
	result := make([]gin.H, 0, len(stats))

	for _, stat := range stats {
		errMsg := ""
		if stat.LastError != nil {
			errMsg = stat.LastError.Error()
		}

		result = append(result, gin.H{
			"camera_id":    stat.CameraID,
			"is_connected": stat.IsConnected,
			"frames_read":  stat.FramesRead,
			"fps":          stat.FPS,
			"last_frame":   stat.LastFrame,
			"last_error":   errMsg,
		})
	}

	c.JSON(http.StatusOK, result)
}

func (s *Server) handleAPIRestartCamera(c *gin.Context) {
	cameraID := c.Param("id")

	if s.pipeline == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Streaming not enabled"})
		return
	}

	if err := s.pipeline.RefreshCamera(cameraID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleAPINATSInfo(c *gin.Context) {
	if s.nats == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled": false,
		})
		return
	}

	stats := s.nats.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"enabled":          true,
		"address":          s.nats.Address(),
		"port":             s.nats.Port(),
		"clients":          stats.Clients,
		"subscriptions":    stats.Subscriptions,
		"frames_published": stats.FramesPublished,
		"frames_dropped":   stats.FramesDropped,
		"in_msgs":          stats.InMsgs,
		"out_msgs":         stats.OutMsgs,
		"in_bytes":         stats.InBytes,
		"out_bytes":        stats.OutBytes,
		"slow_consumers":   stats.SlowConsumers,
	})
}

func (s *Server) handleAPICentralStats(c *gin.Context) {
	if s.central == nil {
		c.JSON(http.StatusOK, gin.H{
			"enabled": false,
		})
		return
	}

	stats := s.central.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"enabled":              true,
		"connected":            stats.Connected,
		"central_url":          stats.CentralURL,
		"events_forwarded":     stats.EventsForwarded,
		"frames_forwarded":     stats.FramesForwarded,
		"detections_forwarded": stats.DetectionsForwarded,
		"active_streams":       stats.ActiveStreams,
	})
}

// WireGuard handlers

func (s *Server) handleAPIMagicNetworkStatus(c *gin.Context) {
	status := s.wireguard.GetStatus()
	wgCfg := s.config.GetWireGuard()
	
	c.JSON(http.StatusOK, gin.H{
		"installed":      status.Installed,
		"interface_up":   status.InterfaceUp,
		"connected":      status.Connected,
		"public_key":     status.PublicKey,
		"assigned_ip":    wgCfg.AssignedIP,
		"server_ip":      wgCfg.ServerIP,
		"server_pubkey":  wgCfg.ServerPubKey,
		"server_endpoint": wgCfg.ServerEndpoint,
		"last_handshake": status.LastHandshake,
		"transfer_rx":    status.TransferRx,
		"transfer_tx":    status.TransferTx,
		"configured":     wgCfg.Configured,
		"enabled":        wgCfg.Enabled,
	})
}

// MagicNetworkSetupRequest from UI
type MagicNetworkSetupRequest struct {
	MagicNetworkURL    string `json:"magicNetworkUrl" binding:"required"`
	MagicNetworkAPIKey string `json:"magicNetworkApiKey" binding:"required"`
	ServerEndpoint     string `json:"serverEndpoint" binding:"required"` // MagicNetwork endpoint (host:port)
}

func (s *Server) handleAPIMagicNetworkSetup(c *gin.Context) {
	var req MagicNetworkSetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields: magicNetworkUrl, magicNetworkApiKey, serverEndpoint"})
		return
	}
	
	cfg := s.config.Get()
	
	// Check WireGuard is installed
	if !s.wireguard.IsInstalled() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MagicNetwork not installed yet, please wait"})
		return
	}
	
	// Generate or load keys
	privateKey, publicKey, err := s.wireguard.LoadOrGenerateKeys()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate keys: %v", err)})
		return
	}
	
	// Call MagicNetwork API to register this node
	wgResp, err := s.registerWithMagicNetwork(req.MagicNetworkURL, req.MagicNetworkAPIKey, cfg, publicKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("MagicNetwork registration failed: %v", err)})
		return
	}
	
	// Save WireGuard config
	wgCfg := config.WireGuardConfig{
		Enabled:            true,
		PrivateKey:         privateKey,
		PublicKey:          publicKey,
		AssignedIP:         wgResp.AssignedIP,
		ServerPubKey:       wgResp.ServerPubKey,
		ServerEndpoint:     req.ServerEndpoint, // Use provided endpoint
		ServerIP:           wgResp.ServerIP,
		Configured:         true,
		MagicNetworkURL:    req.MagicNetworkURL,
		MagicNetworkAPIKey: req.MagicNetworkAPIKey,
	}
	
	if err := s.config.SetWireGuard(wgCfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save config: %v", err)})
		return
	}
	
	// Configure native WireGuard
	nativeConfig := &wireguard.Config{
		PrivateKey:     privateKey,
		PublicKey:      publicKey,
		AssignedIP:     wgResp.AssignedIP,
		ServerPubKey:   wgResp.ServerPubKey,
		ServerEndpoint: req.ServerEndpoint,
		PersistentKA:   25, // NAT keepalive
	}
	
	if err := s.wireguard.Configure(nativeConfig); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to configure WireGuard: %v", err)})
		return
	}
	
	// Bring up interface
	if err := s.wireguard.Up(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to bring up WireGuard: %v", err)})
		return
	}
	
	// Enable on boot
	s.wireguard.EnableOnBoot()
	
	log.Printf("🔐 MagicNetwork tunnel established: %s -> %s", wgResp.AssignedIP, wgResp.ServerIP)
	
	c.JSON(http.StatusOK, gin.H{
		"status":      "ok",
		"assigned_ip": wgResp.AssignedIP,
		"server_ip":   wgResp.ServerIP,
		"message":     "MagicNetwork tunnel established",
	})
}

// MagicNetworkResponse from MagicNetwork API
type MagicNetworkResponse struct {
	AssignedIP   string `json:"assigned_ip"`
	ServerPubKey string `json:"public_key"`
	ServerIP     string `json:"server_ip"`
}

// registerWithMagicNetwork calls MagicNetwork API to register this node
func (s *Server) registerWithMagicNetwork(url, apiKey string, cfg config.NodeConfig, publicKey string) (*MagicNetworkResponse, error) {
	// Build request
	reqBody := map[string]string{
		"id":         cfg.Platform.WorkerID,
		"name":       cfg.NodeName,
		"public_key": publicKey,
	}
	
	// If no worker ID, use MAC address
	if reqBody["id"] == "" {
		reqBody["id"] = "mb_" + cfg.MAC
	}
	
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Create request
	req, err := http.NewRequest("POST", url+"/api/peers", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	
	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("MagicNetwork API error (%d): %s", resp.StatusCode, string(respBody))
	}
	
	// Parse response
	var result struct {
		Status string `json:"status"`
		Peer   struct {
			AssignedIP string `json:"assigned_ip"`
		} `json:"peer"`
		Server struct {
			PublicKey string `json:"public_key"`
			ServerIP  string `json:"server_ip"`
		} `json:"server"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return &MagicNetworkResponse{
		AssignedIP:   result.Peer.AssignedIP,
		ServerPubKey: result.Server.PublicKey,
		ServerIP:     result.Server.ServerIP,
	}, nil
}

func (s *Server) handleAPIMagicNetworkUp(c *gin.Context) {
	if err := s.wireguard.Up(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) handleAPIMagicNetworkDown(c *gin.Context) {
	if err := s.wireguard.Down(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) handleAPIMagicNetworkRestart(c *gin.Context) {
	if err := s.wireguard.Restart(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

