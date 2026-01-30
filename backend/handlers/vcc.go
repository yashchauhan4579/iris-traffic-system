package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/irisdrone/backend/database"
	"github.com/irisdrone/backend/models"
)

// GetVCCStats handles GET /api/vcc/stats - Vehicle Classification and Counting statistics
func GetVCCStats(c *gin.Context) {
	// Parse time range
	startTime := time.Now().AddDate(0, 0, -7) // Default: last 7 days
	endTime := time.Now()

	if startTimeStr := c.Query("startTime"); startTimeStr != "" {
		if parsed, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			startTime = parsed
		}
	}
	}
	if endTimeStr := c.Query("endTime"); endTimeStr != "" {
		if parsed, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			endTime = parsed
		}
	}

	location := c.Query("location")

	// Group by time period
	groupBy := c.DefaultQuery("groupBy", "hour") // hour, day, week, month

	var stats struct {
		TotalDetections   int64                        `json:"totalDetections"`
		UniqueVehicles    int64                        `json:"uniqueVehicles"`
		ByVehicleType    map[string]int64             `json:"byVehicleType"`
		ByTime           []map[string]interface{}     `json:"byTime"`
		ByDevice         []map[string]interface{}      `json:"byDevice"`
		ByHour           map[int]int64                `json:"byHour"`      // 0-23 hour distribution
		ByDayOfWeek      map[string]int64              `json:"byDayOfWeek"` // Mon-Sun
		PeakHour         int                           `json:"peakHour"`
		PeakDay          string                        `json:"peakDay"`
		AveragePerHour   float64                       `json:"averagePerHour"`
		Classification   map[string]interface{}        `json:"classification"`
	}

	stats.ByVehicleType = make(map[string]int64)
	stats.ByHour = make(map[int]int64)
	stats.ByDayOfWeek = make(map[string]int64)

	// Total detections in time range
	totalQuery := database.DB.Model(&models.VehicleDetection{}).
		Where("timestamp >= ? AND timestamp <= ?", startTime, endTime)
	if location != "" {
		totalQuery = totalQuery.Joins("JOIN devices ON vehicle_detections.device_id = devices.id").
			Where("devices.metadata->>'location' = ?", location)
	}
	totalQuery.Count(&stats.TotalDetections)

	// Unique vehicles detected
	uniqueQuery := database.DB.Model(&models.VehicleDetection{}).
		Where("timestamp >= ? AND timestamp <= ? AND vehicle_id IS NOT NULL", startTime, endTime)
	if location != "" {
		uniqueQuery = uniqueQuery.Joins("JOIN devices ON vehicle_detections.device_id = devices.id").
			Where("devices.metadata->>'location' = ?", location)
	}
	uniqueQuery.Distinct("vehicle_id").Count(&stats.UniqueVehicles)

	// Count by vehicle type
	var typeCounts []struct {
		VehicleType string
		Count       int64
	}
	typeQuery := database.DB.Model(&models.VehicleDetection{}).
		Select("vehicle_type, COUNT(*) as count").
		Where("timestamp >= ? AND timestamp <= ?", startTime, endTime)
	if location != "" {
		typeQuery = typeQuery.Joins("JOIN devices ON vehicle_detections.device_id = devices.id").
			Where("devices.metadata->>'location' = ?", location)
	}
	typeQuery.Group("vehicle_type").Scan(&typeCounts)

	for _, tc := range typeCounts {
		stats.ByVehicleType[tc.VehicleType] = tc.Count
	}

	// Count by time period (hourly, daily, etc.)
	var timeTrunc string
	var timeLabel string
	var timeFormat string
	switch groupBy {
	case "minute":
		timeTrunc = "minute"
		timeLabel = "minute"
		timeFormat = "YYYY-MM-DD HH24:MI"
	case "hour":
		timeTrunc = "hour"
		timeLabel = "hour"
		timeFormat = "YYYY-MM-DD HH24:00"
	case "day":
		timeTrunc = "day"
		timeLabel = "day"
		timeFormat = "YYYY-MM-DD"
	case "week":
		timeTrunc = "week"
		timeLabel = "week"
		timeFormat = "IYYY-\"W\"IW"
	case "month":
		timeTrunc = "month"
		timeLabel = "month"
		timeFormat = "YYYY-MM"
	default:
		timeTrunc = "hour"
		timeLabel = "hour"
		timeFormat = "YYYY-MM-DD HH24:00"
	}

	var timeCounts []struct {
		TimePeriod string
		Count      int64
		Count2W    int64
		Count4W    int64
		CountAuto  int64
		CountBus   int64
		CountTruck int64
		CountHMV   int64
	}
	
	// PostgreSQL: Use DATE_TRUNC for grouping, then format for display
	// This is safer than using TO_CHAR with parameters
	var rawQuery string
	var args []interface{}

	selectClause := fmt.Sprintf(`
		SELECT TO_CHAR(DATE_TRUNC('%s', T.timestamp), '%s') as time_period, 
		COUNT(T.*) as count,
		SUM(CASE WHEN T.vehicle_type = '2W' THEN 1 ELSE 0 END) as count2_w,
		SUM(CASE WHEN T.vehicle_type = '4W' THEN 1 ELSE 0 END) as count4_w,
		SUM(CASE WHEN T.vehicle_type IN ('AUTO', '3W') THEN 1 ELSE 0 END) as count_auto,
		SUM(CASE WHEN T.vehicle_type = 'BUS' THEN 1 ELSE 0 END) as count_bus,
		SUM(CASE WHEN T.vehicle_type = 'TRUCK' THEN 1 ELSE 0 END) as count_truck,
		SUM(CASE WHEN T.vehicle_type = 'HMV' THEN 1 ELSE 0 END) as count_hmv
	`, timeTrunc, timeFormat)

	if location != "" {
		rawQuery = fmt.Sprintf(`
			%s
			FROM vehicle_detections T
			JOIN devices ON T.device_id = devices.id
			WHERE T.timestamp >= ? AND T.timestamp <= ?
			AND devices.metadata->>'location' = ?
			GROUP BY DATE_TRUNC('%s', T.timestamp)
			ORDER BY DATE_TRUNC('%s', T.timestamp)
		`, selectClause, timeTrunc, timeTrunc)
		args = []interface{}{startTime, endTime, location}
	} else {
		rawQuery = fmt.Sprintf(`
			%s
			FROM vehicle_detections T
			WHERE T.timestamp >= ? AND T.timestamp <= ?
			GROUP BY DATE_TRUNC('%s', T.timestamp)
			ORDER BY DATE_TRUNC('%s', T.timestamp)
		`, selectClause, timeTrunc, timeTrunc)
		args = []interface{}{startTime, endTime}
	}
	
	database.DB.Raw(rawQuery, args...).Scan(&timeCounts)

	stats.ByTime = make([]map[string]interface{}, len(timeCounts))
	for i, tc := range timeCounts {
		stats.ByTime[i] = map[string]interface{}{
			timeLabel: tc.TimePeriod,
			"count":   tc.Count,
			"2W":      tc.Count2W,
			"4W":      tc.Count4W,
			"AUTO":    tc.CountAuto,
			"BUS":     tc.CountBus,
			"TRUCK":   tc.CountTruck,
			"HMV":     tc.CountHMV,
		}
	}

	// Count by device and vehicle type
	var deviceTypeCounts []struct {
		DeviceID    string
		DeviceName  string
		VehicleType string
		Count       int64
	}
	
	dtQuery := database.DB.Model(&models.VehicleDetection{}).
		Select("vehicle_detections.device_id, devices.name as device_name, vehicle_type, COUNT(*) as count").
		Joins("LEFT JOIN devices ON vehicle_detections.device_id = devices.id").
		Where("vehicle_detections.timestamp >= ? AND vehicle_detections.timestamp <= ?", startTime, endTime)
	
	if location != "" {
		dtQuery = dtQuery.Where("devices.metadata->>'location' = ?", location)
	}

	dtQuery.Group("vehicle_detections.device_id, devices.name, vehicle_type").
		Scan(&deviceTypeCounts)

	// Aggregate by device
	deviceMap := make(map[string]map[string]interface{})
	for _, dtc := range deviceTypeCounts {
		if _, ok := deviceMap[dtc.DeviceID]; !ok {
			deviceMap[dtc.DeviceID] = map[string]interface{}{
				"deviceId":        dtc.DeviceID,
				"deviceName":      dtc.DeviceName,
				"totalDetections": int64(0),
				"byType":         make(map[string]int64),
			}
		}
		entry := deviceMap[dtc.DeviceID]
		entry["totalDetections"] = entry["totalDetections"].(int64) + dtc.Count
		entry["byType"].(map[string]int64)[dtc.VehicleType] = dtc.Count
	}

	// Convert map to slice
	stats.ByDevice = make([]map[string]interface{}, 0, len(deviceMap))
	for _, dev := range deviceMap {
		stats.ByDevice = append(stats.ByDevice, dev)
	}

	// Sort by total detections desc
	// Basic bubble sort or similar since slice is small, or just leave unsorted if frontend handles it?
	// The original code used Order("count DESC"). Let's sort it here to match behavior.
	// Importing sort package would require modifying imports.
	// Instead, let's just let the frontend sort it or do a simple bubble sort here since it's cleaner.
	for i := 0; i < len(stats.ByDevice)-1; i++ {
		for j := 0; j < len(stats.ByDevice)-i-1; j++ {
			if stats.ByDevice[j]["totalDetections"].(int64) < stats.ByDevice[j+1]["totalDetections"].(int64) {
				stats.ByDevice[j], stats.ByDevice[j+1] = stats.ByDevice[j+1], stats.ByDevice[j]
			}
		}
	}

	// Hourly distribution (0-23)
	var hourCounts []struct {
		Hour  int
		Count int64
	}
	var hourQuery string
	var hourArgs []interface{}

	if location != "" {
		hourQuery = `
			SELECT EXTRACT(HOUR FROM vehicle_detections.timestamp)::int as hour, COUNT(*) as count
			FROM vehicle_detections
			JOIN devices ON vehicle_detections.device_id = devices.id
			WHERE vehicle_detections.timestamp >= ? AND vehicle_detections.timestamp <= ?
			AND devices.metadata->>'location' = ?
			GROUP BY EXTRACT(HOUR FROM vehicle_detections.timestamp)
			ORDER BY hour
		`
		hourArgs = []interface{}{startTime, endTime, location}
	} else {
		hourQuery = `
			SELECT EXTRACT(HOUR FROM timestamp)::int as hour, COUNT(*) as count
			FROM vehicle_detections
			WHERE timestamp >= ? AND timestamp <= ?
			GROUP BY EXTRACT(HOUR FROM timestamp)
			ORDER BY hour
		`
		hourArgs = []interface{}{startTime, endTime}
	}
	database.DB.Raw(hourQuery, hourArgs...).Scan(&hourCounts)

	for _, hc := range hourCounts {
		stats.ByHour[int(hc.Hour)] = hc.Count
	}

	// Find peak hour
	maxHourCount := int64(0)
	for hour, count := range stats.ByHour {
		if count > maxHourCount {
			maxHourCount = count
			stats.PeakHour = hour
		}
	}

	// Day of week distribution
	var dayCounts []struct {
		DayOfWeek string
		Count     int64
	}

	var dayQuery string
	var dayArgs []interface{}

	if location != "" {
		dayQuery = `
			SELECT TO_CHAR(vehicle_detections.timestamp, 'Day') as day_of_week, COUNT(*) as count
			FROM vehicle_detections
			JOIN devices ON vehicle_detections.device_id = devices.id
			WHERE vehicle_detections.timestamp >= ? AND vehicle_detections.timestamp <= ?
			AND devices.metadata->>'location' = ?
			GROUP BY TO_CHAR(vehicle_detections.timestamp, 'Day')
			ORDER BY count DESC
		`
		dayArgs = []interface{}{startTime, endTime, location}
	} else {
		dayQuery = `
			SELECT TO_CHAR(timestamp, 'Day') as day_of_week, COUNT(*) as count
			FROM vehicle_detections
			WHERE timestamp >= ? AND timestamp <= ?
			GROUP BY TO_CHAR(timestamp, 'Day')
			ORDER BY count DESC
		`
		dayArgs = []interface{}{startTime, endTime}
	}
	database.DB.Raw(dayQuery, dayArgs...).Scan(&dayCounts)

	for _, dc := range dayCounts {
		dayName := strings.TrimSpace(dc.DayOfWeek)
		stats.ByDayOfWeek[dayName] = dc.Count
	}

	// Find peak day
	maxDayCount := int64(0)
	for day, count := range stats.ByDayOfWeek {
		if count > maxDayCount {
			maxDayCount = count
			stats.PeakDay = day
		}
	}

	// Calculate average per hour
	hoursDiff := endTime.Sub(startTime).Hours()
	if hoursDiff > 0 {
		stats.AveragePerHour = float64(stats.TotalDetections) / hoursDiff
	}

	// Classification breakdown
	stats.Classification = map[string]interface{}{
		"withPlates": 0,
		"withoutPlates": 0,
		"withMakeModel": 0,
		"plateOnly": 0,
		"fullClassification": 0,
	}

	var withPlates, withoutPlates, withMakeModel int64
	
	wpQuery := database.DB.Model(&models.VehicleDetection{}).
		Where("timestamp >= ? AND timestamp <= ? AND plate_detected = ?", startTime, endTime, true)
	if location != "" {
		wpQuery = wpQuery.Joins("JOIN devices ON vehicle_detections.device_id = devices.id").
			Where("devices.metadata->>'location' = ?", location)
	}
	wpQuery.Count(&withPlates)
	
	wopQuery := database.DB.Model(&models.VehicleDetection{}).
		Where("timestamp >= ? AND timestamp <= ? AND plate_detected = ?", startTime, endTime, false)
	if location != "" {
		wopQuery = wopQuery.Joins("JOIN devices ON vehicle_detections.device_id = devices.id").
			Where("devices.metadata->>'location' = ?", location)
	}
	wopQuery.Count(&withoutPlates)

	wmmQuery := database.DB.Model(&models.VehicleDetection{}).
		Where("timestamp >= ? AND timestamp <= ? AND make_model_detected = ?", startTime, endTime, true)
	if location != "" {
		wmmQuery = wmmQuery.Joins("JOIN devices ON vehicle_detections.device_id = devices.id").
			Where("devices.metadata->>'location' = ?", location)
	}
	wmmQuery.Count(&withMakeModel)

	stats.Classification["withPlates"] = withPlates
	stats.Classification["withoutPlates"] = withoutPlates
	stats.Classification["withMakeModel"] = withMakeModel
	stats.Classification["plateOnly"] = withPlates - withMakeModel
	stats.Classification["fullClassification"] = withMakeModel

	// Direction distribution (Added)
	stats.Classification["byDirection"] = make(map[string]int64)
	var directionCounts []struct {
		Direction string
		Count     int64
	}
	dirQuery := database.DB.Model(&models.VehicleDetection{}).
		Select("direction, COUNT(*) as count").
		Where("timestamp >= ? AND timestamp <= ?", startTime, endTime)
	if location != "" {
		dirQuery = dirQuery.Joins("JOIN devices ON vehicle_detections.device_id = devices.id").
			Where("devices.metadata->>'location' = ?", location)
	}
	dirQuery.Group("direction").Scan(&directionCounts)

	byDirection := make(map[string]int64)
	for _, dc := range directionCounts {
		dir := "Unknown"
		if dc.Direction != "" {
			dir = dc.Direction
		}
		byDirection[dir] = dc.Count
	}
	stats.Classification["byDirection"] = byDirection

	c.JSON(http.StatusOK, stats)
}

// GetVCCByDevice handles GET /api/vcc/device/:deviceId - VCC stats for specific device
func GetVCCByDevice(c *gin.Context) {
	deviceID := c.Param("deviceId")

	// Parse time range
	startTime := time.Now().AddDate(0, 0, -1) // Default: last 24 hours
	endTime := time.Now()

	if startTimeStr := c.Query("startTime"); startTimeStr != "" {
		if parsed, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			startTime = parsed
		}
	}
	if endTimeStr := c.Query("endTime"); endTimeStr != "" {
		if parsed, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			endTime = parsed
		}
	}

	// Group by time period
	groupBy := c.DefaultQuery("groupBy", "hour")

	var stats struct {
		DeviceID        string                `json:"deviceId"`
		DeviceName      string                `json:"deviceName"`
		TotalDetections int64                 `json:"totalDetections"`
		UniqueVehicles  int64                 `json:"uniqueVehicles"`
		ByVehicleType   map[string]int64      `json:"byVehicleType"`
		ByTime          []map[string]interface{} `json:"byTime"`
		ByHour          map[int]int64         `json:"byHour"`
		ByDayOfWeek     map[string]int64      `json:"byDayOfWeek"`
		PeakHour        int                   `json:"peakHour"`
		AveragePerHour  float64               `json:"averagePerHour"`
		Classification  map[string]interface{} `json:"classification"`
	}

	stats.ByVehicleType = make(map[string]int64)
	stats.ByHour = make(map[int]int64)

	// Get device info
	var device models.Device
	if err := database.DB.First(&device, "id = ?", deviceID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Device not found"})
		return
	}

	stats.DeviceID = device.ID
	if device.Name != nil {
		stats.DeviceName = *device.Name
	}

	// Count by time period (trend)
	var timeTrunc string
	var timeLabel string
	var timeFormat string
	switch groupBy {
	case "minute":
		timeTrunc = "minute"
		timeLabel = "minute"
		timeFormat = "YYYY-MM-DD HH24:MI"
	case "hour":
		timeTrunc = "hour"
		timeLabel = "hour"
		timeFormat = "YYYY-MM-DD HH24:00"
	case "day":
		timeTrunc = "day"
		timeLabel = "day"
		timeFormat = "YYYY-MM-DD"
	case "week":
		timeTrunc = "week"
		timeLabel = "week"
		timeFormat = "IYYY-\"W\"IW"
	case "month":
		timeTrunc = "month"
		timeLabel = "month"
		timeFormat = "YYYY-MM"
	default:
		timeTrunc = "hour"
		timeLabel = "hour"
		timeFormat = "YYYY-MM-DD HH24:00"
	}

	var timeCounts []struct {
		TimePeriod string
		Count      int64
		Count2W    int64
		Count4W    int64
		CountAuto  int64
		CountBus   int64
		CountTruck int64
		CountHMV   int64
	}
	
	query := fmt.Sprintf(`
		SELECT TO_CHAR(DATE_TRUNC('%s', timestamp), '%s') as time_period, 
		COUNT(*) as count,
		SUM(CASE WHEN vehicle_type = '2W' THEN 1 ELSE 0 END) as count2_w,
		SUM(CASE WHEN vehicle_type = '4W' THEN 1 ELSE 0 END) as count4_w,
		SUM(CASE WHEN vehicle_type IN ('AUTO', '3W') THEN 1 ELSE 0 END) as count_auto,
		SUM(CASE WHEN vehicle_type = 'BUS' THEN 1 ELSE 0 END) as count_bus,
		SUM(CASE WHEN vehicle_type = 'TRUCK' THEN 1 ELSE 0 END) as count_truck,
		SUM(CASE WHEN vehicle_type = 'HMV' THEN 1 ELSE 0 END) as count_hmv
		FROM vehicle_detections
		WHERE device_id = $1 AND timestamp >= $2 AND timestamp <= $3
		GROUP BY DATE_TRUNC('%s', timestamp)
		ORDER BY DATE_TRUNC('%s', timestamp)
	`, timeTrunc, timeFormat, timeTrunc, timeTrunc)
	
	database.DB.Raw(query, deviceID, startTime, endTime).Scan(&timeCounts)

	stats.ByTime = make([]map[string]interface{}, len(timeCounts))
	for i, tc := range timeCounts {
		stats.ByTime[i] = map[string]interface{}{
			timeLabel: tc.TimePeriod,
			"count":   tc.Count,
			"2W":      tc.Count2W,
			"4W":      tc.Count4W,
			"AUTO":    tc.CountAuto,
			"BUS":     tc.CountBus,
			"TRUCK":   tc.CountTruck,
			"HMV":     tc.CountHMV,
		}
	}

	// Total detections
	database.DB.Model(&models.VehicleDetection{}).
		Where("device_id = ? AND timestamp >= ? AND timestamp <= ?", deviceID, startTime, endTime).
		Count(&stats.TotalDetections)

	// Unique vehicles
	database.DB.Model(&models.VehicleDetection{}).
		Where("device_id = ? AND timestamp >= ? AND timestamp <= ? AND vehicle_id IS NOT NULL", deviceID, startTime, endTime).
		Distinct("vehicle_id").
		Count(&stats.UniqueVehicles)

	// By vehicle type
	var typeCounts []struct {
		VehicleType string
		Count       int64
	}
	database.DB.Model(&models.VehicleDetection{}).
		Select("vehicle_type, COUNT(*) as count").
		Where("device_id = ? AND timestamp >= ? AND timestamp <= ?", deviceID, startTime, endTime).
		Group("vehicle_type").
		Scan(&typeCounts)

	for _, tc := range typeCounts {
		stats.ByVehicleType[tc.VehicleType] = tc.Count
	}

	// Hourly distribution
	var hourCounts []struct {
		Hour  int
		Count int64
	}
	database.DB.Raw(`
		SELECT EXTRACT(HOUR FROM timestamp)::int as hour, COUNT(*) as count
		FROM vehicle_detections
		WHERE device_id = ? AND timestamp >= ? AND timestamp <= ?
		GROUP BY EXTRACT(HOUR FROM timestamp)
		ORDER BY hour
	`, deviceID, startTime, endTime).Scan(&hourCounts)

	for _, hc := range hourCounts {
		stats.ByHour[int(hc.Hour)] = hc.Count
	}

	// Peak hour
	maxHourCount := int64(0)
	for hour, count := range stats.ByHour {
		if count > maxHourCount {
			maxHourCount = count
			stats.PeakHour = hour
		}
	}
	
	// Day of week distribution (Added)
	stats.ByDayOfWeek = make(map[string]int64)
	var dayCounts []struct {
		DayOfWeek string
		Count     int64
	}
	database.DB.Raw(`
		SELECT TO_CHAR(timestamp, 'Day') as day_of_week, COUNT(*) as count
		FROM vehicle_detections
		WHERE device_id = ? AND timestamp >= ? AND timestamp <= ?
		GROUP BY TO_CHAR(timestamp, 'Day')
		ORDER BY count DESC
	`, deviceID, startTime, endTime).Scan(&dayCounts)

	for _, dc := range dayCounts {
		dayName := strings.TrimSpace(dc.DayOfWeek)
		stats.ByDayOfWeek[dayName] = dc.Count
	}

	// Average per hour
	hoursDiff := endTime.Sub(startTime).Hours()
	if hoursDiff > 0 {
		stats.AveragePerHour = float64(stats.TotalDetections) / hoursDiff
	}

	// Classification
	var withPlates, withMakeModel int64
	database.DB.Model(&models.VehicleDetection{}).
		Where("device_id = ? AND timestamp >= ? AND timestamp <= ? AND plate_detected = ?", deviceID, startTime, endTime, true).
		Count(&withPlates)

	database.DB.Model(&models.VehicleDetection{}).
		Where("device_id = ? AND timestamp >= ? AND timestamp <= ? AND make_model_detected = ?", deviceID, startTime, endTime, true).
		Count(&withMakeModel)

	stats.Classification = map[string]interface{}{
		"withPlates":          withPlates,
		"withoutPlates":       stats.TotalDetections - withPlates,
		"withMakeModel":       withMakeModel,
		"plateOnly":           withPlates - withMakeModel,
		"fullClassification": withMakeModel,
	}

	c.JSON(http.StatusOK, stats)
}

// GetVCCRealtime handles GET /api/vcc/realtime - Real-time vehicle counts
func GetVCCRealtime(c *gin.Context) {
	// Last 5 minutes
	startTime := time.Now().Add(-5 * time.Minute)
	endTime := time.Now()

	var stats struct {
		TotalDetections int64                `json:"totalDetections"`
		ByVehicleType   map[string]int64     `json:"byVehicleType"`
		ByDevice        []map[string]interface{} `json:"byDevice"`
		PerMinute       float64              `json:"perMinute"`
	}

	stats.ByVehicleType = make(map[string]int64)

	// Total in last 5 minutes
	database.DB.Model(&models.VehicleDetection{}).
		Where("timestamp >= ? AND timestamp <= ?", startTime, endTime).
		Count(&stats.TotalDetections)

	// By vehicle type
	var typeCounts []struct {
		VehicleType string
		Count       int64
	}
	database.DB.Model(&models.VehicleDetection{}).
		Select("vehicle_type, COUNT(*) as count").
		Where("timestamp >= ? AND timestamp <= ?", startTime, endTime).
		Group("vehicle_type").
		Scan(&typeCounts)

	for _, tc := range typeCounts {
		stats.ByVehicleType[tc.VehicleType] = tc.Count
	}

	// By device (top 10)
	var deviceCounts []struct {
		DeviceID   string
		DeviceName string
		Count      int64
	}
	database.DB.Model(&models.VehicleDetection{}).
		Select("vehicle_detections.device_id, devices.name as device_name, COUNT(*) as count").
		Joins("LEFT JOIN devices ON vehicle_detections.device_id = devices.id").
		Where("vehicle_detections.timestamp >= ? AND vehicle_detections.timestamp <= ?", startTime, endTime).
		Group("vehicle_detections.device_id, devices.name").
		Order("count DESC").
		Limit(10).
		Scan(&deviceCounts)

	stats.ByDevice = make([]map[string]interface{}, len(deviceCounts))
	for i, dc := range deviceCounts {
		stats.ByDevice[i] = map[string]interface{}{
			"deviceId":   dc.DeviceID,
			"deviceName": dc.DeviceName,
			"count":      dc.Count,
		}
	}

	stats.PerMinute = float64(stats.TotalDetections) / 5.0

	c.JSON(http.StatusOK, stats)
}

// GetVCCEvents handles GET /api/vcc/events - List raw VCC detection events
func GetVCCEvents(c *gin.Context) {
	// Parse range
	startTime := time.Now().AddDate(0, 0, -7)
	endTime := time.Now()

	if startTimeStr := c.Query("startTime"); startTimeStr != "" {
		if parsed, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			startTime = parsed
		}
	}
	if endTimeStr := c.Query("endTime"); endTimeStr != "" {
		if parsed, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			endTime = parsed
		}
	}

	query := database.DB.Model(&models.VehicleDetection{}).
		Preload("Device").
		Where("timestamp >= ? AND timestamp <= ?", startTime, endTime)

	// Filters
	if deviceID := c.Query("deviceId"); deviceID != "" {
		query = query.Where("device_id = ?", deviceID)
	}
	if vehicleType := c.Query("vehicleType"); vehicleType != "" {
		query = query.Where("vehicle_type = ?", vehicleType)
	}

	// Pagination
	limit := 1000
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 30000 {
			limit = parsed
		}
	}
	offset := 0
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsed, err := strconv.Atoi(offsetStr); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	var detections []models.VehicleDetection
	var total int64

	query.Count(&total)
	if err := query.Order("timestamp DESC").Limit(limit).Offset(offset).Find(&detections).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch events"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"events": detections,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}
