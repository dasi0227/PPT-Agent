import { create } from 'zustand';

interface ConfirmationState {
  pending: { message: string; resolve: (confirmed: boolean) => void } | null;
}
export const useHistoryConfirmationStore = create<ConfirmationState>(() => ({ pending: null }));
export function confirmDiscardFuture(message: string): Promise<boolean> {
  // A confirmation authorizes one specific request. Concurrent requests must ask again.
  if (useHistoryConfirmationStore.getState().pending) return Promise.resolve(false);
  return new Promise((resolve) => {
    useHistoryConfirmationStore.setState({ pending: { message, resolve: (value) => {
      useHistoryConfirmationStore.setState({ pending: null });
      resolve(value);
    } } });
  });
}
