import { create } from 'zustand';

export type PageView = 'outline' | 'html';
export type ContentMode = 'preview' | 'source';
export type ProjectDocument = 'manifest' | 'design';

interface DeckState {
  currentSlideId: string | null;
  activeDocument: ProjectDocument | null;
  previewMode: 'main' | 'overview';
  globalView: PageView;
  contentMode: ContentMode;

  setCurrentSlideId: (slideId: string | null) => void;
  setActiveDocument: (document: ProjectDocument | null) => void;
  enterOverview: () => void;
  exitOverview: () => void;
  setGlobalView: (view: PageView) => void;
  setContentMode: (mode: ContentMode) => void;
  effectiveView: (slideId: string, hasHtml: boolean) => PageView;
}

export const useDeckStore = create<DeckState>((set, get) => ({
  currentSlideId: null,
  activeDocument: null,
  previewMode: 'main',
  globalView: 'html',
  contentMode: 'preview',

  setCurrentSlideId: (slideId) => set({ currentSlideId: slideId, activeDocument: null }),
  setActiveDocument: (document) => set({ activeDocument: document, previewMode: 'main', contentMode: 'preview' }),
  enterOverview: () => set({ previewMode: 'overview', activeDocument: null, contentMode: 'preview' }),
  exitOverview: () => set({ previewMode: 'main' }),

  setGlobalView: (view) => set({ globalView: view, contentMode: 'preview' }),
  setContentMode: (mode) => set({ contentMode: mode === 'source' && !get().activeDocument && get().globalView === 'html' && get().previewMode === 'main' ? 'source' : 'preview' }),

  // 全局视图优先：用户点“幻灯片”时即使当前页未生成 HTML，也保持幻灯片视图并展示空态。
  effectiveView: (_slideId, _hasHtml) => {
    if (get().globalView === 'outline') return 'outline';
    return 'html';
  },
}));
