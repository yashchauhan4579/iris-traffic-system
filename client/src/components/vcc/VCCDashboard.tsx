import { useState, useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { apiClient, type VCCStats, type VCCRealtime, type VCCDeviceStats } from '@/lib/api';
import { TrendingUp, Car, Clock, BarChart3, Loader2, RefreshCw, Activity, ArrowLeft } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { cn } from '@/lib/utils';
import { DateTimeRangePicker, type DateTimeRange } from '@/components/vcc/DateTimeRangePicker';
import { CameraSelector, type CameraOption } from '@/components/vcc/CameraSelector';
import { VCCInsights } from '@/components/vcc/VCCInsights';
import { VCCHeatmap } from '@/components/vcc/VCCHeatmap';
import { VCCDevicesView } from '@/components/vcc/VCCDevicesView';
import { VCCReportModal } from '@/components/vcc/VCCReportModal';
import { LocationSelector } from '@/components/vcc/LocationSelector';
import { useMemo } from 'react';

export function VCCDashboard() {
  const [stats, setStats] = useState<VCCStats | null>(null);
  const [deviceStats, setDeviceStats] = useState<VCCDeviceStats | null>(null);
  const [todayStats, setTodayStats] = useState<VCCStats | null>(null);
  const [heatmapStats, setHeatmapStats] = useState<VCCStats | null>(null);
  const [realtime, setRealtime] = useState<VCCRealtime | null>(null);
  const [loading, setLoading] = useState(true);
  const [realtimeLoading, setRealtimeLoading] = useState(false);

  // Camera filter state
  const [cameras, setCameras] = useState<CameraOption[]>([]);

  const [searchParams, setSearchParams] = useSearchParams();
  const selectedCamera = searchParams.get('camera');

  const setSelectedCamera = (cameraId: string | null) => {
    setSearchParams(prev => {
      const newParams = new URLSearchParams(prev);
      if (cameraId) {
        newParams.set('camera', cameraId);
      } else {
        newParams.delete('camera');
      }
      return newParams;
    });
  };

  const selectedLocation = searchParams.get('location');
  const setSelectedLocation = (location: string | null) => {
    setSearchParams(prev => {
      const newParams = new URLSearchParams(prev);
      if (location) {
        newParams.set('location', location);
        newParams.delete('camera'); // Clear camera when location changes
      } else {
        newParams.delete('location');
      }
      return newParams;
    });
  };

  // Derive unique locations
  const locations = useMemo(() => {
    const locs = new Set<string>();
    cameras.forEach(c => {
      if (c.metadata?.location) locs.add(c.metadata.location);
    });
    return Array.from(locs);
  }, [cameras]);

  // Filter cameras by location
  const filteredCameras = useMemo(() => {
    if (!selectedLocation) return cameras;
    return cameras.filter(c => c.metadata?.location === selectedLocation);
  }, [cameras, selectedLocation]);

  // Initialize with last 7 days
  const [dateRange, setDateRange] = useState<DateTimeRange>(() => {
    const end = new Date();
    const start = new Date();
    start.setDate(start.getDate() - 7);
    start.setHours(0, 0, 0, 0);
    return { startDate: start, endDate: end };
  });

  const [groupBy, setGroupBy] = useState<'minute' | 'hour' | 'day'>('day');

  // Modals state
  const [showDevicesModal, setShowDevicesModal] = useState(false);
  const [showReportModal, setShowReportModal] = useState(false);

  // Fetch available cameras from devices API
  useEffect(() => {
    const fetchCameras = async () => {
      try {
        const devices = await apiClient.getDevices({ type: 'CAMERA' }) as { id: string; name: string; metadata?: any }[];
        setCameras(devices.map(d => ({ id: d.id, name: d.name, metadata: d.metadata })));
      } catch (err) {
        console.error('Failed to fetch cameras:', err);
      }
    };
    fetchCameras();
  }, []);

  const fetchHeatmapStats = async () => {
    try {
      // Always fetch hourly data for the heatmap, using the selected date range
      const params = {
        startTime: dateRange.startDate.toISOString(),
        endTime: dateRange.endDate.toISOString(),
        groupBy: 'hour' as const, // Force hourly grouping
      };

      if (selectedCamera) {
        const data = await apiClient.getVCCByDevice(selectedCamera, params);
        setHeatmapStats(data as unknown as VCCStats);
      } else {
        const data = await apiClient.getVCCStats(params);
        setHeatmapStats(data);
      }
    } catch (err) {
      console.error("Failed to fetch heatmap stats:", err);
    }
  };

  const fetchTodayStats = async (silent = false) => {
    try {
      const start = new Date();
      start.setHours(0, 0, 0, 0); // Start of today
      const end = new Date(); // Now

      if (selectedCamera) {
        const data = await apiClient.getVCCByDevice(selectedCamera, {
          startTime: start.toISOString(),
          endTime: end.toISOString(),
          groupBy: 'hour',
        });
        setTodayStats(data as unknown as VCCStats);
      } else {
        const data = await apiClient.getVCCStats({
          startTime: start.toISOString(),
          endTime: end.toISOString(),
          groupBy: 'hour',
          location: selectedLocation || undefined,
        });
        setTodayStats(data);
      }
    } catch (err) {
      console.error("Failed to fetch today's stats:", err);
    }
  };

  const fetchStats = async (silent = false) => {
    try {
      if (!silent) setLoading(true);

      if (selectedCamera) {
        // Fetch per-camera stats
        const data = await apiClient.getVCCByDevice(selectedCamera, {
          startTime: dateRange.startDate.toISOString(),
          endTime: dateRange.endDate.toISOString(),
          groupBy: groupBy,
        });
        setDeviceStats(data);
        setStats(null); // Clear global stats only after new data is ready
      } else {
        // Fetch all cameras stats
        const data = await apiClient.getVCCStats({
          startTime: dateRange.startDate.toISOString(),
          endTime: dateRange.endDate.toISOString(),
          groupBy: groupBy,
          location: selectedLocation || undefined,
        });
        setStats(data);
        setDeviceStats(null); // Clear device stats only after new data is ready
      }
    } catch (err) {
      console.error('Failed to fetch VCC stats:', err);
    } finally {
      setLoading(false);
    }
  };

  const fetchRealtime = async () => {
    try {
      setRealtimeLoading(true);
      const data = await apiClient.getVCCRealtime();
      setRealtime(data);
    } catch (err) {
      console.error('Failed to fetch realtime data:', err);
    } finally {
      setRealtimeLoading(false);
    }
  };

  useEffect(() => {
    fetchStats();
    fetchTodayStats();
    fetchHeatmapStats();
    fetchRealtime();

    const interval = setInterval(() => {
      fetchRealtime();
      fetchStats(true);
      fetchTodayStats(true);
    }, 5000);

    return () => clearInterval(interval);
  }, [dateRange, groupBy, selectedCamera, selectedLocation]);

  // Auto-adjust groupBy based on date range
  useEffect(() => {
    const diffMs = dateRange.endDate.getTime() - dateRange.startDate.getTime();
    const diffHours = diffMs / (1000 * 60 * 60);
    const diffMinutes = diffMs / (1000 * 60);

    if (diffMinutes <= 30) {
      setGroupBy('minute');
    } else if (diffHours <= 24) {
      setGroupBy('hour');
    } else {
      setGroupBy('day');
    }
  }, [dateRange]);



  const getVehicleTypeColor = (type: string) => {
    const colors: Record<string, string> = {
      '2W': 'bg-blue-500',
      '4W': 'bg-green-500',
      'AUTO': 'bg-yellow-500',
      'TRUCK': 'bg-red-500',
      'BUS': 'bg-purple-500',
      'HMV': 'bg-red-500',
      'UNKNOWN': 'bg-gray-500',
    };
    return colors[type] || 'bg-gray-500';
  };

  const getVehicleTypeLabel = (type: string) => {
    const labels: Record<string, string> = {
      '2W': '2 Wheeler',
      '4W': '4 Wheeler',
      'AUTO': 'Auto',
      'TRUCK': 'Truck',
      'BUS': 'Bus',
      'HMV': 'Heavy Vehicle',
      'UNKNOWN': 'Unknown',
    };
    return labels[type] || type;
  };

  // Removed the full page loading check to prevent hard refresh effect. 
  // We will show the dashboard with a loading indicator if needed, or just let 'loading' prop handle inner component states.

  return (
    <div className="h-full overflow-y-auto p-4 space-y-3 bg-background/50">
      <div className="flex items-center justify-between pb-2 pt-2">
        <div className="flex items-center gap-2">
          {selectedCamera && (
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8 mr-1"
              onClick={() => setSelectedCamera(null)}
              title="Back to All Cameras"
            >
              <ArrowLeft className="w-5 h-5" />
            </Button>
          )}
          <div className="flex flex-col">
            <h1 className="text-xl font-semibold">Vehicle Classification & Counting</h1>
            {selectedCamera && (() => {
              const cam = cameras.find(c => c.id === selectedCamera);
              if (cam) {
                return (
                  <div className="text-sm text-muted-foreground flex items-center gap-2">
                    <span className="font-medium">{cam.name.replace(/^Camera\s+/i, "")}</span>
                    {cam.metadata?.location && (
                      <>
                        <span className="w-1 h-1 rounded-full bg-gray-400" />
                        <span>{cam.metadata.location}</span>
                      </>
                    )}
                  </div>
                );
              }
              return null;
            })()}
          </div>
        </div>
        <div className="flex items-center gap-2">
          {stats && (
            <Button
              variant="outline"
              size="sm"
              className="h-8 px-2 gap-2"
              onClick={() => setShowReportModal(true)}
            >
              <BarChart3 className="w-3 h-3" />
              Report
            </Button>
          )}
          {!selectedCamera && (
            <LocationSelector
              locations={locations}
              selectedLocation={selectedLocation}
              onSelect={setSelectedLocation}
            />
          )}
          <CameraSelector
            cameras={filteredCameras}
            selectedCamera={selectedCamera}
            onSelect={setSelectedCamera}
            loading={loading}
          />
          <DateTimeRangePicker
            value={dateRange}
            onChange={setDateRange}
          />
          <Tabs value={groupBy} onValueChange={(v) => setGroupBy(v as 'minute' | 'hour' | 'day')}>
            <TabsList className="h-8 ml-2">
              <TabsTrigger value="minute" className="text-xs px-2">Min</TabsTrigger>
              <TabsTrigger value="hour" className="text-xs px-2">Hour</TabsTrigger>
              <TabsTrigger value="day" className="text-xs px-2">Day</TabsTrigger>
            </TabsList>
          </Tabs>
          <Button variant="outline" size="sm" onClick={() => fetchStats()} className="h-8 px-2">
            <RefreshCw className="w-3 h-3" />
          </Button>
        </div>
      </div>

      {/* Insights Section */}
      <VCCInsights
        stats={deviceStats || stats}
        loading={loading}
        cameras={cameras}
        isSingleCamera={!!selectedCamera}
      />

      {/* Real-time Stats - Compact (Only for Global View) */}
      {!selectedCamera && (
        <Card className="glass p-3 border-blue-500/20">
          <div className="flex items-center gap-2 mb-2">
            <Activity className="w-4 h-4 text-blue-500" />
            <h2 className="text-sm font-semibold">Real-time (Last 5 Minutes)</h2>
            {realtimeLoading && <Loader2 className="w-3 h-3 animate-spin" />}
          </div>
          {realtime ? (
            <div className="grid grid-cols-4 gap-3">
              <div>
                <div className="text-xs text-gray-500 dark:text-gray-400">Detections</div>
                <div className="text-xl font-semibold">{realtime.totalDetections}</div>
              </div>
              <div>
                <div className="text-xs text-gray-500 dark:text-gray-400">Per Minute</div>
                <div className="text-xl font-semibold">{realtime.perMinute.toFixed(1)}</div>
              </div>
              <div>
                <div className="text-xs text-gray-500 dark:text-gray-400">Active Devices</div>
                <div className="text-xl font-semibold">{realtime.byDevice?.length || 0}</div>
              </div>
              <div>
                <div className="text-xs text-gray-500 dark:text-gray-400">Vehicle Types</div>
                <div className="text-xl font-semibold">{Object.keys(realtime.byVehicleType || {}).length}</div>
              </div>
            </div>
          ) : (
            <div className="text-sm text-gray-500 dark:text-gray-400">Loading realtime data...</div>
          )}
        </Card>
      )}

      {/* Main Stats Cards */}
      {
        (stats || deviceStats) && (() => {
          // Normalize stats for display - works with both all-cameras and per-camera data
          const displayStats = stats || deviceStats;
          const totalDetections = displayStats?.totalDetections || 0;
          const averagePerHour = displayStats?.averagePerHour || 0;
          const peakHour = displayStats?.peakHour || 0;
          const peakDay = stats?.peakDay || (selectedCamera ? cameras.find(c => c.id === selectedCamera)?.name || 'N/A' : 'N/A');
          const byVehicleType = displayStats?.byVehicleType || {};
          const byTime = stats?.byTime || [];
          const byDevice = stats?.byDevice || [];

          return (
            <>
              <div className="grid grid-cols-2 gap-4">
                <Card className="glass p-4">
                  <div className="flex items-center justify-between mb-2">
                    <div className="text-sm text-gray-500 dark:text-gray-400">Total Detections</div>
                    <Car className="w-5 h-5 text-blue-500" />
                  </div>
                  <div className="text-3xl font-semibold">{totalDetections.toLocaleString()}</div>
                  <div className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                    {averagePerHour.toFixed(1)} per hour avg
                  </div>
                </Card>

                <Card className="glass p-4">
                  <div className="flex items-center justify-between mb-2">
                    <div className="text-sm text-gray-500 dark:text-gray-400">Peak Hour</div>
                    <Clock className="w-5 h-5 text-yellow-500" />
                  </div>
                  <div className="text-3xl font-semibold">{peakHour}:00</div>
                  <div className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                    {selectedCamera ? `Camera: ${peakDay}` : `Peak day: ${peakDay}`}
                  </div>
                </Card>
              </div>

              {/* Vehicle Type Distribution */}
              <Card className="glass p-4">
                <h2 className="text-lg font-semibold mb-4">Vehicle Type Distribution</h2>

                {(() => {
                  const allTypes = ['2W', '3W', '4W', 'BUS', 'TRUCK', 'HMV']; // Standardized list
                  // Fallback for '3W' vs 'AUTO' naming if needed, or just map standard known types
                  // The helper uses 'AUTO', let's stick to the keys used in getVehicleTypeLabel
                  const displayTypes = ['2W', '4W', 'AUTO', 'BUS', 'HMV'];

                  // Check if we have ANY data to avoid showing empty 0s if completely loading/broken
                  const hasAnyData = byVehicleType && Object.keys(byVehicleType).length >= 0;

                  return (
                    <div className="grid grid-cols-5 gap-4">
                      {displayTypes.map((type) => {
                        const count = Number(byVehicleType?.[type]) || 0;
                        const percentage = totalDetections > 0 ? ((count / totalDetections) * 100).toFixed(1) : '0';

                        return (
                          <div key={type} className="text-center">
                            <Badge className={cn("w-full justify-center mb-2", getVehicleTypeColor(type))}>
                              {getVehicleTypeLabel(type)}
                            </Badge>
                            <div className="text-2xl font-semibold">{count.toLocaleString()}</div>
                            <div className="text-xs text-gray-500 dark:text-gray-400">
                              {percentage}%
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  );
                })()}
              </Card>

              {/* Charts and Devices - 2/3 and 1/3 layout */}
              <div className="grid grid-cols-3 gap-4">
                {/* Left Column - Charts (2/3) */}
                <div className="col-span-2 space-y-4">
                  {/* Time Series Chart */}
                  {byTime && byTime.length > 0 && (
                    <Card className="glass p-4">
                      <h2 className="text-lg font-semibold mb-4">Detections Over Time</h2>
                      <div className="h-80 w-full p-4 relative group/chart">
                        {(() => {
                          const counts = byTime.map(item => Number(item.count) || 0);
                          const maxCount = Math.max(...counts, 1);

                          // Generate percentage points for 100x100 coordinate system
                          const points = byTime.map((item, index) => {
                            const x = (index / (byTime.length - 1 || 1)) * 100;
                            const y = 100 - ((Number(item.count) || 0) / maxCount) * 100;
                            return { x, y, item, count: Number(item.count) || 0 };
                          });

                          const pathD = `M ${points.map(p => `${p.x},${p.y}`).join(' L ')}`;
                          const areaD = `${pathD} L 100,100 L 0,100 Z`;

                          return (
                            <>
                              <svg viewBox="0 0 100 100" preserveAspectRatio="none" className="absolute inset-0 w-full h-full overflow-visible z-0">
                                <defs>
                                  <linearGradient id="trendGradient" x1="0" y1="0" x2="0" y2="1">
                                    <stop offset="0%" stopColor="#3b82f6" stopOpacity="0.5" />
                                    <stop offset="100%" stopColor="#3b82f6" stopOpacity="0" />
                                  </linearGradient>
                                </defs>
                                <path d={areaD} fill="url(#trendGradient)" vectorEffect="non-scaling-stroke" />
                                <path d={pathD} fill="none" stroke="#3b82f6" strokeWidth="3" vectorEffect="non-scaling-stroke" strokeLinecap="round" strokeLinejoin="round" />
                              </svg>
                              <div className="absolute inset-0 w-full h-full z-10 pointer-events-none">
                                {points.map((p, index) => {
                                  const rawLabel = p.item.hour || p.item.day || p.item.week || p.item.month || '';
                                  let label = rawLabel;
                                  let fullLabel = rawLabel;
                                  try {
                                    const date = new Date(rawLabel);
                                    if (!isNaN(date.getTime())) {
                                      if (p.item.hour) {
                                        label = date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
                                        fullLabel = `${date.toLocaleDateString()} ${label}`;
                                      } else {
                                        label = date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
                                        fullLabel = date.toLocaleDateString();
                                      }
                                    }
                                  } catch (e) { }

                                  return (
                                    <div
                                      key={index}
                                      className="absolute transform -translate-x-1/2 -translate-y-1/2 group pointer-events-auto"
                                      style={{ left: `${p.x}%`, top: `${p.y}%` }}
                                    >
                                      <div className="w-3 h-3 bg-blue-500 rounded-full border-2 border-background shadow-sm transition-all group-hover:w-4 group-hover:h-4 group-hover:border-blue-300"></div>
                                      <div className="absolute bottom-full left-1/2 transform -translate-x-1/2 mb-2 opacity-0 group-hover:opacity-100 transition-opacity whitespace-nowrap z-50 pointer-events-none">
                                        <div className="bg-black/90 text-white text-xs p-2 rounded shadow-lg">
                                          <div className="font-bold">{p.count.toLocaleString()} vehicles</div>
                                          <div className="text-gray-400">{fullLabel}</div>
                                        </div>
                                        <div className="w-2 h-2 bg-black/90 rotate-45 mx-auto -mt-1"></div>
                                      </div>
                                      <div className="absolute top-6 left-1/2 transform -translate-x-1/2 text-xs text-gray-500 whitespace-nowrap mt-2">
                                        {label}
                                      </div>
                                    </div>
                                  );
                                })}
                              </div>
                            </>
                          );
                        })()}
                      </div>
                    </Card>
                  )}

                  {/* Today's Activity */}
                  <Card className="glass p-4">
                    <h2 className="text-lg font-semibold mb-4">Today's Activity</h2>
                    {(() => {
                      const byTime = todayStats?.byTime || [];
                      const hourlyData: Record<string, number> = {};
                      byTime.forEach((item: any) => {
                        const dateStr = (item.hour || item.time_period);
                        const d = new Date(dateStr.endsWith('Z') ? dateStr : dateStr + 'Z');

                        if (!isNaN(d.getTime())) {
                          const hour = d.getHours().toString();
                          hourlyData[hour] = (hourlyData[hour] || 0) + (Number(item.count) || 0);
                        }
                      });
                      const currentHour = new Date().getHours();
                      return byTime.length > 0 ? (
                        <div className="h-64 flex gap-1">
                          {Array.from({ length: 24 }, (_, hour) => {
                            const isFuture = hour > currentHour;
                            const count = isFuture ? 0 : (Number(hourlyData[hour.toString()]) || 0);
                            const maxCount = Math.max(...Object.values(hourlyData).map(v => Number(v) || 0), 1);
                            const height = (count / maxCount) * 100;
                            const visibleHeight = Math.max(height, 5);

                            return (
                              <div key={hour} className={cn("flex-1 h-full flex flex-col justify-end items-center min-w-[20px] relative group pointer-events-auto", isFuture && "opacity-30")}>
                                {!isFuture && (
                                  <div className="absolute bottom-full mb-2 opacity-0 group-hover:opacity-100 transition-opacity z-50 pointer-events-none">
                                    <div className="bg-black/90 text-white text-xs p-2 rounded shadow-lg whitespace-nowrap">
                                      <div className="font-bold">{count.toLocaleString()} vehicles</div>
                                      <div className="text-gray-400">{hour}:00</div>
                                    </div>
                                    <div className="w-2 h-2 bg-black/90 rotate-45 mx-auto -mt-1"></div>
                                  </div>
                                )}
                                <div
                                  className={cn(
                                    "w-full rounded-t transition-all",
                                    isFuture ? "bg-gray-700/20" : "bg-blue-500 hover:bg-blue-400 cursor-pointer"
                                  )}
                                  style={{ height: isFuture ? '5%' : `${visibleHeight}%` }}
                                />
                                <div className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                                  {hour % 4 === 0 ? hour : ''}
                                </div>
                              </div>
                            );
                          })}
                        </div>
                      ) : (
                        <div className="h-64 flex items-center justify-center text-gray-500 dark:text-gray-400">
                          No activity recorded today
                        </div>
                      );
                    })()}
                  </Card>
                </div>

                {/* Right Column - Top Devices Table (1/3) */}
                {/* Right Column - Top Devices Table (1/3) */}
                {!selectedCamera && (
                  <div className="col-span-1">
                    <Card className="glass p-4 h-full flex flex-col">
                      <div className="flex items-center justify-between mb-4">
                        <h2 className="text-lg font-semibold">Top Devices</h2>
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-6 text-xs"
                          onClick={() => setShowDevicesModal(true)}
                        >
                          View All
                        </Button>
                      </div>
                      <div className="overflow-y-auto max-h-[calc(100vh-300px)] flex-1">
                        <table className="w-full">
                          <thead className="sticky top-0 bg-background/80 backdrop-blur">
                            <tr className="border-b border-white/10">
                              <th className="text-left p-2 text-xs font-medium">Device</th>
                              <th className="text-right p-2 text-xs font-medium">Count</th>
                            </tr>
                          </thead>
                          <tbody>
                            {byDevice.slice(0, 10).map((device, index) => {
                              const cam = cameras.find(c => c.id === device.deviceId);
                              const location = cam?.metadata?.location;

                              return (
                                <tr
                                  key={device.deviceId}
                                  className="border-b border-white/5 hover:bg-white/10 cursor-pointer transition-colors"
                                  onClick={() => setSelectedCamera(device.deviceId)}
                                  title={`Click to view stats for ${device.deviceName || device.deviceId}`}
                                >
                                  <td className="p-2">
                                    <div className="flex items-start gap-2">
                                      <Badge variant="outline" className="text-xs px-1 mt-0.5">#{index + 1}</Badge>
                                      <div className="flex flex-col min-w-0">
                                        <span className="text-sm font-medium truncate">{(device.deviceName || device.deviceId).replace(/^Camera\s+/i, "")}</span>
                                        {location && <span className="text-xs text-muted-foreground truncate">{location}</span>}
                                      </div>
                                    </div>
                                  </td>
                                  <td className="p-2 text-right">
                                    <div className="text-sm font-semibold">{device.totalDetections.toLocaleString()}</div>
                                    <div className="text-xs text-gray-500 dark:text-gray-400">
                                      {totalDetections > 0
                                        ? ((device.totalDetections / totalDetections) * 100).toFixed(1)
                                        : '0'}%
                                    </div>
                                  </td>
                                </tr>
                              );
                            })}
                          </tbody>
                        </table>
                      </div>
                    </Card>
                  </div>
                )}

                {/* Heatmap Section */}
                <div className="col-span-3">
                  <VCCHeatmap stats={heatmapStats} loading={loading} />
                </div>
              </div>
            </>
          );
        })()
      }

      {/* Modals */}
      {stats && (
        <VCCDevicesView
          open={showDevicesModal}
          onOpenChange={setShowDevicesModal}
          devices={stats.byDevice || []}
          totalDetections={stats.totalDetections}
          onSelectCamera={setSelectedCamera}
          cameras={cameras}
        />
      )}

      <VCCReportModal
        open={showReportModal}
        onOpenChange={setShowReportModal}
        cameras={cameras}
        initialDateRange={dateRange}
        selectedCamera={selectedCamera}
      />
    </div>
  );
}
