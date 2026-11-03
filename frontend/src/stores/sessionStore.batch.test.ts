import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { sessionApi, type ChangePage, type SessionBatch, type SessionView } from '../api/session';
import { commandFixture, groupFixture, itemFixture, runFixture, threadFixture, viewFixture } from '../test/sessionFixtures';
import { useSessionStore } from './sessionStore';

class Stream extends EventTarget {
  static opened: Stream[] = [];
  onopen: (() => void) | null = null;
  onerror: (() => void) | null = null;
  close = vi.fn();
  constructor(readonly url: string) { super(); Stream.opened.push(this); }
  batch(batch: SessionBatch) {
    this.dispatchEvent(new MessageEvent('thread.change', { lastEventId: `cursor${batch.seq}`, data: JSON.stringify(batch) }));
  }
  reset() {
    this.dispatchEvent(new MessageEvent('thread.reset', { data: JSON.stringify({ reason: 'SCENE_CHANGED' }) }));
  }
}

const store = () => useSessionStore.getState();
const batch = (seq: string, changes: SessionBatch['changes'] = []): SessionBatch => ({
  project_id: 'p1', thread_id: 't1', scene_revision: '1', seq, commit_id: `commit${seq}`, changes,
});
const item = (seq: string) => itemFixture({
  id: `item${seq}`, kind: 'tool', entity_revision: seq, updated_seq: seq, position: seq,
  summary: { tool: 'read_resource', status: 'completed' },
});
const nextScene = () => viewFixture({
  scene_revision: '2', seq: '5', cursor: 'scene2-cursor5',
  timeline: { scene_revision: '2', read_seq: '5', groups: [], items: [], older_cursor: null, has_more: false, is_empty: true },
});

async function retain(view: SessionView = viewFixture()) {
  vi.spyOn(sessionApi, 'view').mockResolvedValue(view);
  const release = store().retain('t1', 'p1');
  await store().ensure('t1', 'p1');
  return release;
}

beforeEach(() => {
  vi.useFakeTimers();
  Stream.opened = [];
  vi.stubGlobal('EventSource', Stream);
  // Hidden tabs can suspend animation frames entirely.
  vi.stubGlobal('requestAnimationFrame', vi.fn());
});
afterEach(() => {
  store().drop(Object.keys(store().sessions));
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

describe('session batch coalescing', () => {
  it('commits a burst once with the same result as sequential delivery', async () => {
    const batches = [
      batch('2', [
        { type: 'timeline.group.upsert', entity: groupFixture({ id: 'later', order_seq: '2', updated_seq: '2' }) },
        { type: 'timeline.item.upsert', entity: item('2') },
      ]),
      batch('3', [
        { type: 'timeline.group.upsert', entity: groupFixture({ id: 'earlier', order_seq: '1', updated_seq: '3' }) },
        { type: 'timeline.item.upsert', entity: { ...item('2'), entity_revision: '3', updated_seq: '3', summary: { status: 'failed' } } },
        { type: 'timeline.item.remove', id: 'removed', entity_revision: '3' },
      ]),
      batch('4', [
        { type: 'timeline.item.upsert', entity: { ...item('2'), id: 'removed', entity_revision: '2', updated_seq: '2' } },
        { type: 'timeline.item.upsert', entity: item('4') },
      ]),
    ];
    await retain();
    const before = store().sessions.t1;
    const notifications = vi.fn();
    const unsubscribe = useSessionStore.subscribe(notifications);
    batches.forEach(value => Stream.opened[0].batch(value));
    expect(store().sessions.t1).toBe(before);
    await vi.advanceTimersByTimeAsync(16);
    const merged = store().sessions.t1;
    expect(notifications).toHaveBeenCalledOnce();
    expect(merged).toMatchObject({ appliedSeq: '4', cursor: 'cursor4', orderedGroupIds: ['earlier', 'later'] });
    expect(merged.itemsById.removed).toBeUndefined();
    expect(before.itemsById).toEqual({});
    unsubscribe();

    store().drop(['t1']);
    store().retain('t1', 'p1');
    await store().ensure('t1', 'p1');
    for (const value of batches) {
      Stream.opened[1].batch(value);
      await vi.advanceTimersByTimeAsync(16);
    }
    expect(store().sessions.t1).toEqual(merged);
  });

  it('preserves timeline references for run and command updates', async () => {
    await retain(viewFixture({ thread: threadFixture({ active_run_id: 'r1' }), active_run: runFixture() }));
    const before = store().sessions.t1;
    Stream.opened[0].batch(batch('2', [{ type: 'run.upsert', entity: runFixture({ entity_revision: '2', updated_seq: '2' }) }]));
    Stream.opened[0].batch(batch('3', [{ type: 'command.upsert', entity: commandFixture({ entity_revision: '3', updated_seq: '3' }) }]));
    await vi.advanceTimersByTimeAsync(16);
    const after = store().sessions.t1;
    expect(after.itemsById).toBe(before.itemsById);
    expect(after.groupsById).toBe(before.groupsById);
    expect(after.orderedGroupIds).toBe(before.orderedGroupIds);
    expect(after.optimisticInputs).toBe(before.optimisticInputs);
    expect(after.runsById).not.toBe(before.runsById);
    expect(after.commandsById).not.toBe(before.commandsById);
    expect(before.commandsById).toEqual({});
  });

  it('keeps the loaded history boundary after earlier batches remove its oldest group', async () => {
    await retain(viewFixture({ seq: '5', cursor: 'cursor5', timeline: {
      scene_revision: '1', read_seq: '5', groups: [
        groupFixture({ id: 'oldest', order_seq: '1' }),
        groupFixture({ id: 'latest', order_seq: '5', updated_seq: '5' }),
      ], items: [], has_more: true, older_cursor: 'history5', is_empty: false,
    } }));
    Stream.opened[0].batch(batch('6', [
      { type: 'timeline.group.upsert', entity: groupFixture({ id: 'middle2', order_seq: '2', updated_seq: '6' }) },
      { type: 'timeline.group.remove', id: 'oldest', entity_revision: '6' },
    ]));
    Stream.opened[0].batch(batch('7', [
      { type: 'timeline.group.upsert', entity: groupFixture({ id: 'middle3', order_seq: '3', updated_seq: '7' }) },
    ]));
    await vi.advanceTimersByTimeAsync(16);
    expect(store().sessions.t1.orderedGroupIds).toEqual(['middle2', 'middle3', 'latest']);
    expect(store().sessions.t1.groupsById.oldest).toBeUndefined();
  });

  it('copies the item map once for an entire HTTP catchup page', async () => {
    await retain();
    const before = store().sessions.t1;
    const copies = vi.fn(Reflect.ownKeys);
    const itemsById = new Proxy<typeof before.itemsById>(before.itemsById, { ownKeys: copies });
    useSessionStore.setState(state => ({ sessions: { ...state.sessions, t1: { ...before, itemsById } } }));
    const batches = ['2', '3', '4'].map(seq => batch(seq, [{ type: 'timeline.item.upsert', entity: item(seq) }]));
    vi.spyOn(sessionApi, 'changes').mockResolvedValue({ batches, cursor: 'cursor4', has_more: false });
    const applied: string[] = [];
    const unsubscribe = useSessionStore.subscribe((state, previous) => {
      if (state.sessions.t1.appliedSeq !== previous.sessions.t1.appliedSeq) applied.push(state.sessions.t1.appliedSeq!);
    });
    await store().reconnect('t1');
    unsubscribe();
    expect(copies).toHaveBeenCalledOnce();
    expect(applied).toEqual(['4']);
    expect(Object.keys(store().sessions.t1.itemsById)).toEqual(['item2', 'item3', 'item4']);
  });

  it('does not publish a partial burst or skip the missing batch on a gap', async () => {
    await retain();
    let finish!: (page: ChangePage) => void;
    const changes = vi.spyOn(sessionApi, 'changes').mockReturnValue(new Promise(resolve => { finish = resolve; }));
    Stream.opened[0].batch(batch('2', [{ type: 'timeline.item.upsert', entity: item('2') }]));
    Stream.opened[0].batch(batch('4', [{ type: 'timeline.item.upsert', entity: item('4') }]));
    await vi.advanceTimersByTimeAsync(16);
    expect(changes).toHaveBeenCalledWith('t1', 'cursor1', expect.any(AbortSignal));
    expect(store().sessions.t1).toMatchObject({ appliedSeq: '1', cursor: 'cursor1', itemsById: {} });
    finish({ batches: ['2', '3', '4'].map(seq => batch(seq, [{ type: 'timeline.item.upsert', entity: item(seq) }])), cursor: 'cursor4', has_more: false });
    await store().reconnect('t1');
    expect(store().sessions.t1).toMatchObject({ appliedSeq: '4', cursor: 'cursor4' });
    expect(Object.keys(store().sessions.t1.itemsById)).toEqual(['item2', 'item3', 'item4']);
    expect(Stream.opened[0].close).toHaveBeenCalledOnce();
  });

  it('validates each batch before later batches can repair an invalid active run', async () => {
    let finish!: (view: SessionView) => void;
    const read = vi.spyOn(sessionApi, 'view').mockResolvedValueOnce(viewFixture())
      .mockReturnValueOnce(new Promise(resolve => { finish = resolve; }));
    store().retain('t1', 'p1');
    await store().ensure('t1', 'p1');
    Stream.opened[0].batch(batch('2', [{ type: 'thread.upsert', entity: threadFixture({ active_run_id: 'r1', entity_revision: '2', updated_seq: '2' }) }]));
    Stream.opened[0].batch(batch('3', [{ type: 'run.upsert', entity: runFixture({ updated_seq: '3' }) }]));
    await vi.advanceTimersByTimeAsync(16);
    expect(read).toHaveBeenCalledTimes(2);
    expect(store().sessions.t1).toMatchObject({ appliedSeq: '1', cursor: 'cursor1', thread: { active_run_id: null }, runsById: {} });
    finish(nextScene());
    await store().ensure('t1', 'p1');
    expect(store().sessions.t1.sceneRevision).toBe('2');
  });

  it('discards queued changes on reset before loading the new scene', async () => {
    await retain();
    vi.mocked(sessionApi.view).mockResolvedValue(nextScene());
    const source = Stream.opened[0];
    source.batch(batch('2', [{ type: 'timeline.item.upsert', entity: item('2') }]));
    source.reset();
    await store().ensure('t1', 'p1');
    source.batch(batch('6', [{ type: 'timeline.item.upsert', entity: item('6') }]));
    await vi.advanceTimersByTimeAsync(16);
    expect(store().sessions.t1).toMatchObject({ sceneRevision: '2', appliedSeq: '5', cursor: 'scene2-cursor5', itemsById: {} });
  });

  it('keeps frozen queues and a dropped controller out of a replacement session', async () => {
    await retain();
    const source = Stream.opened[0];
    source.batch(batch('2', [{ type: 'timeline.item.upsert', entity: item('2') }]));
    store().freezeProject('p1');
    await vi.advanceTimersByTimeAsync(16);
    expect(store().sessions.t1).toMatchObject({ loadStatus: 'refreshing', appliedSeq: '1', itemsById: {} });
    vi.mocked(sessionApi.view).mockResolvedValue(nextScene());
    store().resetProject('p1');
    await store().ensure('t1', 'p1');
    const replacement = Stream.opened[1];
    replacement.batch({ ...batch('6', [{ type: 'timeline.item.upsert', entity: item('6') }]), scene_revision: '2' });
    store().drop(['t1']);
    store().retain('t1', 'p1');
    await store().ensure('t1', 'p1');
    source.batch(batch('7', [{ type: 'timeline.item.upsert', entity: item('7') }]));
    await vi.advanceTimersByTimeAsync(16);
    expect(store().sessions.t1).toMatchObject({ sceneRevision: '2', appliedSeq: '5', cursor: 'scene2-cursor5', itemsById: {} });
  });

  it.each(['disconnect', 'release'] as const)('flushes buffered batches before %s', async action => {
    const release = await retain();
    const source = Stream.opened[0];
    source.batch(batch('2', [{ type: 'timeline.item.upsert', entity: item('2') }]));
    if (action === 'disconnect') source.onerror!();
    else { release(); await vi.advanceTimersByTimeAsync(0); }
    expect(source.close).toHaveBeenCalledOnce();
    expect(store().sessions.t1).toMatchObject({ appliedSeq: '2', cursor: 'cursor2' });
    await vi.advanceTimersByTimeAsync(16);
    expect(Object.keys(store().sessions.t1.itemsById)).toEqual(['item2']);
  });
});
