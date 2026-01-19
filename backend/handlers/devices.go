package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/irisdrone/backend/database"
	"github.com/irisdrone/backend/models"
	"gorm.io/gorm"
)

// GetDevices handles GET /api/devices
func GetDevices(c *gin.Context) {
	var devices []models.Device
	query := database.DB

	// Filter by type
	if deviceType := c.Query("type"); deviceType != "" {
		query = query.Where("type = ?", deviceType)
	}

	// Filter by zone
	if zoneID := c.Query("zone"); zoneID != "" {
		query = query.Where("zone_id = ?", zoneID)
	}

	// Minimal mode - return only essential fields
	if minimal := c.Query("minimal"); minimal == "true" {
		var devices []models.Device
		if err := query.Select("id, name, type, lat, lng, status").
			Order("id ASC").
			Find(&devices).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch devices"})
			return
		}

		// Return slim response
		type MinimalDevice struct {
			ID     string            `json:"id"`
			Name   *string           `json:"name"`
			Type   models.DeviceType `json:"type"`
			Lat    float64           `json:"lat"`
			Lng    float64           `json:"lng"`
			Status string            `json:"status"`
		}
		result := make([]MinimalDevice, len(devices))
		for i, d := range devices {
			result[i] = MinimalDevice{
				ID:     d.ID,
				Name:   d.Name,
				Type:   d.Type,
				Lat:    d.Lat,
				Lng:    d.Lng,
				Status: d.Status,
			}
		}
		c.JSON(http.StatusOK, result)
		return
	}

	// Full mode - include latest event
	if err := query.Preload("Events", func(db *gorm.DB) *gorm.DB {
		return db.Order("timestamp DESC").Limit(1)
	}).Order("id ASC").Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch devices"})
		return
	}

	c.JSON(http.StatusOK, devices)
}

// GetDeviceLatest handles GET /api/devices/:id/latest
func GetDeviceLatest(c *gin.Context) {
	deviceID := c.Param("id")

	var event models.Event
	if err := database.DB.Where("device_id = ?", deviceID).
		Order("timestamp DESC").
		First(&event).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "No events found for device"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch latest event"})
		return
	}

	c.JSON(http.StatusOK, event)
}

// GetDeviceSurges handles GET /api/devices/analytics/surges
func GetDeviceSurges(c *gin.Context) {
	type SurgeEvent struct {
		ID        int64   `json:"id"`
		DeviceID  string  `json:"device_id"`
		Timestamp string  `json:"timestamp"`
		Type      string  `json:"type"`
		Data      string  `json:"data"`
		RiskLevel *string `json:"risk_level"`
		Name      *string `json:"name"`
		Lat       float64 `json:"lat"`
		Lng       float64 `json:"lng"`
		ZoneID    *string `json:"zone_id"`
	}

	var results []SurgeEvent
	query := `
		SELECT DISTINCT ON (e.device_id) 
			e.id, e.device_id, e.timestamp, e.type, e.data::text, e.risk_level,
			d.name, d.lat, d.lng, d.zone_id
		FROM events e
		JOIN devices d ON e.device_id = d.id
		WHERE e.risk_level IN ('high', 'critical')
		AND e.timestamp > NOW() - INTERVAL '5 minutes'
		ORDER BY e.device_id, e.timestamp DESC
	`

	if err := database.DB.Raw(query).Scan(&results).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch surge data"})
		return
	}

	c.JSON(http.StatusOK, results)
}

