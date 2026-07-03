import { create } from 'zustand';
import { Project, Slide, Thread } from '../api/types';
import { projectsApi } from '../api/projects';

interface ProjectState {
  projects: Project[];
  activeProjectId: string | null;
  slidesByProjectId: Record<string, Slide[]>;
  threadsByProjectId: Record<string, Thread[]>;
  loadingProjects: boolean;

  loadProjects: () => Promise<void>;
  selectProject: (projectId: string) => void;
  loadProjectSlides: (projectId: string) => Promise<void>;
  loadProjectThreads: (projectId: string) => Promise<void>;
  createProject: (topic: string, brief?: string, slide_count?: number, language?: string) => Promise<void>;
}

export const useProjectStore = create<ProjectState>((set, get) => ({
  projects: [],
  activeProjectId: null,
  slidesByProjectId: {},
  threadsByProjectId: {},
  loadingProjects: false,

  loadProjects: async () => {
    set({ loadingProjects: true });
    try {
      const projects = await projectsApi.list();
      set({ projects, loadingProjects: false });
      if (projects.length > 0 && !get().activeProjectId) {
        get().selectProject(projects[0].id);
      }
    } catch (err) {
      set({ loadingProjects: false });
      console.error(err);
    }
  },

  selectProject: (projectId: string) => {
    set({ activeProjectId: projectId });
    get().loadProjectSlides(projectId);
    get().loadProjectThreads(projectId);
  },

  loadProjectSlides: async (projectId: string) => {
    try {
      const slides = await projectsApi.getSlides(projectId);
      set((state) => ({
        slidesByProjectId: { ...state.slidesByProjectId, [projectId]: slides }
      }));
    } catch (err) {
      console.error(err);
    }
  },

  loadProjectThreads: async (projectId: string) => {
    try {
      const threads = await projectsApi.getThreads(projectId);
      set((state) => ({
        threadsByProjectId: { ...state.threadsByProjectId, [projectId]: threads }
      }));
    } catch (err) {
      console.error(err);
    }
  },

  createProject: async (topic: string, brief: string = '', slide_count: number = 10, language: string = 'zh') => {
    try {
      const project = await projectsApi.create(topic, brief, slide_count, language);
      set((state) => ({
        projects: [...state.projects, project],
        activeProjectId: project.id
      }));
      get().selectProject(project.id);
    } catch (err) {
      console.error(err);
    }
  }
}));
