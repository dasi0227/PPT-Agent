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
    theme: 'default',
    appearance: null,
    hashes: { outline: `outline-${revision}` },
    manifest: { version: '5.0', project_id: 'pro_1', title: 'Deck', goal: '', audience: '', language: 'zh-CN', requirements: [], prohibitions: [], created_at: 1, updated_at: 1 },
    outline: { version: '5.0', project_id: 'pro_1', sections: [], created_at: 1, updated_at: 1 },
    design: { version: '5.0', project_id: 'pro_1', direction: '', layout_preferences: [], decorations: { page_number: 'bottom-right', deck_title: 'none', section_title: 'none', key_message: 'none' }, created_at: 1, updated_at: 1 },
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

  it('refreshes appearance even when project content hashes are unchanged', async () => {
    const original = { ...snapshot(1), appearance: { hash: 'old', theme_css_url: '/theme.css?v=old', decoration_tokens: {} } };
    const changed = { ...original, appearance: { ...original.appearance, hash: 'new', theme_css_url: '/theme.css?v=new' } };
    useProjectStore.setState({ contentByProjectId: { pro_1: original } });
    getContent.mockResolvedValue(changed);
    await useProjectStore.getState().checkProjectContent('pro_1');
    expect(useProjectStore.getState().contentByProjectId.pro_1.appearance?.hash).toBe('new');
  });

  it('checks content silently and keeps the same snapshot when nothing changed', async () => {
    const original = snapshot(1);
    useProjectStore.setState({ contentByProjectId: { pro_1: original } });
    getContent.mockResolvedValue(snapshot(1));

    await Promise.all([
      useProjectStore.getState().checkProjectContent('pro_1'),
      useProjectStore.getState().checkProjectContent('pro_1'),
    ]);

    expect(useProjectStore.getState().contentByProjectId.pro_1).toBe(original);
    expect(useProjectStore.getState().contentLoadingByProjectId.pro_1).toBeUndefined();
    expect(getContent).toHaveBeenCalledTimes(1);
    getContent.mockResolvedValue(snapshot(2));
    await useProjectStore.getState().checkProjectContent('pro_1');
    expect(useProjectStore.getState().contentByProjectId.pro_1.hashes.outline).toBe('outline-2');
  });

  it('does not let an older background check overwrite a local mutation', async () => {
    let finishCheck!: (value: ProjectContentSnapshot) => void;
    getContent.mockReturnValue(new Promise<ProjectContentSnapshot>((resolve) => { finishCheck = resolve; }));
    mutate.mockResolvedValue({ mutation: { operation: 'outline.insert' }, content: snapshot(3) });

    const check = useProjectStore.getState().checkProjectContent('pro_1');
    await useProjectStore.getState().mutateProject('pro_1', { op: 'outline.insert', node: { kind: 'section', client_ref: 'section', title: 'Section', purpose: 'Purpose', slides: [], subsections: [] }, position: {} });
    finishCheck(snapshot(2));
    await check;

    expect(useProjectStore.getState().contentByProjectId.pro_1.hashes.outline).toBe('outline-3');
  });

  it('applies a mutation response directly without a duplicate refresh', async () => {
    mutate.mockResolvedValue({ mutation: { operation: 'outline.insert' }, content: snapshot(2) });
    await useProjectStore.getState().mutateProject('pro_1', { op: 'outline.insert', node: { kind: 'section', client_ref: 'section', title: 'Section', purpose: 'Purpose', slides: [], subsections: [] }, position: {} });
    expect(mutate).toHaveBeenCalledTimes(1);
    expect(getContent).not.toHaveBeenCalled();
    expect(useProjectStore.getState().contentByProjectId.pro_1.hashes.outline).toBe('outline-2');
    expect(useProjectStore.getState().mutationPendingByProjectId.pro_1).toBe(false);
  });

  it('applies a theme response to project metadata and runtime snapshot', async () => {
    const content = snapshot(1);
    setTheme.mockResolvedValue({
      id: 'pro_1', title: '项目', work_dir: '/projects/pro_1', theme: 'tokyo-night',
      status: 'ready', design_path: 'design.json', outline_path: 'outline.json',
      created_at: 1, updated_at: 2,
    });
    useProjectStore.setState({
      projects: [{
        id: 'pro_1', title: '项目', work_dir: '/projects/pro_1', theme: 'swiss-modern',
        status: 'ready', design_path: 'design.json', outline_path: 'outline.json',
        created_at: 1, updated_at: 1,
      }],
      contentByProjectId: { pro_1: content },
    });

    getContent.mockResolvedValue({ ...content, theme: 'tokyo-night' });
    await useProjectStore.getState().setProjectTheme('pro_1', 'tokyo-night');

    expect(setTheme).toHaveBeenCalledWith('pro_1', 'tokyo-night');
    expect(useProjectStore.getState().projects[0]).toMatchObject({
      theme: 'tokyo-night',
      });
    expect(useProjectStore.getState().contentByProjectId.pro_1.theme).toBe('tokyo-night');
    expect(useProjectStore.getState().contentByProjectId.pro_1.design).toEqual(content.design);
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
