import { create } from 'zustand';

export type PageView = 'outline' | 'html';
export type ContentMode = 'preview' | 'source';
export type ProjectDocument = 'manifest' | 'design' | 'outline';

interface DeckState {
  currentSlideId: string | null;
  activeDocument: ProjectDocument | null;
  previewMode: 'main' | 'overview';
  globalView: PageView;
  contentMode: ContentMode;
  sourceBlocked: boolean;
  setSourceBlocked: (blocked: boolean) => void;

  setCurrentSlideId: (slideId: string | null) => void;
  setActiveDocument: (document: ProjectDocument | null) => void;
  enterOverview: () => void;
  exitOverview: () => void;
  setGlobalView: (view: PageView) => void;
  setContentMode: (mode: ContentMode) => void;
}

export const useDeckStore = create<DeckState>((set, get) => ({
  currentSlideId: null,
  activeDocument: null,
  previewMode: 'main',
  globalView: 'html',
  contentMode: 'preview',
  sourceBlocked: false,
  setSourceBlocked: (sourceBlocked) => set({ sourceBlocked }),

  setCurrentSlideId: (slideId) => set({ currentSlideId: slideId, activeDocument: null }),
  setActiveDocument: (document) => set({ activeDocument: document, previewMode: 'main' }),
  enterOverview: () => set({ previewMode: 'overview', activeDocument: null }),
  exitOverview: () => set({ previewMode: 'main' }),

  setGlobalView: (view) => set({ globalView: view }),
  setContentMode: (mode) => {
    if (mode === 'source' && (get().sourceBlocked || get().previewMode !== 'main')) return;
    set({ contentMode: mode });
  },
}));
