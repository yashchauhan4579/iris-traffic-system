// Package wireguard manages WireGuard server configuration
package wireguard

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	InterfaceName = "wg0"
	ConfigDir     = "/etc/wireguard"
	ConfigFile    = "/etc/wireguard/wg0.conf"
	KeysDir       = "/etc/wireguard/keys"
	DataFile      = "peers.json"
)

// ServerConfig holds server configuration
type ServerConfig struct {
	ListenPort  int    `json:"listen_port"`
	Address     string `json:"address"`      // e.g., "10.10.0.1/24"
	PrivateKey  string `json:"private_key"`
	PublicKey   string `json:"public_key"`
	IPPoolStart string `json:"ip_pool_start"` // e.g., "10.10.0.2"
	IPPoolEnd   string `json:"ip_pool_end"`   // e.g., "10.10.255.254"
}

// Peer represents a registered MagicBox node
type Peer struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	PublicKey   string    `json:"public_key"`
	AssignedIP  string    `json:"assigned_ip"`
	AllowedIPs  string    `json:"allowed_ips"`
	Endpoint    string    `json:"endpoint,omitempty"`    // Last known endpoint
	LastSeen    time.Time `json:"last_seen,omitempty"`
	TransferRx  uint64    `json:"transfer_rx,omitempty"`
	TransferTx  uint64    `json:"transfer_tx,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Server manages WireGuard server
type Server struct {
	config   ServerConfig
	peers    map[string]*Peer // keyed by public key
	dataDir  string
	mu       sync.RWMutex
}

// NewServer creates a new WireGuard server manager
func NewServer(dataDir string, listenPort int, address string) (*Server, error) {
	s := &Server{
		config: ServerConfig{
			ListenPort:  listenPort,
			Address:     address,
			IPPoolStart: "10.10.0.2",
			IPPoolEnd:   "10.10.255.254",
		},
		peers:   make(map[string]*Peer),
		dataDir: dataDir,
	}

	// Load existing peers
	if err := s.loadPeers(); err != nil {
		log.Printf("⚠️ Could not load existing peers: %v", err)
	}

	return s, nil
}

// Initialize sets up WireGuard server if not already configured
func (s *Server) Initialize() error {
	// Check if WireGuard is installed
	if _, err := exec.LookPath("wg"); err != nil {
		return fmt.Errorf("wireguard not installed: %w", err)
	}

	// Ensure keys directory exists
	if err := os.MkdirAll(KeysDir, 0700); err != nil {
		return fmt.Errorf("failed to create keys directory: %w", err)
	}

	// Generate or load server keys
	privKeyPath := filepath.Join(KeysDir, "server_private.key")
	pubKeyPath := filepath.Join(KeysDir, "server_public.key")

	if _, err := os.Stat(privKeyPath); os.IsNotExist(err) {
		log.Println("🔑 Generating server keys...")
		
		// Generate private key
		cmd := exec.Command("wg", "genkey")
		privKey, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to generate private key: %w", err)
		}
		s.config.PrivateKey = strings.TrimSpace(string(privKey))

		// Derive public key
		cmd = exec.Command("wg", "pubkey")
		cmd.Stdin = strings.NewReader(s.config.PrivateKey)
		pubKey, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("failed to derive public key: %w", err)
		}
		s.config.PublicKey = strings.TrimSpace(string(pubKey))

		// Save keys
		if err := os.WriteFile(privKeyPath, []byte(s.config.PrivateKey), 0600); err != nil {
			return fmt.Errorf("failed to save private key: %w", err)
		}
		if err := os.WriteFile(pubKeyPath, []byte(s.config.PublicKey), 0644); err != nil {
			return fmt.Errorf("failed to save public key: %w", err)
		}

		log.Printf("✅ Server keys generated")
	} else {
		// Load existing keys
		privData, err := os.ReadFile(privKeyPath)
		if err != nil {
			return fmt.Errorf("failed to read private key: %w", err)
		}
		s.config.PrivateKey = strings.TrimSpace(string(privData))

		pubData, err := os.ReadFile(pubKeyPath)
		if err != nil {
			return fmt.Errorf("failed to read public key: %w", err)
		}
		s.config.PublicKey = strings.TrimSpace(string(pubData))

		log.Printf("✅ Server keys loaded")
	}

	log.Printf("🔐 Server Public Key: %s", s.config.PublicKey)
	return nil
}

// Start starts the WireGuard interface
func (s *Server) Start() error {
	// Enable IP forwarding
	if err := s.enableIPForwarding(); err != nil {
		log.Printf("⚠️ Failed to enable IP forwarding: %v", err)
		// Continue anyway, user might have enabled it manually
	}

	// Check if interface already exists
	cmd := exec.Command("ip", "link", "show", InterfaceName)
	if cmd.Run() == nil {
		log.Printf("✅ WireGuard interface %s already running", InterfaceName)
		return nil
	}

	// Write config file
	if err := s.writeConfig(); err != nil {
		return err
	}

	// Start interface
	cmd = exec.Command("wg-quick", "up", InterfaceName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start WireGuard: %s - %w", output, err)
	}

	log.Printf("⚡ WireGuard interface %s started", InterfaceName)
	return nil
}

// enableIPForwarding enables IP forwarding for routing between peers
func (s *Server) enableIPForwarding() error {
	// Enable IP forwarding temporarily
	cmd := exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to enable IP forwarding: %s - %w", output, err)
	}

	// Make it persistent
	sysctlFile := "/etc/sysctl.d/99-wireguard.conf"
	content := "net.ipv4.ip_forward = 1\n"
	if err := os.WriteFile(sysctlFile, []byte(content), 0644); err != nil {
		log.Printf("⚠️ Failed to write sysctl config: %v", err)
		// Not critical, continue
	}

	log.Printf("✅ IP forwarding enabled")
	return nil
}

// Stop stops the WireGuard interface
func (s *Server) Stop() error {
	cmd := exec.Command("wg-quick", "down", InterfaceName)
	cmd.Run() // Ignore errors if not running
	log.Printf("🔌 WireGuard interface %s stopped", InterfaceName)
	return nil
}

// writeConfig writes the WireGuard config file
func (s *Server) writeConfig() error {
	var sb strings.Builder

	sb.WriteString("[Interface]\n")
	sb.WriteString(fmt.Sprintf("PrivateKey = %s\n", s.config.PrivateKey))
	sb.WriteString(fmt.Sprintf("Address = %s\n", s.config.Address))
	sb.WriteString(fmt.Sprintf("ListenPort = %d\n", s.config.ListenPort))
	sb.WriteString("SaveConfig = false\n")
	sb.WriteString("\n")

	// Add peers
	s.mu.RLock()
	for _, peer := range s.peers {
		sb.WriteString("[Peer]\n")
		sb.WriteString(fmt.Sprintf("PublicKey = %s\n", peer.PublicKey))
		sb.WriteString(fmt.Sprintf("AllowedIPs = %s\n", peer.AllowedIPs))
		sb.WriteString("\n")
	}
	s.mu.RUnlock()

	// Write file
	if err := os.MkdirAll(ConfigDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(ConfigFile, []byte(sb.String()), 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// RegisterPeer registers a new peer and returns assigned IP
func (s *Server) RegisterPeer(id, name, publicKey string) (*Peer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if peer already exists
	if existing, ok := s.peers[publicKey]; ok {
		return existing, nil
	}

	// Allocate IP
	ip, err := s.allocateIP()
	if err != nil {
		return nil, err
	}

	peer := &Peer{
		ID:         id,
		Name:       name,
		PublicKey:  publicKey,
		AssignedIP: ip,
		AllowedIPs: ip + "/32",
		CreatedAt:  time.Now(),
	}

	s.peers[publicKey] = peer

	// Add peer to running WireGuard
	if err := s.addPeerToWG(peer); err != nil {
		delete(s.peers, publicKey)
		return nil, err
	}

	// Rewrite config file to include new peer
	if err := s.writeConfig(); err != nil {
		log.Printf("⚠️ Failed to rewrite config file: %v", err)
	}

	// Save peers
	if err := s.savePeers(); err != nil {
		log.Printf("⚠️ Failed to save peers: %v", err)
	}

	log.Printf("✅ Registered peer: %s (%s) -> %s", name, id, ip)
	return peer, nil
}

// RemovePeer removes a peer
func (s *Server) RemovePeer(publicKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	peer, ok := s.peers[publicKey]
	if !ok {
		return fmt.Errorf("peer not found")
	}

	// Remove from WireGuard
	cmd := exec.Command("wg", "set", InterfaceName, "peer", publicKey, "remove")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove peer from WireGuard: %w", err)
	}

	delete(s.peers, publicKey)

	// Save peers
	if err := s.savePeers(); err != nil {
		log.Printf("⚠️ Failed to save peers: %v", err)
	}

	log.Printf("🗑️ Removed peer: %s (%s)", peer.Name, peer.ID)
	return nil
}

// addPeerToWG adds a peer to the running WireGuard interface
func (s *Server) addPeerToWG(peer *Peer) error {
	cmd := exec.Command("wg", "set", InterfaceName,
		"peer", peer.PublicKey,
		"allowed-ips", peer.AllowedIPs,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add peer: %s - %w", output, err)
	}
	return nil
}

// allocateIP finds the next available IP
func (s *Server) allocateIP() (string, error) {
	usedIPs := make(map[string]bool)
	for _, peer := range s.peers {
		usedIPs[peer.AssignedIP] = true
	}

	// Parse IP range
	startIP := net.ParseIP(s.config.IPPoolStart).To4()
	endIP := net.ParseIP(s.config.IPPoolEnd).To4()

	if startIP == nil || endIP == nil {
		return "", fmt.Errorf("invalid IP pool configuration")
	}

	// Find next available
	for ip := startIP; !ip.Equal(endIP); incrementIP(ip) {
		ipStr := ip.String()
		if !usedIPs[ipStr] {
			return ipStr, nil
		}
	}

	return "", fmt.Errorf("no available IPs in pool")
}

func incrementIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// GetPeers returns all registered peers
func (s *Server) GetPeers() []*Peer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	peers := make([]*Peer, 0, len(s.peers))
	for _, peer := range s.peers {
		peers = append(peers, peer)
	}
	return peers
}

// GetPeer returns a specific peer
func (s *Server) GetPeer(publicKey string) *Peer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.peers[publicKey]
}

// GetConfig returns server configuration
func (s *Server) GetConfig() ServerConfig {
	return s.config
}

// UpdatePeerStatus updates peer status from wg show output
func (s *Server) UpdatePeerStatus() {
	cmd := exec.Command("wg", "show", InterfaceName, "dump")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 8 {
			continue
		}

		pubKey := fields[0]
		if peer, ok := s.peers[pubKey]; ok {
			peer.Endpoint = fields[2]
			if fields[4] != "0" {
				// Has handshake
				peer.LastSeen = time.Now()
			}
			fmt.Sscanf(fields[5], "%d", &peer.TransferRx)
			fmt.Sscanf(fields[6], "%d", &peer.TransferTx)
		}
	}
}

// savePeers saves peers to disk
func (s *Server) savePeers() error {
	data, err := json.MarshalIndent(s.peers, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(s.dataDir, DataFile)
	return os.WriteFile(path, data, 0644)
}

// loadPeers loads peers from disk
func (s *Server) loadPeers() error {
	path := filepath.Join(s.dataDir, DataFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &s.peers)
}

