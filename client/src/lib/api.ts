// API Client - Updated 2025-12-26
// Use relative path for API calls - Vite will proxy /api to backend
const API_BASE_URL = '';

// Re-export worker types from separate file
export type {
  WorkerStatus,
  Worker,
  WorkerWithCounts,
  WorkerToken,
  WorkerTokenWithStatus,
  WorkerApprovalRequest,
  WorkerCameraAssignment,
  CameraAssignment
} from './worker-types';

export type DeviceType = 'CAMERA' | 'DRONE' | 'SENSOR';
export type DeviceStatus = 'ACTIVE' | 'INACTIVE' | 'MAINTENANCE' | 'active' | 'inactive' | 'maintenance';

// Full device interface (for detail views)
export interface Device {
  id: string;
  name: string;
  type: DeviceType;
  lat: number;
  lng: number;
  status: DeviceStatus;
  zoneId?: string;
  description?: string | null;
  rtspUrl?: string | null;
  metadata?: Record<string, any>;
  config?: Record<string, any>;
  events?: any[];
  workerId?: string | null;
  createdAt: string;
  updatedAt: string;
  latestEvent?: {
    id: string;
    eventType: string;
    data: Record<string, any>;
    timestamp: string;
  };
}

// Minimal device interface for map view (reduces payload size)
export interface DeviceMapMarker {
  id: string;
  name: string;
  type: DeviceType;
  lat: number;
  lng: number;
  status: DeviceStatus;
}

// Hotspot interface for crowd visualization
export interface Hotspot {
  deviceId: string;
  name: string;
  lat: number;
  lng: number;
  type: DeviceType;
  status: DeviceStatus;
  zoneId?: string;
  hotspotSeverity: 'GREEN' | 'YELLOW' | 'ORANGE' | 'RED';
  peopleCount: number | null;
  densityLevel: 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL';
  congestionLevel: number | null;
  lastUpdated: string | null;
}

// Crowd Analysis interface
export interface CrowdAnalysis {
  id: string;
  deviceId: string;
  timestamp: string;
  peopleCount: number | null;
  crowdLevel: number; // 0-100 percentage relative to min/max in response
  densityValue: number | null;
  densityLevel: 'LOW' | 'MEDIUM' | 'HIGH' | 'CRITICAL';
  movementType: 'STATIC' | 'MOVING' | 'FLOWING' | 'CHAOTIC';
  flowRate: number | null;
  velocity: number | null;
  freeSpace: number | null;
  congestionLevel: number | null;
  occupancyRate: number | null;
  hotspotSeverity: 'GREEN' | 'YELLOW' | 'ORANGE' | 'RED';
  hotspotZones?: Array<{ x: number; y: number; radius: number; severity: string }>;
  maxDensityPoint?: { x: number; y: number; density: number };
  demographics?: {
    gender?: { male: number; female: number };
    ageGroups?: { adults: number; seniors: number; children: number };
  };
  behavior?: string | null;
  anomalies?: string[];
  heatmapData?: any;
  heatmapImageUrl?: string | null;
  frameId?: string | null;
  frameUrl?: string | null;
  modelType?: string | null;
  confidence?: number | null;
  device: {
    id: string;
    name: string;
    lat: number;
    lng: number;
    type: DeviceType;
  };
}

export interface ApiResponse<T> {
  data: T;
  error?: string;
}

// Import worker types for use in ApiClient methods
import type {
  WorkerStatus,
  Worker,
  WorkerWithCounts,
  WorkerToken,
  WorkerTokenWithStatus,
  WorkerApprovalRequest,
  WorkerCameraAssignment,
  CameraAssignment
} from './worker-types';

class ApiClient {
  private baseUrl: string;

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl;
  }

  private async request<T>(
    endpoint: string,
    options?: RequestInit
  ): Promise<T> {
    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options?.headers,
      },
    });

    if (!response.ok) {
      throw new Error(`API Error: ${response.statusText}`);
    }

    return response.json();
  }

  // Device endpoints
  async getDevices(options?: {
    type?: DeviceType;
    minimal?: boolean; // Return only essential fields for map view
  }): Promise<Device[] | DeviceMapMarker[]> {
    const params = new URLSearchParams();
    if (options?.type) {
      params.append('type', options.type);
    }
    if (options?.minimal) {
      params.append('minimal', 'true');
    }
    const query = params.toString();
    return this.request<Device[] | DeviceMapMarker[]>(
      `/api/devices${query ? `?${query}` : ''}`
    );
  }

  // Get devices by type (optimized for map view)
  async getDevicesByType(type: DeviceType): Promise<DeviceMapMarker[]> {
    return this.getDevices({ type, minimal: true }) as Promise<DeviceMapMarker[]>;
  }

  // Get all devices for map (fetches by type in parallel)
  async getDevicesForMap(): Promise<DeviceMapMarker[]> {
    const [cameras, drones, sensors] = await Promise.all([
      this.getDevicesByType('CAMERA'),
      this.getDevicesByType('DRONE'),
      this.getDevicesByType('SENSOR'),
    ]);
    return [...cameras, ...drones, ...sensors];
  }

  async getDevice(id: string): Promise<Device> {
    return this.request<Device>(`/api/devices/${id}`);
  }

  async createDevice(device: Omit<Device, 'id' | 'createdAt' | 'updatedAt'>): Promise<Device> {
    return this.request<Device>('/api/devices', {
      method: 'POST',
      body: JSON.stringify(device),
    });
  }

  async updateDevice(id: string, updates: Partial<Device>): Promise<Device> {
    return this.request<Device>(`/api/devices/${id}`, {
      method: 'PUT',
      body: JSON.stringify(updates),
    });
  }

  async deleteDevice(id: string): Promise<void> {
    return this.request<void>(`/api/devices/${id}`, {
      method: 'DELETE',
    });
  }

  // Crowd endpoints
  async getHotspots(): Promise<Hotspot[]> {
    return this.request<Hotspot[]>('/api/crowd/hotspots');
  }

  async getCrowdAnalysis(options?: {
    deviceId?: string;
    startTime?: string;
    endTime?: string;
    limit?: number;
    severity?: 'GREEN' | 'YELLOW' | 'ORANGE' | 'RED';
  }): Promise<CrowdAnalysis[]> {
    const params = new URLSearchParams();
    if (options?.deviceId) {
      params.append('deviceId', options.deviceId);
    }
    if (options?.startTime) {
      params.append('startTime', options.startTime);
    }
    if (options?.endTime) {
      params.append('endTime', options.endTime);
    }
    if (options?.limit) {
      params.append('limit', options.limit.toString());
    }
    if (options?.severity) {
      params.append('severity', options.severity);
    }
    const query = params.toString();
    return this.request<CrowdAnalysis[]>(`/api/crowd/analysis${query ? `?${query}` : ''}`);
  }

  async getLatestCrowdAnalysis(deviceIds?: string[]): Promise<CrowdAnalysis[]> {
    const params = new URLSearchParams();
    if (deviceIds && deviceIds.length > 0) {
      params.append('deviceIds', deviceIds.join(','));
    }
    const query = params.toString();
    return this.request<CrowdAnalysis[]>(`/api/crowd/analysis/latest${query ? `?${query}` : ''}`);
  }

  // Event endpoints
  async ingestEvent(event: {
    deviceId: string;
    eventType: string;
    data: Record<string, any>;
    timestamp?: string;
  }): Promise<void> {
    return this.request<void>('/api/ingest', {
      method: 'POST',
      body: JSON.stringify(event),
    });
  }

  // Violation endpoints (ITMS)
  async getViolations(options?: {
    status?: 'PENDING' | 'APPROVED' | 'REJECTED' | 'FINED';
    violationType?: 'SPEED' | 'HELMET' | 'WRONG_SIDE' | 'RED_LIGHT' | 'NO_SEATBELT' | 'OVERLOADING' | 'ILLEGAL_PARKING' | 'OTHER';
    deviceId?: string;
    plateNumber?: string;
    startTime?: string;
    endTime?: string;
    limit?: number;
    offset?: number;
  }): Promise<{ violations: TrafficViolation[]; total: number; limit: number; offset: number }> {
    const params = new URLSearchParams();
    if (options?.status) params.append('status', options.status);
    if (options?.violationType) params.append('violationType', options.violationType);
    if (options?.deviceId) params.append('deviceId', options.deviceId);
    if (options?.plateNumber) params.append('plateNumber', options.plateNumber);
    if (options?.startTime) params.append('startTime', options.startTime);
    if (options?.endTime) params.append('endTime', options.endTime);
    if (options?.limit) params.append('limit', options.limit.toString());
    if (options?.offset) params.append('offset', options.offset.toString());
    const query = params.toString();
    return this.request<{ violations: TrafficViolation[]; total: number; limit: number; offset: number }>(
      `/api/violations${query ? `?${query}` : ''}`
    );
  }

  async getViolation(id: string): Promise<TrafficViolation> {
    return this.request<TrafficViolation>(`/api/violations/${id}`);
  }

  async approveViolation(id: string, data?: { reviewNote?: string; reviewedBy?: string }): Promise<TrafficViolation> {
    return this.request<TrafficViolation>(`/api/violations/${id}/approve`, {
      method: 'PATCH',
      body: JSON.stringify(data || {}),
    });
  }

  async rejectViolation(id: string, data: { rejectionReason: string; reviewedBy?: string }): Promise<TrafficViolation> {
    return this.request<TrafficViolation>(`/api/violations/${id}/reject`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    });
  }

  async updateViolationPlate(id: string, plateNumber: string): Promise<TrafficViolation> {
    return this.request<TrafficViolation>(`/api/violations/${id}/plate`, {
      method: 'PATCH',
      body: JSON.stringify({ plateNumber }),
    });
  }

  async getViolationStats(): Promise<ViolationStats> {
    return this.request<ViolationStats>('/api/violations/stats');
  }

  // Vehicle endpoints (ANPR/VCC)
  async detectVehicle(detection: {
    deviceId: string;
    plateNumber?: string;
    plateConfidence?: number;
    make?: string;
    model?: string;
    vehicleType: VehicleType;
    color?: string;
    confidence?: number;
    fullImageUrl?: string;
    plateImageUrl?: string;
    vehicleImageUrl?: string;
    frameId?: string;
    direction?: string;
    lane?: number;
    metadata?: any;
    timestamp?: string;
  }): Promise<{ success: boolean; detectionId: string; vehicleId?: string }> {
    return this.request<{ success: boolean; detectionId: string; vehicleId?: string }>('/api/vehicles/detect', {
      method: 'POST',
      body: JSON.stringify(detection),
    });
  }

  async getVehicles(options?: {
    plateNumber?: string;
    vehicleType?: VehicleType;
    make?: string;
    model?: string;
    color?: string;
    watchlisted?: boolean;
    startTime?: string;
    endTime?: string;
    limit?: number;
    offset?: number;
    orderBy?: string;
    orderDir?: 'asc' | 'desc';
  }): Promise<{ vehicles: Vehicle[]; total: number; limit: number; offset: number }> {
    const params = new URLSearchParams();
    if (options?.plateNumber) params.append('plateNumber', options.plateNumber);
    if (options?.vehicleType) params.append('vehicleType', options.vehicleType);
    if (options?.make) params.append('make', options.make);
    if (options?.model) params.append('model', options.model);
    if (options?.color) params.append('color', options.color);
    if (options?.watchlisted !== undefined) params.append('watchlisted', options.watchlisted.toString());
    if (options?.startTime) params.append('startTime', options.startTime);
    if (options?.endTime) params.append('endTime', options.endTime);
    if (options?.limit) params.append('limit', options.limit.toString());
    if (options?.offset) params.append('offset', options.offset.toString());
    if (options?.orderBy) params.append('orderBy', options.orderBy);
    if (options?.orderDir) params.append('orderDir', options.orderDir);
    const query = params.toString();
    return this.request<{ vehicles: Vehicle[]; total: number; limit: number; offset: number }>(
      `/api/vehicles${query ? `?${query}` : ''}`
    );
  }

  async getVehicle(id: string): Promise<Vehicle> {
    return this.request<Vehicle>(`/api/vehicles/${id}`);
  }

  async updateVehicle(id: string, updates: {
    plateNumber?: string;
    make?: string;
    model?: string;
    vehicleType?: VehicleType;
    color?: string;
    metadata?: any;
  }): Promise<Vehicle> {
    return this.request<Vehicle>(`/api/vehicles/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(updates),
    });
  }

  async getVehicleDetections(id: string, options?: {
    deviceId?: string;
    startTime?: string;
    endTime?: string;
    limit?: number;
  }): Promise<VehicleDetection[]> {
    const params = new URLSearchParams();
    if (options?.deviceId) params.append('deviceId', options.deviceId);
    if (options?.startTime) params.append('startTime', options.startTime);
    if (options?.endTime) params.append('endTime', options.endTime);
    if (options?.limit) params.append('limit', options.limit.toString());
    const query = params.toString();
    return this.request<VehicleDetection[]>(`/api/vehicles/${id}/detections${query ? `?${query}` : ''}`);
  }

  async getVehicleViolations(id: string, options?: {
    status?: ViolationStatus;
    limit?: number;
  }): Promise<TrafficViolation[]> {
    const params = new URLSearchParams();
    if (options?.status) params.append('status', options.status);
    if (options?.limit) params.append('limit', options.limit.toString());
    const query = params.toString();
    return this.request<TrafficViolation[]>(`/api/vehicles/${id}/violations${query ? `?${query}` : ''}`);
  }

  async addToWatchlist(id: string, data: {
    reason: string;
    addedBy: string;
    alertOnDetection?: boolean;
    alertOnViolation?: boolean;
    notes?: string;
  }): Promise<Watchlist> {
    return this.request<Watchlist>(`/api/vehicles/${id}/watchlist`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async removeFromWatchlist(id: string): Promise<void> {
    return this.request<void>(`/api/vehicles/${id}/watchlist`, {
      method: 'DELETE',
    });
  }

  async getWatchlist(): Promise<Watchlist[]> {
    return this.request<Watchlist[]>('/api/watchlist');
  }

  async getVehicleStats(): Promise<VehicleStats> {
    return this.request<VehicleStats>('/api/vehicles/stats');
  }

  // VCC (Vehicle Classification and Counting) endpoints
  async getVCCStats(options?: {
    startTime?: string;
    endTime?: string;
    groupBy?: 'minute' | 'hour' | 'day' | 'week' | 'month';
  }): Promise<VCCStats> {
    const params = new URLSearchParams();
    if (options?.startTime) params.append('startTime', options.startTime);
    if (options?.endTime) params.append('endTime', options.endTime);
    if (options?.groupBy) params.append('groupBy', options.groupBy);
    const query = params.toString();
    return this.request<VCCStats>(`/api/vcc/stats${query ? `?${query}` : ''}`);
  }

  async getVCCByDevice(deviceId: string, options?: {
    startTime?: string;
    endTime?: string;
    groupBy?: 'minute' | 'hour' | 'day' | 'week' | 'month';
  }): Promise<VCCDeviceStats> {
    const params = new URLSearchParams();
    if (options?.startTime) params.append('startTime', options.startTime);
    if (options?.endTime) params.append('endTime', options.endTime);
    if (options?.groupBy) params.append('groupBy', options.groupBy);
    const query = params.toString();
    return this.request<VCCDeviceStats>(`/api/vcc/device/${deviceId}${query ? `?${query}` : ''}`);
  }

  async getVCCRealtime(): Promise<VCCRealtime> {
    return this.request<VCCRealtime>('/api/vcc/realtime');
  }

  async getVCCEvents(options?: {
    startTime?: string;
    endTime?: string;
    deviceId?: string;
    vehicleType?: string;
    limit?: number;
    offset?: number;
  }): Promise<{ events: VehicleDetection[]; total: number; limit: number; offset: number }> {
    const params = new URLSearchParams();
    if (options?.startTime) params.append('startTime', options.startTime);
    if (options?.endTime) params.append('endTime', options.endTime);
    if (options?.deviceId) params.append('deviceId', options.deviceId);
    if (options?.vehicleType) params.append('vehicleType', options.vehicleType);
    if (options?.limit) params.append('limit', options.limit.toString());
    if (options?.offset) params.append('offset', options.offset.toString());
    const query = params.toString();
    return this.request<{ events: VehicleDetection[]; total: number; limit: number; offset: number }>(
      `/api/vcc/events${query ? `?${query}` : ''}`
    );
  }

  // ==================== Worker Management ====================

  // Admin: Get all workers
  async getWorkers(status?: WorkerStatus): Promise<WorkerWithCounts[]> {
    const params = new URLSearchParams();
    if (status) params.append('status', status);
    const query = params.toString();
    return this.request<WorkerWithCounts[]>(`/api/admin/workers${query ? `?${query}` : ''}`);
  }

  // Admin: Get single worker
  async getWorker(id: string): Promise<Worker> {
    return this.request<Worker>(`/api/admin/workers/${id}`);
  }

  // Admin: Update worker
  async updateWorker(id: string, updates: { name?: string; tags?: string[] }): Promise<Worker> {
    return this.request<Worker>(`/api/admin/workers/${id}`, {
      method: 'PUT',
      body: JSON.stringify(updates),
    });
  }

  // Admin: Revoke worker
  async revokeWorker(id: string): Promise<void> {
    return this.request<void>(`/api/admin/workers/${id}/revoke`, {
      method: 'POST',
    });
  }

  // Admin: Delete worker
  async deleteWorker(id: string): Promise<void> {
    return this.request<void>(`/api/admin/workers/${id}`, {
      method: 'DELETE',
    });
  }

  // Admin: Get pending approval requests
  async getApprovalRequests(status?: string): Promise<WorkerApprovalRequest[]> {
    const params = new URLSearchParams();
    if (status) params.append('status', status);
    const query = params.toString();
    return this.request<WorkerApprovalRequest[]>(`/api/admin/workers/approval-requests${query ? `?${query}` : ''}`);
  }

  // Admin: Approve worker request
  async approveWorkerRequest(id: string): Promise<{ message: string; worker_id: string }> {
    return this.request<{ message: string; worker_id: string }>(`/api/admin/workers/approval-requests/${id}/approve`, {
      method: 'POST',
    });
  }

  // Admin: Reject worker request
  async rejectWorkerRequest(id: string, reason?: string): Promise<void> {
    return this.request<void>(`/api/admin/workers/approval-requests/${id}/reject`, {
      method: 'POST',
      body: JSON.stringify({ reason }),
    });
  }

  // Admin: Assign cameras to worker
  async assignCamerasToWorker(workerId: string, assignments: CameraAssignment[]): Promise<Worker> {
    return this.request<Worker>(`/api/admin/workers/${workerId}/cameras`, {
      method: 'POST',
      body: JSON.stringify({ assignments }),
    });
  }

  // Admin: Get worker cameras
  async getWorkerCameras(workerId: string): Promise<WorkerCameraAssignment[]> {
    return this.request<WorkerCameraAssignment[]>(`/api/admin/workers/${workerId}/cameras`);
  }

  // Admin: Unassign camera from worker
  async unassignCameraFromWorker(workerId: string, deviceId: string): Promise<void> {
    return this.request<void>(`/api/admin/workers/${workerId}/cameras/${deviceId}`, {
      method: 'DELETE',
    });
  }

  // ==================== Worker Tokens ====================

  // Admin: Create worker token
  async createWorkerToken(data: { name: string; expires_in?: number; created_by?: string }): Promise<WorkerToken> {
    return this.request<WorkerToken>('/api/admin/worker-tokens', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  // Admin: Get all tokens
  async getWorkerTokens(options?: { show_used?: boolean; show_revoked?: boolean }): Promise<WorkerTokenWithStatus[]> {
    const params = new URLSearchParams();
    if (options?.show_used) params.append('show_used', 'true');
    if (options?.show_revoked) params.append('show_revoked', 'true');
    const query = params.toString();
    return this.request<WorkerTokenWithStatus[]>(`/api/admin/worker-tokens${query ? `?${query}` : ''}`);
  }

  // Admin: Revoke token
  async revokeWorkerToken(id: string): Promise<void> {
    return this.request<void>(`/api/admin/worker-tokens/${id}/revoke`, {
      method: 'POST',
    });
  }

  // Admin: Delete token
  async deleteWorkerToken(id: string): Promise<void> {
    return this.request<void>(`/api/admin/worker-tokens/${id}`, {
      method: 'DELETE',
    });
  }

  // Admin: Bulk create tokens
  async bulkCreateWorkerTokens(data: { count: number; prefix?: string; expires_in?: number }): Promise<{ tokens: WorkerToken[] }> {
    return this.request<{ tokens: WorkerToken[] }>('/api/admin/worker-tokens/bulk', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }
}

export const apiClient = new ApiClient(API_BASE_URL);

// Violation Types
export type ViolationType = 'SPEED' | 'HELMET' | 'WRONG_SIDE' | 'RED_LIGHT' | 'NO_SEATBELT' | 'OVERLOADING' | 'ILLEGAL_PARKING' | 'OTHER';
export type ViolationStatus = 'PENDING' | 'APPROVED' | 'REJECTED' | 'FINED';
export type DetectionMethod = 'RADAR' | 'CAMERA' | 'AI_VISION' | 'MANUAL';

export interface TrafficViolation {
  id: string;
  deviceId: string;
  device?: {
    id: string;
    name: string;
    lat: number;
    lng: number;
    type: DeviceType;
  };
  timestamp: string;
  violationType: ViolationType;
  status: ViolationStatus;
  detectionMethod: DetectionMethod;
  plateNumber?: string | null;
  plateConfidence?: number | null;
  plateImageUrl?: string | null;
  fullSnapshotUrl?: string | null;
  frameId?: string | null;
  detectedSpeed?: number | null;
  speedLimit2W?: number | null;
  speedLimit4W?: number | null;
  speedOverLimit?: number | null;
  confidence?: number | null;
  metadata?: any;
  reviewedAt?: string | null;
  reviewedBy?: string | null;
  reviewNote?: string | null;
  rejectionReason?: string | null;
  fineAmount?: number | null;
  fineIssuedAt?: string | null;
  fineReference?: string | null;
}

export interface ViolationStats {
  total: number;
  pending: number;
  approved: number;
  rejected: number;
  fined: number;
  byType: Record<string, number>;
  byDevice: Record<string, number>;
}

// Vehicle Types
export type VehicleType = '2W' | '4W' | 'AUTO' | 'TRUCK' | 'BUS' | 'UNKNOWN';

export interface Vehicle {
  id: string;
  plateNumber?: string | null;
  make?: string | null;
  model?: string | null;
  vehicleType: VehicleType;
  color?: string | null;
  firstSeen: string;
  lastSeen: string;
  detectionCount: number;
  isWatchlisted: boolean;
  metadata?: any;
  createdAt: string;
  updatedAt: string;
  watchlist?: Watchlist;
}

export interface VehicleDetection {
  id: string;
  vehicleId?: string | null;
  vehicle?: Vehicle;
  deviceId: string;
  device?: {
    id: string;
    name: string;
    lat: number;
    lng: number;
    type: DeviceType;
  };
  timestamp: string;
  plateNumber?: string | null;
  plateConfidence?: number | null;
  make?: string | null;
  model?: string | null;
  vehicleType: VehicleType;
  color?: string | null;
  confidence?: number | null;
  plateDetected: boolean;
  makeModelDetected: boolean;
  fullImageUrl?: string | null;
  plateImageUrl?: string | null;
  vehicleImageUrl?: string | null;
  frameId?: string | null;
  direction?: string | null;
  lane?: number | null;
  metadata?: any;
}

export interface Watchlist {
  id: string;
  vehicleId: string;
  vehicle?: Vehicle;
  reason: string;
  addedBy: string;
  addedAt: string;
  isActive: boolean;
  alertOnDetection: boolean;
  alertOnViolation: boolean;
  notes?: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface VehicleStats {
  total: number;
  withPlates: number;
  withoutPlates: number;
  watchlisted: number;
  byType: Record<string, number>;
  byMake: Record<string, number>;
  detectionsToday: number;
}

// VCC (Vehicle Classification and Counting) Types
export interface VCCStats {
  totalDetections: number;
  uniqueVehicles: number;
  byVehicleType: Record<string, number>;
  byTime: Array<{ hour?: string; day?: string; week?: string; month?: string; count: number }>;
  byDevice: Array<{
    deviceId: string;
    deviceName: string;
    totalDetections: number;
    byType: Record<string, number>;
  }>;
  byHour: Record<string, number>; // 0-23
  byDayOfWeek: Record<string, number>;
  peakHour: number;
  peakDay: string;
  averagePerHour: number;
  classification: {
    withPlates: number;
    withoutPlates: number;
    withMakeModel: number;
    plateOnly: number;
    fullClassification: number;
  };
}

export interface VCCDeviceStats {
  deviceId: string;
  deviceName: string;
  totalDetections: number;
  uniqueVehicles: number;
  byVehicleType: Record<string, number>;
  byTime: Array<{ hour?: string; day?: string; week?: string; month?: string; count: number }>;
  byHour: Record<string, number>;
  byDayOfWeek: Record<string, number>;
  peakHour: number;
  averagePerHour: number;
  classification: {
    withPlates: number;
    withoutPlates: number;
    withMakeModel: number;
    plateOnly: number;
    fullClassification: number;
  };
}

export interface VCCRealtime {
  totalDetections: number;
  byVehicleType: Record<string, number>;
  byDevice: Array<{ deviceId: string; deviceName: string; count: number }>;
  perMinute: number;
}

