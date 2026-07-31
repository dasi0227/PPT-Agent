import { create } from 'zustand';
import { blueprintsApi } from '../api/blueprints';
import type { BlueprintProjectView, SlideBlueprint } from '../api/types';

interface BlueprintState {
  byProjectId: Record<string, BlueprintProjectView>;
  loading: Record<string, boolean>;
  error: Record<string, string | undefined>;
  loadProject: (projectId: string) => Promise<void>;
  refreshSlide: (projectId: string, slideId: string) => Promise<void>;
  patchSlide: (projectId: string, slide: SlideBlueprint) => Promise<void>;
  clearProject: (projectId: string) => void;
}

export const useBlueprintStore = create<BlueprintState>((set) => ({
  byProjectId: {},
  loading: {},
  error: {},
  loadProject: async (projectId) => {
    set((state) => ({ loading: { ...state.loading, [projectId]: true } }));
    try {
      const view = await blueprintsApi.getProject(projectId);
      set((state) => ({
        byProjectId: { ...state.byProjectId, [projectId]: view },
        loading: { ...state.loading, [projectId]: false },
        error: { ...state.error, [projectId]: undefined },
      }));
    } catch (error) {
      set((state) => ({
        loading: { ...state.loading, [projectId]: false },
        error: { ...state.error, [projectId]: error instanceof Error ? error.message : String(error) },
      }));
    }
  },
  refreshSlide: async (projectId, slideId) => {
    const updated = await blueprintsApi.getSlide(slideId);
    set((state) => {
      const view = state.byProjectId[projectId];
      if (!view) return state;
      return {
        byProjectId: {
          ...state.byProjectId,
          [projectId]: {
            ...view,
            slides: { ...view.slides, [slideId]: updated.blueprint },
            materialization: { ...view.materialization, [slideId]: updated.materialization },
          },
        },
      };
    });
  },
  patchSlide: async (projectId, slide) => {
    const updated = await blueprintsApi.patchSlide(slide.slide_id, slide.revision, slide);
    set((state) => {
      const view = state.byProjectId[projectId];
      if (!view) return state;
      return { byProjectId: { ...state.byProjectId, [projectId]: {
        ...view,
        slides: { ...view.slides, [updated.slide_id]: updated },
        materialization: {
          ...view.materialization,
          [updated.slide_id]: { ...view.materialization[updated.slide_id], state: 'blueprint_stale' },
        },
      } } };
    });
  },
  clearProject: (projectId) => set((state) => {
    const next = { ...state.byProjectId };
    delete next[projectId];
    return { byProjectId: next };
  }),
}));
