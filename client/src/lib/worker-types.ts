// Worker Types - Separate file to avoid import issues
import type { Device } from './api';

export type WorkerStatus = 'pending' | 'approved' | 'active' | 'offline' | 'revoked';

export interface WorkerCameraAssignment {
  id: number;
  workerId: string;
  deviceId: string;
  device?: Device;
  analytics: string[];
  fps: number;
  resolution: string;
  isActive: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface CameraAssignment {
  device_id: string;
  analytics: string[];
  fps?: number;
  resolution?: string;
}

export interface Worker {
  id: string;
  name: string;
  status: WorkerStatus;
  ip: string;
  mac: string;
  model: string;
  version?: string | null;
  approvedAt?: string | null;
  approvedBy?: string | null;
  lastSeen: string;
  lastIp?: string | null;
  resources?: {
    cpu_percent?: number;
    gpu_percent?: number;
    memory_mb?: number;
    temperature_c?: number;
  } | null;
  config?: any;
  configVersion: number;
  metadata?: any;
  tags?: string[] | null;
  createdAt: string;
  updatedAt: string;
  cameraAssignments?: WorkerCameraAssignment[];
}

export interface WorkerWithCounts extends Worker {
  cameraCount: number;
}

export interface WorkerToken {
  id: string;
  token: string;
  name: string;
  usedBy?: string | null;
  usedAt?: string | null;
  expiresAt?: string | null;
  isRevoked: boolean;
  createdBy: string;
  createdAt: string;
}

export interface WorkerTokenWithStatus extends WorkerToken {
  status: 'active' | 'used' | 'expired' | 'revoked';
}

export interface WorkerApprovalRequest {
  id: string;
  deviceName: string;
  ip: string;
  mac: string;
  model: string;
  status: 'pending' | 'approved' | 'rejected';
  workerId?: string | null;
  rejectedBy?: string | null;
  rejectedAt?: string | null;
  rejectReason?: string | null;
  createdAt: string;
  updatedAt: string;
}

