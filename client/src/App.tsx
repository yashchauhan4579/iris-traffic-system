import { BrowserRouter, Routes, Route, useNavigate, useLocation, Navigate, Outlet } from 'react-router-dom';
import { ThemeProvider } from './contexts/ThemeContext';
import { DeviceFilterProvider } from './contexts/DeviceFilterContext';
import { LayerVisibilityProvider } from './contexts/LayerVisibilityContext';
import { CameraGridProvider } from './contexts/CameraGridContext';
import { CrowdDashboardProvider } from './contexts/CrowdDashboardContext';
import { MapTypeProvider } from './contexts/MapTypeContext';
import { AuthProvider, useAuth } from './contexts/AuthContext';
import { Sidebar } from './components/layout/Sidebar';
import { TopBar } from './components/layout/TopBar';
import { HomePage } from './components/home/HomePage';
import { MapView } from './components/map/MapView';
import { CameraView } from './components/cameras/CameraView';
import { CrowdDashboard } from './components/crowd/CrowdDashboard';
import { ViolationsDashboard } from './components/violations/ViolationsDashboard';
import { ANPRDashboard } from './components/anpr/ANPRDashboard';
import { VCCDashboard } from './components/vcc/VCCDashboard';
import { WorkersDashboard } from './components/workers/WorkersDashboard';
import { LoginPage } from './pages/LoginPage';

function RequireAuth() {
  const { isAuthenticated, checkAuth } = useAuth();
  const location = useLocation();

  // Check both state and direct storage to avoid blips
  if (!isAuthenticated && !checkAuth()) {
    return <Navigate to="/login" state={{ from: location }} replace />;
  }

  return <Outlet />;
}

function AppContent() {
  const location = useLocation();
  const navigate = useNavigate();

  // Get active view from URL path
  const activeView = location.pathname === '/' ? 'home' :
    location.pathname.startsWith('/itms/') ? location.pathname.slice(1) :
      location.pathname.slice(1);

  const handleViewChange = (view: string) => {
    if (view === 'home') {
      navigate('/');
    } else if (view === 'map') {
      navigate('/map');
    } else {
      navigate(`/${view}`);
    }
  };

  // Determine if sidebar and topbar should be shown
  const isLoginPage = location.pathname === '/login';
  const isHomePage = activeView === 'home';
  const showTopBar = !isLoginPage && !isHomePage && activeView !== 'itms/vcc';
  const showSidebar = !isLoginPage && !isHomePage;

  return (
    <ThemeProvider>
      <DeviceFilterProvider>
        <LayerVisibilityProvider>
          <CameraGridProvider>
            <CrowdDashboardProvider>
              <MapTypeProvider>
                <div className="h-screen w-screen overflow-hidden bg-background text-foreground transition-colors duration-300">
                  {/* Sidebar - Hidden on home page and login */}
                  {showSidebar && <Sidebar activeView={activeView} onViewChange={handleViewChange} />}

                  {/* Top Bar - Hidden for VCC and Home pages and login */}
                  {showTopBar && <TopBar activeView={activeView} />}

                  {/* Main Content */}
                  <main
                    className={
                      isLoginPage || isHomePage
                        ? "absolute inset-0" // Full screen for login/home
                        : showTopBar
                          ? "absolute top-16 left-20 right-0 bottom-0" // With sidebar and topbar
                          : "absolute top-0 left-20 right-0 bottom-0" // With sidebar, no topbar
                    }
                  >
                    <Routes>
                      <Route path="/login" element={<LoginPage />} />

                      <Route element={<RequireAuth />}>
                        <Route path="/" element={<HomePage />} />
                        <Route path="/map" element={<MapView />} />
                        <Route path="/cameras" element={<CameraView />} />

                        {/* ITMS Routes */}
                        <Route path="/itms/anpr" element={<ANPRDashboard />} />
                        <Route path="/itms/vcc" element={<VCCDashboard />} />
                        <Route path="/itms/violations" element={<ViolationsDashboard />} />

                        {/* Crowd Routes */}
                        <Route path="/crowd" element={<CrowdDashboard />} />

                        {/* Other Routes */}
                        <Route path="/analytics" element={
                          <div className="flex items-center justify-center h-full">
                            <div className="glass rounded-2xl p-8">
                              <h2 className="text-2xl font-semibold mb-2">Analytics View</h2>
                              <p className="text-gray-500 dark:text-gray-400">Coming soon...</p>
                            </div>
                          </div>
                        } />
                        <Route path="/analytics/reports" element={
                          <div className="flex items-center justify-center h-full">
                            <div className="glass rounded-2xl p-8">
                              <h2 className="text-2xl font-semibold mb-2">Reports View</h2>
                              <p className="text-gray-500 dark:text-gray-400">Coming soon...</p>
                            </div>
                          </div>
                        } />
                        <Route path="/alerts" element={
                          <div className="flex items-center justify-center h-full">
                            <div className="glass rounded-2xl p-8">
                              <h2 className="text-2xl font-semibold mb-2">Alerts View</h2>
                              <p className="text-gray-500 dark:text-gray-400">Coming soon...</p>
                            </div>
                          </div>
                        } />
                        <Route path="/alerts/rules" element={
                          <div className="flex items-center justify-center h-full">
                            <div className="glass rounded-2xl p-8">
                              <h2 className="text-2xl font-semibold mb-2">Alert Rules View</h2>
                              <p className="text-gray-500 dark:text-gray-400">Coming soon...</p>
                            </div>
                          </div>
                        } />
                        <Route path="/settings" element={
                          <div className="flex items-center justify-center h-full">
                            <div className="glass rounded-2xl p-8">
                              <h2 className="text-2xl font-semibold mb-2">Settings View</h2>
                              <p className="text-gray-500 dark:text-gray-400">Coming soon...</p>
                            </div>
                          </div>
                        } />

                        {/* Settings - Workers */}
                        <Route path="/settings/workers" element={<WorkersDashboard />} />
                        <Route path="/settings/workers/:id" element={<WorkersDashboard />} />
                      </Route>

                      <Route path="*" element={<Navigate to="/" replace />} />
                    </Routes>
                  </main>
                </div>
              </MapTypeProvider>
            </CrowdDashboardProvider>
          </CameraGridProvider>
        </LayerVisibilityProvider>
      </DeviceFilterProvider>
    </ThemeProvider>
  );
}

function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <AppContent />
      </BrowserRouter>
    </AuthProvider>
  );
}

export default App;
