package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/irisdrone/backend/database"
	"github.com/irisdrone/backend/models"
	"gorm.io/gorm"
)

// Helper function to generate random ID
func generateID(prefix string) string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return prefix + "_" + hex.EncodeToString(bytes)[:16]
}

// Helper function to generate auth token
func generateAuthToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// ==================== Worker Registration ====================

// RegisterWorkerRequest - Token-based registration
type RegisterWorkerRequest struct {
	Token      string `json:"token" binding:"required"`
	DeviceName string `json:"device_name" binding:"required"`
	IP         string `json:"ip" binding:"required"`
	MAC        string `json:"mac" binding:"required"`
	Model      string `json:"model" binding:"required"`
	Version    string `json:"version,omitempty"`
}

// RegisterWorker handles token-based worker registration
// POST /api/workers/register
func RegisterWorker(c *gin.Context) {
	var req RegisterWorkerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find and validate token
	var token models.WorkerToken
	result := database.DB.Where("token = ? AND is_revoked = false", req.Token).First(&token)
	if result.Error != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		return
	}

	// Check if token is already used
	if token.UsedBy != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token has already been used"})
		return
	}

	// Check if token is expired
	if token.ExpiresAt != nil && token.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Token has expired"})
		return
	}

	// Check if device with this MAC already exists
	var existingWorker models.Worker
	if err := database.DB.Where("mac = ?", req.MAC).First(&existingWorker).Error; err == nil {
		// Device exists - update and return
		existingWorker.Name = req.DeviceName
		existingWorker.IP = req.IP
		existingWorker.Status = models.WorkerStatusActive
		existingWorker.LastSeen = time.Now()
		if req.Version != "" {
			existingWorker.Version = &req.Version
		}
		database.DB.Save(&existingWorker)

		c.JSON(http.StatusOK, gin.H{
			"status":     "reconnected",
			"worker_id":  existingWorker.ID,
			"auth_token": existingWorker.AuthToken,
			"message":    "Worker reconnected successfully",
		})
		return
	}

	// Create new worker
	authToken := generateAuthToken()
	now := time.Now()
	worker := models.Worker{
		ID:         generateID("wk"),
		Name:       req.DeviceName,
		Status:     models.WorkerStatusApproved, // Token-based = auto-approved
		IP:         req.IP,
		MAC:        req.MAC,
		Model:      req.Model,
		AuthToken:  authToken,
		ApprovedAt: &now,
		ApprovedBy: &token.CreatedBy,
		LastSeen:   now,
		LastIP:     &req.IP,
	}
	if req.Version != "" {
		worker.Version = &req.Version
	}

	if err := database.DB.Create(&worker).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create worker"})
		return
	}

	// Mark token as used
	token.UsedBy = &worker.ID
	token.UsedAt = &now
	database.DB.Save(&token)

	c.JSON(http.StatusCreated, gin.H{
		"status":     "registered",
		"worker_id":  worker.ID,
		"auth_token": authToken,
		"message":    "Worker registered successfully",
	})
}

// RequestApprovalRequest - Tokenless registration request
type RequestApprovalRequest struct {
	DeviceName string `json:"device_name" binding:"required"`
	IP         string `json:"ip" binding:"required"`
	MAC        string `json:"mac" binding:"required"`
	Model      string `json:"model" binding:"required"`
}

// RequestApproval handles tokenless registration requests (needs admin approval)
// POST /api/workers/request-approval
func RequestApproval(c *gin.Context) {
	var req RequestApprovalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if there's already a pending request for this MAC
	var existingRequest models.WorkerApprovalRequest
	if err := database.DB.Where("mac = ? AND status = 'pending'", req.MAC).First(&existingRequest).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":     "pending",
			"request_id": existingRequest.ID,
			"message":    "Approval request already pending",
		})
		return
	}

	// Check if device is already registered
	var existingWorker models.Worker
	if err := database.DB.Where("mac = ?", req.MAC).First(&existingWorker).Error; err == nil {
		if existingWorker.Status == models.WorkerStatusRevoked {
			c.JSON(http.StatusForbidden, gin.H{"error": "This device has been revoked. Contact administrator."})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":     "already_registered",
			"worker_id":  existingWorker.ID,
			"auth_token": existingWorker.AuthToken,
		})
		return
	}

	// Create approval request
	request := models.WorkerApprovalRequest{
		ID:         generateID("req"),
		DeviceName: req.DeviceName,
		IP:         req.IP,
		MAC:        req.MAC,
		Model:      req.Model,
		Status:     "pending",
	}

	if err := database.DB.Create(&request).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create approval request"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":     "pending",
		"request_id": request.ID,
		"message":    "Approval request submitted. Waiting for admin approval.",
	})
}

// CheckApprovalStatus checks the status of an approval request
// GET /api/workers/approval-status/:requestId
func CheckApprovalStatus(c *gin.Context) {
	requestID := c.Param("requestId")

	var request models.WorkerApprovalRequest
	if err := database.DB.First(&request, "id = ?", requestID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Request not found"})
		return
	}

	response := gin.H{
		"status":     request.Status,
		"request_id": request.ID,
	}

	if request.Status == "approved" && request.WorkerID != nil {
		// Fetch the worker to return auth token
		var worker models.Worker
		if err := database.DB.First(&worker, "id = ?", *request.WorkerID).Error; err == nil {
			response["worker_id"] = worker.ID
			response["auth_token"] = worker.AuthToken
		}
	} else if request.Status == "rejected" {
		response["reject_reason"] = request.RejectReason
	}

	c.JSON(http.StatusOK, response)
}

// ==================== Worker Heartbeat & Config ====================

// HeartbeatRequest - Worker heartbeat data
type HeartbeatRequest struct {
	Resources map[string]interface{} `json:"resources,omitempty"` // CPU, GPU, memory, temp
	Cameras   int                    `json:"cameras_active"`
	Analytics []string               `json:"analytics_running"`
	Events    map[string]int         `json:"events_stats,omitempty"` // Events sent stats
}

// WorkerHeartbeat handles worker heartbeat/status updates
// POST /api/workers/:id/heartbeat
func WorkerHeartbeat(c *gin.Context) {
	workerID := c.Param("id")
	authToken := c.GetHeader("X-Auth-Token")

	// Validate worker
	var worker models.Worker
	if err := database.DB.First(&worker, "id = ?", workerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Worker not found"})
		return
	}

	// Validate auth token
	if worker.AuthToken != authToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid auth token"})
		return
	}

	// Check if worker is revoked
	if worker.Status == models.WorkerStatusRevoked {
		c.JSON(http.StatusForbidden, gin.H{"error": "Worker has been revoked"})
		return
	}

	var req HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update worker status
	ip := c.ClientIP()
	worker.LastSeen = time.Now()
	worker.LastIP = &ip
	worker.Status = models.WorkerStatusActive

	if req.Resources != nil {
		worker.Resources = models.NewJSONB(req.Resources)
	}

	database.DB.Save(&worker)

	// Return current config version (for config sync)
	c.JSON(http.StatusOK, gin.H{
		"status":         "ok",
		"config_version": worker.ConfigVersion,
	})
}

// GetWorkerConfig returns the worker's configuration
// GET /api/workers/:id/config
func GetWorkerConfig(c *gin.Context) {
	workerID := c.Param("id")
	authToken := c.GetHeader("X-Auth-Token")

	// Validate worker
	var worker models.Worker
	if err := database.DB.First(&worker, "id = ?", workerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Worker not found"})
		return
	}

	// Validate auth token
	if worker.AuthToken != authToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid auth token"})
		return
	}

	// Get camera assignments with device details
	var assignments []models.WorkerCameraAssignment
	database.DB.Preload("Device").Where("worker_id = ? AND is_active = true", workerID).Find(&assignments)

	// Build camera config
	cameras := make([]gin.H, 0)
	for _, a := range assignments {
		if a.Device == nil {
			continue
		}
		camera := gin.H{
			"device_id":  a.DeviceID,
			"name":       a.Device.Name,
			"rtsp_url":   a.Device.RTSPUrl,
			"analytics":  a.Analytics,
			"fps":        a.FPS,
			"resolution": a.Resolution,
		}
		cameras = append(cameras, camera)
	}

	c.JSON(http.StatusOK, gin.H{
		"worker_id":      worker.ID,
		"worker_name":    worker.Name,
		"config_version": worker.ConfigVersion,
		"cameras":        cameras,
		"updated_at":     worker.UpdatedAt,
	})
}

// ==================== Worker Camera Discovery ====================

// ReportCameraRequest - Camera discovered/added by worker
type ReportCameraRequest struct {
	DeviceID string `json:"device_id"` // UUID from MagicBox - use this if provided
	Name     string `json:"name" binding:"required"`
	RTSPUrl  string `json:"rtsp_url" binding:"required"`
}

// ReportCameras handles worker reporting discovered cameras
// POST /api/workers/:id/cameras
func ReportCameras(c *gin.Context) {
	workerID := c.Param("id")
	authToken := c.GetHeader("X-Auth-Token")

	// Validate worker
	var worker models.Worker
	if err := database.DB.First(&worker, "id = ?", workerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Worker not found"})
		return
	}

	// Validate auth token
	if worker.AuthToken != authToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid auth token"})
		return
	}

	var cameras []ReportCameraRequest
	if err := c.ShouldBindJSON(&cameras); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	created := 0
	updated := 0
	deviceIDs := []string{}

	for _, cam := range cameras {
		// Check if camera already exists by ID (preferred) or RTSP URL
		var existingDevice models.Device
		var err error
		
		if cam.DeviceID != "" {
			// Check by provided device ID first
			err = database.DB.Where("id = ?", cam.DeviceID).First(&existingDevice).Error
		}
		if err != nil || cam.DeviceID == "" {
			// Fallback: check by RTSP URL for this worker
			err = database.DB.Where("rtsp_url = ? AND worker_id = ?", cam.RTSPUrl, workerID).First(&existingDevice).Error
		}
		
		if err == nil {
			// Update existing
			existingDevice.Name = &cam.Name
			existingDevice.RTSPUrl = &cam.RTSPUrl
			existingDevice.WorkerID = &workerID
			database.DB.Save(&existingDevice)
			updated++
			deviceIDs = append(deviceIDs, existingDevice.ID)
		} else {
			// Create new device - use provided ID or generate one
			deviceID := cam.DeviceID
			if deviceID == "" {
				deviceID = generateID("cam") // Changed prefix from "dev" to "cam"
			}
			device := models.Device{
				ID:       deviceID,
				Type:     models.DeviceTypeCamera,
				Name:     &cam.Name,
				RTSPUrl:  &cam.RTSPUrl,
				WorkerID: &workerID,
				Status:   "discovered", // Mark as discovered, needs admin approval for analytics
				Lat:      0,
				Lng:      0,
			}
			database.DB.Create(&device)
			created++
			deviceIDs = append(deviceIDs, deviceID)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"created":    created,
		"updated":    updated,
		"device_ids": deviceIDs,
	})
}

// GetWorkerDiscoveredCameras returns cameras reported by a worker
// GET /api/workers/:id/cameras
func GetWorkerDiscoveredCameras(c *gin.Context) {
	workerID := c.Param("id")
	authToken := c.GetHeader("X-Auth-Token")

	// Validate worker
	var worker models.Worker
	if err := database.DB.First(&worker, "id = ?", workerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Worker not found"})
		return
	}

	// Validate auth token
	if worker.AuthToken != authToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid auth token"})
		return
	}

	// Get all devices reported by this worker
	var devices []models.Device
	database.DB.Where("worker_id = ?", workerID).Find(&devices)

	// Get assignments to know which analytics are enabled
	var assignments []models.WorkerCameraAssignment
	database.DB.Where("worker_id = ? AND is_active = true", workerID).Find(&assignments)

	// Build assignment map
	assignmentMap := make(map[string]*models.WorkerCameraAssignment)
	for i := range assignments {
		assignmentMap[assignments[i].DeviceID] = &assignments[i]
	}

	// Build response
	result := make([]gin.H, 0)
	for _, d := range devices {
		cam := gin.H{
			"device_id": d.ID,
			"name":      d.Name,
			"rtsp_url":  d.RTSPUrl,
			"status":    d.Status,
		}
		
		// Add analytics if assigned
		if a, ok := assignmentMap[d.ID]; ok {
			cam["analytics"] = a.Analytics
			cam["fps"] = a.FPS
			cam["resolution"] = a.Resolution
			cam["is_active"] = a.IsActive
		}
		
		result = append(result, cam)
	}

	c.JSON(http.StatusOK, gin.H{
		"cameras": result,
	})
}

// DeleteWorkerCamera allows worker to remove a discovered camera
// DELETE /api/workers/:id/cameras/:deviceId
func DeleteWorkerCamera(c *gin.Context) {
	workerID := c.Param("id")
	deviceID := c.Param("deviceId")
	authToken := c.GetHeader("X-Auth-Token")

	// Validate worker
	var worker models.Worker
	if err := database.DB.First(&worker, "id = ?", workerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Worker not found"})
		return
	}

	// Validate auth token
	if worker.AuthToken != authToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid auth token"})
		return
	}

	// Find and delete the device (only if it belongs to this worker)
	result := database.DB.Where("id = ? AND worker_id = ?", deviceID, workerID).Delete(&models.Device{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Camera not found"})
		return
	}

	// Also remove any assignments
	database.DB.Where("device_id = ? AND worker_id = ?", deviceID, workerID).Delete(&models.WorkerCameraAssignment{})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// ==================== Admin: Worker Management ====================

// GetWorkers returns list of all workers (admin)
// GET /api/admin/workers
func GetWorkers(c *gin.Context) {
	status := c.Query("status")

	query := database.DB.Model(&models.Worker{})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var workers []models.Worker
	query.Order("created_at DESC").Find(&workers)

	// Get camera counts for each worker
	type WorkerWithCounts struct {
		models.Worker
		CameraCount int `json:"cameraCount"`
	}

	result := make([]WorkerWithCounts, len(workers))
	for i, w := range workers {
		var count int64
		database.DB.Model(&models.WorkerCameraAssignment{}).Where("worker_id = ? AND is_active = true", w.ID).Count(&count)
		result[i] = WorkerWithCounts{
			Worker:      w,
			CameraCount: int(count),
		}
	}

	c.JSON(http.StatusOK, result)
}

// GetWorker returns a single worker details (admin)
// GET /api/admin/workers/:id
func GetWorker(c *gin.Context) {
	workerID := c.Param("id")

	var worker models.Worker
	if err := database.DB.Preload("CameraAssignments").Preload("CameraAssignments.Device").First(&worker, "id = ?", workerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Worker not found"})
		return
	}

	c.JSON(http.StatusOK, worker)
}

// UpdateWorker updates worker details (admin)
// PUT /api/admin/workers/:id
func UpdateWorker(c *gin.Context) {
	workerID := c.Param("id")

	var worker models.Worker
	if err := database.DB.First(&worker, "id = ?", workerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Worker not found"})
		return
	}

	var req struct {
		Name string                  `json:"name"`
		Tags []string                `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name != "" {
		worker.Name = req.Name
	}
	if req.Tags != nil {
		worker.Tags = models.NewJSONB(req.Tags)
	}

	database.DB.Save(&worker)
	c.JSON(http.StatusOK, worker)
}

// RevokeWorker revokes a worker's access (admin)
// POST /api/admin/workers/:id/revoke
func RevokeWorker(c *gin.Context) {
	workerID := c.Param("id")

	var worker models.Worker
	if err := database.DB.First(&worker, "id = ?", workerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Worker not found"})
		return
	}

	worker.Status = models.WorkerStatusRevoked
	database.DB.Save(&worker)

	c.JSON(http.StatusOK, gin.H{"message": "Worker revoked successfully"})
}

// DeleteWorker deletes a worker (admin)
// DELETE /api/admin/workers/:id
func DeleteWorker(c *gin.Context) {
	workerID := c.Param("id")

	// Delete camera assignments first
	database.DB.Where("worker_id = ?", workerID).Delete(&models.WorkerCameraAssignment{})

	// Delete worker
	result := database.DB.Delete(&models.Worker{}, "id = ?", workerID)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Worker not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Worker deleted successfully"})
}

// ==================== Admin: Approval Requests ====================

// GetApprovalRequests returns pending approval requests (admin)
// GET /api/admin/workers/approval-requests
func GetApprovalRequests(c *gin.Context) {
	status := c.DefaultQuery("status", "pending")

	var requests []models.WorkerApprovalRequest
	database.DB.Where("status = ?", status).Order("created_at DESC").Find(&requests)

	c.JSON(http.StatusOK, requests)
}

// ApproveWorkerRequest approves a worker request (admin)
// POST /api/admin/workers/approval-requests/:id/approve
func ApproveWorkerRequest(c *gin.Context) {
	requestID := c.Param("id")
	adminUser := c.DefaultQuery("admin_user", "admin") // TODO: Get from auth

	var request models.WorkerApprovalRequest
	if err := database.DB.First(&request, "id = ?", requestID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Request not found"})
		return
	}

	if request.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request is not pending"})
		return
	}

	// Create worker
	authToken := generateAuthToken()
	now := time.Now()
	worker := models.Worker{
		ID:         generateID("wk"),
		Name:       request.DeviceName,
		Status:     models.WorkerStatusApproved,
		IP:         request.IP,
		MAC:        request.MAC,
		Model:      request.Model,
		AuthToken:  authToken,
		ApprovedAt: &now,
		ApprovedBy: &adminUser,
		LastSeen:   now,
		LastIP:     &request.IP,
	}

	if err := database.DB.Create(&worker).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create worker"})
		return
	}

	// Update request
	request.Status = "approved"
	request.WorkerID = &worker.ID
	database.DB.Save(&request)

	c.JSON(http.StatusOK, gin.H{
		"message":   "Worker approved successfully",
		"worker_id": worker.ID,
	})
}

// RejectWorkerRequest rejects a worker request (admin)
// POST /api/admin/workers/approval-requests/:id/reject
func RejectWorkerRequest(c *gin.Context) {
	requestID := c.Param("id")
	adminUser := c.DefaultQuery("admin_user", "admin")

	var req struct {
		Reason string `json:"reason"`
	}
	c.ShouldBindJSON(&req)

	var request models.WorkerApprovalRequest
	if err := database.DB.First(&request, "id = ?", requestID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Request not found"})
		return
	}

	if request.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request is not pending"})
		return
	}

	now := time.Now()
	request.Status = "rejected"
	request.RejectedBy = &adminUser
	request.RejectedAt = &now
	request.RejectReason = &req.Reason
	database.DB.Save(&request)

	c.JSON(http.StatusOK, gin.H{"message": "Request rejected"})
}

// ==================== Admin: Camera Assignment ====================

// AssignCamerasRequest - Request body for camera assignment
type AssignCamerasRequest struct {
	Assignments []struct {
		DeviceID   string   `json:"device_id" binding:"required"`
		Analytics  []string `json:"analytics" binding:"required"`
		FPS        int      `json:"fps"`
		Resolution string   `json:"resolution"`
	} `json:"assignments" binding:"required"`
}

// AssignCameras assigns cameras to a worker (admin)
// POST /api/admin/workers/:id/cameras
func AssignCameras(c *gin.Context) {
	workerID := c.Param("id")

	var worker models.Worker
	if err := database.DB.First(&worker, "id = ?", workerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Worker not found"})
		return
	}

	var req AssignCamerasRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Start transaction
	tx := database.DB.Begin()

	// Deactivate existing assignments
	tx.Model(&models.WorkerCameraAssignment{}).Where("worker_id = ?", workerID).Update("is_active", false)

	// Create/update assignments
	for _, a := range req.Assignments {
		// Verify device exists
		var device models.Device
		if err := tx.First(&device, "id = ?", a.DeviceID).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusBadRequest, gin.H{"error": "Device not found: " + a.DeviceID})
			return
		}

		fps := a.FPS
		if fps == 0 {
			fps = 15
		}
		resolution := a.Resolution
		if resolution == "" {
			resolution = "720p"
		}

		// Check if assignment exists
		var existing models.WorkerCameraAssignment
		err := tx.Where("worker_id = ? AND device_id = ?", workerID, a.DeviceID).First(&existing).Error
		
		if err == gorm.ErrRecordNotFound {
			// Create new
			assignment := models.WorkerCameraAssignment{
				WorkerID:   workerID,
				DeviceID:   a.DeviceID,
				Analytics:  models.NewJSONB(a.Analytics),
				FPS:        fps,
				Resolution: resolution,
				IsActive:   true,
			}
			tx.Create(&assignment)
		} else {
			// Update existing
			existing.Analytics = models.NewJSONB(a.Analytics)
			existing.FPS = fps
			existing.Resolution = resolution
			existing.IsActive = true
			tx.Save(&existing)
		}

		// Update device's worker_id
		tx.Model(&device).Update("worker_id", workerID)
	}

	// Increment config version
	tx.Model(&worker).Update("config_version", gorm.Expr("config_version + 1"))

	tx.Commit()

	// Return updated worker with assignments
	database.DB.Preload("CameraAssignments").Preload("CameraAssignments.Device").First(&worker, "id = ?", workerID)
	c.JSON(http.StatusOK, worker)
}

// GetWorkerCameras returns cameras assigned to a worker
// GET /api/admin/workers/:id/cameras
func GetWorkerCameras(c *gin.Context) {
	workerID := c.Param("id")

	var assignments []models.WorkerCameraAssignment
	database.DB.Preload("Device").Where("worker_id = ? AND is_active = true", workerID).Find(&assignments)

	c.JSON(http.StatusOK, assignments)
}

// UnassignCamera removes a camera from a worker
// DELETE /api/admin/workers/:id/cameras/:deviceId
func UnassignCamera(c *gin.Context) {
	workerID := c.Param("id")
	deviceID := c.Param("deviceId")

	result := database.DB.Model(&models.WorkerCameraAssignment{}).
		Where("worker_id = ? AND device_id = ?", workerID, deviceID).
		Update("is_active", false)

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Assignment not found"})
		return
	}

	// Clear device's worker_id
	database.DB.Model(&models.Device{}).Where("id = ?", deviceID).Update("worker_id", nil)

	// Increment config version
	database.DB.Model(&models.Worker{}).Where("id = ?", workerID).Update("config_version", gorm.Expr("config_version + 1"))

	c.JSON(http.StatusOK, gin.H{"message": "Camera unassigned"})
}
