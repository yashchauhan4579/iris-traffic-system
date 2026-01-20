import { useState, useMemo } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Search, ArrowUpDown, ArrowUp, ArrowDown } from 'lucide-react';
import { type VCCStats } from '@/lib/api';

import { type CameraOption } from '@/components/vcc/CameraSelector';

interface VCCDevicesViewProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    devices: NonNullable<VCCStats['byDevice']>;
    totalDetections: number;
    onSelectCamera: (deviceId: string) => void;
    cameras: CameraOption[];
}

type SortField = 'count' | 'name';
type SortOrder = 'asc' | 'desc';

export function VCCDevicesView({ open, onOpenChange, devices, totalDetections, onSelectCamera, cameras }: VCCDevicesViewProps) {
    const [search, setSearch] = useState('');
    const [sortField, setSortField] = useState<SortField>('count');
    const [sortOrder, setSortOrder] = useState<SortOrder>('desc');

    const filteredAndSortedDevices = useMemo(() => {
        let result = [...devices];

        // Filter
        if (search) {
            const q = search.toLowerCase();
            result = result.filter(d =>
                (d.deviceName && d.deviceName.toLowerCase().includes(q)) ||
                d.deviceId.toLowerCase().includes(q)
            );
        }

        // Sort
        result.sort((a, b) => {
            let valA: any = sortField === 'name' ? (a.deviceName || a.deviceId) : Number(a.totalDetections);
            let valB: any = sortField === 'name' ? (b.deviceName || b.deviceId) : Number(b.totalDetections);

            if (sortField === 'name') {
                valA = valA.toLowerCase();
                valB = valB.toLowerCase();
            }

            if (valA < valB) return sortOrder === 'asc' ? -1 : 1;
            if (valA > valB) return sortOrder === 'asc' ? 1 : -1;
            return 0;
        });

        return result;
    }, [devices, search, sortField, sortOrder]);

    const toggleSort = (field: SortField) => {
        if (sortField === field) {
            setSortOrder(sortOrder === 'asc' ? 'desc' : 'asc');
        } else {
            setSortField(field);
            setSortOrder(field === 'count' ? 'desc' : 'asc');
        }
    };

    const SortIcon = ({ field }: { field: SortField }) => {
        if (sortField !== field) return <ArrowUpDown className="w-3 h-3 ml-1 opacity-50" />;
        return sortOrder === 'asc' ? <ArrowUp className="w-3 h-3 ml-1" /> : <ArrowDown className="w-3 h-3 ml-1" />;
    };

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="max-w-3xl h-[80vh] flex flex-col">
                <DialogHeader>
                    <DialogTitle>All Cameras</DialogTitle>
                    <DialogDescription>
                        View and select from all active cameras.
                    </DialogDescription>
                </DialogHeader>

                <div className="relative mb-4">
                    <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                    <Input
                        placeholder="Search cameras..."
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                        className="pl-9"
                    />
                </div>

                <div className="flex-1 overflow-y-auto border rounded-md">
                    <table className="w-full">
                        <thead className="sticky top-0 bg-background/95 backdrop-blur z-10 border-b">
                            <tr>
                                <th className="p-3 text-left text-xs font-medium text-muted-foreground w-16">Rank</th>
                                <th
                                    className="p-3 text-left text-xs font-medium text-muted-foreground cursor-pointer hover:bg-muted/50 transition-colors"
                                    onClick={() => toggleSort('name')}
                                >
                                    <div className="flex items-center">
                                        Device Name
                                        <SortIcon field="name" />
                                    </div>
                                </th>
                                <th
                                    className="p-3 text-right text-xs font-medium text-muted-foreground cursor-pointer hover:bg-muted/50 transition-colors"
                                    onClick={() => toggleSort('count')}
                                >
                                    <div className="flex items-center justify-end">
                                        Detections
                                        <SortIcon field="count" />
                                    </div>
                                </th>
                                <th className="p-3 text-right text-xs font-medium text-muted-foreground">%</th>
                                <th className="p-3 w-20"></th>
                            </tr>
                        </thead>
                        <tbody>
                            {filteredAndSortedDevices.length > 0 ? (
                                filteredAndSortedDevices.map((device, index) => {
                                    const cam = cameras.find(c => c.id === device.deviceId);
                                    const location = cam?.metadata?.location;

                                    return (
                                        <tr
                                            key={device.deviceId}
                                            className="border-b last:border-0 hover:bg-muted/50 transition-colors cursor-pointer group"
                                            onClick={() => {
                                                onSelectCamera(device.deviceId);
                                                onOpenChange(false);
                                            }}
                                        >
                                            <td className="p-3 text-xs text-muted-foreground">
                                                #{index + 1}
                                            </td>
                                            <td className="p-3">
                                                <div className="font-medium">{(device.deviceName || device.deviceId).replace(/^Camera\s+/i, "")}</div>
                                                {location && <div className="text-xs text-gray-500 font-medium mb-0.5">{location}</div>}
                                                <div className="text-xs text-muted-foreground font-mono">{device.deviceId}</div>
                                            </td>
                                            <td className="p-3 text-right">
                                                <div className="font-semibold">{Number(device.totalDetections).toLocaleString()}</div>
                                            </td>
                                            <td className="p-3 text-right text-xs text-muted-foreground">
                                                {totalDetections > 0 ? ((Number(device.totalDetections) / totalDetections) * 100).toFixed(1) : '0'}%
                                            </td>
                                            <td className="p-3 text-right">
                                                <Button variant="ghost" size="sm" className="h-8 opacity-0 group-hover:opacity-100 transition-opacity">
                                                    View
                                                </Button>
                                            </td>
                                        </tr>
                                    );
                                })
                            ) : (
                                <tr>
                                    <td colSpan={5} className="p-8 text-center text-muted-foreground">
                                        No cameras found matching "{search}"
                                    </td>
                                </tr>
                            )}
                        </tbody>
                    </table>
                </div>

                <div className="pt-2 text-xs text-muted-foreground flex justify-between">
                    <span>{filteredAndSortedDevices.length} cameras found</span>
                    <span>Total Detections: {totalDetections.toLocaleString()}</span>
                </div>
            </DialogContent>
        </Dialog>
    );
}
