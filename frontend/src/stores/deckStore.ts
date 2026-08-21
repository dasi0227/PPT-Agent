import { create } from 'zustand';

export type PageView = 'outline' | 'html';

interface DeckState {
  currentSlideId: string | null;
  previewMode: 'main' | 'overview';
  globalView: PageView;

  setCurrentSlideId: (slideId: string | null) => void;
  enterOverview: () => void;
  exitOverview: () => void;
  setGlobalView: (view: PageView) => void;
  effectiveView: (slideId: string, hasHtml: boolean) => PageView;
}

export const useDeckStore = create<DeckState>((set, get) => ({
  currentSlideId: null,
  previewMode: 'main',
  globalView: 'html',

  setCurrentSlideId: (slideId) => set({ currentSlideId: slideId }),
  enterOverview: () => set({ previewMode: 'overview' }),
  exitOverview: () => set({ previewMode: 'main' }),

  setGlobalView: (view) => set({ globalView: view }),

  // 全局视图优先：用户点“幻灯片”时即使当前页未生成 HTML，也保持幻灯片视图并展示空态。
  effectiveView: (_slideId, _hasHtml) => {
    if (get().globalView === 'outline') return 'outline';
    return 'html';
  },
}));
