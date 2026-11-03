import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { APIError } from '../api/client';
import { sessionApi, type ItemPage, type PublicObject, type SessionBatch, type TimelinePage } from '../api/session';
import { groupFixture, itemFixture, threadFixture, viewFixture } from '../test/sessionFixtures';
import { useSessionStore } from './sessionStore';

class Stream extends EventTarget {
  static opened: Stream[] = [];
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  close = vi.fn();
  constructor(readonly url: string) { super(); Stream.opened.push(this); }
  batch(seq: string, changes: SessionBatch['changes']) {
    this.dispatchEvent(new MessageEvent('thread.change', { lastEventId: `cursor${seq}`, data: JSON.stringify({
      project_id: 'p1', thread_id: 't1', scene_revision: '1', seq, commit_id: `commit${seq}`, changes,
    }) }));
  }
}
const store = () => useSessionStore.getState();
beforeEach(() => { Stream.opened = []; vi.stubGlobal('EventSource', Stream); });
afterEach(() => { store().drop(Object.keys(store().sessions)); vi.restoreAllMocks(); vi.unstubAllGlobals(); });

describe('shared snapshot and incremental synchronization', () => {
  it('shares initial requests and displays the snapshot before SSE opens', async () => {
    const read = vi.spyOn(sessionApi, 'view').mockResolvedValue(viewFixture());
    const release1 = store().retain('t1', 'p1'), release2 = store().retain('t1', 'p1');
    await store().ensure('t1', 'p1');
    expect(read).toHaveBeenCalledOnce();
    expect(Stream.opened).toHaveLength(1);
    expect(store().sessions.t1).toMatchObject({ loadStatus: 'ready', connectionStatus: 'connecting', isEmpty: true });
    release1();
    expect(Stream.opened[0].close).not.toHaveBeenCalled();
    release2();
  });

  it('does not treat a failed snapshot as an empty conversation', async () => {
    vi.spyOn(sessionApi, 'view').mockRejectedValue(new Error('offline'));
    await store().ensure('t1', 'p1');
    expect(store().sessions.t1).toMatchObject({ loadStatus: 'error', isEmpty: false, appliedSeq: null });
  });

  it('turns a persistence reset into a visible session error with the safe view message', async () => {
    const error = new APIError(500, 'PERSISTENCE_FAILED', '命令结果未能保存，请恢复存储后重启服务确认结果。');
    const read = vi.spyOn(sessionApi, 'view').mockResolvedValueOnce(viewFixture()).mockRejectedValueOnce(error);
    store().retain('t1', 'p1'); await store().ensure('t1', 'p1');
    Stream.opened[0].dispatchEvent(new MessageEvent('thread.reset', { data: JSON.stringify({ reason: 'PERSISTENCE_FAILED' }) }));
    await vi.waitFor(() => expect(store().sessions.t1.loadStatus).toBe('error'));
    expect(read).toHaveBeenCalledTimes(2);
    expect(store().sessions.t1).toMatchObject({ connectionStatus: 'closed', error });
    expect(Stream.opened).toHaveLength(1);
  });

  it('ignores duplicate batches and fills a sequence gap before advancing', async () => {
    vi.spyOn(sessionApi, 'view').mockResolvedValue(viewFixture());
    const changes = vi.spyOn(sessionApi, 'changes').mockResolvedValue({
      batches: ['2', '3'].map(seq => ({ project_id: 'p1', thread_id: 't1', scene_revision: '1', seq, commit_id: `c${seq}`, changes: [] })),
      cursor: 'cursor3', has_more: false,
    });
    store().retain('t1', 'p1'); await store().ensure('t1', 'p1');
    Stream.opened[0].batch('1', []);
    expect(store().sessions.t1.cursor).toBe('cursor1');
    Stream.opened[0].batch('3', []);
    await vi.waitFor(() => expect(store().sessions.t1.appliedSeq).toBe('3'));
    expect(changes).toHaveBeenCalledWith('t1', 'cursor1', expect.any(AbortSignal));
  });

  it('keeps live versions and tombstones when an older item page arrives', async () => {
    const group = groupFixture({ items_mode: 'summary', has_more_items: true });
    const old = itemFixture({ id: 'tool1', kind: 'tool', summary: { tool: 'read_resource', status: 'running' } });
    vi.spyOn(sessionApi, 'view').mockResolvedValue(viewFixture({ timeline: {
      scene_revision: '1', read_seq: '1', groups: [group], items: [old], has_more: false, older_cursor: null, is_empty: false,
    } }));
    let finish!: (page: ItemPage) => void;
    const items = vi.spyOn(sessionApi, 'items').mockReturnValue(new Promise(resolve => { finish = resolve; }));
    store().retain('t1', 'p1'); await store().ensure('t1', 'p1');
    const page = store().loadGroup('t1', group.id);
    await vi.waitFor(() => expect(items).toHaveBeenCalledOnce());
    const newer = { ...old, entity_revision: '2', updated_seq: '2', summary: { tool: 'read_resource', status: 'completed' } };
    Stream.opened[0].batch('2', [{ type: 'timeline.item.upsert', entity: newer },
      { type: 'timeline.item.remove', id: 'removed', entity_revision: '2' }]);
    finish({ scene_revision: '1', read_seq: '1', group_id: group.id, items: [old, { ...old, id: 'removed' }], older_cursor: null, has_more: false });
    await page;
    expect(store().sessions.t1.itemsById.tool1.summary.status).toBe('completed');
    expect(store().sessions.t1.itemsById.removed).toBeUndefined();
    expect(store().sessions.t1).toMatchObject({ appliedSeq: '2', cursor: 'cursor2' });
  });

  it('merges historical groups without moving the live cursor or duplicating current groups', async () => {
    const current = groupFixture({ id: 'current', order_seq: '2' });
    const older = groupFixture({ id: 'older', order_seq: '1' });
    vi.spyOn(sessionApi, 'view').mockResolvedValue(viewFixture({ timeline: {
      scene_revision: '1', read_seq: '1', groups: [current], items: [],
      has_more: true, older_cursor: 'history1', is_empty: false,
    } }));
    let finish!: (page: TimelinePage) => void;
    const timeline = vi.spyOn(sessionApi, 'timeline').mockReturnValue(new Promise(resolve => { finish = resolve; }));
    store().retain('t1', 'p1'); await store().ensure('t1', 'p1');
    const page = store().loadOlder('t1');
    await vi.waitFor(() => expect(timeline).toHaveBeenCalledOnce());
    Stream.opened[0].batch('2', [{ type: 'timeline.group.upsert', entity: {
      ...current, entity_revision: '2', updated_seq: '2', status: 'done',
    } }]);
    finish({ scene_revision: '1', read_seq: '1', groups: [older, current], items: [],
      older_cursor: null, has_more: false, is_empty: false });
    await page;
    expect(timeline).toHaveBeenCalledWith('t1', '1', 'history1', expect.any(AbortSignal));
    expect(store().sessions.t1).toMatchObject({
      orderedGroupIds: ['older', 'current'], appliedSeq: '2', cursor: 'cursor2', hasMore: false,
    });
    expect(store().sessions.t1.groupsById.current.status).toBe('done');
    await store().loadOlder('t1');
    expect(timeline).toHaveBeenCalledOnce();
  });

  it('discards a delayed detail body after its immutable reference changes', async () => {
    const item = itemFixture({ id: 'tool1', kind: 'tool', detail_ref: 'detail1', summary: { tool: 'read_resource', status: 'running' } });
    vi.spyOn(sessionApi, 'view').mockResolvedValue(viewFixture({ timeline: {
      scene_revision: '1', read_seq: '1', groups: [groupFixture()], items: [item],
      has_more: false, older_cursor: null, is_empty: false,
    } }));
    let finish!: (body: PublicObject) => void;
    const detail = vi.spyOn(sessionApi, 'detail')
      .mockReturnValueOnce(new Promise(resolve => { finish = resolve; }))
      .mockResolvedValueOnce({ text: 'current detail' });
    store().retain('t1', 'p1'); await store().ensure('t1', 'p1');
    const pending = store().loadDetail('t1', item.id);
    await vi.waitFor(() => expect(detail).toHaveBeenCalledOnce());
    Stream.opened[0].batch('2', [{ type: 'timeline.item.upsert', entity: {
      ...item, entity_revision: '2', updated_seq: '2', detail_ref: 'detail2',
    } }]);
    finish({ text: 'expired detail' });
    await pending;
    expect(store().sessions.t1.detailsByRef).toEqual({});
    await store().loadDetail('t1', item.id);
    expect(detail).toHaveBeenLastCalledWith('t1', item.id, 'detail2', expect.any(AbortSignal));
    expect(store().sessions.t1.detailsByRef).toEqual({ detail2: { text: 'current detail' } });
  });

  it('isolates a new scene from pending history and detail responses of the old scene', async () => {
    const item = itemFixture({ body_ref: 'body1' });
    const read = vi.spyOn(sessionApi, 'view')
      .mockResolvedValueOnce(viewFixture({ timeline: {
        scene_revision: '1', read_seq: '1', groups: [groupFixture()], items: [item],
        has_more: true, older_cursor: 'history1', is_empty: false,
      } }))
      .mockResolvedValueOnce(viewFixture({ scene_revision: '2', seq: '5', cursor: 'scene2-cursor5', timeline: {
        scene_revision: '2', read_seq: '5', groups: [], items: [],
        has_more: false, older_cursor: null, is_empty: true,
      } }));
    let finishPage!: (page: TimelinePage) => void;
    let finishDetail!: (body: PublicObject) => void;
    const timeline = vi.spyOn(sessionApi, 'timeline').mockReturnValue(new Promise(resolve => { finishPage = resolve; }));
    const detail = vi.spyOn(sessionApi, 'detail').mockReturnValue(new Promise(resolve => { finishDetail = resolve; }));
    store().retain('t1', 'p1'); await store().ensure('t1', 'p1');
    const oldStream = Stream.opened[0];
    const page = store().loadOlder('t1'), body = store().loadDetail('t1', item.id);
    await vi.waitFor(() => { expect(timeline).toHaveBeenCalledOnce(); expect(detail).toHaveBeenCalledOnce(); });
    store().freezeProject('p1');
    expect(store().sessions.t1.loadStatus).toBe('refreshing');
    expect(oldStream.close).toHaveBeenCalledOnce();
    store().resetProject('p1');
    await store().ensure('t1', 'p1');
    finishPage({ scene_revision: '1', read_seq: '1', groups: [groupFixture({ id: 'old' })], items: [item],
      older_cursor: null, has_more: false, is_empty: false });
    finishDetail({ text: 'old scene detail' });
    oldStream.batch('2', [{ type: 'timeline.item.upsert', entity: item }]);
    await Promise.all([page, body]);
    expect(read).toHaveBeenCalledTimes(2);
    expect(store().sessions.t1).toMatchObject({ loadStatus: 'ready', sceneRevision: '2',
      appliedSeq: '5', cursor: 'scene2-cursor5', isEmpty: true, hasMore: false,
      orderedGroupIds: [], groupsById: {}, itemsById: {}, detailsByRef: {}, detailStates: {},
    });
  });

  it('invalidates token statistics without resetting loaded content', async () => {
    const read = vi.spyOn(sessionApi, 'view').mockResolvedValue(viewFixture());
    store().retain('t1', 'p1'); await store().ensure('t1', 'p1');
    Stream.opened[0].batch('2', [
      { type: 'context_window.invalidate', entity: { context_revision: '1' } },
      { type: 'thread.upsert', entity: threadFixture({ context_revision: '1', entity_revision: '2', updated_seq: '2' }) },
    ]);
    expect(store().sessions.t1).toMatchObject({ loadStatus: 'ready', appliedSeq: '2', contextWindow: null });
    expect(read).toHaveBeenCalledOnce();
  });
});
