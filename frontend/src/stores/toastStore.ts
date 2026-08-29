import { create } from 'zustand';

export interface ErrorToast {
  id: string;
  message: string;
}

interface ToastState {
  errors: ErrorToast[];
  pushError: (message: string) => string;
  removeError: (id: string) => void;
  clearErrors: () => void;
}

let nextToastId = 0;

export const useToastStore = create<ToastState>((set) => ({
  errors: [],
  pushError: (message) => {
    nextToastId += 1;
    const id = `error-toast-${nextToastId}`;
    set((state) => ({ errors: [...state.errors, { id, message }] }));
    return id;
  },
  removeError: (id) => {
    set((state) => ({ errors: state.errors.filter((toast) => toast.id !== id) }));
  },
  clearErrors: () => set({ errors: [] }),
}));

export function showGlobalError(message: string): string {
  return useToastStore.getState().pushError(message);
}
