import { useEffect, useState } from 'react';
import { BarChart3, TrendingUp, MessageSquare, Clock, RefreshCw } from 'lucide-react';
import Card from '../components/common/Card';
import Button from '../components/common/Button';
import { useAccountStore } from '../store/accountStore';
import { useUIStore } from '../store/uiStore';
import { DailyStats, ReplyPerformanceReport } from '../types';
import {
    GetAccounts,
    GetDailyStats,
    GetReplyPerformance,
} from '../../wailsjs/go/main/App';

export default function Metrics() {
    const { accounts, activeAccountId, setAccounts, setActiveAccount } = useAccountStore();
    const { showToast } = useUIStore();
    const [dailyStats, setDailyStats] = useState<DailyStats[]>([]);
    const [replyPerformance, setReplyPerformance] = useState<ReplyPerformanceReport | null>(null);
    const [days, setDays] = useState(7);
    const [autoRefresh, setAutoRefresh] = useState(true);
    const [isRefreshing, setIsRefreshing] = useState(false);

    useEffect(() => {
        loadAccounts();
    }, []);

    useEffect(() => {
        if (activeAccountId) {
            loadMetrics(activeAccountId);
        }
    }, [activeAccountId, days]);

    // Auto-refresh effect
    useEffect(() => {
        if (!autoRefresh || !activeAccountId) return;

        const interval = setInterval(() => {
            loadMetrics(activeAccountId, true);
        }, 5000); // Refresh every 5 seconds

        return () => clearInterval(interval);
    }, [autoRefresh, activeAccountId, days]);

    const loadAccounts = async () => {
        try {
            const accs = await GetAccounts();
            setAccounts(accs || []);
            if (accs?.length > 0 && !activeAccountId) {
                setActiveAccount(accs[0].id);
            }
        } catch (err) {
            showToast('Failed to load accounts', 'error');
        }
    };

    const loadMetrics = async (accountId: string, silent = false) => {
        if (!silent) setIsRefreshing(true);
        try {
            const [stats, performance] = await Promise.all([
                GetDailyStats(accountId, days),
                GetReplyPerformance(accountId, days),
            ]);
            setDailyStats(stats || []);
            setReplyPerformance(performance);
        } catch (err) {
            if (!silent) showToast('Failed to load metrics', 'error');
        } finally {
            if (!silent) setIsRefreshing(false);
        }
    };

    const handleManualRefresh = () => {
        if (activeAccountId) {
            loadMetrics(activeAccountId);
        }
    };

    const totalReplies = dailyStats.reduce((sum, s) => sum + s.repliesSent, 0);
    const totalSearched = dailyStats.reduce((sum, s) => sum + s.tweetsSearched, 0);
    const totalTokens = dailyStats.reduce((sum, s) => sum + s.tokensUsed, 0);

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <h1 className="text-2xl font-bold">Metrics</h1>
            </div>

            {/* Filters */}
            <Card>
                <div className="flex items-center justify-between">
                    <div className="flex items-center gap-4">
                        <select
                            value={activeAccountId || ''}
                            onChange={(e) => setActiveAccount(e.target.value)}
                            className="px-3 py-2 bg-gray-700 border border-gray-600 rounded-lg focus:ring-2 focus:ring-blue-500"
                        >
                            <option value="">Select Account</option>
                            {accounts.map((acc) => (
                                <option key={acc.id} value={acc.id}>
                                    @{acc.username}
                                </option>
                            ))}
                        </select>

                        <select
                            value={days}
                            onChange={(e) => setDays(Number(e.target.value))}
                            className="px-3 py-2 bg-gray-700 border border-gray-600 rounded-lg focus:ring-2 focus:ring-blue-500"
                        >
                            <option value={7}>Last 7 days</option>
                            <option value={14}>Last 14 days</option>
                            <option value={30}>Last 30 days</option>
                        </select>
                    </div>

                    <div className="flex items-center gap-4">
                        <label className="flex items-center gap-2 text-sm text-gray-400">
                            <input
                                type="checkbox"
                                checked={autoRefresh}
                                onChange={(e) => setAutoRefresh(e.target.checked)}
                                className="rounded bg-gray-700 border-gray-600"
                            />
                            Auto-refresh
                        </label>
                        <Button
                            variant="secondary"
                            size="sm"
                            onClick={handleManualRefresh}
                            disabled={isRefreshing || !activeAccountId}
                        >
                            <RefreshCw size={16} className={isRefreshing ? 'animate-spin' : ''} />
                            Refresh
                        </Button>
                    </div>
                </div>
            </Card>

            {/* Summary Cards */}
            <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
                <Card className="flex items-center gap-4 p-4">
                    <div className="p-3 bg-blue-600/20 rounded-lg">
                        <BarChart3 className="text-blue-400" size={24} />
                    </div>
                    <div>
                        <p className="text-sm text-gray-400">Tweets Searched</p>
                        <p className="text-2xl font-bold">{totalSearched}</p>
                    </div>
                </Card>

                <Card className="flex items-center gap-4 p-4">
                    <div className="p-3 bg-green-600/20 rounded-lg">
                        <MessageSquare className="text-green-400" size={24} />
                    </div>
                    <div>
                        <p className="text-sm text-gray-400">Replies Sent</p>
                        <p className="text-2xl font-bold">{totalReplies}</p>
                    </div>
                </Card>

                <Card className="flex items-center gap-4 p-4">
                    <div className="p-3 bg-yellow-600/20 rounded-lg">
                        <TrendingUp className="text-yellow-400" size={24} />
                    </div>
                    <div>
                        <p className="text-sm text-gray-400">Avg Likes/Reply</p>
                        <p className="text-2xl font-bold">
                            {replyPerformance?.avgLikesPerReply.toFixed(1) || '0'}
                        </p>
                    </div>
                </Card>

                <Card className="flex items-center gap-4 p-4">
                    <div className="p-3 bg-purple-600/20 rounded-lg">
                        <Clock className="text-purple-400" size={24} />
                    </div>
                    <div>
                        <p className="text-sm text-gray-400">Tokens Used</p>
                        <p className="text-2xl font-bold">{totalTokens.toLocaleString()}</p>
                    </div>
                </Card>
            </div>

            {/* Daily Stats Table */}
            <Card title="Daily Activity">
                {dailyStats.length === 0 ? (
                    <p className="text-gray-400 text-center py-8">
                        No data available for the selected period.
                    </p>
                ) : (
                    <div className="overflow-x-auto">
                        <table className="w-full">
                            <thead>
                                <tr className="border-b border-gray-700">
                                    <th className="text-left py-2 px-3 text-gray-400">Date</th>
                                    <th className="text-right py-2 px-3 text-gray-400">Searched</th>
                                    <th className="text-right py-2 px-3 text-gray-400">Generated</th>
                                    <th className="text-right py-2 px-3 text-gray-400">Sent</th>
                                    <th className="text-right py-2 px-3 text-gray-400">Failed</th>
                                    <th className="text-right py-2 px-3 text-gray-400">Tokens</th>
                                </tr>
                            </thead>
                            <tbody>
                                {dailyStats.map((stat) => (
                                    <tr key={stat.date} className="border-b border-gray-700/50">
                                        <td className="py-2 px-3">{new Date(stat.date).toLocaleDateString()}</td>
                                        <td className="py-2 px-3 text-right">{stat.tweetsSearched}</td>
                                        <td className="py-2 px-3 text-right">{stat.repliesGenerated}</td>
                                        <td className="py-2 px-3 text-right text-green-400">{stat.repliesSent}</td>
                                        <td className="py-2 px-3 text-right text-red-400">{stat.repliesFailed}</td>
                                        <td className="py-2 px-3 text-right">{stat.tokensUsed.toLocaleString()}</td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                )}
            </Card>
        </div>
    );
}
