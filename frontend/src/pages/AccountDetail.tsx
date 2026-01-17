import { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
    ArrowLeft,
    Play,
    Pause,
    RefreshCw,
    Trash2,
    Edit2,
    Check,
    X,
    CheckCircle,
    XCircle,
    Clock,
    AlertCircle,
    AlertTriangle,
    Info,
} from 'lucide-react';
import Card from '../components/common/Card';
import Button from '../components/common/Button';
import { Badge } from '../components/ui/badge';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../components/ui/tabs';
import { useAccountStore } from '../store/accountStore';
import { useReplyStore } from '../store/replyStore';
import { useUIStore } from '../store/uiStore';
import { AccountConfig, ActivityLog, ApprovalQueueItem, Reply } from '../types';
import {
    GetAccounts,
    GetWorkerStatus,
    StartAccount,
    StopAccount,
    GetActivityLogs,
    ClearActivityLogs,
    GetPendingReplies,
    GetReplyHistory,
    ApproveReply,
    RejectReply,
} from '../../wailsjs/go/main/App';
import AccountEditor from '../components/AccountEditor';

export default function AccountDetail() {
    const { accountId } = useParams<{ accountId: string }>();
    const navigate = useNavigate();
    const { workerStatuses, setAccounts, setWorkerStatuses } = useAccountStore();
    const { pendingReplies, setPendingReplies, removePendingReply } = useReplyStore();
    const { showToast } = useUIStore();

    const [account, setAccount] = useState<AccountConfig | null>(null);
    const [logs, setLogs] = useState<ActivityLog[]>([]);
    const [replyHistory, setReplyHistory] = useState<Reply[]>([]);
    const [isLoading, setIsLoading] = useState(false);
    const [autoRefresh, setAutoRefresh] = useState(true);
    const [editingAccount, setEditingAccount] = useState<AccountConfig | null>(null);
    const [activeTab, setActiveTab] = useState('actions');

    useEffect(() => {
        loadData();
    }, [accountId]);

    // Auto-refresh logs every 3 seconds
    useEffect(() => {
        if (!autoRefresh || !accountId) return;

        const interval = setInterval(() => {
            loadLogs();
            loadReplies();
        }, 3000);

        return () => clearInterval(interval);
    }, [autoRefresh, accountId]);

    const loadData = async () => {
        if (!accountId) return;
        setIsLoading(true);
        try {
            const [accs, statuses] = await Promise.all([
                GetAccounts(),
                GetWorkerStatus(),
            ]);
            setAccounts(accs || []);
            setWorkerStatuses(statuses || {});

            const acc = accs?.find((a: AccountConfig) => a.id === accountId);
            if (acc) {
                setAccount(acc);
            } else {
                showToast('Account not found', 'error');
                navigate('/accounts');
                return;
            }

            await Promise.all([loadLogs(), loadReplies()]);
        } catch (err: any) {
            showToast(err?.message || 'Failed to load account', 'error');
        } finally {
            setIsLoading(false);
        }
    };

    const loadLogs = async () => {
        if (!accountId) return;
        try {
            const result = await GetActivityLogs(accountId, 100);
            setLogs(result || []);
        } catch (err) {
            // Silent fail for auto-refresh
        }
    };

    const loadReplies = async () => {
        if (!accountId) return;
        try {
            const [pending, history] = await Promise.all([
                GetPendingReplies(accountId),
                GetReplyHistory(accountId, 50),
            ]);
            setPendingReplies(accountId, pending || []);
            setReplyHistory(history || []);
        } catch (err) {
            // Silent fail for auto-refresh
        }
    };

    const toggleWorker = async () => {
        if (!accountId) return;
        try {
            if (workerStatuses[accountId]) {
                await StopAccount(accountId);
                showToast('Account stopped', 'info');
            } else {
                await StartAccount(accountId);
                showToast('Account started', 'success');
            }
            const statuses = await GetWorkerStatus();
            setWorkerStatuses(statuses || {});
        } catch (err: any) {
            showToast(err?.message || 'Failed to toggle account', 'error');
        }
    };

    const handleClearLogs = async () => {
        if (!accountId) return;
        try {
            await ClearActivityLogs(accountId);
            setLogs([]);
            showToast('Logs cleared', 'success');
        } catch (err) {
            showToast('Failed to clear logs', 'error');
        }
    };

    const handleApprove = async (replyId: string) => {
        try {
            await ApproveReply(replyId);
            if (accountId) {
                removePendingReply(accountId, replyId);
            }
            showToast('Reply approved and posted', 'success');
            loadReplies();
        } catch (err: any) {
            const errorMsg = typeof err === 'string' ? err : (err?.message || 'Failed to approve reply');
            showToast(errorMsg, 'error');
        }
    };

    const handleReject = async (replyId: string) => {
        try {
            await RejectReply(replyId);
            if (accountId) {
                removePendingReply(accountId, replyId);
            }
            showToast('Reply rejected', 'info');
            loadReplies();
        } catch (err: any) {
            const errorMsg = typeof err === 'string' ? err : (err?.message || 'Failed to reject reply');
            showToast(errorMsg, 'error');
        }
    };

    const handleSaveAccount = async () => {
        setEditingAccount(null);
        await loadData();
    };

    if (!account) {
        return (
            <div className="flex items-center justify-center h-64">
                <RefreshCw className="animate-spin text-muted-foreground" size={32} />
            </div>
        );
    }

    const currentPending = accountId ? pendingReplies[accountId] || [] : [];
    const isRunning = accountId ? workerStatuses[accountId] : false;

    return (
        <div className="space-y-6">
            {/* Header */}
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                    <Button variant="ghost" size="sm" onClick={() => navigate('/accounts')}>
                        <ArrowLeft size={16} />
                        Back
                    </Button>
                    <div>
                        <div className="flex items-center gap-3">
                            <h1 className="text-2xl font-bold">@{account.username}</h1>
                            <Badge variant={isRunning ? 'default' : 'secondary'}>
                                {isRunning ? 'Running' : 'Stopped'}
                            </Badge>
                        </div>
                        <p className="text-sm text-muted-foreground">
                            {account.searchConfig.keywords.length} keywords | {account.replyConfig.approvalMode} mode
                        </p>
                    </div>
                </div>
                <div className="flex items-center gap-2">
                    <Button variant="ghost" size="sm" onClick={() => setEditingAccount(account)}>
                        <Edit2 size={16} />
                        Edit
                    </Button>
                    <Button variant="ghost" size="sm" onClick={() => loadData()} disabled={isLoading}>
                        <RefreshCw size={16} className={isLoading ? 'animate-spin' : ''} />
                        Refresh
                    </Button>
                </div>
            </div>

            {/* Tabs */}
            <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
                <TabsList className="grid w-full grid-cols-3">
                    <TabsTrigger value="actions">Actions</TabsTrigger>
                    <TabsTrigger value="logs">
                        Logs {logs.length > 0 && <span className="ml-1 text-xs">({logs.length})</span>}
                    </TabsTrigger>
                    <TabsTrigger value="replies">
                        Replies {currentPending.length > 0 && <span className="ml-1 text-xs text-yellow-500">({currentPending.length})</span>}
                    </TabsTrigger>
                </TabsList>

                {/* Actions Tab */}
                <TabsContent value="actions" className="space-y-4 mt-4">
                    <Card title="Worker Control">
                        <div className="space-y-4">
                            <div className="flex items-center justify-between p-4 bg-secondary/50 rounded-lg border border-border">
                                <div>
                                    <p className="font-medium">Auto Reply with Search</p>
                                    <p className="text-sm text-muted-foreground">
                                        Automatically search for tweets and generate replies based on your configuration
                                    </p>
                                </div>
                                <Button
                                    variant={isRunning ? 'secondary' : 'primary'}
                                    onClick={toggleWorker}
                                >
                                    {isRunning ? (
                                        <>
                                            <Pause size={16} />
                                            Stop
                                        </>
                                    ) : (
                                        <>
                                            <Play size={16} />
                                            Start
                                        </>
                                    )}
                                </Button>
                            </div>

                            {isRunning && (
                                <div className="p-3 bg-green-500/10 border border-green-500/20 rounded-lg">
                                    <div className="flex items-center gap-2">
                                        <span className="flex h-2 w-2">
                                            <span className="animate-ping absolute inline-flex h-2 w-2 rounded-full bg-green-400 opacity-75"></span>
                                            <span className="relative inline-flex rounded-full h-2 w-2 bg-green-500"></span>
                                        </span>
                                        <span className="text-sm text-green-400">Worker is active and searching...</span>
                                    </div>
                                </div>
                            )}
                        </div>
                    </Card>

                    <Card title="Quick Stats">
                        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                            <div className="p-4 bg-secondary/50 rounded-lg text-center">
                                <p className="text-2xl font-bold">{logs.length}</p>
                                <p className="text-xs text-muted-foreground">Activity Logs</p>
                            </div>
                            <div className="p-4 bg-secondary/50 rounded-lg text-center">
                                <p className="text-2xl font-bold text-yellow-500">{currentPending.length}</p>
                                <p className="text-xs text-muted-foreground">Pending Replies</p>
                            </div>
                            <div className="p-4 bg-secondary/50 rounded-lg text-center">
                                <p className="text-2xl font-bold text-green-500">
                                    {replyHistory.filter(r => r.status === 'posted').length}
                                </p>
                                <p className="text-xs text-muted-foreground">Sent Replies</p>
                            </div>
                            <div className="p-4 bg-secondary/50 rounded-lg text-center">
                                <p className="text-2xl font-bold">{account.searchConfig.keywords.length}</p>
                                <p className="text-xs text-muted-foreground">Keywords</p>
                            </div>
                        </div>
                    </Card>
                </TabsContent>

                {/* Logs Tab */}
                <TabsContent value="logs" className="space-y-4 mt-4">
                    <div className="flex items-center justify-between">
                        <div className="flex items-center gap-3">
                            <label className="flex items-center gap-2 cursor-pointer">
                                <input
                                    type="checkbox"
                                    checked={autoRefresh}
                                    onChange={(e) => setAutoRefresh(e.target.checked)}
                                    className="rounded bg-secondary border-border"
                                />
                                <span className="text-sm text-muted-foreground">Auto-refresh</span>
                                {autoRefresh && (
                                    <span className="flex h-2 w-2">
                                        <span className="animate-ping absolute inline-flex h-2 w-2 rounded-full bg-green-400 opacity-75"></span>
                                        <span className="relative inline-flex rounded-full h-2 w-2 bg-green-500"></span>
                                    </span>
                                )}
                            </label>
                        </div>
                        <Button variant="ghost" size="sm" onClick={handleClearLogs}>
                            <Trash2 size={14} />
                            Clear
                        </Button>
                    </div>

                    <Card>
                        {logs.length === 0 ? (
                            <p className="text-muted-foreground text-center py-8">
                                No activity logs yet. Activities will appear here as they happen.
                            </p>
                        ) : (
                            <div className="space-y-2 max-h-[500px] overflow-y-auto">
                                {logs.map((log) => (
                                    <LogItem key={log.id} log={log} />
                                ))}
                            </div>
                        )}
                    </Card>
                </TabsContent>

                {/* Replies Tab */}
                <TabsContent value="replies" className="space-y-4 mt-4">
                    {/* Pending Replies */}
                    <Card title={`Pending Approvals (${currentPending.length})`}>
                        {currentPending.length === 0 ? (
                            <p className="text-muted-foreground text-center py-8">
                                No pending replies. Replies awaiting approval will appear here.
                            </p>
                        ) : (
                            <div className="space-y-4">
                                {currentPending.map((item) => (
                                    <ApprovalCard
                                        key={item.reply.id}
                                        item={item}
                                        onApprove={() => handleApprove(item.reply.id)}
                                        onReject={() => handleReject(item.reply.id)}
                                    />
                                ))}
                            </div>
                        )}
                    </Card>

                    {/* Reply History */}
                    <Card title={`Reply History (${replyHistory.length})`}>
                        {replyHistory.length === 0 ? (
                            <p className="text-muted-foreground text-center py-8">
                                No reply history yet. Sent and processed replies will appear here.
                            </p>
                        ) : (
                            <div className="space-y-3 max-h-[400px] overflow-y-auto">
                                {replyHistory.map((reply) => (
                                    <ReplyHistoryCard key={reply.id} reply={reply} />
                                ))}
                            </div>
                        )}
                    </Card>
                </TabsContent>
            </Tabs>

            {/* Account Editor Modal */}
            {editingAccount && (
                <AccountEditor
                    account={editingAccount}
                    isCreating={false}
                    onSave={handleSaveAccount}
                    onClose={() => setEditingAccount(null)}
                    showToast={showToast}
                />
            )}
        </div>
    );
}

// Log Item Component
function LogItem({ log }: { log: ActivityLog }) {
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
            config: 'bg-muted text-muted-foreground',
        };
        return colors[type] || 'bg-muted text-muted-foreground';
    };

    return (
        <div className={`p-3 bg-secondary/50 rounded-lg border-l-4 ${getLevelBg(log.level)}`}>
            <div className="flex items-start gap-3">
                <div className="mt-0.5">{getLevelIcon(log.level)}</div>
                <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-1">
                        <span className={`px-2 py-0.5 text-xs rounded ${getTypeBadge(log.type)}`}>
                            {log.type}
                        </span>
                        <span className="text-xs text-muted-foreground ml-auto">
                            {new Date(log.timestamp).toLocaleString()}
                        </span>
                    </div>
                    <p className="text-sm">{log.message}</p>
                    {log.details && (
                        <p className="text-xs text-muted-foreground mt-1">{log.details}</p>
                    )}
                </div>
            </div>
        </div>
    );
}

// Approval Card Component
function ApprovalCard({ item, onApprove, onReject }: {
    item: ApprovalQueueItem;
    onApprove: () => void;
    onReject: () => void;
}) {
    return (
        <div className="p-4 bg-secondary/50 rounded-lg">
            {/* Original Tweet */}
            <div className="mb-4">
                <p className="text-xs text-muted-foreground mb-1">Original Tweet</p>
                <div className="p-3 bg-background rounded-lg border border-border">
                    <div className="flex items-center gap-2 mb-1">
                        <span className="font-semibold text-sm">
                            {item.originalTweet.authorName || item.originalTweet.authorUsername}
                        </span>
                        <span className="text-muted-foreground text-sm">@{item.originalTweet.authorUsername}</span>
                    </div>
                    <p>{item.originalTweet.text}</p>
                </div>
            </div>

            {/* Generated Reply */}
            <div className="mb-4">
                <p className="text-xs text-muted-foreground mb-1">Generated Reply</p>
                <div className="p-3 bg-blue-900/20 border border-blue-800/50 rounded-lg">
                    <p>{item.reply.text}</p>
                </div>
            </div>

            {/* Actions */}
            <div className="flex items-center justify-between">
                <p className="text-xs text-muted-foreground">
                    Queued {new Date(item.queuedAt).toLocaleString()}
                </p>
                <div className="flex items-center gap-2">
                    <Button variant="ghost" size="sm" onClick={onReject}>
                        <X size={14} />
                        Reject
                    </Button>
                    <Button size="sm" onClick={onApprove}>
                        <Check size={14} />
                        Approve
                    </Button>
                </div>
            </div>
        </div>
    );
}

// Reply History Card Component
function ReplyHistoryCard({ reply }: { reply: Reply }) {
    const getStatusIcon = (status: string) => {
        switch (status) {
            case 'posted':
                return <CheckCircle size={16} className="text-green-400" />;
            case 'rejected':
                return <XCircle size={16} className="text-red-400" />;
            case 'failed':
                return <AlertCircle size={16} className="text-red-400" />;
            case 'pending':
            case 'approved':
                return <Clock size={16} className="text-yellow-400" />;
            default:
                return <Clock size={16} className="text-muted-foreground" />;
        }
    };

    const getStatusBadge = (status: string) => {
        switch (status) {
            case 'posted':
                return 'bg-green-600/20 text-green-400';
            case 'rejected':
                return 'bg-red-600/20 text-red-400';
            case 'failed':
                return 'bg-red-600/20 text-red-400';
            case 'pending':
                return 'bg-yellow-600/20 text-yellow-400';
            case 'approved':
                return 'bg-blue-600/20 text-blue-400';
            default:
                return 'bg-muted text-muted-foreground';
        }
    };

    return (
        <div className="p-3 bg-secondary/50 rounded-lg">
            <div className="flex items-start gap-3">
                <div className="mt-1">{getStatusIcon(reply.status)}</div>
                <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-2">
                        <span className={`px-2 py-0.5 text-xs rounded ${getStatusBadge(reply.status)}`}>
                            {reply.status}
                        </span>
                        <span className="text-xs text-muted-foreground">
                            Tweet ID: {reply.tweetId.slice(0, 10)}...
                        </span>
                        <span className="text-xs text-muted-foreground ml-auto">
                            {reply.postedAt
                                ? new Date(reply.postedAt).toLocaleString()
                                : new Date(reply.generatedAt).toLocaleString()}
                        </span>
                    </div>
                    <p className="text-sm">{reply.text}</p>
                    {reply.errorMessage && (
                        <p className="text-red-400 text-xs mt-1">{reply.errorMessage}</p>
                    )}
                    {reply.postedReplyId && (
                        <a
                            href={`https://x.com/i/web/status/${reply.postedReplyId}`}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-primary text-xs mt-1 hover:underline"
                        >
                            View on X
                        </a>
                    )}
                </div>
            </div>
        </div>
    );
}
