import { create } from 'zustand';
import { ApprovalQueueItem, Reply, Tweet } from '../types';

interface ReplyState {
  pendingReplies: Record<string, ApprovalQueueItem[]>;
  replyHistory: Record<string, Reply[]>;
  selectedTweet: Tweet | null;
  isGenerating: boolean;

  // Actions
  setPendingReplies: (accountId: string, items: ApprovalQueueItem[]) => void;
  addPendingReply: (item: ApprovalQueueItem) => void;
  removePendingReply: (accountId: string, replyId: string) => void;
  setReplyHistory: (accountId: string, replies: Reply[]) => void;
  setSelectedTweet: (tweet: Tweet | null) => void;
  setGenerating: (generating: boolean) => void;
}

export const useReplyStore = create<ReplyState>((set) => ({
  pendingReplies: {},
  replyHistory: {},
  selectedTweet: null,
  isGenerating: false,

  setPendingReplies: (accountId, items) =>
    set((state) => ({
      pendingReplies: { ...state.pendingReplies, [accountId]: items },
    })),

  addPendingReply: (item) =>
    set((state) => {
      const accountId = item.reply.accountId;
      const existing = state.pendingReplies[accountId] || [];
      return {
        pendingReplies: {
          ...state.pendingReplies,
          [accountId]: [...existing, item],
        },
      };
    }),

  removePendingReply: (accountId, replyId) =>
    set((state) => ({
      pendingReplies: {
        ...state.pendingReplies,
        [accountId]: (state.pendingReplies[accountId] || []).filter(
          (item) => item.reply.id !== replyId
        ),
      },
    })),

  setReplyHistory: (accountId, replies) =>
    set((state) => ({
      replyHistory: { ...state.replyHistory, [accountId]: replies },
    })),

  setSelectedTweet: (tweet) => set({ selectedTweet: tweet }),

  setGenerating: (isGenerating) => set({ isGenerating }),
}));
