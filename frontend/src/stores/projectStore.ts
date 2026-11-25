import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { Decorations, Project, ProjectContentSnapshot } from '../api/types';
import { projectsApi, type ProjectFiles, type ProjectFileWrite } from '../api/projects';
import { assetsApi } from '../api/assets';
import { useThreadStore } from './threadStore';
import { isProjectExportBlocking, useExportStore } from './exportStore';
import { isProjectDeleted, markProjectDeleted } from './deletedTargets';
import { showGlobalError } from './toastStore';

const contentRequests = new Map<string, number>();
const appearanceRequests = new Map<string, number>();

function advance(requests: Map<string, number>, id: string) {
  const version = (requests.get(id) ?? 0) + 1;
  requests.set(id, version);
  return version;
}

type Appearance = Pick<ProjectContentSnapshot, 'theme' | 'decorations' | 'logo' | 'appearance'>;
export const defaultPositions: Decorations = {
  slide_pagination: 'bottom-right', deck_title: 'none', chapter_title: 'none', logo: 'none',
};
const defaultAppearance: Appearance = { theme: null, decorations: defaultPositions, logo: null, appearance: null };

function asSnapshot(project: Project, files: ProjectFiles, appearance: Appearance): ProjectContentSnapshot {
  const hashes: Record<string, string> = {};
  for (const part of ['meta', 'constraint', 'outline'] as const) {
    const hash = files.hashes[part];
    if (hash !== null) hashes[part] = hash;
  }
  const slides = Object.entries(files.slides).map(([id, slide]) => {
    const { spec, html } = files.hashes.slides[id];
    if (spec !== null) hashes['spec:' + id] = spec;
    if (html !== null) hashes['html:' + id] = html;
    return [id, {
      spec: slide.spec,
      spec_state: slide.spec !== null ? 'ready' : spec !== null ? 'invalid' : 'pending',
      html: slide.html,
      html_state: slide.html !== null ? 'available' : 'missing',
      html_hash: html ?? '',
    }] as const;
  });
  return {
    project_id: project.id,
    project_title: project.title,
    ...appearance,
    meta: files.meta,
    constraint: files.constraint,
    outline: files.outline ?? { chapters: [] },
    hashes,
    slides_by_id: Object.fromEntries(slides),
  };
}

interface ProjectState {
  projects: Project[];
  openProjectIds: string[];
  activeProjectId: string | null;
  appearanceByProjectId: Record<string, Appearance>;
  contentByProjectId: Record<string, ProjectContentSnapshot>;
  contentLoadingByProjectId: Record<string, boolean>;
  contentErrorByProjectId: Record<string, string | undefined>;
  mutationPendingByProjectId: Record<string, boolean>;
  loadingProjects: boolean;
  loadProjects: () => Promise<boolean>;
  openProject: (id: string) => void;
  createProject: (title: string) => Promise<{ id: string }>;
  closeProject: (id: string) => string | null;
  renameProject: (id: string, title: string) => Promise<void>;
  deleteProject: (id: string) => Promise<void>;
  selectProject: (id: string) => void;
  setProjectTheme: (projectId: string, themeId: string) => Promise<void>;
  loadProjectAppearance: (projectId: string) => Promise<void>;
  setProjectDecorations: (projectId: string, positions: Decorations) => Promise<void>;
  uploadProjectLogo: (projectId: string, file: File) => Promise<void>;
  deleteProjectLogo: (projectId: string) => Promise<void>;
  loadProjectContent: (projectId: string) => Promise<void>;
  saveProjectFile: (projectId: string, baseHash: string | null, file: ProjectFileWrite) => Promise<string>;
}

export const useProjectStore = create<ProjectState>()(persist((set, get) => {
  let listVersion = 0;
  const refreshProjectList = async () => {
    const version = ++listVersion;
    set({ loadingProjects: true });
    try {
      const projects = (await projectsApi.list()).filter(project => !isProjectDeleted(project.id));
      if (version !== listVersion) return false;
      set(state => {
        const titles = new Map(projects.map(project => [project.id, project.title]));
        const openProjectIds = state.openProjectIds.filter(id => titles.has(id));
        const contentByProjectId = { ...state.contentByProjectId };
        for (const [id, snapshot] of Object.entries(contentByProjectId)) {
          const title = titles.get(id);
          if (title === undefined) delete contentByProjectId[id];
          else if (snapshot.project_title !== title) contentByProjectId[id] = { ...snapshot, project_title: title };
        }
        return {
          projects, openProjectIds, contentByProjectId,
          activeProjectId: state.activeProjectId && openProjectIds.includes(state.activeProjectId) ? state.activeProjectId : null,
        };
      });
      return true;
    } catch {
      return false;
    } finally {
      if (version === listVersion) set({ loadingProjects: false });
    }
  };
  const loadWorkspace = (id: string) => {
    void get().loadProjectContent(id);
    void get().loadProjectAppearance(id).catch(cause => showGlobalError(cause instanceof Error ? cause.message : '项目外观加载失败'));
    void useThreadStore.getState().loadThreads(id);
  };
  const withProjectWrite = async <T,>(projectId: string, write: () => Promise<T>) => {
    if (get().mutationPendingByProjectId[projectId]) throw new Error('正在保存修改');
    set(state => ({ mutationPendingByProjectId: { ...state.mutationPendingByProjectId, [projectId]: true } }));
    try {
      return await write();
    } finally {
      if (!isProjectDeleted(projectId)) set(state => ({ mutationPendingByProjectId: { ...state.mutationPendingByProjectId, [projectId]: false } }));
    }
  };
  const saveAppearance = (projectId: string, write: () => Promise<void>) => withProjectWrite(projectId, async () => {
    advance(appearanceRequests, projectId);
    await write();
    try {
      await get().loadProjectAppearance(projectId);
    } catch (cause) {
      showGlobalError('外观已保存，重新读取失败。' + (cause instanceof Error ? cause.message : ''));
    }
  });

  return {
    projects: [], openProjectIds: [], activeProjectId: null, appearanceByProjectId: {},
    contentByProjectId: {}, contentLoadingByProjectId: {}, contentErrorByProjectId: {}, mutationPendingByProjectId: {},
    loadingProjects: false,

    loadProjects: async () => {
      if (!await refreshProjectList()) return false;
      const id = get().activeProjectId;
      if (id) loadWorkspace(id);
      return true;
    },
    openProject: (id) => {
      if (isProjectDeleted(id)) return;
      set(state => ({ openProjectIds: state.openProjectIds.includes(id) ? state.openProjectIds : [...state.openProjectIds, id] }));
      get().selectProject(id);
    },
    createProject: async (title) => {
      const created = await projectsApi.create(title);
      await refreshProjectList();
      return created;
    },
    closeProject: (id) => {
      if (isProjectExportBlocking(id)) return get().activeProjectId;
      advance(contentRequests, id);
      set(state => {
        const openProjectIds = state.openProjectIds.filter(value => value !== id);
        return { openProjectIds, activeProjectId: state.activeProjectId === id ? openProjectIds[openProjectIds.length - 1] ?? null : state.activeProjectId };
      });
      return get().activeProjectId;
    },
    renameProject: async (id, title) => {
      await projectsApi.update(id, title);
      await refreshProjectList();
    },
    deleteProject: async (id) => {
      if (isProjectDeleted(id)) return;
      await projectsApi.delete(id);
      markProjectDeleted(id);
      advance(contentRequests, id);
      advance(appearanceRequests, id);
      useExportStore.getState().dropProject(id);
      useThreadStore.getState().dropProject(id);
      set(state => {
        const omit = <T,>({ [id]: ignored, ...rest }: Record<string, T>) => rest;
        const openProjectIds = state.openProjectIds.filter(value => value !== id);
        return {
          projects: state.projects.filter(project => project.id !== id), openProjectIds,
          activeProjectId: state.activeProjectId === id ? openProjectIds[openProjectIds.length - 1] ?? null : state.activeProjectId,
          appearanceByProjectId: omit(state.appearanceByProjectId),
          contentByProjectId: omit(state.contentByProjectId),
          contentLoadingByProjectId: omit(state.contentLoadingByProjectId),
          contentErrorByProjectId: omit(state.contentErrorByProjectId),
          mutationPendingByProjectId: omit(state.mutationPendingByProjectId),
        };
      });
    },
    selectProject: (id) => {
      if (isProjectDeleted(id)) return;
      const current = get().activeProjectId;
      if (current !== id && isProjectExportBlocking(current)) return;
      if (current !== id) set({ activeProjectId: id });
    },
    loadProjectAppearance: async (id) => {
      if (isProjectDeleted(id)) return;
      const version = advance(appearanceRequests, id);
      const response = await projectsApi.getAppearance(id);
      const previous = get().appearanceByProjectId[id];
      const [theme, logo] = await Promise.all([
        response.theme_id ? assetsApi.detail(response.theme_id) : null,
        response.logo ? projectsApi.getLogo(id, previous?.logo ?? null) : null,
      ]);
      if (appearanceRequests.get(id) !== version) return;
      const appearance: Appearance = {
        theme: response.theme_id,
        decorations: response.positions,
        logo,
        appearance: theme ? {
          hash: String(theme.updated_at),
          theme_css_url: 'data:text/css;charset=utf-8,' + encodeURIComponent(theme.content),
          decoration_tokens: {},
        } : null,
      };
      set(state => {
        const snapshot = state.contentByProjectId[id];
        return {
          appearanceByProjectId: { ...state.appearanceByProjectId, [id]: appearance },
          contentByProjectId: snapshot
            ? { ...state.contentByProjectId, [id]: { ...snapshot, ...appearance } }
            : state.contentByProjectId,
        };
      });
    },
    setProjectTheme: (id, themeId) => saveAppearance(id, () => projectsApi.setTheme(id, themeId)),
    setProjectDecorations: (id, positions) => saveAppearance(id, () => projectsApi.setPositions(id, positions)),
    uploadProjectLogo: (id, file) => saveAppearance(id, () => projectsApi.uploadLogo(id, file)),
    deleteProjectLogo: (id) => saveAppearance(id, () => projectsApi.deleteLogo(id)),
    loadProjectContent: async (id) => {
      if (isProjectDeleted(id)) return;
      const version = advance(contentRequests, id);
      set(state => ({
        contentLoadingByProjectId: { ...state.contentLoadingByProjectId, [id]: true },
        contentErrorByProjectId: { ...state.contentErrorByProjectId, [id]: undefined },
      }));
      try {
        const files = await projectsApi.getContent(id);
        if (contentRequests.get(id) !== version) return;
        set(state => {
          const project = state.projects.find(row => row.id === id);
          return {
            contentByProjectId: project ? {
              ...state.contentByProjectId,
              [id]: asSnapshot(project, files, state.appearanceByProjectId[id] ?? defaultAppearance),
            } : state.contentByProjectId,
            contentLoadingByProjectId: { ...state.contentLoadingByProjectId, [id]: false },
          };
        });
      } catch (cause) {
        if (contentRequests.get(id) !== version) return;
        set(state => ({
          contentLoadingByProjectId: { ...state.contentLoadingByProjectId, [id]: false },
          contentErrorByProjectId: { ...state.contentErrorByProjectId, [id]: cause instanceof Error ? cause.message : '页面加载失败，请重试' },
        }));
      }
    },
    saveProjectFile: (id, baseHash, file) => withProjectWrite(id, async () => {
      const { hash } = await projectsApi.saveFile(id, baseHash, file);
      await get().loadProjectContent(id);
      return hash;
    }),
  };
}, {
  name: 'ppt-agent-project-v7',
  partialize: state => ({ openProjectIds: state.openProjectIds }),
  merge: (persisted, current) => ({ ...current, ...persisted as Partial<ProjectState>, activeProjectId: null }),
}));
