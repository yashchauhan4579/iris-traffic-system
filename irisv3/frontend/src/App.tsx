import React, { useState, useEffect, useRef } from 'react';
import { VideoList } from './components/VideoList';
import { MainDisplay } from './components/MainDisplay';
import { InsightsSidebar } from './components/InsightsSidebar';
import { Timeline } from './components/Timeline';
import axios from 'axios';

interface Insight {
    count: number;
    density: string;
    movement?: string;
    flow_rate?: number;
    free_space?: number;
    congestion?: number;
    demographics?: string;
    behavior: string;
    alerts: string[];
    timestamp: string;
    frame_id: number;
}

function App() {
    const [videos, setVideos] = useState<string[]>([]);
    const [selectedVideo, setSelectedVideo] = useState<string | null>(null);
    
    // Analysis State
    const [status, setStatus] = useState<string>('idle');
    const [streamImage, setStreamImage] = useState<string | null>(null);
    const [currentInsight, setCurrentInsight] = useState<Insight | null>(null);
    const [history, setHistory] = useState<Insight[]>([]);
    const [isStreaming, setIsStreaming] = useState(false);
    const [selectedHistoricalFrame, setSelectedHistoricalFrame] = useState<{ url: string; id: number } | null>(null);
    
    const wsRef = useRef<WebSocket | null>(null);

    // Fetch videos on mount
    useEffect(() => {
        fetchVideos();
        return () => {
            if (wsRef.current) wsRef.current.close();
        };
    }, []);

    // Load history when selecting a video
    useEffect(() => {
        if (selectedVideo && status === 'idle') {
            fetchInsights(selectedVideo);
        }
    }, [selectedVideo]);

    const fetchVideos = async () => {
        try {
            const res = await axios.get('/api/videos');
            setVideos(res.data.videos);
        } catch (e) {
            console.error("Failed to fetch videos", e);
        }
    };

    const fetchInsights = async (video: string) => {
        try {
            const res = await axios.get(`/api/insights/${encodeURIComponent(video)}`);
            if (res.data.status === 'success') {
                setHistory(res.data.insights || []);
            }
        } catch (e) {
            console.error("Failed to fetch insights", e);
        }
    };

    const handleSelectVideo = (video: string) => {
        if (isStreaming) {
            if (wsRef.current) wsRef.current.close();
            setIsStreaming(false);
        }
        setSelectedVideo(video);
        setStatus('idle');
        setStreamImage(null);
        setCurrentInsight(null);
        setHistory([]); // Will be populated by useEffect
        setSelectedHistoricalFrame(null);
    };

    const handleFrameSelect = (frameId: number) => {
        if (selectedVideo) {
             const url = `/api/processed_frame/${encodeURIComponent(selectedVideo)}/${frameId}`;
             setSelectedHistoricalFrame({ url, id: frameId });
             
             // Update insight sidebar to match selected frame
             const frameInsight = history.find(h => h.frame_id === frameId);
             if (frameInsight) {
                 setCurrentInsight(frameInsight);
             }
        }
    };

    const handleCloseHistoricalFrame = () => {
        setSelectedHistoricalFrame(null);
        // Reset insight to latest if streaming or clear if not? 
        // For now, let's leave the last viewed insight or maybe clear it.
        // If we are streaming, currentInsight will update on next frame.
    };

    const startLiveAnalysis = () => {
        if (!selectedVideo) return;
        
        setIsStreaming(true);
        setStatus('processing');
        setSelectedHistoricalFrame(null); // Clear manual selection when starting stream
        
        // Connect to WebSocket
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        // When using Vite proxy, we might need absolute URL or handle proxy ws
        // Vite proxy handles ws usually if target is set. 
        // But for ws://localhost:8000/ws/analyze...
        // Let's try relative path if proxy is set up for ws, or direct port 8000
        
        // Use direct port 8000 for simplicity as Vite proxy ws support can be flaky without config
        const wsUrl = `ws://localhost:8000/ws/analyze/${encodeURIComponent(selectedVideo)}`;
        
        const ws = new WebSocket(wsUrl);
        wsRef.current = ws;

        ws.onopen = () => {
            console.log("Connected to WS");
        };

        ws.onmessage = (event) => {
            const data = JSON.parse(event.data);
            if (data.type === 'frame') {
                // Only update live view if not viewing a historical frame
                // User must close historical view to see new live frames
                if (!selectedHistoricalFrame) {
                    setStreamImage(data.image);
                }
                if (data.insight && !data.insight.error) {
                    setCurrentInsight(data.insight);
                    setHistory(prev => [...prev, data.insight]);
                }
            } else if (data.type === 'complete') {
                setIsStreaming(false);
                setStatus('completed');
                ws.close();
            }
        };

        ws.onerror = (e) => {
            console.error("WS Error", e);
            setIsStreaming(false);
            setStatus('failed');
        };

        ws.onclose = () => {
            console.log("WS Closed");
            setIsStreaming(false);
        };
    };

    return (
        <div className="flex h-screen w-full bg-background text-foreground overflow-hidden">
            {/* Left Sidebar */}
            <div className="w-64 h-full">
                <VideoList 
                    videos={videos} 
                    onSelect={handleSelectVideo} 
                    selectedVideo={selectedVideo} 
                />
            </div>

            {/* Middle Content */}
            <div className="flex-1 h-full overflow-hidden flex flex-col">
                <div className="flex-1 overflow-hidden">
                    <MainDisplay 
                        selectedVideo={selectedVideo}
                        status={status}
                        onStartAnalysis={startLiveAnalysis}
                        streamImage={streamImage}
                        isStreaming={isStreaming}
                        selectedHistoricalFrame={selectedHistoricalFrame}
                        onCloseHistoricalFrame={handleCloseHistoricalFrame}
                    />
                </div>
                {/* Timeline */}
                <div className="h-64">
                    <Timeline 
                        data={history} 
                        videoName={selectedVideo} 
                        onFrameSelect={handleFrameSelect}
                    />
                </div>
            </div>

            {/* Right Sidebar */}
            <div className="w-80 h-full">
                <InsightsSidebar 
                    currentInsight={currentInsight} 
                    history={history}
                />
            </div>
        </div>
    );
}

export default App;
