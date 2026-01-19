import { useEffect, useRef, useState, useCallback } from 'react';
import { Camera, Wifi, WifiOff, AlertCircle } from 'lucide-react';

interface Detection {
  type: string;
  confidence: number;
  bbox?: [number, number, number, number]; // x, y, width, height
  label?: string;
  color?: string;
}

interface WebSocketVideoFrameProps {
  workerId: string;
  cameraId: string;
  showOverlays?: boolean;
  className?: string;
  onConnectionChange?: (connected: boolean) => void;
}

// Global WebSocket connection (shared across all video frames)
let globalWs: WebSocket | null = null;
let wsConnecting = false;
const wsSubscribers = new Map<string, Set<(data: ArrayBuffer | Detection[]) => void>>();
let reconnectTimeout: number | null = null;

function getWsUrl(): string {
  // Connect to backend WebSocket
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  // Backend is on port 3001
  return `${protocol}//${window.location.hostname}:3001/ws/feeds`;
}

function connectWebSocket() {
  if (globalWs?.readyState === WebSocket.OPEN || wsConnecting) {
    return;
  }

  wsConnecting = true;
  const ws = new WebSocket(getWsUrl());
  ws.binaryType = 'arraybuffer';

  ws.onopen = () => {
    console.log('📺 WebSocket connected to feed hub');
    globalWs = ws;
    wsConnecting = false;

    // Re-subscribe to all cameras
    wsSubscribers.forEach((_, cameraKey) => {
      ws.send(JSON.stringify({ type: 'subscribe', camera: cameraKey }));
    });
  };

  ws.onmessage = (event) => {
    if (event.data instanceof ArrayBuffer) {
      // Binary frame message
      // Format: [1 byte type][1 byte key length][camera key][frame data]
      const data = new Uint8Array(event.data);
      if (data[0] !== 0x01) return; // Not a frame

      const keyLength = data[1];
      const cameraKey = new TextDecoder().decode(data.slice(2, 2 + keyLength));
      const frameData = data.slice(2 + keyLength);

      // Dispatch to subscribers
      const handlers = wsSubscribers.get(cameraKey);
      if (handlers) {
        handlers.forEach(handler => handler(frameData.buffer));
      }
    } else {
      // Text JSON message (detections, errors, etc.)
      try {
        const msg = JSON.parse(event.data);
        if (msg.type === 'detection' && msg.camera) {
          const handlers = wsSubscribers.get(msg.camera);
          if (handlers) {
            handlers.forEach(handler => handler(msg.data));
          }
        } else if (msg.type === 'error') {
          console.error('Feed hub error:', msg.error);
        }
      } catch (e) {
        console.error('Failed to parse WebSocket message:', e);
      }
    }
  };

  ws.onclose = () => {
    console.log('📺 WebSocket disconnected');
    globalWs = null;
    wsConnecting = false;

    // Reconnect after delay
    if (reconnectTimeout) clearTimeout(reconnectTimeout);
    reconnectTimeout = window.setTimeout(() => {
      if (wsSubscribers.size > 0) {
        connectWebSocket();
      }
    }, 3000);
  };

  ws.onerror = (error) => {
    console.error('📺 WebSocket error:', error);
    wsConnecting = false;
  };
}

function subscribe(cameraKey: string, handler: (data: ArrayBuffer | Detection[]) => void) {
  if (!wsSubscribers.has(cameraKey)) {
    wsSubscribers.set(cameraKey, new Set());
  }
  wsSubscribers.get(cameraKey)!.add(handler);

  // Connect if not connected
  connectWebSocket();

  // Send subscribe message if connected
  if (globalWs?.readyState === WebSocket.OPEN) {
    globalWs.send(JSON.stringify({ type: 'subscribe', camera: cameraKey }));
  }
}

function unsubscribe(cameraKey: string, handler: (data: ArrayBuffer | Detection[]) => void) {
  const handlers = wsSubscribers.get(cameraKey);
  if (handlers) {
    handlers.delete(handler);
    if (handlers.size === 0) {
      wsSubscribers.delete(cameraKey);
      
      // Send unsubscribe message
      if (globalWs?.readyState === WebSocket.OPEN) {
        globalWs.send(JSON.stringify({ type: 'unsubscribe', camera: cameraKey }));
      }
    }
  }

  // Disconnect if no more subscribers
  if (wsSubscribers.size === 0 && globalWs) {
    globalWs.close();
    globalWs = null;
  }
}

export function WebSocketVideoFrame({
  workerId,
  cameraId,
  showOverlays = true,
  className = '',
  onConnectionChange,
}: WebSocketVideoFrameProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const [connected, setConnected] = useState(false);
  const [lastFrameTime, setLastFrameTime] = useState<number>(0);
  const [fps, setFps] = useState<number>(0);
  const [detections, setDetections] = useState<Detection[]>([]);
  const [error, setError] = useState<string | null>(null);
  const frameCountRef = useRef(0);
  const lastFpsUpdateRef = useRef(Date.now());

  const cameraKey = `${workerId}.${cameraId}`;

  // Handle incoming data (frames or detections)
  const handleData = useCallback((data: ArrayBuffer | Detection[]) => {
    if (data instanceof ArrayBuffer) {
      // Frame data
      const blob = new Blob([data], { type: 'image/jpeg' });
      const url = URL.createObjectURL(blob);
      const img = new Image();
      
      img.onload = () => {
        const canvas = canvasRef.current;
        if (canvas) {
          const ctx = canvas.getContext('2d');
          if (ctx) {
            // Resize canvas to match image
            if (canvas.width !== img.width || canvas.height !== img.height) {
              canvas.width = img.width;
              canvas.height = img.height;
            }
            
            // Draw frame
            ctx.drawImage(img, 0, 0);
            
            // Draw detection overlays
            if (showOverlays && detections.length > 0) {
              drawDetections(ctx, detections, img.width, img.height);
            }
          }
        }
        URL.revokeObjectURL(url);
        
        // Update stats
        setLastFrameTime(Date.now());
        setConnected(true);
        setError(null);
        
        // Calculate FPS
        frameCountRef.current++;
        const now = Date.now();
        if (now - lastFpsUpdateRef.current >= 1000) {
          setFps(frameCountRef.current);
          frameCountRef.current = 0;
          lastFpsUpdateRef.current = now;
        }
      };
      
      img.onerror = () => {
        URL.revokeObjectURL(url);
        setError('Failed to decode frame');
      };
      
      img.src = url;
    } else {
      // Detection data - can be array or object with detections property
      if (Array.isArray(data)) {
        setDetections(data as Detection[]);
      } else if (data && typeof data === 'object' && 'detections' in data) {
        // Handle YOLO worker format: { camera_id, timestamp, detections: [...] }
        setDetections((data as { detections: Detection[] }).detections);
      }
    }
  }, [showOverlays, detections]);

  // Subscribe/unsubscribe on mount/unmount
  useEffect(() => {
    subscribe(cameraKey, handleData);
    
    return () => {
      unsubscribe(cameraKey, handleData);
    };
  }, [cameraKey, handleData]);

  // Check for stale connection
  useEffect(() => {
    const interval = setInterval(() => {
      if (lastFrameTime && Date.now() - lastFrameTime > 5000) {
        setConnected(false);
        setError('No frames received');
      }
    }, 1000);
    
    return () => clearInterval(interval);
  }, [lastFrameTime]);

  // Notify parent of connection changes
  useEffect(() => {
    onConnectionChange?.(connected);
  }, [connected, onConnectionChange]);

  return (
    <div className={`relative w-full h-full bg-gray-900 ${className}`}>
      <canvas
        ref={canvasRef}
        className="w-full h-full object-contain"
      />
      
      {/* Status overlay */}
      <div className="absolute top-2 right-2 flex items-center gap-2">
        {connected ? (
          <div className="bg-green-500/80 rounded-full px-2 py-0.5 flex items-center gap-1">
            <Wifi className="w-3 h-3 text-white" />
            <span className="text-xs text-white font-medium">{fps} fps</span>
          </div>
        ) : (
          <div className="bg-red-500/80 rounded-full px-2 py-0.5 flex items-center gap-1">
            <WifiOff className="w-3 h-3 text-white" />
            <span className="text-xs text-white font-medium">Offline</span>
          </div>
        )}
      </div>

      {/* Error/waiting overlay */}
      {!connected && (
        <div className="absolute inset-0 flex items-center justify-center bg-gray-900/80">
          <div className="text-center">
            {error ? (
              <>
                <AlertCircle className="w-12 h-12 text-red-400 mx-auto mb-2" />
                <p className="text-sm text-red-400">{error}</p>
              </>
            ) : (
              <>
                <Camera className="w-12 h-12 text-gray-500 mx-auto mb-2 animate-pulse" />
                <p className="text-sm text-gray-400">Waiting for stream...</p>
                <p className="text-xs text-gray-500 mt-1">{cameraKey}</p>
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

// Draw detection bounding boxes and labels
function drawDetections(
  ctx: CanvasRenderingContext2D,
  detections: Detection[],
  width: number,
  height: number
) {
  detections.forEach((det) => {
    if (!det.bbox) return;
    
    const [x, y, w, h] = det.bbox;
    const color = det.color || getColorForType(det.type);
    
    // Draw box
    ctx.strokeStyle = color;
    ctx.lineWidth = 2;
    ctx.strokeRect(x, y, w, h);
    
    // Draw label background
    const label = det.label || `${det.type} ${Math.round(det.confidence * 100)}%`;
    ctx.font = '12px sans-serif';
    const textWidth = ctx.measureText(label).width;
    ctx.fillStyle = color;
    ctx.fillRect(x, y - 18, textWidth + 8, 18);
    
    // Draw label text
    ctx.fillStyle = '#fff';
    ctx.fillText(label, x + 4, y - 5);
  });
}

// Get color for detection type
function getColorForType(type: string): string {
  const colors: Record<string, string> = {
    person: '#22c55e',
    vehicle: '#3b82f6',
    car: '#3b82f6',
    truck: '#8b5cf6',
    bus: '#f59e0b',
    motorcycle: '#ec4899',
    bicycle: '#14b8a6',
    plate: '#ef4444',
    face: '#f97316',
    default: '#6b7280',
  };
  return colors[type.toLowerCase()] || colors.default;
}

export default WebSocketVideoFrame;

