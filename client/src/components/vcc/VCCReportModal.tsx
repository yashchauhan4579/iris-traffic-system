import { useState, useEffect } from 'react';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Loader2, FileDown, RefreshCw, FileSpreadsheet } from 'lucide-react';
import { CameraSelector, type CameraOption } from '@/components/vcc/CameraSelector';
import { VCCReportPDF } from '@/components/vcc/VCCReportPDF';
import { PDFDownloadLink } from '@react-pdf/renderer';
import { apiClient, type VCCStats } from '@/lib/api';
import { format } from 'date-fns';
import { DateTimeRangeContent, type DateTimeRange } from '@/components/vcc/DateTimeRangePicker';
import * as XLSX from 'xlsx';

interface VCCReportModalProps {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    cameras: CameraOption[];
    initialDateRange: { startDate: Date; endDate: Date };
}

export function VCCReportModal({ open, onOpenChange, cameras, initialDateRange }: VCCReportModalProps) {
    const [dateRange, setDateRange] = useState(initialDateRange);
    const [selectedCamera, setSelectedCamera] = useState<string | null>(null);
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
        }
    }, [open, initialDateRange]);

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
            const worksheetData = events.map(event => ({
                'Timestamp': format(new Date(event.timestamp), 'yyyy-MM-dd HH:mm:ss'),
                'Vehicle Type': event.vehicleType,
                'Confidence': event.confidence ? (event.confidence * 100).toFixed(1) + '%' : 'N/A',
                'Camera Name': event.device?.name || event.deviceId,
                'Direction': event.direction || 'N/A'
            }));

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
            const fileName = `iris_vcc_events_${format(new Date(), 'yyyy-MM-dd_HH-mm')}.xlsx`;
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
                            <Button variant="outline" onClick={handleDownloadExcel} disabled={excelLoading}>
                                {excelLoading ? <Loader2 className="w-4 h-4 animate-spin mr-2" /> : <FileSpreadsheet className="w-4 h-4 mr-2" />}
                                Download Excel
                            </Button>
                            {stats && (
                                <PDFDownloadLink
                                    key={Date.now()}
                                    document={
                                        <VCCReportPDF
                                            stats={stats}
                                            startDate={dateRange.startDate}
                                            endDate={dateRange.endDate}
                                            selectedCameraName={selectedCamera ? cameras.find(c => c.id === selectedCamera)?.name : undefined}
                                        />
                                    }
                                    fileName={`iris_atcc_${format(new Date(), 'yyyy-MM-dd_HH-mm')}.pdf`}
                                    className="w-full sm:w-auto"
                                >
                                    {({ loading: pdfLoading }) => (
                                        <Button className="w-full sm:w-auto" disabled={pdfLoading}>
                                            <FileDown className="w-4 h-4 mr-2" />
                                            {pdfLoading ? 'Building PDF...' : 'Download PDF Report'}
                                        </Button>
                                    )}
                                </PDFDownloadLink>
                            )}
                        </div>
                    )}
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
