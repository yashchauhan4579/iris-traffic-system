package platform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/irisdrone/magicbox-node/internal/config"
	"github.com/irisdrone/magicbox-node/internal/queue"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
)

// Client handles communication with the IRIS platform
type Client struct {
	config      *config.Manager
	queue       *queue.FileQueue
	httpClient  *http.Client
	stopChan    chan struct{}
	wg          sync.WaitGroup
	mu          sync.Mutex
}

// RegistrationRequest is sent when registering with a token
type RegistrationRequest struct {
	Token      string `json:"token"`
	DeviceName string `json:"device_name"`
	MAC        string `json:"mac"`
	Model      string `json:"model"`
	IP         string `json:"ip"`
	Version    string `json:"version,omitempty"`
}

// RegistrationResponse from platform
type RegistrationResponse struct {
	Status    string `json:"status"`    // "registered"
	WorkerID  string `json:"worker_id"`
	AuthToken string `json:"auth_token"`
	Message   string `json:"message,omitempty"`
}

// ApprovalRequest for token-less registration
type ApprovalRequest struct {
	DeviceName string `json:"device_name"`
	MAC        string `json:"mac"`
	Model      string `json:"model"`
	IP         string `json:"ip"`
}

// ApprovalResponse from platform
type ApprovalResponse struct {
	Success   bool   `json:"success"`
	RequestID string `json:"requestId"`
	Message   string `json:"message,omitempty"`
}

// ApprovalStatusResponse for checking approval status
type ApprovalStatusResponse struct {
	Status    string `json:"status"` // pending, approved, rejected
	WorkerID  string `json:"workerId,omitempty"`
	AuthToken string `json:"authToken,omitempty"`
	Message   string `json:"message,omitempty"`
}

// HeartbeatRequest sent periodically
type HeartbeatRequest struct {
	Status        string                 `json:"status"`
	Resources     map[string]interface{} `json:"resources"`
	CameraStatus  []CameraStatus         `json:"cameraStatus"`
	QueueStats    queue.QueueStats       `json:"queueStats"`
	ConfigVersion int                    `json:"configVersion"`
}

// CameraStatus for each camera
type CameraStatus struct {
	DeviceID  string  `json:"deviceId"`
	Connected bool    `json:"connected"`
	FPS       float64 `json:"fps"`
	Errors    int     `json:"errors"`
}

// WorkerConfig from platform
type WorkerConfig struct {
	ConfigVersion int                   `json:"configVersion"`
	Cameras       []config.CameraConfig `json:"cameras"`
}

// NewClient creates a new platform client
func NewClient(cfg *config.Manager, q *queue.FileQueue) *Client {
	return &Client{
		config: cfg,
		queue:  q,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		stopChan: make(chan struct{}),
	}
}

// Start begins background tasks
func (c *Client) Start() {
	c.wg.Add(2)
	
	// Heartbeat loop
	go c.heartbeatLoop()
	
	// Config sync loop
	go c.configSyncLoop()
}

// Stop halts background tasks
func (c *Client) Stop() {
	close(c.stopChan)
	c.wg.Wait()
}

// RegisterWithToken registers using a provisioning token
func (c *Client) RegisterWithToken(serverURL, token, nodeName string) error {
	cfg := c.config.Get()
	
	req := RegistrationRequest{
		Token:      token,
		DeviceName: nodeName,
		MAC:        cfg.MAC,
		Model:      cfg.NodeModel,
		IP:         getLocalIP(),
		Version:    "1.0.0",
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(
		serverURL+"/api/workers/register",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("registration failed: %s", string(respBody))
	}

	var regResp RegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if regResp.Status != "registered" || regResp.WorkerID == "" {
		return fmt.Errorf("registration failed: %s", regResp.Message)
	}

	// Update config with registration info
	platCfg := config.PlatformConfig{
		ServerURL: serverURL,
		Token:     token,
		WorkerID:  regResp.WorkerID,
		AuthToken: regResp.AuthToken,
	}
	
	if err := c.config.SetPlatformConfig(platCfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	if err := c.config.SetNodeName(nodeName); err != nil {
		return fmt.Errorf("failed to save node name: %w", err)
	}
	if err := c.config.SetState(config.StateApproved); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	log.Printf("✅ Successfully registered as worker %s", regResp.WorkerID)
	return nil
}

// RequestApproval requests approval without a token
func (c *Client) RequestApproval(serverURL, nodeName string) error {
	cfg := c.config.Get()

	req := ApprovalRequest{
		DeviceName: nodeName,
		MAC:        cfg.MAC,
		Model:      cfg.NodeModel,
		IP:         getLocalIP(),
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.httpClient.Post(
		serverURL+"/api/workers/request-approval",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("approval request failed: %s", string(respBody))
	}

	var appResp ApprovalResponse
	if err := json.NewDecoder(resp.Body).Decode(&appResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if !appResp.Success {
		return fmt.Errorf("approval request failed: %s", appResp.Message)
	}

	// Update config with pending approval
	platCfg := config.PlatformConfig{
		ServerURL: serverURL,
		RequestID: appResp.RequestID,
	}

	if err := c.config.SetPlatformConfig(platCfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	if err := c.config.SetNodeName(nodeName); err != nil {
		return fmt.Errorf("failed to save node name: %w", err)
	}
	if err := c.config.SetState(config.StatePending); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	log.Printf("📨 Approval request submitted (ID: %s)", appResp.RequestID)
	return nil
}

// CheckApprovalStatus checks if approval request was approved
func (c *Client) CheckApprovalStatus() (*ApprovalStatusResponse, error) {
	cfg := c.config.Get()
	
	if cfg.Platform.RequestID == "" {
		return nil, fmt.Errorf("no pending approval request")
	}

	resp, err := c.httpClient.Get(
		cfg.Platform.ServerURL + "/api/workers/approval-status/" + cfg.Platform.RequestID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	var status ApprovalStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// If approved, update config
	if status.Status == "approved" && status.WorkerID != "" {
		platCfg := cfg.Platform
		platCfg.WorkerID = status.WorkerID
		platCfg.AuthToken = status.AuthToken
		
		if err := c.config.SetPlatformConfig(platCfg); err != nil {
			return nil, fmt.Errorf("failed to save config: %w", err)
		}
		if err := c.config.SetState(config.StateApproved); err != nil {
			return nil, fmt.Errorf("failed to save state: %w", err)
		}
		log.Printf("✅ Approval granted! Worker ID: %s", status.WorkerID)
	}

	return &status, nil
}

// FetchConfig fetches the latest config from platform
func (c *Client) FetchConfig() (*WorkerConfig, error) {
	cfg := c.config.Get()

	if cfg.Platform.WorkerID == "" || cfg.Platform.AuthToken == "" {
		return nil, fmt.Errorf("not registered with platform")
	}

	req, err := http.NewRequest(
		"GET",
		cfg.Platform.ServerURL+"/api/workers/"+cfg.Platform.WorkerID+"/config",
		nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Platform.AuthToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch config: %s", string(respBody))
	}

	var workerCfg WorkerConfig
	if err := json.NewDecoder(resp.Body).Decode(&workerCfg); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	return &workerCfg, nil
}

// SendHeartbeat sends a heartbeat to the platform
func (c *Client) SendHeartbeat() error {
	cfg := c.config.Get()

	if cfg.Platform.WorkerID == "" || cfg.Platform.AuthToken == "" {
		return nil
	}

	hb := HeartbeatRequest{
		Status:        string(cfg.State),
		Resources:     c.getResources(),
		CameraStatus:  c.getCameraStatus(),
		QueueStats:    c.queue.GetStats(),
		ConfigVersion: cfg.ConfigVersion,
	}

	body, err := json.Marshal(hb)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(
		"POST",
		cfg.Platform.ServerURL+"/api/workers/"+cfg.Platform.WorkerID+"/heartbeat",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.Platform.AuthToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("heartbeat failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat failed with status %d", resp.StatusCode)
	}

	return nil
}

// SendEvent sends an event to the platform (used by queue processor)
func (c *Client) SendEvent(event *queue.Event) error {
	cfg := c.config.Get()

	if cfg.Platform.WorkerID == "" || cfg.Platform.AuthToken == "" {
		return fmt.Errorf("not registered with platform")
	}

	// Create multipart form if there are images
	var body bytes.Buffer
	var contentType string

	if len(event.Images) > 0 {
		writer := multipart.NewWriter(&body)
		
		// Add event data
		eventData, _ := json.Marshal(event)
		if err := writer.WriteField("event", string(eventData)); err != nil {
			return err
		}

		// Add images
		for i, imgPath := range event.Images {
			file, err := os.Open(imgPath)
			if err != nil {
				continue
			}
			defer file.Close()

			part, err := writer.CreateFormFile(fmt.Sprintf("image_%d", i), filepath.Base(imgPath))
			if err != nil {
				continue
			}
			io.Copy(part, file)
		}

		writer.Close()
		contentType = writer.FormDataContentType()
	} else {
		eventData, _ := json.Marshal(event)
		body.Write(eventData)
		contentType = "application/json"
	}

	req, err := http.NewRequest(
		"POST",
		cfg.Platform.ServerURL+"/api/events/ingest",
		&body,
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+cfg.Platform.AuthToken)
	req.Header.Set("X-Worker-ID", cfg.Platform.WorkerID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("event rejected: %s", string(respBody))
	}

	return nil
}

// Disconnect disconnects from the platform
func (c *Client) Disconnect() error {
	if err := c.config.Reset(); err != nil {
		return err
	}
	return nil
}

// heartbeatLoop sends periodic heartbeats
func (c *Client) heartbeatLoop() {
	defer c.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			cfg := c.config.Get()
			if cfg.State == config.StateApproved || cfg.State == config.StateActive {
				if err := c.SendHeartbeat(); err != nil {
					log.Printf("⚠️ Heartbeat failed: %v", err)
				}
			} else if cfg.State == config.StatePending {
				// Check approval status
				status, err := c.CheckApprovalStatus()
				if err != nil {
					log.Printf("⚠️ Approval check failed: %v", err)
				} else if status.Status == "approved" {
					log.Printf("✅ Approval granted!")
				} else if status.Status == "rejected" {
					log.Printf("❌ Approval rejected: %s", status.Message)
					c.config.SetState(config.StateError)
				}
			}
		}
	}
}

// configSyncLoop periodically syncs config from platform
func (c *Client) configSyncLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			cfg := c.config.Get()
			if cfg.State == config.StateApproved || cfg.State == config.StateActive {
				workerCfg, err := c.FetchConfig()
				if err != nil {
					log.Printf("⚠️ Config sync failed: %v", err)
					continue
				}

				// Check if config has changed
				if workerCfg.ConfigVersion > cfg.ConfigVersion {
					log.Printf("📥 New config version %d (was %d)", workerCfg.ConfigVersion, cfg.ConfigVersion)
					c.config.SetCameras(workerCfg.Cameras)
					c.config.SetConfigVersion(workerCfg.ConfigVersion)
					c.config.UpdateLastSync()
				}
			}
		}
	}
}

// getResources returns current system resources
func (c *Client) getResources() map[string]interface{} {
	resources := make(map[string]interface{})

	// CPU
	if cpuPercent, err := cpu.Percent(0, false); err == nil && len(cpuPercent) > 0 {
		resources["cpuPercent"] = cpuPercent[0]
	}

	// Memory
	if memInfo, err := mem.VirtualMemory(); err == nil {
		resources["memoryTotal"] = memInfo.Total
		resources["memoryUsed"] = memInfo.Used
		resources["memoryPercent"] = memInfo.UsedPercent
	}

	// GPU (Jetson specific)
	if gpuUtil, err := getJetsonGPUUsage(); err == nil {
		resources["gpuPercent"] = gpuUtil
	}

	// Temperature
	if temp, err := getTemperature(); err == nil {
		resources["temperature"] = temp
	}

	return resources
}

// getCameraStatus returns status of all cameras
func (c *Client) getCameraStatus() []CameraStatus {
	cfg := c.config.Get()
	status := make([]CameraStatus, len(cfg.Cameras))
	
	for i, cam := range cfg.Cameras {
		status[i] = CameraStatus{
			DeviceID:  cam.DeviceID,
			Connected: cam.Enabled, // TODO: actual connection status
			FPS:       float64(cam.FPS),
			Errors:    0,
		}
	}
	
	return status
}

// getLocalIP returns the local IP address
func getLocalIP() string {
	addrs, err := localAddresses()
	if err != nil || len(addrs) == 0 {
		return "unknown"
	}
	return addrs[0]
}

func localAddresses() ([]string, error) {
	var ips []string
	addrs, err := getNetworkAddresses()
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*ipNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP.String())
			}
		}
	}
	return ips, nil
}

type ipNet struct {
	IP   ipAddress
	Mask []byte
}

type ipAddress []byte

func (ip ipAddress) IsLoopback() bool {
	return len(ip) == 4 && ip[0] == 127
}

func (ip ipAddress) To4() ipAddress {
	if len(ip) == 4 {
		return ip
	}
	return nil
}

func (ip ipAddress) String() string {
	if len(ip) == 4 {
		return fmt.Sprintf("%d.%d.%d.%d", ip[0], ip[1], ip[2], ip[3])
	}
	return ""
}

func getNetworkAddresses() ([]interface{}, error) {
	ifaces, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return nil, err
	}
	
	var addrs []interface{}
	for _, iface := range ifaces {
		name := iface.Name()
		if name == "lo" {
			continue
		}
		// Use net package for actual IP
		// This is simplified - in production use net.InterfaceAddrs()
	}
	return addrs, nil
}

// getJetsonGPUUsage reads Jetson GPU utilization
func getJetsonGPUUsage() (float64, error) {
	data, err := os.ReadFile("/sys/devices/gpu.0/load")
	if err != nil {
		return 0, err
	}
	var load float64
	fmt.Sscanf(string(data), "%f", &load)
	return load / 10, nil // Jetson reports in 0.1% units
}

// getTemperature reads CPU temperature
func getTemperature() (float64, error) {
	// Try thermal zone
	data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return 0, err
	}
	var temp float64
	fmt.Sscanf(string(data), "%f", &temp)
	return temp / 1000, nil // Convert millidegrees to degrees
}

// ==================== Camera Sync ====================

// CameraSyncResult contains the result of syncing cameras to platform
type CameraSyncResult struct {
	Created int
	Updated int
}

// SyncCameras syncs local cameras to the platform
func (c *Client) SyncCameras(cameras []config.CameraConfig) (*CameraSyncResult, error) {
	cfg := c.config.Get()
	
	if cfg.Platform.WorkerID == "" || cfg.Platform.AuthToken == "" {
		return nil, fmt.Errorf("not connected to platform")
	}
	
	// Build the request payload - include device_id so platform uses our ID
	type cameraRequest struct {
		DeviceID string `json:"device_id"`
		Name     string `json:"name"`
		RTSPUrl  string `json:"rtsp_url"`
	}
	
	payload := make([]cameraRequest, len(cameras))
	for i, cam := range cameras {
		payload[i] = cameraRequest{
			DeviceID: cam.DeviceID,
			Name:     cam.Name,
			RTSPUrl:  cam.RTSPUrl,
		}
	}
	
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cameras: %w", err)
	}
	
	url := fmt.Sprintf("%s/api/workers/%s/cameras", cfg.Platform.ServerURL, cfg.Platform.WorkerID)
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", cfg.Platform.AuthToken)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sync failed: %s", string(body))
	}
	
	var result struct {
		Success   bool     `json:"success"`
		Created   int      `json:"created"`
		Updated   int      `json:"updated"`
		DeviceIDs []string `json:"device_ids"`
	}
	
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	
	// Note: Device IDs are now generated at source (MagicBox) and preserved
	// No need to update local config - IDs should already match
	
	log.Printf("📷 Synced %d cameras to platform (created: %d, updated: %d)", 
		len(cameras), result.Created, result.Updated)
	
	return &CameraSyncResult{
		Created: result.Created,
		Updated: result.Updated,
	}, nil
}

// DeleteCamera removes a camera from the platform
func (c *Client) DeleteCamera(deviceID string) error {
	cfg := c.config.Get()
	
	if cfg.Platform.WorkerID == "" || cfg.Platform.AuthToken == "" {
		return fmt.Errorf("not connected to platform")
	}
	
	url := fmt.Sprintf("%s/api/workers/%s/cameras/%s", cfg.Platform.ServerURL, cfg.Platform.WorkerID, deviceID)
	
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("X-Auth-Token", cfg.Platform.AuthToken)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed: %s", string(body))
	}
	
	log.Printf("📷 Deleted camera %s from platform", deviceID)
	return nil
}

// WireGuardSetupRequest to platform
type WireGuardSetupRequest struct {
	WorkerID  string `json:"worker_id"`
	PublicKey string `json:"public_key"`
}

// WireGuardSetupResponse from platform
type WireGuardSetupResponse struct {
	AssignedIP     string `json:"assigned_ip"`
	ServerPubKey   string `json:"server_pubkey"`
	ServerEndpoint string `json:"server_endpoint"`
	ServerIP       string `json:"server_ip"`
}

// SetupWireGuard requests WireGuard configuration from the platform
func (c *Client) SetupWireGuard(publicKey string) (*WireGuardSetupResponse, error) {
	cfg := c.config.Get()
	
	if cfg.Platform.WorkerID == "" || cfg.Platform.AuthToken == "" {
		return nil, fmt.Errorf("not connected to platform")
	}
	
	reqBody := WireGuardSetupRequest{
		WorkerID:  cfg.Platform.WorkerID,
		PublicKey: publicKey,
	}
	
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	url := fmt.Sprintf("%s/api/workers/%s/wireguard/setup", cfg.Platform.ServerURL, cfg.Platform.WorkerID)
	
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-Token", cfg.Platform.AuthToken)
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("WireGuard setup failed: %s", string(respBody))
	}
	
	var result struct {
		Status    string                  `json:"status"`
		WireGuard WireGuardSetupResponse `json:"wireguard"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	log.Printf("🔐 WireGuard setup received: IP=%s", result.WireGuard.AssignedIP)
	return &result.WireGuard, nil
}

