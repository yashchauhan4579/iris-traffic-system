import { Card } from '@/components/ui/card';
import { type VCCStats, type VCCDeviceStats } from '@/lib/api';
import { Award, Clock, TrendingUp } from 'lucide-react';
import { cn } from '@/lib/utils';
import { type CameraOption } from '@/components/vcc/CameraSelector';

interface VCCInsightsProps {
    stats: VCCStats | VCCDeviceStats | null;
    loading?: boolean;
    cameras?: CameraOption[];
    isSingleCamera?: boolean;
}

export function VCCInsights({ stats, loading, cameras, isSingleCamera }: VCCInsightsProps) {
    if (loading || !stats) {
        return null;
    }

    // Helper to safely access stats properties which might vary between VCCStats and VCCDeviceStats
    const safeStats = stats as any;

    // 1. Most Active Camera (only for global stats)
    const getTopDevice = () => {
        if (!safeStats.byDevice || safeStats.byDevice.length === 0) return null;
        return safeStats.byDevice[0]; // Already sorted by backend
    };

    // 2. Peak Traffic Time
    const getPeakTime = () => {
        return {
            hour: safeStats.peakHour,
            day: safeStats.peakDay
        };
    };

    // 3. Dominant Vehicle Type
    const getDominantVehicle = () => {
        const types = safeStats.byVehicleType || {};
        let max = 0;
        let type = 'N/A';
        Object.entries(types).forEach(([k, v]) => {
            if (Number(v) > max) {
                max = Number(v);
                type = k;
            }
        });
        return { type, count: max };
    };

    // 4. Top 3 Peak & Quiet Hours
    const getHourlyInsights = () => {
        const hourly = safeStats.byHour || {};
        const hours = Object.entries(hourly).map(([h, count]) => ({
            hour: Number(h),
            count: Number(count)
        }));

        // Sort by count
        const sorted = [...hours].sort((a, b) => b.count - a.count);

        const top3Peak = sorted.slice(0, 3);
        const top3Quiet = [...sorted].reverse().slice(0, 3).sort((a, b) => a.count - b.count); // re-sort asc for display

        return { top3Peak, top3Quiet };
    };

    const topDevice = getTopDevice();

    // Resolve location for top device
    let topDeviceLocation = '';
    let topDeviceName = topDevice ? (topDevice.deviceName || topDevice.deviceId) : '';

    if (topDevice) {
        // Strip prefix
        topDeviceName = topDeviceName.replace(/^Camera\s+/i, "");

        // Find location
        if (cameras) {
            const cam = cameras.find(c => c.id === topDevice.deviceId);
            if (cam && cam.metadata && cam.metadata.location) {
                topDeviceLocation = cam.metadata.location;
            }
        }
    }

    const peakTime = getPeakTime();
    const dominantVehicle = getDominantVehicle();
    const { top3Peak, top3Quiet } = getHourlyInsights();
    const total = safeStats.totalDetections || 0;

    const insights = [
        ...(!isSingleCamera && topDevice ? [{
            title: 'Busiest Camera',
            value: topDeviceName,
            subtext: (
                <span>
                    {topDeviceLocation && <span className="block font-medium text-xs text-gray-400 mb-0.5">{topDeviceLocation}</span>}
                    {`${(topDevice.totalDetections || (topDevice as any).count || 0).toLocaleString()} detections`}
                </span>
            ),
            icon: Award,
            color: 'text-yellow-500',
            bgColor: 'bg-yellow-500/10'
        }] : []),
        {
            title: 'Peak Traffic',
            value: peakTime.hour !== undefined ? `${peakTime.hour}:00` : 'N/A',
            subtext: peakTime.day ? `On ${peakTime.day}` : null,
            icon: Clock,
            color: 'text-blue-500',
            bgColor: 'bg-blue-500/10'
        },
        {
            title: 'Dominant Vehicle',
            value: dominantVehicle.type,
            subtext: `${total > 0 ? ((dominantVehicle.count / total) * 100).toFixed(1) : 0}% of traffic`,
            icon: TrendingUp,
            color: 'text-green-500',
            bgColor: 'bg-green-500/10'
        }
    ];

    return (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
            {insights.map((insight, idx) => (
                <Card key={idx} className="glass p-4 transition-all hover:bg-white/5">
                    <div className="flex items-start justify-between">
                        <div>
                            <p className="text-xs text-gray-500 dark:text-gray-400 font-medium uppercase tracking-wider mb-1">
                                {insight.title}
                            </p>
                            <h3 className="text-xl font-bold truncate pr-2" title={typeof insight.value === 'string' ? insight.value : ''}>
                                {insight.value}
                            </h3>
                            <div className="text-sm text-gray-500 mt-1">
                                {insight.subtext}
                            </div>
                        </div>
                        <div className={cn("p-2 rounded-lg", insight.bgColor)}>
                            <insight.icon className={cn("w-5 h-5", insight.color)} />
                        </div>
                    </div>
                </Card>
            ))}

            {/* Hourly Insights Lists */}
            <div className="md:col-span-3 grid grid-cols-1 md:grid-cols-2 gap-4">
                <Card className="glass p-4">
                    <h3 className="text-sm font-semibold mb-3 flex items-center gap-2">
                        <TrendingUp className="w-4 h-4 text-red-500" />
                        Top 3 Peak Hours
                    </h3>
                    <div className="space-y-2">
                        {top3Peak.map((item, idx) => (
                            <div key={idx} className="flex items-center justify-between p-2 rounded bg-white/5 border border-white/5">
                                <span className="text-gray-300 font-medium">{item.hour}:00 - {item.hour + 1}:00</span>
                                <span className="font-bold text-red-400">{item.count.toLocaleString()}</span>
                            </div>
                        ))}
                    </div>
                </Card>

                <Card className="glass p-4">
                    <h3 className="text-sm font-semibold mb-3 flex items-center gap-2">
                        <Clock className="w-4 h-4 text-emerald-500" />
                        Top 3 Quiet Hours
                    </h3>
                    <div className="space-y-2">
                        {top3Quiet.map((item, idx) => (
                            <div key={idx} className="flex items-center justify-between p-2 rounded bg-white/5 border border-white/5">
                                <span className="text-gray-300 font-medium">{item.hour}:00 - {item.hour + 1}:00</span>
                                <span className="font-bold text-emerald-400">{item.count.toLocaleString()}</span>
                            </div>
                        ))}
                    </div>
                </Card>
            </div>
        </div>
    );
}
