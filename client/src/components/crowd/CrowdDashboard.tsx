import { useState, useEffect } from 'react';
import { apiClient, type CrowdAnalysis } from '@/lib/api';
import { Users, AlertTriangle, Loader2 } from 'lucide-react';
import { useCrowdDashboard } from '@/contexts/CrowdDashboardContext';
import { cn } from '@/lib/utils';
import { CrowdDeviceSidebar } from './CrowdDeviceSidebar';

export function CrowdDashboard() {
  const [analyses, setAnalyses] = useState<CrowdAnalysis[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedAnalysis, setSelectedAnalysis] = useState<CrowdAnalysis | null>(null);
  const { autoRefresh } = useCrowdDashboard();

  const fetchAnalyses = async () => {
    try {
      setLoading(true);
      setError(null);
      // Get latest analysis for each device (one entry per device)
      const data = await apiClient.getLatestCrowdAnalysis();
      setAnalyses(data);
    } catch (err) {
      console.error('Failed to fetch crowd analysis:', err);
      setError('Failed to load crowd analysis data');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchAnalyses();
  }, []);

  useEffect(() => {
    if (!autoRefresh) return;

    const interval = setInterval(() => {
      fetchAnalyses();
    }, 5000); // Refresh every 5 seconds

    return () => clearInterval(interval);
  }, [autoRefresh]);

  if (loading && analyses.length === 0) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-center">
          <Loader2 className="w-8 h-8 animate-spin text-blue-500 mx-auto mb-2" />
          <p className="text-gray-500 dark:text-gray-400">Loading crowd analysis...</p>
        </div>
      </div>
    );
  }

  if (error && analyses.length === 0) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-center">
          <AlertTriangle className="w-8 h-8 text-red-500 mx-auto mb-2" />
          <p className="text-red-500">{error}</p>
        </div>
      </div>
    );
  }

  return (
    <>
      <div className={cn(
        "h-full w-full overflow-auto p-6 bg-gray-50 dark:bg-gray-900 transition-all",
        selectedAnalysis && "mr-96"
      )}>
        {/* Grid of Camera Cards */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-4">
          {analyses.map((analysis, index) => (
            <CameraCard
              key={analysis.id}
              analysis={analysis}
              onClick={() => setSelectedAnalysis(analysis)}
              isFirst={index === 0}
            />
          ))}
        </div>
      </div>

      {/* Sidebar */}
      {selectedAnalysis && (
        <CrowdDeviceSidebar
          analysis={selectedAnalysis}
          onClose={() => setSelectedAnalysis(null)}
        />
      )}
    </>
  );
}

function CameraCard({ analysis, onClick, isFirst }: { analysis: CrowdAnalysis; onClick: () => void; isFirst?: boolean }) {
  return (
    <div
      className={cn(
        "glass rounded-xl overflow-hidden border border-white/10 dark:border-white/5 hover:shadow-lg transition-shadow cursor-pointer",
        isFirst && "md:col-span-2 md:row-span-2"
      )}
      onClick={onClick}
    >
      {/* Image/Heatmap Section */}
      <div className={cn(
        "relative bg-gray-900",
        isFirst ? "h-96" : "h-48"
      )}>
        {analysis.heatmapImageUrl ? (
          <img
            src={analysis.heatmapImageUrl}
            alt={`Heatmap for ${analysis.device.name}`}
            className="w-full h-full object-cover"
          />
        ) : analysis.frameUrl ? (
          <img
            src={analysis.frameUrl}
            alt={`Frame for ${analysis.device.name}`}
            className="w-full h-full object-cover"
          />
        ) : (
          <div className="w-full h-full flex items-center justify-center bg-gray-800">
            <Users className="w-12 h-12 text-gray-600" />
          </div>
        )}
        {/* Device Name Overlay */}
        <div className="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/80 to-transparent p-3">
          <p className="text-sm font-medium text-white truncate">
            {analysis.device.name}
          </p>
        </div>
      </div>

      {/* Stats Section */}
      <div className="p-4">
        {/* Crowd Level Bar */}
        <div>
          <div className="flex items-center justify-between mb-1">
            <p className="text-xs text-gray-500 dark:text-gray-400">Crowd Level</p>
            <p className="text-xs font-semibold text-gray-900 dark:text-white">
              {analysis.crowdLevel}%
            </p>
          </div>
          <div className="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2.5">
            <div
              className={cn(
                "h-2.5 rounded-full transition-all duration-300",
                analysis.crowdLevel >= 75
                  ? "bg-red-500"
                  : analysis.crowdLevel >= 50
                    ? "bg-orange-500"
                    : analysis.crowdLevel >= 25
                      ? "bg-yellow-500"
                      : "bg-green-500"
              )}
              style={{ width: `${analysis.crowdLevel}%` }}
            />
          </div>
        </div>
      </div>
    </div>
  );
}


