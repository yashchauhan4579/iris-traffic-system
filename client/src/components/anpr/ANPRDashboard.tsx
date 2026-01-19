import { useState, useEffect } from 'react';
import { apiClient, type Vehicle, type VehicleType } from '@/lib/api';
import { Search, Filter, Loader2, Car, Eye, EyeOff, TrendingUp } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { cn } from '@/lib/utils';
import { VehicleDetail } from './VehicleDetail';

export function ANPRDashboard() {
  const [vehicles, setVehicles] = useState<Vehicle[]>([]);
  const [selectedVehicle, setSelectedVehicle] = useState<Vehicle | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [activeTab, setActiveTab] = useState('all');
  const [searchQuery, setSearchQuery] = useState('');
  const [filters, setFilters] = useState({
    vehicleType: '' as VehicleType | '',
    make: '',
    watchlisted: '',
  });

  const fetchVehicles = async () => {
    try {
      setLoading(true);
      setError(null);
      const result = await apiClient.getVehicles({
        plateNumber: searchQuery || undefined,
        vehicleType: filters.vehicleType || undefined,
        make: filters.make || undefined,
        watchlisted: filters.watchlisted === 'true' ? true : filters.watchlisted === 'false' ? false : undefined,
        limit: 100,
        orderBy: 'last_seen',
        orderDir: 'desc',
      });
      setVehicles(result.vehicles);
      setTotal(result.total);
    } catch (err) {
      console.error('Failed to fetch vehicles:', err);
      setError('Failed to load vehicles');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchVehicles();
  }, [activeTab, filters, searchQuery]);

  const getVehicleTypeColor = (type: VehicleType) => {
    const colors: Record<VehicleType, string> = {
      '2W': 'bg-blue-500',
      '4W': 'bg-green-500',
      'AUTO': 'bg-yellow-500',
      'TRUCK': 'bg-red-500',
      'BUS': 'bg-purple-500',
      'UNKNOWN': 'bg-gray-500',
    };
    return colors[type] || 'bg-gray-500';
  };

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
  };

  if (loading && vehicles.length === 0) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-center">
          <Loader2 className="w-8 h-8 animate-spin text-blue-500 mx-auto mb-2" />
          <p className="text-gray-500 dark:text-gray-400">Loading vehicles...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="h-full flex gap-4 p-4">
      {/* Left Panel - Search and List */}
      <div className="w-96 flex flex-col gap-4">
        <Card className="glass p-4">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-xl font-semibold">ANPR System</h2>
            <Badge variant="outline">{total} Vehicles</Badge>
          </div>

          {/* Search */}
          <div className="relative mb-4">
            <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-gray-400" />
            <Input
              placeholder="Search by plate number..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-10"
            />
          </div>

          {/* Tabs */}
          <Tabs value={activeTab} onValueChange={setActiveTab}>
            <TabsList className="grid w-full grid-cols-3 mb-4">
              <TabsTrigger value="all">All</TabsTrigger>
              <TabsTrigger value="watchlisted">Watchlist</TabsTrigger>
              <TabsTrigger value="recent">Recent</TabsTrigger>
            </TabsList>
          </Tabs>

          {/* Filters */}
          <div className="space-y-2 mb-4">
            <div className="flex items-center gap-2">
              <Filter className="w-4 h-4 text-gray-500" />
              <span className="text-sm font-medium">Filters</span>
            </div>
            <select
              value={filters.vehicleType}
              onChange={(e) => setFilters({ ...filters, vehicleType: e.target.value as VehicleType | '' })}
              className="w-full h-8 rounded-md border border-input bg-background px-3 py-1 text-sm"
            >
              <option value="">All Types</option>
              <option value="2W">2 Wheeler</option>
              <option value="4W">4 Wheeler</option>
              <option value="AUTO">Auto</option>
              <option value="TRUCK">Truck</option>
              <option value="BUS">Bus</option>
            </select>
            <Input
              placeholder="Make (e.g., Honda)"
              value={filters.make}
              onChange={(e) => setFilters({ ...filters, make: e.target.value })}
              className="h-8"
            />
            <select
              value={filters.watchlisted}
              onChange={(e) => setFilters({ ...filters, watchlisted: e.target.value })}
              className="w-full h-8 rounded-md border border-input bg-background px-3 py-1 text-sm"
            >
              <option value="">All</option>
              <option value="true">Watchlisted</option>
              <option value="false">Not Watchlisted</option>
            </select>
          </div>

          {/* Vehicle List */}
          <div className="space-y-2 max-h-[calc(100vh-400px)] overflow-y-auto">
            {vehicles.map((vehicle) => (
              <Card
                key={vehicle.id}
                className={cn(
                  "p-3 cursor-pointer transition-all hover:bg-white/50 dark:hover:bg-white/5",
                  selectedVehicle?.id === vehicle.id && "ring-2 ring-blue-500"
                )}
                onClick={() => setSelectedVehicle(vehicle)}
              >
                <div className="flex items-start justify-between mb-2">
                  <div className="flex-1">
                    <div className="flex items-center gap-2 mb-1">
                      <span className="font-semibold text-sm font-mono">
                        {vehicle.plateNumber || 'UNKNOWN'}
                      </span>
                      {vehicle.isWatchlisted && (
                        <Eye className="w-4 h-4 text-yellow-500" />
                      )}
                    </div>
                    <div className="flex items-center gap-2 mb-1">
                      {vehicle.make && vehicle.model && (
                        <span className="text-xs text-gray-500 dark:text-gray-400">
                          {vehicle.make} {vehicle.model}
                        </span>
                      )}
                      <Badge className={cn("text-xs", getVehicleTypeColor(vehicle.vehicleType))}>
                        {vehicle.vehicleType}
                      </Badge>
                    </div>
                    <div className="text-xs text-gray-500 dark:text-gray-400">
                      {vehicle.detectionCount} detections • Last seen {formatDate(vehicle.lastSeen)}
                    </div>
                  </div>
                </div>
              </Card>
            ))}
            {vehicles.length === 0 && !loading && (
              <div className="text-center text-gray-500 dark:text-gray-400 py-8">
                <Car className="w-12 h-12 mx-auto mb-2 opacity-50" />
                <p>No vehicles found</p>
              </div>
            )}
          </div>
        </Card>
      </div>

      {/* Right Panel - Detail View */}
      <div className="flex-1">
        {selectedVehicle ? (
          <VehicleDetail
            vehicle={selectedVehicle}
            onClose={() => setSelectedVehicle(null)}
            onUpdate={fetchVehicles}
          />
        ) : (
          <Card className="glass h-full flex items-center justify-center">
            <div className="text-center text-gray-500 dark:text-gray-400">
              <Car className="w-12 h-12 mx-auto mb-4 opacity-50" />
              <p>Select a vehicle to view details</p>
            </div>
          </Card>
        )}
      </div>
    </div>
  );
}

