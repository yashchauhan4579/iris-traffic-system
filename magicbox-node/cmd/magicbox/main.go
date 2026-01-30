package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/irisdrone/magicbox-node/internal/central"
	"github.com/irisdrone/magicbox-node/internal/config"
	"github.com/irisdrone/magicbox-node/internal/decoder"
	"github.com/irisdrone/magicbox-node/internal/natsserver"
	"github.com/irisdrone/magicbox-node/internal/platform"
	"github.com/irisdrone/magicbox-node/internal/queue"
	"github.com/irisdrone/magicbox-node/internal/streamer"
	"github.com/irisdrone/magicbox-node/internal/web"
)

var (
	version   = "1.0.0"
	buildTime = "unknown"
)

func main() {
	// Command line flags
	configPath := flag.String("config", "/etc/magicbox/config.json", "Path to config file")
	dataDir := flag.String("data", "/var/lib/magicbox", "Path to data directory")
	webPort := flag.Int("port", 8080, "Web UI port")
	natsPort := flag.Int("nats-port", 4222, "NATS server port")
	enableStreamer := flag.Bool("enable-streamer", true, "Enable frame streaming pipeline")
	showVersion := flag.Bool("version", false, "Show version")
	install := flag.Bool("install", false, "Install MagicBox as systemd service")
	uninstall := flag.Bool("uninstall", false, "Uninstall MagicBox systemd service")
	flag.Parse()

	if *showVersion {
		fmt.Printf("MagicBox Node v%s (built %s)\n", version, buildTime)
		os.Exit(0)
	}

	if *install {
		if err := installService(); err != nil {
			log.Fatalf("Installation failed: %v", err)
		}
		fmt.Println("✅ MagicBox installed successfully!")
		fmt.Println("   Binary: /opt/magicbox/magicbox")
		fmt.Println("   Config: /opt/magicbox/config.json")
		fmt.Println("   Data: /opt/magicbox/data")
		fmt.Println("   Service: magicbox.service")
		fmt.Println("")
		fmt.Println("Service is enabled and started!")
		fmt.Println("")
		fmt.Println("To check status:")
		fmt.Println("  sudo systemctl status magicbox")
		fmt.Println("")
		fmt.Println("To view logs:")
		fmt.Println("  sudo journalctl -u magicbox -f")
		os.Exit(0)
	}

	if *uninstall {
		if err := uninstallService(); err != nil {
			log.Fatalf("Uninstallation failed: %v", err)
		}
		fmt.Println("✅ MagicBox uninstalled successfully!")
		os.Exit(0)
	}

	log.Printf("🚀 Starting MagicBox Node v%s", version)

	// Detect hardware and available decoders
	hwInfo := decoder.Init()
	log.Printf("🎬 Decoder: %s backend with %s acceleration", hwInfo.Backend, hwInfo.Type)

	// Initialize config manager
	cfg, err := config.NewManager(*configPath, *dataDir)
	if err != nil {
		log.Fatalf("Failed to initialize config: %v", err)
	}

	// Start embedded NATS server
	nats, err := natsserver.New(natsserver.Config{
		Port:       *natsPort,
		MaxPayload: 8 * 1024 * 1024, // 8MB for frames
	})
	if err != nil {
		log.Fatalf("Failed to start NATS server: %v", err)
	}
	defer nats.Shutdown()

	// Initialize event queue
	eventQueue, err := queue.NewFileQueue(cfg.GetQueueDir())
	if err != nil {
		log.Fatalf("Failed to initialize event queue: %v", err)
	}

	// Initialize platform client
	platformClient := platform.NewClient(cfg, eventQueue)

	// Initialize streaming pipeline (optional, can be disabled for management-only mode)
	var pipeline *streamer.Pipeline
	if *enableStreamer {
		pipeline = streamer.NewPipeline(cfg, nats)
	}

	// Initialize central NATS client (forwards events/frames to central)
	centralClient := central.NewClient(cfg, nats)

	// Initialize web server with all components
	webServer := web.NewServer(cfg, platformClient, eventQueue, nats, pipeline, centralClient, *webPort)

	// Start background services
	go platformClient.Start()
	go eventQueue.StartProcessor()

	// Start streaming pipeline if enabled
	if pipeline != nil {
		go pipeline.Start()
	}

	// Start central NATS forwarder
	if err := centralClient.Start(); err != nil {
		log.Printf("⚠️ Central NATS client failed to start: %v", err)
	}

	// Start web server
	go func() {
		if err := webServer.Start(); err != nil {
			log.Fatalf("Web server failed: %v", err)
		}
	}()

	log.Printf("✅ MagicBox Node running")
	log.Printf("🌐 Web UI: http://localhost:%d", *webPort)
	log.Printf("📡 NATS: nats://localhost:%d", *natsPort)
	if *enableStreamer {
		log.Printf("🎥 Streamer: enabled (subscribe to frames.<camera_id>)")
	} else {
		log.Printf("🎥 Streamer: disabled")
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("🛑 Shutting down...")
	if pipeline != nil {
		pipeline.Stop()
	}
	centralClient.Stop()
	platformClient.Stop()
	eventQueue.Stop()
	webServer.Stop()
}

// installService installs MagicBox as a systemd service
func installService() error {
	// Check if running as root
	if os.Geteuid() != 0 {
		return fmt.Errorf("installation requires root privileges (use sudo)")
	}

	// Get the current binary path
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	installDir := "/opt/magicbox"
	binPath := filepath.Join(installDir, "magicbox")
	configPath := filepath.Join(installDir, "config.json")
	dataDir := filepath.Join(installDir, "data")
	serviceFile := "/etc/systemd/system/magicbox.service"

	fmt.Println("Installing MagicBox Node...")
	fmt.Printf("  Source: %s\n", exePath)
	fmt.Printf("  Target: %s\n", binPath)

	// Create directories
	dirs := []string{installDir, dataDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		fmt.Printf("  ✓ Created directory: %s\n", dir)
	}

	// Copy binary
	sourceFile, err := os.Open(exePath)
	if err != nil {
		return fmt.Errorf("failed to open source binary: %w", err)
	}
	defer sourceFile.Close()

	destFile, err := os.OpenFile(binPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("failed to create target binary: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}
	fmt.Printf("  ✓ Copied binary to: %s\n", binPath)

	// Create default config if it doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		defaultConfig := `{
  "nodeName": "",
  "nodeModel": "Generic Linux",
  "mac": "",
  "state": "unconfigured",
  "platform": {
    "serverUrl": ""
  },
  "wireguard": {
    "enabled": false,
    "configured": false
  },
  "cameras": [],
  "configVersion": 0,
  "lastSync": "0001-01-01T00:00:00Z",
  "createdAt": "0001-01-01T00:00:00Z",
  "updatedAt": "0001-01-01T00:00:00Z"
}
`
		if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
			return fmt.Errorf("failed to create default config: %w", err)
		}
		fmt.Printf("  ✓ Created default config: %s\n", configPath)
	}

	// Create systemd service file
	serviceContent := `[Unit]
Description=MagicBox Node - IRIS Edge Worker
Documentation=https://github.com/irisdrone/magicbox-node
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=/opt/magicbox

ExecStart=/opt/magicbox/magicbox \
    -config /opt/magicbox/config.json \
    -data /opt/magicbox/data \
    -port 8080

Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=magicbox

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ReadWritePaths=/opt/magicbox

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

# Environment
Environment=GOGC=100
Environment=GOMAXPROCS=4

[Install]
WantedBy=multi-user.target
`

	if err := os.WriteFile(serviceFile, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to create service file: %w", err)
	}
	fmt.Printf("  ✓ Created service file: %s\n", serviceFile)

	// Reload systemd
	cmd := exec.Command("systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}
	fmt.Printf("  ✓ Reloaded systemd daemon\n")

	// Enable service
	cmd = exec.Command("systemctl", "enable", "magicbox.service")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable service: %w", err)
	}
	fmt.Printf("  ✓ Enabled magicbox.service\n")

	// Start service
	cmd = exec.Command("systemctl", "start", "magicbox.service")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}
	fmt.Printf("  ✓ Started magicbox.service\n")

	return nil
}

// uninstallService removes MagicBox systemd service
func uninstallService() error {
	// Check if running as root
	if os.Geteuid() != 0 {
		return fmt.Errorf("uninstallation requires root privileges (use sudo)")
	}

	fmt.Println("Uninstalling MagicBox Node...")

	// Stop and disable service
	cmd := exec.Command("systemctl", "stop", "magicbox.service")
	cmd.Run() // Ignore errors if not running

	cmd = exec.Command("systemctl", "disable", "magicbox.service")
	cmd.Run() // Ignore errors if not enabled

	// Remove service file
	serviceFile := "/etc/systemd/system/magicbox.service"
	if err := os.Remove(serviceFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file: %w", err)
	}
	fmt.Printf("  ✓ Removed service file: %s\n", serviceFile)

	// Reload systemd
	cmd = exec.Command("systemctl", "daemon-reload")
	cmd.Run() // Ignore errors

	// Optionally remove binary and directories (commented out to preserve data)
	// Uncomment if you want to remove everything:
	// os.RemoveAll("/opt/magicbox")
	// os.RemoveAll("/var/lib/magicbox")
	// os.RemoveAll("/etc/magicbox")

	fmt.Println("  ℹ️  All files preserved at:")
	fmt.Println("     /opt/magicbox")

	return nil
}
