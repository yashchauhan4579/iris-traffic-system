// Package wireguard manages native WireGuard configuration on MagicBox
package wireguard

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/curve25519"
)

const (
	// Interface name for MagicBox WireGuard tunnel
	InterfaceName = "wg-iris"

	// Default paths
	ConfigDir  = "/etc/wireguard"
	ConfigFile = "/etc/wireguard/wg-iris.conf"
	KeyDir     = "/etc/wireguard/keys"
)

// Config holds WireGuard configuration from platform
type Config struct {
	PrivateKey     string `json:"private_key"`      // Generated locally
	PublicKey      string `json:"public_key"`       // Generated locally
	AssignedIP     string `json:"assigned_ip"`      // e.g., "10.10.0.10/24"
	ServerPubKey   string `json:"server_pubkey"`    // Platform's public key
	ServerEndpoint string `json:"server_endpoint"`  // e.g., "platform.example.com:51820"
	DNS            string `json:"dns,omitempty"`    // Optional DNS server
	PersistentKA   int    `json:"persistent_keepalive"` // Keepalive interval (25 for NAT)
}

// Status represents current WireGuard status
type Status struct {
	Installed     bool      `json:"installed"`
	InterfaceUp   bool      `json:"interface_up"`
	PublicKey     string    `json:"public_key,omitempty"`
	AssignedIP    string    `json:"assigned_ip,omitempty"`
	ServerPubKey  string    `json:"server_pubkey,omitempty"`
	LastHandshake time.Time `json:"last_handshake,omitempty"`
	TransferRx    uint64    `json:"transfer_rx,omitempty"`
	TransferTx    uint64    `json:"transfer_tx,omitempty"`
	Connected     bool      `json:"connected"` // Has recent handshake
}

// Manager handles WireGuard installation and configuration
type Manager struct {
	config     *Config
	configPath string
	mu         sync.RWMutex
	installing bool
}

// NewManager creates a new WireGuard manager
func NewManager() *Manager {
	return &Manager{
		configPath: ConfigFile,
	}
}

// IsInstalled checks if WireGuard is installed
func (m *Manager) IsInstalled() bool {
	_, err := exec.LookPath("wg")
	return err == nil
}

// Install installs WireGuard (runs in background)
func (m *Manager) Install() error {
	m.mu.Lock()
	if m.installing {
		m.mu.Unlock()
		return fmt.Errorf("installation already in progress")
	}
	m.installing = true
	m.mu.Unlock()

	go func() {
		defer func() {
			m.mu.Lock()
			m.installing = false
			m.mu.Unlock()
		}()

		log.Println("🔧 Installing WireGuard...")

		// Detect OS and install
		if err := m.installWireGuard(); err != nil {
			log.Printf("❌ Failed to install WireGuard: %v", err)
			return
		}

		log.Println("✅ WireGuard installed successfully")
	}()

	return nil
}

// installWireGuard handles actual installation
func (m *Manager) installWireGuard() error {
	// Check for common package managers
	if _, err := exec.LookPath("apt-get"); err == nil {
		// Debian/Ubuntu (including Jetson)
		cmd := exec.Command("sudo", "apt-get", "update")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("apt-get update failed: %w", err)
		}

		cmd = exec.Command("sudo", "apt-get", "install", "-y", "wireguard", "wireguard-tools")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("apt-get install failed: %w", err)
		}
		return nil
	}

	if _, err := exec.LookPath("yum"); err == nil {
		// RHEL/CentOS
		cmd := exec.Command("sudo", "yum", "install", "-y", "wireguard-tools")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	if _, err := exec.LookPath("pacman"); err == nil {
		// Arch
		cmd := exec.Command("sudo", "pacman", "-S", "--noconfirm", "wireguard-tools")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	return fmt.Errorf("unsupported package manager - please install wireguard manually")
}

// GenerateKeys generates a new WireGuard keypair
func (m *Manager) GenerateKeys() (privateKey, publicKey string, err error) {
	// Generate private key (32 random bytes, clamped per WireGuard spec)
	var privKey [32]byte
	if _, err := rand.Read(privKey[:]); err != nil {
		return "", "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Clamp the private key per Curve25519 spec
	privKey[0] &= 248
	privKey[31] &= 127
	privKey[31] |= 64

	// Derive public key
	var pubKey [32]byte
	curve25519.ScalarBaseMult(&pubKey, &privKey)

	privateKey = base64.StdEncoding.EncodeToString(privKey[:])
	publicKey = base64.StdEncoding.EncodeToString(pubKey[:])

	return privateKey, publicKey, nil
}

// LoadOrGenerateKeys loads existing keys or generates new ones
func (m *Manager) LoadOrGenerateKeys() (privateKey, publicKey string, err error) {
	privKeyPath := filepath.Join(KeyDir, "private.key")
	pubKeyPath := filepath.Join(KeyDir, "public.key")

	// Try to load existing keys
	if privData, err := os.ReadFile(privKeyPath); err == nil {
		if pubData, err := os.ReadFile(pubKeyPath); err == nil {
			return strings.TrimSpace(string(privData)), strings.TrimSpace(string(pubData)), nil
		}
	}

	// Generate new keys
	privateKey, publicKey, err = m.GenerateKeys()
	if err != nil {
		return "", "", err
	}

	// Ensure key directory exists
	if err := os.MkdirAll(KeyDir, 0700); err != nil {
		// Try without sudo first, might work if running as root
		cmd := exec.Command("sudo", "mkdir", "-p", KeyDir)
		if err := cmd.Run(); err != nil {
			return "", "", fmt.Errorf("failed to create key directory: %w", err)
		}
		cmd = exec.Command("sudo", "chmod", "700", KeyDir)
		cmd.Run()
	}

	// Save keys
	if err := m.writeFileWithSudo(privKeyPath, privateKey, 0600); err != nil {
		return "", "", fmt.Errorf("failed to save private key: %w", err)
	}
	if err := m.writeFileWithSudo(pubKeyPath, publicKey, 0644); err != nil {
		return "", "", fmt.Errorf("failed to save public key: %w", err)
	}

	log.Println("🔑 Generated new WireGuard keypair")
	return privateKey, publicKey, nil
}

// Configure writes WireGuard config and brings up interface
func (m *Manager) Configure(cfg *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.IsInstalled() {
		return fmt.Errorf("wireguard is not installed")
	}

	m.config = cfg

	// Generate config file content
	configContent := m.generateConfig(cfg)

	// Ensure config directory exists
	cmd := exec.Command("sudo", "mkdir", "-p", ConfigDir)
	cmd.Run()

	// Write config file
	if err := m.writeFileWithSudo(m.configPath, configContent, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	log.Printf("📝 WireGuard config written to %s", m.configPath)
	return nil
}

// generateConfig creates WireGuard config file content
func (m *Manager) generateConfig(cfg *Config) string {
	var sb strings.Builder

	sb.WriteString("[Interface]\n")
	sb.WriteString(fmt.Sprintf("PrivateKey = %s\n", cfg.PrivateKey))
	sb.WriteString(fmt.Sprintf("Address = %s\n", cfg.AssignedIP))

	if cfg.DNS != "" {
		sb.WriteString(fmt.Sprintf("DNS = %s\n", cfg.DNS))
	}

	sb.WriteString("\n[Peer]\n")
	sb.WriteString(fmt.Sprintf("PublicKey = %s\n", cfg.ServerPubKey))
	sb.WriteString(fmt.Sprintf("Endpoint = %s\n", cfg.ServerEndpoint))
	// Route all 10.10.x.x traffic through tunnel
	sb.WriteString("AllowedIPs = 10.10.0.0/16\n")

	// Keepalive for NAT traversal (important for 4G/5G connections)
	if cfg.PersistentKA > 0 {
		sb.WriteString(fmt.Sprintf("PersistentKeepalive = %d\n", cfg.PersistentKA))
	} else {
		sb.WriteString("PersistentKeepalive = 25\n")
	}

	return sb.String()
}

// Up brings the WireGuard interface up
func (m *Manager) Up() error {
	// First try wg-quick
	cmd := exec.Command("sudo", "wg-quick", "up", InterfaceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if already up
		if strings.Contains(string(output), "already exists") {
			log.Println("⚡ WireGuard interface already up")
			return nil
		}
		return fmt.Errorf("failed to bring up interface: %s - %w", output, err)
	}

	log.Println("⚡ WireGuard tunnel established")
	return nil
}

// Down brings the WireGuard interface down
func (m *Manager) Down() error {
	cmd := exec.Command("sudo", "wg-quick", "down", InterfaceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Ignore if not up
		if strings.Contains(string(output), "is not a WireGuard interface") {
			return nil
		}
		return fmt.Errorf("failed to bring down interface: %s - %w", output, err)
	}

	log.Println("🔌 WireGuard tunnel disconnected")
	return nil
}

// Restart restarts the WireGuard interface
func (m *Manager) Restart() error {
	m.Down()
	time.Sleep(500 * time.Millisecond)
	return m.Up()
}

// EnableOnBoot enables WireGuard to start on boot
func (m *Manager) EnableOnBoot() error {
	cmd := exec.Command("sudo", "systemctl", "enable", "wg-quick@"+InterfaceName)
	return cmd.Run()
}

// DisableOnBoot disables WireGuard from starting on boot
func (m *Manager) DisableOnBoot() error {
	cmd := exec.Command("sudo", "systemctl", "disable", "wg-quick@"+InterfaceName)
	return cmd.Run()
}

// GetStatus returns current WireGuard status
func (m *Manager) GetStatus() *Status {
	status := &Status{
		Installed: m.IsInstalled(),
	}

	if !status.Installed {
		return status
	}

	// Check if interface is up
	cmd := exec.Command("ip", "link", "show", InterfaceName)
	if err := cmd.Run(); err == nil {
		status.InterfaceUp = true
	}

	// Get detailed status from wg show
	cmd = exec.Command("sudo", "wg", "show", InterfaceName)
	output, err := cmd.Output()
	if err != nil {
		return status
	}

	// Parse wg show output
	m.parseWgShow(string(output), status)

	// Consider connected if handshake within last 3 minutes
	if !status.LastHandshake.IsZero() {
		status.Connected = time.Since(status.LastHandshake) < 3*time.Minute
	}

	// Get assigned IP from config
	m.mu.RLock()
	if m.config != nil {
		status.AssignedIP = m.config.AssignedIP
	}
	m.mu.RUnlock()

	return status
}

// parseWgShow parses output of `wg show` command
func (m *Manager) parseWgShow(output string, status *Status) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "public key:") {
			status.PublicKey = strings.TrimSpace(strings.TrimPrefix(line, "public key:"))
		} else if strings.HasPrefix(line, "peer:") {
			status.ServerPubKey = strings.TrimSpace(strings.TrimPrefix(line, "peer:"))
		} else if strings.HasPrefix(line, "latest handshake:") {
			// Parse handshake time (e.g., "1 minute, 30 seconds ago")
			timeStr := strings.TrimSpace(strings.TrimPrefix(line, "latest handshake:"))
			status.LastHandshake = m.parseHandshakeTime(timeStr)
		} else if strings.HasPrefix(line, "transfer:") {
			// Parse transfer stats
			// Format: "1.23 MiB received, 456.78 KiB sent"
			parts := strings.Split(strings.TrimPrefix(line, "transfer:"), ",")
			if len(parts) >= 2 {
				status.TransferRx = m.parseTransferSize(strings.TrimSpace(parts[0]))
				status.TransferTx = m.parseTransferSize(strings.TrimSpace(parts[1]))
			}
		}
	}
}

// parseHandshakeTime converts handshake time string to time.Time
func (m *Manager) parseHandshakeTime(timeStr string) time.Time {
	// Simple parsing - just check if it contains "ago"
	if !strings.Contains(timeStr, "ago") {
		return time.Time{}
	}

	// Rough estimation
	duration := time.Duration(0)

	if strings.Contains(timeStr, "second") {
		var secs int
		fmt.Sscanf(timeStr, "%d second", &secs)
		duration = time.Duration(secs) * time.Second
	} else if strings.Contains(timeStr, "minute") {
		var mins int
		fmt.Sscanf(timeStr, "%d minute", &mins)
		duration = time.Duration(mins) * time.Minute
	} else if strings.Contains(timeStr, "hour") {
		var hours int
		fmt.Sscanf(timeStr, "%d hour", &hours)
		duration = time.Duration(hours) * time.Hour
	}

	if duration > 0 {
		return time.Now().Add(-duration)
	}
	return time.Time{}
}

// parseTransferSize converts size string to bytes
func (m *Manager) parseTransferSize(sizeStr string) uint64 {
	var value float64
	var unit string
	fmt.Sscanf(sizeStr, "%f %s", &value, &unit)

	multiplier := uint64(1)
	switch {
	case strings.HasPrefix(unit, "KiB"):
		multiplier = 1024
	case strings.HasPrefix(unit, "MiB"):
		multiplier = 1024 * 1024
	case strings.HasPrefix(unit, "GiB"):
		multiplier = 1024 * 1024 * 1024
	}

	return uint64(value * float64(multiplier))
}

// writeFileWithSudo writes content to file using sudo
func (m *Manager) writeFileWithSudo(path, content string, perm os.FileMode) error {
	// First try direct write (works if running as root)
	if err := os.WriteFile(path, []byte(content), perm); err == nil {
		return nil
	}

	// Use sudo tee
	cmd := exec.Command("sudo", "tee", path)
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = nil // Suppress output
	if err := cmd.Run(); err != nil {
		return err
	}

	// Set permissions
	cmd = exec.Command("sudo", "chmod", fmt.Sprintf("%o", perm), path)
	return cmd.Run()
}

// TestConnection tests connectivity to the server through tunnel
func (m *Manager) TestConnection(serverIP string) bool {
	cmd := exec.Command("ping", "-c", "1", "-W", "3", serverIP)
	return cmd.Run() == nil
}

