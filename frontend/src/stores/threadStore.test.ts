import { beforeEach, describe, expect, it, vi } from 'vitest';

// 记录所有 threadsApi 调用，验证「未使用不落库」。
const calls: Array<{ fn: string; args: any[] }> = [];

vi.mock('../api/threads', () => ({
  threadsApi: {
    list: async (projectId: string) => { calls.push({ fn: 'list', args: [projectId] }); return []; },
    create: async (projectId: string, title?: string) => {
      calls.push({ fn: 'create', args: [projectId, title] });
      return { id: `real_${calls.filter((c) => c.fn === 'create').length}`, project_id: projectId, title: title ?? '', created_at: 0, updated_at: 0 };
    },
    delete: async (threadId: string) => { calls.push({ fn: 'delete', args: [threadId] }); },
    history: async () => [],
  },
}));

import { useThreadStore } from './threadStore';
import { useRunStore } from './runStore';
import { isDraftId } from '../lib/draft';

function reset() {
  calls.length = 0;
  useThreadStore.setState({
    threadsByProjectId: {},
    draftThreadsByProjectId: {},
    openThreadIdsByProjectId: {},
    activeThreadIdByProjectId: {},
  });
  useRunStore.setState({ sessions: {} });
}

describe('threadStore soft-create', () => {
  beforeEach(reset);

  it('createDraftThread makes a front-only draft without any request', () => {
    const id = useThreadStore.getState().createDraftThread('p1', '新对话');
    expect(isDraftId(id)).toBe(true);
    expect(calls).toHaveLength(0);
    const st = useThreadStore.getState();
    expect(st.draftThreadsByProjectId['p1']).toHaveLength(1);
    expect(st.openThreadIdsByProjectId['p1']).toEqual([id]);
    expect(st.activeThreadIdByProjectId['p1']).toBe(id);
  });

  it('closeThread discards a draft (no request, zero residue)', () => {
    const id = useThreadStore.getState().createDraftThread('p1');
    useThreadStore.getState().closeThread('p1', id);
    expect(calls).toHaveLength(0);
    const st = useThreadStore.getState();
    expect(st.draftThreadsByProjectId['p1']).toHaveLength(0);
    expect(st.openThreadIdsByProjectId['p1']).toEqual([]);
    expect(st.activeThreadIdByProjectId['p1']).toBeNull();
  });

  it('closeThread on real thread only removes from open (keeps backend)', () => {
    useThreadStore.setState({
      threadsByProjectId: { p1: [{ id: 't1', project_id: 'p1', created_at: 0, updated_at: 0 }] },
      openThreadIdsByProjectId: { p1: ['t1'] },
      activeThreadIdByProjectId: { p1: 't1' },
    });
    useThreadStore.getState().closeThread('p1', 't1');
    expect(calls.filter((c) => c.fn === 'delete')).toHaveLength(0);
    expect(useThreadStore.getState().threadsByProjectId['p1']).toHaveLength(1); // 后端仍在
    expect(useThreadStore.getState().openThreadIdsByProjectId['p1']).toEqual([]);
  });

  it('flushThread POSTs once and rebinds draft id → real id everywhere', async () => {
    const store = useThreadStore.getState();
    const tmp = store.createDraftThread('p1', 'X');
    // 模拟草稿 thread 已有 run 分片。
    useRunStore.setState({ sessions: { [tmp]: { ...useRunStore.getState().getSession(tmp), status: 'running', timelineItems: [{ id: 'a', type: 'thought', text: 'hi', timestamp: 0 }] } } });

    const realId = await store.flushThread('p1', tmp);
    expect(isDraftId(realId)).toBe(false);
    expect(calls.filter((c) => c.fn === 'create')).toHaveLength(1);

    const st = useThreadStore.getState();
    expect(st.draftThreadsByProjectId['p1']).toHaveLength(0);
    expect(st.threadsByProjectId['p1'].map((t) => t.id)).toContain(realId);
    expect(st.openThreadIdsByProjectId['p1']).toEqual([realId]);
    expect(st.activeThreadIdByProjectId['p1']).toBe(realId);
    // session 已改绑到 realId。
    expect(useRunStore.getState().sessions[tmp]).toBeUndefined();
    expect(useRunStore.getState().sessions[realId].timelineItems).toHaveLength(1);
  });

  it('flushThread is idempotent for a non-draft id', async () => {
    const out = await useThreadStore.getState().flushThread('p1', 't-real');
    expect(out).toBe('t-real');
    expect(calls.filter((c) => c.fn === 'create')).toHaveLength(0);
  });

  it('flushThread dedupes concurrent calls into a single POST', async () => {
    const store = useThreadStore.getState();
    const tmp = store.createDraftThread('p1');
    const [a, b] = await Promise.all([store.flushThread('p1', tmp), store.flushThread('p1', tmp)]);
    expect(a).toBe(b);
    expect(calls.filter((c) => c.fn === 'create')).toHaveLength(1);
  });

  it('ensureActiveThread flushes a draft active thread', async () => {
    const store = useThreadStore.getState();
    const tmp = store.createDraftThread('p1');
    const real = await store.ensureActiveThread('p1');
    expect(isDraftId(real)).toBe(false);
    expect(real).not.toBe(tmp);
    expect(calls.filter((c) => c.fn === 'create')).toHaveLength(1);
  });

  it('ensureActiveThread reuses first backend thread when present', async () => {
    useThreadStore.setState({
      threadsByProjectId: { p1: [{ id: 't1', project_id: 'p1', created_at: 0, updated_at: 0 }] },
      activeThreadIdByProjectId: { p1: null },
    });
    const id = await useThreadStore.getState().ensureActiveThread('p1');
    expect(id).toBe('t1');
    expect(calls.filter((c) => c.fn === 'create')).toHaveLength(0);
  });

  it('ensureActiveThread creates a draft then flushes when nothing exists', async () => {
    const id = await useThreadStore.getState().ensureActiveThread('p1');
    expect(isDraftId(id)).toBe(false);
    expect(calls.filter((c) => c.fn === 'create')).toHaveLength(1);
  });

  it('loadThreads skips backend fetch for a draft project id', async () => {
    await useThreadStore.getState().loadThreads('draft_p');
    expect(calls.filter((c) => c.fn === 'list')).toHaveLength(0);
  });

  it('rekeyProject moves all thread keys from old project id to new', () => {
    const store = useThreadStore.getState();
    store.createDraftThread('draft_p', 'X');
    store.rekeyProject('draft_p', 'realP');
    const st = useThreadStore.getState();
    expect(st.draftThreadsByProjectId['draft_p']).toBeUndefined();
    expect(st.draftThreadsByProjectId['realP']).toHaveLength(1);
    expect(st.draftThreadsByProjectId['realP'][0].project_id).toBe('realP');
    expect(st.activeThreadIdByProjectId['realP']).toBeDefined();
  });
});

describe('threadStore.nextUntitledName', () => {
  beforeEach(reset);

  it('empty project returns 未命名 1', () => {
    expect(useThreadStore.getState().nextUntitledName('p1')).toBe('未命名 1');
  });

  it('is monotonic across existing untitled threads (real + draft)', () => {
    useThreadStore.setState({
      threadsByProjectId: {
        p1: [
          { id: 't1', project_id: 'p1', title: '未命名 1', created_at: 0, updated_at: 0 },
          { id: 't2', project_id: 'p1', title: '未命名 3', created_at: 0, updated_at: 0 },
        ],
      },
      draftThreadsByProjectId: {
        p1: [
          { id: 'draft_1', project_id: 'p1', title: '未命名 2', created_at: 0, updated_at: 0, draft: true },
        ],
      },
    });
    expect(useThreadStore.getState().nextUntitledName('p1')).toBe('未命名 4');
  });

  it('ignores non-matching titles (e.g. legacy 主线程)', () => {
    useThreadStore.setState({
      threadsByProjectId: {
        p1: [
          { id: 't1', project_id: 'p1', title: '主线程', created_at: 0, updated_at: 0 },
          { id: 't2', project_id: 'p1', title: 'foo bar', created_at: 0, updated_at: 0 },
        ],
      },
    });
    expect(useThreadStore.getState().nextUntitledName('p1')).toBe('未命名 1');
  });

  it('ensureActiveThread uses 未命名 1 for empty project', async () => {
    const id = await useThreadStore.getState().ensureActiveThread('p1');
    expect(isDraftId(id)).toBe(false);
    const created = calls.find((c) => c.fn === 'create');
    expect(created?.args[1]).toBe('未命名 1');
  });
});
