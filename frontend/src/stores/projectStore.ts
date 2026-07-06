import { create } from 'zustand';
import { Project, Slide } from '../api/types';
import { projectsApi } from '../api/projects';
import { useThreadStore } from './threadStore';
import { useRunStore } from './runStore';

interface ProjectState {
  projects: Project[];
  activeProjectId: string | null;
  slidesByProjectId: Record<string, Slide[]>;
  loadingProjects: boolean;

  loadProjects: () => Promise<void>;
  selectProject: (projectId: string) => void;
  loadProjectSlides: (projectId: string) => Promise<void>;
  createProject: (topic: string, brief?: string, slide_count?: number, language?: string) => Promise<void>;
}

// 切 project 时关闭上一个 project 下所有 thread 的 SSE 连接（对齐 m7-design-spec §6.1）。
function closePreviousProjectSessions(prevProjectId: string | null) {
  if (!prevProjectId) return;
  const openIds = useThreadStore.getState().openThreadIdsByProjectId[prevProjectId] || [];
  if (openIds.length > 0) useRunStore.getState().closeSessions(openIds);
}

export const useProjectStore = create<ProjectState>((set, get) => ({
  projects: [],
  activeProjectId: null,
  slidesByProjectId: {},
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
    const prev = get().activeProjectId;
    if (prev !== projectId) closePreviousProjectSessions(prev);
    set({ activeProjectId: projectId });
    get().loadProjectSlides(projectId);
    useThreadStore.getState().loadThreads(projectId);
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

  createProject: async (topic: string, brief: string = '', slide_count: number = 10, language: string = 'zh') => {
    try {
      const project = await projectsApi.create(topic, brief, slide_count, language);
      set((state) => ({
        projects: [...state.projects, project],
      }));
      get().selectProject(project.id);
    } catch (err) {
      console.error(err);
    }
  }
}));
