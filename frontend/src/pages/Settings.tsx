import { useState, useEffect } from 'react';
import { FolderOpen, RefreshCw, FileText, Key, Loader2 } from 'lucide-react';
import Card from '../components/common/Card';
import Button from '../components/common/Button';
import { useUIStore } from '../store/uiStore';
import { useAccountStore } from '../store/accountStore';
import { GetAccounts, ExtractCookies, SaveBrowserAuth } from '../../wailsjs/go/main/App';

export default function Settings() {
  const { showToast } = useUIStore();
  const { accounts, setAccounts } = useAccountStore();
  const [selectedAccountId, setSelectedAccountId] = useState('');
  const [isExtracting, setIsExtracting] = useState(false);

  useEffect(() => {
    loadAccounts();
  }, []);

  const loadAccounts = async () => {
    try {
      const accs = await GetAccounts();
      setAccounts(accs || []);
      if (accs?.length > 0 && !selectedAccountId) {
        setSelectedAccountId(accs[0].id);
      }
    } catch (err) {
      showToast('Failed to load accounts', 'error');
    }
  };

  const handleExtractCookies = async () => {
    if (!selectedAccountId) {
      showToast('Select an account first', 'warning');
      return;
    }

    setIsExtracting(true);
    showToast('Browser opening... Please log in to Twitter', 'info');

    try {
      const auth = await ExtractCookies();
      if (auth) {
        await SaveBrowserAuth(selectedAccountId, auth);
        showToast('Cookies extracted and saved! Account switched to browser mode.', 'success');
        loadAccounts();
      }
    } catch (err: any) {
      showToast(err?.message || 'Failed to extract cookies', 'error');
    } finally {
      setIsExtracting(false);
    }
  };

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold">Settings</h1>

      <Card title="Browser Authentication Setup">
        <div className="space-y-4">
          <p className="text-gray-400">
            Twitter's free API tier doesn't support search. Use browser authentication instead by extracting cookies from your logged-in browser session.
          </p>

          <div className="p-4 bg-yellow-900/20 border border-yellow-700/50 rounded-lg">
            <p className="text-yellow-400 text-sm">
              This will open a browser window. Log in to Twitter/X with your account, and cookies will be automatically extracted after successful login.
            </p>
          </div>

          <div className="flex items-center gap-4">
            <select
              value={selectedAccountId}
              onChange={(e) => setSelectedAccountId(e.target.value)}
              className="px-3 py-2 bg-gray-700 border border-gray-600 rounded-lg focus:ring-2 focus:ring-blue-500"
            >
              <option value="">Select Account</option>
              {accounts.map((acc) => (
                <option key={acc.id} value={acc.id}>
                  @{acc.username} ({acc.authType})
                </option>
              ))}
            </select>

            <Button
              onClick={handleExtractCookies}
              disabled={!selectedAccountId || isExtracting}
            >
              {isExtracting ? (
                <>
                  <Loader2 size={16} className="animate-spin" />
                  Waiting for login...
                </>
              ) : (
                <>
                  <Key size={16} />
                  Extract Cookies from Browser
                </>
              )}
            </Button>
          </div>

          {selectedAccountId && accounts.find(a => a.id === selectedAccountId)?.authType === 'browser' && (
            <p className="text-green-400 text-sm">
              This account is already using browser authentication.
            </p>
          )}
        </div>
      </Card>

      <Card title="Configuration Files">
        <div className="space-y-4">
          <p className="text-gray-400">
            Account configurations are stored as YAML files in the <code className="bg-gray-700 px-1 rounded">data/accounts/</code> directory.
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
          <p className="text-gray-400">
            Found tweets are automatically saved to Excel files in the <code className="bg-gray-700 px-1 rounded">data/exports/</code> directory.
            Each account has its own export file.
          </p>

          <Button variant="secondary">
            <FileText size={16} />
            Open Exports Folder
          </Button>
        </div>
      </Card>

      <Card title="About">
        <div className="space-y-2 text-gray-400">
          <p><strong className="text-gray-200">XTools</strong> - Twitter Automation Tool</p>
          <p>Version 1.0.0</p>
          <p className="text-sm">
            Built with Wails, Go, React, and TailwindCSS.
          </p>
        </div>
      </Card>
    </div>
  );
}
