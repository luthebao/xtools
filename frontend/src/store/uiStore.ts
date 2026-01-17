import { create } from 'zustand';
import { toast } from 'sonner';

interface UIState {
  isModalOpen: boolean;
  modalContent: React.ReactNode | null;
  sidebarCollapsed: boolean;

  // Actions
  showToast: (message: string, type: 'success' | 'error' | 'info' | 'warning') => void;
  openModal: (content: React.ReactNode) => void;
  closeModal: () => void;
  toggleSidebar: () => void;
}

export const useUIStore = create<UIState>((set) => ({
  isModalOpen: false,
  modalContent: null,
  sidebarCollapsed: false,

  showToast: (message, type) => {
    switch (type) {
      case 'success':
        toast.success(message);
        break;
      case 'error':
        toast.error(message);
        break;
      case 'warning':
        toast.warning(message);
        break;
      case 'info':
        toast.info(message);
        break;
      default:
        toast(message);
    }
  },

  openModal: (content) => set({ isModalOpen: true, modalContent: content }),

  closeModal: () => set({ isModalOpen: false, modalContent: null }),

  toggleSidebar: () =>
    set((state) => ({ sidebarCollapsed: !state.sidebarCollapsed })),
}));
