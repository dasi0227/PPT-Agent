import { create } from 'zustand';

export type PageView = 'outline' | 'html';

interface DeckState {
  currentPage: number;
  previewMode: 'main' | 'overview';
  globalView: PageView;

  setCurrentPage: (index: number) => void;
  goNext: () => void;
  goPrev: () => void;
  enterOverview: () => void;
  exitOverview: () => void;
  setGlobalView: (view: PageView) => void;
  effectiveView: (slideId: string, hasHtml: boolean) => PageView;
}

export const useDeckStore = create<DeckState>((set, get) => ({
  currentPage: 0,
  previewMode: 'main',
  globalView: 'html',

  setCurrentPage: (index) => set({ currentPage: index }),
  goNext: () => set((state) => ({ currentPage: state.currentPage + 1 })),
  goPrev: () => set((state) => ({ currentPage: Math.max(0, state.currentPage - 1) })),
  enterOverview: () => set({ previewMode: 'overview' }),
  exitOverview: () => set({ previewMode: 'main' }),

  setGlobalView: (view) => set({ globalView: view }),

  // 全局视图优先：outline 全部走大纲；html 全局时按页 hasHtml 兜底（无 html → outline）。
  effectiveView: (_slideId, hasHtml) => {
    if (get().globalView === 'outline') return 'outline';
    return hasHtml ? 'html' : 'outline';
  },
}));
