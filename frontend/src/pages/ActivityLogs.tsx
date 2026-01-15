import { useEffect, useState } from 'react';
import { RefreshCw, Trash2, AlertCircle, CheckCircle, Info, AlertTriangle } from 'lucide-react';
import Card from '../components/common/Card';
import Button from '../components/common/Button';
import { useAccountStore } from '../store/accountStore';
import { useUIStore } from '../store/uiStore';
import { ActivityLog } from '../types';
import { GetAccounts, GetActivityLogs, GetAllActivityLogs, ClearActivityLogs } from '../../wailsjs/go/main/App';

export default function ActivityLogs() {
  const { accounts, activeAccountId, setAccounts, setActiveAccount } = useAccountStore();
  const { showToast } = useUIStore();
  const [logs, setLogs] = useState<ActivityLog[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [autoRefresh, setAutoRefresh] = useState(true);

  useEffect(() => {
    loadAccounts();
  }, []);

  useEffect(() => {
    loadLogs(true);
  }, [activeAccountId]);

  // Auto-refresh every 3 seconds
  useEffect(() => {
    if (!autoRefresh) return;

    const interval = setInterval(() => {
      loadLogs();
    }, 3000);

    return () => clearInterval(interval);
  }, [autoRefresh, activeAccountId]);

  const loadAccounts = async () => {
    try {
      const accs = await GetAccounts();
      setAccounts(accs || []);
    } catch (err) {
      showToast('Failed to load accounts', 'error');
    }
  };

  const loadLogs = async (showLoading = false) => {
    if (showLoading) setIsLoading(true);
    try {
      let result: ActivityLog[];
      if (activeAccountId) {
        result = await GetActivityLogs(activeAccountId, 100);
      } else {
        result = await GetAllActivityLogs(200);
      }
      setLogs(result || []);
    } catch (err) {
      // Only show error if not auto-refreshing
      if (showLoading) showToast('Failed to load logs', 'error');
    } finally {
      if (showLoading) setIsLoading(false);
    }
  };

  const handleClearLogs = async () => {
    if (!activeAccountId) {
      showToast('Select an account to clear logs', 'warning');
      return;
    }
    try {
      await ClearActivityLogs(activeAccountId);
      setLogs([]);
      showToast('Logs cleared', 'success');
    } catch (err) {
      showToast('Failed to clear logs', 'error');
    }
  };

  const getLevelIcon = (level: string) => {
    switch (level) {
      case 'success':
        return <CheckCircle size={16} className="text-green-400" />;
      case 'error':
        return <AlertCircle size={16} className="text-red-400" />;
      case 'warning':
        return <AlertTriangle size={16} className="text-yellow-400" />;
      default:
        return <Info size={16} className="text-blue-400" />;
    }
  };

  const getLevelBg = (level: string) => {
    switch (level) {
      case 'success':
        return 'border-l-green-500';
      case 'error':
        return 'border-l-red-500';
      case 'warning':
        return 'border-l-yellow-500';
      default:
        return 'border-l-blue-500';
    }
  };

  const getTypeBadge = (type: string) => {
    const colors: Record<string, string> = {
      search: 'bg-blue-600/20 text-blue-400',
      reply: 'bg-green-600/20 text-green-400',
      auth: 'bg-purple-600/20 text-purple-400',
      error: 'bg-red-600/20 text-red-400',
      worker: 'bg-yellow-600/20 text-yellow-400',
      config: 'bg-gray-600/20 text-gray-400',
    };
    return colors[type] || 'bg-gray-600/20 text-gray-400';
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Activity Logs</h1>
      </div>

      <Card>
        <div className="flex items-center gap-4">
          <select
            value={activeAccountId || ''}
            onChange={(e) => setActiveAccount(e.target.value || null)}
            className="px-3 py-2 bg-gray-700 border border-gray-600 rounded-lg focus:ring-2 focus:ring-blue-500"
          >
            <option value="">All Accounts</option>
            {accounts.map((acc) => (
              <option key={acc.id} value={acc.id}>
                @{acc.username}
              </option>
            ))}
          </select>

          <Button variant="secondary" onClick={() => loadLogs(true)} disabled={isLoading}>
            <RefreshCw size={16} className={isLoading ? 'animate-spin' : ''} />
            Refresh
          </Button>

          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={autoRefresh}
              onChange={(e) => setAutoRefresh(e.target.checked)}
              className="rounded bg-gray-700 border-gray-600"
            />
            <span className="text-sm text-gray-400">Auto-refresh</span>
            {autoRefresh && (
              <span className="flex h-2 w-2">
                <span className="animate-ping absolute inline-flex h-2 w-2 rounded-full bg-green-400 opacity-75"></span>
                <span className="relative inline-flex rounded-full h-2 w-2 bg-green-500"></span>
              </span>
            )}
          </label>

          {activeAccountId && (
            <Button variant="danger" onClick={handleClearLogs}>
              <Trash2 size={16} />
              Clear Logs
            </Button>
          )}
        </div>
      </Card>

      <Card title={`Logs (${logs.length})`}>
        {logs.length === 0 ? (
          <p className="text-gray-400 text-center py-8">
            No activity logs yet. Activities will appear here as they happen.
          </p>
        ) : (
          <div className="space-y-2 max-h-[600px] overflow-y-auto">
            {logs.map((log) => (
              <div
                key={log.id}
                className={`p-3 bg-gray-700/50 rounded-lg border-l-4 ${getLevelBg(log.level)}`}
              >
                <div className="flex items-start gap-3">
                  <div className="mt-0.5">{getLevelIcon(log.level)}</div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                      <span className={`px-2 py-0.5 text-xs rounded ${getTypeBadge(log.type)}`}>
                        {log.type}
                      </span>
                      {log.accountId && (
                        <span className="text-xs text-gray-500">@{log.accountId}</span>
                      )}
                      <span className="text-xs text-gray-500 ml-auto">
                        {new Date(log.timestamp).toLocaleString()}
                      </span>
                    </div>
                    <p className="text-gray-200">{log.message}</p>
                    {log.details && (
                      <p className="text-sm text-gray-400 mt-1">{log.details}</p>
                    )}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}
