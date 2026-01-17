import { useEffect, useState, useCallback } from 'react';
import {
    Play,
    Square,
    RefreshCw,
    Filter,
    X,
    Activity,
    TrendingUp,
    TrendingDown,
    ExternalLink,
    AlertTriangle,
    Wallet,
    Loader2,
    Zap,
    Settings,
    Plus,
    Trash2,
} from 'lucide-react';
import Card from '../components/common/Card';
import Button from '../components/common/Button';
import { Badge } from '../components/ui/badge';
import { Input } from '../components/ui/input';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '../components/ui/select';
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogFooter,
} from '../components/ui/dialog';
import { useUIStore } from '../store/uiStore';
import { usePolymarketStore } from '../store/polymarketStore';
import { PolymarketEvent, PolymarketConfig } from '../types';
import {
    StartPolymarketWatcher,
    StopPolymarketWatcher,
    GetPolymarketWatcherStatus,
    GetPolymarketEvents,
    SetPolymarketSaveFilter,
    GetPolymarketConfig,
    SetPolymarketConfig,
} from '../../wailsjs/go/main/App';
import { EventsOn, EventsOff, BrowserOpenURL } from '../../wailsjs/runtime/runtime';

const EVENT_TYPE_LABELS: Record<string, string> = {
    trade: 'Trade',
    book: 'Order Book',
    price_change: 'Price Change',
    last_trade_price: 'Last Trade',
    tick_size_change: 'Tick Size Change',
};

const EVENT_TYPE_COLORS: Record<string, string> = {
    trade: 'bg-emerald-500/10 text-emerald-500 border-emerald-500/20',
    book: 'bg-blue-500/10 text-blue-500 border-blue-500/20',
    price_change: 'bg-purple-500/10 text-purple-500 border-purple-500/20',
    last_trade_price: 'bg-green-500/10 text-green-500 border-green-500/20',
    tick_size_change: 'bg-orange-500/10 text-orange-500 border-orange-500/20',
};

export default function PolymarketWatcher() {
    const { showToast } = useUIStore();
    const {
        events,
        status,
        filter,
        isLoading,
        setEvents,
        addEvent,
        setStatus,
        setFilter,
        resetFilter,
        setIsLoading,
    } = usePolymarketStore();

    const [showFilters, setShowFilters] = useState(false);
    const [showSettings, setShowSettings] = useState(false);
    const [autoRefresh, setAutoRefresh] = useState(true);
    const [freshWalletsOnly, setFreshWalletsOnly] = useState(false);
    const [config, setConfig] = useState<PolymarketConfig | null>(null);
    const [rpcUrls, setRpcUrls] = useState<string[]>([]);
    const [newRpcUrl, setNewRpcUrl] = useState('');

    const loadConfig = useCallback(async () => {
        try {
            const cfg = await GetPolymarketConfig();
            setConfig(cfg);
            setRpcUrls(cfg.polygonRpcUrls || []);
        } catch (err) {
            console.error('Failed to load config:', err);
        }
    }, []);

    const loadStatus = useCallback(async () => {
        try {
            const s = await GetPolymarketWatcherStatus();
            setStatus(s);
        } catch (err) {
            console.error('Failed to get status:', err);
        }
    }, [setStatus]);

    const loadEvents = useCallback(async () => {
        try {
            setIsLoading(true);
            const currentFilter = { ...filter };
            if (freshWalletsOnly) {
                currentFilter.freshWalletsOnly = true;
            }
            const evts = await GetPolymarketEvents(currentFilter);
            setEvents(evts || []);
        } catch (err: any) {
            const errorMsg = typeof err === 'string' ? err : err?.message || 'Failed to load events';
            showToast(errorMsg, 'error');
        } finally {
            setIsLoading(false);
        }
    }, [filter, freshWalletsOnly, setEvents, setIsLoading, showToast]);

    useEffect(() => {
        loadStatus();
        loadEvents();
        loadConfig();

        const handleEvent = (event: PolymarketEvent) => {
            if (!autoRefresh) return;
            const notional = getNotionalValue(event);
            const minValue = filter.minSize || 100;
            if (notional < minValue) return;
            if (freshWalletsOnly && !event.isFreshWallet) return;
            addEvent(event);
        };

        const handleFreshWallet = (event: PolymarketEvent) => {
            if (event.riskScore && event.riskScore >= 0.7) {
                showToast(
                    `Fresh Wallet Alert: ${shortenAddress(event.walletAddress || '')} made a $${formatNotionalValue(event)} trade`,
                    'warning'
                );
            }
        };

        EventsOn('polymarket:event', handleEvent);
        EventsOn('polymarket:fresh_wallet', handleFreshWallet);
        const statusInterval = setInterval(loadStatus, 3000);

        return () => {
            EventsOff('polymarket:event');
            EventsOff('polymarket:fresh_wallet');
            clearInterval(statusInterval);
        };
    }, [loadStatus, loadEvents, loadConfig, autoRefresh, freshWalletsOnly, addEvent, showToast, filter.minSize]);

    const handleStart = async () => {
        try {
            await StartPolymarketWatcher();
            showToast('Polymarket watcher started', 'success');
            loadStatus();
        } catch (err: any) {
            const errorMsg = typeof err === 'string' ? err : err?.message || 'Failed to start watcher';
            showToast(errorMsg, 'error');
        }
    };

    const handleStop = async () => {
        try {
            await StopPolymarketWatcher();
            showToast('Polymarket watcher stopped', 'info');
            loadStatus();
        } catch (err: any) {
            const errorMsg = typeof err === 'string' ? err : err?.message || 'Failed to stop watcher';
            showToast(errorMsg, 'error');
        }
    };

    const handleApplyFilters = async () => {
        const saveFilter = {
            ...filter,
            freshWalletsOnly: freshWalletsOnly,
        };
        try {
            await SetPolymarketSaveFilter(saveFilter);
            showToast('Filter applied - only matching events will be saved', 'success');
        } catch (err) {
            console.error('Failed to set save filter:', err);
        }
        loadEvents();
        setShowFilters(false);
    };

    const handleResetFilters = async () => {
        resetFilter();
        setFreshWalletsOnly(false);
        try {
            await SetPolymarketSaveFilter({ minSize: 100 });
        } catch (err) {
            console.error('Failed to reset save filter:', err);
        }
        loadEvents();
    };

    const handleAddRpcUrl = () => {
        if (newRpcUrl && !rpcUrls.includes(newRpcUrl)) {
            setRpcUrls([...rpcUrls, newRpcUrl]);
            setNewRpcUrl('');
        }
    };

    const handleRemoveRpcUrl = (url: string) => {
        setRpcUrls(rpcUrls.filter(u => u !== url));
    };

    const handleSaveSettings = async () => {
        if (!config) return;
        try {
            const updatedConfig: PolymarketConfig = {
                ...config,
                polygonRpcUrls: rpcUrls,
            };
            await SetPolymarketConfig(updatedConfig);
            setConfig(updatedConfig);
            showToast('Settings saved. Restart watcher to apply RPC changes.', 'success');
            setShowSettings(false);
        } catch (err) {
            console.error('Failed to save settings:', err);
            showToast('Failed to save settings', 'error');
        }
    };

    const formatTimestamp = (ts: string) => {
        if (!ts) return '-';
        const date = new Date(ts);
        return date.toLocaleString();
    };

    const getStatusBadge = () => {
        if (status.isConnecting) {
            return (
                <Badge variant="secondary" className="flex items-center gap-1">
                    <Loader2 size={12} className="animate-spin" />
                    Connecting...
                </Badge>
            );
        }
        if (status.isRunning) {
            return <Badge variant="default" className="bg-green-500">Connected</Badge>;
        }
        return <Badge variant="secondary">Disconnected</Badge>;
    };

    return (
        <div className="flex flex-col h-full">
            {/* Header */}
            <div className="flex items-center justify-between mb-4">
                <div className="flex items-center gap-3">
                    <h1 className="text-2xl font-bold">Polymarket Watcher</h1>
                    <Badge variant="outline" className="text-xs">
                        Live Data Feed
                    </Badge>
                </div>
                <div className="flex items-center gap-2">
                    <Button variant="ghost" size="sm" onClick={() => setShowFilters(true)}>
                        <Filter size={16} />
                        Filters
                    </Button>
                    <Button variant="ghost" size="sm" onClick={() => setShowSettings(true)}>
                        <Settings size={16} />
                        RPC
                    </Button>
                    {status.isRunning || status.isConnecting ? (
                        <Button variant="danger" onClick={handleStop}>
                            <Square size={16} />
                            Stop
                        </Button>
                    ) : (
                        <Button variant="primary" onClick={handleStart}>
                            <Play size={16} />
                            Start
                        </Button>
                    )}
                    <Button variant="secondary" onClick={loadEvents} loading={isLoading}>
                        <RefreshCw size={16} />
                    </Button>
                </div>
            </div>

            {/* Status Bar */}
            <div className="flex flex-wrap items-center gap-4 p-3 rounded-lg border border-border bg-card/50 mb-4">
                <div className="flex items-center gap-2">
                    <Activity size={18} className={status.isRunning ? 'text-green-500' : 'text-muted-foreground'} />
                    <span className="text-sm font-medium">Status:</span>
                    {getStatusBadge()}
                </div>

                <div className="flex items-center gap-2">
                    <Zap size={16} className="text-yellow-500" />
                    <span className="text-sm text-muted-foreground">Trades:</span>
                    <span className="font-mono font-medium">{(status.tradesReceived || status.eventsReceived).toLocaleString()}</span>
                </div>

                <div className="flex items-center gap-2">
                    <AlertTriangle size={16} className="text-orange-500" />
                    <span className="text-sm text-muted-foreground">Fresh Wallets:</span>
                    <span className="font-mono font-medium text-orange-500">{(status.freshWalletsFound || 0).toLocaleString()}</span>
                </div>

                {status.lastEventAt && (
                    <div className="flex items-center gap-2">
                        <span className="text-sm text-muted-foreground">Last:</span>
                        <span className="text-sm">{formatTimestamp(status.lastEventAt)}</span>
                    </div>
                )}

                {status.errorMessage && (
                    <div className="flex items-center gap-2 text-destructive">
                        <span className="text-sm">Error: {status.errorMessage}</span>
                    </div>
                )}

                <div className="ml-auto flex items-center gap-2">
                    <label className="flex items-center gap-2 text-sm cursor-pointer">
                        <input
                            type="checkbox"
                            checked={autoRefresh}
                            onChange={(e) => setAutoRefresh(e.target.checked)}
                            className="rounded border-border"
                        />
                        Auto-refresh
                    </label>
                </div>
            </div>

            {/* Events List - Flex 1 to fill remaining height */}
            <div className="flex-1 min-h-0 rounded-lg border border-border bg-card/50 overflow-hidden flex flex-col">
                <div className="px-4 py-3 border-b border-border flex items-center justify-between">
                    <span className="font-medium">Events ({events.length})</span>
                </div>
                <div className="flex-1 overflow-y-auto p-2 space-y-2">
                    {events.length === 0 ? (
                        <div className="text-center py-8 text-muted-foreground">
                            {status.isRunning
                                ? 'Waiting for events...'
                                : 'Start the watcher to receive events'}
                        </div>
                    ) : (
                        events.map((event) => (
                            <EventCard key={event.id || `${event.timestamp}-${event.tradeId}`} event={event} />
                        ))
                    )}
                </div>
            </div>

            {/* Filter Modal */}
            <Dialog open={showFilters} onOpenChange={setShowFilters}>
                <DialogContent className="max-w-2xl max-h-[80vh] overflow-y-auto">
                    <DialogHeader>
                        <DialogTitle className="flex items-center gap-2">
                            <Filter size={20} />
                            Event Filters
                        </DialogTitle>
                    </DialogHeader>

                    <div className="space-y-4 py-4">
                        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                            <div className="space-y-2">
                                <label className="text-sm font-medium">Event Type</label>
                                <Select
                                    value={filter.eventTypes?.[0] || 'all'}
                                    onValueChange={(value) =>
                                        setFilter({ eventTypes: value === 'all' ? [] : [value] })
                                    }
                                >
                                    <SelectTrigger>
                                        <SelectValue placeholder="All Types" />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="all">All Types</SelectItem>
                                        <SelectItem value="trade">Trades</SelectItem>
                                        <SelectItem value="book">Order Book</SelectItem>
                                        <SelectItem value="price_change">Price Change</SelectItem>
                                        <SelectItem value="last_trade_price">Last Trade</SelectItem>
                                    </SelectContent>
                                </Select>
                            </div>

                            <div className="space-y-2">
                                <label className="text-sm font-medium">Side</label>
                                <Select
                                    value={filter.side || 'all'}
                                    onValueChange={(value) => setFilter({ side: value === 'all' ? '' : value })}
                                >
                                    <SelectTrigger>
                                        <SelectValue placeholder="All Sides" />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="all">All Sides</SelectItem>
                                        <SelectItem value="BUY">Buy</SelectItem>
                                        <SelectItem value="SELL">Sell</SelectItem>
                                    </SelectContent>
                                </Select>
                            </div>

                            <div className="space-y-2">
                                <label className="text-sm font-medium">Market Name</label>
                                <Input
                                    placeholder="Search by name..."
                                    value={filter.marketName || ''}
                                    onChange={(e) => setFilter({ marketName: e.target.value })}
                                />
                            </div>
                        </div>

                        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
                            <div className="space-y-2">
                                <label className="text-sm font-medium">Min Price</label>
                                <Input
                                    type="number"
                                    step="0.01"
                                    placeholder="0.00"
                                    value={filter.minPrice || ''}
                                    onChange={(e) => setFilter({ minPrice: parseFloat(e.target.value) || 0 })}
                                />
                            </div>

                            <div className="space-y-2">
                                <label className="text-sm font-medium">Max Price</label>
                                <Input
                                    type="number"
                                    step="0.01"
                                    placeholder="1.00"
                                    value={filter.maxPrice || ''}
                                    onChange={(e) => setFilter({ maxPrice: parseFloat(e.target.value) || 0 })}
                                />
                            </div>

                            <div className="space-y-2">
                                <label className="text-sm font-medium">Min Value ($)</label>
                                <Input
                                    type="number"
                                    step="10"
                                    placeholder="100"
                                    value={filter.minSize || 100}
                                    onChange={(e) => setFilter({ minSize: parseFloat(e.target.value) || 100 })}
                                />
                            </div>

                            <div className="space-y-2">
                                <label className="text-sm font-medium">Limit</label>
                                <Input
                                    type="number"
                                    step="10"
                                    placeholder="100"
                                    value={filter.limit || 100}
                                    onChange={(e) => setFilter({ limit: parseInt(e.target.value) || 100 })}
                                />
                            </div>
                        </div>

                        {/* Fresh Wallet Filters */}
                        <div className="pt-4 border-t border-border">
                            <h4 className="text-sm font-medium mb-3 flex items-center gap-2">
                                <Wallet size={16} className="text-orange-500" />
                                Fresh Wallet Filters
                            </h4>
                            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                                <div className="space-y-2">
                                    <label className="flex items-center gap-2 text-sm cursor-pointer">
                                        <input
                                            type="checkbox"
                                            checked={freshWalletsOnly}
                                            onChange={(e) => setFreshWalletsOnly(e.target.checked)}
                                            className="rounded border-border"
                                        />
                                        Fresh Wallets Only
                                    </label>
                                </div>

                                <div className="space-y-2">
                                    <label className="text-sm font-medium">Min Risk Score</label>
                                    <Select
                                        value={String(filter.minRiskScore || 0)}
                                        onValueChange={(value) => setFilter({ minRiskScore: parseFloat(value) })}
                                    >
                                        <SelectTrigger>
                                            <SelectValue placeholder="Any" />
                                        </SelectTrigger>
                                        <SelectContent>
                                            <SelectItem value="0">Any</SelectItem>
                                            <SelectItem value="0.5">50%+ (Medium)</SelectItem>
                                            <SelectItem value="0.7">70%+ (High)</SelectItem>
                                            <SelectItem value="0.85">85%+ (Very High)</SelectItem>
                                        </SelectContent>
                                    </Select>
                                </div>

                                <div className="space-y-2">
                                    <label className="text-sm font-medium">Max Wallet Nonce</label>
                                    <Select
                                        value={String(filter.maxWalletNonce || 0)}
                                        onValueChange={(value) => setFilter({ maxWalletNonce: parseInt(value) })}
                                    >
                                        <SelectTrigger>
                                            <SelectValue placeholder="Any" />
                                        </SelectTrigger>
                                        <SelectContent>
                                            <SelectItem value="0">Any</SelectItem>
                                            <SelectItem value="1">0-1 (Brand New)</SelectItem>
                                            <SelectItem value="3">0-3 (Very Fresh)</SelectItem>
                                            <SelectItem value="5">0-5 (Fresh)</SelectItem>
                                            <SelectItem value="10">0-10</SelectItem>
                                        </SelectContent>
                                    </Select>
                                </div>
                            </div>
                        </div>
                    </div>

                    <DialogFooter>
                        <Button variant="ghost" onClick={handleResetFilters}>
                            <X size={14} />
                            Reset
                        </Button>
                        <Button variant="primary" onClick={handleApplyFilters}>
                            Apply Filters
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* RPC Settings Modal */}
            <Dialog open={showSettings} onOpenChange={setShowSettings}>
                <DialogContent className="max-w-xl">
                    <DialogHeader>
                        <DialogTitle className="flex items-center gap-2">
                            <Settings size={20} />
                            RPC Settings
                        </DialogTitle>
                    </DialogHeader>

                    <div className="space-y-4 py-4">
                        <div className="space-y-2">
                            <label className="text-sm font-medium">Polygon RPC URLs (with fallback)</label>
                            <p className="text-xs text-muted-foreground">
                                Add multiple RPC URLs for wallet analysis. The system will automatically fall back to the next URL if one fails.
                            </p>

                            {/* Current RPC URLs */}
                            <div className="space-y-2 max-h-48 overflow-y-auto">
                                {rpcUrls.map((url, idx) => (
                                    <div key={idx} className="flex items-center gap-2">
                                        <Input
                                            value={url}
                                            readOnly
                                            className="flex-1 font-mono text-xs"
                                        />
                                        <Button
                                            variant="ghost"
                                            size="sm"
                                            onClick={() => handleRemoveRpcUrl(url)}
                                        >
                                            <Trash2 size={14} className="text-destructive" />
                                        </Button>
                                    </div>
                                ))}
                            </div>

                            {/* Add new URL */}
                            <div className="flex items-center gap-2">
                                <Input
                                    placeholder="https://polygon-rpc.com"
                                    value={newRpcUrl}
                                    onChange={(e) => setNewRpcUrl(e.target.value)}
                                    onKeyDown={(e) => e.key === 'Enter' && handleAddRpcUrl()}
                                    className="flex-1 font-mono text-xs"
                                />
                                <Button
                                    variant="secondary"
                                    size="sm"
                                    onClick={handleAddRpcUrl}
                                    disabled={!newRpcUrl}
                                >
                                    <Plus size={14} />
                                    Add
                                </Button>
                            </div>
                        </div>
                    </div>

                    <DialogFooter>
                        <Button
                            variant="ghost"
                            onClick={() => {
                                setRpcUrls(config?.polygonRpcUrls || []);
                                setShowSettings(false);
                            }}
                        >
                            Cancel
                        </Button>
                        <Button variant="primary" onClick={handleSaveSettings}>
                            Save Settings
                        </Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>
        </div>
    );
}

interface EventCardProps {
    event: PolymarketEvent;
}

function shortenAddress(addr: string): string {
    if (!addr || addr.length <= 10) return addr || '';
    return addr.slice(0, 6) + '...' + addr.slice(-4);
}

function getNotionalValue(event: PolymarketEvent): number {
    if (!event.price || !event.size) return 0;
    const price = parseFloat(event.price);
    const size = parseFloat(event.size);
    if (isNaN(price) || isNaN(size)) return 0;
    return price * size;
}

function formatNotionalValue(event: PolymarketEvent): string {
    return getNotionalValue(event).toFixed(2);
}

function EventCard({ event }: EventCardProps) {
    const getSideIcon = (side?: string) => {
        if (side === 'BUY') return <TrendingUp size={14} className="text-green-500" />;
        if (side === 'SELL') return <TrendingDown size={14} className="text-red-500" />;
        return null;
    };

    const formatPrice = (price?: string) => {
        if (!price) return '-';
        const num = parseFloat(price);
        if (isNaN(num)) return price;
        return `$${num.toFixed(4)}`;
    };

    const formatSize = (size?: string) => {
        if (!size) return '-';
        const num = parseFloat(size);
        if (isNaN(num)) return size;
        return num.toLocaleString(undefined, { maximumFractionDigits: 2 });
    };

    const getRiskBadge = () => {
        if (!event.isFreshWallet) return null;

        const score = event.riskScore || 0;
        let variant = 'outline';
        let label = 'Low Risk';

        if (score >= 0.85) {
            variant = 'destructive';
            label = 'Very High Risk';
        } else if (score >= 0.7) {
            variant = 'default';
            label = 'High Risk';
        } else if (score >= 0.5) {
            variant = 'secondary';
            label = 'Medium Risk';
        }

        return (
            <Badge variant={variant as any} className="flex items-center gap-1">
                <AlertTriangle size={12} />
                {label} ({(score * 100).toFixed(0)}%)
            </Badge>
        );
    };

    const getTraderProfileUrl = () => {
        if (event.walletAddress) {
            return `https://polymarket.com/profile/${event.walletAddress}`;
        }
        return null;
    };

    return (
        <div
            className={`p-3 rounded-lg border ${
                event.isFreshWallet
                    ? 'border-orange-500/50 bg-orange-500/5'
                    : 'border-border bg-card/50'
            }`}
        >
            <div className="flex items-center justify-between gap-3">
                <div className="flex items-center gap-3 flex-1 min-w-0">
                    <Badge
                        variant="outline"
                        className={EVENT_TYPE_COLORS[event.eventType] || 'bg-secondary'}
                    >
                        {EVENT_TYPE_LABELS[event.eventType] || event.eventType}
                    </Badge>

                    {event.isFreshWallet && (
                        <Badge variant="outline" className="bg-orange-500/10 text-orange-500 border-orange-500/30 flex items-center gap-1">
                            <Wallet size={12} />
                            Fresh Wallet
                        </Badge>
                    )}

                    {event.side && (
                        <div className="flex items-center gap-1">
                            {getSideIcon(event.side)}
                            <span className={event.side === 'BUY' ? 'text-green-500' : 'text-red-500'}>
                                {event.side}
                            </span>
                        </div>
                    )}

                    {event.outcome && (
                        <span className="text-sm font-medium">{event.outcome}</span>
                    )}

                    {event.price && (
                        <span className="font-mono text-sm">{formatPrice(event.price)}</span>
                    )}

                    {event.size && (
                        <span className="text-sm text-muted-foreground">
                            Size: {formatSize(event.size)}
                        </span>
                    )}

                    {(event.marketName || event.eventTitle || event.marketSlug) && (
                        <span className="text-sm text-muted-foreground truncate max-w-[200px]">
                            {event.marketName || event.eventTitle || event.marketSlug}
                        </span>
                    )}
                </div>

                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    {getRiskBadge()}
                    <span>{new Date(event.timestamp).toLocaleTimeString()}</span>
                </div>
            </div>

            {/* Always show details */}
            <div className="mt-3 pt-3 border-t border-border space-y-2 text-sm">
                <div className="grid grid-cols-2 gap-2">
                    {event.walletAddress && (
                        <div>
                            <span className="text-muted-foreground">Wallet: </span>
                            <a
                                href={getTraderProfileUrl() || '#'}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="font-mono text-primary hover:underline"
                            >
                                {shortenAddress(event.walletAddress)}
                            </a>
                        </div>
                    )}
                    {event.traderName && (
                        <div>
                            <span className="text-muted-foreground">Trader: </span>
                            {event.walletAddress ? (
                                <a
                                    href={getTraderProfileUrl() || '#'}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    className="text-primary hover:underline"
                                >
                                    {event.traderName}
                                </a>
                            ) : (
                                <span>{event.traderName}</span>
                            )}
                        </div>
                    )}
                    <div>
                        <span className="text-muted-foreground">Asset ID: </span>
                        <span className="font-mono">{event.assetId ? shortenAddress(event.assetId) : '-'}</span>
                    </div>
                    <div>
                        <span className="text-muted-foreground">Market: </span>
                        <span>{event.marketSlug || event.eventSlug || '-'}</span>
                    </div>
                    {event.bestBid && (
                        <div>
                            <span className="text-muted-foreground">Best Bid: </span>
                            <span className="font-mono text-green-500">{formatPrice(event.bestBid)}</span>
                        </div>
                    )}
                    {event.bestAsk && (
                        <div>
                            <span className="text-muted-foreground">Best Ask: </span>
                            <span className="font-mono text-red-500">{formatPrice(event.bestAsk)}</span>
                        </div>
                    )}
                    {event.price && event.size && (
                        <div>
                            <span className="text-muted-foreground">Notional Value: </span>
                            <span className="font-mono font-medium">${formatNotionalValue(event)}</span>
                        </div>
                    )}
                    {event.feeRateBps !== undefined && event.feeRateBps > 0 && (
                        <div>
                            <span className="text-muted-foreground">Fee Rate: </span>
                            <span>{event.feeRateBps} bps</span>
                        </div>
                    )}
                </div>

                {/* Fresh Wallet Info */}
                {event.isFreshWallet && event.walletProfile && (
                    <div className="p-2 rounded bg-orange-500/10 border border-orange-500/30">
                        <div className="text-orange-500 font-medium mb-1 flex items-center gap-2">
                            <AlertTriangle size={14} />
                            Fresh Wallet Detection
                        </div>
                        <div className="grid grid-cols-2 gap-2 text-xs">
                            <div>
                                <span className="text-muted-foreground">Wallet Nonce: </span>
                                <span className="font-mono">{event.walletProfile.nonce}</span>
                            </div>
                            <div>
                                <span className="text-muted-foreground">Risk Score: </span>
                                <span className="font-mono">{((event.riskScore || 0) * 100).toFixed(0)}%</span>
                            </div>
                            {event.riskSignals && event.riskSignals.length > 0 && (
                                <div className="col-span-2">
                                    <span className="text-muted-foreground">Signals: </span>
                                    <span>{event.riskSignals.join(', ')}</span>
                                </div>
                            )}
                            {event.freshWalletSignal && (
                                <div className="col-span-2">
                                    <span className="text-muted-foreground">Factors: </span>
                                    <span>
                                        {Object.entries(event.freshWalletSignal.factors || {})
                                            .map(([k, v]) => `${k}: ${(v * 100).toFixed(0)}%`)
                                            .join(', ')}
                                    </span>
                                </div>
                            )}
                        </div>
                    </div>
                )}

                {/* Links */}
                <div className="flex items-center gap-4">
                    {event.walletAddress && (
                        <a
                            href={getTraderProfileUrl() || '#'}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="inline-flex items-center gap-1 text-primary hover:underline"
                        >
                            <Wallet size={14} />
                            Trader Profile
                        </a>
                    )}
                    {event.marketLink && (
                        <a
                            href={event.marketLink}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="inline-flex items-center gap-1 text-primary hover:underline"
                        >
                            <ExternalLink size={14} />
                            View Market
                        </a>
                    )}
                </div>

                {event.rawData && (
                    <details className="pt-2">
                        <summary className="cursor-pointer text-muted-foreground hover:text-foreground">
                            Raw Data
                        </summary>
                        <pre className="mt-2 p-2 bg-secondary rounded text-xs overflow-x-auto">
                            {JSON.stringify(JSON.parse(event.rawData), null, 2)}
                        </pre>
                    </details>
                )}
            </div>
        </div>
    );
}
