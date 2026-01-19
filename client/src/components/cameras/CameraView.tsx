import { useState, useEffect, useMemo, useRef } from 'react';
import { apiClient, type Device } from '@/lib/api';
import { Camera, ChevronDown, ChevronRight, LayoutGrid, Maximize2, Minimize2 } from 'lucide-react';
import { useCameraGrid } from '@/contexts/CameraGridContext';
import { cn } from '@/lib/utils';
import Hls from 'hls.js';
import { WebSocketVideoFrame } from './WebSocketVideoFrame';
import { useTheme } from '@/contexts/ThemeContext';

// VideoFrame component with native HLS support (works on web and iOS/iPad)
function VideoFrame({ src, fullscreenToggleRef, onFullscreenChange }: { src: string; fullscreenToggleRef?: React.MutableRefObject<(() => void) | undefined>; onFullscreenChange?: (isFullscreen: boolean) => void }) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const hlsRef = useRef<Hls | null>(null);
  const [isPlaying, setIsPlaying] = useState(false);
  const [hasError, setHasError] = useState(false);

  // HLS stream URL
  const hlsUrl = `${src}/index.m3u8`;

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    // Check if browser supports native HLS (Safari/iOS)
    const isNativeHlsSupported = video.canPlayType('application/vnd.apple.mpegurl');

    if (isNativeHlsSupported) {
      // Native HLS support (Safari/iOS)
      video.src = hlsUrl;
      video.addEventListener('loadedmetadata', () => {
        // Try autoplay (muted for iOS)
        video.muted = true;
        video.play().catch(() => {
          // Autoplay blocked, will need user interaction
        });
      });
    } else if (Hls.isSupported()) {
      // Use hls.js for browsers that don't support native HLS
      const hls = new Hls({
        enableWorker: true,
        lowLatencyMode: true,
        backBufferLength: 90,
      });

      hlsRef.current = hls;
      hls.loadSource(hlsUrl);
      hls.attachMedia(video);

      hls.on(Hls.Events.MANIFEST_PARSED, () => {
        // Try autoplay (muted for autoplay policy)
        video.muted = true;
        video.play().catch(() => {
          // Autoplay blocked, will need user interaction
        });
      });

      hls.on(Hls.Events.ERROR, (_event, data) => {
        if (data.fatal) {
          switch (data.type) {
            case Hls.ErrorTypes.NETWORK_ERROR:
              console.error('HLS network error, trying to recover...');
              hls.startLoad();
              break;
            case Hls.ErrorTypes.MEDIA_ERROR:
              console.error('HLS media error, trying to recover...');
              hls.recoverMediaError();
              break;
            default:
              console.error('HLS fatal error, destroying...');
              hls.destroy();
              setHasError(true);
              break;
          }
        }
      });
    } else {
      setHasError(true);
    }

    return () => {
      if (hlsRef.current) {
        hlsRef.current.destroy();
      }
    };
  }, [hlsUrl]);

  // Handle user interaction to enable autoplay
  const handleInteraction = async (e: React.MouseEvent | React.TouchEvent) => {
    e.preventDefault();
    e.stopPropagation();
    const video = videoRef.current;
    if (video && !isPlaying) {
      try {
        video.muted = true; // Muted for autoplay policy
        await video.play();
        setIsPlaying(true);
      } catch (err) {
        console.error('Failed to play video:', err);
      }
    }
  };

  // Track playing state
  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    const handlePlay = () => setIsPlaying(true);
    const handlePause = () => setIsPlaying(false);

    video.addEventListener('play', handlePlay);
    video.addEventListener('pause', handlePause);

    return () => {
      video.removeEventListener('play', handlePlay);
      video.removeEventListener('pause', handlePause);
    };
  }, []);

  // Track fullscreen state
  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const handleFullscreenChange = () => {
      const isFs = !!document.fullscreenElement;
      if (onFullscreenChange) {
        onFullscreenChange(isFs);
      }
    };

    document.addEventListener('fullscreenchange', handleFullscreenChange);
    document.addEventListener('webkitfullscreenchange', handleFullscreenChange);
    document.addEventListener('mozfullscreenchange', handleFullscreenChange);
    document.addEventListener('MSFullscreenChange', handleFullscreenChange);

    return () => {
      document.removeEventListener('fullscreenchange', handleFullscreenChange);
      document.removeEventListener('webkitfullscreenchange', handleFullscreenChange);
      document.removeEventListener('mozfullscreenchange', handleFullscreenChange);
      document.removeEventListener('MSFullscreenChange', handleFullscreenChange);
    };
  }, []);

  // Toggle fullscreen
  const toggleFullscreen = async () => {
    const container = containerRef.current;
    if (!container) return;

    try {
      if (!document.fullscreenElement) {
        // Enter fullscreen
        if (container.requestFullscreen) {
          await container.requestFullscreen();
        } else if ((container as any).webkitRequestFullscreen) {
          await (container as any).webkitRequestFullscreen();
        } else if ((container as any).mozRequestFullScreen) {
          await (container as any).mozRequestFullScreen();
        } else if ((container as any).msRequestFullscreen) {
          await (container as any).msRequestFullscreen();
        }
      } else {
        // Exit fullscreen
        if (document.exitFullscreen) {
          await document.exitFullscreen();
        } else if ((document as any).webkitExitFullscreen) {
          await (document as any).webkitExitFullscreen();
        } else if ((document as any).mozCancelFullScreen) {
          await (document as any).mozCancelFullScreen();
        } else if ((document as any).msExitFullscreen) {
          await (document as any).msExitFullscreen();
        }
      }
    } catch (err) {
      console.error('Error toggling fullscreen:', err);
    }
  };

  // Expose fullscreen toggle to parent
  useEffect(() => {
    if (fullscreenToggleRef) {
      fullscreenToggleRef.current = toggleFullscreen;
    }
  }, [fullscreenToggleRef]);

  return (
    <div
      ref={containerRef}
      className="w-full h-full relative flex flex-col"
    >
      {hasError && (
        <div className="absolute inset-0 z-20 flex items-center justify-center bg-red-900/50 rounded-lg">
          <div className="text-center">
            <Camera className="w-12 h-12 text-red-400 mx-auto mb-2" />
            <p className="text-xs text-red-400">Stream unavailable</p>
          </div>
        </div>
      )}
      <video
        ref={videoRef}
        className="w-full h-full flex-1 object-cover cursor-pointer"
        playsInline
        muted
        autoPlay={false}
        controls={false}
        onClick={handleInteraction}
        onTouchStart={handleInteraction}
      />
    </div>
  );
}

interface GridSlot {
  id: string | null;
  deviceId: string | null;
  device: Device | null;
  fullscreenToggleRef?: React.MutableRefObject<(() => void) | undefined>;
}

export function CameraView() {
  const { gridSize, setUsedSlots } = useCameraGrid();
  const [cameras, setCameras] = useState<Device[]>([]);
  const [zones, setZones] = useState<Record<string, Device[]>>({});
  const [fullscreenStates, setFullscreenStates] = useState<Record<number, boolean>>({});
  const [expandedZones, setExpandedZones] = useState<Set<string>>(new Set());
  const [gridSlots, setGridSlots] = useState<GridSlot[]>([]);
  const [draggedDevice, setDraggedDevice] = useState<Device | null>(null);
  const [loading, setLoading] = useState(true);
  const [touchStartPos, setTouchStartPos] = useState<{ x: number; y: number } | null>(null);
  const [touchDraggedDevice, setTouchDraggedDevice] = useState<Device | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const { theme } = useTheme();

  // Get MediaMTX server URL (proxied through /media)
  const mediaServerUrl = useMemo(() => {
    // Use relative path to proxy through Vite (dev) or Express (prod)
    return '/media';
  }, []);

  // Parse grid size (format: colsxrows, e.g., 2x3 = 2 columns, 3 rows)
  const gridDimensions = useMemo(() => {
    const [cols, rows] = gridSize.split('x').map(Number);
    return { rows, cols, total: rows * cols };
  }, [gridSize]);

  // Initialize grid slots when grid size changes
  useEffect(() => {
    try {
      const savedGridState = localStorage.getItem('cameraGridState');
      if (savedGridState) {
        const parsed = JSON.parse(savedGridState);
        // Only load if grid size matches
        if (parsed.gridSize === gridSize && parsed.slots && parsed.slots.length === gridDimensions.total) {
          // We'll restore devices after cameras are loaded
          const savedSlots = parsed.slots as Array<{ id: string; deviceId: string | null }>;
          setGridSlots(savedSlots.map((s, i) => ({
            id: s.id || `slot-${i}`,
            deviceId: s.deviceId,
            device: null, // Will be restored after cameras load
            fullscreenToggleRef: { current: undefined },
          })));
          return;
        }
      }
    } catch (err) {
      console.error('Failed to load grid state from localStorage:', err);
    }

    // Initialize empty slots if no saved state or grid size changed
    const slots: GridSlot[] = Array.from({ length: gridDimensions.total }, (_, i) => ({
      id: `slot-${i}`,
      deviceId: null,
      device: null,
      fullscreenToggleRef: { current: undefined },
    }));
    setGridSlots(slots);
  }, [gridSize, gridDimensions.total]);

  // Fetch cameras and restore grid state
  useEffect(() => {
    const fetchCameras = async () => {
      try {
        setLoading(true);
        const devices = await apiClient.getDevices({ type: 'CAMERA' }) as Device[];
        setCameras(devices);

        // Group by zone
        const zonesMap: Record<string, Device[]> = {};
        devices.forEach((device) => {
          const zone = device.zoneId || 'Unassigned';
          if (!zonesMap[zone]) {
            zonesMap[zone] = [];
          }
          zonesMap[zone].push(device);
        });
        setZones(zonesMap);

        // Expand all zones by default
        setExpandedZones(new Set(Object.keys(zonesMap)));

        // Restore devices in grid slots from localStorage
        try {
          const savedGridState = localStorage.getItem('cameraGridState');
          if (savedGridState) {
            const parsed = JSON.parse(savedGridState);
            if (parsed.gridSize === gridSize && parsed.slots && parsed.slots.length === gridDimensions.total) {
              setGridSlots((prevSlots) => {
                // Match saved slots with current slots by index
                return prevSlots.map((slot, index) => {
                  const savedSlot = parsed.slots[index];
                  if (savedSlot && savedSlot.deviceId) {
                    const device = devices.find((d) => d.id === savedSlot.deviceId);
                    return {
                      id: slot.id,
                      deviceId: savedSlot.deviceId,
                      device: device || null,
                      fullscreenToggleRef: slot.fullscreenToggleRef || { current: undefined },
                    };
                  }
                  return slot;
                });
              });
            }
          }
        } catch (err) {
          console.error('Failed to restore grid state:', err);
        }
      } catch (err) {
        console.error('Failed to fetch cameras:', err);
      } finally {
        setLoading(false);
      }
    };

    fetchCameras();
  }, [gridSize]);

  const toggleZone = (zoneId: string) => {
    setExpandedZones((prev) => {
      const next = new Set(prev);
      if (next.has(zoneId)) {
        next.delete(zoneId);
      } else {
        next.add(zoneId);
      }
      return next;
    });
  };

  const handleDragStart = (device: Device) => {
    setDraggedDevice(device);
  };

  const handleDragEnd = () => {
    setDraggedDevice(null);
  };

  const handleDrop = (slotIndex: number, device?: Device) => {
    const deviceToAdd = device || draggedDevice;
    if (!deviceToAdd) return;

    setGridSlots((prev) => {
      const newSlots = [...prev];
      // Remove device from previous slot if it exists
      const prevSlotIndex = newSlots.findIndex((s) => s.deviceId === deviceToAdd.id);
      if (prevSlotIndex !== -1) {
        newSlots[prevSlotIndex] = {
          id: newSlots[prevSlotIndex].id,
          deviceId: null,
          device: null,
          fullscreenToggleRef: newSlots[prevSlotIndex].fullscreenToggleRef || { current: undefined },
        };
      }
      // Add device to new slot
      newSlots[slotIndex] = {
        id: newSlots[slotIndex].id,
        deviceId: deviceToAdd.id,
        device: deviceToAdd,
        fullscreenToggleRef: newSlots[slotIndex].fullscreenToggleRef || { current: undefined },
      };
      // Update used slots count
      const usedCount = newSlots.filter((s) => s.device).length;
      setUsedSlots(usedCount);

      // Save to localStorage
      try {
        localStorage.setItem('cameraGridState', JSON.stringify({
          gridSize,
          slots: newSlots.map((s) => ({
            id: s.id,
            deviceId: s.deviceId,
          })),
        }));
      } catch (err) {
        console.error('Failed to save grid state to localStorage:', err);
      }

      return newSlots;
    });

    setDraggedDevice(null);
  };

  // Tap to add camera to next available slot
  const handleCameraTap = (device: Device) => {
    // Find first available slot
    const availableSlotIndex = gridSlots.findIndex((slot) => !slot.device);

    if (availableSlotIndex !== -1) {
      handleDrop(availableSlotIndex, device);
    } else {
      // No available slots - could show a message or replace the first one
      // For now, just replace the first slot
      handleDrop(0, device);
    }
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
  };

  // Touch event handlers for iPad/mobile support
  const handleTouchStart = (e: React.TouchEvent, device: Device) => {
    const touch = e.touches[0];
    setTouchStartPos({ x: touch.clientX, y: touch.clientY });
    setTouchDraggedDevice(device);
    setIsDragging(false);
    // Don't preventDefault here - allow normal taps to work
  };

  const handleTouchMove = (e: React.TouchEvent) => {
    if (!touchStartPos || !touchDraggedDevice) {
      // Not dragging, allow normal behavior
      return;
    }

    const touch = e.touches[0];
    if (!touch) return;

    const deltaX = Math.abs(touch.clientX - touchStartPos.x);
    const deltaY = Math.abs(touch.clientY - touchStartPos.y);
    const dragThreshold = 10; // pixels

    // Only start dragging if moved more than threshold
    if (deltaX > dragThreshold || deltaY > dragThreshold) {
      if (!isDragging) {
        setIsDragging(true);
      }
      // Prevent scrolling while dragging
      e.preventDefault();
      e.stopPropagation();

      // Find the element under the touch point
      const elementUnderTouch = document.elementFromPoint(touch.clientX, touch.clientY);

      // Check if we're over a grid slot
      if (elementUnderTouch) {
        const slotElement = elementUnderTouch.closest('[data-slot-index]');
        if (slotElement) {
          const slotIndex = parseInt(slotElement.getAttribute('data-slot-index') || '-1');
          if (slotIndex >= 0) {
            // Visual feedback - highlight the slot
            slotElement.classList.add('border-blue-400', 'border-solid', 'bg-blue-50/50', 'dark:bg-blue-900/20');
          }
        }
      }
    } else {
      // Small movement, might be a tap - don't prevent default
      // This allows normal tap behavior
    }
  };

  const handleTouchEnd = (_e?: React.TouchEvent, slotIndex?: number) => {
    if (!touchDraggedDevice) {
      setTouchStartPos(null);
      setTouchDraggedDevice(null);
      setIsDragging(false);
      return;
    }

    // Only drop if we were actually dragging (not just a tap)
    if (isDragging && slotIndex !== undefined) {
      // Drop on a slot
      handleDrop(slotIndex);
    }

    setTouchStartPos(null);
    setTouchDraggedDevice(null);
    setIsDragging(false);
  };

  const handleTouchCancel = () => {
    setTouchStartPos(null);
    setTouchDraggedDevice(null);
    setIsDragging(false);
  };

  const removeFromGrid = (slotIndex: number) => {
    setGridSlots((prev) => {
      const newSlots = [...prev];
      newSlots[slotIndex] = {
        id: newSlots[slotIndex].id,
        deviceId: null,
        device: null,
        fullscreenToggleRef: newSlots[slotIndex].fullscreenToggleRef || { current: undefined },
      };
      // Update used slots count
      const usedCount = newSlots.filter((s) => s.device).length;
      setUsedSlots(usedCount);

      // Save to localStorage
      try {
        localStorage.setItem('cameraGridState', JSON.stringify({
          gridSize,
          slots: newSlots.map((s) => ({
            id: s.id,
            deviceId: s.deviceId,
          })),
        }));
      } catch (err) {
        console.error('Failed to save grid state to localStorage:', err);
      }

      return newSlots;
    });
  };

  // Update used slots count when grid slots change
  useEffect(() => {
    const usedCount = gridSlots.filter((s) => s.device).length;
    setUsedSlots(usedCount);
  }, [gridSlots, setUsedSlots]);

  const sortedZones = useMemo(() => {
    return Object.keys(zones).sort((a, b) => {
      if (a === 'Unassigned') return 1;
      if (b === 'Unassigned') return -1;
      return a.localeCompare(b);
    });
  }, [zones]);

  return (
    <div className="h-full flex">
      {/* Sidebar - Zone and Camera List */}
      <div className={cn(
        "w-64 border-r overflow-y-auto",
        theme === 'light' ? 'bg-white border-gray-200' : 'bg-gray-900/50 border-white/5'
      )}>
        <div className={cn(
          "p-4 border-b",
          theme === 'light' ? 'border-gray-200' : 'border-white/5'
        )}>
          <h2 className={cn(
            "text-sm font-semibold flex items-center gap-2",
            theme === 'light' ? 'text-black' : 'text-white'
          )}>
            <Camera className="w-4 h-4" />
            Cameras ({cameras.length})
          </h2>
        </div>

        {loading ? (
          <div className="p-4 text-sm text-gray-500 dark:text-gray-400">Loading cameras...</div>
        ) : (
          <div className="p-2 space-y-1">
            {sortedZones.map((zoneId) => {
              const zoneCameras = zones[zoneId];
              const isExpanded = expandedZones.has(zoneId);

              return (
                <div key={zoneId} className="mb-1">
                  <button
                    onClick={() => toggleZone(zoneId)}
                    className={cn(
                      "w-full flex items-center justify-between px-3 py-2 rounded-lg transition-colors text-left",
                      theme === 'light' ? 'hover:bg-gray-100' : 'hover:bg-white/5'
                    )}
                  >
                    <div className="flex items-center gap-2">
                      {isExpanded ? (
                        <ChevronDown className="w-4 h-4 text-gray-500" />
                      ) : (
                        <ChevronRight className="w-4 h-4 text-gray-500" />
                      )}
                      <span className={cn(
                        "text-sm font-medium",
                        theme === 'light' ? 'text-black' : 'text-gray-300'
                      )}>
                        {zoneId}
                      </span>
                      <span className="text-xs text-gray-500 dark:text-gray-400">
                        ({zoneCameras.length})
                      </span>
                    </div>
                  </button>

                  {isExpanded && (
                    <div className="ml-6 mt-1 space-y-1">
                      {zoneCameras.map((camera) => {
                        const isInGrid = gridSlots.some((s) => s.deviceId === camera.id);
                        return (
                          <div
                            key={camera.id}
                            draggable={!('ontouchstart' in window)}
                            onDragStart={() => handleDragStart(camera)}
                            onDragEnd={handleDragEnd}
                            onTouchStart={(e) => {
                              // Only handle touch if not a quick tap
                              handleTouchStart(e, camera);
                            }}
                            onTouchMove={handleTouchMove}
                            onTouchEnd={(e) => {
                              // If it was just a tap (not a drag), add to grid
                              if (!isDragging) {
                                e.preventDefault();
                                handleCameraTap(camera);
                                setTouchStartPos(null);
                                setTouchDraggedDevice(null);
                                return;
                              }
                              handleTouchEnd(e);
                            }}
                            onTouchCancel={handleTouchCancel}
                            onClick={(e) => {
                              // Handle click/tap - add to grid
                              if (!isDragging && !touchDraggedDevice) {
                                e.preventDefault();
                                e.stopPropagation();
                                handleCameraTap(camera);
                              }
                            }}
                            className={cn(
                              "px-3 py-2 rounded-lg cursor-pointer transition-all select-none",
                              "border flex items-center gap-2",
                              theme === 'light'
                                ? 'bg-white border-gray-200 hover:bg-gray-100'
                                : 'bg-white/5 border-white/10 hover:bg-white/10',
                              "hover:shadow-sm active:scale-95",
                              isInGrid && "opacity-50"
                            )}
                            style={{
                              WebkitUserSelect: 'none',
                              userSelect: 'none',
                              WebkitTouchCallout: 'none',
                              WebkitTapHighlightColor: 'transparent',
                              touchAction: 'manipulation'
                            }}
                          >
                            <Camera className="w-3.5 h-3.5 text-blue-500 flex-shrink-0" />
                            <span className={cn(
                              "text-xs truncate flex-1",
                              theme === 'light' ? 'text-black' : 'text-gray-300'
                            )}>
                              {(camera.name || camera.id).replace(/^Camera\s+/i, "")}
                            </span>
                            {isInGrid && (
                              <span className="text-xs text-green-500">✓</span>
                            )}
                          </div>
                        );
                      })}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Main Grid Area */}
      <div className="flex-1 flex flex-col overflow-hidden">
        {/* Grid Container */}
        <div className="flex-1 p-4 overflow-auto">
          <div
            className="grid gap-2 h-full"
            style={{
              gridTemplateColumns: `repeat(${gridDimensions.cols}, minmax(0, 1fr))`,
              gridTemplateRows: `repeat(${gridDimensions.rows}, minmax(0, 1fr))`,
            }}
          >
            {gridSlots.map((slot, index) => (
              <div
                key={slot.id}
                data-slot-index={index}
                onDragOver={handleDragOver}
                onDrop={() => handleDrop(index)}
                onTouchEnd={(e) => {
                  if (touchDraggedDevice) {
                    e.preventDefault();
                    handleTouchEnd(undefined, index);
                  }
                }}
                onTouchMove={(e) => {
                  if (touchDraggedDevice) {
                    e.preventDefault();
                  }
                }}
                className={cn(
                  "relative rounded-lg border-2 border-dashed transition-all select-none overflow-hidden flex flex-col",
                  slot.device
                    ? "border-blue-500/50 bg-gray-900/90 dark:bg-gray-800/90"
                    : "border-gray-300 dark:border-gray-700 bg-gray-100/50 dark:bg-gray-900/30",
                  (draggedDevice || touchDraggedDevice) && !slot.device && "border-blue-400 border-solid bg-blue-50/50 dark:bg-blue-900/20"
                )}
                style={{
                  WebkitUserSelect: 'none',
                  userSelect: 'none',
                  touchAction: 'none'
                }}
              >
                {slot.device ? (
                  <>
                    <div className="absolute top-2 left-2 z-10">
                      <div className="bg-black/70 rounded px-2 py-1">
                        <p className="text-xs text-white font-medium truncate max-w-[200px]">
                          {(slot.device.name || slot.device.id).replace(/^Camera\s+/i, "")}
                        </p>
                      </div>
                    </div>
                    <div className="flex-1 relative">
                      {/* Video stream - use WebSocket if workerId available, else HLS */}
                      {slot.device.workerId ? (
                        <WebSocketVideoFrame
                          workerId={slot.device.workerId}
                          cameraId={slot.device.id}
                          showOverlays={true}
                          className="w-full h-full"
                        />
                      ) : (
                        <VideoFrame
                          src={(() => {
                            // Helper to construct video source
                            if (slot.device.rtspUrl) {
                              try {
                                const url = new URL(slot.device.rtspUrl);
                                const path = url.pathname;
                                return `${mediaServerUrl}${path}`;
                              } catch (e) {
                                console.warn('Invalid RTSP URL:', slot.device.rtspUrl);
                              }
                            }
                            // Fallback to convention
                            return `${mediaServerUrl}/camera_${slot.device.id}`;
                          })()}
                          fullscreenToggleRef={slot.fullscreenToggleRef}
                          onFullscreenChange={(isFs) => setFullscreenStates(prev => ({ ...prev, [index]: isFs }))}
                        />
                      )}
                    </div>
                    {/* Footer with controls */}
                    <div className="absolute bottom-0 left-0 right-0 z-10 bg-black/70 backdrop-blur-sm flex items-center justify-between px-2 py-1.5">
                      <span className="text-xs text-white/90 truncate flex-1">
                        {(slot.device.name || slot.device.id).replace(/^Camera\s+/i, "")}
                      </span>
                      <div className="flex items-center gap-2">
                        <button
                          onClick={() => {
                            if (slot.fullscreenToggleRef?.current) {
                              slot.fullscreenToggleRef.current();
                            }
                          }}
                          className="bg-white/20 hover:bg-white/30 rounded p-1 transition-colors"
                          title={fullscreenStates[index] ? "Exit fullscreen" : "Enter fullscreen"}
                        >
                          {fullscreenStates[index] ? (
                            <Minimize2 className="w-3.5 h-3.5 text-white" />
                          ) : (
                            <Maximize2 className="w-3.5 h-3.5 text-white" />
                          )}
                        </button>
                        <button
                          onClick={() => removeFromGrid(index)}
                          className="bg-red-500/80 hover:bg-red-600 rounded p-1 transition-colors"
                          title="Remove from grid"
                        >
                          <span className="text-white text-xs">×</span>
                        </button>
                      </div>
                    </div>
                  </>
                ) : (
                  <div className="w-full h-full flex items-center justify-center">
                    <div className="text-center">
                      <LayoutGrid className="w-8 h-8 text-gray-400 mx-auto mb-2" />
                      <p className="text-xs text-gray-500 dark:text-gray-400">
                        Drop camera here
                      </p>
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>

      </div>
    </div>
  );
}

