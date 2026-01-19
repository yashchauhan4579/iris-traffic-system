import { Search, Camera, Plane, Radio, Layers, MapPin, Car, Grid3x3, RefreshCw, Satellite, Map as MapIcon } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useDeviceFilter } from '@/contexts/DeviceFilterContext';
import { useLayerVisibility } from '@/contexts/LayerVisibilityContext';
import { useCameraGrid, type GridSize } from '@/contexts/CameraGridContext';
import { useCrowdDashboard } from '@/contexts/CrowdDashboardContext';
import { useMapType } from '@/contexts/MapTypeContext';
import type { DeviceType } from '@/lib/api';
import { cn } from '@/lib/utils';

// Component for camera grid controls (separate to handle hook usage)
function CameraGridControls() {
  const { gridSize, setGridSize, usedSlots } = useCameraGrid();
  const [cols, rows] = gridSize.split('x').map(Number);
  const totalSlots = rows * cols;

  return (
    <div className="flex items-center gap-2 border-r border-white/10 dark:border-white/5 pr-4">
      <Grid3x3 className="w-4 h-4 text-gray-500 dark:text-gray-400" />
      <span className="text-sm text-gray-700 dark:text-gray-300">Grid:</span>
      <div className="flex items-center gap-1">
        {(['1x1', '2x2', '2x3', '3x4', '4x5'] as GridSize[]).map((size) => (
          <Button
            key={size}
            variant="ghost"
            size="sm"
            onClick={() => setGridSize(size)}
            className={cn(
              "rounded-lg transition-all px-2 py-1 text-xs",
              gridSize === size
                ? "bg-blue-500 text-white hover:bg-blue-600"
                : "bg-white/50 dark:bg-white/5 hover:bg-white/70 dark:hover:bg-white/10 text-gray-700 dark:text-gray-300"
            )}
          >
            {size}
          </Button>
        ))}
      </div>
      <span className="text-xs text-gray-500 dark:text-gray-400 ml-2">
        {usedSlots} / {totalSlots}
      </span>
    </div>
  );
}

const deviceTypeConfig: Record<DeviceType, { label: string; icon: typeof Camera; color: string }> = {
  CAMERA: {
    label: 'Cameras',
    icon: Camera,
    color: 'blue',
  },
  DRONE: {
    label: 'Drones',
    icon: Plane,
    color: 'green',
  },
  SENSOR: {
    label: 'Sensors',
    icon: Radio,
    color: 'amber',
  },
};

interface TopBarProps {
  activeView?: string;
}

// Component for crowd dashboard controls (separate to handle hook usage)
function CrowdDashboardControls() {
  const { autoRefresh, setAutoRefresh } = useCrowdDashboard();

  return (
    <div className="flex items-center gap-2 border-r border-white/10 dark:border-white/5 pr-4">
      <Button
        variant="ghost"
        size="sm"
        onClick={() => setAutoRefresh(!autoRefresh)}
        className={cn(
          "rounded-xl transition-all duration-200 flex items-center gap-2 border",
          autoRefresh
            ? "bg-blue-500 hover:bg-blue-600 text-white border-blue-500 shadow-lg shadow-blue-500/30"
            : "bg-white/50 dark:bg-white/5 border-white/20 dark:border-white/10 hover:bg-white/70 dark:hover:bg-white/10 text-gray-700 dark:text-gray-300"
        )}
      >
        <RefreshCw className={cn("w-4 h-4", autoRefresh && "animate-spin")} />
        {autoRefresh ? 'Auto-refresh: ON' : 'Auto-refresh: OFF'}
      </Button>
    </div>
  );
}

export function TopBar({ activeView = 'map' }: TopBarProps) {
  const { selectedTypes, toggleType, isTypeSelected } = useDeviceFilter();
  const { showCameras, showHotspots, showTraffic, toggleCameras, toggleHotspots, toggleTraffic } = useLayerVisibility();
  const { mapType, toggleMapType } = useMapType();

  return (
    <div className="fixed top-0 left-20 right-0 h-16 glass border-b border-white/10 dark:border-white/5 z-40 flex items-center px-6 gap-4">
      {/* Title */}
      <div className="flex-1">
        <h1 className="text-xl font-semibold text-gray-900 dark:text-white">
          {activeView === 'crowd' ? 'Crowd Analysis Dashboard' : 'IRIS Command Center'}
        </h1>
        <p className="text-xs text-gray-500 dark:text-gray-400">
          {activeView === 'crowd' ? 'Real-time crowd monitoring and hotspot detection' : 'Real-time Surveillance & Analytics'}
        </p>
      </div>

      {/* Crowd Dashboard Controls */}
      {activeView === 'crowd' && <CrowdDashboardControls />}

      {/* Camera Grid Controls (only for camera view) */}
      {activeView === 'cameras' && <CameraGridControls />}

      {/* Layer Visibility Controls (only for map view) */}
      {activeView === 'map' && (
        <div className="flex items-center gap-2 border-r border-white/10 dark:border-white/5 pr-4">
          <Layers className="w-4 h-4 text-gray-500 dark:text-gray-400" />
          <Button
            variant="ghost"
            size="sm"
            onClick={toggleCameras}
            className={cn(
              "rounded-xl transition-all duration-200 flex items-center gap-2 border",
              showCameras && "bg-blue-500 hover:bg-blue-600 text-white border-blue-500 shadow-lg shadow-blue-500/30",
              !showCameras && "bg-white/50 dark:bg-white/5 border-white/20 dark:border-white/10 hover:bg-white/70 dark:hover:bg-white/10 text-gray-700 dark:text-gray-300"
            )}
          >
            <MapPin className="w-4 h-4" />
            Devices
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={toggleHotspots}
            className={cn(
              "rounded-xl transition-all duration-200 flex items-center gap-2 border",
              showHotspots && "bg-red-500 hover:bg-red-600 text-white border-red-500 shadow-lg shadow-red-500/30",
              !showHotspots && "bg-white/50 dark:bg-white/5 border-white/20 dark:border-white/10 hover:bg-white/70 dark:hover:bg-white/10 text-gray-700 dark:text-gray-300"
            )}
          >
            <div className="w-4 h-4 rounded-full bg-gradient-to-br from-yellow-400 via-orange-500 to-red-600" />
            Hotspots
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={toggleTraffic}
            className={cn(
              "rounded-xl transition-all duration-200 flex items-center gap-2 border",
              showTraffic && "bg-purple-500 hover:bg-purple-600 text-white border-purple-500 shadow-lg shadow-purple-500/30",
              !showTraffic && "bg-white/50 dark:bg-white/5 border-white/20 dark:border-white/10 hover:bg-white/70 dark:hover:bg-white/10 text-gray-700 dark:text-gray-300"
            )}
          >
            <Car className="w-4 h-4" />
            Traffic
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={toggleMapType}
            className={cn(
              "rounded-xl transition-all duration-200 flex items-center gap-2 border",
              mapType === 'satellite'
                ? "bg-blue-500 hover:bg-blue-600 text-white border-blue-500 shadow-lg shadow-blue-500/30"
                : "bg-white/50 dark:bg-white/5 border-white/20 dark:border-white/10 hover:bg-white/70 dark:hover:bg-white/10 text-gray-700 dark:text-gray-300"
            )}
            title={mapType === 'satellite' ? 'Switch to Roadmap' : 'Switch to Satellite'}
          >
            {mapType === 'satellite' ? (
              <>
                <Satellite className="w-4 h-4" />
                <span>Satellite</span>
              </>
            ) : (
              <>
                <MapIcon className="w-4 h-4" />
                <span>Roadmap</span>
              </>
            )}
          </Button>
        </div>
      )}

      {/* Device Type Filters (only for map view) */}
      {activeView === 'map' && (
        <div className="flex items-center gap-2">
          {(Object.keys(deviceTypeConfig) as DeviceType[]).map((type) => {
            const config = deviceTypeConfig[type];
            const Icon = config.icon;
            const isSelected = isTypeSelected(type);

            return (
              <Button
                key={type}
                variant="ghost"
                size="sm"
                onClick={() => toggleType(type)}
                className={cn(
                  "rounded-xl transition-all duration-200 flex items-center gap-2 border",
                  isSelected && type === 'CAMERA' && "bg-blue-500 hover:bg-blue-600 text-white border-blue-500 shadow-lg shadow-blue-500/30",
                  isSelected && type === 'DRONE' && "bg-green-500 hover:bg-green-600 text-white border-green-500 shadow-lg shadow-green-500/30",
                  isSelected && type === 'SENSOR' && "bg-amber-500 hover:bg-amber-600 text-white border-amber-500 shadow-lg shadow-amber-500/30",
                  !isSelected && "bg-white/50 dark:bg-white/5 border-white/20 dark:border-white/10 hover:bg-white/70 dark:hover:bg-white/10 text-gray-700 dark:text-gray-300"
                )}
              >
                <Icon className="w-4 h-4" />
                {config.label}
              </Button>
            );
          })}
        </div>
      )}

      {/* Search */}
      <div className="flex items-center gap-2 bg-white/50 dark:bg-white/5 rounded-xl px-4 py-2 min-w-[300px] border border-transparent dark:border-white/10">
        <Search className="w-4 h-4 text-gray-400 dark:text-gray-500" />
        <input
          type="text"
          placeholder="Search cameras, locations..."
          className="flex-1 bg-transparent border-none outline-none text-sm text-gray-900 dark:text-gray-100 placeholder:text-gray-400 dark:placeholder:text-gray-500"
        />
      </div>
    </div>
  );
}

