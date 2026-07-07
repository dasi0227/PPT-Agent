import { create } from 'zustand';

export type PageView = 'outline' | 'html';

interface DeckState {
  currentPage: number;
  overviewOpen: boolean;
  previewMode: 'main' | 'overview' | 'single';
  iframeReady: boolean;
  loadCount: number;
  viewByPage: Record<string, PageView>;

  setCurrentPage: (index: number) => void;
  goNext: () => void;
  goPrev: () => void;
  enterOverview: () => void;
  exitOverview: () => void;
  setIframeReady: (ready: boolean) => void;
  reloadSlide: (index: number) => void;
  setPageView: (slideId: string, view: PageView) => void;
  effectiveView: (slideId: string, hasHtml: boolean) => PageView;
}

export const useDeckStore = create<DeckState>((set, get) => ({
  currentPage: 0,
  overviewOpen: false,
  previewMode: 'main',
  iframeReady: false,
  loadCount: 0,
  viewByPage: {},

  setCurrentPage: (index) => set({ currentPage: index }),
  goNext: () => set((state) => ({ currentPage: state.currentPage + 1 })),
  goPrev: () => set((state) => ({ currentPage: Math.max(0, state.currentPage - 1) })),
  enterOverview: () => set({ overviewOpen: true, previewMode: 'overview' }),
  exitOverview: () => set({ overviewOpen: false, previewMode: 'main' }),
  setIframeReady: (ready) => set({ iframeReady: ready }),
  reloadSlide: () => set((state) => ({ loadCount: state.loadCount + 1 })),

  // 每页视图偏好按 slideId 独立记忆。
  setPageView: (slideId, view) => set((state) => ({ viewByPage: { ...state.viewByPage, [slideId]: view } })),
  // 智能默认：用户手动选过则尊重其偏好，否则有 html 显 HTML、仅有 json 显大纲。
  effectiveView: (slideId, hasHtml) => {
    const v = get().viewByPage[slideId];
    return v ?? (hasHtml ? 'html' : 'outline');
  },
}));
