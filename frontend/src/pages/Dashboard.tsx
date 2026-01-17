import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { RefreshCw, Users, MessageSquare, TrendingUp, ChevronRight } from 'lucide-react';
import Card from '../components/common/Card';
import Button from '../components/common/Button';
import { Badge } from '../components/ui/badge';
import { useAccountStore } from '../store/accountStore';
import { useReplyStore } from '../store/replyStore';
import { GetAccounts, GetWorkerStatus } from '../../wailsjs/go/main/App';

export default function Dashboard() {
    const navigate = useNavigate();
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
                    <div className="p-3 bg-primary/10 rounded-lg">
                        <Users className="text-primary" size={24} />
                    </div>
                    <div>
                        <p className="text-sm text-muted-foreground">Active Accounts</p>
                        <p className="text-2xl font-bold">{activeAccounts} / {accounts.length}</p>
                    </div>
                </Card>

                <Card className="flex items-center gap-4 p-4">
                    <div className="p-3 bg-yellow-500/10 rounded-lg">
                        <MessageSquare className="text-yellow-500" size={24} />
                    </div>
                    <div>
                        <p className="text-sm text-muted-foreground">Pending Replies</p>
                        <p className="text-2xl font-bold">{totalPending}</p>
                    </div>
                </Card>

                <Card className="flex items-center gap-4 p-4">
                    <div className="p-3 bg-green-500/10 rounded-lg">
                        <TrendingUp className="text-green-500" size={24} />
                    </div>
                    <div>
                        <p className="text-sm text-muted-foreground">Status</p>
                        <p className="text-2xl font-bold">
                            {activeAccounts > 0 ? 'Running' : 'Idle'}
                        </p>
                    </div>
                </Card>
            </div>

            {/* Accounts Overview */}
            <Card title="Accounts">
                {accounts.length === 0 ? (
                    <p className="text-muted-foreground text-center py-8">
                        No accounts configured. Go to Accounts to add one.
                    </p>
                ) : (
                    <div className="space-y-3">
                        {accounts.map((account) => (
                            <div
                                key={account.id}
                                className="flex items-center justify-between p-3 bg-secondary/50 rounded-lg border border-border cursor-pointer hover:border-primary/50 transition-colors"
                                onClick={() => navigate(`/accounts/${account.id}`)}
                            >
                                <div>
                                    <p className="font-medium">@{account.username}</p>
                                    <p className="text-sm text-muted-foreground">
                                        {account.authType === 'api' ? 'API' : 'Browser'} •{' '}
                                        {account.searchConfig.keywords.length} keywords
                                    </p>
                                </div>
                                <div className="flex items-center gap-2">
                                    <Badge variant={workerStatuses[account.id] ? 'default' : 'secondary'}>
                                        {workerStatuses[account.id] ? 'Running' : 'Stopped'}
                                    </Badge>
                                    <ChevronRight size={16} className="text-muted-foreground" />
                                </div>
                            </div>
                        ))}
                    </div>
                )}
            </Card>
        </div>
    );
}
