import React, { useState, useEffect, useRef } from 'react';
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Play, Pause, Loader2, X } from "lucide-react";

interface MainDisplayProps {
  selectedVideo: string | null;
  status: string;
  onStartAnalysis: () => void;
  streamImage: string | null;
  isStreaming: boolean;
  selectedHistoricalFrame: { url: string; id: number } | null;
  onCloseHistoricalFrame: () => void;
}

export const MainDisplay: React.FC<MainDisplayProps> = ({ 
    selectedVideo, 
    status, 
    onStartAnalysis,
    streamImage,
    isStreaming,
    selectedHistoricalFrame,
    onCloseHistoricalFrame
}) => {
    if (!selectedVideo) {
        return (
            <div className="flex-1 flex items-center justify-center bg-muted/20 text-muted-foreground">
                Select a video to begin
            </div>
        );
    }

    // Determine what image to show
    let displayImage = null;
    let isHistorical = false;

    if (selectedHistoricalFrame) {
        displayImage = selectedHistoricalFrame.url;
        isHistorical = true;
    } else if (streamImage) {
        displayImage = streamImage;
    }

    return (
        <div className="flex-1 flex flex-col p-4 gap-4 h-full">
            <div className="flex justify-between items-center">
                <h2 className="text-xl font-bold flex items-center gap-2">
                    {selectedVideo}
                    {isHistorical && (
                        <span className="text-sm font-normal text-muted-foreground bg-muted px-2 py-0.5 rounded">
                            Viewing Frame {selectedHistoricalFrame?.id}
                        </span>
                    )}
                </h2>
                <div className="space-x-2">
                    {isHistorical ? (
                        <Button variant="outline" onClick={onCloseHistoricalFrame}>
                            <X className="mr-2 h-4 w-4" />
                            Close Frame
                        </Button>
                    ) : (
                        <>
                            {!isStreaming && (
                                <Button onClick={onStartAnalysis}>Start Live Analysis</Button>
                            )}
                            {isStreaming && (
                                <Button disabled variant="destructive">
                                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                    Live Streaming
                                </Button>
                            )}
                        </>
                    )}
                </div>
            </div>

            <Card className="flex-1 overflow-hidden flex items-center justify-center bg-black/5 relative">
                {displayImage ? (
                    <img 
                        src={displayImage} 
                        alt="Display" 
                        className="max-h-full max-w-full object-contain" 
                    />
                ) : (
                   isStreaming ? (
                       <div className="text-muted-foreground">Connecting to stream...</div>
                   ) : (
                       <div className="text-muted-foreground">Ready to start</div>
                   )
                )}
            </Card>
        </div>
    );
};
