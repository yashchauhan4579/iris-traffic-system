import { useState, useEffect } from 'react';
import { apiClient, type Vehicle, type VehicleDetection, type TrafficViolation } from '@/lib/api';
import { X, Eye, EyeOff, Edit2, MapPin, Clock, TrendingUp, AlertTriangle, Camera } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import { cn } from '@/lib/utils';

interface VehicleDetailProps {
  vehicle: Vehicle;
  onClose: () => void;
  onUpdate: () => void;
}

export function VehicleDetail({ vehicle, onUpdate, onClose }: VehicleDetailProps) {
  const [detections, setDetections] = useState<VehicleDetection[]>([]);
  const [violations, setViolations] = useState<TrafficViolation[]>([]);
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState('overview');
  const [editing, setEditing] = useState(false);
  const [editData, setEditData] = useState({
    plateNumber: vehicle.plateNumber || '',
    make: vehicle.make || '',
    model: vehicle.model || '',
    color: vehicle.color || '',
  });
  const [watchlistReason, setWatchlistReason] = useState('');
  const [showWatchlistDialog, setShowWatchlistDialog] = useState(false);

  useEffect(() => {
    fetchDetections();
    fetchViolations();
  }, [vehicle.id]);

  const fetchDetections = async () => {
    try {
      setLoading(true);
      const data = await apiClient.getVehicleDetections(vehicle.id, { limit: 50 });
      setDetections(data);
    } catch (err) {
      console.error('Failed to fetch detections:', err);
    } finally {
      setLoading(false);
    }
  };

  const fetchViolations = async () => {
    try {
      const data = await apiClient.getVehicleViolations(vehicle.id);
      setViolations(data);
    } catch (err) {
      console.error('Failed to fetch violations:', err);
    }
  };

  const handleUpdate = async () => {
    try {
      await apiClient.updateVehicle(vehicle.id, {
        plateNumber: editData.plateNumber || undefined,
        make: editData.make || undefined,
        model: editData.model || undefined,
        color: editData.color || undefined,
      });
      setEditing(false);
      onUpdate();
    } catch (err) {
      console.error('Failed to update vehicle:', err);
      alert('Failed to update vehicle');
    }
  };

  const handleAddToWatchlist = async () => {
    if (!watchlistReason.trim()) {
      alert('Please provide a reason');
      return;
    }
    try {
      await apiClient.addToWatchlist(vehicle.id, {
        reason: watchlistReason,
        addedBy: 'user',
        alertOnDetection: true,
        alertOnViolation: true,
      });
      setShowWatchlistDialog(false);
      setWatchlistReason('');
      onUpdate();
    } catch (err) {
      console.error('Failed to add to watchlist:', err);
      alert('Failed to add to watchlist');
    }
  };

  const handleRemoveFromWatchlist = async () => {
    try {
      await apiClient.removeFromWatchlist(vehicle.id);
      onUpdate();
    } catch (err) {
      console.error('Failed to remove from watchlist:', err);
      alert('Failed to remove from watchlist');
    }
  };

  const getVehicleTypeColor = (type: string) => {
    const colors: Record<string, string> = {
      '2W': 'bg-blue-500',
      '4W': 'bg-green-500',
      'AUTO': 'bg-yellow-500',
      'TRUCK': 'bg-red-500',
      'BUS': 'bg-purple-500',
      'UNKNOWN': 'bg-gray-500',
    };
    return colors[type] || 'bg-gray-500';
  };

  const formatDateTime = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  return (
    <Card className="glass h-full flex flex-col">
      {/* Header */}
      <div className="p-6 border-b border-white/10">
        <div className="flex items-center justify-between mb-4">
          <div className="flex-1">
            <div className="flex items-center gap-3 mb-2">
              <h2 className="text-2xl font-semibold font-mono">
                {vehicle.plateNumber || 'UNKNOWN VEHICLE'}
              </h2>
              {vehicle.isWatchlisted && (
                <Badge variant="warning" className="gap-1">
                  <Eye className="w-3 h-3" />
                  Watchlisted
                </Badge>
              )}
            </div>
            <div className="flex items-center gap-4 text-sm text-gray-500 dark:text-gray-400">
              {vehicle.make && vehicle.model && (
                <span>{vehicle.make} {vehicle.model}</span>
              )}
              <Badge className={cn("text-xs", getVehicleTypeColor(vehicle.vehicleType))}>
                {vehicle.vehicleType}
              </Badge>
              {vehicle.color && <span>Color: {vehicle.color}</span>}
            </div>
          </div>
          <div className="flex gap-2">
            {vehicle.isWatchlisted ? (
              <Button variant="outline" size="sm" onClick={handleRemoveFromWatchlist}>
                <EyeOff className="w-4 h-4 mr-2" />
                Remove from Watchlist
              </Button>
            ) : (
              <Button variant="outline" size="sm" onClick={() => setShowWatchlistDialog(true)}>
                <Eye className="w-4 h-4 mr-2" />
                Add to Watchlist
              </Button>
            )}
            <Button variant="ghost" size="sm" onClick={onClose}>
              <X className="w-4 h-4" />
            </Button>
          </div>
        </div>

        {/* Stats */}
        <div className="grid grid-cols-4 gap-4">
          <div>
            <div className="text-sm text-gray-500 dark:text-gray-400">Detections</div>
            <div className="text-2xl font-semibold">{vehicle.detectionCount}</div>
          </div>
          <div>
            <div className="text-sm text-gray-500 dark:text-gray-400">Violations</div>
            <div className="text-2xl font-semibold">{violations.length}</div>
          </div>
          <div>
            <div className="text-sm text-gray-500 dark:text-gray-400">First Seen</div>
            <div className="text-sm font-medium">{new Date(vehicle.firstSeen).toLocaleDateString()}</div>
          </div>
          <div>
            <div className="text-sm text-gray-500 dark:text-gray-400">Last Seen</div>
            <div className="text-sm font-medium">{new Date(vehicle.lastSeen).toLocaleDateString()}</div>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-6">
        <Tabs value={activeTab} onValueChange={setActiveTab}>
          <TabsList className="mb-4">
            <TabsTrigger value="overview">Overview</TabsTrigger>
            <TabsTrigger value="detections">Detections ({detections.length})</TabsTrigger>
            <TabsTrigger value="violations">Violations ({violations.length})</TabsTrigger>
            <TabsTrigger value="edit">Edit</TabsTrigger>
          </TabsList>

          <TabsContent value="overview">
            <div className="space-y-4">
              <Card className="p-4">
                <h3 className="font-semibold mb-3">Vehicle Information</h3>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <div className="text-sm text-gray-500 dark:text-gray-400 mb-1">Plate Number</div>
                    <div className="font-mono font-semibold">{vehicle.plateNumber || 'Not identified'}</div>
                  </div>
                  <div>
                    <div className="text-sm text-gray-500 dark:text-gray-400 mb-1">Vehicle Type</div>
                    <Badge className={getVehicleTypeColor(vehicle.vehicleType)}>
                      {vehicle.vehicleType}
                    </Badge>
                  </div>
                  {vehicle.make && (
                    <div>
                      <div className="text-sm text-gray-500 dark:text-gray-400 mb-1">Make</div>
                      <div>{vehicle.make}</div>
                    </div>
                  )}
                  {vehicle.model && (
                    <div>
                      <div className="text-sm text-gray-500 dark:text-gray-400 mb-1">Model</div>
                      <div>{vehicle.model}</div>
                    </div>
                  )}
                  {vehicle.color && (
                    <div>
                      <div className="text-sm text-gray-500 dark:text-gray-400 mb-1">Color</div>
                      <div>{vehicle.color}</div>
                    </div>
                  )}
                </div>
              </Card>

              {vehicle.watchlist && (
                <Card className="p-4">
                  <h3 className="font-semibold mb-3">Watchlist Information</h3>
                  <div className="space-y-2">
                    <div>
                      <div className="text-sm text-gray-500 dark:text-gray-400 mb-1">Reason</div>
                      <div>{vehicle.watchlist.reason}</div>
                    </div>
                    <div>
                      <div className="text-sm text-gray-500 dark:text-gray-400 mb-1">Added By</div>
                      <div>{vehicle.watchlist.addedBy}</div>
                    </div>
                    {vehicle.watchlist.notes && (
                      <div>
                        <div className="text-sm text-gray-500 dark:text-gray-400 mb-1">Notes</div>
                        <div>{vehicle.watchlist.notes}</div>
                      </div>
                    )}
                  </div>
                </Card>
              )}
            </div>
          </TabsContent>

          <TabsContent value="detections">
            <div className="space-y-3">
              {detections.map((detection) => (
                <Card key={detection.id} className="p-4">
                  <div className="flex items-start justify-between mb-2">
                    <div className="flex-1">
                      <div className="flex items-center gap-2 mb-2">
                        {detection.device && (
                          <div className="flex items-center gap-1 text-sm text-gray-500 dark:text-gray-400">
                            <MapPin className="w-4 h-4" />
                            {detection.device.name || detection.device.id}
                          </div>
                        )}
                        <div className="flex items-center gap-1 text-sm text-gray-500 dark:text-gray-400">
                          <Clock className="w-4 h-4" />
                          {formatDateTime(detection.timestamp)}
                        </div>
                      </div>
                      <div className="flex items-center gap-2">
                        {detection.plateDetected && (
                          <Badge variant="success" className="text-xs">Plate Detected</Badge>
                        )}
                        {detection.makeModelDetected && (
                          <Badge variant="success" className="text-xs">Make/Model Detected</Badge>
                        )}
                        <Badge className={cn("text-xs", getVehicleTypeColor(detection.vehicleType))}>
                          {detection.vehicleType}
                        </Badge>
                      </div>
                    </div>
                    {detection.fullImageUrl && (
                      <img
                        src={detection.fullImageUrl}
                        alt="Detection"
                        className="w-24 h-16 object-cover rounded"
                      />
                    )}
                  </div>
                </Card>
              ))}
              {detections.length === 0 && (
                <div className="text-center text-gray-500 dark:text-gray-400 py-8">
                  <Camera className="w-12 h-12 mx-auto mb-2 opacity-50" />
                  <p>No detections found</p>
                </div>
              )}
            </div>
          </TabsContent>

          <TabsContent value="violations">
            <div className="space-y-3">
              {violations.map((violation) => (
                <Card key={violation.id} className="p-4">
                  <div className="flex items-start justify-between mb-2">
                    <div className="flex-1">
                      <div className="flex items-center gap-2 mb-2">
                        <Badge className="bg-red-500">{violation.violationType}</Badge>
                        <Badge
                          variant={
                            violation.status === 'APPROVED' ? 'success' :
                            violation.status === 'REJECTED' ? 'destructive' :
                            violation.status === 'FINED' ? 'warning' : 'default'
                          }
                        >
                          {violation.status}
                        </Badge>
                      </div>
                      <div className="text-sm text-gray-500 dark:text-gray-400">
                        {formatDateTime(violation.timestamp)}
                      </div>
                      {violation.detectedSpeed && (
                        <div className="text-sm font-semibold text-red-500 mt-1">
                          Speed: {violation.detectedSpeed.toFixed(1)} km/h
                        </div>
                      )}
                    </div>
                    {violation.fullSnapshotUrl && (
                      <img
                        src={violation.fullSnapshotUrl}
                        alt="Violation"
                        className="w-24 h-16 object-cover rounded"
                      />
                    )}
                  </div>
                </Card>
              ))}
              {violations.length === 0 && (
                <div className="text-center text-gray-500 dark:text-gray-400 py-8">
                  <AlertTriangle className="w-12 h-12 mx-auto mb-2 opacity-50" />
                  <p>No violations found</p>
                </div>
              )}
            </div>
          </TabsContent>

          <TabsContent value="edit">
            <Card className="p-4">
              <h3 className="font-semibold mb-4">Edit Vehicle Information</h3>
              <div className="space-y-4">
                <div>
                  <label className="text-sm font-medium mb-1 block">Plate Number</label>
                  <Input
                    value={editData.plateNumber}
                    onChange={(e) => setEditData({ ...editData, plateNumber: e.target.value })}
                    placeholder="KA01AB1234"
                  />
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="text-sm font-medium mb-1 block">Make</label>
                    <Input
                      value={editData.make}
                      onChange={(e) => setEditData({ ...editData, make: e.target.value })}
                      placeholder="Honda"
                    />
                  </div>
                  <div>
                    <label className="text-sm font-medium mb-1 block">Model</label>
                    <Input
                      value={editData.model}
                      onChange={(e) => setEditData({ ...editData, model: e.target.value })}
                      placeholder="City"
                    />
                  </div>
                </div>
                <div>
                  <label className="text-sm font-medium mb-1 block">Color</label>
                  <Input
                    value={editData.color}
                    onChange={(e) => setEditData({ ...editData, color: e.target.value })}
                    placeholder="White"
                  />
                </div>
                <div className="flex gap-2">
                  <Button onClick={handleUpdate}>Save Changes</Button>
                  <Button variant="outline" onClick={() => setEditing(false)}>Cancel</Button>
                </div>
              </div>
            </Card>
          </TabsContent>
        </Tabs>
      </div>

      {/* Watchlist Dialog */}
      {showWatchlistDialog && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <Card className="glass p-6 w-96">
            <h3 className="text-lg font-semibold mb-4">Add to Watchlist</h3>
            <p className="text-sm text-gray-500 dark:text-gray-400 mb-4">
              Please provide a reason for adding this vehicle to the watchlist:
            </p>
            <Input
              value={watchlistReason}
              onChange={(e) => setWatchlistReason(e.target.value)}
              placeholder="Reason for watchlisting..."
              className="mb-4"
            />
            <div className="flex gap-2 justify-end">
              <Button variant="outline" onClick={() => {
                setShowWatchlistDialog(false);
                setWatchlistReason('');
              }}>
                Cancel
              </Button>
              <Button onClick={handleAddToWatchlist}>Add to Watchlist</Button>
            </div>
          </Card>
        </div>
      )}
    </Card>
  );
}

