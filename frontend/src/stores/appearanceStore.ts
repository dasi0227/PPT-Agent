import { create } from 'zustand';

export type ColorMode = 'light' | 'dark';
// The head bootstrap uses this same key to apply the palette before first paint.
export const COLOR_MODE_KEY = 'dasi-color-mode';

const normalizeMode = (value: unknown): ColorMode => value === 'dark' ? 'dark' : 'light';

function applyMode(mode: ColorMode) {
  document.documentElement.dataset.colorMode = mode;
}

interface AppearanceState {
  colorMode: ColorMode;
  toggleColorMode: () => void;
}

export const useAppearanceStore = create<AppearanceState>((set, get) => ({
  colorMode: normalizeMode(document.documentElement.dataset.colorMode),
  toggleColorMode: () => {
    const colorMode = get().colorMode === 'dark' ? 'light' : 'dark';
    applyMode(colorMode);
    set({ colorMode });
    try { localStorage.setItem(COLOR_MODE_KEY, colorMode); } catch { /* The current tab can still switch when storage is unavailable. */ }
  },
}));

// A different tab can change or clear the preference without reloading this one.
const syncPreference = (event: StorageEvent) => {
  if (event.key !== COLOR_MODE_KEY && event.key !== null) return;
  const colorMode = normalizeMode(event.newValue);
  applyMode(colorMode);
  useAppearanceStore.setState({ colorMode });
};
window.addEventListener('storage', syncPreference);
if (import.meta.hot) import.meta.hot.dispose(() => window.removeEventListener('storage', syncPreference));
