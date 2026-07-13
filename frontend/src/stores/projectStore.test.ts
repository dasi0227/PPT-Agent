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

function reset() {
  calls.length = 0;
  useProjectStore.setState({ projects: [], openProjectIds: [], activeProjectId: null, pendingNewProject: false, slidesByProjectId: {}, loadingProjects: false });
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

  it('startPendingNewProject creates a sentinel id without POSTing', () => {
    useProjectStore.getState().startPendingNewProject();
    expect(calls.filter((c) => c.fn === 'create')).toHaveLength(0);
    const st = useProjectStore.getState();
    expect(st.pendingNewProject).toBe(true);
    expect(st.activeProjectId).toBe('new-pending');
    expect(st.openProjectIds).toContain('new-pending');
  });

  it('cancelPendingNewProject cleans up new-pending', () => {
    useProjectStore.getState().startPendingNewProject();
    useProjectStore.getState().cancelPendingNewProject();
    const st = useProjectStore.getState();
    expect(st.pendingNewProject).toBe(false);
    expect(st.openProjectIds).not.toContain('new-pending');
    expect(st.activeProjectId).toBeNull();
  });

  it('finalizePendingNewProject replaces new-pending with real id', () => {
    useProjectStore.getState().startPendingNewProject();
    useProjectStore.getState().finalizePendingNewProject('realP_1');
    const st = useProjectStore.getState();
    expect(st.pendingNewProject).toBe(false);
    expect(st.openProjectIds).toContain('realP_1');
    expect(st.openProjectIds).not.toContain('new-pending');
    expect(st.activeProjectId).toBe('realP_1');
  });

  it('renameProject PATCHes and updates in-place', async () => {
    useProjectStore.setState({
      projects: [{ id: 'A', title: 'Old', work_dir: '', theme: '', status: 'draft', design_path: '', created_at: 0, updated_at: 0 } as any]
    });
    await useProjectStore.getState().renameProject('A', 'New');
    expect(calls.find(c => c.fn === 'patch')).toBeTruthy();
    expect(useProjectStore.getState().projects[0].title).toBe('New');
  });

  it('deleteProject on new-pending cancels it', async () => {
    useProjectStore.getState().startPendingNewProject();
    await useProjectStore.getState().deleteProject('new-pending');
    expect(calls.filter((c) => c.fn === 'delete')).toHaveLength(0);
    expect(useProjectStore.getState().activeProjectId).toBeNull();
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
});
