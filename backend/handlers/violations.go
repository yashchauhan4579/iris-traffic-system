package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/irisdrone/backend/database"
	"github.com/irisdrone/backend/models"
	"gorm.io/gorm"
)

// PostViolation handles POST /api/violations - Ingest violation from edge worker
func PostViolation(c *gin.Context) {
	var req struct {
		DeviceID       string                 `json:"deviceId" binding:"required"`
		ViolationType  models.ViolationType   `json:"violationType" binding:"required"`
		DetectionMethod models.DetectionMethod `json:"detectionMethod"`
		PlateNumber    *string                `json:"plateNumber"`
		PlateConfidence *float64              `json:"plateConfidence"`
		PlateImageURL  *string                `json:"plateImageUrl"`
		FullSnapshotURL *string               `json:"fullSnapshotUrl"`
		FrameID        *string                `json:"frameId"`
		DetectedSpeed  *float64               `json:"detectedSpeed"`
		SpeedLimit2W   *float64               `json:"speedLimit2W"`
		SpeedLimit4W   *float64               `json:"speedLimit4W"`
		SpeedOverLimit *float64               `json:"speedOverLimit"`
		Confidence     *float64               `json:"confidence"`
		Metadata       models.JSONB           `json:"metadata"`
		Timestamp      *string                `json:"timestamp"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Upsert device - create if not exists
	device := models.Device{
		ID:     req.DeviceID,
		Type:   models.DeviceTypeCamera, // Default to camera
		Status: "active",
	}

	// Try to extract lat/lng from metadata if available
	if req.Metadata.Data != nil {
		if dataMap, ok := req.Metadata.Data.(map[string]interface{}); ok {
			if lat, ok := dataMap["lat"].(float64); ok {
				device.Lat = lat
			}
			if lng, ok := dataMap["lng"].(float64); ok {
				device.Lng = lng
			}
		}
	}

	// Set default name if not provided
	if device.Name == nil {
		name := "Camera " + req.DeviceID
		device.Name = &name
	}

	// Create or update device
	if err := database.DB.Where("id = ?", req.DeviceID).
		Assign(models.Device{
			UpdatedAt: time.Now(),
		}).
		FirstOrCreate(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upsert device"})
		return
	}

	// Try to link to vehicle if plate number is provided
	var vehicleID *int64
	if req.PlateNumber != nil && *req.PlateNumber != "" {
		var vehicle models.Vehicle
		err := database.DB.Where("plate_number = ?", *req.PlateNumber).First(&vehicle).Error
		if err == nil {
			vehicleID = &vehicle.ID
		}
	}

	detectionMethod := req.DetectionMethod
	if detectionMethod == "" {
		detectionMethod = models.DetectionAIVision
	}

	timestamp := time.Now()
	if req.Timestamp != nil {
		if parsed, err := time.Parse(time.RFC3339, *req.Timestamp); err == nil {
			timestamp = parsed
		}
	}

	violation := models.TrafficViolation{
		DeviceID:        req.DeviceID,
		VehicleID:      vehicleID, // Link to vehicle if found
		ViolationType:   req.ViolationType,
		Status:          models.ViolationPending,
		DetectionMethod: detectionMethod,
		PlateNumber:     req.PlateNumber,
		PlateConfidence: req.PlateConfidence,
		PlateImageURL:   req.PlateImageURL,
		FullSnapshotURL: req.FullSnapshotURL,
		FrameID:         req.FrameID,
		DetectedSpeed:   req.DetectedSpeed,
		SpeedLimit2W:    req.SpeedLimit2W,
		SpeedLimit4W:    req.SpeedLimit4W,
		SpeedOverLimit:  req.SpeedOverLimit,
		Confidence:      req.Confidence,
		Metadata:        req.Metadata,
		Timestamp:       timestamp,
	}

	if err := database.DB.Create(&violation).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create violation"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "id": strconv.FormatInt(violation.ID, 10)})
}

// GetViolations handles GET /api/violations - List violations with filters
func GetViolations(c *gin.Context) {
	query := database.DB.Model(&models.TrafficViolation{})

	// Filter by status
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	// Filter by violation type
	if violationType := c.Query("violationType"); violationType != "" {
		query = query.Where("violation_type = ?", violationType)
	}

	// Filter by device
	if deviceID := c.Query("deviceId"); deviceID != "" {
		query = query.Where("device_id = ?", deviceID)
	}

	// Filter by plate number
	if plateNumber := c.Query("plateNumber"); plateNumber != "" {
		query = query.Where("plate_number ILIKE ?", "%"+plateNumber+"%")
	}

	// Filter by date range
	if startTime := c.Query("startTime"); startTime != "" {
		if parsed, err := time.Parse(time.RFC3339, startTime); err == nil {
			query = query.Where("timestamp >= ?", parsed)
		}
	}
	if endTime := c.Query("endTime"); endTime != "" {
		if parsed, err := time.Parse(time.RFC3339, endTime); err == nil {
			query = query.Where("timestamp <= ?", parsed)
		}
	}

	// Pagination
	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}
	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	var violations []models.TrafficViolation
	var total int64

	// Get total count
	query.Model(&models.TrafficViolation{}).Count(&total)

	// Get violations
	if err := query.Preload("Device", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, name, lat, lng, type")
	}).Order("timestamp DESC").Limit(limit).Offset(offset).Find(&violations).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch violations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"violations": violations,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
	})
}

// GetViolation handles GET /api/violations/:id - Get single violation
func GetViolation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid violation ID"})
		return
	}

	var violation models.TrafficViolation
	if err := database.DB.Preload("Device").First(&violation, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Violation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch violation"})
		return
	}

	c.JSON(http.StatusOK, violation)
}

// ApproveViolation handles PATCH /api/violations/:id/approve
func ApproveViolation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid violation ID"})
		return
	}

	var req struct {
		ReviewNote *string `json:"reviewNote"`
		ReviewedBy *string `json:"reviewedBy"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		// Optional body, continue without it
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":      models.ViolationApproved,
		"reviewed_at": now,
	}
	if req.ReviewNote != nil {
		updates["review_note"] = *req.ReviewNote
	}
	if req.ReviewedBy != nil {
		updates["reviewed_by"] = *req.ReviewedBy
	}

	if err := database.DB.Model(&models.TrafficViolation{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Violation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve violation"})
		return
	}

	var violation models.TrafficViolation
	database.DB.First(&violation, id)
	c.JSON(http.StatusOK, violation)
}

// RejectViolation handles PATCH /api/violations/:id/reject
func RejectViolation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid violation ID"})
		return
	}

	var req struct {
		RejectionReason string  `json:"rejectionReason" binding:"required"`
		ReviewedBy      *string `json:"reviewedBy"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rejectionReason is required"})
		return
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":           models.ViolationRejected,
		"reviewed_at":      now,
		"rejection_reason": req.RejectionReason,
	}
	if req.ReviewedBy != nil {
		updates["reviewed_by"] = *req.ReviewedBy
	}

	if err := database.DB.Model(&models.TrafficViolation{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Violation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reject violation"})
		return
	}

	var violation models.TrafficViolation
	database.DB.First(&violation, id)
	c.JSON(http.StatusOK, violation)
}

// UpdateViolationPlate handles PATCH /api/violations/:id/plate - Update plate number
func UpdateViolationPlate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid violation ID"})
		return
	}

	var req struct {
		PlateNumber string `json:"plateNumber" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "plateNumber is required"})
		return
	}

	if err := database.DB.Model(&models.TrafficViolation{}).Where("id = ?", id).Update("plate_number", req.PlateNumber).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Violation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update plate number"})
		return
	}

	var violation models.TrafficViolation
	database.DB.First(&violation, id)
	c.JSON(http.StatusOK, violation)
}

// GetViolationStats handles GET /api/violations/stats - Get violation statistics
func GetViolationStats(c *gin.Context) {
	var stats struct {
		Total       int64 `json:"total"`
		Pending     int64 `json:"pending"`
		Approved    int64 `json:"approved"`
		Rejected    int64 `json:"rejected"`
		Fined       int64 `json:"fined"`
		ByType      map[string]int64 `json:"byType"`
		ByDevice    map[string]int64 `json:"byDevice"`
	}

	stats.ByType = make(map[string]int64)
	stats.ByDevice = make(map[string]int64)

	// Get counts by status
	database.DB.Model(&models.TrafficViolation{}).Count(&stats.Total)
	database.DB.Model(&models.TrafficViolation{}).Where("status = ?", models.ViolationPending).Count(&stats.Pending)
	database.DB.Model(&models.TrafficViolation{}).Where("status = ?", models.ViolationApproved).Count(&stats.Approved)
	database.DB.Model(&models.TrafficViolation{}).Where("status = ?", models.ViolationRejected).Count(&stats.Rejected)
	database.DB.Model(&models.TrafficViolation{}).Where("status = ?", models.ViolationFined).Count(&stats.Fined)

	// Get counts by type
	var typeCounts []struct {
		ViolationType string
		Count         int64
	}
	database.DB.Model(&models.TrafficViolation{}).
		Select("violation_type, COUNT(*) as count").
		Group("violation_type").
		Scan(&typeCounts)
	
	for _, tc := range typeCounts {
		stats.ByType[tc.ViolationType] = tc.Count
	}

	// Get counts by device
	var deviceCounts []struct {
		DeviceID string
		Count    int64
	}
	database.DB.Model(&models.TrafficViolation{}).
		Select("device_id, COUNT(*) as count").
		Group("device_id").
		Scan(&deviceCounts)
	
	for _, dc := range deviceCounts {
		stats.ByDevice[dc.DeviceID] = dc.Count
	}

	c.JSON(http.StatusOK, stats)
}

