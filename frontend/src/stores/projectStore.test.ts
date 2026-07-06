import { beforeEach, describe, expect, it, vi } from 'vitest';

const calls: Array<{ fn: string; args: any[] }> = [];

vi.mock('../api/projects', () => ({
  projectsApi: {
    list: async () => { calls.push({ fn: 'list', args: [] }); return []; },
    create: async (topic: string, brief: string, slide_count: number, language: string) => {
      calls.push({ fn: 'create', args: [topic, brief, slide_count, language] });
      const n = calls.filter((c) => c.fn === 'create').length;
      return { id: `realP_${n}`, title: topic, theme: 'swiss-modern', status: 'draft', created_at: 0, updated_at: 0 };
    },
    get: async (id: string) => ({ id, title: '', theme: '', status: '', created_at: 0, updated_at: 0 }),
    getSlides: async (id: string) => { calls.push({ fn: 'getSlides', args: [id] }); return []; },
    delete: async (id: string) => { calls.push({ fn: 'delete', args: [id] }); },
  },
}));

import { useProjectStore } from './projectStore';
import { useThreadStore } from './threadStore';
import { useRunStore } from './runStore';
import { isDraftId } from '../lib/draft';

function reset() {
  calls.length = 0;
  useProjectStore.setState({ projects: [], activeProjectId: null, slidesByProjectId: {}, loadingProjects: false });
  useThreadStore.setState({ threadsByProjectId: {}, draftThreadsByProjectId: {}, openThreadIdsByProjectId: {}, activeThreadIdByProjectId: {} });
  useRunStore.setState({ sessions: {} });
}

describe('projectStore soft-create', () => {
  beforeEach(reset);

  it('createDraftProject makes a front-only draft, selected, no POST', () => {
    const id = useProjectStore.getState().createDraftProject();
    expect(isDraftId(id)).toBe(true);
    expect(calls.filter((c) => c.fn === 'create')).toHaveLength(0);
    expect(calls.filter((c) => c.fn === 'getSlides')).toHaveLength(0); // 草稿不拉数据
    const st = useProjectStore.getState();
    expect(st.projects.find((p) => p.id === id)?.draft).toBe(true);
    expect(st.activeProjectId).toBe(id);
  });

  it('switching away from an unused draft discards it (zero residue)', () => {
    useProjectStore.setState({ projects: [{ id: 'realP', title: 'Real', theme: '', status: '', created_at: 0, updated_at: 0 }] });
    const draftId = useProjectStore.getState().createDraftProject();
    useProjectStore.getState().selectProject('realP');
    const st = useProjectStore.getState();
    expect(st.projects.find((p) => p.id === draftId)).toBeUndefined();
    expect(st.activeProjectId).toBe('realP');
    expect(calls.filter((c) => c.fn === 'create')).toHaveLength(0);
  });

  it('flushProject POSTs once and rebinds draft id → real id', async () => {
    const store = useProjectStore.getState();
    const tmp = store.createDraftProject();
    // 该草稿下先建一个草稿 thread。
    useThreadStore.getState().createDraftThread(tmp, 'X');

    const realId = await store.flushProject(tmp, { topic: '主题', slide_count: 8, language: 'zh' });
    expect(isDraftId(realId)).toBe(false);
    expect(calls.filter((c) => c.fn === 'create')).toHaveLength(1);
    expect(calls.find((c) => c.fn === 'create')!.args).toEqual(['主题', '', 8, 'zh']);

    const st = useProjectStore.getState();
    expect(st.projects.find((p) => p.id === tmp)).toBeUndefined();
    expect(st.projects.find((p) => p.id === realId)?.draft).toBeFalsy();
    expect(st.activeProjectId).toBe(realId);
    // thread 归属键改绑。
    expect(useThreadStore.getState().draftThreadsByProjectId[realId]).toHaveLength(1);
    expect(useThreadStore.getState().draftThreadsByProjectId[tmp]).toBeUndefined();
  });

  it('flushProject dedupes concurrent calls', async () => {
    const store = useProjectStore.getState();
    const tmp = store.createDraftProject();
    const [a, b] = await Promise.all([
      store.flushProject(tmp, { topic: 'T' }),
      store.flushProject(tmp, { topic: 'T' }),
    ]);
    expect(a).toBe(b);
    expect(calls.filter((c) => c.fn === 'create')).toHaveLength(1);
  });

  it('deleteProject on draft discards without DELETE', async () => {
    const store = useProjectStore.getState();
    const tmp = store.createDraftProject();
    await store.deleteProject(tmp);
    expect(calls.filter((c) => c.fn === 'delete')).toHaveLength(0);
    expect(useProjectStore.getState().projects.find((p) => p.id === tmp)).toBeUndefined();
  });

  it('deleteProject on real project DELETEs and cleans up, switching neighbor', async () => {
    useProjectStore.setState({
      projects: [
        { id: 'A', title: 'A', theme: '', status: '', created_at: 0, updated_at: 0 },
        { id: 'B', title: 'B', theme: '', status: '', created_at: 0, updated_at: 0 },
      ],
      activeProjectId: 'B',
      slidesByProjectId: { B: [] },
    });
    await useProjectStore.getState().deleteProject('B');
    expect(calls.filter((c) => c.fn === 'delete')).toHaveLength(1);
    const st = useProjectStore.getState();
    expect(st.projects.map((p) => p.id)).toEqual(['A']);
    expect(st.activeProjectId).toBe('A');
    expect(st.slidesByProjectId['B']).toBeUndefined();
  });
});
