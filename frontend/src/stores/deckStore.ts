import { create } from 'zustand';

interface DeckState {
  currentPage: number;
  overviewOpen: boolean;
  previewMode: 'main' | 'overview' | 'single';
  iframeReady: boolean;
  loadCount: number;

  setCurrentPage: (index: number) => void;
  goNext: () => void;
  goPrev: () => void;
  enterOverview: () => void;
  exitOverview: () => void;
  setIframeReady: (ready: boolean) => void;
  reloadSlide: (index: number) => void;
}

export const useDeckStore = create<DeckState>((set) => ({
  currentPage: 0,
  overviewOpen: false,
  previewMode: 'main',
  iframeReady: false,
  loadCount: 0,

  setCurrentPage: (index) => set({ currentPage: index }),
  goNext: () => set((state) => ({ currentPage: state.currentPage + 1 })),
  goPrev: () => set((state) => ({ currentPage: Math.max(0, state.currentPage - 1) })),
  enterOverview: () => set({ overviewOpen: true, previewMode: 'overview' }),
  exitOverview: () => set({ overviewOpen: false, previewMode: 'main' }),
  setIframeReady: (ready) => set({ iframeReady: ready }),
  reloadSlide: () => set((state) => ({ loadCount: state.loadCount + 1 }))
}));
