import { useState } from 'react';
import { type TrafficViolation } from '@/lib/api';
import { CheckCircle2, XCircle, Edit2, Camera, Clock, MapPin, Gauge, AlertTriangle, FileText } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { apiClient } from '@/lib/api';
import { cn } from '@/lib/utils';
import { pdf } from '@react-pdf/renderer';
import { ChallanPDF } from './ChallanPDF';

interface ViolationDetailProps {
  violation: TrafficViolation;
  onApprove: (id: string) => void;
  onReject: (id: string, reason: string) => void;
  onClose: () => void;
}

export function ViolationDetail({ violation, onApprove, onReject, onClose }: ViolationDetailProps) {
  const [editingPlate, setEditingPlate] = useState(false);
  const [plateNumber, setPlateNumber] = useState(violation.plateNumber || '');
  const [rejectionReason, setRejectionReason] = useState('');
  const [showRejectDialog, setShowRejectDialog] = useState(false);

  const handleUpdatePlate = async () => {
    try {
      await apiClient.updateViolationPlate(violation.id, plateNumber);
      setEditingPlate(false);
    } catch (err) {
      console.error('Failed to update plate:', err);
      alert('Failed to update plate number');
    }
  };

  const handleReject = () => {
    if (!rejectionReason.trim()) {
      alert('Please provide a rejection reason');
      return;
    }
    onReject(violation.id, rejectionReason);
    setShowRejectDialog(false);
    setRejectionReason('');
  };

  const handleGenerateChallan = async () => {
    try {
      const blob = await pdf(<ChallanPDF violation={violation} />).toBlob();
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      const violationIdStr = String(violation.id);
      const challanNumber = `KSP-BGM-${violationIdStr.slice(0, 8).toUpperCase()}-${Date.now().toString().slice(-6)}`;
      link.download = `Challan-${challanNumber}.pdf`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
    } catch (err) {
      console.error('Failed to generate challan:', err);
      alert('Failed to generate challan PDF');
    }
  };

  const getViolationTypeColor = (type: string) => {
    const colors: Record<string, string> = {
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

  return (
    <Card className="glass h-full flex flex-col">
      {/* Header */}
      <div className="p-6 border-b border-white/10">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h2 className="text-2xl font-semibold mb-2">Verification Required</h2>
            <p className="text-sm text-gray-500 dark:text-gray-400">
              Review the violation evidence below. Approving will move it to the Approved tab.
            </p>
          </div>
          <div className="flex gap-2">
            {(violation.status === 'APPROVED' || violation.status === 'FINED') && (
              <Button
                variant="default"
                onClick={handleGenerateChallan}
                className="gap-2 bg-blue-600 hover:bg-blue-700"
              >
                <FileText className="w-4 h-4" />
                Generate Challan
              </Button>
            )}
            {violation.status === 'PENDING' && (
              <>
                <Button
                  variant="destructive"
                  onClick={() => setShowRejectDialog(true)}
                  className="gap-2"
                >
                  <XCircle className="w-4 h-4" />
                  Reject
                </Button>
                <Button
                  variant="default"
                  onClick={() => onApprove(violation.id)}
                  className="gap-2 bg-green-500 hover:bg-green-600"
                >
                  <CheckCircle2 className="w-4 h-4" />
                  Approve
                </Button>
              </>
            )}
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-6">
        <div className="grid grid-cols-2 gap-6">
          {/* Left Column - Images */}
          <div className="space-y-4">
            {/* Full Snapshot */}
            <div>
              <div className="flex items-center justify-between mb-2">
                <h3 className="text-lg font-semibold">FULL SNAPSHOT</h3>
                <Badge className={cn("text-xs", getViolationTypeColor(violation.violationType))}>
                  {violation.violationType}
                </Badge>
              </div>
              {violation.fullSnapshotUrl ? (
                <div className="relative rounded-lg overflow-hidden border border-white/10">
                  <img
                    src={violation.fullSnapshotUrl}
                    alt="Violation snapshot"
                    className="w-full h-auto"
                  />
                </div>
              ) : (
                <div className="aspect-video bg-gray-100 dark:bg-gray-800 rounded-lg flex items-center justify-center">
                  <Camera className="w-12 h-12 text-gray-400" />
                </div>
              )}
            </div>
          </div>

          {/* Right Column - Details */}
          <div className="space-y-4">
            {/* Violation Type */}
            <div>
              <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-2">VIOLATION TYPE</h3>
              <Badge className={cn("text-sm px-3 py-1", getViolationTypeColor(violation.violationType))}>
                {violation.violationType.toLowerCase()}
              </Badge>
            </div>

            {/* License Plate */}
            <div>
              <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-2">LICENSE PLATE</h3>
              {violation.plateImageUrl ? (
                <div className="relative rounded-lg overflow-hidden border border-white/10">
                  <img
                    src={violation.plateImageUrl}
                    alt="License plate"
                    className="w-full h-auto"
                  />
                </div>
              ) : (
                <div className="aspect-video bg-gray-100 dark:bg-gray-800 rounded-lg flex items-center justify-center">
                  <span className="text-gray-400">Plate</span>
                </div>
              )}
            </div>

            {/* Plate Number */}
            <div>
              <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-2">PLATE NUMBER</h3>
              {editingPlate ? (
                <div className="flex gap-2">
                  <Input
                    value={plateNumber}
                    onChange={(e) => setPlateNumber(e.target.value)}
                    className="flex-1"
                  />
                  <Button size="sm" onClick={handleUpdatePlate}>Save</Button>
                  <Button size="sm" variant="outline" onClick={() => {
                    setEditingPlate(false);
                    setPlateNumber(violation.plateNumber || '');
                  }}>Cancel</Button>
                </div>
              ) : (
                <div className="flex items-center gap-2">
                  <div className="px-3 py-2 bg-yellow-100 dark:bg-yellow-900/30 rounded-md font-mono font-semibold flex-1">
                    {violation.plateNumber || 'UNKNOWN'}
                  </div>
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => setEditingPlate(true)}
                  >
                    <Edit2 className="w-4 h-4" />
                  </Button>
                </div>
              )}
            </div>

            {/* Confidence */}
            {violation.confidence && (
              <div>
                <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-2">CONFIDENCE</h3>
                <div className="text-2xl font-semibold">
                  {(violation.confidence * 100).toFixed(1)}%
                </div>
              </div>
            )}

            {/* Speed Details (for speed violations) */}
            {violation.violationType === 'SPEED' && violation.detectedSpeed && (
              <div className="space-y-3">
                <div>
                  <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-2">SPEED</h3>
                  <div className="text-2xl font-semibold text-red-500">
                    {violation.detectedSpeed.toFixed(1)} km/h
                  </div>
                </div>
                <div>
                  <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-2">SPEED VIOLATION</h3>
                  <div className="space-y-2">
                    <Badge className="bg-purple-500">RADAR</Badge>
                    <div className="text-sm">
                      <div>Detected Speed: {violation.detectedSpeed.toFixed(1)} km/h</div>
                      <div className="text-gray-500 dark:text-gray-400">
                        Limit: {violation.speedLimit2W?.toFixed(0) || 'N/A'} km/h (2W) / {violation.speedLimit4W?.toFixed(0) || 'N/A'} km/h (4W)
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            )}

            {/* Time */}
            <div>
              <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-2">TIME</h3>
              <div className="flex items-center gap-2">
                <Clock className="w-4 h-4 text-gray-400" />
                <span className="font-mono">
                  {new Date(violation.timestamp).toLocaleTimeString('en-US', {
                    hour: '2-digit',
                    minute: '2-digit',
                    second: '2-digit',
                  })}
                </span>
              </div>
            </div>

            {/* Device Info */}
            {violation.device && (
              <div>
                <h3 className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-2">CAMERA</h3>
                <div className="flex items-center gap-2">
                  <MapPin className="w-4 h-4 text-gray-400" />
                  <span>{violation.device.name || violation.device.id}</span>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Reject Dialog */}
      {showRejectDialog && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <Card className="glass p-6 w-96">
            <h3 className="text-lg font-semibold mb-4">Reject Violation</h3>
            <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
              Please provide a reason for rejecting this violation:
            </p>
            <Input
              value={rejectionReason}
              onChange={(e) => setRejectionReason(e.target.value)}
              placeholder="Rejection reason..."
              className="mb-4"
            />
            <div className="flex gap-2 justify-end">
              <Button variant="outline" onClick={() => {
                setShowRejectDialog(false);
                setRejectionReason('');
              }}>
                Cancel
              </Button>
              <Button variant="destructive" onClick={handleReject}>
                Reject
              </Button>
            </div>
          </Card>
        </div>
      )}
    </Card>
  );
}

