import React from 'react';
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

interface Insight {
  count: number;
  timestamp: string;
  frame_id: number;
}

interface TimelineProps {
  data: Insight[];
  videoName: string | null;
  onFrameSelect: (frameId: number) => void;
}

export const Timeline: React.FC<TimelineProps> = ({ data, videoName, onFrameSelect }) => {
  const chartData = data.map(item => ({
    time: new Date(item.timestamp).toLocaleTimeString(),
    count: item.count,
    rawTimestamp: new Date(item.timestamp).getTime(),
    frame_id: item.frame_id
  }));

  // Sort by time just in case
  chartData.sort((a, b) => a.rawTimestamp - b.rawTimestamp);

  return (
    <Card className="h-full border-t rounded-none flex flex-col">
      <CardHeader className="py-2">
        <CardTitle className="text-sm">Processed Frames</CardTitle>
      </CardHeader>
      <CardContent className="flex-1 p-0 flex flex-col overflow-hidden">
        {/* Frames Strip */}
        <div className="h-full bg-muted/30 overflow-x-auto whitespace-nowrap p-4 flex gap-4">
            {videoName && chartData.map((item) => (
                <div 
                    key={item.frame_id} 
                    className="inline-block relative h-full aspect-video bg-black/10 rounded-lg overflow-hidden flex-shrink-0 cursor-pointer hover:ring-2 ring-primary transition-all shadow-md group"
                    onClick={() => onFrameSelect(item.frame_id)}
                >
                    <img 
                        src={`/api/processed_frame/${encodeURIComponent(videoName)}/${item.frame_id}`} 
                        alt={`Frame ${item.frame_id}`}
                        className="h-full w-full object-cover transition-transform group-hover:scale-105"
                        loading="lazy"
                    />
                    <div className="absolute bottom-0 left-0 right-0 bg-black/60 text-xs text-white p-1 truncate">
                        {item.time} • {item.count} ppl
                    </div>
                </div>
            ))}
            {chartData.length === 0 && <div className="text-sm text-muted-foreground p-4">No frames available</div>}
        </div>
      </CardContent>
    </Card>
  );
};

