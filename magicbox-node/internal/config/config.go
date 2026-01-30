package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// NodeState represents the current state of the node
type NodeState string

const (
	StateUnconfigured NodeState = "unconfigured"
	StatePending      NodeState = "pending"
	StateApproved     NodeState = "approved"
	StateActive       NodeState = "active"
	StateError        NodeState = "error"
)

// PlatformConfig holds the platform connection settings
type PlatformConfig struct {
	ServerURL   string `json:"serverUrl"`
	ServerIP    string `json:"serverIp,omitempty"`    // Direct server IP (optional, for direct connection)
	Token       string `json:"token,omitempty"`
	WorkerID    string `json:"workerId,omitempty"`
	AuthToken   string `json:"authToken,omitempty"`
	RequestID   string `json:"requestId,omitempty"` // For approval-based registration
	CentralNATS string `json:"centralNats,omitempty"` // Central NATS URL (e.g., nats://10.0.0.5:4222)
}

// WireGuardConfig holds WireGuard VPN settings
type WireGuardConfig struct {
	Enabled        bool   `json:"enabled"`
	PrivateKey     string `json:"privateKey,omitempty"`     // Stored locally, not synced
	PublicKey      string `json:"publicKey,omitempty"`      // Sent to MagicNetwork
	AssignedIP     string `json:"assignedIp,omitempty"`     // e.g., "10.10.0.10/24"
	ServerPubKey   string `json:"serverPubKey,omitempty"`   // MagicNetwork server's public key
	ServerEndpoint string `json:"serverEndpoint,omitempty"` // e.g., "vpn.example.com:51820"
	ServerIP       string `json:"serverIp,omitempty"`       // e.g., "10.10.0.1"
	Configured     bool   `json:"configured"`               // Has been set up
	
	// MagicNetwork server
	MagicNetworkURL    string `json:"magicNetworkUrl,omitempty"`    // e.g., "http://vpn.example.com:8080"
	MagicNetworkAPIKey string `json:"magicNetworkApiKey,omitempty"` // API key for MagicNetwork
}

// CameraConfig holds camera settings
type CameraConfig struct {
	DeviceID   string   `json:"deviceId"`
	Name       string   `json:"name"`
	RTSPUrl    string   `json:"rtspUrl"`
	Analytics  []string `json:"analytics"` // ["anpr", "vcc", "crowd"]
	FPS        int      `json:"fps"`
	Resolution string   `json:"resolution"`
	Enabled    bool     `json:"enabled"`
}

// NodeConfig holds the complete node configuration
type NodeConfig struct {
	// Identity
	NodeName    string `json:"nodeName"`
	NodeModel   string `json:"nodeModel"`
	MAC         string `json:"mac"`
	
	// State
	State       NodeState `json:"state"`
	
	// Platform connection
	Platform    PlatformConfig `json:"platform"`
	
	// WireGuard VPN
	WireGuard   WireGuardConfig `json:"wireguard"`
	
	// Camera assignments
	Cameras     []CameraConfig `json:"cameras"`
	
	// Config version (from platform)
	ConfigVersion int `json:"configVersion"`
	
	// Timestamps
	LastSync    time.Time `json:"lastSync"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Manager handles configuration persistence and access
type Manager struct {
	configPath string
	dataDir    string
	config     *NodeConfig
	mu         sync.RWMutex
}

// NewManager creates a new config manager
func NewManager(configPath, dataDir string) (*Manager, error) {
	m := &Manager{
		configPath: configPath,
		dataDir:    dataDir,
	}

	// Ensure directories exist
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	if err := os.MkdirAll(m.GetQueueDir(), 0755); err != nil {
		return nil, fmt.Errorf("failed to create queue directory: %w", err)
	}
	if err := os.MkdirAll(m.GetImagesDir(), 0755); err != nil {
		return nil, fmt.Errorf("failed to create images directory: %w", err)
	}
	if err := os.MkdirAll(m.GetLogsDir(), 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Load or create config
	if err := m.load(); err != nil {
		// Create default config
		m.config = m.createDefaultConfig()
		if err := m.save(); err != nil {
			return nil, fmt.Errorf("failed to save default config: %w", err)
		}
	}

	return m, nil
}

// GetQueueDir returns the events queue directory
func (m *Manager) GetQueueDir() string {
	return filepath.Join(m.dataDir, "events")
}

// GetImagesDir returns the images directory
func (m *Manager) GetImagesDir() string {
	return filepath.Join(m.dataDir, "images")
}

// GetLogsDir returns the logs directory
func (m *Manager) GetLogsDir() string {
	return filepath.Join(m.dataDir, "logs")
}

// Get returns a copy of the current config
func (m *Manager) Get() NodeConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return *m.config
}

// GetState returns the current node state
func (m *Manager) GetState() NodeState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.State
}

// SetState updates the node state
func (m *Manager) SetState(state NodeState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.State = state
	m.config.UpdatedAt = time.Now()
	return m.saveUnsafe()
}

// SetPlatformConfig updates the platform configuration
func (m *Manager) SetPlatformConfig(platform PlatformConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.Platform = platform
	m.config.UpdatedAt = time.Now()
	return m.saveUnsafe()
}

// SetNodeName updates the node name
func (m *Manager) SetNodeName(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.NodeName = name
	m.config.UpdatedAt = time.Now()
	return m.saveUnsafe()
}

// SetCameras updates the camera configurations
func (m *Manager) SetCameras(cameras []CameraConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.Cameras = cameras
	m.config.UpdatedAt = time.Now()
	return m.saveUnsafe()
}

// SetConfigVersion updates the config version
func (m *Manager) SetConfigVersion(version int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.ConfigVersion = version
	m.config.UpdatedAt = time.Now()
	return m.saveUnsafe()
}

// GetWireGuard returns the WireGuard configuration
func (m *Manager) GetWireGuard() WireGuardConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.WireGuard
}

// SetWireGuard updates the WireGuard configuration
func (m *Manager) SetWireGuard(wg WireGuardConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.WireGuard = wg
	m.config.UpdatedAt = time.Now()
	return m.saveUnsafe()
}

// UpdateLastSync updates the last sync timestamp
func (m *Manager) UpdateLastSync() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.LastSync = time.Now()
	return m.saveUnsafe()
}

// Reset clears the configuration to default
func (m *Manager) Reset() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = m.createDefaultConfig()
	return m.saveUnsafe()
}

// IsConfigured returns true if the node is connected to a platform
func (m *Manager) IsConfigured() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.State != StateUnconfigured && m.config.Platform.ServerURL != ""
}

// load reads config from file
func (m *Manager) load() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	var cfg NodeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	m.config = &cfg
	return nil
}

// save writes config to file (must hold lock)
func (m *Manager) save() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveUnsafe()
}

// saveUnsafe writes config to file (caller must hold lock)
func (m *Manager) saveUnsafe() error {
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.configPath, data, 0644)
}

// createDefaultConfig creates a new default configuration
func (m *Manager) createDefaultConfig() *NodeConfig {
	mac := getMACAddress()
	hostname, _ := os.Hostname()
	
	return &NodeConfig{
		NodeName:  hostname,
		NodeModel: detectNodeModel(),
		MAC:       mac,
		State:     StateUnconfigured,
		Platform:  PlatformConfig{},
		Cameras:   []CameraConfig{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// getMACAddress returns the primary MAC address
func getMACAddress() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "unknown"
	}

	for _, iface := range interfaces {
		// Skip loopback and virtual interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		if len(iface.HardwareAddr) == 0 {
			continue
		}
		// Skip docker/virtual interfaces
		if iface.Name == "docker0" || iface.Name == "br-" {
			continue
		}
		return iface.HardwareAddr.String()
	}
	return "unknown"
}

// detectNodeModel detects the hardware model
func detectNodeModel() string {
	// Try to detect Jetson model
	data, err := os.ReadFile("/proc/device-tree/model")
	if err == nil {
		return string(data)
	}
	
	// Check for Jetson via tegra
	if _, err := os.Stat("/sys/devices/soc0/family"); err == nil {
		return "NVIDIA Jetson"
	}
	
	// Default
	return "Generic Linux"
}

