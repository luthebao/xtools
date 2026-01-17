import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Plus, Trash2, Edit2, ChevronRight, MessageSquare, ScrollText } from 'lucide-react';
import Card from '../components/common/Card';
import Button from '../components/common/Button';
import ConfirmModal from '../components/common/ConfirmModal';
import AccountEditor from '../components/AccountEditor';
import { Badge } from '../components/ui/badge';
import { useAccountStore } from '../store/accountStore';
import { useUIStore } from '../store/uiStore';
import { AccountConfig, Reply } from '../types';
import {
    GetAccounts,
    CreateAccount,
    DeleteAccount,
    UpdateAccount,
    GetWorkerStatus,
    GetActivityLogs,
    GetPendingReplies,
    GetReplyHistory,
} from '../../wailsjs/go/main/App';

const createDefaultAccount = (): AccountConfig => ({
    id: '',
    username: '',
    enabled: false,
    authType: 'browser',
    debugMode: false,
    llmConfig: {
        baseUrl: 'https://api.openai.com/v1',
        apiKey: '',
        model: 'gpt-4',
        temperature: 0.7,
        maxTokens: 500,
        persona: 'You are a helpful assistant that replies to tweets in a professional and engaging manner.',
    },
    searchConfig: {
        keywords: [],
        excludeKeywords: [],
        blocklist: [],
        englishOnly: true,
        minFaves: 2,
        minReplies: 12,
        minRetweets: 10,
        maxAgeMins: 60,
        intervalSecs: 300,
    },
    replyConfig: {
        approvalMode: 'queue',
        replyMethod: 'api',
        maxReplyLength: 280,
        tone: 'professional',
        includeHashtags: false,
    },
    rateLimits: {
        searchesPerHour: 10,
        repliesPerHour: 5,
        repliesPerDay: 50,
        minDelayBetween: 60,
    },
});

interface AccountStats {
    logsCount: number;
    pendingReplies: number;
    sentReplies: number;
}

export default function Accounts() {
    const navigate = useNavigate();
    const { accounts, workerStatuses, setAccounts, setWorkerStatuses, removeAccount } = useAccountStore();
    const { showToast } = useUIStore();
    const [editingAccount, setEditingAccount] = useState<AccountConfig | null>(null);
    const [isCreating, setIsCreating] = useState(false);
    const [deleteAccountId, setDeleteAccountId] = useState<string | null>(null);
    const [accountStats, setAccountStats] = useState<Record<string, AccountStats>>({});

    useEffect(() => {
        loadAccounts();
    }, []);

    const loadAccounts = async () => {
        try {
            const [accs, statuses] = await Promise.all([
                GetAccounts(),
                GetWorkerStatus(),
            ]);
            setAccounts(accs || []);
            setWorkerStatuses(statuses || {});

            // Load stats for each account
            if (accs?.length) {
                const statsPromises = accs.map(async (acc: AccountConfig) => {
                    try {
                        const [logs, pending, history] = await Promise.all([
                            GetActivityLogs(acc.id, 100),
                            GetPendingReplies(acc.id),
                            GetReplyHistory(acc.id, 100),
                        ]);
                        return {
                            id: acc.id,
                            stats: {
                                logsCount: (logs || []).length,
                                pendingReplies: (pending || []).length,
                                sentReplies: (history || []).filter((r: Reply) => r.status === 'posted').length,
                            },
                        };
                    } catch {
                        return { id: acc.id, stats: { logsCount: 0, pendingReplies: 0, sentReplies: 0 } };
                    }
                });

                const statsResults = await Promise.all(statsPromises);
                const statsMap: Record<string, AccountStats> = {};
                statsResults.forEach(({ id, stats }) => {
                    statsMap[id] = stats;
                });
                setAccountStats(statsMap);
            }
        } catch (err) {
            showToast('Failed to load accounts', 'error');
        }
    };

    const handleDelete = (id: string, e: React.MouseEvent) => {
        e.stopPropagation();
        setDeleteAccountId(id);
    };

    const confirmDelete = async () => {
        if (!deleteAccountId) return;
        try {
            await DeleteAccount(deleteAccountId);
            removeAccount(deleteAccountId);
            showToast('Account deleted', 'success');
        } catch (err: any) {
            const errorMsg = typeof err === 'string' ? err : (err?.message || 'Failed to delete account');
            console.error('DeleteAccount error:', err);
            showToast(errorMsg, 'error');
        } finally {
            setDeleteAccountId(null);
        }
    };

    const handleSaveAccount = async (updatedAccount: AccountConfig) => {
        try {
            if (isCreating) {
                await CreateAccount(updatedAccount as any);
                showToast('Account created successfully', 'success');
            } else {
                await UpdateAccount(updatedAccount as any);
                showToast('Account updated successfully', 'success');
            }
            setEditingAccount(null);
            setIsCreating(false);
            loadAccounts();
        } catch (err: any) {
            const errorMsg = typeof err === 'string' ? err : (err?.message || 'Failed to save account');
            console.error('SaveAccount error:', err);
            showToast(errorMsg, 'error');
        }
    };

    const handleAddAccount = () => {
        setIsCreating(true);
        setEditingAccount(createDefaultAccount());
    };

    const handleEditClick = (account: AccountConfig, e: React.MouseEvent) => {
        e.stopPropagation();
        setEditingAccount(account);
    };

    const handleAccountClick = (accountId: string) => {
        navigate(`/accounts/${accountId}`);
    };

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <h1 className="text-2xl font-bold">Accounts</h1>
                <Button onClick={handleAddAccount}>
                    <Plus size={16} />
                    Add Account
                </Button>
            </div>

            {accounts.length === 0 ? (
                <Card>
                    <div className="text-center py-12">
                        <p className="text-muted-foreground mb-4">No accounts configured yet.</p>
                        <Button onClick={handleAddAccount}>
                            <Plus size={16} />
                            Add Your First Account
                        </Button>
                    </div>
                </Card>
            ) : (
                <div className="grid gap-4">
                    {accounts.map((account) => {
                        const stats = accountStats[account.id] || { logsCount: 0, pendingReplies: 0, sentReplies: 0 };
                        return (
                            <div
                                key={account.id}
                                className="cursor-pointer"
                                onClick={() => handleAccountClick(account.id)}
                            >
                            <Card className="hover:border-primary/50 transition-colors">
                                <div className="flex items-start justify-between">
                                    <div className="flex-1">
                                        <div className="flex items-center gap-3 mb-3">
                                            <h3 className="text-lg font-semibold">@{account.username}</h3>
                                            <Badge variant={workerStatuses[account.id] ? 'default' : 'secondary'}>
                                                {workerStatuses[account.id] ? 'Running' : 'Stopped'}
                                            </Badge>
                                            <Badge variant="outline">
                                                {account.authType.toUpperCase()}
                                            </Badge>
                                            {account.debugMode && (
                                                <Badge variant="outline" className="border-yellow-500/50 text-yellow-500">
                                                    DEBUG
                                                </Badge>
                                            )}
                                        </div>

                                        {/* Stats Row */}
                                        <div className="flex items-center gap-4 mb-3">
                                            <div className="flex items-center gap-1.5 text-sm">
                                                <ScrollText size={14} className="text-muted-foreground" />
                                                <span className="text-muted-foreground">Logs:</span>
                                                <span className="font-medium">{stats.logsCount}</span>
                                            </div>
                                            <div className="flex items-center gap-1.5 text-sm">
                                                <MessageSquare size={14} className="text-yellow-500" />
                                                <span className="text-muted-foreground">Pending:</span>
                                                <span className={`font-medium ${stats.pendingReplies > 0 ? 'text-yellow-500' : ''}`}>
                                                    {stats.pendingReplies}
                                                </span>
                                            </div>
                                            <div className="flex items-center gap-1.5 text-sm">
                                                <MessageSquare size={14} className="text-green-500" />
                                                <span className="text-muted-foreground">Sent:</span>
                                                <span className="font-medium text-green-500">{stats.sentReplies}</span>
                                            </div>
                                        </div>

                                        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
                                            <div>
                                                <p className="text-muted-foreground">Keywords</p>
                                                <p className="font-medium">{account.searchConfig.keywords.length}</p>
                                            </div>
                                            <div>
                                                <p className="text-muted-foreground">Approval Mode</p>
                                                <p className="font-medium capitalize">{account.replyConfig.approvalMode}</p>
                                            </div>
                                            <div>
                                                <p className="text-muted-foreground">Search Interval</p>
                                                <p className="font-medium">{account.searchConfig.intervalSecs}s</p>
                                            </div>
                                            <div>
                                                <p className="text-muted-foreground">LLM Model</p>
                                                <p className="font-medium">{account.llmConfig.model}</p>
                                            </div>
                                        </div>

                                        <div className="mt-3">
                                            <p className="text-muted-foreground text-sm">Keywords:</p>
                                            <div className="flex flex-wrap gap-1 mt-1">
                                                {account.searchConfig.keywords.slice(0, 5).map((kw, i) => (
                                                    <Badge key={i} variant="secondary" className="text-xs">
                                                        {kw}
                                                    </Badge>
                                                ))}
                                                {account.searchConfig.keywords.length > 5 && (
                                                    <Badge variant="secondary" className="text-xs">
                                                        +{account.searchConfig.keywords.length - 5} more
                                                    </Badge>
                                                )}
                                            </div>
                                        </div>
                                    </div>

                                    <div className="flex items-center gap-2 ml-4">
                                        <Button
                                            variant="ghost"
                                            size="sm"
                                            onClick={(e) => handleEditClick(account, e)}
                                            title="Edit"
                                        >
                                            <Edit2 size={16} />
                                        </Button>
                                        <Button
                                            variant="ghost"
                                            size="sm"
                                            onClick={(e) => handleDelete(account.id, e)}
                                            title="Delete"
                                            className="text-destructive hover:text-destructive hover:bg-destructive/10"
                                        >
                                            <Trash2 size={16} />
                                        </Button>
                                        <ChevronRight size={20} className="text-muted-foreground" />
                                    </div>
                                </div>
                            </Card>
                            </div>
                        );
                    })}
                </div>
            )}

            {/* Account Editor Modal */}
            {editingAccount && (
                <AccountEditor
                    account={editingAccount}
                    isCreating={isCreating}
                    onSave={handleSaveAccount}
                    onClose={() => {
                        setEditingAccount(null);
                        setIsCreating(false);
                    }}
                    showToast={showToast}
                />
            )}

            {/* Delete Confirmation Modal */}
            <ConfirmModal
                isOpen={deleteAccountId !== null}
                title="Delete Account"
                message="Are you sure you want to delete this account? This action cannot be undone."
                confirmText="Delete"
                cancelText="Cancel"
                variant="danger"
                onConfirm={confirmDelete}
                onCancel={() => setDeleteAccountId(null)}
            />
        </div>
    );
}
