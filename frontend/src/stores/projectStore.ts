import { create } from 'zustand';
import { Project, Slide } from '../api/types';
import { projectsApi } from '../api/projects';
import { useThreadStore } from './threadStore';
import { useRunStore } from './runStore';
import { isDraftId, newDraftId } from '../lib/draft';

interface FlushOpts {
  topic: string;
  brief?: string;
  slide_count?: number;
  language?: string;
}

interface ProjectState {
  projects: Project[];   // 含草稿（draft:true 未落库）
  activeProjectId: string | null;
  slidesByProjectId: Record<string, Slide[]>;
  loadingProjects: boolean;

  loadProjects: () => Promise<void>;
  selectProject: (projectId: string) => void;
  loadProjectSlides: (projectId: string) => Promise<void>;
  createDraftProject: (title?: string) => string;
  flushProject: (tmpId: string, opts: FlushOpts) => Promise<string>;
  discardDraftProject: (projectId: string) => void;
  deleteProject: (projectId: string) => Promise<void>;
}

// 切 project 时关闭上一个 project 下所有 thread 的 SSE 连接（对齐 m7-design-spec §6.1）。
function closePreviousProjectSessions(prevProjectId: string | null) {
  if (!prevProjectId) return;
  const openIds = useThreadStore.getState().openThreadIdsByProjectId[prevProjectId] || [];
  if (openIds.length > 0) useRunStore.getState().closeSessions(openIds);
}

// flush 去重：同一草稿 project 的并发 flush 只发一次 POST。
const pendingProjectFlush = new Map<string, Promise<string>>();

export const useProjectStore = create<ProjectState>((set, get) => ({
  projects: [],
  activeProjectId: null,
  slidesByProjectId: {},
  loadingProjects: false,

  loadProjects: async () => {
    set({ loadingProjects: true });
    try {
      const projects = await projectsApi.list();
      // 合并：保留前端已存在的草稿 project（后端列表只含真实 project）。
      set((state) => {
        const drafts = state.projects.filter((p) => p.draft);
        return { projects: [...projects, ...drafts], loadingProjects: false };
      });
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
    if (prev === projectId) return;
    // 离开未 flush 的草稿 project → 直接丢弃（零残留）。
    if (prev && isDraftId(prev) && get().projects.find((p) => p.id === prev)?.draft) {
      get().discardDraftProject(prev);
    } else if (prev) {
      closePreviousProjectSessions(prev);
    }
    set({ activeProjectId: projectId });
    // 草稿 project 无后端数据：跳过 slides/threads 拉取（严禁把 draft_* 拼进 REST 路径）。
    if (!isDraftId(projectId)) {
      get().loadProjectSlides(projectId);
      useThreadStore.getState().loadThreads(projectId);
    }
  },

  loadProjectSlides: async (projectId: string) => {
    if (isDraftId(projectId)) return;
    try {
      const slides = await projectsApi.getSlides(projectId);
      set((state) => ({
        slidesByProjectId: { ...state.slidesByProjectId, [projectId]: slides }
      }));
    } catch (err) {
      console.error(err);
    }
  },

  // 软创建：建草稿 project + 选中，不发 POST /projects。
  createDraftProject: (title = 'New Presentation') => {
    const now = Math.floor(Date.now() / 1000);
    const draft: Project = {
      id: newDraftId(),
      title,
      theme: '',
      status: 'draft',
      created_at: now,
      updated_at: now,
      draft: true,
    };
    set((state) => ({ projects: [...state.projects, draft] }));
    get().selectProject(draft.id);
    return draft.id;
  },

  // flush：POST /projects 拿真实 project，用真实值替换草稿（id/title/...），
  // 改绑 projects/active/slides 与该 project 下所有草稿 thread 的归属键。去重 + 失败清理。
  flushProject: async (tmpId, opts) => {
    if (!isDraftId(tmpId)) return tmpId;

    const pending = pendingProjectFlush.get(tmpId);
    if (pending) return pending;

    const p = (async () => {
      const project = await projectsApi.create(
        opts.topic, opts.brief ?? '', opts.slide_count ?? 10, opts.language ?? 'zh',
      );
      // 先改绑 thread 归属键（thread id 不变），再替换 project 列表与 active。
      useThreadStore.getState().rekeyProject(tmpId, project.id);
      set((state) => {
        const projects = state.projects.map((pr) => (pr.id === tmpId ? project : pr));
        const slidesByProjectId = { ...state.slidesByProjectId };
        if (tmpId in slidesByProjectId) {
          slidesByProjectId[project.id] = slidesByProjectId[tmpId];
          delete slidesByProjectId[tmpId];
        }
        return {
          projects,
          slidesByProjectId,
          activeProjectId: state.activeProjectId === tmpId ? project.id : state.activeProjectId,
        };
      });
      return project.id;
    })();

    pendingProjectFlush.set(tmpId, p);
    try {
      return await p;
    } finally {
      pendingProjectFlush.delete(tmpId);
    }
  },

  // 丢弃草稿 project：纯前端移除 + 清理其 thread/session，切到相邻真实 project（零残留）。
  discardDraftProject: (projectId) => {
    if (!isDraftId(projectId)) return;
    useThreadStore.getState().dropProject(projectId);
    set((state) => {
      const projects = state.projects.filter((p) => p.id !== projectId);
      const slidesByProjectId = { ...state.slidesByProjectId };
      delete slidesByProjectId[projectId];
      let activeProjectId = state.activeProjectId;
      if (activeProjectId === projectId) {
        activeProjectId = projects[projects.length - 1]?.id ?? null;
      }
      return { projects, slidesByProjectId, activeProjectId };
    });
  },

  // 删除 project：草稿 → 丢弃；真实 → DELETE /projects/{id} + 清理本地并切相邻。
  deleteProject: async (projectId) => {
    if (isDraftId(projectId)) {
      get().discardDraftProject(projectId);
      return;
    }
    await projectsApi.delete(projectId);
    useThreadStore.getState().dropProject(projectId);
    const nextActive = (() => {
      const list = get().projects.filter((p) => p.id !== projectId);
      if (get().activeProjectId !== projectId) return get().activeProjectId;
      return list[list.length - 1]?.id ?? null;
    })();
    set((state) => {
      const projects = state.projects.filter((p) => p.id !== projectId);
      const slidesByProjectId = { ...state.slidesByProjectId };
      delete slidesByProjectId[projectId];
      return { projects, slidesByProjectId, activeProjectId: nextActive };
    });
    // 切到相邻真实 project 时加载其数据。
    if (nextActive && !isDraftId(nextActive)) {
      get().loadProjectSlides(nextActive);
      useThreadStore.getState().loadThreads(nextActive);
    }
  },
}));
