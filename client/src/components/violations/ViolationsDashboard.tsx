import { useState, useEffect, useCallback, useRef } from 'react';
import { apiClient, type TrafficViolation, type ViolationStatus, type ViolationType } from '@/lib/api';
import { AlertTriangle, Loader2, CheckCircle2, XCircle, Download, Calendar, Camera, Filter } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import { cn } from '@/lib/utils';
import { ViolationDetail } from './ViolationDetail';
import { pdf } from '@react-pdf/renderer';
import { ViolationsReportPDF } from './ViolationsReportPDF';

export function ViolationsDashboard() {
  const [violations, setViolations] = useState<TrafficViolation[]>([]);
  const [selectedViolation, setSelectedViolation] = useState<TrafficViolation | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<string>('PENDING');
  const [total, setTotal] = useState(0);
  const [filters, setFilters] = useState({
    violationType: '' as ViolationType | '',
    deviceId: '',
    plateNumber: '',
    date: '',
  });

  const isFetchingRef = useRef(false);

  const fetchViolations = useCallback(async (isInitialLoad = false) => {
    // Skip if already fetching
    if (isFetchingRef.current && !isInitialLoad) {
      return;
    }

    try {
      isFetchingRef.current = true;
      if (isInitialLoad) {
        setLoading(true);
      }
      setError(null);
      const result = await apiClient.getViolations({
        status: activeTab === 'ALL' ? undefined : (activeTab as ViolationStatus),
        violationType: filters.violationType || undefined,
        deviceId: filters.deviceId || undefined,
        plateNumber: filters.plateNumber || undefined,
        startTime: filters.date ? new Date(filters.date).toISOString() : undefined,
        limit: 100,
      });
      setViolations(result.violations);
      setTotal(result.total);
    } catch (err) {
      console.error('Failed to fetch violations:', err);
      setError('Failed to load violations');
    } finally {
      isFetchingRef.current = false;
      if (isInitialLoad) {
        setLoading(false);
      }
    }
  }, [activeTab, filters]);

  useEffect(() => {
    // Initial load
    fetchViolations(true);

    // Set up polling every second
    const intervalId = setInterval(() => {
      fetchViolations(false);
    }, 1000);

    // Cleanup interval on unmount or when dependencies change
    return () => {
      clearInterval(intervalId);
    };
  }, [fetchViolations]);

  const getStatusCount = async (status: ViolationStatus) => {
    try {
      const result = await apiClient.getViolations({ status, limit: 1 });
      return result.total;
    } catch {
      return 0;
    }
  };

  const handleApprove = async (id: string) => {
    try {
      await apiClient.approveViolation(id);
      fetchViolations();
      if (selectedViolation?.id === id) {
        setSelectedViolation(null);
      }
    } catch (err) {
      console.error('Failed to approve violation:', err);
      alert('Failed to approve violation');
    }
  };

  const handleReject = async (id: string, reason: string) => {
    try {
      await apiClient.rejectViolation(id, { rejectionReason: reason });
      fetchViolations();
      if (selectedViolation?.id === id) {
        setSelectedViolation(null);
      }
    } catch (err) {
      console.error('Failed to reject violation:', err);
      alert('Failed to reject violation');
    }
  };

  const getViolationTypeColor = (type: ViolationType) => {
    const colors: Record<ViolationType, string> = {
      SPEED: 'bg-red-500',
      HELMET: 'bg-orange-500',
      WRONG_SIDE: 'bg-yellow-500',
      RED_LIGHT: 'bg-purple-500',
      NO_SEATBELT: 'bg-pink-500',
      OVERLOADING: 'bg-indigo-500',
      ILLEGAL_PARKING: 'bg-gray-500',
      OTHER: 'bg-blue-500',
    };
    return colors[type] || 'bg-gray-500';
  };

  const formatTime = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' });
  };

  const handleGenerateReport = async () => {
    try {
      const generatedAt = new Date().toLocaleString('en-IN', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      });

      const reportTitle = `Traffic Violations Report - ${activeTab === 'ALL' ? 'All Violations' : activeTab + ' Violations'}`;

      const blob = await pdf(
        <ViolationsReportPDF
          violations={violations}
          reportTitle={reportTitle}
          generatedAt={generatedAt}
          filters={{
            status: activeTab === 'ALL' ? undefined : activeTab,
            violationType: filters.violationType || undefined,
            dateRange: filters.date || undefined,
          }}
        />
      ).toBlob();

      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `Violations-Report-${Date.now()}.pdf`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
    } catch (err) {
      console.error('Failed to generate report:', err);
      alert('Failed to generate report PDF');
    }
  };

  if (loading && violations.length === 0) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-center">
          <Loader2 className="w-8 h-8 animate-spin text-blue-500 mx-auto mb-2" />
          <p className="text-gray-500 dark:text-gray-400">Loading violations...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="h-full flex gap-4 p-4 bg-background/50">
      {/* Left Panel - Filters and List */}
      <div className="w-96 flex flex-col gap-4">
        <Card className="glass p-4">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-xl font-semibold">Violations</h2>
            <Button variant="outline" size="sm" onClick={handleGenerateReport}>
              <Download className="w-4 h-4 mr-2" />
              Report
            </Button>
          </div>

          <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v)}>
            <TabsList className="grid w-full grid-cols-4 mb-4">
              <TabsTrigger value="PENDING">Pending</TabsTrigger>
              <TabsTrigger value="APPROVED">Approved</TabsTrigger>
              <TabsTrigger value="FINED">Fines</TabsTrigger>
              <TabsTrigger value="ALL">All</TabsTrigger>
            </TabsList>
          </Tabs>

          {/* Filters */}
          <div className="space-y-2 mb-4">
            <div className="flex items-center gap-2">
              <Filter className="w-4 h-4 text-gray-500" />
              <span className="text-sm font-medium">Filters</span>
            </div>
            <Input
              placeholder="Plate Number"
              value={filters.plateNumber}
              onChange={(e) => setFilters({ ...filters, plateNumber: e.target.value })}
              className="h-8"
            />
            <Input
              type="date"
              value={filters.date}
              onChange={(e) => setFilters({ ...filters, date: e.target.value })}
              className="h-8"
            />
            <select
              value={filters.violationType}
              onChange={(e) => setFilters({ ...filters, violationType: e.target.value as ViolationType | '' })}
              className="w-full h-8 rounded-md border border-input bg-background px-3 py-1 text-sm"
            >
              <option value="">All Types</option>
              <option value="SPEED">Speed</option>
              <option value="HELMET">Helmet</option>
              <option value="WRONG_SIDE">Wrong Side</option>
              <option value="RED_LIGHT">Red Light</option>
              <option value="NO_SEATBELT">No Seatbelt</option>
              <option value="OVERLOADING">Overloading</option>
              <option value="ILLEGAL_PARKING">Illegal Parking</option>
              <option value="OTHER">Other</option>
            </select>
          </div>

          <div className="text-sm text-gray-500 dark:text-gray-400 mb-4">
            {total} Violations
          </div>

          {/* Violations List */}
          <div className="space-y-2 max-h-[calc(100vh-400px)] overflow-y-auto">
            {violations.map((violation) => (
              <Card
                key={violation.id}
                className={cn(
                  "p-3 cursor-pointer transition-all hover:bg-white/50 dark:hover:bg-white/5",
                  selectedViolation?.id === violation.id && "ring-2 ring-blue-500"
                )}
                onClick={() => setSelectedViolation(violation)}
              >
                <div className="flex items-start justify-between mb-2">
                  <div className="flex-1">
                    <div className="flex items-center gap-2 mb-1">
                      <span className="font-semibold text-sm">
                        {violation.plateNumber || 'UNKNOWN'}
                      </span>
                      <Badge
                        className={cn("text-xs", getViolationTypeColor(violation.violationType))}
                      >
                        {violation.violationType}
                      </Badge>
                    </div>
                    <div className="text-xs text-gray-500 dark:text-gray-400">
                      {formatTime(violation.timestamp)}
                    </div>
                  </div>
                  <Badge
                    variant={
                      violation.status === 'APPROVED' ? 'success' :
                        violation.status === 'REJECTED' ? 'destructive' :
                          violation.status === 'FINED' ? 'warning' : 'default'
                    }
                    className="text-xs"
                  >
                    {violation.status}
                  </Badge>
                </div>
              </Card>
            ))}
          </div>
        </Card>
      </div>

      {/* Right Panel - Detail View */}
      <div className="flex-1">
        {selectedViolation ? (
          <ViolationDetail
            violation={selectedViolation}
            onApprove={handleApprove}
            onReject={handleReject}
            onClose={() => setSelectedViolation(null)}
          />
        ) : (
          <Card className="glass h-full flex items-center justify-center">
            <div className="text-center text-gray-500 dark:text-gray-400">
              <AlertTriangle className="w-12 h-12 mx-auto mb-4 opacity-50" />
              <p>Select a violation to view details</p>
            </div>
          </Card>
        )}
      </div>
    </div>
  );
}

