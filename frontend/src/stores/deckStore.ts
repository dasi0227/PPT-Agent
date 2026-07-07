import { create } from 'zustand';

export type PageView = 'outline' | 'html';

interface DeckState {
  currentPage: number;
  overviewOpen: boolean;
  previewMode: 'main' | 'overview' | 'single';
  iframeReady: boolean;
  loadCount: number;
  viewByPage: Record<string, PageView>;
  globalView: PageView;

  setCurrentPage: (index: number) => void;
  goNext: () => void;
  goPrev: () => void;
  enterOverview: () => void;
  exitOverview: () => void;
  setIframeReady: (ready: boolean) => void;
  reloadSlide: (index: number) => void;
  // 保留但不再参与 effectiveView 决策；仅写入 viewByPage 以维持 API 兼容（Phase 3 后可清理）。
  setPageView: (slideId: string, view: PageView) => void;
  setGlobalView: (view: PageView) => void;
  effectiveView: (slideId: string, hasHtml: boolean) => PageView;
}

export const useDeckStore = create<DeckState>((set, get) => ({
  currentPage: 0,
  overviewOpen: false,
  previewMode: 'main',
  iframeReady: false,
  loadCount: 0,
  viewByPage: {},
  globalView: 'html',

  setCurrentPage: (index) => set({ currentPage: index }),
  goNext: () => set((state) => ({ currentPage: state.currentPage + 1 })),
  goPrev: () => set((state) => ({ currentPage: Math.max(0, state.currentPage - 1) })),
  enterOverview: () => set({ overviewOpen: true, previewMode: 'overview' }),
  exitOverview: () => set({ overviewOpen: false, previewMode: 'main' }),
  setIframeReady: (ready) => set({ iframeReady: ready }),
  reloadSlide: () => set((state) => ({ loadCount: state.loadCount + 1 })),

  // 写入 viewByPage，但决策由 globalView 主导（保留仅为兼容 Phase 3 之前的调用点）。
  setPageView: (slideId, view) => set((state) => ({ viewByPage: { ...state.viewByPage, [slideId]: view } })),
  setGlobalView: (view) => set({ globalView: view }),

  // 全局视图优先：outline 全部走大纲；html 全局时按页 hasHtml 兜底（无 html → outline）。
  effectiveView: (_slideId, hasHtml) => {
    if (get().globalView === 'outline') return 'outline';
    return hasHtml ? 'html' : 'outline';
  },
}));
