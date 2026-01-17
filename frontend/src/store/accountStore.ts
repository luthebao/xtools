import { create } from 'zustand';
import { AccountConfig, AccountStatus } from '../types';

interface AccountState {
    accounts: AccountConfig[];
    activeAccountId: string | null;
    accountStatuses: Record<string, AccountStatus>;
    workerStatuses: Record<string, boolean>;
    isLoading: boolean;
    error: string | null;

    // Actions
    setAccounts: (accounts: AccountConfig[]) => void;
    setActiveAccount: (id: string | null) => void;
    updateAccountStatus: (id: string, status: AccountStatus) => void;
    setWorkerStatuses: (statuses: Record<string, boolean>) => void;
    addAccount: (account: AccountConfig) => void;
    updateAccount: (account: AccountConfig) => void;
    removeAccount: (id: string) => void;
    setLoading: (loading: boolean) => void;
    setError: (error: string | null) => void;
}

export const useAccountStore = create<AccountState>((set) => ({
    accounts: [],
    activeAccountId: null,
    accountStatuses: {},
    workerStatuses: {},
    isLoading: false,
    error: null,

    setAccounts: (accounts) => set({ accounts }),

    setActiveAccount: (id) => set({ activeAccountId: id }),

    updateAccountStatus: (id, status) =>
        set((state) => ({
            accountStatuses: { ...state.accountStatuses, [id]: status },
        })),

    setWorkerStatuses: (statuses) => set({ workerStatuses: statuses }),

    addAccount: (account) =>
        set((state) => ({ accounts: [...state.accounts, account] })),

    updateAccount: (account) =>
        set((state) => ({
            accounts: state.accounts.map((a) =>
                a.id === account.id ? account : a
            ),
        })),

    removeAccount: (id) =>
        set((state) => ({
            accounts: state.accounts.filter((a) => a.id !== id),
            accountStatuses: Object.fromEntries(
                Object.entries(state.accountStatuses).filter(([key]) => key !== id)
            ),
        })),

    setLoading: (isLoading) => set({ isLoading }),

    setError: (error) => set({ error }),
}));
