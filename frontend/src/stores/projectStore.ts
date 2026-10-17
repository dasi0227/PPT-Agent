import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { Decorations, PPTMutation, Project, ProjectContentSnapshot } from '../api/types';
import { projectsApi } from '../api/projects';
import { APIError } from '../api/client';
import { useThreadStore } from './threadStore';
import { isProjectExportBlocking, useExportStore } from './exportStore';
import { isProjectDeleted, markProjectDeleted, restoreProject, restoreThread } from './deletedTargets';

const requestVersions = new Map<string, number>();
const advance = (id: string) => { const next = (requestVersions.get(id) ?? 0) + 1; requestVersions.set(id, next); return next; };
const contentChecks = new Map<string, Promise<void>>();
const appearanceVersions = new Map<string, number>();
const advanceAppearance = (id: string) => { const next = (appearanceVersions.get(id) ?? 0) + 1; appearanceVersions.set(id, next); return next; };

function sameDecorations(a: Decorations, b: Decorations): boolean {
  return a.slide_pagination === b.slide_pagination && a.deck_title === b.deck_title
    && a.section_title === b.section_title && a.logo === b.logo;
}

function sameContent(a: ProjectContentSnapshot, b: ProjectContentSnapshot): boolean {
  if (a.project_id !== b.project_id || a.scene_revision !== b.scene_revision) return false;
  if (a.project_title !== b.project_title || Boolean(a.manifest) !== Boolean(b.manifest) || Boolean(a.design) !== Boolean(b.design)) return false;
  if (a.theme !== b.theme) return false;
  if (a.theme_error !== b.theme_error) return false;
  if (a.appearance?.hash !== b.appearance?.hash) return false;
  if (!sameDecorations(a.decorations, b.decorations)) return false;
  if (a.logo?.hash !== b.logo?.hash || a.logo?.src !== b.logo?.src) return false;
  const aHashes = Object.entries(a.hashes);
  if (aHashes.length !== Object.keys(b.hashes).length || aHashes.some(([key, value]) => b.hashes[key] !== value)) return false;
  const aSlides = Object.entries(a.slides_by_id);
  if (aSlides.length !== Object.keys(b.slides_by_id).length) return false;
  return aSlides.every(([id, slide]) => {
    const other = b.slides_by_id[id];
    return other && slide.html_hash === other.html_hash && slide.html_state === other.html_state
      && slide.spec_state === other.spec_state;
  });
}

interface ProjectState {
  projects: Project[]; openProjectIds: string[]; activeProjectId: string | null;
  contentByProjectId: Record<string, ProjectContentSnapshot>;
  contentLoadingByProjectId: Record<string, boolean>; contentErrorByProjectId: Record<string, string | undefined>;
  mutationPendingByProjectId: Record<string, boolean>; loadingProjects: boolean;
  loadProjects: () => Promise<void>; openProject: (id: string) => void;
  createProject: (topic: string) => Promise<Project>;
  closeProject: (id: string) => string | null; renameProject: (id: string, title: string) => Promise<void>; deleteProject: (id: string) => Promise<void>; selectProject: (id: string) => void;
  setProjectTheme: (projectId: string, themeId: string) => Promise<Project>;
  loadProjectAppearance: (projectId: string, signal?: AbortSignal) => Promise<void>;
  setProjectDecorations: (projectId: string, updates: Partial<Decorations>) => Promise<Project>;
  uploadProjectLogo: (projectId: string, file: File) => Promise<Project>;
  deleteProjectLogo: (projectId: string) => Promise<Project>;
  loadProjectContent: (projectId: string) => Promise<void>; mutateProject: (projectId: string, mutation: PPTMutation) => Promise<ProjectContentSnapshot>;
  checkProjectContent: (projectId: string) => Promise<void>;
  applyProjectContentSnapshot: (projectId: string, snapshot: ProjectContentSnapshot) => void;
}

export const useProjectStore = create<ProjectState>()(persist((set, get) => {
  const applyAppearance = (projectId: string, appearance: Pick<Project, 'decorations' | 'logo'>, project?: Project) => {
    set((state) => {
      if (!state.projects.some((value) => value.id === projectId)) return state;
      const snapshot = state.contentByProjectId[projectId];
      const logo = appearance.logo ? { hash: appearance.logo.hash, src: projectsApi.logoContentUrl(projectId, appearance.logo.hash) } : null;
      const changed = snapshot && (!sameDecorations(snapshot.decorations, appearance.decorations)
        || snapshot.logo?.hash !== logo?.hash || snapshot.logo?.src !== logo?.src);
      return {
        projects: state.projects.map((value) => value.id === projectId ? project ?? { ...value, ...appearance } : value),
        contentByProjectId: changed ? {
          ...state.contentByProjectId,
          [projectId]: {
            ...snapshot,
            decorations: appearance.decorations,
            logo,
          },
        } : state.contentByProjectId,
      };
    });
  };
  const saveAppearance = async (projectId: string, write: () => Promise<Project>): Promise<Project> => {
    if (get().mutationPendingByProjectId[projectId]) throw new Error('正在保存修改');
    advanceAppearance(projectId);
    set((state) => ({ mutationPendingByProjectId: { ...state.mutationPendingByProjectId, [projectId]: true } }));
    try {
      const project = await write();
      applyAppearance(projectId, project, project);
      await get().loadProjectContent(projectId);
      return project;
    } finally {
      if (!isProjectDeleted(projectId)) set((state) => ({ mutationPendingByProjectId: { ...state.mutationPendingByProjectId, [projectId]: false } }));
    }
  };
  return {
  projects: [], openProjectIds: [], activeProjectId: null, contentByProjectId: {}, contentLoadingByProjectId: {}, contentErrorByProjectId: {}, mutationPendingByProjectId: {}, loadingProjects: false,
  loadProjects: async () => { set({ loadingProjects:true });try{const projects=(await projectsApi.list()).filter(project => !isProjectDeleted(project.id));set((state)=>{const ids=new Set(projects.map((p)=>p.id));const openProjectIds=state.openProjectIds.filter((id)=>ids.has(id));const activeProjectId=state.activeProjectId&&openProjectIds.includes(state.activeProjectId)?state.activeProjectId:null;return{projects,openProjectIds,activeProjectId,loadingProjects:false}});const id=get().activeProjectId;if(id){void get().loadProjectContent(id);void useThreadStore.getState().loadThreads(id)}}catch{set({loadingProjects:false})}},
  openProject: (id) => { if(isProjectDeleted(id))return;set((state)=>({openProjectIds:state.openProjectIds.includes(id)?state.openProjectIds:[...state.openProjectIds,id]}));get().selectProject(id) },
  createProject: async (topic) => { const project=await projectsApi.create(topic);set((state)=>({projects:[...state.projects,project]}));return project },
  closeProject: (id) => { if(isProjectExportBlocking(id))return get().activeProjectId;let nextActiveProjectId: string | null = null;advance(id);useThreadStore.getState().closeProjectThreads(id);set((state)=>{const openProjectIds=state.openProjectIds.filter((value)=>value!==id);nextActiveProjectId=state.activeProjectId===id?openProjectIds[openProjectIds.length-1]??null:state.activeProjectId;return{openProjectIds,activeProjectId:nextActiveProjectId}});return nextActiveProjectId },
  renameProject: async (id,title) => { await projectsApi.patch(id,{title});set((state)=>({projects:state.projects.map((project)=>project.id===id?{...project,title}:project)})) },
  deleteProject: async (id) => {
    if (isProjectDeleted(id)) return;
    markProjectDeleted(id);
    advance(id);
    advanceAppearance(id);
    useExportStore.getState().dropProject(id);
    const threadIds = useThreadStore.getState().dropProject(id);
    set((state) => {
      const omit = <T,>(values: Record<string, T>) => { const next = { ...values }; delete next[id]; return next; };
      const openProjectIds = state.openProjectIds.filter(value => value !== id);
      return {
        projects: state.projects.filter(project => project.id !== id), openProjectIds,
        activeProjectId: state.activeProjectId === id ? openProjectIds[openProjectIds.length - 1] ?? null : state.activeProjectId,
        contentByProjectId: omit(state.contentByProjectId), contentLoadingByProjectId: omit(state.contentLoadingByProjectId),
        contentErrorByProjectId: omit(state.contentErrorByProjectId), mutationPendingByProjectId: omit(state.mutationPendingByProjectId),
      };
    });
    try { await projectsApi.delete(id); }
    catch (error) {
      try {
        const projects = await projectsApi.list();
        if (projects.some(project => project.id === id)) {
          restoreProject(id);
          threadIds.forEach(restoreThread);
          await get().loadProjects();
        }
      } catch { /* Keep the fence when acceptance could not be reconciled. */ }
      throw error;
    }
  },
  setProjectTheme: (projectId,themeId) => saveAppearance(projectId, () => projectsApi.setTheme(projectId,themeId)),
  loadProjectAppearance: async (projectId,signal) => {
    if (isProjectDeleted(projectId)) return;
    const version = advanceAppearance(projectId);
    const [decorations, logo] = await Promise.all([
      projectsApi.getDecorations(projectId, signal), projectsApi.getLogo(projectId, signal),
    ]);
    if (signal?.aborted || appearanceVersions.get(projectId) !== version || get().mutationPendingByProjectId[projectId]) return;
    applyAppearance(projectId, { decorations, logo });
    await get().loadProjectContent(projectId);
  },
  setProjectDecorations: (projectId,updates) => saveAppearance(projectId, () => projectsApi.setDecorations(projectId,updates)),
  uploadProjectLogo: (projectId,file) => saveAppearance(projectId, () => projectsApi.uploadLogo(projectId,file)),
  deleteProjectLogo: (projectId) => saveAppearance(projectId, () => projectsApi.deleteLogo(projectId)),
  selectProject: (id) => { if(isProjectDeleted(id))return;const current=get().activeProjectId;if(current!==id&&isProjectExportBlocking(current))return;if(current!==id)set({activeProjectId:id});void get().loadProjectContent(id);void useThreadStore.getState().loadThreads(id) },
  loadProjectContent: async (projectId) => { if(isProjectDeleted(projectId))return;const version=advance(projectId);set((state)=>({contentLoadingByProjectId:{...state.contentLoadingByProjectId,[projectId]:true},contentErrorByProjectId:{...state.contentErrorByProjectId,[projectId]:undefined}}));try{const snapshot=await projectsApi.getContent(projectId);if(requestVersions.get(projectId)!==version)return;const current=get().contentByProjectId[projectId];if(current&&sameContent(current,snapshot)){set((state)=>({contentLoadingByProjectId:{...state.contentLoadingByProjectId,[projectId]:false}}));return}get().applyProjectContentSnapshot(projectId,snapshot)}catch(error){if(requestVersions.get(projectId)!==version)return;const message=error instanceof APIError&&error.status===404?undefined:error instanceof Error?error.message:'页面加载失败，请重试';set((state)=>({contentLoadingByProjectId:{...state.contentLoadingByProjectId,[projectId]:false},contentErrorByProjectId:{...state.contentErrorByProjectId,[projectId]:message}}))} },
  checkProjectContent: (projectId) => {
    if (isProjectDeleted(projectId)) return Promise.resolve();
    const pending = contentChecks.get(projectId);
    if (pending) return pending;
    if (get().mutationPendingByProjectId[projectId] || get().contentLoadingByProjectId[projectId]) return Promise.resolve();
    const version = requestVersions.get(projectId) ?? 0;
    const check = projectsApi.getContent(projectId).then((snapshot) => {
      if (requestVersions.get(projectId) !== version || get().mutationPendingByProjectId[projectId]) return;
      const current = get().contentByProjectId[projectId];
      if (!current || !sameContent(current, snapshot)) get().applyProjectContentSnapshot(projectId, snapshot);
    }).catch(() => {
      // Background checks leave the visible project and its error state untouched.
    }).finally(() => { contentChecks.delete(projectId); });
    contentChecks.set(projectId, check);
    return check;
  },
  mutateProject: async (projectId,mutation) => { if(get().mutationPendingByProjectId[projectId])throw new Error('结构操作正在进行');advance(projectId);set((state)=>({mutationPendingByProjectId:{...state.mutationPendingByProjectId,[projectId]:true}}));try{const response=await projectsApi.mutate(projectId,mutation);get().applyProjectContentSnapshot(projectId,response.content);return response.content}finally{if(!isProjectDeleted(projectId))set((state)=>({mutationPendingByProjectId:{...state.mutationPendingByProjectId,[projectId]:false}}))} },
  applyProjectContentSnapshot: (projectId,snapshot) => { if(isProjectDeleted(projectId))return;advance(projectId);set((state)=>({contentByProjectId:{...state.contentByProjectId,[projectId]:snapshot},contentLoadingByProjectId:{...state.contentLoadingByProjectId,[projectId]:false},contentErrorByProjectId:{...state.contentErrorByProjectId,[projectId]:undefined}})) },
  };
},{name:'ppt-agent-project-v7',partialize:(state)=>({openProjectIds:state.openProjectIds}),merge:(persisted,current)=>({...current,...persisted as Partial<ProjectState>,activeProjectId:null})}));
