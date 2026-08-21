import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { Project, ProjectContentSnapshot, Slide, SlideSpec, SpecProjectView } from '../api/types';
import { projectsApi } from '../api/projects';
import { useThreadStore } from './threadStore';
import { specsApi } from '../api/specs';
import { APIError } from '../api/client';

const contentRequestVersions = new Map<string, number>();

function advanceContentRequest(projectId: string): number {
  const version = (contentRequestVersions.get(projectId) ?? 0) + 1;
  contentRequestVersions.set(projectId, version);
  return version;
}

interface ProjectState {
  projects: Project[];
  openProjectIds: string[];
  activeProjectId: string | null;
  slidesByProjectId: Record<string, Slide[]>;
  specByProjectId: Record<string, SpecProjectView>;
  contentLoadingByProjectId: Record<string, boolean>;
  contentErrorByProjectId: Record<string, string | undefined>;
  loadingProjects: boolean;
  projectError: string | null;

  loadProjects: () => Promise<void>;
  openProject: (id: string) => void;
  createProject: (topic: string, brief?: string, slide_count?: number, language?: string) => Promise<Project>;
  closeProject: (id: string) => void;
  renameProject: (id: string, title: string) => Promise<void>;
  deleteProject: (id: string) => Promise<void>;
  selectProject: (id: string) => void;
  loadProjectContent: (projectId: string) => Promise<void>;
  refreshSlideSpec: (projectId: string, slideId: string) => Promise<void>;
  patchSlideSpec: (projectId: string, slide: SlideSpec) => Promise<void>;
  applyProjectContentSnapshot: (projectId: string, snapshot: ProjectContentSnapshot) => void;
}

export const useProjectStore = create<ProjectState>()(
  persist(
    (set, get) => ({
      projects: [],
      openProjectIds: [],
      activeProjectId: null,
      slidesByProjectId: {},
      specByProjectId: {},
      contentLoadingByProjectId: {},
      contentErrorByProjectId: {},
      loadingProjects: false,
      projectError: null,

      loadProjects: async () => {
        set({ loadingProjects: true, projectError: null });
        try {
          const projects = await projectsApi.list();
          set((state) => {
            // Keep openProjectIds valid
            const validIds = new Set(projects.map(p => p.id));
            const newOpenIds = state.openProjectIds.filter(id => validIds.has(id));
            let newActive = state.activeProjectId;
            if (newActive && !validIds.has(newActive)) {
              newActive = newOpenIds.length > 0 ? newOpenIds[newOpenIds.length - 1] : null;
            }
            return { 
              projects, 
              loadingProjects: false,
              openProjectIds: newOpenIds,
              activeProjectId: newActive
            };
          });
          // Ensure we load slides and threads for the active project if it was restored from persistence
          const currentActive = get().activeProjectId;
          if (currentActive) {
            void get().loadProjectContent(currentActive);
            useThreadStore.getState().loadThreads(currentActive);
          }
        } catch (err) {
          set({
            loadingProjects: false,
            projectError: err instanceof Error ? err.message : '项目加载失败，请重试',
          });
        }
      },

      openProject: (id: string) => {
        set((state) => {
          if (!state.projects.find(p => p.id === id)) return state;
          const newOpenIds = state.openProjectIds.includes(id) 
            ? state.openProjectIds 
            : [...state.openProjectIds, id];
          return { openProjectIds: newOpenIds };
        });
        get().selectProject(id);
      },

      createProject: async (topic, brief = '', slide_count = 10, language = 'zh') => {
        const project = await projectsApi.create(topic, brief, slide_count, language);
        set((state) => ({
          projects: [...state.projects, project],
        }));
        return project;
      },

      closeProject: (id: string) => {
        set((state) => {
          const newOpenIds = state.openProjectIds.filter(pid => pid !== id);
          let newActive = state.activeProjectId;
          if (newActive === id) {
            newActive = newOpenIds.length > 0 ? newOpenIds[newOpenIds.length - 1] : null;
          }
          return { openProjectIds: newOpenIds, activeProjectId: newActive };
        });
        const newActive = get().activeProjectId;
        if (newActive) {
          get().selectProject(newActive);
        }
      },

      renameProject: async (id: string, title: string) => {
        await projectsApi.patch(id, { title });
        set((state) => ({
          projects: state.projects.map(p => p.id === id ? { ...p, title, updated_at: Math.floor(Date.now()/1000) } : p)
        }));
      },

      deleteProject: async (id: string) => {
        await projectsApi.delete(id);
        advanceContentRequest(id);
        useThreadStore.getState().dropProject(id);
        set((state) => {
          const projects = state.projects.filter((p) => p.id !== id);
          const openProjectIds = state.openProjectIds.filter(pid => pid !== id);
          const slidesByProjectId = { ...state.slidesByProjectId };
          const specByProjectId = { ...state.specByProjectId };
          const contentLoadingByProjectId = { ...state.contentLoadingByProjectId };
          const contentErrorByProjectId = { ...state.contentErrorByProjectId };
          delete slidesByProjectId[id];
          delete specByProjectId[id];
          delete contentLoadingByProjectId[id];
          delete contentErrorByProjectId[id];
          
          let activeProjectId = state.activeProjectId;
          if (activeProjectId === id) {
            activeProjectId = openProjectIds.length > 0 ? openProjectIds[openProjectIds.length - 1] : null;
          }
          return {
            projects,
            openProjectIds,
            slidesByProjectId,
            specByProjectId,
            contentLoadingByProjectId,
            contentErrorByProjectId,
            activeProjectId,
          };
        });
        
        const nextActive = get().activeProjectId;
        if (nextActive) {
          void get().loadProjectContent(nextActive);
          useThreadStore.getState().loadThreads(nextActive);
        }
      },

      selectProject: (projectId: string) => {
        if (get().activeProjectId === projectId) return;
        
        set({ activeProjectId: projectId });
        
        if (projectId) {
          void get().loadProjectContent(projectId);
          useThreadStore.getState().loadThreads(projectId);
        }
      },

      loadProjectContent: async (projectId: string) => {
        const requestVersion = advanceContentRequest(projectId);
        set((state) => ({
          contentLoadingByProjectId: { ...state.contentLoadingByProjectId, [projectId]: true },
          contentErrorByProjectId: { ...state.contentErrorByProjectId, [projectId]: undefined },
        }));
        try {
          const [slides, spec] = await Promise.all([
            projectsApi.getSlides(projectId),
            specsApi.getProject(projectId),
          ]);
          if (contentRequestVersions.get(projectId) !== requestVersion) return;
          set((state) => ({
            slidesByProjectId: { ...state.slidesByProjectId, [projectId]: slides },
            specByProjectId: { ...state.specByProjectId, [projectId]: spec },
            contentLoadingByProjectId: { ...state.contentLoadingByProjectId, [projectId]: false },
            contentErrorByProjectId: { ...state.contentErrorByProjectId, [projectId]: undefined },
            projectError: null,
          }));
        } catch (err) {
          if (contentRequestVersions.get(projectId) !== requestVersion) return;
          const message = err instanceof APIError && err.status === 404
            ? undefined
            : err instanceof Error ? err.message : '页面加载失败，请重试';
          set((state) => ({
            contentLoadingByProjectId: { ...state.contentLoadingByProjectId, [projectId]: false },
            contentErrorByProjectId: { ...state.contentErrorByProjectId, [projectId]: message },
            projectError: message ?? null,
          }));
        }
      },

      refreshSlideSpec: async (projectId, slideId) => {
        const updated = await specsApi.getSlide(slideId);
        set((state) => {
          const view = state.specByProjectId[projectId];
          if (!view) return state;
          return {
            specByProjectId: {
              ...state.specByProjectId,
              [projectId]: {
                ...view,
                slide_specs: { ...view.slide_specs, [slideId]: updated.spec },
                materialization: { ...view.materialization, [slideId]: updated.materialization },
              },
            },
          };
        });
      },

      patchSlideSpec: async (projectId, slide) => {
        const updated = await specsApi.patchSlide(slide.slide_id, slide.revision, slide);
        set((state) => {
          const view = state.specByProjectId[projectId];
          if (!view) return state;
          return {
            specByProjectId: {
              ...state.specByProjectId,
              [projectId]: {
                ...view,
                slide_specs: { ...view.slide_specs, [updated.slide_id]: updated },
                materialization: {
                  ...view.materialization,
                  [updated.slide_id]: { ...view.materialization[updated.slide_id], state: 'spec_stale' },
                },
              },
            },
          };
        });
      },

      applyProjectContentSnapshot: (projectId, snapshot) => {
        // Invalidate any older GET pair that may still be in flight so it
        // cannot overwrite this mutation response after it resolves.
        advanceContentRequest(projectId);
        set((state) => ({
          slidesByProjectId: { ...state.slidesByProjectId, [projectId]: snapshot.slides },
          specByProjectId: { ...state.specByProjectId, [projectId]: snapshot.spec },
          contentLoadingByProjectId: { ...state.contentLoadingByProjectId, [projectId]: false },
          contentErrorByProjectId: { ...state.contentErrorByProjectId, [projectId]: undefined },
          projectError: null,
        }));
      },
    }),
    { 
      name: 'ppt-agent-project-v6', 
      partialize: (s) => {
        return { openProjectIds: s.openProjectIds };
      },
      merge: (persisted, current) => ({
        ...current,
        ...(persisted as Partial<ProjectState>),
        activeProjectId: null,
      }),
    }
  )
);
