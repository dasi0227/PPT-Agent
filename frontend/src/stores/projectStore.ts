import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { Project, Slide } from '../api/types';
import { projectsApi } from '../api/projects';
import { useThreadStore } from './threadStore';
import { useRunStore } from './runStore';

interface ProjectState {
  projects: Project[];
  openProjectIds: string[];
  activeProjectId: string | null;
  pendingNewProject: boolean;
  slidesByProjectId: Record<string, Slide[]>;
  loadingProjects: boolean;

  loadProjects: () => Promise<void>;
  openProject: (id: string) => void;
  createProject: (topic: string, brief?: string, slide_count?: number, language?: string) => Promise<Project>;
  startPendingNewProject: () => void;
  finalizePendingNewProject: (realId: string) => void;
  cancelPendingNewProject: () => void;
  closeProject: (id: string) => void;
  renameProject: (id: string, title: string) => Promise<void>;
  deleteProject: (id: string) => Promise<void>;
  selectProject: (id: string) => void;
  loadProjectSlides: (projectId: string) => Promise<void>;
}

function closePreviousProjectSessions(prevProjectId: string | null) {
  if (!prevProjectId) return;
  const openIds = useThreadStore.getState().openThreadIdsByProjectId[prevProjectId] || [];
  if (openIds.length > 0) {
    useRunStore.getState().dropSessions(openIds);
  }
}

export const useProjectStore = create<ProjectState>()(
  persist(
    (set, get) => ({
      projects: [],
      openProjectIds: [],
      activeProjectId: null,
      pendingNewProject: false,
      slidesByProjectId: {},
      loadingProjects: false,

      loadProjects: async () => {
        set({ loadingProjects: true });
        try {
          const projects = await projectsApi.list();
          set((state) => {
            // Keep openProjectIds valid
            const validIds = new Set(projects.map(p => p.id));
            const newOpenIds = state.openProjectIds.filter(id => validIds.has(id));
            let newActive = state.activeProjectId;
            if (newActive && newActive !== 'new-pending' && !validIds.has(newActive)) {
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
          if (currentActive && currentActive !== 'new-pending') {
            get().loadProjectSlides(currentActive);
            useThreadStore.getState().loadThreads(currentActive);
          }
        } catch (err) {
          set({ loadingProjects: false });
          console.error(err);
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

      startPendingNewProject: () => {
        set((state) => {
          const newOpenIds = state.openProjectIds.includes('new-pending') 
            ? state.openProjectIds 
            : [...state.openProjectIds, 'new-pending'];
          return {
            pendingNewProject: true,
            openProjectIds: newOpenIds
          };
        });
        get().selectProject('new-pending');
      },

      finalizePendingNewProject: (realId: string) => {
        set((state) => {
          const newOpenIds = state.openProjectIds.map(id => id === 'new-pending' ? realId : id);
          return {
            pendingNewProject: false,
            openProjectIds: newOpenIds
          };
        });
        get().selectProject(realId);
      },

      cancelPendingNewProject: () => {
        set((state) => {
          const newOpenIds = state.openProjectIds.filter(id => id !== 'new-pending');
          let newActive = state.activeProjectId;
          if (newActive === 'new-pending') {
            newActive = newOpenIds.length > 0 ? newOpenIds[newOpenIds.length - 1] : null;
          }
          return {
            pendingNewProject: false,
            openProjectIds: newOpenIds,
            activeProjectId: newActive
          };
        });
      },

      closeProject: (id: string) => {
        set((state) => {
          if (id === 'new-pending') {
            get().cancelPendingNewProject();
            return state;
          }
          const newOpenIds = state.openProjectIds.filter(pid => pid !== id);
          let newActive = state.activeProjectId;
          if (newActive === id) {
            newActive = newOpenIds.length > 0 ? newOpenIds[newOpenIds.length - 1] : null;
          }
          return { openProjectIds: newOpenIds, activeProjectId: newActive };
        });
        const newActive = get().activeProjectId;
        if (newActive && newActive !== 'new-pending') {
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
        if (id === 'new-pending') {
          get().cancelPendingNewProject();
          return;
        }
        await projectsApi.delete(id);
        useThreadStore.getState().dropProject(id);
        set((state) => {
          const projects = state.projects.filter((p) => p.id !== id);
          const openProjectIds = state.openProjectIds.filter(pid => pid !== id);
          const slidesByProjectId = { ...state.slidesByProjectId };
          delete slidesByProjectId[id];
          
          let activeProjectId = state.activeProjectId;
          if (activeProjectId === id) {
            activeProjectId = openProjectIds.length > 0 ? openProjectIds[openProjectIds.length - 1] : null;
          }
          return { projects, openProjectIds, slidesByProjectId, activeProjectId };
        });
        
        const nextActive = get().activeProjectId;
        if (nextActive && nextActive !== 'new-pending') {
          get().loadProjectSlides(nextActive);
          useThreadStore.getState().loadThreads(nextActive);
        }
      },

      selectProject: (projectId: string) => {
        const prev = get().activeProjectId;
        if (prev === projectId) return;
        
        if (prev && prev !== 'new-pending') {
          closePreviousProjectSessions(prev);
        }
        
        set({ activeProjectId: projectId });
        
        if (projectId && projectId !== 'new-pending') {
          get().loadProjectSlides(projectId);
          useThreadStore.getState().loadThreads(projectId);
        }
      },

      loadProjectSlides: async (projectId: string) => {
        if (projectId === 'new-pending') return;
        try {
          const slides = await projectsApi.getSlides(projectId);
          set((state) => ({
            slidesByProjectId: { ...state.slidesByProjectId, [projectId]: slides }
          }));
        } catch (err) {
          console.error(err);
        }
      },
    }),
    { 
      name: 'ppt-agent-project-v6', 
      partialize: (s) => {
        // filter out new-pending from persistence
        const openProjectIds = s.openProjectIds.filter(id => id !== 'new-pending');
        const activeProjectId = s.activeProjectId === 'new-pending' ? (openProjectIds.length > 0 ? openProjectIds[openProjectIds.length - 1] : null) : s.activeProjectId;
        return { openProjectIds, activeProjectId };
      }
    }
  )
);
