import { create } from 'zustand';
import { APIError, RequestCanceledError } from '../api/client';
import {
  compareRevision, parseSessionBatch, parseSessionEntity, sessionApi, SessionProtocolError,
  type CommandView, type EntityKind, type InteractionView, type PublicObject,
  type Revision, type Revisioned, type RunView, type SessionBatch, type SessionChange,
  type SessionContextWindow, type SessionView, type ThreadView, type TimelineGroup, type TimelinePage, type TimelineRecord,
} from '../api/session';
import { isProjectDeleted, isThreadDeleted } from './deletedTargets';
import type { CreateRunRequest } from '../api/types';

export type SessionLoadStatus = 'uninitialized' | 'loading' | 'ready' | 'refreshing' | 'error';
export type SessionConnectionStatus = 'closed' | 'connecting' | 'open' | 'reconnecting';

interface PageState {
  status: 'idle' | 'loading' | 'error';
  error: Error | null;
}

export interface GroupPageState extends PageState {
  mode: 'summary' | 'page';
  olderCursor: string | null;
  hasMore: boolean;
}

export interface OptimisticInput {
  clientMessageId: string;
  text: string;
  createdAt: number;
  status: 'sending' | 'accepted' | 'error';
  sceneRevision?: Revision | null;
  runId?: string;
  request?: CreateRunRequest;
  error?: Error;
}

export interface SessionState {
  threadId: string;
  projectId: string;
  generation: number;
  loadStatus: SessionLoadStatus;
  connectionStatus: SessionConnectionStatus;
  sceneRevision: Revision | null;
  appliedSeq: Revision | null;
  snapshotSeq: Revision | null;
  cursor: string | null;
  thread: ThreadView | null;
  runsById: Record<string, RunView>;
  commandsById: Record<string, CommandView>;
  interactionsById: Record<string, InteractionView>;
  groupsById: Record<string, TimelineGroup>;
  itemsById: Record<string, TimelineRecord>;
  orderedGroupIds: string[];
  removedVersions: Record<string, Revision>;
  groupPages: Record<string, GroupPageState>;
  detailsByRef: Record<string, PublicObject>;
  detailStates: Record<string, PageState>;
  optimisticInputs: Record<string, OptimisticInput>;
  olderCursor: string | null;
  hasMore: boolean;
  isEmpty: boolean;
  historyPage: PageState;
  contextWindow: SessionContextWindow | null;
  error: Error | null;
}

interface SessionStore {
  sessions: Record<string, SessionState>;
  retain: (threadId: string, projectId: string) => () => void;
  ensure: (threadId: string, projectId: string) => Promise<void>;
  retry: (threadId: string) => Promise<void>;
  reconnect: (threadId: string) => Promise<void>;
  loadOlder: (threadId: string) => Promise<void>;
  loadGroup: (threadId: string, groupId: string) => Promise<void>;
  loadDetail: (threadId: string, itemId: string, changeRef?: string) => Promise<void>;
  putOptimisticInput: (threadId: string, projectId: string, input: OptimisticInput) => void;
  freezeProject: (projectId: string) => void;
  resetProject: (projectId: string) => void;
  drop: (threadIds: string[]) => void;
}

function fresh(threadId: string, projectId: string, generation = 0): SessionState {
  return {
    threadId, projectId, generation, loadStatus: 'uninitialized', connectionStatus: 'closed',
    sceneRevision: null, appliedSeq: null, snapshotSeq: null, cursor: null, thread: null,
    runsById: {}, commandsById: {}, interactionsById: {}, groupsById: {}, itemsById: {},
    orderedGroupIds: [], removedVersions: {}, groupPages: {}, detailsByRef: {}, detailStates: {},
    optimisticInputs: {}, olderCursor: null, hasMore: false, isEmpty: false,
    historyPage: { status: 'idle', error: null }, contextWindow: null, error: null,
  };
}

function clone(state: SessionState): SessionState {
  return { ...state, runsById: { ...state.runsById }, commandsById: { ...state.commandsById },
    interactionsById: { ...state.interactionsById }, groupsById: { ...state.groupsById },
    itemsById: { ...state.itemsById }, removedVersions: { ...state.removedVersions },
    groupPages: { ...state.groupPages }, detailsByRef: { ...state.detailsByRef },
    detailStates: { ...state.detailStates }, optimisticInputs: { ...state.optimisticInputs },
    orderedGroupIds: [...state.orderedGroupIds] };
}

function assertSync(valid: unknown, field: string): asserts valid {
  if (!valid) throw new SessionProtocolError(field);
}

function upsert<T extends Revisioned>(state: SessionState, kind: EntityKind, map: Record<string, T>, id: string, value: T): boolean {
  const removed = state.removedVersions[`${kind}:${id}`];
  if (removed && compareRevision(removed, value.entity_revision) >= 0) return false;
  const previous = map[id];
  if (previous && compareRevision(previous.entity_revision, value.entity_revision) >= 0) return false;
  map[id] = value;
  return true;
}

function sortGroups(state: SessionState): void {
  state.orderedGroupIds = [...new Set(state.orderedGroupIds)].filter(id => state.groupsById[id])
    .sort((a, b) => compareRevision(state.groupsById[a].order_seq, state.groupsById[b].order_seq) || (a === b ? 0 : a < b ? -1 : 1));
}

function changeDiffRefs(item: TimelineRecord | undefined): string[] {
  const targets = item?.summary.affected_targets;
  return Array.isArray(targets)
    ? targets.flatMap(target => typeof target?.diff_ref === 'string' ? [target.diff_ref] : [])
    : [];
}

function putItem(state: SessionState, item: TimelineRecord): void {
  const previous = state.itemsById[item.id];
  if (!upsert(state, 'timeline.item', state.itemsById, item.id, item)) return;
  if (previous) {
    for (const ref of [previous.detail_ref, previous.body_ref]) {
      if (ref && ref !== item.detail_ref && ref !== item.body_ref) delete state.detailsByRef[ref];
    }
    const changes = new Set(changeDiffRefs(item));
    for (const ref of changeDiffRefs(previous)) if (!changes.has(ref)) {
      delete state.detailsByRef[ref];
      delete state.detailStates[ref];
    }
    delete state.detailStates[item.id];
  }
  if (item.kind === 'user_turn' && typeof item.summary.clientMessageId === 'string') {
    delete state.optimisticInputs[item.summary.clientMessageId];
  }
  if (item.kind === 'command') {
    const command = parseSessionEntity(item.summary, 'command', state.threadId);
    assertSync(command.project_id === state.projectId, 'item.command.project_id');
    upsert(state, 'command', state.commandsById, command.command_id, command);
  } else if (item.kind === 'interaction') {
    const interaction = parseSessionEntity(item.summary, 'interaction', state.threadId);
    upsert(state, 'interaction', state.interactionsById, interaction.id, interaction);
  }
}

function acceptPage(state: SessionState, page: TimelinePage): void {
  assertSync(page.scene_revision === state.sceneRevision, 'page.scene_revision');
  for (const group of page.groups) {
    assertSync(compareRevision(group.updated_seq, page.read_seq) <= 0, 'group.read_seq');
    upsert(state, 'timeline.group', state.groupsById, group.id, group);
    const removed = state.removedVersions[`timeline.group:${group.id}`];
    if (removed && compareRevision(removed, group.entity_revision) >= 0) continue;
    state.orderedGroupIds.push(group.id);
    if (!state.groupPages[group.id]) state.groupPages[group.id] = {
      mode: group.items_mode!, olderCursor: group.older_item_cursor, hasMore: group.has_more_items,
      status: 'idle', error: null,
    };
  }
  for (const item of page.items) {
    assertSync(compareRevision(item.updated_seq, page.read_seq) <= 0, 'item.read_seq');
    putItem(state, item);
  }
  sortGroups(state);
  // A historical page never changes appliedSeq or the live cursor.
  state.olderCursor = page.older_cursor;
  state.hasMore = page.has_more;
  state.historyPage = { status: 'idle', error: null };
}

function fromView(previous: SessionState, view: SessionView): SessionState {
  assertSync(view.project_id === previous.projectId, 'view.project_id');
  const next = fresh(previous.threadId, previous.projectId, previous.generation);
  next.optimisticInputs = { ...previous.optimisticInputs };
  for (const [id, input] of Object.entries(next.optimisticInputs)) {
    if (input.sceneRevision && input.sceneRevision !== view.scene_revision) next.optimisticInputs[id] = {
      ...input, status: 'error', error: new APIError(409, 'SCENE_CHANGED', 'SCENE_CHANGED'),
    };
  }
  next.sceneRevision = view.scene_revision;
  next.appliedSeq = view.seq;
  next.snapshotSeq = view.seq;
  next.cursor = view.cursor;
  next.thread = view.thread;
  next.loadStatus = 'ready';
  next.isEmpty = view.timeline.is_empty;
  next.contextWindow = view.context_window;
  if (view.active_run) next.runsById[view.active_run.id] = view.active_run;
  for (const command of view.active_commands) next.commandsById[command.command_id] = command;
  for (const interaction of view.pending_interactions) next.interactionsById[interaction.id] = interaction;
  acceptPage(next, view.timeline);
  return next;
}

function removeEntity(state: SessionState, kind: EntityKind, id: string, version: Revision): void {
  const key = `${kind}:${id}`;
  if (state.removedVersions[key] && compareRevision(state.removedVersions[key], version) >= 0) return;
  state.removedVersions[key] = version;
  const remove = <T extends Revisioned>(map: Record<string, T>) => {
    if (map[id] && compareRevision(map[id].entity_revision, version) <= 0) delete map[id];
  };
  switch (kind) {
    case 'thread':
      assertSync(id === state.threadId, 'thread.remove');
      if (state.thread && compareRevision(state.thread.entity_revision, version) <= 0) state.thread = null;
      break;
    case 'run': remove(state.runsById); break;
    case 'command': remove(state.commandsById); break;
    case 'interaction': remove(state.interactionsById); break;
    case 'timeline.group':
      remove(state.groupsById);
      if (!state.groupsById[id]) {
        state.orderedGroupIds = state.orderedGroupIds.filter(value => value !== id);
        delete state.groupPages[id];
      }
      break;
    case 'timeline.item': {
      const item = state.itemsById[id];
      remove(state.itemsById);
      if (item && !state.itemsById[id]) {
        if (item.detail_ref) delete state.detailsByRef[item.detail_ref];
        if (item.body_ref) delete state.detailsByRef[item.body_ref];
        for (const ref of changeDiffRefs(item)) {
          delete state.detailsByRef[ref];
          delete state.detailStates[ref];
        }
        delete state.detailStates[id];
      }
      break;
    }
  }
}

function applyChange(state: SessionState, change: SessionChange, seq: Revision): void {
  if (change.type === 'context_window.invalidate') { state.contextWindow = null; return; }
  if ('entity' in change) assertSync(compareRevision(change.entity.updated_seq, seq) <= 0, 'change.updated_seq');
  switch (change.type) {
    case 'thread.upsert': {
      const row = change.entity;
      assertSync(row.project_id === state.projectId, 'thread.project_id');
      const map = state.thread ? { [state.thread.id]: state.thread } : {};
      if (upsert(state, 'thread', map, row.id, row)) state.thread = row;
      break;
    }
    case 'run.upsert':
      assertSync(change.entity.project_id === state.projectId && change.entity.scene_revision === state.sceneRevision, 'run.scene');
      upsert(state, 'run', state.runsById, change.entity.id, change.entity);
      break;
    case 'command.upsert':
      assertSync(change.entity.project_id === state.projectId, 'command.project_id');
      upsert(state, 'command', state.commandsById, change.entity.command_id, change.entity);
      break;
    case 'interaction.upsert':
      upsert(state, 'interaction', state.interactionsById, change.entity.id, change.entity);
      break;
    case 'timeline.group.upsert': {
      const group = change.entity;
      if (!upsert(state, 'timeline.group', state.groupsById, group.id, group)) break;
      // Old, unloaded groups remain sparse until the user requests their page.
      // An upsert carries no paging coverage, even when it has item_count.
      let oldest = state.groupsById[state.orderedGroupIds[0]];
      // Several commits share one final sort, so the first loaded ID may no
      // longer be the oldest after an earlier commit removes a group.
      if (state.hasMore) for (const id of state.orderedGroupIds) {
        const candidate = state.groupsById[id];
        if (candidate && (!oldest || compareRevision(candidate.order_seq, oldest.order_seq) < 0)) oldest = candidate;
      }
      if (!state.hasMore || oldest && compareRevision(group.order_seq, oldest.order_seq) >= 0
        || !oldest && state.appliedSeq !== null && compareRevision(group.order_seq, state.appliedSeq) > 0) {
        state.orderedGroupIds.push(group.id);
      }
      state.isEmpty = false;
      break;
    }
    case 'timeline.item.upsert': putItem(state, change.entity); state.isEmpty = false; break;
    default:
      if ('id' in change) removeEntity(state, change.type.slice(0, -7) as EntityKind, change.id, change.entity_revision);
  }
}

class SequenceGap extends Error {}

function cloneBatches(previous: SessionState, batches: readonly SessionBatch[]): SessionState {
  const types = new Set<SessionChange['type']>();
  let commandItem = false, interactionItem = false;
  for (const batch of batches) {
    if (compareRevision(batch.seq, previous.appliedSeq!) <= 0) continue;
    for (const change of batch.changes) {
      types.add(change.type);
      if (change.type === 'timeline.item.upsert') {
        commandItem ||= change.entity.kind === 'command';
        interactionItem ||= change.entity.kind === 'interaction';
      }
    }
  }
  const has = (...values: SessionChange['type'][]) => values.some(value => types.has(value));
  const items = has('timeline.item.upsert', 'timeline.item.remove');
  const groups = has('timeline.group.upsert', 'timeline.group.remove');
  return { ...previous,
    runsById: has('run.upsert', 'run.remove') ? { ...previous.runsById } : previous.runsById,
    commandsById: commandItem || has('command.upsert', 'command.remove') ? { ...previous.commandsById } : previous.commandsById,
    interactionsById: interactionItem || has('interaction.upsert', 'interaction.remove') ? { ...previous.interactionsById } : previous.interactionsById,
    groupsById: groups ? { ...previous.groupsById } : previous.groupsById,
    itemsById: items ? { ...previous.itemsById } : previous.itemsById,
    removedVersions: [...types].some(type => type.endsWith('.remove')) ? { ...previous.removedVersions } : previous.removedVersions,
    groupPages: types.has('timeline.group.remove') ? { ...previous.groupPages } : previous.groupPages,
    detailsByRef: items ? { ...previous.detailsByRef } : previous.detailsByRef,
    detailStates: items ? { ...previous.detailStates } : previous.detailStates,
    optimisticInputs: types.has('timeline.item.upsert') ? { ...previous.optimisticInputs } : previous.optimisticInputs,
    orderedGroupIds: groups ? [...previous.orderedGroupIds] : previous.orderedGroupIds,
  };
}

function applyBatch(next: SessionState, batch: SessionBatch): void {
  const previousActiveId = next.thread?.active_run_id;
  for (const change of batch.changes) applyChange(next, change, batch.seq);
  const activeId = next.thread?.active_run_id;
  assertSync(!activeId || next.runsById[activeId], 'batch.active_run');
  if (activeId) {
    next.isEmpty = false;
    const estimate = next.runsById[activeId].context_window;
    next.contextWindow = estimate?.context_revision === next.thread?.context_revision && estimate?.plan_revision === next.thread?.plan_revision ? estimate ?? null : null;
  } else if (previousActiveId) {
    const estimate = next.runsById[previousActiveId]?.context_window;
    next.contextWindow = estimate?.context_revision === next.thread?.context_revision && estimate?.plan_revision === next.thread?.plan_revision ? estimate ?? null : null;
  }
  if (Object.values(next.commandsById).some(row => row.kind !== 'rename' && row.kind !== 'suggest'
    && ['accepted', 'running', 'cancel_requested'].includes(row.status))) next.isEmpty = false;
  next.appliedSeq = batch.seq;
  if (!next.thread) {
    next.loadStatus = 'error';
    next.connectionStatus = 'closed';
    next.error = new APIError(404, 'NOT_FOUND', 'NOT_FOUND');
  }
}

function applyBatches(previous: SessionState, batches: readonly SessionBatch[]): SessionState {
  let next = previous;
  for (const batch of batches) {
    assertSync(batch.project_id === next.projectId && batch.thread_id === next.threadId, 'batch.identity');
    assertSync(batch.scene_revision === next.sceneRevision && next.appliedSeq !== null, 'batch.scene_revision');
    if (compareRevision(batch.seq, next.appliedSeq) <= 0) continue;
    if (BigInt(batch.seq) !== BigInt(next.appliedSeq) + 1n) throw new SequenceGap();
    if (next === previous) next = cloneBatches(previous, batches);
    applyBatch(next, batch);
  }
  if (next.orderedGroupIds !== previous.orderedGroupIds) {
    sortGroups(next);
    if (next.orderedGroupIds.length === previous.orderedGroupIds.length
      && next.orderedGroupIds.every((id, index) => id === previous.orderedGroupIds[index])) {
      next.orderedGroupIds = previous.orderedGroupIds;
    }
  }
  return next;
}

interface QueuedBatch { batch: SessionBatch; cursor: string }

interface Controller {
  threadId: string;
  projectId: string;
  generation: number;
  refs: number;
  frozen: boolean;
  abort: AbortController;
  source?: EventSource;
  retryTimer?: ReturnType<typeof setTimeout>;
  releaseTimer?: ReturnType<typeof setTimeout>;
  batchTimer?: ReturnType<typeof setTimeout>;
  batches: QueuedBatch[];
  failures: number;
  pending: Map<string, Promise<void>>;
}

const controllers = new Map<string, Controller>();
const resetCodes = new Set(['SCENE_CHANGED', 'CURSOR_RESET_REQUIRED', 'PERSISTENCE_FAILED']);
function asError(error: unknown): Error { return error instanceof Error ? error : new Error(String(error)); }

export const useSessionStore = create<SessionStore>((set, get) => {
  function controller(threadId: string, projectId: string): Controller {
    const existing = controllers.get(threadId);
    if (existing) {
      assertSync(existing.projectId === projectId, 'session.project_id');
      return existing;
    }
    const value: Controller = { threadId, projectId, generation: 0, refs: 0, frozen: false,
      abort: new AbortController(), failures: 0, pending: new Map(), batches: [] };
    controllers.set(threadId, value);
    set(state => ({ sessions: { ...state.sessions, [threadId]: fresh(threadId, projectId) } }));
    return value;
  }

  function current(c: Controller, generation = c.generation): boolean {
    return controllers.get(c.threadId) === c && c.generation === generation
      && !isThreadDeleted(c.threadId) && !isProjectDeleted(c.projectId);
  }

  function update(c: Controller, transform: (value: SessionState) => SessionState): void {
    if (!current(c)) return;
    set(state => {
      const value = state.sessions[c.threadId];
      if (!value || value.generation !== c.generation) return state;
      const next = transform(value);
      return next === value ? state : { sessions: { ...state.sessions, [c.threadId]: next } };
    });
  }

  function disconnect(c: Controller): void {
    c.source?.close();
    c.source = undefined;
    clearTimeout(c.retryTimer);
    c.retryTimer = undefined;
    clearTimeout(c.batchTimer);
    c.batchTimer = undefined;
    c.batches = [];
  }

  function flushBatches(c: Controller): void {
    const queued = c.batches;
    clearTimeout(c.batchTimer);
    c.batchTimer = undefined;
    c.batches = [];
    if (!queued.length || !current(c) || c.frozen) return;
    update(c, previous => {
      const next = applyBatches(previous, queued.map(value => value.batch));
      if (next === previous) return previous;
      const cursor = queued.find(value => value.batch.seq === next.appliedSeq)!.cursor;
      return { ...next, cursor, error: null };
    });
    c.failures = 0;
  }

  function flushStream(c: Controller): boolean {
    try { flushBatches(c); return true; }
    catch (error) {
      if (error instanceof SequenceGap) {
        disconnect(c);
        void catchUp(c);
      } else syncFailure(c, error);
      return false;
    }
  }

  function invalidate(c: Controller): void {
    disconnect(c);
    c.generation++;
    c.abort.abort();
    c.abort = new AbortController();
    c.pending.clear();
    set(state => {
      const value = state.sessions[c.threadId];
      return !value ? state : { sessions: { ...state.sessions, [c.threadId]: {
        ...value, generation: c.generation, connectionStatus: 'closed',
        loadStatus: value.thread ? 'refreshing' : 'loading', error: null,
        historyPage: { status: 'idle', error: null }, detailStates: {},
        groupPages: Object.fromEntries(Object.entries(value.groupPages).map(([id, page]) => [id, { ...page, status: 'idle', error: null }])),
      } } };
    });
  }

  function schedule(c: Controller, action: () => Promise<void>): void {
    if (!current(c) || c.refs === 0 || c.frozen || c.retryTimer) return;
    const generation = c.generation;
    c.retryTimer = setTimeout(() => {
      c.retryTimer = undefined;
      if (current(c, generation)) void action();
    }, Math.min(10_000, 500 * 2 ** Math.min(c.failures++, 5)));
  }

  async function single(c: Controller, key: string, work: (generation: number) => Promise<void>): Promise<void> {
    const pending = c.pending.get(key);
    if (pending) return pending;
    const generation = c.generation;
    const promise = Promise.resolve().then(() => current(c, generation) && !c.frozen ? work(generation) : undefined);
    c.pending.set(key, promise);
    try { await promise; }
    finally { if (c.pending.get(key) === promise) c.pending.delete(key); }
  }

  async function readView(c: Controller, replace = false): Promise<void> {
    if (!current(c) || c.frozen) return;
    if (replace) invalidate(c);
    return single(c, 'view', async generation => {
      update(c, state => ({ ...state, loadStatus: state.thread ? 'refreshing' : 'loading', error: null }));
      try {
        const view = await sessionApi.view(c.threadId, c.abort.signal);
        if (!current(c, generation) || c.frozen) return;
        update(c, state => fromView(state, view));
        c.failures = 0;
        connect(c);
      } catch (error) {
        if (!current(c, generation) || error instanceof RequestCanceledError) return;
        const restoring = error instanceof APIError && error.code === 'PROJECT_RESTORING';
        update(c, state => ({ ...state, loadStatus: restoring ? 'refreshing' : 'error', error: asError(error) }));
        if (restoring) schedule(c, () => readView(c));
      }
    });
  }

  function syncFailure(c: Controller, error: unknown): void {
    if (!current(c) || error instanceof RequestCanceledError) return;
    disconnect(c);
    if (error instanceof APIError && error.status === 404) {
      update(c, state => ({ ...state, loadStatus: 'error', connectionStatus: 'closed', error }));
    } else if (error instanceof SessionProtocolError || error instanceof APIError && resetCodes.has(error.code ?? '')) {
      void readView(c, true);
    } else if (error instanceof APIError && error.code === 'PROJECT_RESTORING') {
      invalidate(c);
      update(c, state => ({ ...state, loadStatus: 'refreshing', error }));
      schedule(c, () => readView(c));
    } else {
      update(c, state => ({ ...state, connectionStatus: 'reconnecting', error: asError(error) }));
      schedule(c, () => catchUp(c));
    }
  }

  async function catchUp(c: Controller): Promise<void> {
    if (!current(c) || c.frozen) return;
    const initial = get().sessions[c.threadId];
    if (!initial?.cursor || initial.loadStatus !== 'ready') return readView(c);
    return single(c, 'changes', async generation => {
      try {
        // Buffered stream batches may precede a manual reconnect. A gap is
        // recovered from the last committed cursor by the range read below.
        try { flushBatches(c); }
        catch (error) { if (!(error instanceof SequenceGap)) throw error; }
        disconnect(c);
        update(c, state => ({ ...state, connectionStatus: 'reconnecting' }));
        let hasMore = true;
        while (hasMore && current(c, generation) && !c.frozen) {
          const after = get().sessions[c.threadId]?.cursor;
          if (!after) return;
          const page = await sessionApi.changes(c.threadId, after, c.abort.signal);
          if (!current(c, generation) || c.frozen) return;
          update(c, previous => {
            assertSync(previous.cursor === after, 'changes.cursor');
            const next = applyBatches(previous, page.batches);
            assertSync(!page.has_more || next.appliedSeq !== previous.appliedSeq, 'changes.progress');
            return { ...next, cursor: page.cursor, error: null };
          });
          hasMore = page.has_more;
        }
        if (current(c, generation)) connect(c);
      } catch (error) {
        if (current(c, generation)) syncFailure(c, error instanceof SequenceGap ? new SessionProtocolError('changes.sequence') : error);
      }
    });
  }

  function connect(c: Controller): void {
    const state = get().sessions[c.threadId];
    if (!current(c) || c.frozen || c.refs === 0 || c.source || state?.loadStatus !== 'ready' || !state.cursor || typeof EventSource === 'undefined') return;
    const generation = c.generation;
    const source = new EventSource(`/api/v1/threads/${encodeURIComponent(c.threadId)}/events?${new URLSearchParams({ cursor: state.cursor })}`);
    c.source = source;
    update(c, value => ({ ...value, connectionStatus: 'connecting' }));
    const active = () => current(c, generation) && c.source === source && !c.frozen;
    source.onopen = () => { if (active()) update(c, value => ({ ...value, connectionStatus: 'open', error: null })); };
    source.addEventListener('thread.change', (message: MessageEvent<string>) => {
      if (!active()) return;
      try {
        assertSync(message.lastEventId.length > 0, 'stream.cursor');
        const batch = parseSessionBatch(JSON.parse(message.data) as unknown, c.threadId);
        c.batches.push({ batch, cursor: message.lastEventId });
        // A timer also runs when animation frames are suspended in a hidden tab.
        if (!c.batchTimer) c.batchTimer = setTimeout(() => {
          c.batchTimer = undefined;
          if (active()) flushStream(c);
        }, 16);
      } catch (error) {
        syncFailure(c, error instanceof SyntaxError ? new SessionProtocolError('stream.json') : error);
      }
    });
    source.addEventListener('thread.reset', (message: MessageEvent<string>) => {
      if (!active()) return;
      try {
        const payload = JSON.parse(message.data) as { reason?: unknown };
        const reason = typeof payload.reason === 'string' ? payload.reason : 'SYNC_FAILED';
        syncFailure(c, new APIError(reason === 'NOT_FOUND' ? 404 : 409, reason, reason));
      } catch { syncFailure(c, new SessionProtocolError('reset')); }
    });
    source.onerror = () => {
      if (!active()) return;
      if (!flushStream(c)) return;
      disconnect(c);
      // EventSource hides the HTTP status. A range read distinguishes a reset
      // from a transient disconnect without replacing an otherwise valid view.
      update(c, value => ({ ...value, connectionStatus: 'reconnecting' }));
      schedule(c, () => catchUp(c));
    };
  }

  function pageFailure(c: Controller, error: unknown): boolean {
    if (error instanceof APIError && (resetCodes.has(error.code ?? '') || error.code === 'PROJECT_RESTORING' || error.status === 404)
      || error instanceof SessionProtocolError) {
      syncFailure(c, error);
      return true;
    }
    return error instanceof RequestCanceledError;
  }

  return {
    sessions: {},
    retain: (threadId, projectId) => {
      if (isThreadDeleted(threadId) || isProjectDeleted(projectId)) return () => {};
      const c = controller(threadId, projectId);
      clearTimeout(c.releaseTimer);
      c.refs++;
      void get().ensure(threadId, projectId);
      let released = false;
      return () => {
        if (released) return;
        released = true;
        c.refs = Math.max(0, c.refs - 1);
        if (c.refs === 0) c.releaseTimer = setTimeout(() => {
          if (current(c) && c.refs === 0) {
            if (!flushStream(c)) return;
            disconnect(c);
            update(c, state => ({ ...state, connectionStatus: 'closed' }));
          }
        }, 0);
      };
    },
    ensure: (threadId, projectId) => {
      if (isThreadDeleted(threadId) || isProjectDeleted(projectId)) return Promise.resolve();
      const c = controller(threadId, projectId);
      const value = get().sessions[threadId];
      if (value.loadStatus === 'ready') {
        const pending = c.pending.get('changes');
        if (pending) return pending;
        connect(c);
        return Promise.resolve();
      }
      return readView(c);
    },
    retry: async threadId => {
      const c = controllers.get(threadId);
      if (c) await readView(c, true);
    },
    reconnect: async threadId => {
      const c = controllers.get(threadId);
      if (c && !c.frozen) await catchUp(c);
    },
    loadOlder: async threadId => {
      const c = controllers.get(threadId);
      const start = get().sessions[threadId];
      if (!c || c.frozen || start?.loadStatus !== 'ready' || !start.hasMore || !start.olderCursor || !start.sceneRevision) return;
      await single(c, 'older', async generation => {
        update(c, state => ({ ...state, historyPage: { status: 'loading', error: null } }));
        try {
          const page = await sessionApi.timeline(threadId, start.sceneRevision!, start.olderCursor!, c.abort.signal);
          if (!current(c, generation)) return;
          if (!flushStream(c)) return;
          update(c, state => {
            assertSync(!page.has_more || page.older_cursor !== start.olderCursor, 'page.progress');
            const next = clone(state);
            acceptPage(next, page);
            return next;
          });
        } catch (error) {
          if (current(c, generation) && !pageFailure(c, error)) update(c, state => ({ ...state, historyPage: { status: 'error', error: asError(error) } }));
        }
      });
    },
    loadGroup: async (threadId, groupId) => {
      const c = controllers.get(threadId);
      const start = get().sessions[threadId];
      const coverage = start?.groupPages[groupId];
      if (!c || c.frozen || start?.loadStatus !== 'ready' || !start.sceneRevision || !start.groupsById[groupId]
        || coverage?.mode === 'page' && !coverage.hasMore) return;
      const before = coverage?.mode === 'page' ? coverage.olderCursor : null;
      await single(c, `group:${groupId}`, async generation => {
        update(c, state => ({ ...state, groupPages: { ...state.groupPages, [groupId]: {
          mode: coverage?.mode ?? 'summary', olderCursor: before, hasMore: true, status: 'loading', error: null,
        } } }));
        try {
          const page = await sessionApi.items(threadId, start.sceneRevision!, groupId, before, c.abort.signal);
          if (!current(c, generation)) return;
          if (!flushStream(c)) return;
          update(c, state => {
            assertSync(page.scene_revision === state.sceneRevision, 'items.scene_revision');
            assertSync(!page.has_more || page.older_cursor !== before, 'items.progress');
            if (!state.groupsById[groupId]) return state;
            const next = clone(state);
            for (const item of page.items) {
              assertSync(compareRevision(item.updated_seq, page.read_seq) <= 0, 'item.read_seq');
              putItem(next, item);
            }
            next.groupPages[groupId] = { mode: 'page', olderCursor: page.older_cursor, hasMore: page.has_more, status: 'idle', error: null };
            return next;
          });
        } catch (error) {
          if (current(c, generation) && !pageFailure(c, error)) update(c, state => ({ ...state,
            groupPages: { ...state.groupPages, [groupId]: { ...state.groupPages[groupId], status: 'error', error: asError(error) } },
          }));
        }
      });
    },
    loadDetail: async (threadId, itemId, changeRef) => {
      const c = controllers.get(threadId);
      const start = get().sessions[threadId];
      const reference = (state: SessionState | undefined) => {
        if (!state) return undefined;
        const item = state.itemsById[itemId];
        if (changeRef !== undefined) return changeDiffRefs(item).includes(changeRef) ? changeRef : undefined;
        const interaction = itemId.startsWith('interaction:') ? state.interactionsById[itemId.slice(12)] : undefined;
        const command = itemId.startsWith('command:') ? state.commandsById[itemId.slice(8)] : undefined;
        // Pinned current entities can be outside the historical page. Their
        // opaque ref binds the same saved item and revision on the server.
        if (interaction?.detail_ref) return interaction.detail_ref;
        if (command?.content_ref) return command.content_ref;
        return item?.detail_ref ?? item?.body_ref;
      };
      const ref = reference(start);
      if (!c || c.frozen || start?.loadStatus !== 'ready' || !ref || start.detailsByRef[ref]) return;
      const detailKey = changeRef ?? itemId;
      await single(c, `detail:${ref}`, async generation => {
        update(c, state => ({ ...state, detailStates: { ...state.detailStates, [detailKey]: { status: 'loading', error: null } } }));
        try {
          const value = await sessionApi.detail(threadId, itemId, ref, c.abort.signal);
          if (!current(c, generation)) return;
          if (!flushStream(c)) return;
          update(c, state => reference(state) !== ref ? state : ({ ...state,
            detailsByRef: { ...state.detailsByRef, [ref]: value },
            detailStates: { ...state.detailStates, [detailKey]: { status: 'idle', error: null } },
          }));
        } catch (error) {
          if (!current(c, generation) || reference(get().sessions[threadId]) !== ref) return;
          if (error instanceof APIError && error.code === 'CONTENT_REF_EXPIRED') { await readView(c, true); return; }
          if (!pageFailure(c, error)) update(c, state => ({ ...state,
            detailStates: { ...state.detailStates, [detailKey]: { status: 'error', error: asError(error) } },
          }));
        }
      });
    },
    putOptimisticInput: (threadId, projectId, input) => {
      if (isThreadDeleted(threadId) || isProjectDeleted(projectId)) return;
      const c = controller(threadId, projectId);
      update(c, state => Object.values(state.itemsById).some(item => item.kind === 'user_turn' && item.summary.clientMessageId === input.clientMessageId)
        ? state : { ...state, optimisticInputs: { ...state.optimisticInputs, [input.clientMessageId]: {
          ...input, sceneRevision: state.optimisticInputs[input.clientMessageId]?.sceneRevision ?? input.sceneRevision ?? state.sceneRevision,
        } } });
    },
    freezeProject: projectId => {
      for (const c of controllers.values()) if (c.projectId === projectId) { c.frozen = true; invalidate(c); }
    },
    resetProject: projectId => {
      for (const c of controllers.values()) if (c.projectId === projectId) {
        c.frozen = false;
        invalidate(c);
        if (c.refs > 0) void readView(c);
      }
    },
    drop: threadIds => {
      for (const id of threadIds) {
        const c = controllers.get(id);
        if (!c) continue;
        disconnect(c);
        clearTimeout(c.releaseTimer);
        c.abort.abort();
        controllers.delete(id);
      }
      set(state => ({ sessions: Object.fromEntries(Object.entries(state.sessions).filter(([id]) => !threadIds.includes(id))) }));
    },
  };
});

export function selectActiveRun(session: SessionState | undefined): RunView | null {
  const id = session?.thread?.active_run_id;
  return id && session ? session.runsById[id] ?? null : null;
}

const loadedGroupCounts = new WeakMap<SessionState['itemsById'], Map<string, number>>();

export function selectGroupHasMore(session: SessionState | undefined, groupId: string): boolean {
  if (!session) return false;
  const page = session.groupPages[groupId];
  if (page) return page.hasMore;
  const group = session.groupsById[groupId];
  if (!group || group.item_count === 0) return false;
  // Live-created groups have no page metadata. Only a known record deficit
  // warrants a paging entry; sharing the count avoids scanning for each group.
  let counts = loadedGroupCounts.get(session.itemsById);
  if (!counts) {
    counts = new Map();
    for (const item of Object.values(session.itemsById)) counts.set(item.group_id, (counts.get(item.group_id) ?? 0) + 1);
    loadedGroupCounts.set(session.itemsById, counts);
  }
  return group.item_count > (counts.get(groupId) ?? 0);
}

export function sessionCanWrite(session: SessionState | undefined): session is SessionState & { sceneRevision: Revision } {
  return session?.loadStatus === 'ready' && session.sceneRevision !== null && session.thread !== null;
}
