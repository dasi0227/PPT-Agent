import { create } from 'zustand';
import { specsApi } from '../api/specs';
import type { SlideSpec, SpecProjectView } from '../api/types';
import { APIError } from '../api/client';

interface SpecState {
  byProjectId: Record<string, SpecProjectView>;
  loading: Record<string, boolean>;
  error: Record<string, string | undefined>;
  loadProject: (projectId: string) => Promise<void>;
  refreshSlide: (projectId: string, slideId: string) => Promise<void>;
  patchSlide: (projectId: string, slide: SlideSpec) => Promise<void>;
  clearProject: (projectId: string) => void;
}

export const useSpecStore = create<SpecState>((set) => ({
  byProjectId: {},
  loading: {},
  error: {},
  loadProject: async (projectId) => {
    set((state) => ({ loading: { ...state.loading, [projectId]: true } }));
    try {
      const view = await specsApi.getProject(projectId);
      set((state) => ({
        byProjectId: { ...state.byProjectId, [projectId]: view },
        loading: { ...state.loading, [projectId]: false },
        error: { ...state.error, [projectId]: undefined },
      }));
    } catch (error) {
      set((state) => ({
        loading: { ...state.loading, [projectId]: false },
        error: {
          ...state.error,
          [projectId]: error instanceof APIError && error.status === 404
            ? undefined
            : error instanceof Error ? error.message : String(error),
        },
      }));
    }
  },
  refreshSlide: async (projectId, slideId) => {
    const updated = await specsApi.getSlide(slideId);
    set((state) => {
      const view = state.byProjectId[projectId];
      if (!view) return state;
      return {
        byProjectId: {
          ...state.byProjectId,
          [projectId]: {
            ...view,
            slide_specs: { ...view.slide_specs, [slideId]: updated.spec },
            materialization: { ...view.materialization, [slideId]: updated.materialization },
          },
        },
      };
    });
  },
  patchSlide: async (projectId, slide) => {
    const updated = await specsApi.patchSlide(slide.slide_id, slide.revision, slide);
    set((state) => {
      const view = state.byProjectId[projectId];
      if (!view) return state;
      return { byProjectId: { ...state.byProjectId, [projectId]: {
        ...view,
        slide_specs: { ...view.slide_specs, [updated.slide_id]: updated },
        materialization: {
          ...view.materialization,
          [updated.slide_id]: { ...view.materialization[updated.slide_id], state: 'spec_stale' },
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
