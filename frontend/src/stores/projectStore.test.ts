import { beforeEach, describe, expect, it, vi } from 'vitest';

const calls: Array<{ fn: string; args: any[] }> = [];

vi.mock('../api/projects', () => ({
  projectsApi: {
    list: async () => { calls.push({ fn: 'list', args: [] }); return []; },
    create: async (topic: string, brief: string, slide_count: number, language: string) => {
      calls.push({ fn: 'create', args: [topic, brief, slide_count, language] });
      const n = calls.filter((c) => c.fn === 'create').length;
      return { id: `realP_${n}`, title: topic, work_dir: '', theme: 'swiss-modern', status: 'draft', design_path: '', created_at: 0, updated_at: 0 };
    },
    patch: async (id: string, patch: any) => { calls.push({ fn: 'patch', args: [id, patch] }); },
    get: async (id: string) => ({ id, title: '', theme: '', status: '', created_at: 0, updated_at: 0 }),
    getSlides: async (id: string) => { calls.push({ fn: 'getSlides', args: [id] }); return []; },
    delete: async (id: string) => { calls.push({ fn: 'delete', args: [id] }); },
  },
}));

import { useProjectStore } from './projectStore';
import { useThreadStore } from './threadStore';
import { useRunStore } from './runStore';
import { projectsApi } from '../api/projects';
import { specsApi } from '../api/specs';

function reset() {
  calls.length = 0;
  useProjectStore.setState({
    projects: [],
    openProjectIds: [],
    activeProjectId: null,
    slidesByProjectId: {},
    specByProjectId: {},
    contentLoadingByProjectId: {},
    contentErrorByProjectId: {},
    loadingProjects: false,
  });
  useThreadStore.setState({ threadsByProjectId: {}, openThreadIdsByProjectId: {}, activeThreadIdByProjectId: {} });
  useRunStore.setState({ sessions: {} });
}

describe('projectStore v6', () => {
  beforeEach(reset);

  it('loadProjects keeps openProjectIds valid', async () => {
    useProjectStore.setState({ openProjectIds: ['p1', 'missing'], activeProjectId: 'missing' });
    const originalList = projectsApi.list;
    projectsApi.list = async () => [
      { id: 'p1', title: 'P1', work_dir: '', theme: '', status: 'draft', design_path: '', created_at: 0, updated_at: 0 } as any
    ];
    await useProjectStore.getState().loadProjects();
    projectsApi.list = originalList;
    const st = useProjectStore.getState();
    expect(st.openProjectIds).toEqual(['p1']);
    expect(st.activeProjectId).toBe('p1');
  });

  it('createProject creates only a real project', async () => {
    const project = await useProjectStore.getState().createProject('发布会方案', '', 10, 'zh-CN');
    expect(calls.filter((c) => c.fn === 'create')).toEqual([
      { fn: 'create', args: ['发布会方案', '', 10, 'zh-CN'] },
    ]);
    expect(project.id).toBe('realP_1');
    expect(useProjectStore.getState().projects.map((item) => item.id)).toEqual(['realP_1']);
  });

  it('renameProject PATCHes and updates in-place', async () => {
    useProjectStore.setState({
      projects: [{ id: 'A', title: 'Old', work_dir: '', theme: '', status: 'draft', design_path: '', created_at: 0, updated_at: 0 } as any]
    });
    await useProjectStore.getState().renameProject('A', 'New');
    expect(calls.find(c => c.fn === 'patch')).toBeTruthy();
    expect(useProjectStore.getState().projects[0].title).toBe('New');
  });

  it('deleteProject DELETEs and cleans up, switching neighbor', async () => {
    useProjectStore.setState({
      projects: [
        { id: 'A', title: 'A', work_dir: '', theme: '', status: 'draft', design_path: '', created_at: 0, updated_at: 0 } as any,
        { id: 'B', title: 'B', work_dir: '', theme: '', status: 'draft', design_path: '', created_at: 0, updated_at: 0 } as any,
      ],
      openProjectIds: ['A', 'B'],
      activeProjectId: 'B',
      slidesByProjectId: { B: [] },
    });
    await useProjectStore.getState().deleteProject('B');
    expect(calls.filter((c) => c.fn === 'delete')).toHaveLength(1);
    const st = useProjectStore.getState();
    expect(st.projects.map((p) => p.id)).toEqual(['A']);
    expect(st.openProjectIds).toEqual(['A']);
    expect(st.activeProjectId).toBe('A');
    expect(st.slidesByProjectId['B']).toBeUndefined();
  });

  it('publishes slides and spec only after the complete project snapshot is ready', async () => {
    const oldSlide = { id: 'old' } as any;
    const newSlide = { id: 'new' } as any;
    const oldSpec = { outline: { revision: 1 } } as any;
    const newSpec = { outline: { revision: 2 } } as any;
    useProjectStore.setState({ slidesByProjectId: { p1: [oldSlide] }, specByProjectId: { p1: oldSpec } });

    const originalGetSlides = projectsApi.getSlides;
    const originalGetSpec = specsApi.getProject;
    let resolveSpec!: (value: typeof newSpec) => void;
    projectsApi.getSlides = async () => [newSlide];
    specsApi.getProject = () => new Promise((resolve) => { resolveSpec = resolve; });

    const loading = useProjectStore.getState().loadProjectContent('p1');
    await Promise.resolve();
    expect(useProjectStore.getState().slidesByProjectId.p1).toEqual([oldSlide]);
    expect(useProjectStore.getState().specByProjectId.p1).toBe(oldSpec);

    resolveSpec(newSpec);
    await loading;
    expect(useProjectStore.getState().slidesByProjectId.p1).toEqual([newSlide]);
    expect(useProjectStore.getState().specByProjectId.p1).toBe(newSpec);

    projectsApi.getSlides = originalGetSlides;
    specsApi.getProject = originalGetSpec;
  });

  it('does not let an older content request overwrite a mutation snapshot', async () => {
    const originalGetSlides = projectsApi.getSlides;
    const originalGetSpec = specsApi.getProject;
    let resolveSpec!: (value: any) => void;
    projectsApi.getSlides = async () => [{ id: 'stale-slide' } as any];
    specsApi.getProject = () => new Promise((resolve) => { resolveSpec = resolve; });

    const staleLoad = useProjectStore.getState().loadProjectContent('p1');
    await Promise.resolve();
    useProjectStore.getState().applyProjectContentSnapshot('p1', {
      slides: [{ id: 'authoritative-slide' } as any],
      spec: { outline: { revision: 3 } } as any,
    });
    resolveSpec({ outline: { revision: 2 } });
    await staleLoad;

    expect(useProjectStore.getState().slidesByProjectId.p1[0].id).toBe('authoritative-slide');
    expect(useProjectStore.getState().specByProjectId.p1.outline.revision).toBe(3);

    projectsApi.getSlides = originalGetSlides;
    specsApi.getProject = originalGetSpec;
  });
});
