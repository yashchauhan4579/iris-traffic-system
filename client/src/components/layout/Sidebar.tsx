import { Camera, Map, BarChart3, Settings, Bell, Users, Sun, Moon, AlertTriangle, Car, TrendingUp, Home, Server, LogOut } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useTheme } from '@/contexts/ThemeContext';
import { useAuth } from '@/contexts/AuthContext';

interface SidebarProps {
  activeView: string;
  onViewChange: (view: string) => void;
}

// Define module groups
const moduleGroups = {
  itms: {
    label: 'ITMS',
    color: 'purple',
    items: [
      { id: 'itms/vcc', icon: TrendingUp, label: 'VCC Dashboard' },
    ],
  },
  crowd: {
    label: 'Crowd',
    color: 'orange',
    items: [
      { id: 'map', icon: Map, label: 'Map View' },
      { id: 'crowd', icon: Users, label: 'Crowd Analytics' },
    ],
  },
  vms: {
    label: 'VMS',
    color: 'blue',
    items: [
      { id: 'cameras', icon: Camera, label: 'Camera Grid' },
    ],
  },
  analytics: {
    label: 'Analytics',
    color: 'cyan',
    items: [
      { id: 'analytics', icon: BarChart3, label: 'Analytics' },
    ],
  },
  alerts: {
    label: 'Alerts',
    color: 'red',
    items: [
      { id: 'alerts', icon: Bell, label: 'Alerts' },
    ],
  },
  settings: {
    label: 'Settings',
    color: 'gray',
    items: [
      { id: 'settings/workers', icon: Server, label: 'Edge Workers' },
      { id: 'settings', icon: Settings, label: 'Settings' },
    ],
  },
};

// Determine which module group the current view belongs to
function getActiveModule(activeView: string): string | null {
  if (activeView.startsWith('itms/') || activeView === 'itms') return 'itms';
  if (activeView === 'map' || activeView === 'crowd') return 'crowd';
  if (activeView === 'cameras') return 'vms';
  if (activeView === 'analytics') return 'analytics';
  if (activeView === 'alerts') return 'alerts';
  if (activeView.startsWith('settings') || activeView === 'settings') return 'settings';
  return null;
}

// Get color class for active state
function getColorClass(color: string, type: 'bg' | 'shadow') {
  const colors: Record<string, { bg: string; shadow: string }> = {
    purple: { bg: 'bg-purple-500', shadow: 'shadow-purple-500/30' },
    orange: { bg: 'bg-orange-500', shadow: 'shadow-orange-500/30' },
    blue: { bg: 'bg-blue-500', shadow: 'shadow-blue-500/30' },
    cyan: { bg: 'bg-cyan-500', shadow: 'shadow-cyan-500/30' },
    red: { bg: 'bg-red-500', shadow: 'shadow-red-500/30' },
    gray: { bg: 'bg-gray-500', shadow: 'shadow-gray-500/30' },
  };
  return colors[color]?.[type] || colors.blue[type];
}

export function Sidebar({ activeView, onViewChange }: SidebarProps) {
  const { theme, toggleTheme } = useTheme();
  const { logout } = useAuth();

  const activeModule = getActiveModule(activeView);
  const currentGroup = activeModule ? moduleGroups[activeModule as keyof typeof moduleGroups] : null;

  return (
    <div className="fixed left-0 top-0 h-screen w-20 glass border-r border-white/10 dark:border-white/5 z-50 flex flex-col items-center py-6 gap-4">
      {/* Logo / Home Button */}
      <button
        onClick={() => onViewChange('home')}
        className="w-12 h-12 rounded-2xl bg-gradient-to-br from-blue-500 to-blue-600 flex items-center justify-center mb-4 shadow-lg shadow-blue-500/20 hover:scale-105 transition-transform active:scale-95"
        title="Home"
      >
        <Home className="w-6 h-6 text-white" />
      </button>

      {/* Module Label */}
      {currentGroup && (
        <div className="px-2 mb-2">
          <div className="text-[10px] font-semibold text-gray-400 dark:text-gray-500 uppercase tracking-wider text-center">
            {currentGroup.label}
          </div>
        </div>
      )}

      {/* Navigation - Only show items for current module */}
      <nav className="flex-1 flex flex-col gap-2 w-full px-3 overflow-y-auto">
        {currentGroup?.items.map((item) => {
          const Icon = item.icon;
          const isActive = activeView === item.id;

          return (
            <button
              key={item.id}
              onClick={() => onViewChange(item.id)}
              className={cn(
                "w-14 h-14 rounded-xl flex items-center justify-center transition-all duration-200",
                "hover:bg-white/10 dark:hover:bg-white/5 active:scale-95",
                isActive
                  ? `${getColorClass(currentGroup.color, 'bg')} shadow-lg ${getColorClass(currentGroup.color, 'shadow')}`
                  : "bg-transparent"
              )}
              title={item.label}
            >
              <Icon className={cn(
                "w-6 h-6 transition-colors",
                isActive ? "text-white" : "text-gray-500 dark:text-gray-400"
              )} />
            </button>
          );
        })}
      </nav>

      {/* Dark Mode Toggle */}
      <button
        onClick={toggleTheme}
        className="w-14 h-14 rounded-xl flex items-center justify-center transition-all duration-200 hover:bg-white/10 dark:hover:bg-white/5 active:scale-95 mb-2"
        title={`Switch to ${theme === 'light' ? 'dark' : 'light'} mode`}
      >
        {theme === 'light' ? (
          <Moon className="w-5 h-5 text-gray-600" />
        ) : (
          <Sun className="w-5 h-5 text-yellow-400" />
        )}
      </button>

      {/* Logout Button */}
      <button
        onClick={logout}
        className="w-14 h-14 rounded-xl flex items-center justify-center transition-all duration-200 hover:bg-red-500/10 active:scale-95 mb-2 group"
        title="Sign Out"
      >
        <LogOut className="w-5 h-5 text-gray-500 group-hover:text-red-500 transition-colors" />
      </button>

      {/* User Avatar */}
      <div className="w-12 h-12 rounded-full bg-gradient-to-br from-gray-300 to-gray-400 dark:from-gray-600 dark:to-gray-700 flex items-center justify-center shadow-lg ring-2 ring-white/10">
        <span className="text-sm font-semibold text-gray-800 dark:text-gray-100">OP</span>
      </div>
    </div>
  );
}
