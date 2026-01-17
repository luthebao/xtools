import { create } from 'zustand';
import { Tweet } from '../types';

interface SearchState {
    tweets: Record<string, Tweet[]>;
    isSearching: boolean;
    searchQuery: string;

    // Actions
    setTweets: (accountId: string, tweets: Tweet[]) => void;
    addTweets: (accountId: string, tweets: Tweet[]) => void;
    clearTweets: (accountId: string) => void;
    setSearching: (searching: boolean) => void;
    setSearchQuery: (query: string) => void;
}

export const useSearchStore = create<SearchState>((set) => ({
    tweets: {},
    isSearching: false,
    searchQuery: '',

    setTweets: (accountId, tweets) =>
        set((state) => ({
            tweets: { ...state.tweets, [accountId]: tweets },
        })),

    addTweets: (accountId, newTweets) =>
        set((state) => {
            const existing = state.tweets[accountId] || [];
            const existingIds = new Set(existing.map((t) => t.id));
            const uniqueNew = newTweets.filter((t) => !existingIds.has(t.id));
            return {
                tweets: { ...state.tweets, [accountId]: [...existing, ...uniqueNew] },
            };
        }),

    clearTweets: (accountId) =>
        set((state) => ({
            tweets: { ...state.tweets, [accountId]: [] },
        })),

    setSearching: (isSearching) => set({ isSearching }),

    setSearchQuery: (searchQuery) => set({ searchQuery }),
}));
