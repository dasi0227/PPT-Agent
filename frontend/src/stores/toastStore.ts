import { create } from 'zustand';

export type ToastTone = 'neutral' | 'success' | 'warning' | 'error';

export interface ToastItem {
  id: string;
  message: string;
  tone: ToastTone;
}

interface ToastState {
  toasts: ToastItem[];
  pushToast: (message: string, tone?: ToastTone) => string;
  removeToast: (id: string) => void;
  clearToasts: () => void;
}

let nextToastId = 0;

export const useToastStore = create<ToastState>((set) => ({
  toasts: [],
  pushToast: (message, tone = 'neutral') => {
    nextToastId += 1;
    const id = `toast-${nextToastId}`;
    set((state) => ({ toasts: [...state.toasts, { id, message, tone }] }));
    return id;
  },
  removeToast: (id) => {
    set((state) => ({ toasts: state.toasts.filter((toast) => toast.id !== id) }));
  },
  clearToasts: () => set({ toasts: [] }),
}));

export function showGlobalError(message: string): string {
  return useToastStore.getState().pushToast(message, 'error');
}

export function showGlobalNotice(message: string): string {
  return useToastStore.getState().pushToast(message, 'neutral');
}

export function showGlobalWarning(message: string): string {
  return useToastStore.getState().pushToast(message, 'warning');
}

export function showGlobalSuccess(message: string): string {
  return useToastStore.getState().pushToast(message, 'success');
}
