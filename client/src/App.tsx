import { BrowserRouter, Routes, Route, useNavigate, useLocation } from 'react-router-dom';
import { ThemeProvider } from './contexts/ThemeContext';
import { DeviceFilterProvider } from './contexts/DeviceFilterContext';
import { LayerVisibilityProvider } from './contexts/LayerVisibilityContext';
import { CameraGridProvider } from './contexts/CameraGridContext';
import { CrowdDashboardProvider } from './contexts/CrowdDashboardContext';
import { MapTypeProvider } from './contexts/MapTypeContext';
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
  const isHomePage = activeView === 'home';
  const showTopBar = !isHomePage && activeView !== 'itms/vcc';
  const showSidebar = !isHomePage;

  return (
    <ThemeProvider>
      <DeviceFilterProvider>
        <LayerVisibilityProvider>
          <CameraGridProvider>
            <CrowdDashboardProvider>
              <MapTypeProvider>
                <div className="h-screen w-screen overflow-hidden bg-background text-foreground transition-colors duration-300">
                  {/* Sidebar - Hidden on home page */}
                  {showSidebar && <Sidebar activeView={activeView} onViewChange={handleViewChange} />}

                  {/* Top Bar - Hidden for VCC and Home pages */}
                  {showTopBar && <TopBar activeView={activeView} />}

                  {/* Main Content */}
                  <main
                    className={
                      isHomePage
                        ? "absolute inset-0" // Full screen for home
                        : showTopBar
                          ? "absolute top-16 left-20 right-0 bottom-0" // With sidebar and topbar
                          : "absolute top-0 left-20 right-0 bottom-0" // With sidebar, no topbar
                    }
                  >
                    <Routes>
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
                      <Route path="*" element={<HomePage />} />
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
    <BrowserRouter>
      <AppContent />
    </BrowserRouter>
  );
}

export default App;
