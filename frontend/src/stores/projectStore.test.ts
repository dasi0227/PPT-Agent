import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { ProjectContentSnapshot } from '../api/types';

const { closeProjectThreads, dropProject, getContent, list, mutate, setTheme } = vi.hoisted(() => ({
  closeProjectThreads: vi.fn(),
  dropProject: vi.fn(),
  getContent: vi.fn(),
  list: vi.fn(),
  mutate: vi.fn(),
  setTheme: vi.fn(),
}));
vi.mock('../api/projects', () => ({ projectsApi: { getContent, list, mutate, setTheme } }));
vi.mock('./threadStore', () => ({ useThreadStore: { getState: () => ({ closeProjectThreads, loadThreads: vi.fn(), dropProject }) } }));

import { useProjectStore } from './projectStore';

function snapshot(revision: number): ProjectContentSnapshot {
  return {
    manifest: { version: '4.0', revision, project_id: 'pro_1', title: 'Deck', goal: '', audience: '', language: 'zh-CN', requirements: [], prohibitions: [], canvas: { aspect_ratio: '16:9' }, numbering: { enabled: true, hidden_roles: [], format: 'number' }, created_at: 1, updated_at: 1 },
    outline: { version: '4.0', revision, project_id: 'pro_1', sections: [], created_at: 1, updated_at: 1 },
    design: { version: '4.0', revision, project_id: 'pro_1', theme: 'default', direction: '', density: 'medium', chrome: [], created_at: 1, updated_at: 1 },
    slides_by_id: {},
  };
}

describe('projectStore canonical content snapshots', () => {
  beforeEach(() => {
    closeProjectThreads.mockReset(); dropProject.mockReset(); getContent.mockReset(); list.mockReset(); mutate.mockReset(); setTheme.mockReset();
    useProjectStore.setState({
      projects: [],
      openProjectIds: [],
      activeProjectId: null,
      contentByProjectId: {},
      contentLoadingByProjectId: {},
      contentErrorByProjectId: {},
      mutationPendingByProjectId: {},
      projectError: null,
    });
  });

  it('publishes deck, outline, design and pages in one store update', async () => {
    getContent.mockResolvedValue(snapshot(1));
    await useProjectStore.getState().loadProjectContent('pro_1');
    expect(useProjectStore.getState().contentByProjectId.pro_1).toEqual(snapshot(1));
  });

  it('applies a mutation response directly without a duplicate refresh', async () => {
    mutate.mockResolvedValue({ mutation: { operation: 'outline.insert' }, content: snapshot(2) });
    await useProjectStore.getState().mutateProject('pro_1', { op: 'outline.insert', node: { kind: 'section', client_ref: 'section', title: 'Section', purpose: 'Purpose', slides: [], subsections: [] }, position: {} });
    expect(mutate).toHaveBeenCalledTimes(1);
    expect(getContent).not.toHaveBeenCalled();
    expect(useProjectStore.getState().contentByProjectId.pro_1.outline.revision).toBe(2);
    expect(useProjectStore.getState().mutationPendingByProjectId.pro_1).toBe(false);
  });

  it('applies a theme response to project metadata and cached design state', async () => {
    const content = snapshot(1);
    setTheme.mockResolvedValue({
      id: 'pro_1', title: '项目', work_dir: '/projects/pro_1', theme: 'tokyo-night',
      status: 'ready', design_path: 'design.json', outline_path: 'outline.json',
      outline_revision: 1, design_revision: 2, created_at: 1, updated_at: 2,
    });
    useProjectStore.setState({
      projects: [{
        id: 'pro_1', title: '项目', work_dir: '/projects/pro_1', theme: 'swiss-modern',
        status: 'ready', design_path: 'design.json', outline_path: 'outline.json',
        outline_revision: 1, design_revision: 1, created_at: 1, updated_at: 1,
      }],
      contentByProjectId: { pro_1: content },
    });

    await useProjectStore.getState().setProjectTheme('pro_1', 'tokyo-night');

    expect(setTheme).toHaveBeenCalledWith('pro_1', 'tokyo-night');
    expect(useProjectStore.getState().projects[0]).toMatchObject({
      theme: 'tokyo-night',
      design_revision: 2,
    });
    expect(useProjectStore.getState().contentByProjectId.pro_1.design).toMatchObject({
      theme: 'tokyo-night',
      revision: 2,
      updated_at: 2,
    });
  });

  it('closes a project tab without dropping project-local threads or cached content', () => {
    useProjectStore.setState({
      openProjectIds: ['pro_1', 'pro_2'],
      activeProjectId: 'pro_1',
      contentByProjectId: { pro_1: snapshot(1), pro_2: snapshot(2) },
      contentLoadingByProjectId: { pro_1: true, pro_2: false },
      contentErrorByProjectId: { pro_1: '旧错误', pro_2: undefined },
      mutationPendingByProjectId: { pro_1: true, pro_2: false },
    });

    const nextActiveProjectId = useProjectStore.getState().closeProject('pro_1');

    expect(closeProjectThreads).toHaveBeenCalledWith('pro_1');
    expect(dropProject).not.toHaveBeenCalled();
    expect(nextActiveProjectId).toBe('pro_2');
    expect(useProjectStore.getState()).toMatchObject({
      openProjectIds: ['pro_2'],
      activeProjectId: 'pro_2',
      contentByProjectId: { pro_1: snapshot(1), pro_2: snapshot(2) },
      contentLoadingByProjectId: { pro_1: true, pro_2: false },
      contentErrorByProjectId: { pro_1: '旧错误', pro_2: undefined },
      mutationPendingByProjectId: { pro_1: true, pro_2: false },
    });
  });

  it('returns null after closing the last open project', () => {
    useProjectStore.setState({
      openProjectIds: ['pro_1'],
      activeProjectId: 'pro_1',
      contentByProjectId: { pro_1: snapshot(1) },
    });

    const nextActiveProjectId = useProjectStore.getState().closeProject('pro_1');

    expect(nextActiveProjectId).toBeNull();
    expect(closeProjectThreads).toHaveBeenCalledWith('pro_1');
    expect(dropProject).not.toHaveBeenCalled();
    expect(useProjectStore.getState()).toMatchObject({
      openProjectIds: [],
      activeProjectId: null,
      contentByProjectId: { pro_1: snapshot(1) },
    });
  });

  it('does not revive a closed active project when projects reload', async () => {
    list.mockResolvedValue([{ id: 'pro_1', title: '项目', created_at: 0, updated_at: 0 }]);
    useProjectStore.setState({
      openProjectIds: [],
      activeProjectId: 'pro_1',
    });

    await useProjectStore.getState().loadProjects();

    expect(useProjectStore.getState()).toMatchObject({
      openProjectIds: [],
      activeProjectId: null,
    });
  });

  it('does not activate a persisted project while loading the picker list', async () => {
    list.mockResolvedValue([{ id: 'pro_1', title: '项目', created_at: 0, updated_at: 0 }]);
    useProjectStore.setState({
      openProjectIds: ['pro_1'],
      activeProjectId: null,
    });

    await useProjectStore.getState().loadProjects();

    expect(useProjectStore.getState()).toMatchObject({
      openProjectIds: ['pro_1'],
      activeProjectId: null,
    });
    expect(getContent).not.toHaveBeenCalled();
  });
});
