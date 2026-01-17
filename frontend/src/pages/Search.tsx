import { useEffect, useState } from 'react';
import { Search as SearchIcon, RefreshCw, Heart, Repeat2, MessageCircle, Eye } from 'lucide-react';
import Card from '../components/common/Card';
import Button from '../components/common/Button';
import { useAccountStore } from '../store/accountStore';
import { useSearchStore } from '../store/searchStore';
import { useUIStore } from '../store/uiStore';
import { Tweet } from '../types';
import { GetAccounts, SearchTweets, ManualSearch } from '../../wailsjs/go/main/App';

export default function Search() {
    const { accounts, activeAccountId, setAccounts, setActiveAccount } = useAccountStore();
    const { tweets, isSearching, searchQuery, setTweets, setSearching, setSearchQuery } = useSearchStore();
    const { showToast } = useUIStore();
    const [customQuery, setCustomQuery] = useState('');

    useEffect(() => {
        loadAccounts();
    }, []);

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

    const handleSearch = async () => {
        if (!activeAccountId) {
            showToast('Select an account first', 'warning');
            return;
        }

        setSearching(true);
        try {
            const results = await SearchTweets(activeAccountId);
            setTweets(activeAccountId, results || []);
            showToast(`Found ${results?.length || 0} tweets`, 'success');
        } catch (err: any) {
            showToast(err?.message || 'Search failed', 'error');
        } finally {
            setSearching(false);
        }
    };

    const handleManualSearch = async () => {
        if (!activeAccountId || !customQuery.trim()) {
            showToast('Enter a search query', 'warning');
            return;
        }

        setSearching(true);
        try {
            const results = await ManualSearch(activeAccountId, customQuery, 50);
            setTweets(activeAccountId, results || []);
            showToast(`Found ${results?.length || 0} tweets`, 'success');
        } catch (err: any) {
            showToast(err?.message || 'Search failed', 'error');
        } finally {
            setSearching(false);
        }
    };

    const currentTweets = activeAccountId ? tweets[activeAccountId] || [] : [];

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <h1 className="text-2xl font-bold">Search Tweets</h1>
            </div>

            {/* Account Selector */}
            <Card>
                <div className="flex items-center gap-4 flex-wrap">
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

                    <Button onClick={handleSearch} loading={isSearching} disabled={!activeAccountId}>
                        <SearchIcon size={16} />
                        Search Keywords
                    </Button>

                    <div className="flex-1 flex items-center gap-2">
                        <input
                            type="text"
                            value={customQuery}
                            onChange={(e) => setCustomQuery(e.target.value)}
                            placeholder="Custom search query..."
                            className="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded-lg focus:ring-2 focus:ring-blue-500"
                            onKeyDown={(e) => e.key === 'Enter' && handleManualSearch()}
                        />
                        <Button
                            onClick={handleManualSearch}
                            loading={isSearching}
                            disabled={!activeAccountId || !customQuery.trim()}
                            variant="secondary"
                        >
                            Search
                        </Button>
                    </div>
                </div>
            </Card>

            {/* Results */}
            <Card title={`Results (${currentTweets.length})`}>
                {currentTweets.length === 0 ? (
                    <p className="text-gray-400 text-center py-8">
                        No tweets found. Run a search to find tweets.
                    </p>
                ) : (
                    <div className="space-y-4">
                        {currentTweets.map((tweet) => (
                            <TweetCard key={tweet.id} tweet={tweet} />
                        ))}
                    </div>
                )}
            </Card>
        </div>
    );
}

function TweetCard({ tweet }: { tweet: Tweet }) {
    return (
        <div className="p-4 bg-gray-700/50 rounded-lg">
            <div className="flex items-start gap-3">
                <div className="flex-1">
                    <div className="flex items-center gap-2 mb-1">
                        <span className="font-semibold">{tweet.authorName || tweet.authorUsername}</span>
                        <span className="text-gray-400">@{tweet.authorUsername}</span>
                    </div>
                    <p className="text-gray-100 mb-3">{tweet.text}</p>

                    <div className="flex items-center gap-4 text-sm text-gray-400">
                        <span className="flex items-center gap-1">
                            <Heart size={14} />
                            {formatNumber(tweet.likeCount)}
                        </span>
                        <span className="flex items-center gap-1">
                            <Repeat2 size={14} />
                            {formatNumber(tweet.retweetCount)}
                        </span>
                        <span className="flex items-center gap-1">
                            <MessageCircle size={14} />
                            {formatNumber(tweet.replyCount)}
                        </span>
                        <span className="flex items-center gap-1">
                            <Eye size={14} />
                            {formatNumber(tweet.viewCount)}
                        </span>
                    </div>

                    {tweet.matchedKeywords?.length > 0 && (
                        <div className="flex items-center gap-1 mt-2">
                            {tweet.matchedKeywords.map((kw, i) => (
                                <span key={i} className="px-2 py-0.5 text-xs bg-blue-600/20 text-blue-400 rounded">
                                    {kw}
                                </span>
                            ))}
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}

function formatNumber(num: number): string {
    if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M';
    if (num >= 1000) return (num / 1000).toFixed(1) + 'K';
    return num.toString();
}
