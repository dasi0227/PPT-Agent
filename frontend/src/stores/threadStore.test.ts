import { beforeEach, describe, expect, it, vi } from 'vitest';

const calls: Array<{ fn: string; args: any[] }> = [];

vi.mock('../api/threads', () => ({
  threadsApi: {
    list: async (projectId: string) => { calls.push({ fn: 'list', args: [projectId] }); return []; },
    create: async (projectId: string, title?: string) => {
      calls.push({ fn: 'create', args: [projectId, title] });
      const n = calls.filter((c) => c.fn === 'create').length;
      return { id: `realT_${n}`, project_id: projectId, title: title || '', history_path: '', status: 'active', created_at: 0, updated_at: 0 };
    },
    patch: async (id: string, patch: any) => { calls.push({ fn: 'patch', args: [id, patch] }); },
    delete: async (id: string) => { calls.push({ fn: 'delete', args: [id] }); },
    history: async () => [],
  },
}));

import { useThreadStore } from './threadStore';
import { useRunStore } from './runStore';

function reset() {
  calls.length = 0;
  useThreadStore.setState({ threadsByProjectId: {}, openThreadIdsByProjectId: {}, activeThreadIdByProjectId: {} });
  useRunStore.setState({ sessions: {} });
}

describe('threadStore v6', () => {
  beforeEach(reset);

  it('createThread makes a real thread immediately', async () => {
    const id = await useThreadStore.getState().createThread('p1', 'X');
    expect(calls.filter((c) => c.fn === 'create')).toHaveLength(1);
    const st = useThreadStore.getState();
    expect(st.threadsByProjectId['p1']).toHaveLength(1);
    expect(st.activeThreadIdByProjectId['p1']).toBe(id);
    expect(st.openThreadIdsByProjectId['p1']).toContain(id);
  });

  it('ensureActiveThread uses existing active if present', async () => {
    useThreadStore.setState({
      activeThreadIdByProjectId: { p1: 't1' },
      threadsByProjectId: { p1: [{ id: 't1', project_id: 'p1', title: 'T1', history_path: '', status: 'active', created_at: 0, updated_at: 0 }] }
    });
    const id = await useThreadStore.getState().ensureActiveThread('p1');
    expect(id).toBe('t1');
    expect(calls.filter((c) => c.fn === 'create')).toHaveLength(0);
  });

  it('ensureActiveThread uses first existing thread if no active', async () => {
    useThreadStore.setState({
      threadsByProjectId: { p1: [{ id: 't2', project_id: 'p1', title: 'T2', history_path: '', status: 'active', created_at: 0, updated_at: 0 }] }
    });
    const id = await useThreadStore.getState().ensureActiveThread('p1');
    expect(id).toBe('t2');
    expect(useThreadStore.getState().activeThreadIdByProjectId['p1']).toBe('t2');
  });

  it('ensureActiveThread creates new if none exists', async () => {
    const id = await useThreadStore.getState().ensureActiveThread('p1');
    expect(calls.filter((c) => c.fn === 'create')).toHaveLength(1);
    expect(id).toContain('realT_');
  });

  it('renameThread PATCHes and updates in-place', async () => {
    useThreadStore.setState({
      threadsByProjectId: { p1: [{ id: 't1', project_id: 'p1', title: 'Old', history_path: '', status: 'active', created_at: 0, updated_at: 0 }] }
    });
    await useThreadStore.getState().renameThread('p1', 't1', 'New');
    expect(calls.find(c => c.fn === 'patch')).toBeTruthy();
    expect(useThreadStore.getState().threadsByProjectId['p1'][0].title).toBe('New');
  });

  it('deleteThread DELETEs and cleans up, switching neighbor', async () => {
    useThreadStore.setState({
      threadsByProjectId: {
        p1: [
          { id: 't1', project_id: 'p1', title: '', history_path: '', status: '', created_at: 0, updated_at: 0 },
          { id: 't2', project_id: 'p1', title: '', history_path: '', status: '', created_at: 0, updated_at: 0 },
        ],
      },
      openThreadIdsByProjectId: { p1: ['t1', 't2'] },
      activeThreadIdByProjectId: { p1: 't2' },
    });
    useRunStore.setState({
      sessions: { t2: { activeRunId: null, status: 'idle', target: { artifact: 'presentation', level: 'slide' }, interaction: { intent: 'apply', clarification: 'when_blocked' }, timelineItems: [], pendingInput: null, progress: null, eventSourceClose: null, plan: null } },
    });

    await useThreadStore.getState().deleteThread('p1', 't2');
    expect(calls.filter((c) => c.fn === 'delete')).toHaveLength(1);

    const st = useThreadStore.getState();
    expect(st.openThreadIdsByProjectId['p1']).toEqual(['t1']);
    expect(st.activeThreadIdByProjectId['p1']).toBe('t1');
    expect(st.threadsByProjectId['p1']).toHaveLength(1);
    expect(useRunStore.getState().sessions['t2']).toBeUndefined();
  });
});
