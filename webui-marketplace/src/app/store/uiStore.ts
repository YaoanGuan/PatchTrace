import { create } from 'zustand';
import type { Toast } from '../../types/domain';

export interface LegalModalState {
  open: boolean;
  taskId: string | null;
}

export interface UiState {
  legalModal: LegalModalState;
  toasts: Toast[];
  openLegalModal(taskId: string): void;
  closeLegalModal(): void;
  pushToast(toast: Toast): void;
  dismissToast(id: string): void;
}

export const useUiStore = create<UiState>((set) => ({
  legalModal: { open: false, taskId: null },
  toasts: [],

  openLegalModal: (taskId) => set({ legalModal: { open: true, taskId } }),

  closeLegalModal: () => set({ legalModal: { open: false, taskId: null } }),

  pushToast: (toast) =>
    set((state) => {
      const exists = state.toasts.some((item) => item.id === toast.id);
      return {
        toasts: exists
          ? state.toasts.map((item) => (item.id === toast.id ? toast : item))
          : [...state.toasts, toast],
      };
    }),

  dismissToast: (id) =>
    set((state) => ({
      toasts: state.toasts.filter((toast) => toast.id !== id),
    })),
}));
