import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { PPTMutation, Project, ProjectContentSnapshot } from '../api/types';
import { projectsApi } from '../api/projects';
import { APIError } from '../api/client';
import { useThreadStore } from './threadStore';
import { isProjectExportBlocking } from './exportStore';

const requestVersions = new Map<string, number>();
const advance = (id: string) => { const next = (requestVersions.get(id) ?? 0) + 1; requestVersions.set(id, next); return next; };
const contentChecks = new Map<string, Promise<void>>();

function sameContent(a: ProjectContentSnapshot, b: ProjectContentSnapshot): boolean {
  if (a.project_id !== b.project_id) return false;
  if (a.theme !== b.theme) return false;
  if (a.theme_error !== b.theme_error) return false;
  if (a.appearance?.hash !== b.appearance?.hash) return false;
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
  mutationPendingByProjectId: Record<string, boolean>; loadingProjects: boolean; projectError: string | null;
  loadProjects: () => Promise<void>; openProject: (id: string) => void;
  createProject: (topic: string) => Promise<Project>;
  closeProject: (id: string) => string | null; renameProject: (id: string, title: string) => Promise<void>; deleteProject: (id: string) => Promise<void>; selectProject: (id: string) => void;
  setProjectTheme: (projectId: string, themeId: string) => Promise<Project>;
  loadProjectContent: (projectId: string) => Promise<void>; mutateProject: (projectId: string, mutation: PPTMutation) => Promise<ProjectContentSnapshot>;
  checkProjectContent: (projectId: string) => Promise<void>;
  applyProjectContentSnapshot: (projectId: string, snapshot: ProjectContentSnapshot) => void;
}

export const useProjectStore = create<ProjectState>()(persist((set, get) => ({
  projects: [], openProjectIds: [], activeProjectId: null, contentByProjectId: {}, contentLoadingByProjectId: {}, contentErrorByProjectId: {}, mutationPendingByProjectId: {}, loadingProjects: false, projectError: null,
  loadProjects: async () => { set({ loadingProjects:true,projectError:null });try{const projects=await projectsApi.list();set((state)=>{const ids=new Set(projects.map((p)=>p.id));const openProjectIds=state.openProjectIds.filter((id)=>ids.has(id));const activeProjectId=state.activeProjectId&&openProjectIds.includes(state.activeProjectId)?state.activeProjectId:null;return{projects,openProjectIds,activeProjectId,loadingProjects:false}});const id=get().activeProjectId;if(id){void get().loadProjectContent(id);void useThreadStore.getState().loadThreads(id)}}catch(error){set({loadingProjects:false,projectError:error instanceof Error?error.message:'项目加载失败，请重试'})}},
  openProject: (id) => { set((state)=>({openProjectIds:state.openProjectIds.includes(id)?state.openProjectIds:[...state.openProjectIds,id]}));get().selectProject(id) },
  createProject: async (topic) => { const project=await projectsApi.create(topic);set((state)=>({projects:[...state.projects,project]}));return project },
  closeProject: (id) => { if(isProjectExportBlocking(id))return get().activeProjectId;let nextActiveProjectId: string | null = null;advance(id);useThreadStore.getState().closeProjectThreads(id);set((state)=>{const openProjectIds=state.openProjectIds.filter((value)=>value!==id);nextActiveProjectId=state.activeProjectId===id?openProjectIds[openProjectIds.length-1]??null:state.activeProjectId;return{openProjectIds,activeProjectId:nextActiveProjectId}});return nextActiveProjectId },
  renameProject: async (id,title) => { await projectsApi.patch(id,{title});set((state)=>({projects:state.projects.map((project)=>project.id===id?{...project,title}:project)})) },
  deleteProject: async (id) => { await projectsApi.delete(id);advance(id);useThreadStore.getState().dropProject(id);set((state)=>{const contentByProjectId={...state.contentByProjectId};delete contentByProjectId[id];const openProjectIds=state.openProjectIds.filter((value)=>value!==id);return{projects:state.projects.filter((project)=>project.id!==id),openProjectIds,activeProjectId:state.activeProjectId===id?openProjectIds[openProjectIds.length-1]??null:state.activeProjectId,contentByProjectId}}) },
  setProjectTheme: async (projectId,themeId) => {
    const project = await projectsApi.setTheme(projectId,themeId);
    set((state) => ({ projects: state.projects.map((value) => value.id === projectId ? project : value) }));
    await get().loadProjectContent(projectId);
    return project;
  },
  selectProject: (id) => { const current=get().activeProjectId;if(current!==id&&isProjectExportBlocking(current))return;if(current!==id)set({activeProjectId:id});void get().loadProjectContent(id);void useThreadStore.getState().loadThreads(id) },
  loadProjectContent: async (projectId) => { const version=advance(projectId);set((state)=>({contentLoadingByProjectId:{...state.contentLoadingByProjectId,[projectId]:true},contentErrorByProjectId:{...state.contentErrorByProjectId,[projectId]:undefined}}));try{const snapshot=await projectsApi.getContent(projectId);if(requestVersions.get(projectId)!==version)return;const current=get().contentByProjectId[projectId];if(current&&sameContent(current,snapshot)){set((state)=>({contentLoadingByProjectId:{...state.contentLoadingByProjectId,[projectId]:false}}));return}get().applyProjectContentSnapshot(projectId,snapshot)}catch(error){if(requestVersions.get(projectId)!==version)return;const message=error instanceof APIError&&error.status===404?undefined:error instanceof Error?error.message:'页面加载失败，请重试';set((state)=>({contentLoadingByProjectId:{...state.contentLoadingByProjectId,[projectId]:false},contentErrorByProjectId:{...state.contentErrorByProjectId,[projectId]:message},projectError:message??null}))} },
  checkProjectContent: (projectId) => {
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
  mutateProject: async (projectId,mutation) => { if(get().mutationPendingByProjectId[projectId])throw new Error('结构操作正在进行');advance(projectId);set((state)=>({mutationPendingByProjectId:{...state.mutationPendingByProjectId,[projectId]:true}}));try{const response=await projectsApi.mutate(projectId,mutation);get().applyProjectContentSnapshot(projectId,response.content);return response.content}finally{set((state)=>({mutationPendingByProjectId:{...state.mutationPendingByProjectId,[projectId]:false}}))} },
  applyProjectContentSnapshot: (projectId,snapshot) => { advance(projectId);set((state)=>({contentByProjectId:{...state.contentByProjectId,[projectId]:snapshot},contentLoadingByProjectId:{...state.contentLoadingByProjectId,[projectId]:false},contentErrorByProjectId:{...state.contentErrorByProjectId,[projectId]:undefined},projectError:null})) },
}),{name:'ppt-agent-project-v7',partialize:(state)=>({openProjectIds:state.openProjectIds}),merge:(persisted,current)=>({...current,...persisted as Partial<ProjectState>,activeProjectId:null})}));
