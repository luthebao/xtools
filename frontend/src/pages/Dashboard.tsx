import { useEffect } from 'react';
import { Play, Pause, RefreshCw, Users, MessageSquare, TrendingUp } from 'lucide-react';
import Card from '../components/common/Card';
import Button from '../components/common/Button';
import { useAccountStore } from '../store/accountStore';
import { useReplyStore } from '../store/replyStore';
import { GetAccounts, GetWorkerStatus, StartAccount, StopAccount } from '../../wailsjs/go/main/App';

export default function Dashboard() {
  const { accounts, workerStatuses, setAccounts, setWorkerStatuses } = useAccountStore();
  const { pendingReplies } = useReplyStore();

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    try {
      const [accs, statuses] = await Promise.all([
        GetAccounts(),
        GetWorkerStatus(),
      ]);
      setAccounts(accs || []);
      setWorkerStatuses(statuses || {});
    } catch (err) {
      console.error('Failed to load data:', err);
    }
  };

  const toggleAccount = async (accountId: string) => {
    try {
      if (workerStatuses[accountId]) {
        await StopAccount(accountId);
      } else {
        await StartAccount(accountId);
      }
      const statuses = await GetWorkerStatus();
      setWorkerStatuses(statuses || {});
    } catch (err) {
      console.error('Failed to toggle account:', err);
    }
  };

  const totalPending = Object.values(pendingReplies).flat().length;
  const activeAccounts = Object.values(workerStatuses).filter(Boolean).length;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Dashboard</h1>
        <Button onClick={loadData} variant="ghost" size="sm">
          <RefreshCw size={16} />
          Refresh
        </Button>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card className="flex items-center gap-4 p-4">
          <div className="p-3 bg-blue-600/20 rounded-lg">
            <Users className="text-blue-400" size={24} />
          </div>
          <div>
            <p className="text-sm text-gray-400">Active Accounts</p>
            <p className="text-2xl font-bold">{activeAccounts} / {accounts.length}</p>
          </div>
        </Card>

        <Card className="flex items-center gap-4 p-4">
          <div className="p-3 bg-yellow-600/20 rounded-lg">
            <MessageSquare className="text-yellow-400" size={24} />
          </div>
          <div>
            <p className="text-sm text-gray-400">Pending Replies</p>
            <p className="text-2xl font-bold">{totalPending}</p>
          </div>
        </Card>

        <Card className="flex items-center gap-4 p-4">
          <div className="p-3 bg-green-600/20 rounded-lg">
            <TrendingUp className="text-green-400" size={24} />
          </div>
          <div>
            <p className="text-sm text-gray-400">Status</p>
            <p className="text-2xl font-bold">
              {activeAccounts > 0 ? 'Running' : 'Idle'}
            </p>
          </div>
        </Card>
      </div>

      {/* Accounts Overview */}
      <Card title="Accounts">
        {accounts.length === 0 ? (
          <p className="text-gray-400 text-center py-8">
            No accounts configured. Go to Accounts to add one.
          </p>
        ) : (
          <div className="space-y-3">
            {accounts.map((account) => (
              <div
                key={account.id}
                className="flex items-center justify-between p-3 bg-gray-700/50 rounded-lg"
              >
                <div>
                  <p className="font-medium">@{account.username}</p>
                  <p className="text-sm text-gray-400">
                    {account.authType === 'api' ? 'API' : 'Browser'} •{' '}
                    {account.searchConfig.keywords.length} keywords
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  <span
                    className={`px-2 py-1 text-xs rounded ${
                      workerStatuses[account.id]
                        ? 'bg-green-600/20 text-green-400'
                        : 'bg-gray-600/20 text-gray-400'
                    }`}
                  >
                    {workerStatuses[account.id] ? 'Running' : 'Stopped'}
                  </span>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => toggleAccount(account.id)}
                  >
                    {workerStatuses[account.id] ? (
                      <Pause size={16} />
                    ) : (
                      <Play size={16} />
                    )}
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}
