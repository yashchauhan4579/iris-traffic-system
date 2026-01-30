// MagicNetwork - WireGuard VPN Server for IRIS MagicBox nodes
package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/irisdrone/magicnetwork/internal/api"
	"github.com/irisdrone/magicnetwork/internal/wireguard"
)

const systemdService = `[Unit]
Description=MagicNetwork WireGuard VPN Server
After=network.target

[Service]
Type=simple
ExecStart=%s --port %d --wg-port %d --address %s --data %s --api-key %s
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`

func main() {
	// Parse flags
	port := flag.Int("port", 8080, "API server port")
	wgPort := flag.Int("wg-port", 51820, "WireGuard listen port")
	address := flag.String("address", "10.10.0.1/24", "WireGuard server address")
	dataDir := flag.String("data", "/var/lib/magicnetwork", "Data directory")
	apiKey := flag.String("api-key", "", "API key for authentication (auto-generated if empty)")
	genKey := flag.Bool("gen-key", false, "Generate a new API key and exit")
	install := flag.Bool("install", false, "Install as systemd service and start")
	uninstall := flag.Bool("uninstall", false, "Uninstall systemd service")
	flag.Parse()

	// Generate API key mode
	if *genKey {
		key := generateAPIKey()
		fmt.Printf("Generated API Key: %s\n", key)
		return
	}

	// Install mode
	if *install {
		installService(*port, *wgPort, *address, *dataDir)
		return
	}

	// Uninstall mode
	if *uninstall {
		uninstallService()
		return
	}

	// Get API key from flag or environment
	key := *apiKey
	if key == "" {
		key = os.Getenv("MAGICNETWORK_API_KEY")
	}
	if key == "" {
		// Generate one
		key = generateAPIKey()
		log.Printf("⚠️  No API key provided, generated: %s", key)
		log.Printf("   Set MAGICNETWORK_API_KEY or use --api-key flag")
	}

	// Ensure data directory exists
	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		log.Fatalf("❌ Failed to create data directory: %v", err)
	}

	// Initialize WireGuard server
	wg, err := wireguard.NewServer(*dataDir, *wgPort, *address)
	if err != nil {
		log.Fatalf("❌ Failed to create WireGuard server: %v", err)
	}

	// Initialize server keys
	if err := wg.Initialize(); err != nil {
		log.Fatalf("❌ Failed to initialize WireGuard: %v", err)
	}

	// Start WireGuard interface
	if err := wg.Start(); err != nil {
		log.Fatalf("❌ Failed to start WireGuard: %v", err)
	}

	// Setup API
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	apiHandler := api.NewAPI(wg, key)

	// Public endpoints (no auth required)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
	router.GET("/api/info", apiHandler.GetServerInfo)

	// Protected endpoints
	protected := router.Group("/api")
	protected.Use(apiHandler.AuthMiddleware())
	{
		protected.GET("/status", apiHandler.GetStatus)
		protected.GET("/peers", apiHandler.GetPeers)
		protected.POST("/peers", apiHandler.RegisterPeer)
		protected.GET("/peers/:pubkey", apiHandler.GetPeer)
		protected.DELETE("/peers/:pubkey", apiHandler.RemovePeer)
	}

	// Print startup info
	cfg := wg.GetConfig()
	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║              🌐 MagicNetwork VPN Server                    ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  WireGuard Interface: %-36s  ║\n", wireguard.InterfaceName)
	fmt.Printf("║  WireGuard Port:      %-36d  ║\n", cfg.ListenPort)
	fmt.Printf("║  Server IP:           %-36s  ║\n", cfg.Address)
	fmt.Printf("║  API Port:            %-36d  ║\n", *port)
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Public Key: %s  ║\n", cfg.PublicKey[:40]+"...")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  API Key: %s...                      ║\n", key[:16])
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Handle shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("🛑 Shutting down...")
		wg.Stop()
		os.Exit(0)
	}()

	// Start API server
	log.Printf("🚀 API server listening on :%d", *port)
	if err := router.Run(fmt.Sprintf(":%d", *port)); err != nil {
		log.Fatalf("❌ Server error: %v", err)
	}
}

func generateAPIKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "mn_" + hex.EncodeToString(b)
}

func installService(port, wgPort int, address, dataDir string) {
	// Check if running as root
	if os.Geteuid() != 0 {
		fmt.Println("❌ Please run with sudo for installation")
		os.Exit(1)
	}

	fmt.Println("🔧 Installing MagicNetwork as systemd service...")

	// Use /opt/magicnetwork as base directory
	baseDir := "/opt/magicnetwork"
	dataDir = filepath.Join(baseDir, "data")
	installPath := filepath.Join(baseDir, "magicnetwork")

	// Create directories
	fmt.Printf("📁 Creating directory %s...\n", baseDir)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatalf("❌ Failed to create directory: %v", err)
	}

	// Get current binary path
	binaryPath, err := os.Executable()
	if err != nil {
		log.Fatalf("❌ Failed to get executable path: %v", err)
	}
	binaryPath, _ = filepath.Abs(binaryPath)

	// Copy binary to /opt/magicnetwork if not already there
	if binaryPath != installPath {
		fmt.Printf("📦 Copying binary to %s...\n", installPath)
		input, err := os.ReadFile(binaryPath)
		if err != nil {
			log.Fatalf("❌ Failed to read binary: %v", err)
		}
		if err := os.WriteFile(installPath, input, 0755); err != nil {
			log.Fatalf("❌ Failed to copy binary: %v", err)
		}
	}

	// Generate API key
	apiKey := generateAPIKey()
	fmt.Printf("🔑 Generated API Key: %s\n", apiKey)

	// Save API key to file for reference
	keyFile := filepath.Join(baseDir, "api_key")
	if err := os.WriteFile(keyFile, []byte(apiKey+"\n"), 0600); err != nil {
		log.Printf("⚠️ Failed to save API key to file: %v", err)
	} else {
		fmt.Printf("📝 API key saved to: %s\n", keyFile)
	}

	// Create systemd service file
	serviceContent := fmt.Sprintf(systemdService,
		installPath, port, wgPort, address, dataDir, apiKey)

	servicePath := "/etc/systemd/system/magicnetwork.service"
	fmt.Printf("📝 Creating systemd service at %s...\n", servicePath)
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		log.Fatalf("❌ Failed to create service file: %v", err)
	}

	// Reload systemd
	fmt.Println("🔄 Reloading systemd...")
	cmd := exec.Command("systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		log.Fatalf("❌ Failed to reload systemd: %v", err)
	}

	// Enable service
	fmt.Println("✅ Enabling service on boot...")
	cmd = exec.Command("systemctl", "enable", "magicnetwork")
	if err := cmd.Run(); err != nil {
		log.Fatalf("❌ Failed to enable service: %v", err)
	}

	// Start service
	fmt.Println("🚀 Starting service...")
	cmd = exec.Command("systemctl", "start", "magicnetwork")
	if err := cmd.Run(); err != nil {
		log.Fatalf("❌ Failed to start service: %v", err)
	}

	// Wait a moment and check status
	fmt.Println("⏳ Checking service status...")
	cmd = exec.Command("systemctl", "is-active", "magicnetwork")
	output, _ := cmd.Output()

	fmt.Println()
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║         ✅ MagicNetwork Installed Successfully!            ║")
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Install Dir: %-44s  ║\n", baseDir)
	fmt.Printf("║  Binary:      %-44s  ║\n", installPath)
	fmt.Printf("║  Data:        %-44s  ║\n", dataDir)
	fmt.Printf("║  API Port:    %-44d  ║\n", port)
	fmt.Printf("║  WG Port:     %-44d  ║\n", wgPort)
	fmt.Printf("║  Status:      %-44s  ║\n", string(output))
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  API Key: %s  ║\n", apiKey)
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Println("║  Files:                                                    ║")
	fmt.Printf("║    %s/api_key       - API key\n", baseDir)
	fmt.Printf("║    %s/data/peers.json - Registered peers\n", baseDir)
	fmt.Println("╠════════════════════════════════════════════════════════════╣")
	fmt.Println("║  Commands:                                                 ║")
	fmt.Println("║    systemctl status magicnetwork   - Check status          ║")
	fmt.Println("║    journalctl -u magicnetwork -f   - View logs             ║")
	fmt.Println("║    systemctl restart magicnetwork  - Restart               ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("⚠️  Make sure to open port", wgPort, "UDP in your firewall!")
}

func uninstallService() {
	// Check if running as root
	if os.Geteuid() != 0 {
		fmt.Println("❌ Please run with sudo for uninstallation")
		os.Exit(1)
	}

	fmt.Println("🔧 Uninstalling MagicNetwork...")

	// Stop service
	fmt.Println("🛑 Stopping service...")
	exec.Command("systemctl", "stop", "magicnetwork").Run()

	// Disable service
	fmt.Println("🚫 Disabling service...")
	exec.Command("systemctl", "disable", "magicnetwork").Run()

	// Remove service file
	fmt.Println("🗑️ Removing service file...")
	os.Remove("/etc/systemd/system/magicnetwork.service")

	// Reload systemd
	exec.Command("systemctl", "daemon-reload").Run()

	fmt.Println()
	fmt.Println("✅ MagicNetwork uninstalled successfully!")
	fmt.Println("📁 Installation directory NOT removed: /opt/magicnetwork")
	fmt.Println("   Remove manually if needed: sudo rm -rf /opt/magicnetwork")
}

