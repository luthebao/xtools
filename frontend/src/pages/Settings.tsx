import { useEffect, useState } from 'react';
import { FolderOpen, RefreshCw, FileText, Download, ExternalLink, Check } from 'lucide-react';
import Card from '../components/common/Card';
import Button from '../components/common/Button';
import { Badge } from '../components/ui/badge';
import { useUIStore } from '../store/uiStore';
import { UpdateInfo } from '../types';
import { CheckForUpdates, GetAppVersion } from '../../wailsjs/go/main/App';

export default function Settings() {
    const { showToast } = useUIStore();
    const [version, setVersion] = useState<string>('');
    const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null);
    const [isChecking, setIsChecking] = useState(false);

    useEffect(() => {
        loadVersion();
    }, []);

    const loadVersion = async () => {
        try {
            const v = await GetAppVersion();
            setVersion(v);
        } catch (err) {
            console.error('Failed to get version:', err);
        }
    };

    const handleCheckForUpdates = async () => {
        setIsChecking(true);
        try {
            const info = await CheckForUpdates();
            setUpdateInfo(info);
            if (info.isUpdateAvailable) {
                showToast(`New version ${info.latestVersion} available!`, 'info');
            } else {
                showToast('You are using the latest version', 'success');
            }
        } catch (err: any) {
            const errorMsg = typeof err === 'string' ? err : (err?.message || 'Failed to check for updates');
            showToast(errorMsg, 'error');
        } finally {
            setIsChecking(false);
        }
    };

    const openReleasePage = () => {
        if (updateInfo?.releaseUrl) {
            window.open(updateInfo.releaseUrl, '_blank');
        }
    };

    return (
        <div className="space-y-6">
            <h1 className="text-2xl font-bold">Settings</h1>

            <Card title="Configuration Files">
                <div className="space-y-4">
                    <p className="text-muted-foreground">
                        Account configurations are stored as YAML files in the <code className="bg-secondary px-1.5 py-0.5 rounded text-sm">data/accounts/</code> directory.
                        You can edit these files manually and reload them.
                    </p>

                    <div className="flex items-center gap-2">
                        <Button variant="secondary">
                            <FolderOpen size={16} />
                            Open Config Folder
                        </Button>
                        <Button variant="secondary">
                            <RefreshCw size={16} />
                            Reload All Configs
                        </Button>
                    </div>
                </div>
            </Card>

            <Card title="Data Export">
                <div className="space-y-4">
                    <p className="text-muted-foreground">
                        Found tweets are automatically saved to Excel files in the <code className="bg-secondary px-1.5 py-0.5 rounded text-sm">data/exports/</code> directory.
                        Each account has its own export file.
                    </p>

                    <Button variant="secondary">
                        <FileText size={16} />
                        Open Exports Folder
                    </Button>
                </div>
            </Card>

            <Card title="About">
                <div className="space-y-4">
                    <div className="space-y-2">
                        <p><strong className="text-foreground">XTools</strong> - Twitter Automation Tool</p>
                        <div className="flex items-center gap-2">
                            <span className="text-muted-foreground">Version</span>
                            <Badge variant="outline">{version || '...'}</Badge>
                            {updateInfo && !updateInfo.isUpdateAvailable && (
                                <span className="flex items-center gap-1 text-sm text-green-500">
                                    <Check size={14} />
                                    Up to date
                                </span>
                            )}
                        </div>
                        <p className="text-sm text-muted-foreground">
                            Built with Wails, Go, React, and TailwindCSS.
                        </p>
                    </div>

                    {/* Update Checker */}
                    <div className="pt-2 border-t border-border">
                        <div className="flex items-center justify-between">
                            <div>
                                <p className="text-sm font-medium">Check for Updates</p>
                                <p className="text-xs text-muted-foreground">
                                    Check GitHub releases for new versions
                                </p>
                            </div>
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={handleCheckForUpdates}
                                disabled={isChecking}
                            >
                                {isChecking ? (
                                    <>
                                        <RefreshCw size={14} className="animate-spin" />
                                        Checking...
                                    </>
                                ) : (
                                    <>
                                        <Download size={14} />
                                        Check for Updates
                                    </>
                                )}
                            </Button>
                        </div>

                        {/* Update Available Notification */}
                        {updateInfo?.isUpdateAvailable && (
                            <div className="mt-4 p-4 bg-primary/10 border border-primary/20 rounded-lg">
                                <div className="flex items-start justify-between gap-4">
                                    <div className="space-y-1">
                                        <p className="font-medium text-primary">
                                            New Version Available!
                                        </p>
                                        <p className="text-sm text-muted-foreground">
                                            Version <strong className="text-foreground">{updateInfo.latestVersion}</strong> is available.
                                            You are currently using version {updateInfo.currentVersion}.
                                        </p>
                                        {updateInfo.publishedAt && (
                                            <p className="text-xs text-muted-foreground">
                                                Released: {new Date(updateInfo.publishedAt).toLocaleDateString()}
                                            </p>
                                        )}
                                    </div>
                                    <Button
                                        variant="primary"
                                        size="sm"
                                        onClick={openReleasePage}
                                    >
                                        <ExternalLink size={14} />
                                        Download
                                    </Button>
                                </div>

                                {/* Release Notes */}
                                {updateInfo.releaseNotes && (
                                    <div className="mt-3 pt-3 border-t border-primary/20">
                                        <p className="text-xs font-medium text-muted-foreground mb-1">Release Notes:</p>
                                        <div className="text-sm text-muted-foreground max-h-32 overflow-y-auto">
                                            <pre className="whitespace-pre-wrap font-sans text-xs">
                                                {updateInfo.releaseNotes}
                                            </pre>
                                        </div>
                                    </div>
                                )}
                            </div>
                        )}
                    </div>
                </div>
            </Card>
        </div>
    );
}
