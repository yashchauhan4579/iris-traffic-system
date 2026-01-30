import React from 'react';
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";
import { AlertTriangle, Users, Activity } from "lucide-react";

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

interface InsightsSidebarProps {
  currentInsight: Insight | null;
  history: Insight[];
}

export const InsightsSidebar: React.FC<InsightsSidebarProps> = ({ currentInsight }) => {
  // Density color mapping
  const getDensityColor = (density: string) => {
      switch(density?.toLowerCase()) {
          case 'low': return 'text-green-500';
          case 'medium': return 'text-yellow-500';
          case 'high': return 'text-orange-500';
          case 'critical': return 'text-red-600';
          default: return 'text-muted-foreground';
      }
  };

  return (
    <Card className="h-full border-l rounded-none flex flex-col">
      <CardHeader>
        <CardTitle>Real-time Insights</CardTitle>
      </CardHeader>
      <CardContent className="flex-1 flex flex-col gap-6 overflow-hidden">
        {currentInsight ? (
          <div className="space-y-6">
            
            {/* Condition - Full Row */}
            <Card className="p-3 flex flex-col items-center justify-center bg-muted/20 border-l-4 border-l-primary">
                <span className={`text-lg font-bold capitalize ${getDensityColor(currentInsight.density)}`}>
                    {currentInsight.density || 'Unknown'}
                </span>
                <span className="text-[10px] text-muted-foreground uppercase tracking-wider">Condition</span>
            </Card>

            {/* Congestion & Free Space - Single Row */}
            <div className="grid grid-cols-2 gap-2">
                <Card className="p-3 flex flex-col items-center justify-center bg-muted/20">
                    <span className="text-lg font-bold">{currentInsight.congestion !== undefined ? `${currentInsight.congestion}/10` : 'N/A'}</span>
                    <span className="text-[10px] text-muted-foreground uppercase tracking-wider">Congestion</span>
                </Card>
                <Card className="p-3 flex flex-col items-center justify-center bg-muted/20">
                    <span className="text-lg font-bold">{currentInsight.free_space !== undefined ? `${currentInsight.free_space}%` : 'N/A'}</span>
                    <span className="text-[10px] text-muted-foreground uppercase tracking-wider">Free Space</span>
                </Card>
            </div>

            {/* Demographics */}
            {currentInsight.demographics && (
                <div className="text-xs text-muted-foreground bg-muted/10 p-2 rounded border border-dashed">
                    <span className="font-semibold mr-1">Demographics:</span> {currentInsight.demographics}
                </div>
            )}

            {/* Movement Widget */}
            <Card className="p-4 bg-muted/20">
                <h4 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-2">Crowd Movement</h4>
                <div className="flex items-center gap-2">
                    {currentInsight.movement?.toLowerCase() === 'moving' ? (
                         <div className="flex items-center gap-2 text-blue-500">
                             <div className="h-2 w-2 rounded-full bg-blue-500 animate-pulse" />
                             <span className="font-bold">Moving</span>
                         </div>
                    ) : (
                        <div className="flex items-center gap-2 text-gray-500">
                             <div className="h-2 w-2 rounded-full bg-gray-500" />
                             <span className="font-bold">Static</span>
                        </div>
                    )}
                </div>
            </Card>

            {/* Behavior Widget */}
            <div className="space-y-2">
                <h4 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Observed Behavior</h4>
                <div className="bg-muted p-3 rounded-md text-sm border">
                    {currentInsight.behavior}
                </div>
            </div>

            {/* Alerts Widget */}
            <div className="space-y-2">
                <h4 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground flex items-center gap-2">
                    Safety Alerts
                </h4>
                <div className="flex flex-col gap-2">
                    {currentInsight.alerts.length > 0 && currentInsight.alerts[0] !== 'none' ? (
                        currentInsight.alerts.map((alert, i) => (
                            <div key={i} className="flex items-center gap-2 bg-red-100 dark:bg-red-900/30 text-red-600 dark:text-red-400 p-2 rounded border border-red-200 dark:border-red-900">
                                <AlertTriangle className="h-4 w-4 flex-shrink-0" />
                                <span className="text-sm font-medium">{alert}</span>
                            </div>
                        ))
                    ) : (
                        <div className="flex items-center gap-2 bg-green-100 dark:bg-green-900/30 text-green-600 dark:text-green-400 p-2 rounded border border-green-200 dark:border-green-900">
                            <span className="h-4 w-4 rounded-full border-2 border-current flex items-center justify-center text-[10px]">✓</span>
                            <span className="text-sm font-medium">No active alerts</span>
                        </div>
                    )}
                </div>
            </div>

          </div>
        ) : (
            <div className="flex-1 flex flex-col items-center justify-center text-center text-muted-foreground p-4">
                <Activity className="h-12 w-12 mb-4 opacity-20" />
                <p>Select a video and start analysis to view real-time crowd insights.</p>
            </div>
        )}
      </CardContent>
    </Card>
  );
};

