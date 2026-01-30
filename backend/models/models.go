package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// DeviceType enum
type DeviceType string

const (
	DeviceTypeCamera DeviceType = "CAMERA"
	DeviceTypeDrone  DeviceType = "DRONE"
	DeviceTypeSensor DeviceType = "SENSOR"
)

// CrowdDensityLevel enum
type CrowdDensityLevel string

const (
	DensityLow      CrowdDensityLevel = "LOW"
	DensityMedium   CrowdDensityLevel = "MEDIUM"
	DensityHigh     CrowdDensityLevel = "HIGH"
	DensityCritical CrowdDensityLevel = "CRITICAL"
)

// MovementType enum
type MovementType string

const (
	MovementStatic  MovementType = "STATIC"
	MovementMoving  MovementType = "MOVING"
	MovementFlowing MovementType = "FLOWING"
	MovementChaotic MovementType = "CHAOTIC"
)

// HotspotSeverity enum
type HotspotSeverity string

const (
	SeverityGreen  HotspotSeverity = "GREEN"
	SeverityYellow HotspotSeverity = "YELLOW"
	SeverityOrange HotspotSeverity = "ORANGE"
	SeverityRed    HotspotSeverity = "RED"
)

// JSONB type for GORM - can handle both objects and arrays
// Using a pointer to interface{} so we can implement both Value() and Scan()
type JSONB struct {
	Data interface{} `json:"-"`
}

// NewJSONB creates a new JSONB from any value
func NewJSONB(v interface{}) JSONB {
	return JSONB{Data: v}
}

// UnmarshalJSON implements json.Unmarshaler
func (j *JSONB) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &j.Data)
}

// MarshalJSON implements json.Marshaler
func (j JSONB) MarshalJSON() ([]byte, error) {
	if j.Data == nil {
		return []byte("null"), nil
	}
	return json.Marshal(j.Data)
}

func (j JSONB) Value() (driver.Value, error) {
	if j.Data == nil {
		return nil, nil
	}
	return json.Marshal(j.Data)
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		j.Data = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, &j.Data)
}

// Device model
type Device struct {
	ID       string     `gorm:"primaryKey;column:id" json:"id"`
	Type     DeviceType `gorm:"column:type" json:"type"`
	Name     *string    `gorm:"column:name" json:"name,omitempty"`
	ZoneID   *string    `gorm:"column:zone_id" json:"zoneId,omitempty"`
	Lat      float64    `gorm:"column:lat" json:"lat"`
	Lng      float64    `gorm:"column:lng" json:"lng"`
	Status   string     `gorm:"column:status;default:active" json:"status"`
	RTSPUrl  *string    `gorm:"column:rtsp_url" json:"rtspUrl,omitempty"`
	Metadata JSONB      `gorm:"type:jsonb;column:metadata" json:"metadata,omitempty"`
	Config   JSONB      `gorm:"type:jsonb;column:config" json:"config,omitempty"`
	WorkerID *string    `gorm:"column:worker_id" json:"workerId,omitempty"`

	CreatedAt time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`

	Events        []Event        `gorm:"foreignKey:DeviceID" json:"events,omitempty"`
	CrowdAnalyses []CrowdAnalysis `gorm:"foreignKey:DeviceID" json:"crowdAnalyses,omitempty"`
	CrowdAlerts   []CrowdAlert   `gorm:"foreignKey:DeviceID" json:"crowdAlerts,omitempty"`
	Violations    []TrafficViolation `gorm:"foreignKey:DeviceID" json:"violations,omitempty"`
	VehicleDetections []VehicleDetection `gorm:"foreignKey:DeviceID" json:"vehicleDetections,omitempty"`
}

func (Device) TableName() string {
	return "devices"
}

// Event model
type Event struct {
	ID        int64     `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	DeviceID  string    `gorm:"column:device_id;index" json:"deviceId"`
	Device    Device    `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
	Timestamp time.Time `gorm:"column:timestamp;default:CURRENT_TIMESTAMP;index" json:"timestamp"`
	Type      string    `gorm:"column:type" json:"type"`
	Data      JSONB     `gorm:"type:jsonb;column:data" json:"data"`
	RiskLevel *string   `gorm:"column:risk_level" json:"riskLevel,omitempty"`
}

func (Event) TableName() string {
	return "events"
}

// WorkerStatus enum
type WorkerStatus string

const (
	WorkerStatusPending  WorkerStatus = "pending"
	WorkerStatusApproved WorkerStatus = "approved"
	WorkerStatusActive   WorkerStatus = "active"
	WorkerStatusOffline  WorkerStatus = "offline"
	WorkerStatusRevoked  WorkerStatus = "revoked"
)

// Worker model - Edge computing node (Jetson device)
type Worker struct {
	ID          string       `gorm:"primaryKey;column:id" json:"id"`
	Name        string       `gorm:"column:name" json:"name"`
	Status      WorkerStatus `gorm:"column:status;default:pending;index" json:"status"`
	
	// Device info
	IP          string    `gorm:"column:ip" json:"ip"`
	MAC         string    `gorm:"column:mac;uniqueIndex" json:"mac"`
	Model       string    `gorm:"column:model" json:"model"`        // e.g., "Jetson Orin NX 8GB"
	Version     *string   `gorm:"column:version" json:"version"`    // Worker software version
	
	// Authentication
	AuthToken   string    `gorm:"column:auth_token;uniqueIndex" json:"-"` // Hidden from JSON
	
	// Approval
	ApprovedAt  *time.Time `gorm:"column:approved_at" json:"approvedAt,omitempty"`
	ApprovedBy  *string    `gorm:"column:approved_by" json:"approvedBy,omitempty"`
	
	// Status tracking
	LastSeen    time.Time `gorm:"column:last_seen;default:CURRENT_TIMESTAMP;index" json:"lastSeen"`
	LastIP      *string   `gorm:"column:last_ip" json:"lastIp,omitempty"`
	
	// Resource monitoring
	Resources   JSONB     `gorm:"type:jsonb;column:resources" json:"resources,omitempty"` // CPU, GPU, memory, temp
	
	// Configuration
	Config      JSONB     `gorm:"type:jsonb;column:config" json:"config,omitempty"` // Full worker config
	ConfigVersion int     `gorm:"column:config_version;default:0" json:"configVersion"`
	
	// Metadata
	Metadata    JSONB     `gorm:"type:jsonb;column:metadata" json:"metadata,omitempty"`
	Tags        JSONB     `gorm:"type:jsonb;column:tags" json:"tags,omitempty"` // For grouping
	
	// WireGuard VPN
	WireGuardIP     *string `gorm:"column:wireguard_ip;uniqueIndex" json:"wireguardIp,omitempty"`     // e.g., "10.10.0.10"
	WireGuardPubKey *string `gorm:"column:wireguard_pubkey" json:"wireguardPubKey,omitempty"`        // Base64 public key
	
	CreatedAt   time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
	
	// Relations
	CameraAssignments []WorkerCameraAssignment `gorm:"foreignKey:WorkerID" json:"cameraAssignments,omitempty"`
}

func (Worker) TableName() string {
	return "workers"
}

// WorkerToken model - Pre-generated tokens for worker registration
type WorkerToken struct {
	ID          string     `gorm:"primaryKey;column:id" json:"id"`
	Token       string     `gorm:"column:token;uniqueIndex" json:"token"`
	Name        string     `gorm:"column:name" json:"name"` // Description, e.g., "For Brigade Road deployment"
	
	// Usage tracking
	UsedBy      *string    `gorm:"column:used_by" json:"usedBy,omitempty"` // Worker ID that used this token
	UsedAt      *time.Time `gorm:"column:used_at" json:"usedAt,omitempty"`
	
	// Validity
	ExpiresAt   *time.Time `gorm:"column:expires_at" json:"expiresAt,omitempty"`
	IsRevoked   bool       `gorm:"column:is_revoked;default:false" json:"isRevoked"`
	
	// Audit
	CreatedBy   string     `gorm:"column:created_by" json:"createdBy"`
	CreatedAt   time.Time  `gorm:"column:created_at;default:CURRENT_TIMESTAMP" json:"createdAt"`
}

func (WorkerToken) TableName() string {
	return "worker_tokens"
}

// WorkerCameraAssignment model - Which cameras are assigned to which worker
type WorkerCameraAssignment struct {
	ID          int64     `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	WorkerID    string    `gorm:"column:worker_id;index;uniqueIndex:idx_worker_device" json:"workerId"`
	Worker      *Worker   `gorm:"foreignKey:WorkerID" json:"worker,omitempty"`
	DeviceID    string    `gorm:"column:device_id;index;uniqueIndex:idx_worker_device" json:"deviceId"`
	Device      *Device   `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
	
	// Analytics configuration for this camera on this worker
	Analytics   JSONB     `gorm:"type:jsonb;column:analytics" json:"analytics"` // ["anpr", "vcc", "crowd"]
	FPS         int       `gorm:"column:fps;default:15" json:"fps"`
	Resolution  string    `gorm:"column:resolution;default:720p" json:"resolution"`
	
	// Status
	IsActive    bool      `gorm:"column:is_active;default:true" json:"isActive"`
	
	CreatedAt   time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (WorkerCameraAssignment) TableName() string {
	return "worker_camera_assignments"
}

// WorkerApprovalRequest model - For tokenless registration requests
type WorkerApprovalRequest struct {
	ID          string    `gorm:"primaryKey;column:id" json:"id"`
	
	// Device info from request
	DeviceName  string    `gorm:"column:device_name" json:"deviceName"`
	IP          string    `gorm:"column:ip" json:"ip"`
	MAC         string    `gorm:"column:mac;index" json:"mac"`
	Model       string    `gorm:"column:model" json:"model"`
	
	// Request status
	Status      string    `gorm:"column:status;default:pending;index" json:"status"` // pending, approved, rejected
	
	// If approved
	WorkerID    *string   `gorm:"column:worker_id" json:"workerId,omitempty"`
	
	// If rejected
	RejectedBy   *string   `gorm:"column:rejected_by" json:"rejectedBy,omitempty"`
	RejectedAt   *time.Time `gorm:"column:rejected_at" json:"rejectedAt,omitempty"`
	RejectReason *string   `gorm:"column:reject_reason" json:"rejectReason,omitempty"`
	
	CreatedAt   time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (WorkerApprovalRequest) TableName() string {
	return "worker_approval_requests"
}

// CrowdAnalysis model
type CrowdAnalysis struct {
	ID        int64             `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	DeviceID  string            `gorm:"column:device_id;index" json:"deviceId"`
	Device    Device            `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
	Timestamp time.Time         `gorm:"column:timestamp;default:CURRENT_TIMESTAMP;index" json:"timestamp"`
	
	PeopleCount *int `gorm:"column:people_count" json:"peopleCount,omitempty"`
	
	DensityValue *float64           `gorm:"column:density_value" json:"densityValue,omitempty"`
	DensityLevel CrowdDensityLevel  `gorm:"column:density_level" json:"densityLevel"`
	
	MovementType MovementType `gorm:"column:movement_type" json:"movementType"`
	FlowRate     *float64     `gorm:"column:flow_rate" json:"flowRate,omitempty"`
	Velocity     *float64     `gorm:"column:velocity" json:"velocity,omitempty"`
	
	FreeSpace       *float64 `gorm:"column:free_space" json:"freeSpace,omitempty"`
	CongestionLevel *int     `gorm:"column:congestion_level" json:"congestionLevel,omitempty"`
	OccupancyRate   *float64 `gorm:"column:occupancy_rate" json:"occupancyRate,omitempty"`
	
	HotspotSeverity HotspotSeverity `gorm:"column:hotspot_severity;index" json:"hotspotSeverity"`
	HotspotZones    JSONB           `gorm:"type:jsonb;column:hotspot_zones" json:"hotspotZones,omitempty"`
	MaxDensityPoint  JSONB          `gorm:"type:jsonb;column:max_density_point" json:"maxDensityPoint,omitempty"`
	
	Demographics JSONB   `gorm:"type:jsonb;column:demographics" json:"demographics,omitempty"`
	Behavior     *string `gorm:"column:behavior" json:"behavior,omitempty"`
	Anomalies    JSONB   `gorm:"type:jsonb;column:anomalies" json:"anomalies,omitempty"`
	
	HeatmapData     JSONB   `gorm:"type:jsonb;column:heatmap_data" json:"heatmapData,omitempty"`
	HeatmapImageURL *string `gorm:"column:heatmap_image_url" json:"heatmapImageUrl,omitempty"`
	FrameID         *string `gorm:"column:frame_id" json:"frameId,omitempty"`
	FrameURL        *string `gorm:"column:frame_url" json:"frameUrl,omitempty"`
	
	ModelType  *string  `gorm:"column:model_type" json:"modelType,omitempty"`
	Confidence *float64 `gorm:"column:confidence" json:"confidence,omitempty"`
	
	CrowdAlerts []CrowdAlert `gorm:"foreignKey:AnalysisID" json:"crowdAlerts,omitempty"`
}

func (CrowdAnalysis) TableName() string {
	return "crowd_analyses"
}

// CrowdAlert model
type CrowdAlert struct {
	ID         int64     `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	DeviceID   string    `gorm:"column:device_id;index" json:"deviceId"`
	Device     Device    `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
	Timestamp  time.Time `gorm:"column:timestamp;default:CURRENT_TIMESTAMP;index" json:"timestamp"`
	ResolvedAt *time.Time `gorm:"column:resolved_at" json:"resolvedAt,omitempty"`
	IsResolved bool      `gorm:"column:is_resolved;default:false;index" json:"isResolved"`
	
	AlertType string          `gorm:"column:alert_type;index" json:"alertType"`
	Severity  HotspotSeverity `gorm:"column:severity" json:"severity"`
	Priority  int            `gorm:"column:priority;default:5" json:"priority"`
	
	TriggerRule    JSONB    `gorm:"type:jsonb;column:trigger_rule" json:"triggerRule"`
	ThresholdValue *float64 `gorm:"column:threshold_value" json:"thresholdValue,omitempty"`
	ActualValue    float64  `gorm:"column:actual_value" json:"actualValue"`
	
	PeopleCount     *int              `gorm:"column:people_count" json:"peopleCount,omitempty"`
	DensityLevel    CrowdDensityLevel `gorm:"column:density_level" json:"densityLevel"`
	CongestionLevel *int              `gorm:"column:congestion_level" json:"congestionLevel,omitempty"`
	MovementType    *MovementType     `gorm:"column:movement_type" json:"movementType,omitempty"`
	
	Title           string  `gorm:"column:title" json:"title"`
	Description     *string `gorm:"column:description" json:"description,omitempty"`
	Recommendations JSONB   `gorm:"type:jsonb;column:recommendations" json:"recommendations,omitempty"`
	
	AnalysisID      *int64         `gorm:"column:analysis_id" json:"analysisId,omitempty"`
	RelatedAnalysis *CrowdAnalysis `gorm:"foreignKey:AnalysisID" json:"relatedAnalysis,omitempty"`
	
	ResolvedBy     *string `gorm:"column:resolved_by" json:"resolvedBy,omitempty"`
	ResolutionNote *string `gorm:"column:resolution_note" json:"resolutionNote,omitempty"`
}

func (CrowdAlert) TableName() string {
	return "crowd_alerts"
}

// ViolationType enum
type ViolationType string

const (
	ViolationSpeed       ViolationType = "SPEED"
	ViolationHelmet     ViolationType = "HELMET"
	ViolationWrongSide   ViolationType = "WRONG_SIDE"
	ViolationRedLight    ViolationType = "RED_LIGHT"
	ViolationNoSeatbelt  ViolationType = "NO_SEATBELT"
	ViolationOverloading ViolationType = "OVERLOADING"
	ViolationIllegalParking ViolationType = "ILLEGAL_PARKING"
	ViolationOther       ViolationType = "OTHER"
)

// ViolationStatus enum
type ViolationStatus string

const (
	ViolationPending  ViolationStatus = "PENDING"
	ViolationApproved ViolationStatus = "APPROVED"
	ViolationRejected ViolationStatus = "REJECTED"
	ViolationFined    ViolationStatus = "FINED"
)

// DetectionMethod enum
type DetectionMethod string

const (
	DetectionRadar    DetectionMethod = "RADAR"
	DetectionCamera   DetectionMethod = "CAMERA"
	DetectionAIVision DetectionMethod = "AI_VISION"
	DetectionManual   DetectionMethod = "MANUAL"
)

// TrafficViolation model
type TrafficViolation struct {
	ID        int64     `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	DeviceID  string    `gorm:"column:device_id;index" json:"deviceId"`
	Device    Device    `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
	VehicleID *int64    `gorm:"column:vehicle_id;index" json:"vehicleId,omitempty"` // Link to vehicle if identified
	Vehicle   *Vehicle  `gorm:"foreignKey:VehicleID" json:"vehicle,omitempty"`
	Timestamp time.Time `gorm:"column:timestamp;default:CURRENT_TIMESTAMP;index" json:"timestamp"`

	ViolationType  ViolationType  `gorm:"column:violation_type;index" json:"violationType"`
	Status         ViolationStatus `gorm:"column:status;default:PENDING;index" json:"status"`
	DetectionMethod DetectionMethod `gorm:"column:detection_method" json:"detectionMethod"`

	PlateNumber    *string  `gorm:"column:plate_number;index" json:"plateNumber,omitempty"`
	PlateConfidence *float64 `gorm:"column:plate_confidence" json:"plateConfidence,omitempty"`
	PlateImageURL  *string  `gorm:"column:plate_image_url" json:"plateImageUrl,omitempty"`

	FullSnapshotURL *string `gorm:"column:full_snapshot_url" json:"fullSnapshotUrl,omitempty"`
	FrameID         *string `gorm:"column:frame_id" json:"frameId,omitempty"`

	DetectedSpeed  *float64 `gorm:"column:detected_speed" json:"detectedSpeed,omitempty"`
	SpeedLimit2W   *float64 `gorm:"column:speed_limit_2w" json:"speedLimit2W,omitempty"`
	SpeedLimit4W   *float64 `gorm:"column:speed_limit_4w" json:"speedLimit4W,omitempty"`
	SpeedOverLimit *float64 `gorm:"column:speed_over_limit" json:"speedOverLimit,omitempty"`

	Confidence *float64 `gorm:"column:confidence" json:"confidence,omitempty"`
	Metadata   JSONB    `gorm:"type:jsonb;column:metadata" json:"metadata,omitempty"`

	ReviewedAt     *time.Time `gorm:"column:reviewed_at" json:"reviewedAt,omitempty"`
	ReviewedBy     *string    `gorm:"column:reviewed_by" json:"reviewedBy,omitempty"`
	ReviewNote     *string    `gorm:"column:review_note" json:"reviewNote,omitempty"`
	RejectionReason *string   `gorm:"column:rejection_reason" json:"rejectionReason,omitempty"`

	FineAmount    *float64   `gorm:"column:fine_amount" json:"fineAmount,omitempty"`
	FineIssuedAt  *time.Time `gorm:"column:fine_issued_at" json:"fineIssuedAt,omitempty"`
	FineReference *string    `gorm:"column:fine_reference" json:"fineReference,omitempty"`
}

func (TrafficViolation) TableName() string {
	return "traffic_violations"
}

// VehicleType enum
type VehicleType string

const (
	VehicleType2Wheeler VehicleType = "2W"
	VehicleType4Wheeler VehicleType = "4W"
	VehicleTypeAuto     VehicleType = "AUTO"
	VehicleTypeTruck    VehicleType = "TRUCK"
	VehicleTypeBus      VehicleType = "BUS"
	VehicleTypeHMV      VehicleType = "HMV"
	VehicleTypeUnknown  VehicleType = "UNKNOWN"
)

// Vehicle model - Represents a unique vehicle (identified by plate or characteristics)
type Vehicle struct {
	ID        int64     `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	PlateNumber *string `gorm:"column:plate_number;uniqueIndex;index" json:"plateNumber,omitempty"` // Nullable - some vehicles may not have plates
	
	// Vehicle characteristics (may be partial)
	Make       *string    `gorm:"column:make" json:"make,omitempty"`       // e.g., "Honda", "Toyota"
	Model      *string    `gorm:"column:model" json:"model,omitempty"`     // e.g., "City", "Innova"
	VehicleType VehicleType `gorm:"column:vehicle_type" json:"vehicleType"` // 2W, 4W, AUTO, TRUCK, BUS
	Color      *string    `gorm:"column:color" json:"color,omitempty"`     // e.g., "White", "Black"
	
	// Tracking
	FirstSeen      time.Time `gorm:"column:first_seen;index" json:"firstSeen"`
	LastSeen       time.Time `gorm:"column:last_seen;index" json:"lastSeen"`
	DetectionCount int64     `gorm:"column:detection_count;default:0" json:"detectionCount"`
	
	// Watchlist
	IsWatchlisted bool `gorm:"column:is_watchlisted;default:false;index" json:"isWatchlisted"`
	
	// Metadata
	Metadata JSONB `gorm:"type:jsonb;column:metadata" json:"metadata,omitempty"` // Additional vehicle info
	
	CreatedAt time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
	
	// Relations
	Detections []VehicleDetection `gorm:"foreignKey:VehicleID" json:"detections,omitempty"`
	Violations []TrafficViolation  `gorm:"foreignKey:VehicleID" json:"violations,omitempty"`
	Watchlist  *Watchlist          `gorm:"foreignKey:VehicleID" json:"watchlist,omitempty"`
}

func (Vehicle) TableName() string {
	return "vehicles"
}

// VehicleDetection model - Each time a vehicle is detected by a camera
type VehicleDetection struct {
	ID        int64     `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	VehicleID *int64    `gorm:"column:vehicle_id;index:idx_detection_vehicle_id" json:"vehicleId,omitempty"` // Nullable - may not be linked to vehicle yet
	Vehicle   *Vehicle  `gorm:"foreignKey:VehicleID" json:"vehicle,omitempty"`
	DeviceID  string    `gorm:"column:device_id;index:idx_detection_device_id" json:"deviceId"`
	Device    Device    `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
	Timestamp time.Time `gorm:"column:timestamp;default:CURRENT_TIMESTAMP;index:idx_detection_timestamp" json:"timestamp"`
	
	// Detection details (may be partial)
	PlateNumber    *string     `gorm:"column:plate_number;index:idx_detection_plate" json:"plateNumber,omitempty"`
	PlateConfidence *float64   `gorm:"column:plate_confidence" json:"plateConfidence,omitempty"`
	Make           *string     `gorm:"column:make" json:"make,omitempty"`
	Model          *string     `gorm:"column:model" json:"model,omitempty"`
	VehicleType    VehicleType `gorm:"column:vehicle_type;index:idx_detection_type" json:"vehicleType"`
	Color          *string     `gorm:"column:color" json:"color,omitempty"`
	
	// Detection quality
	Confidence     *float64 `gorm:"column:confidence" json:"confidence,omitempty"` // Overall detection confidence
	PlateDetected  bool     `gorm:"column:plate_detected;default:false" json:"plateDetected"`
	MakeModelDetected bool  `gorm:"column:make_model_detected;default:false" json:"makeModelDetected"`
	
	// Images
	FullImageURL   *string `gorm:"column:full_image_url" json:"fullImageUrl,omitempty"`
	PlateImageURL  *string `gorm:"column:plate_image_url" json:"plateImageUrl,omitempty"`
	VehicleImageURL *string `gorm:"column:vehicle_image_url" json:"vehicleImageUrl,omitempty"`
	FrameID        *string `gorm:"column:frame_id" json:"frameId,omitempty"`
	
	// Location and direction
	Direction      *string  `gorm:"column:direction" json:"direction,omitempty"` // "north", "south", "east", "west"
	Lane           *int     `gorm:"column:lane" json:"lane,omitempty"`
	
	// Metadata
	Metadata JSONB `gorm:"type:jsonb;column:metadata" json:"metadata,omitempty"` // Bounding boxes, speed, etc.
}

func (VehicleDetection) TableName() string {
	return "vehicle_detections"
}

// Watchlist model - Vehicles to monitor/watch
type Watchlist struct {
	ID        int64     `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	VehicleID int64     `gorm:"column:vehicle_id;uniqueIndex" json:"vehicleId"`
	Vehicle   Vehicle   `gorm:"foreignKey:VehicleID" json:"vehicle,omitempty"`
	
	Reason    string    `gorm:"column:reason" json:"reason"` // Why it's watchlisted
	AddedBy   string    `gorm:"column:added_by" json:"addedBy"` // User ID
	AddedAt   time.Time `gorm:"column:added_at;default:CURRENT_TIMESTAMP" json:"addedAt"`
	IsActive  bool      `gorm:"column:is_active;default:true;index" json:"isActive"`
	
	// Alerts
	AlertOnDetection bool `gorm:"column:alert_on_detection;default:true" json:"alertOnDetection"`
	AlertOnViolation bool `gorm:"column:alert_on_violation;default:true" json:"alertOnViolation"`
	
	// Notes
	Notes     *string `gorm:"column:notes" json:"notes,omitempty"`
	
	CreatedAt time.Time `gorm:"column:created_at;default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (Watchlist) TableName() string {
	return "watchlist"
}

