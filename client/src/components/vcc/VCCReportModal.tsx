import { useState, useEffect, useMemo } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Loader2, FileDown, RefreshCw, FileSpreadsheet } from 'lucide-react';
import { CameraSelector, type CameraOption } from '@/components/vcc/CameraSelector';
import { VCCReportPDF } from '@/components/vcc/VCCReportPDF';
import { PDFDownloadLink } from '@react-pdf/renderer';
import { apiClient, type VCCStats, type VCCDeviceStats } from '@/lib/api';
import { format } from 'date-fns';
import { DateTimeRangeContent, type DateTimeRange } from '@/components/vcc/DateTimeRangePicker';
import * as XLSX from 'xlsx';

interface VCCReportModalProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    cameras: CameraOption[];
    initialDateRange: { startDate: Date; endDate: Date };
    selectedCamera?: string | null;
}

export function VCCReportModal({ open, onOpenChange, cameras, initialDateRange, selectedCamera: propSelectedCamera }: VCCReportModalProps) {
    const [dateRange, setDateRange] = useState(initialDateRange);
    const [selectedCamera, setSelectedCamera] = useState<string | null>(propSelectedCamera || null);
    const [stats, setStats] = useState<VCCStats | VCCDeviceStats | null>(null);
    const [events, setEvents] = useState<any[]>([]);
    const [loading, setLoading] = useState(false);
    const [excelLoading, setExcelLoading] = useState(false);
    const [ready, setReady] = useState(false);

    useEffect(() => {
        if (open) {
            setReady(false);
            setStats(null);
            setEvents([]);
            setDateRange(initialDateRange);
            setSelectedCamera(propSelectedCamera || null);
        }
    }, [open, initialDateRange, propSelectedCamera]);

    const handleGenerate = async () => {
        try {
            setLoading(true);
            setReady(false);
            setEvents([]);

            // Ensure start < end
            let finalStart = dateRange.startDate;
            let finalEnd = dateRange.endDate;
            if (finalStart > finalEnd) {
                [finalStart, finalEnd] = [finalEnd, finalStart];
            }

            const params = {
                startTime: finalStart.toISOString(),
                endTime: finalEnd.toISOString(),
                groupBy: 'hour',
            };

            const eventsParams = {
                startTime: finalStart.toISOString(),
                endTime: finalEnd.toISOString(),
                deviceId: selectedCamera || undefined,
                limit: 30000,
            };

            const [statsData, eventsData] = await Promise.all([
                selectedCamera
                    ? apiClient.getVCCByDevice(selectedCamera, params as any)
                    : apiClient.getVCCStats(params as any),
                apiClient.getVCCEvents(eventsParams)
            ]);

            setStats(statsData);
            setEvents(eventsData.events || []);
            setReady(true);
        } catch (error) {
            console.error("Failed to generate report data:", error);
            alert("Failed to load report data.");
        } finally {
            setLoading(false);
        }
    };

    const handleDownloadExcel = () => {
        try {
            setExcelLoading(true);

            if (!events || events.length === 0) {
                alert("No events found for the selected range.");
                setExcelLoading(false);
                return;
            }

            // Prepare data for Excel
            const worksheetData = events.map(event => {
                const cam = cameras.find(c => c.id === event.deviceId);
                const location = cam?.metadata?.location || 'N/A';
                const cameraName = (event.device?.name || event.deviceId).replace(/^Camera\s+/i, "");

                return {
                    'Timestamp': format(new Date(event.timestamp), 'yyyy-MM-dd HH:mm:ss'),
                    'Vehicle Type': event.vehicleType,
                    'Camera Name': cameraName,
                    'Location': location,
                    'Direction': event.direction || 'Right' // Default to Right if missing matching backend default explanation
                };
            });

            // Create workbook and worksheet
            const worksheet = XLSX.utils.json_to_sheet(worksheetData);
            const workbook = XLSX.utils.book_new();
            XLSX.utils.book_append_sheet(workbook, worksheet, "VCC Events");

            // Auto-size columns
            const maxWidths = Object.keys(worksheetData[0]).map(key => ({
                wch: Math.max(key.length, ...worksheetData.map(row => String(row[key as keyof typeof row]).length)) + 2
            }));
            worksheet['!cols'] = maxWidths;

            // Trigger download
            let fileName = `iris_vcc_events_${format(new Date(), 'yyyy-MM-dd_HH-mm')}.xlsx`;

            if (selectedCamera) {
                const camName = cameras.find(c => c.id === selectedCamera)?.name || selectedCamera;
                const safeName = camName.replace(/^Camera\s+/i, "").replace(/\s+/g, '_');
                fileName = `iris_vcc_events_${safeName}_${format(new Date(), 'yyyy-MM-dd_HH-mm')}.xlsx`;
            }

            XLSX.writeFile(workbook, fileName);

        } catch (error) {
            console.error("Failed to download Excel:", error);
            alert("Failed to generate Excel file.");
        } finally {
            setExcelLoading(false);
        }
    };

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="max-w-3xl">
                <DialogHeader>
                    <DialogTitle>Generate Traffic Report</DialogTitle>
                    <DialogDescription>
                        Configure the date range and filters for your detailed report.
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-6 py-4">
                    <div className="space-y-2 px-1">
                        <label className="text-sm font-medium">Camera Filter</label>
                        <CameraSelector
                            cameras={cameras}
                            selectedCamera={selectedCamera}
                            onSelect={(c) => {
                                setSelectedCamera(c);
                                setReady(false);
                            }}
                            loading={false}
                        />
                    </div>

                    {/* Reusing the Shared DateTime Content Component */}
                    <div className="border rounded-md bg-muted/10">
                        <DateTimeRangeContent
                            value={dateRange}
                            onChange={(r) => {
                                setDateRange(r);
                                setReady(false);
                            }}
                            showFooter={false}
                        />
                    </div>
                </div>

                <DialogFooter className="flex-col sm:flex-row gap-2">
                    {!ready ? (
                        <>
                            <Button onClick={handleGenerate} disabled={loading} className="w-full sm:w-auto">
                                {loading ? <Loader2 className="w-4 h-4 animate-spin mr-2" /> : <RefreshCw className="w-4 h-4 mr-2" />}
                                Prepare Data
                            </Button>
                        </>
                    ) : (
                        <div className="flex gap-2 w-full sm:w-auto justify-end">
                            <Button variant="ghost" onClick={() => setReady(false)}>Modify Filters</Button>
                            <Button className="bg-white text-black hover:bg-zinc-200" onClick={handleDownloadExcel} disabled={excelLoading}>
                                {excelLoading ? <Loader2 className="w-4 h-4 animate-spin mr-2" /> : <FileSpreadsheet className="w-4 h-4 mr-2" />}
                                Download Excel
                            </Button>
                            {stats && (
                                <PDFDownloadButton
                                    stats={stats}
                                    dateRange={dateRange}
                                    cameras={cameras}
                                    selectedCamera={selectedCamera}
                                />
                            )}
                        </div>
                    )}
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}

// Extracted component to isolate PDF generation and state
function PDFDownloadButton({ stats, dateRange, cameras, selectedCamera }: {
    stats: VCCStats | VCCDeviceStats,
    dateRange: { startDate: Date, endDate: Date },
    cameras: CameraOption[],
    selectedCamera: string | null
}) {
    // Memoize the document to prevent regeneration on every render
    const doc = useMemo(() => (
        <VCCReportPDF
            stats={stats}
            startDate={dateRange.startDate}
            endDate={dateRange.endDate}
            selectedCameraName={selectedCamera ? cameras.find(c => c.id === selectedCamera)?.name : undefined}
            cameras={cameras}
        />
    ), [stats, dateRange, selectedCamera, cameras]);

    // Stable filename
    const fileName = useMemo(() => {
        const baseName = 'iris_atcc';
        const dateStr = format(new Date(), 'yyyy-MM-dd');

        if (selectedCamera) {
            const camName = cameras.find(c => c.id === selectedCamera)?.name || selectedCamera;
            const safeName = camName.replace(/^Camera\s+/i, "").replace(/\s+/g, '_');
            return `${baseName}_${safeName}_${dateStr}.pdf`;
        }

        return `${baseName}_${dateStr}.pdf`;
    }, [selectedCamera, cameras]);

    return (
        <PDFDownloadLink
            document={doc}
            fileName={fileName}
            className="w-full sm:w-auto"
        >
            {({ loading: pdfLoading }) => (
                <Button className="w-full sm:w-auto bg-white text-black hover:bg-zinc-200" disabled={pdfLoading}>
                    <FileDown className="w-4 h-4 mr-2" />
                    {pdfLoading ? 'Building PDF...' : 'Download PDF Report'}
                </Button>
            )}
        </PDFDownloadLink>
    );
}
