import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { 
  Server, 
  Plus, 
  RefreshCw, 
  CheckCircle, 
  XCircle, 
  Clock, 
  AlertTriangle,
  Cpu,
  HardDrive,
  Thermometer,
  Camera,
  Key,
  Copy,
  Trash2,
  MoreVertical
} from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { apiClient } from '@/lib/api';
import type { 
  WorkerWithCounts, 
  WorkerApprovalRequest, 
  WorkerTokenWithStatus,
  WorkerStatus
} from '@/lib/worker-types';

// Status badge component
function StatusBadge({ status }: { status: WorkerStatus | string }) {
  const config: Record<string, { color: string; icon: React.ReactNode; label: string }> = {
    active: { color: 'bg-green-500', icon: <CheckCircle className="w-3 h-3" />, label: 'Active' },
    approved: { color: 'bg-blue-500', icon: <CheckCircle className="w-3 h-3" />, label: 'Approved' },
    pending: { color: 'bg-yellow-500', icon: <Clock className="w-3 h-3" />, label: 'Pending' },
    offline: { color: 'bg-gray-500', icon: <XCircle className="w-3 h-3" />, label: 'Offline' },
    revoked: { color: 'bg-red-500', icon: <XCircle className="w-3 h-3" />, label: 'Revoked' },
  };
  
  const { color, icon, label } = config[status] || config.offline;
  
  return (
    <Badge className={`${color} text-white flex items-center gap-1`}>
      {icon}
      {label}
    </Badge>
  );
}

// Time ago helper
function timeAgo(date: string): string {
  const seconds = Math.floor((Date.now() - new Date(date).getTime()) / 1000);
  
  if (seconds < 60) return `${seconds}s ago`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
  return `${Math.floor(seconds / 86400)}d ago`;
}

export function WorkersDashboard() {
  const navigate = useNavigate();
  const [workers, setWorkers] = useState<WorkerWithCounts[]>([]);
  const [approvalRequests, setApprovalRequests] = useState<WorkerApprovalRequest[]>([]);
  const [tokens, setTokens] = useState<WorkerTokenWithStatus[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState('workers');
  const [creating, setCreating] = useState(false);

  const fetchData = async () => {
    setLoading(true);
    try {
      const [workersData, requestsData, tokensData] = await Promise.all([
        apiClient.getWorkers(),
        apiClient.getApprovalRequests('pending'),
        apiClient.getWorkerTokens(),
      ]);
      setWorkers(workersData);
      setApprovalRequests(requestsData);
      setTokens(tokensData);
    } catch (error) {
      console.error('Failed to fetch workers data:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
    const interval = setInterval(fetchData, 30000); // Refresh every 30s
    return () => clearInterval(interval);
  }, []);

  const handleApprove = async (requestId: string) => {
    try {
      await apiClient.approveWorkerRequest(requestId);
      fetchData();
    } catch (error) {
      console.error('Failed to approve request:', error);
    }
  };

  const handleReject = async (requestId: string) => {
    try {
      await apiClient.rejectWorkerRequest(requestId, 'Rejected by admin');
      fetchData();
    } catch (error) {
      console.error('Failed to reject request:', error);
    }
  };

  const handleCreateToken = async () => {
    setCreating(true);
    try {
      await apiClient.createWorkerToken({
        name: `Token ${new Date().toLocaleDateString()}`,
        expires_in: 168, // 7 days
      });
      fetchData();
    } catch (error) {
      console.error('Failed to create token:', error);
    } finally {
      setCreating(false);
    }
  };

  const handleCopyToken = (token: string) => {
    navigator.clipboard.writeText(token);
  };

  const handleRevokeToken = async (tokenId: string) => {
    try {
      await apiClient.revokeWorkerToken(tokenId);
      fetchData();
    } catch (error) {
      console.error('Failed to revoke token:', error);
    }
  };

  const handleDeleteWorker = async (workerId: string) => {
    if (!confirm('Are you sure you want to delete this worker?')) return;
    try {
      await apiClient.deleteWorker(workerId);
      fetchData();
    } catch (error) {
      console.error('Failed to delete worker:', error);
    }
  };

  const activeWorkers = workers.filter(w => w.status === 'active').length;
  const offlineWorkers = workers.filter(w => w.status === 'offline').length;
  const totalCameras = workers.reduce((sum, w) => sum + w.cameraCount, 0);

  return (
    <div className="p-6 space-y-6">
      {/* Header */}
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold flex items-center gap-2">
            <Server className="w-6 h-6" />
            Edge Workers
          </h1>
          <p className="text-gray-500 dark:text-gray-400">
            Manage edge computing nodes and camera assignments
          </p>
        </div>
        <Button onClick={fetchData} variant="outline" size="sm">
          <RefreshCw className={`w-4 h-4 mr-2 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </Button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card>
          <CardContent className="pt-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-gray-500">Total Workers</p>
                <p className="text-2xl font-bold">{workers.length}</p>
              </div>
              <Server className="w-8 h-8 text-blue-500 opacity-50" />
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-gray-500">Active</p>
                <p className="text-2xl font-bold text-green-500">{activeWorkers}</p>
              </div>
              <CheckCircle className="w-8 h-8 text-green-500 opacity-50" />
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-gray-500">Offline</p>
                <p className="text-2xl font-bold text-gray-500">{offlineWorkers}</p>
              </div>
              <XCircle className="w-8 h-8 text-gray-400 opacity-50" />
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-gray-500">Total Cameras</p>
                <p className="text-2xl font-bold">{totalCameras}</p>
              </div>
              <Camera className="w-8 h-8 text-purple-500 opacity-50" />
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Pending Approvals Alert */}
      {approvalRequests.length > 0 && (
        <Card className="border-yellow-500 bg-yellow-50 dark:bg-yellow-900/20">
          <CardHeader className="pb-2">
            <CardTitle className="text-lg flex items-center gap-2 text-yellow-700 dark:text-yellow-400">
              <AlertTriangle className="w-5 h-5" />
              {approvalRequests.length} Pending Approval{approvalRequests.length > 1 ? 's' : ''}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-2">
              {approvalRequests.map((req) => (
                <div key={req.id} className="flex items-center justify-between bg-white dark:bg-gray-800 p-3 rounded-lg">
                  <div>
                    <p className="font-medium">{req.deviceName}</p>
                    <p className="text-sm text-gray-500">
                      {req.model} • {req.ip} • {timeAgo(req.createdAt)}
                    </p>
                  </div>
                  <div className="flex gap-2">
                    <Button size="sm" onClick={() => handleApprove(req.id)} className="bg-green-500 hover:bg-green-600">
                      <CheckCircle className="w-4 h-4 mr-1" />
                      Approve
                    </Button>
                    <Button size="sm" variant="destructive" onClick={() => handleReject(req.id)}>
                      <XCircle className="w-4 h-4 mr-1" />
                      Reject
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Main Tabs */}
      <Tabs value={activeTab} onValueChange={setActiveTab}>
        <TabsList>
          <TabsTrigger value="workers">Workers ({workers.length})</TabsTrigger>
          <TabsTrigger value="tokens">Registration Tokens</TabsTrigger>
        </TabsList>

        <TabsContent value="workers" className="mt-4">
          <div className="grid gap-4">
            {workers.map((worker) => (
              <Card key={worker.id} className="hover:shadow-md transition-shadow">
                <CardContent className="pt-4">
                  <div className="flex items-start justify-between">
                    <div className="flex items-start gap-4">
                      <div className={`w-12 h-12 rounded-lg flex items-center justify-center ${
                        worker.status === 'active' ? 'bg-green-100 dark:bg-green-900/30' :
                        worker.status === 'offline' ? 'bg-gray-100 dark:bg-gray-800' :
                        'bg-yellow-100 dark:bg-yellow-900/30'
                      }`}>
                        <Server className={`w-6 h-6 ${
                          worker.status === 'active' ? 'text-green-600' :
                          worker.status === 'offline' ? 'text-gray-400' :
                          'text-yellow-600'
                        }`} />
                      </div>
                      <div>
                        <div className="flex items-center gap-2">
                          <h3 className="font-semibold">{worker.name}</h3>
                          <StatusBadge status={worker.status} />
                        </div>
                        <p className="text-sm text-gray-500 mt-1">
                          {worker.model} • {worker.ip}
                        </p>
                        <p className="text-xs text-gray-400 mt-1">
                          Last seen: {timeAgo(worker.lastSeen)}
                        </p>
                      </div>
                    </div>

                    <div className="flex items-center gap-6">
                      {/* Resources */}
                      {worker.resources && (
                        <div className="flex gap-4 text-sm">
                          <div className="flex items-center gap-1" title="CPU">
                            <Cpu className="w-4 h-4 text-gray-400" />
                            <span>{worker.resources.cpu_percent || 0}%</span>
                          </div>
                          <div className="flex items-center gap-1" title="GPU">
                            <HardDrive className="w-4 h-4 text-gray-400" />
                            <span>{worker.resources.gpu_percent || 0}%</span>
                          </div>
                          <div className="flex items-center gap-1" title="Temperature">
                            <Thermometer className="w-4 h-4 text-gray-400" />
                            <span>{worker.resources.temperature_c || 0}°C</span>
                          </div>
                        </div>
                      )}

                      {/* Camera count */}
                      <div className="flex items-center gap-1 px-3 py-1 bg-purple-100 dark:bg-purple-900/30 rounded-lg">
                        <Camera className="w-4 h-4 text-purple-600" />
                        <span className="font-medium text-purple-600">{worker.cameraCount}</span>
                      </div>

                      {/* Actions */}
                      <div className="flex gap-2">
                        <Button 
                          size="sm" 
                          variant="outline"
                          onClick={() => navigate(`/settings/workers/${worker.id}`)}
                        >
                          Configure
                        </Button>
                        <Button 
                          size="sm" 
                          variant="ghost"
                          onClick={() => handleDeleteWorker(worker.id)}
                        >
                          <Trash2 className="w-4 h-4 text-red-500" />
                        </Button>
                      </div>
                    </div>
                  </div>
                </CardContent>
              </Card>
            ))}

            {workers.length === 0 && !loading && (
              <Card>
                <CardContent className="py-12 text-center">
                  <Server className="w-12 h-12 mx-auto text-gray-300 mb-4" />
                  <h3 className="text-lg font-medium text-gray-500">No workers registered</h3>
                  <p className="text-gray-400 mt-1">Generate a token to register new edge workers</p>
                  <Button className="mt-4" onClick={() => setActiveTab('tokens')}>
                    <Key className="w-4 h-4 mr-2" />
                    Generate Token
                  </Button>
                </CardContent>
              </Card>
            )}
          </div>
        </TabsContent>

        <TabsContent value="tokens" className="mt-4">
          <Card>
            <CardHeader>
              <div className="flex justify-between items-center">
                <div>
                  <CardTitle>Registration Tokens</CardTitle>
                  <CardDescription>Generate tokens for edge workers to register with the platform</CardDescription>
                </div>
                <Button onClick={handleCreateToken} disabled={creating}>
                  <Plus className="w-4 h-4 mr-2" />
                  {creating ? 'Creating...' : 'Generate Token'}
                </Button>
              </div>
            </CardHeader>
            <CardContent>
              <div className="space-y-3">
                {tokens.map((token) => (
                  <div 
                    key={token.id} 
                    className={`flex items-center justify-between p-4 rounded-lg border ${
                      token.status === 'active' ? 'border-gray-200 dark:border-gray-700' :
                      token.status === 'used' ? 'border-green-200 bg-green-50 dark:bg-green-900/20' :
                      'border-gray-200 bg-gray-50 dark:bg-gray-800/50'
                    }`}
                  >
                    <div className="flex items-center gap-4">
                      <Key className={`w-5 h-5 ${
                        token.status === 'active' ? 'text-blue-500' :
                        token.status === 'used' ? 'text-green-500' :
                        'text-gray-400'
                      }`} />
                      <div>
                        <p className="font-medium">{token.name}</p>
                        <p className="text-xs text-gray-500 font-mono mt-1">
                          {token.token.substring(0, 20)}...
                        </p>
                        <p className="text-xs text-gray-400 mt-1">
                          Created {timeAgo(token.createdAt)}
                          {token.expiresAt && ` • Expires ${new Date(token.expiresAt).toLocaleDateString()}`}
                          {token.usedBy && ` • Used by ${token.usedBy}`}
                        </p>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <Badge variant={
                        token.status === 'active' ? 'default' :
                        token.status === 'used' ? 'secondary' :
                        'outline'
                      }>
                        {token.status}
                      </Badge>
                      {token.status === 'active' && (
                        <>
                          <Button 
                            size="sm" 
                            variant="ghost"
                            onClick={() => handleCopyToken(token.token)}
                          >
                            <Copy className="w-4 h-4" />
                          </Button>
                          <Button 
                            size="sm" 
                            variant="ghost"
                            onClick={() => handleRevokeToken(token.id)}
                          >
                            <XCircle className="w-4 h-4 text-red-500" />
                          </Button>
                        </>
                      )}
                    </div>
                  </div>
                ))}

                {tokens.length === 0 && (
                  <div className="text-center py-8 text-gray-500">
                    <Key className="w-8 h-8 mx-auto mb-2 opacity-50" />
                    <p>No tokens created yet</p>
                  </div>
                )}
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}

