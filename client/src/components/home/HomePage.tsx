import { useState, useEffect, Suspense } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Map, Camera, Users, Car, TrendingUp, AlertTriangle,
  BarChart3, Bell, Settings, Monitor, Shield,
  Video, Activity, FileText, Cog, ChevronLeft, ChevronRight, Server
} from 'lucide-react';
import { TypeAnimation } from 'react-type-animation';
import { Background3D } from './Background3D';
import { IRISEye3D } from './IRISEye3D';
import { apiClient } from '@/lib/api';

interface SubMenuItem {
  id: string;
  icon: typeof Map;
  label: string;
  path: string;
}

interface MainModule {
  id: string;
  icon: typeof Map;
  label: string;
  color: string;
  side: 'left' | 'right';
  subItems: SubMenuItem[];
}

const mainModules: MainModule[] = [
  // Left side modules
  {
    id: 'vms',
    icon: Monitor,
    label: 'VMS',
    color: '#3b82f6',
    side: 'left',
    subItems: [
      { id: 'cameras', icon: Camera, label: 'CAMERAS', path: '/cameras' },
      { id: 'live', icon: Video, label: 'LIVE VIEW', path: '/cameras/live' },
      { id: 'recordings', icon: FileText, label: 'RECORDINGS', path: '/cameras/recordings' },
    ],
  },
  {
    id: 'crowd',
    icon: Users,
    label: 'CROWD',
    color: '#f97316',
    side: 'left',
    subItems: [
      { id: 'map', icon: Map, label: 'MAP VIEW', path: '/map' },
      { id: 'crowd', icon: Users, label: 'ANALYSIS', path: '/crowd' },
      { id: 'hotspots', icon: Activity, label: 'HOTSPOTS', path: '/crowd/hotspots' },
    ],
  },
  {
    id: 'itms',
    icon: Car,
    label: 'ITMS',
    color: '#a855f7',
    side: 'left',
    subItems: [
      { id: 'anpr', icon: Car, label: 'ANPR', path: '/itms/anpr' },
      { id: 'vcc', icon: TrendingUp, label: 'VCC', path: '/itms/vcc' },
      { id: 'violations', icon: AlertTriangle, label: 'VIOLATIONS', path: '/itms/violations' },
    ],
  },
  // Right side modules
  {
    id: 'analytics',
    icon: BarChart3,
    label: 'ANALYTICS',
    color: '#06b6d4',
    side: 'right',
    subItems: [
      { id: 'dashboard', icon: BarChart3, label: 'DASHBOARD', path: '/analytics' },
      { id: 'reports', icon: FileText, label: 'REPORTS', path: '/analytics/reports' },
    ],
  },
  {
    id: 'alerts',
    icon: Bell,
    label: 'ALERTS',
    color: '#ef4444',
    side: 'right',
    subItems: [
      { id: 'notifications', icon: Bell, label: 'NOTIFICATIONS', path: '/alerts' },
      { id: 'rules', icon: Shield, label: 'RULES', path: '/alerts/rules' },
    ],
  },
  {
    id: 'settings',
    icon: Settings,
    label: 'SETTINGS',
    color: '#6b7280',
    side: 'right',
    subItems: [
      { id: 'workers', icon: Server, label: 'WORKERS', path: '/settings/workers' },
      { id: 'devices', icon: Monitor, label: 'DEVICES', path: '/settings/devices' },
      { id: 'system', icon: Cog, label: 'SYSTEM', path: '/settings' },
    ],
  },
];

export function HomePage() {
  const navigate = useNavigate();
  const [selectedModule, setSelectedModule] = useState<MainModule | null>(null);
  const [hoveredModule, setHoveredModule] = useState<string | null>(null);
  const [hoveredSubItem, setHoveredSubItem] = useState<string | null>(null);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && selectedModule) {
        setSelectedModule(null);
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [selectedModule]);

  const [allDevices, setAllDevices] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchStats = async () => {
      try {
        const devices = await apiClient.getDevices({ minimal: true });
        if (Array.isArray(devices)) {
          setAllDevices(devices);
          const cameras = devices.filter(d => d.type === 'CAMERA');
          const activeCameras = cameras.filter(d => d.status === 'active');
          console.log('Total devices:', devices.length);
          console.log('Total cameras:', cameras.length);
          console.log('Active cameras:', activeCameras.length);
          console.log('Camera statuses:', cameras.map(c => ({ id: c.id, status: c.status })));
        }
      } catch (err) {
        console.error("Failed to fetch device stats", err);
      } finally {
        setLoading(false);
      }
    };

    fetchStats();
    const interval = setInterval(fetchStats, 30000); // Update every 30s
    return () => clearInterval(interval);
  }, []);

  const activeColor = selectedModule?.color || '#f97316';
  const leftModules = mainModules.filter(m => m.side === 'left');
  const rightModules = mainModules.filter(m => m.side === 'right');

  const handleModuleClick = (module: MainModule) => {
    if (selectedModule?.id === module.id) {
      setSelectedModule(null);
    } else {
      setSelectedModule(module);
    }
  };

  const getSubItemPosition = (index: number, total: number, side: 'left' | 'right') => {
    const startAngle = side === 'left' ? -60 : 60;
    const spread = 40;
    const startOffset = -((total - 1) * spread) / 2;
    const angle = startAngle + startOffset + index * spread;
    const radius = 150;
    const x = Math.cos((angle * Math.PI) / 180) * radius;
    const y = Math.sin((angle * Math.PI) / 180) * radius;
    return { x, y };
  };

  return (
    <div
      style={{
        height: '100%',
        width: '100%',
        position: 'relative',
        overflow: 'hidden',
      }}
    >
      {/* 3D Background */}
      <Suspense fallback={
        <div style={{
          position: 'absolute',
          inset: 0,
          background: 'linear-gradient(135deg, #030712 0%, #0f172a 50%, #030712 100%)'
        }} />
      }>
        <Background3D color={activeColor} />
      </Suspense>

      {/* UI Layer */}
      <div
        style={{
          position: 'relative',
          zIndex: 10,
          height: '100%',
          width: '100%',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        {/* Main container */}
        <div
          style={{
            position: 'relative',
            width: '90%',
            maxWidth: 1000,
            height: '80%',
            maxHeight: 500,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          {/* Outer ellipse ring */}
          <div
            style={{
              position: 'absolute',
              inset: 0,
              borderRadius: '50%',
              border: `2px dashed ${activeColor}30`,
              pointerEvents: 'none',
            }}
          />

          {/* Inner ellipse ring */}
          <div
            style={{
              position: 'absolute',
              inset: '15%',
              borderRadius: '50%',
              border: `1px solid ${activeColor}20`,
              pointerEvents: 'none',
            }}
          />

          {/* Left side modules */}
          <div
            style={{
              position: 'absolute',
              left: '3%',
              top: '50%',
              transform: 'translateY(-50%)',
              display: 'flex',
              flexDirection: 'column',
              gap: 12,
              zIndex: 30,
            }}
          >
            {leftModules.map((module) => {
              const Icon = module.icon;
              const isSelected = selectedModule?.id === module.id;
              const isHovered = hoveredModule === module.id;

              return (
                <button
                  key={module.id}
                  type="button"
                  onClick={() => handleModuleClick(module)}
                  onMouseEnter={() => setHoveredModule(module.id)}
                  onMouseLeave={() => setHoveredModule(null)}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 12,
                    padding: '10px 16px',
                    borderRadius: 12,
                    background: isSelected
                      ? `linear-gradient(90deg, ${module.color}40 0%, transparent 100%)`
                      : isHovered
                        ? `linear-gradient(90deg, ${module.color}20 0%, transparent 100%)`
                        : 'linear-gradient(90deg, rgba(15, 23, 42, 0.8) 0%, transparent 100%)',
                    border: `1px solid ${isSelected ? module.color + '60' : 'transparent'}`,
                    cursor: 'pointer',
                    transition: 'all 0.3s ease',
                    transform: isSelected ? 'scale(1.05) translateX(5px)' : isHovered ? 'translateX(8px)' : 'none',
                    backdropFilter: 'blur(10px)',
                  }}
                >
                  <div
                    style={{
                      width: 40,
                      height: 40,
                      borderRadius: 10,
                      backgroundColor: isSelected || isHovered ? module.color + '30' : 'rgba(55, 65, 81, 0.5)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      transition: 'all 0.3s',
                      boxShadow: isSelected ? `0 0 20px ${module.color}50` : 'none',
                    }}
                  >
                    <Icon
                      style={{
                        width: 20,
                        height: 20,
                        color: isSelected || isHovered ? module.color : '#9ca3af',
                        transition: 'color 0.2s',
                      }}
                    />
                  </div>
                  <span
                    style={{
                      fontSize: 12,
                      fontWeight: 700,
                      letterSpacing: '0.15em',
                      color: isSelected || isHovered ? module.color : '#9ca3af',
                      transition: 'color 0.2s',
                      minWidth: 65,
                      textAlign: 'left',
                      textShadow: isSelected ? `0 0 10px ${module.color}` : 'none',
                    }}
                  >
                    {module.label}
                  </span>
                  {/* Active bar */}
                  <div
                    style={{
                      width: 3,
                      height: 28,
                      borderRadius: 2,
                      backgroundColor: isSelected ? module.color : isHovered ? module.color + '60' : 'transparent',
                      marginLeft: 4,
                      transition: 'all 0.3s',
                      boxShadow: isSelected ? `0 0 10px ${module.color}` : 'none',
                    }}
                  />
                </button>
              );
            })}
          </div>

          {/* Right side modules */}
          <div
            style={{
              position: 'absolute',
              right: '3%',
              top: '50%',
              transform: 'translateY(-50%)',
              display: 'flex',
              flexDirection: 'column',
              gap: 12,
              zIndex: 30,
            }}
          >
            {rightModules.map((module) => {
              const Icon = module.icon;
              const isSelected = selectedModule?.id === module.id;
              const isHovered = hoveredModule === module.id;

              return (
                <button
                  key={module.id}
                  type="button"
                  onClick={() => handleModuleClick(module)}
                  onMouseEnter={() => setHoveredModule(module.id)}
                  onMouseLeave={() => setHoveredModule(null)}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    flexDirection: 'row-reverse',
                    gap: 12,
                    padding: '10px 16px',
                    borderRadius: 12,
                    background: isSelected
                      ? `linear-gradient(270deg, ${module.color}40 0%, transparent 100%)`
                      : isHovered
                        ? `linear-gradient(270deg, ${module.color}20 0%, transparent 100%)`
                        : 'linear-gradient(270deg, rgba(15, 23, 42, 0.8) 0%, transparent 100%)',
                    border: `1px solid ${isSelected ? module.color + '60' : 'transparent'}`,
                    cursor: 'pointer',
                    transition: 'all 0.3s ease',
                    transform: isSelected ? 'scale(1.05) translateX(-5px)' : isHovered ? 'translateX(-8px)' : 'none',
                    backdropFilter: 'blur(10px)',
                  }}
                >
                  <div
                    style={{
                      width: 40,
                      height: 40,
                      borderRadius: 10,
                      backgroundColor: isSelected || isHovered ? module.color + '30' : 'rgba(55, 65, 81, 0.5)',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      transition: 'all 0.3s',
                      boxShadow: isSelected ? `0 0 20px ${module.color}50` : 'none',
                    }}
                  >
                    <Icon
                      style={{
                        width: 20,
                        height: 20,
                        color: isSelected || isHovered ? module.color : '#9ca3af',
                        transition: 'color 0.2s',
                      }}
                    />
                  </div>
                  <span
                    style={{
                      fontSize: 12,
                      fontWeight: 700,
                      letterSpacing: '0.15em',
                      color: isSelected || isHovered ? module.color : '#9ca3af',
                      transition: 'color 0.2s',
                      minWidth: 75,
                      textAlign: 'right',
                      textShadow: isSelected ? `0 0 10px ${module.color}` : 'none',
                    }}
                  >
                    {module.label}
                  </span>
                  {/* Active bar */}
                  <div
                    style={{
                      width: 3,
                      height: 28,
                      borderRadius: 2,
                      backgroundColor: isSelected ? module.color : isHovered ? module.color + '60' : 'transparent',
                      marginRight: 4,
                      transition: 'all 0.3s',
                      boxShadow: isSelected ? `0 0 10px ${module.color}` : 'none',
                    }}
                  />
                </button>
              );
            })}
          </div>

          {/* Center hub with 3D eye and submenu */}
          <div
            style={{
              position: 'relative',
              zIndex: 40,
            }}
          >
            {/* Submenu items */}
            {selectedModule && selectedModule.subItems.map((item, index) => {
              const Icon = item.icon;
              const { x, y } = getSubItemPosition(index, selectedModule.subItems.length, selectedModule.side);
              const isHovered = hoveredSubItem === item.id;

              return (
                <div
                  key={item.id}
                  style={{
                    position: 'absolute',
                    left: '50%',
                    top: '50%',
                    transform: `translate(calc(-50% + ${x}px), calc(-50% + ${y}px))`,
                    zIndex: 35,
                    transition: 'all 0.4s ease',
                    transitionDelay: `${index * 80}ms`,
                  }}
                >
                  <button
                    type="button"
                    onClick={() => {
                      // Prevent navigation for Crowd, ANPR, Violations, Live View, and Recordings
                      if (selectedModule?.id === 'crowd' ||
                        item.id === 'anpr' ||
                        item.id === 'violations' ||
                        item.id === 'live' ||
                        item.id === 'recordings') {
                        return;
                      }
                      navigate(item.path);
                    }}
                    onMouseEnter={() => setHoveredSubItem(item.id)}
                    onMouseLeave={() => setHoveredSubItem(null)}
                    style={{
                      width: 76,
                      height: 76,
                      borderRadius: 16,
                      background: 'linear-gradient(135deg, rgba(15, 23, 42, 0.95) 0%, rgba(30, 41, 59, 0.95) 100%)',
                      border: `1px solid ${isHovered ? selectedModule.color + '80' : 'rgba(55, 65, 81, 0.5)'}`,
                      display: 'flex',
                      flexDirection: 'column',
                      alignItems: 'center',
                      justifyContent: 'center',
                      gap: 6,
                      cursor: 'pointer',
                      transform: isHovered ? 'scale(1.15)' : 'scale(1)',
                      boxShadow: isHovered
                        ? `0 0 40px ${selectedModule.color}50, 0 0 60px ${selectedModule.color}20`
                        : '0 4px 20px rgba(0,0,0,0.4)',
                      transition: 'all 0.25s ease',
                      backdropFilter: 'blur(10px)',
                    }}
                  >
                    <Icon
                      style={{
                        width: 24,
                        height: 24,
                        color: isHovered ? selectedModule.color : '#9ca3af',
                        transition: 'all 0.2s',
                        filter: isHovered ? `drop-shadow(0 0 8px ${selectedModule.color})` : 'none',
                      }}
                    />
                    <span
                      style={{
                        fontSize: 9,
                        fontWeight: 700,
                        letterSpacing: '0.08em',
                        color: isHovered ? selectedModule.color : '#6b7280',
                        transition: 'color 0.2s',
                        textShadow: isHovered ? `0 0 10px ${selectedModule.color}` : 'none',
                      }}
                    >
                      {item.label}
                    </span>
                  </button>
                </div>
              );
            })}

            {/* Center circle with 3D Eye */}
            <button
              type="button"
              onClick={() => selectedModule && setSelectedModule(null)}
              style={{
                width: 160,
                height: 160,
                borderRadius: '50%',
                background: 'linear-gradient(135deg, #0f172a 0%, #1e293b 50%, #0f172a 100%)',
                border: `2px solid ${selectedModule ? activeColor + '60' : 'rgba(55, 65, 81, 0.4)'}`,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                cursor: selectedModule ? 'pointer' : 'default',
                boxShadow: `0 0 80px ${activeColor}30, inset 0 0 40px rgba(0,0,0,0.5)`,
                transition: 'all 0.4s ease',
                overflow: 'hidden',
                padding: 0,
              }}
            >
              {selectedModule ? (
                <div style={{ textAlign: 'center' }}>
                  <div
                    style={{
                      width: 56,
                      height: 56,
                      margin: '0 auto 8px',
                      borderRadius: '50%',
                      backgroundColor: selectedModule.color + '25',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      boxShadow: `0 0 30px ${selectedModule.color}40`,
                    }}
                  >
                    <selectedModule.icon
                      style={{
                        width: 28,
                        height: 28,
                        color: selectedModule.color,
                        filter: `drop-shadow(0 0 10px ${selectedModule.color})`,
                      }}
                    />
                  </div>
                  <span
                    style={{
                      fontSize: 12,
                      fontWeight: 700,
                      letterSpacing: '0.15em',
                      color: selectedModule.color,
                      textShadow: `0 0 15px ${selectedModule.color}`,
                    }}
                  >
                    {selectedModule.label}
                  </span>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 4, marginTop: 8 }}>
                    {selectedModule.side === 'left' ? (
                      <ChevronLeft style={{ width: 14, height: 14, color: '#6b7280' }} />
                    ) : (
                      <ChevronRight style={{ width: 14, height: 14, color: '#6b7280' }} />
                    )}
                    <span style={{ fontSize: 9, color: '#6b7280', letterSpacing: '0.1em' }}>BACK</span>
                  </div>
                </div>
              ) : (
                <Suspense fallback={
                  <div style={{ textAlign: 'center' }}>
                    <div style={{
                      width: 48,
                      height: 48,
                      borderRadius: '50%',
                      backgroundColor: activeColor + '30',
                      margin: '0 auto 8px',
                    }} />
                    <span style={{ fontSize: 12, color: '#9ca3af' }}>IRIS</span>
                  </div>
                }>
                  <IRISEye3D color={activeColor} isActive={!!hoveredModule} size={156} />
                </Suspense>
              )}
            </button>
          </div>

          {/* Decorative side dots */}
          <div
            style={{
              position: 'absolute',
              left: 0,
              top: '50%',
              transform: 'translateY(-50%)',
              width: 8,
              height: 8,
              borderRadius: '50%',
              backgroundColor: activeColor,
              boxShadow: `0 0 15px ${activeColor}, 0 0 30px ${activeColor}50`,
              pointerEvents: 'none',
            }}
          />
          <div
            style={{
              position: 'absolute',
              right: 0,
              top: '50%',
              transform: 'translateY(-50%)',
              width: 8,
              height: 8,
              borderRadius: '50%',
              backgroundColor: activeColor,
              boxShadow: `0 0 15px ${activeColor}, 0 0 30px ${activeColor}50`,
              pointerEvents: 'none',
            }}
          />
        </div>
      </div>

      {/* Top info bar */}
      <div
        className="text-gray-900 dark:text-gray-400"
        style={{
          position: 'absolute',
          top: 24,
          left: '50%',
          transform: 'translateX(-50%)',
          display: 'flex',
          alignItems: 'center',
          gap: 40,
          zIndex: 20,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, color: 'inherit' }}>
          <Bell style={{ width: 18, height: 18, color: activeColor, filter: `drop-shadow(0 0 5px ${activeColor})` }} />
          <span style={{ fontSize: 13, fontWeight: 500 }}>3 Alerts</span>
        </div>
        <div
          className="text-white dark:text-white text-gray-900"
          style={{
            fontSize: 36,
            fontWeight: 200,
            letterSpacing: '0.15em',
            textShadow: '0 0 20px rgba(255,255,255,0.3)',
          }}
        >
          {new Date().toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: false })}
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, color: 'inherit' }}>
          <Camera style={{ width: 18, height: 18, color: activeColor, filter: `drop-shadow(0 0 5px ${activeColor})` }} />
          <span style={{ fontSize: 13, fontWeight: 500 }}>{loading ? '...' : `${allDevices.filter(d => d.type === 'CAMERA' && d.status === 'active').length} Online`}</span>
        </div>
      </div>

      {/* Bottom status bar with typing animation */}
      <div
        style={{
          position: 'absolute',
          bottom: 20,
          left: '50%',
          transform: 'translateX(-50%)',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          gap: 8,
          zIndex: 20,
        }}
      >
        {/* Typing animation - shows when no module selected */}
        {!selectedModule && (
          <div
            style={{
              height: 50,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              minWidth: 450,
            }}
          >
            <TypeAnimation
              sequence={[
                'IRIS',
                2500,
                '',
                600,
                'Integrated Realtime Intelligence System',
                4000,
                '',
                800,
              ]}
              speed={45}
              deletionSpeed={65}
              cursor={true}
              repeat={Infinity}
              style={{
                fontSize: 20,
                fontWeight: 600,
                color: '#e5e7eb',
                letterSpacing: '0.12em',
                textShadow: `0 0 25px ${activeColor}60`,
                fontFamily: 'system-ui, -apple-system, sans-serif',
              }}
            />
          </div>
        )}

        {/* Status text */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <div
            style={{
              width: 6,
              height: 6,
              borderRadius: '50%',
              backgroundColor: '#22c55e',
              boxShadow: '0 0 8px #22c55e',
            }}
          />
          <span
            style={{
              fontSize: 11,
              fontWeight: 500,
              color: '#6b7280',
              letterSpacing: '0.15em',
              textTransform: 'uppercase',
            }}
          >
            {selectedModule ? selectedModule.label + ' Module' : 'IRIS Command Center'}
          </span>
          <div
            style={{
              width: 6,
              height: 6,
              borderRadius: '50%',
              backgroundColor: '#22c55e',
              boxShadow: '0 0 8px #22c55e',
            }}
          />
        </div>
      </div>

      {/* Keyboard hint */}
      <div style={{ position: 'absolute', bottom: 24, right: 24, fontSize: 10, color: '#4b5563', zIndex: 20 }}>
        <span
          style={{
            padding: '4px 8px',
            borderRadius: 4,
            backgroundColor: 'rgba(15, 23, 42, 0.8)',
            border: '1px solid rgba(55, 65, 81, 0.5)',
            backdropFilter: 'blur(5px)',
          }}
        >
          ESC
        </span>
        <span style={{ marginLeft: 8 }}>to go back</span>
      </div>
    </div>
  );
}
