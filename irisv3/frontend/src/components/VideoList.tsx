import React from 'react';
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { PlayCircle } from "lucide-react";

interface VideoListProps {
  videos: string[];
  onSelect: (video: string) => void;
  selectedVideo: string | null;
}

export const VideoList: React.FC<VideoListProps> = ({ videos, onSelect, selectedVideo }) => {
  return (
    <Card className="h-full border-r rounded-none">
      <CardHeader>
        <CardTitle>Videos</CardTitle>
      </CardHeader>
      <CardContent className="space-y-2">
        {videos.map((video) => (
          <Button
            key={video}
            variant={selectedVideo === video ? "default" : "ghost"}
            className="w-full justify-start"
            onClick={() => onSelect(video)}
          >
            <PlayCircle className="mr-2 h-4 w-4" />
            {video}
          </Button>
        ))}
        {videos.length === 0 && <div className="text-sm text-muted-foreground">No videos found</div>}
      </CardContent>
    </Card>
  );
};

