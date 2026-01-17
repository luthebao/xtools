import { create } from 'zustand';
import { PolymarketEvent, PolymarketEventFilter, PolymarketWatcherStatus } from '../types';

interface PolymarketState {
    events: PolymarketEvent[];
    status: PolymarketWatcherStatus;
    filter: PolymarketEventFilter;
    isLoading: boolean;
    error: string | null;

    setEvents: (events: PolymarketEvent[]) => void;
    addEvent: (event: PolymarketEvent) => void;
    setStatus: (status: PolymarketWatcherStatus) => void;
    setFilter: (filter: Partial<PolymarketEventFilter>) => void;
    resetFilter: () => void;
    setIsLoading: (loading: boolean) => void;
    setError: (error: string | null) => void;
    clearEvents: () => void;
}

const defaultFilter: PolymarketEventFilter = {
    eventTypes: [],
    marketName: '',
    minPrice: 0,
    maxPrice: 0,
    side: '',
    minSize: 100, // Default $100 minimum notional value
    limit: 100,
    offset: 0,
};

export const usePolymarketStore = create<PolymarketState>((set) => ({
    events: [],
    status: {
        isRunning: false,
        eventsReceived: 0,
    },
    filter: defaultFilter,
    isLoading: false,
    error: null,

    setEvents: (events) => set({
        events: [...events].sort((a, b) =>
            new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime()
        ),
    }),

    addEvent: (event) => set((state) => ({
        events: [event, ...state.events]
            .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
            .slice(0, 500), // Keep last 500 events in memory
    })),

    setStatus: (status) => set({ status }),

    setFilter: (filter) => set((state) => ({
        filter: { ...state.filter, ...filter },
    })),

    resetFilter: () => set({ filter: defaultFilter }),

    setIsLoading: (isLoading) => set({ isLoading }),

    setError: (error) => set({ error }),

    clearEvents: () => set({ events: [] }),
}));
