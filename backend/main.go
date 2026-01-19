package main

import (
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/irisdrone/backend/database"
	"github.com/irisdrone/backend/handlers"
	"github.com/irisdrone/backend/natsserver"
	"github.com/irisdrone/backend/services"
	"github.com/nats-io/nats.go"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Connect to database
	if err := database.Connect(); err != nil {
		log.Fatalf("❌ Failed to start server: %v", err)
	}
	defer database.Close()

	// Start embedded NATS server for central communication
	// Using port 4233 to avoid conflict with MagicBox local NATS on 4222
	natsPort := 4233
	natsServer, err := natsserver.New(natsserver.Config{
		Port:       natsPort,
		MaxPayload: 8 * 1024 * 1024, // 8MB for frames
	})
	if err != nil {
		log.Fatalf("❌ Failed to start NATS server: %v", err)
	}
	defer natsServer.Shutdown()
	log.Printf("📡 Central NATS server started on port %d", natsPort)

	// Connect to NATS for feed hub
	natsConn, err := nats.Connect(fmt.Sprintf("nats://localhost:%d", natsPort))
	if err != nil {
		log.Fatalf("❌ Failed to connect to NATS: %v", err)
	}
	defer natsConn.Close()

	// Initialize feed hub for WebSocket streaming
	feedHub := services.NewFeedHub(natsConn)
	go feedHub.Run()
	handlers.SetFeedHub(feedHub)
	log.Println("📺 Feed hub initialized")

	// Initialize WireGuard service
	wgEndpoint := os.Getenv("WIREGUARD_ENDPOINT")
	if wgEndpoint == "" {
		wgEndpoint = "localhost:51820" // Default for dev
	}
	handlers.InitWireGuard(wgEndpoint)
	log.Printf("🔐 WireGuard service initialized (endpoint: %s)", wgEndpoint)

	// Setup Gin router
	if os.Getenv("ENV") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	// CORS middleware
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowMethods = []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"}
	config.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Auth-Token", "X-Worker-ID"}
	router.Use(cors.New(config))

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "ok",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})

	// Serve heatmaps statically
	usr, err := user.Current()
	if err == nil {
		heatmapsDir := filepath.Join(usr.HomeDir, "heatmaps")
		log.Printf("📁 Serving heatmaps from: %s", heatmapsDir)
		router.Static("/heatmaps", heatmapsDir)
		
		// Serve uploaded images from ~/itms/data
		uploadsDir := filepath.Join(usr.HomeDir, "itms", "data")
		log.Printf("📁 Serving uploads from: %s", uploadsDir)
		router.Static("/uploads", uploadsDir)
	}

	// Debug route for heatmaps
	router.GET("/debug/heatmaps", func(c *gin.Context) {
		usr, err := user.Current()
		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to get user home directory"})
			return
		}
		heatmapsDir := filepath.Join(usr.HomeDir, "heatmaps")
		
		files, err := os.ReadDir(heatmapsDir)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error(), "heatmapsDir": heatmapsDir})
			return
		}

		fileNames := make([]string, 0, len(files))
		for _, file := range files {
			fileNames = append(fileNames, file.Name())
		}

		sampleFiles := fileNames
		if len(sampleFiles) > 5 {
			sampleFiles = sampleFiles[:5]
		}

		testFile := "185_20251225_193400_469259.jpg"
		testPath := filepath.Join(heatmapsDir, testFile)
		exists := false
		if _, err := os.Stat(testPath); err == nil {
			exists = true
		}

		c.JSON(200, gin.H{
			"heatmapsDir": heatmapsDir,
			"fileCount":   len(fileNames),
			"sampleFiles": sampleFiles,
			"testFile":    testFile,
			"exists":      exists,
		})
	})

	// WebSocket route for camera feeds (outside /api group)
	router.GET("/ws/feeds", handlers.HandleFeedWebSocket)

	// API Routes
	api := router.Group("/api")
	{
		// Feed hub stats
		api.GET("/feeds/stats", handlers.GetFeedHubStats)

		// Device routes
		devices := api.Group("/devices")
		{
			devices.GET("", handlers.GetDevices)
			devices.GET("/:id/latest", handlers.GetDeviceLatest)
			devices.GET("/analytics/surges", handlers.GetDeviceSurges)
		}

		// Ingest routes (legacy)
		ingest := api.Group("/ingest")
		{
			ingest.POST("", handlers.PostIngest)
		}

		// Event ingest from edge workers
		events := api.Group("/events")
		{
			events.POST("/ingest", handlers.IngestEvents)
		}

		// Worker routes (for edge workers to call)
		workers := api.Group("/workers")
		{
			// Registration
			workers.POST("/register", handlers.RegisterWorker)
			workers.POST("/request-approval", handlers.RequestApproval)
			workers.GET("/approval-status/:requestId", handlers.CheckApprovalStatus)
			
			// Authenticated worker endpoints
			workers.POST("/:id/heartbeat", handlers.WorkerHeartbeat)
			workers.GET("/:id/config", handlers.GetWorkerConfig)
			
			// Worker camera discovery/management
			workers.POST("/:id/cameras", handlers.ReportCameras)
			workers.GET("/:id/cameras", handlers.GetWorkerDiscoveredCameras)
			workers.DELETE("/:id/cameras/:deviceId", handlers.DeleteWorkerCamera)
			
			// WireGuard setup
			workers.POST("/:id/wireguard/setup", handlers.SetupWireGuard)
		}

		// Admin routes for worker management
		admin := api.Group("/admin")
		{
			// Workers
			adminWorkers := admin.Group("/workers")
			{
				adminWorkers.GET("", handlers.GetWorkers)
				adminWorkers.GET("/:id", handlers.GetWorker)
				adminWorkers.PUT("/:id", handlers.UpdateWorker)
				adminWorkers.POST("/:id/revoke", handlers.RevokeWorker)
				adminWorkers.DELETE("/:id", handlers.DeleteWorker)
				
				// Camera assignments
				adminWorkers.GET("/:id/cameras", handlers.GetWorkerCameras)
				adminWorkers.POST("/:id/cameras", handlers.AssignCameras)
				adminWorkers.DELETE("/:id/cameras/:deviceId", handlers.UnassignCamera)
				
				// Approval requests
				adminWorkers.GET("/approval-requests", handlers.GetApprovalRequests)
				adminWorkers.POST("/approval-requests/:id/approve", handlers.ApproveWorkerRequest)
				adminWorkers.POST("/approval-requests/:id/reject", handlers.RejectWorkerRequest)
			}
			
			// Worker tokens
			tokens := admin.Group("/worker-tokens")
			{
				tokens.POST("", handlers.CreateWorkerToken)
				tokens.POST("/bulk", handlers.BulkCreateWorkerTokens)
				tokens.GET("", handlers.GetWorkerTokens)
				tokens.GET("/:id", handlers.GetWorkerToken)
				tokens.POST("/:id/revoke", handlers.RevokeWorkerToken)
				tokens.DELETE("/:id", handlers.DeleteWorkerToken)
			}

			// WireGuard management
			wg := admin.Group("/wireguard")
			{
				wg.GET("/status", handlers.GetWireGuardStatus)
				wg.DELETE("/peers/:pubkey", handlers.RemoveWireGuardPeer)
			}
		}

		// Crowd routes
		crowd := api.Group("/crowd")
		{
			crowd.POST("/analysis", handlers.PostCrowdAnalysis)
			crowd.GET("/analysis", handlers.GetCrowdAnalysis)
			crowd.GET("/analysis/latest", handlers.GetLatestCrowdAnalysis)
			crowd.POST("/alerts", handlers.PostCrowdAlert)
			crowd.GET("/alerts", handlers.GetCrowdAlerts)
			crowd.PATCH("/alerts/:id/resolve", handlers.ResolveCrowdAlert)
			crowd.GET("/hotspots", handlers.GetHotspots)
		}

		// Violations routes (ITMS)
		violations := api.Group("/violations")
		{
			violations.POST("", handlers.PostViolation)
			violations.GET("", handlers.GetViolations)
			violations.GET("/stats", handlers.GetViolationStats)
			violations.GET("/:id", handlers.GetViolation)
			violations.PATCH("/:id/approve", handlers.ApproveViolation)
			violations.PATCH("/:id/reject", handlers.RejectViolation)
			violations.PATCH("/:id/plate", handlers.UpdateViolationPlate)
		}

		// Vehicles routes (ANPR/VCC)
		vehicles := api.Group("/vehicles")
		{
			vehicles.POST("/detect", handlers.PostVehicleDetection)
			vehicles.GET("", handlers.GetVehicles)
			vehicles.GET("/stats", handlers.GetVehicleStats)
			vehicles.GET("/:id", handlers.GetVehicle)
			vehicles.PATCH("/:id", handlers.UpdateVehicle)
			vehicles.GET("/:id/detections", handlers.GetVehicleDetections)
			vehicles.GET("/:id/violations", handlers.GetVehicleViolations)
			vehicles.POST("/:id/watchlist", handlers.AddToWatchlist)
			vehicles.DELETE("/:id/watchlist", handlers.RemoveFromWatchlist)
		}

		// Watchlist routes
		watchlist := api.Group("/watchlist")
		{
			watchlist.GET("", handlers.GetWatchlist)
		}

		// VCC (Vehicle Classification and Counting) routes
		vcc := api.Group("/vcc")
		{
			vcc.GET("/stats", handlers.GetVCCStats)
			vcc.GET("/device/:deviceId", handlers.GetVCCByDevice)
			vcc.GET("/realtime", handlers.GetVCCRealtime)
			vcc.GET("/events", handlers.GetVCCEvents)
		}
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "3001"
	}

	log.Printf("🚀 Server running on http://localhost:%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

