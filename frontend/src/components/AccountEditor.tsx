import { useState, useEffect } from 'react';
import { Save, Plus, Trash2, Key, Loader2, ChevronLeft, ChevronRight } from 'lucide-react';
import { AccountConfig } from '../types';
import { ExtractCookies } from '../../wailsjs/go/main/App';
import { Button } from './ui/button';
import {
    Dialog,
    DialogContent,
    DialogHeader,
    DialogTitle,
    DialogFooter,
} from './ui/dialog';
import {
    Input,
    Label,
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
    Checkbox,
    Switch,
    Textarea,
    Alert,
    StepIndicator,
    Step,
} from './ui';

interface AccountEditorProps {
    account: AccountConfig | null;
    isCreating?: boolean;
    onSave: (account: AccountConfig) => void;
    onClose: () => void;
    showToast?: (message: string, type: 'success' | 'error' | 'info' | 'warning') => void;
}

const STEPS: Step[] = [
    { number: 1, title: 'Basic Info' },
    { number: 2, title: 'Authentication' },
    { number: 3, title: 'Search' },
    { number: 4, title: 'LLM' },
    { number: 5, title: 'Reply & Limits' },
];

interface ValidationWarning {
    message: string;
    type: 'warning' | 'error';
}

export default function AccountEditor({ account, isCreating, onSave, onClose, showToast }: AccountEditorProps) {
    const [formData, setFormData] = useState<Partial<AccountConfig>>({});
    const [isExtracting, setIsExtracting] = useState(false);
    const [currentStep, setCurrentStep] = useState(1);
    const [completedSteps, setCompletedSteps] = useState<Set<number>>(new Set());

    useEffect(() => {
        if (account) {
            setFormData(account);
            if (!isCreating) {
                setCompletedSteps(new Set([1, 2, 3, 4, 5]));
            }
        }
    }, [account, isCreating]);

    if (!account) return null;

    const handleChange = (path: string, value: any) => {
        setFormData((prev) => {
            const newData = JSON.parse(JSON.stringify(prev));
            const parts = path.split('.');
            let current: any = newData;

            for (let i = 0; i < parts.length - 1; i++) {
                if (!current[parts[i]]) {
                    current[parts[i]] = {};
                }
                current = current[parts[i]];
            }
            current[parts[parts.length - 1]] = value;
            return newData;
        });
    };

    const handleSubmit = () => {
        onSave(formData as AccountConfig);
    };

    const addArrayItem = (path: string) => {
        const currentArray = path.split('.').reduce((obj: any, key) => obj?.[key], formData) || [];
        handleChange(path, [...currentArray, '']);
    };

    const removeArrayItem = (path: string, index: number) => {
        const currentArray = path.split('.').reduce((obj: any, key) => obj?.[key], formData) || [];
        handleChange(path, currentArray.filter((_: any, i: number) => i !== index));
    };

    const updateArrayItem = (path: string, index: number, value: string) => {
        const currentArray = path.split('.').reduce((obj: any, key) => obj?.[key], formData) || [];
        const newArray = [...currentArray];
        newArray[index] = value;
        handleChange(path, newArray);
    };

    const handleExtractCookies = async () => {
        setIsExtracting(true);
        showToast?.('Browser opening... Please log in to Twitter', 'info');

        try {
            const auth = await ExtractCookies();
            if (auth) {
                handleChange('browserAuth', auth);
                showToast?.('Cookies extracted successfully!', 'success');
            }
        } catch (err: any) {
            const errorMsg = typeof err === 'string' ? err : (err?.message || 'Failed to extract cookies');
            showToast?.(errorMsg, 'error');
        } finally {
            setIsExtracting(false);
        }
    };

    const getStepValidation = (step: number): ValidationWarning[] => {
        const warnings: ValidationWarning[] = [];

        switch (step) {
            case 1:
                if (!formData.username?.trim()) {
                    warnings.push({ message: 'Username is required', type: 'error' });
                }
                break;
            case 2:
                const searchMethod = formData.authType || 'browser';
                const replyMethod = formData.replyConfig?.replyMethod || 'api';
                const needsBrowserCookies = searchMethod === 'browser' || replyMethod === 'browser';
                const needsBearerToken = searchMethod === 'api';
                const needsOAuthCreds = replyMethod === 'api';

                if (needsBrowserCookies && !formData.browserAuth?.cookies?.length) {
                    warnings.push({ message: 'Browser cookies not configured', type: 'warning' });
                }
                if (needsBearerToken && !formData.apiCredentials?.bearerToken) {
                    warnings.push({ message: 'Bearer token required for API search', type: 'warning' });
                }
                if (needsOAuthCreds) {
                    const creds = formData.apiCredentials;
                    if (!creds?.apiKey || !creds?.apiSecret || !creds?.accessToken || !creds?.accessSecret) {
                        warnings.push({ message: 'OAuth credentials incomplete for API replies', type: 'warning' });
                    }
                }
                break;
            case 3:
                if (!formData.searchConfig?.keywords?.length) {
                    warnings.push({ message: 'No keywords configured - no tweets will be found', type: 'warning' });
                }
                break;
            case 4:
                if (formData.replyConfig?.approvalMode === 'auto') {
                    if (!formData.llmConfig?.apiKey) {
                        warnings.push({ message: 'LLM API key required for auto-reply mode', type: 'warning' });
                    }
                    if (!formData.llmConfig?.model) {
                        warnings.push({ message: 'LLM model required for auto-reply mode', type: 'warning' });
                    }
                }
                break;
        }

        return warnings;
    };

    const canProceed = (step: number): boolean => {
        const warnings = getStepValidation(step);
        return !warnings.some(w => w.type === 'error');
    };

    const handleNext = () => {
        if (!canProceed(currentStep)) {
            showToast?.('Please fix errors before proceeding', 'error');
            return;
        }
        setCompletedSteps(prev => new Set([...prev, currentStep]));
        setCurrentStep(prev => Math.min(prev + 1, 5));
    };

    const handleBack = () => {
        setCurrentStep(prev => Math.max(prev - 1, 1));
    };

    const handleStepClick = (step: number) => {
        setCurrentStep(step);
    };

    const currentWarnings = getStepValidation(currentStep);

    // Reusable array field component
    const ArrayField = ({ label, path, placeholder }: { label: string; path: string; placeholder: string }) => {
        const items = path.split('.').reduce((obj: any, key) => obj?.[key], formData) || [];

        return (
            <div className="space-y-2">
                <div className="flex items-center justify-between">
                    <Label>{label}</Label>
                    <button
                        type="button"
                        onClick={() => addArrayItem(path)}
                        className="flex items-center gap-1 text-xs text-primary hover:text-primary/80 transition-colors"
                    >
                        <Plus size={14} /> Add
                    </button>
                </div>
                <div className="space-y-2">
                    {items.map((item: string, index: number) => (
                        <div key={index} className="flex items-center gap-2">
                            <Input
                                value={item}
                                onChange={(e) => updateArrayItem(path, index, e.target.value)}
                                placeholder={placeholder}
                            />
                            <Button
                                type="button"
                                variant="ghost"
                                size="icon"
                                onClick={() => removeArrayItem(path, index)}
                                className="h-9 w-9 text-destructive hover:text-destructive hover:bg-destructive/10"
                            >
                                <Trash2 size={16} />
                            </Button>
                        </div>
                    ))}
                    {items.length === 0 && (
                        <p className="text-sm text-muted-foreground italic py-2">None added</p>
                    )}
                </div>
            </div>
        );
    };

    // Step 1: Basic Info
    const renderStep1 = () => (
        <div className="space-y-6">
            <div className="space-y-2">
                <Label required>Username</Label>
                <Input
                    value={formData.username || ''}
                    onChange={(e) => handleChange('username', e.target.value)}
                    placeholder="Twitter username (without @)"
                />
            </div>

            <div className="space-y-2">
                <Label>Account Status</Label>
                <div className="flex items-center justify-between p-4 bg-secondary/50 rounded-lg border border-border">
                    <div>
                        <p className="text-sm font-medium">Enable Account</p>
                        <p className="text-xs text-muted-foreground">Account will be active for searching and replying</p>
                    </div>
                    <Switch
                        checked={formData.enabled || false}
                        onCheckedChange={(checked) => handleChange('enabled', checked)}
                    />
                </div>
            </div>

            <div className="flex items-center justify-between p-4 bg-yellow-500/5 rounded-lg border border-yellow-500/20">
                <div className="flex items-center gap-3">
                    <Checkbox
                        checked={formData.debugMode || false}
                        onCheckedChange={(checked) => handleChange('debugMode', checked)}
                    />
                    <div>
                        <p className="text-sm font-medium">Debug Mode</p>
                        <p className="text-xs text-yellow-500">10s interval, all replies need approval</p>
                    </div>
                </div>
            </div>
        </div>
    );

    // Step 2: Authentication
    const renderStep2 = () => {
        const searchMethod = formData.authType || 'browser';
        const replyMethod = formData.replyConfig?.replyMethod || 'api';
        const needsBrowser = searchMethod === 'browser' || replyMethod === 'browser';
        const needsApi = searchMethod === 'api' || replyMethod === 'api';

        return (
            <div className="space-y-6">
                {/* Method Selection */}
                <div className="p-4 bg-primary/5 rounded-lg border border-primary/20 space-y-4">
                    <h4 className="text-sm font-medium text-primary">Choose Your Methods</h4>
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                        <div className="space-y-2">
                            <Label>Search Tweets Using</Label>
                            <Select
                                value={searchMethod}
                                onValueChange={(value) => handleChange('authType', value)}
                            >
                                <SelectTrigger>
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="browser">Browser (Cookies)</SelectItem>
                                    <SelectItem value="api">API (Bearer Token)</SelectItem>
                                </SelectContent>
                            </Select>
                            <p className="text-xs text-muted-foreground">
                                {searchMethod === 'browser' ? 'Uses logged-in browser session' : 'Uses API bearer token'}
                            </p>
                        </div>
                        <div className="space-y-2">
                            <Label>Post Replies Using</Label>
                            <Select
                                value={replyMethod}
                                onValueChange={(value) => handleChange('replyConfig.replyMethod', value)}
                            >
                                <SelectTrigger>
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="api">API (OAuth 1.0a)</SelectItem>
                                    <SelectItem value="browser">Browser (Cookies)</SelectItem>
                                </SelectContent>
                            </Select>
                            <p className="text-xs text-muted-foreground">
                                {replyMethod === 'api' ? 'Uses OAuth API credentials' : 'Uses logged-in browser session'}
                            </p>
                        </div>
                    </div>
                </div>

                {/* Browser Authentication - always visible */}
                <div className={`p-4 rounded-lg border space-y-3 ${needsBrowser ? 'bg-secondary/50 border-border' : 'bg-secondary/20 border-border/50'}`}>
                    <div className="flex items-center justify-between">
                        <h4 className="text-sm font-medium">Browser Cookies</h4>
                        <div className="flex gap-1">
                            {searchMethod === 'browser' && (
                                <span className="text-xs bg-blue-500/20 text-blue-400 px-2 py-0.5 rounded">Search</span>
                            )}
                            {replyMethod === 'browser' && (
                                <span className="text-xs bg-green-500/20 text-green-400 px-2 py-0.5 rounded">Reply</span>
                            )}
                            {!needsBrowser && (
                                <span className="text-xs bg-muted text-muted-foreground px-2 py-0.5 rounded">Not in use</span>
                            )}
                        </div>
                    </div>
                    <Alert variant="info" icon={false}>
                        Opens a browser window. Log in to Twitter/X and cookies will be extracted automatically.
                    </Alert>
                    <div className="flex flex-wrap items-center gap-3">
                        <Button
                            type="button"
                            onClick={handleExtractCookies}
                            disabled={isExtracting}
                        >
                            {isExtracting ? (
                                <>
                                    <Loader2 size={16} className="animate-spin" />
                                    Waiting for login...
                                </>
                            ) : (
                                <>
                                    <Key size={16} />
                                    Extract Cookies
                                </>
                            )}
                        </Button>
                        {formData.browserAuth?.cookies?.length ? (
                            <span className="text-green-500 text-sm font-medium">
                                ✓ {formData.browserAuth.cookies.length} cookies configured
                            </span>
                        ) : (
                            <span className="text-yellow-500 text-sm">No cookies configured</span>
                        )}
                    </div>
                </div>

                {/* API Credentials - always visible */}
                <div className={`p-4 rounded-lg border space-y-4 ${needsApi ? 'bg-secondary/50 border-border' : 'bg-secondary/20 border-border/50'}`}>
                    <div className="flex items-center justify-between">
                        <h4 className="text-sm font-medium">API Credentials</h4>
                        <div className="flex gap-1">
                            {searchMethod === 'api' && (
                                <span className="text-xs bg-blue-500/20 text-blue-400 px-2 py-0.5 rounded">Search</span>
                            )}
                            {replyMethod === 'api' && (
                                <span className="text-xs bg-green-500/20 text-green-400 px-2 py-0.5 rounded">Reply</span>
                            )}
                            {!needsApi && (
                                <span className="text-xs bg-muted text-muted-foreground px-2 py-0.5 rounded">Not in use</span>
                            )}
                        </div>
                    </div>

                    {/* Bearer Token for API Search */}
                    <div className="space-y-2">
                        <Label>Bearer Token {searchMethod === 'api' && <span className="text-blue-400 text-xs">(used for search)</span>}</Label>
                        <Input
                            type="password"
                            value={formData.apiCredentials?.bearerToken || ''}
                            onChange={(e) => handleChange('apiCredentials.bearerToken', e.target.value)}
                            placeholder="Bearer Token"
                        />
                    </div>

                    {/* OAuth Credentials for API Reply */}
                    <div className="space-y-4">
                        <Label>OAuth 1.0a Credentials {replyMethod === 'api' && <span className="text-green-400 text-xs">(used for replies)</span>}</Label>
                        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                            <div className="space-y-2">
                                <Label>API Key</Label>
                                <Input
                                    type="password"
                                    value={formData.apiCredentials?.apiKey || ''}
                                    onChange={(e) => handleChange('apiCredentials.apiKey', e.target.value)}
                                    placeholder="Consumer Key"
                                />
                            </div>
                            <div className="space-y-2">
                                <Label>API Secret</Label>
                                <Input
                                    type="password"
                                    value={formData.apiCredentials?.apiSecret || ''}
                                    onChange={(e) => handleChange('apiCredentials.apiSecret', e.target.value)}
                                    placeholder="Consumer Secret"
                                />
                            </div>
                            <div className="space-y-2">
                                <Label>Access Token</Label>
                                <Input
                                    type="password"
                                    value={formData.apiCredentials?.accessToken || ''}
                                    onChange={(e) => handleChange('apiCredentials.accessToken', e.target.value)}
                                    placeholder="Access Token"
                                />
                            </div>
                            <div className="space-y-2">
                                <Label>Access Token Secret</Label>
                                <Input
                                    type="password"
                                    value={formData.apiCredentials?.accessSecret || ''}
                                    onChange={(e) => handleChange('apiCredentials.accessSecret', e.target.value)}
                                    placeholder="Access Token Secret"
                                />
                            </div>
                        </div>
                    </div>

                    <p className="text-xs text-muted-foreground">
                        Get these from the <a href="https://developer.twitter.com" target="_blank" rel="noopener noreferrer" className="text-primary hover:underline">Twitter Developer Portal</a>
                    </p>
                </div>

                {/* Info about the combination */}
                <Alert variant="info">
                    <strong>Your setup:</strong> {searchMethod === 'browser' ? 'Browser cookies' : 'Bearer token'} for searching, {replyMethod === 'api' ? 'OAuth API' : 'Browser cookies'} for replies.
                </Alert>
            </div>
        );
    };

    // Step 3: Search Settings
    const renderStep3 = () => (
        <div className="space-y-6">
            <ArrayField label="Keywords" path="searchConfig.keywords" placeholder="Enter keyword" />
            <ArrayField label="Exclude Keywords" path="searchConfig.excludeKeywords" placeholder="Keyword to exclude" />
            <ArrayField label="Blocklist (usernames)" path="searchConfig.blocklist" placeholder="Username to block" />

            <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
                <div className="space-y-2">
                    <Label>Interval (secs)</Label>
                    <Input
                        type="number"
                        value={formData.searchConfig?.intervalSecs || 300}
                        onChange={(e) => handleChange('searchConfig.intervalSecs', parseInt(e.target.value))}
                    />
                </div>
                <div className="space-y-2">
                    <Label>Max Age (mins)</Label>
                    <Input
                        type="number"
                        value={formData.searchConfig?.maxAgeMins || 60}
                        onChange={(e) => handleChange('searchConfig.maxAgeMins', parseInt(e.target.value))}
                    />
                </div>
                <div className="space-y-2">
                    <Label>Min Faves</Label>
                    <Input
                        type="number"
                        value={formData.searchConfig?.minFaves || 2}
                        onChange={(e) => handleChange('searchConfig.minFaves', parseInt(e.target.value))}
                    />
                </div>
                <div className="space-y-2">
                    <Label>Min Replies</Label>
                    <Input
                        type="number"
                        value={formData.searchConfig?.minReplies || 12}
                        onChange={(e) => handleChange('searchConfig.minReplies', parseInt(e.target.value))}
                    />
                </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                    <Label>Min Retweets</Label>
                    <Input
                        type="number"
                        value={formData.searchConfig?.minRetweets || 10}
                        onChange={(e) => handleChange('searchConfig.minRetweets', parseInt(e.target.value))}
                    />
                </div>
                <div className="flex items-center gap-3 p-4 bg-secondary/50 rounded-lg border border-border">
                    <Checkbox
                        checked={formData.searchConfig?.englishOnly || false}
                        onCheckedChange={(checked) => handleChange('searchConfig.englishOnly', checked)}
                    />
                    <Label className="cursor-pointer">English Only</Label>
                </div>
            </div>
        </div>
    );

    // Step 4: LLM Config
    const renderStep4 = () => (
        <div className="space-y-6">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div className="space-y-2">
                    <Label>Base URL</Label>
                    <Input
                        value={formData.llmConfig?.baseUrl || ''}
                        onChange={(e) => handleChange('llmConfig.baseUrl', e.target.value)}
                        placeholder="https://api.openai.com/v1"
                    />
                </div>
                <div className="space-y-2">
                    <Label>Model</Label>
                    <Input
                        value={formData.llmConfig?.model || ''}
                        onChange={(e) => handleChange('llmConfig.model', e.target.value)}
                        placeholder="gpt-4"
                    />
                </div>
            </div>

            <div className="space-y-2">
                <Label>API Key</Label>
                <Input
                    type="password"
                    value={formData.llmConfig?.apiKey || ''}
                    onChange={(e) => handleChange('llmConfig.apiKey', e.target.value)}
                    placeholder="Your LLM API key"
                />
            </div>

            <div className="space-y-2">
                <Label>Persona / System Prompt</Label>
                <Textarea
                    value={formData.llmConfig?.persona || ''}
                    onChange={(e) => handleChange('llmConfig.persona', e.target.value)}
                    placeholder="You are a helpful assistant..."
                    className="min-h-[120px]"
                />
            </div>

            <div className="grid grid-cols-2 gap-4">
                <div className="space-y-2">
                    <Label>Temperature</Label>
                    <Input
                        type="number"
                        step="0.1"
                        min="0"
                        max="2"
                        value={formData.llmConfig?.temperature || 0.7}
                        onChange={(e) => handleChange('llmConfig.temperature', parseFloat(e.target.value))}
                    />
                </div>
                <div className="space-y-2">
                    <Label>Max Tokens</Label>
                    <Input
                        type="number"
                        value={formData.llmConfig?.maxTokens || 500}
                        onChange={(e) => handleChange('llmConfig.maxTokens', parseInt(e.target.value))}
                    />
                </div>
            </div>
        </div>
    );

    // Step 5: Reply & Rate Limits
    const renderStep5 = () => (
        <div className="space-y-6">
            <div>
                <h4 className="text-sm font-medium mb-4">Reply Settings</h4>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                    <div className="space-y-2">
                        <Label>Approval Mode</Label>
                        <Select
                            value={formData.replyConfig?.approvalMode || 'queue'}
                            onValueChange={(value) => handleChange('replyConfig.approvalMode', value)}
                        >
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="auto">Auto (immediate)</SelectItem>
                                <SelectItem value="queue">Queue (manual approval)</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>
                    <div className="space-y-2">
                        <Label>Tone</Label>
                        <Input
                            value={formData.replyConfig?.tone || ''}
                            onChange={(e) => handleChange('replyConfig.tone', e.target.value)}
                            placeholder="professional, friendly"
                        />
                    </div>
                    <div className="space-y-2">
                        <Label>Max Reply Length</Label>
                        <Input
                            type="number"
                            value={formData.replyConfig?.maxReplyLength || 280}
                            onChange={(e) => handleChange('replyConfig.maxReplyLength', parseInt(e.target.value))}
                        />
                    </div>
                    <div className="flex items-center gap-3 p-4 bg-secondary/50 rounded-lg border border-border">
                        <Checkbox
                            checked={formData.replyConfig?.includeHashtags || false}
                            onCheckedChange={(checked) => handleChange('replyConfig.includeHashtags', checked)}
                        />
                        <Label className="cursor-pointer">Include Hashtags</Label>
                    </div>
                </div>
            </div>

            <div>
                <h4 className="text-sm font-medium mb-4">Rate Limits</h4>
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
                    <div className="space-y-2">
                        <Label>Searches/Hour</Label>
                        <Input
                            type="number"
                            value={formData.rateLimits?.searchesPerHour || 10}
                            onChange={(e) => handleChange('rateLimits.searchesPerHour', parseInt(e.target.value))}
                        />
                    </div>
                    <div className="space-y-2">
                        <Label>Replies/Hour</Label>
                        <Input
                            type="number"
                            value={formData.rateLimits?.repliesPerHour || 5}
                            onChange={(e) => handleChange('rateLimits.repliesPerHour', parseInt(e.target.value))}
                        />
                    </div>
                    <div className="space-y-2">
                        <Label>Replies/Day</Label>
                        <Input
                            type="number"
                            value={formData.rateLimits?.repliesPerDay || 50}
                            onChange={(e) => handleChange('rateLimits.repliesPerDay', parseInt(e.target.value))}
                        />
                    </div>
                    <div className="space-y-2">
                        <Label>Min Delay (secs)</Label>
                        <Input
                            type="number"
                            value={formData.rateLimits?.minDelayBetween || 60}
                            onChange={(e) => handleChange('rateLimits.minDelayBetween', parseInt(e.target.value))}
                        />
                    </div>
                </div>
            </div>
        </div>
    );

    const renderStepContent = () => {
        switch (currentStep) {
            case 1: return renderStep1();
            case 2: return renderStep2();
            case 3: return renderStep3();
            case 4: return renderStep4();
            case 5: return renderStep5();
            default: return null;
        }
    };

    return (
        <Dialog open={true} onOpenChange={(open) => !open && onClose()}>
            <DialogContent className="sm:max-w-2xl max-h-[90vh] flex flex-col p-0">
                <DialogHeader className="px-6 py-4 border-b border-border">
                    <DialogTitle>
                        {isCreating ? 'Add Account' : `Edit: @${account.username}`}
                    </DialogTitle>
                </DialogHeader>

                {/* Step Indicator */}
                <div className="px-6 py-3 border-b border-border">
                    <StepIndicator
                        steps={STEPS}
                        currentStep={currentStep}
                        completedSteps={completedSteps}
                        onStepClick={handleStepClick}
                    />
                </div>

                {/* Content */}
                <div className="flex-1 overflow-y-auto min-h-0">
                    <div className="px-6 py-4">
                        {currentWarnings.length > 0 && (
                            <div className="mb-4 space-y-2">
                                {currentWarnings.map((warning, idx) => (
                                    <Alert key={idx} variant={warning.type === 'error' ? 'destructive' : 'warning'}>
                                        {warning.message}
                                    </Alert>
                                ))}
                            </div>
                        )}
                        {renderStepContent()}
                    </div>
                </div>

                {/* Navigation */}
                <DialogFooter className="px-6 py-4 border-t border-border bg-secondary/30">
                    <div className="flex items-center justify-between w-full">
                        <Button
                            variant="ghost"
                            onClick={handleBack}
                            disabled={currentStep === 1}
                        >
                            <ChevronLeft size={16} />
                            Back
                        </Button>

                        <span className="text-sm text-muted-foreground hidden sm:block">
                            Step {currentStep} of {STEPS.length}
                        </span>

                        {currentStep < 5 ? (
                            <Button onClick={handleNext}>
                                Next
                                <ChevronRight size={16} />
                            </Button>
                        ) : (
                            <Button onClick={handleSubmit}>
                                <Save size={16} />
                                {isCreating ? 'Create' : 'Save'}
                            </Button>
                        )}
                    </div>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
