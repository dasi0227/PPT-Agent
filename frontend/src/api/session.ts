import { fetchClient } from './client';
import type { CommandExecution } from './commands';
import type { ContextWindowSnapshot, PlanState, Run, RunProgress, Thread } from './types';

// Revisions and sort positions stay decimal strings throughout synchronization.
export type Revision = string;
export type PublicObject = Record<string, unknown>;

export interface Revisioned {
  entity_revision: Revision;
  updated_seq: Revision;
}

export type SessionContextWindow = Omit<ContextWindowSnapshot, 'details'> & {
  context_revision?: Revision;
  plan_revision?: Revision;
  details?: ContextWindowSnapshot['details'] | null;
};

export interface ThreadView extends Omit<Thread, 'naming_revision'>, Revisioned {
  naming_revision: Revision;
  context_revision: Revision;
  plan: PlanState | null;
  plan_revision: Revision;
  plan_run_id: string | null;
  active_run_id: string | null;
  suggestions: PublicObject | null;
}

export interface RunView extends Run, Revisioned {
  scene_revision: Revision;
  progress: RunProgress | null;
  usage: PublicObject | null;
  terminal: PublicObject | null;
  terminal_ref?: string;
  context_window: SessionContextWindow | null;
  created_at: number;
  updated_at: number;
  active_duration_ms: number;
  active_since: number | null;
}

export type CommandView = Omit<CommandExecution, 'input'> & Revisioned & { input: PublicObject | null; content_ref?: string };

export interface InteractionView extends Revisioned {
  id: string;
  thread_id: string;
  run_id: string;
  call_id: string;
  kind: 'question' | 'plan' | 'command' | 'scope' | 'resource';
  state: 'pending' | 'answered' | 'applied' | 'canceled';
  request_revision: Revision;
  request: PublicObject | null;
  answer: PublicObject | null;
  detail_ref?: string;
}

export interface TimelineGroup extends Revisioned {
  id: string;
  thread_id: string;
  order_seq: Revision;
  kind: 'run' | 'command';
  run_id?: string;
  command_id?: string;
  input_id?: string;
  status: string;
  summary: PublicObject;
  item_count: number;
  // Pagination metadata belongs to a read, not an entity upsert.
  items_mode?: 'summary' | 'page';
  has_more_items: boolean;
  older_item_cursor: string | null;
}

export interface TimelineRecord extends Revisioned {
  created_seq: Revision;
  id: string;
  thread_id: string;
  group_id: string;
  position: Revision;
  kind: string;
  created_at: number;
  message_id?: string;
  tool_call_id?: string;
  interaction_id?: string;
  attempt_id?: string;
  summary: PublicObject;
  body_ref?: string;
  detail_ref?: string;
}

export interface TimelinePage {
  scene_revision: Revision;
  read_seq: Revision;
  groups: TimelineGroup[];
  items: TimelineRecord[];
  older_cursor: string | null;
  has_more: boolean;
  is_empty: boolean;
}

export interface ItemPage {
  scene_revision: Revision;
  read_seq: Revision;
  group_id: string;
  items: TimelineRecord[];
  older_cursor: string | null;
  has_more: boolean;
}

export interface SessionView {
  project_id: string;
  thread_id: string;
  scene_revision: Revision;
  seq: Revision;
  cursor: string;
  thread: ThreadView;
  active_run: RunView | null;
  active_commands: CommandView[];
  pending_interactions: InteractionView[];
  timeline: TimelinePage;
  context_window: SessionContextWindow | null;
}

export interface EntityTypes {
  thread: ThreadView;
  run: RunView;
  command: CommandView;
  interaction: InteractionView;
  'timeline.group': TimelineGroup;
  'timeline.item': TimelineRecord;
}

export type EntityKind = keyof EntityTypes;
export type SessionChange = { type: 'context_window.invalidate'; entity: { context_revision: Revision } } | {
  [K in EntityKind]: { type: `${K}.upsert`; entity: EntityTypes[K] }
    | { type: `${K}.remove`; id: string; entity_revision: Revision }
}[EntityKind];

export interface SessionBatch {
  project_id: string;
  thread_id: string;
  scene_revision: Revision;
  seq: Revision;
  commit_id: string;
  changes: SessionChange[];
}

export interface ChangePage {
  batches: SessionBatch[];
  cursor: string;
  has_more: boolean;
}

export interface CommitReceipt {
  related?: { thread_id: string; commit_id: string; cursor: string; scene_revision: Revision; seq: Revision }[];
  result?: unknown;
  commit_id: string;
  cursor: string;
  scene_revision: Revision;
  seq: Revision;
  context_revision: Revision;
  checkpoint_revision: Revision;
}

export class SessionProtocolError extends Error {
  constructor(readonly field: string) {
    super(`Invalid session response: ${field}`);
    this.name = 'SessionProtocolError';
  }
}

function requireValue(valid: unknown, field: string): asserts valid {
  if (!valid) throw new SessionProtocolError(field);
}

export function parseCommitReceipt(value: unknown): CommitReceipt {
  const row = object(value, 'receipt');
  text(row.commit_id, 'receipt.commit_id');
  cursor(row.cursor, 'receipt.cursor');
  revision(row.scene_revision, 'receipt.scene_revision', true);
  revision(row.seq, 'receipt.seq', true);
  revision(row.context_revision, 'receipt.context_revision');
  revision(row.checkpoint_revision, 'receipt.checkpoint_revision');
  if (row.related !== undefined) values(row.related, 'receipt.related').forEach(value => {
    const commit = object(value, 'receipt.related.commit');
    text(commit.thread_id, 'receipt.related.thread_id');
    text(commit.commit_id, 'receipt.related.commit_id');
    cursor(commit.cursor, 'receipt.related.cursor');
    revision(commit.scene_revision, 'receipt.related.scene_revision', true);
    revision(commit.seq, 'receipt.related.seq', true);
    requireValue(commit.scene_revision === row.scene_revision, 'receipt.related.scene');
  });
  return row as unknown as CommitReceipt;
}

export function isPublicObject(value: unknown): value is PublicObject {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

export function isRevision(value: unknown): value is Revision {
  return typeof value === 'string' && /^(0|[1-9][0-9]*)$/.test(value)
    && (value.length < 19 || (value.length === 19 && value <= '9223372036854775807'));
}

export function compareRevision(a: Revision, b: Revision): number {
  requireValue(isRevision(a) && isRevision(b), 'revision');
  return a.length !== b.length ? a.length - b.length : a === b ? 0 : a < b ? -1 : 1;
}

function object(value: unknown, field: string): PublicObject {
  requireValue(isPublicObject(value), field);
  return value;
}

function text(value: unknown, field: string): void {
  requireValue(typeof value === 'string' && value.length > 0, field);
}

function revision(value: unknown, field: string, positive = false): void {
  requireValue(isRevision(value) && (!positive || value !== '0'), field);
}

function nullableObject(value: unknown, field: string): void {
  requireValue(value === null || isPublicObject(value), field);
}

function cursor(value: unknown, field: string, nullable = false): void {
  requireValue(nullable && value === null || typeof value === 'string' && value.length > 0, field);
}

function numeric(value: unknown, field: string): void {
  requireValue(typeof value === 'number' && Number.isSafeInteger(value) && value >= 0, field);
}

function values(value: unknown, field: string): unknown[] {
  requireValue(Array.isArray(value), field);
  return value;
}

function entity(value: unknown, kind: EntityKind, threadId: string): void {
  const row = object(value, kind);
  const idKey = kind === 'command' ? 'command_id' : 'id';
  text(row[idKey], `${kind}.${idKey}`);
  requireValue((kind === 'thread' ? row.id : row.thread_id) === threadId, `${kind}.thread_id`);
  revision(row.entity_revision, `${kind}.entity_revision`, true);
  revision(row.updated_seq, `${kind}.updated_seq`);
  switch (kind) {
    case 'thread':
      text(row.project_id, 'thread.project_id');
      requireValue(typeof row.title === 'string', 'thread.title');
      revision(row.naming_revision, 'thread.naming_revision');
      revision(row.plan_revision, 'thread.plan_revision');
      revision(row.context_revision, 'thread.context_revision');
      nullableObject(row.plan, 'thread.plan');
      if (row.plan !== null) {
        const plan = object(row.plan, 'thread.plan');
        values(plan.steps, 'thread.plan.steps').forEach(step => {
          const entry = object(step, 'plan.step');
          text(entry.title, 'plan.step.title');
          requireValue(['pending', 'processing', 'completed', 'failed'].includes(String(entry.status)), 'plan.step.status');
        });
      }
      requireValue(row.active_run_id === null || typeof row.active_run_id === 'string' && row.active_run_id.length > 0, 'thread.active_run_id');
      nullableObject(row.suggestions, 'thread.suggestions');
      requireValue(typeof row.auto_suggest_enabled === 'boolean' && typeof row.auto_rename_enabled === 'boolean', 'thread.preferences');
      numeric(row.created_at, 'thread.created_at');
      numeric(row.updated_at, 'thread.updated_at');
      break;
    case 'run':
      text(row.project_id, 'run.project_id');
      revision(row.scene_revision, 'run.scene_revision', true);
      requireValue(['pending', 'running', 'waiting', 'paused', 'recovering', 'done', 'failed', 'canceled'].includes(String(row.status)), 'run.status');
      requireValue(['chat', 'ask', 'plan', 'create'].includes(String(row.mode)), 'run.mode');
      object(row.scope, 'run.scope');
      requireValue(row.model === null || typeof row.model === 'string', 'run.model');
      requireValue(typeof row.can_continue === 'boolean', 'run.can_continue');
      for (const key of ['progress', 'usage', 'terminal', 'context_window']) nullableObject(row[key], `run.${key}`);
      for (const key of ['created_at', 'updated_at', 'active_duration_ms']) numeric(row[key], `run.${key}`);
      if (row.active_since !== null) numeric(row.active_since, 'run.active_since');
      break;
    case 'command':
      text(row.attempt_id, 'command.attempt_id');
      text(row.project_id, 'command.project_id');
      requireValue(['rename', 'polish', 'handoff', 'compact', 'commit', 'suggest'].includes(String(row.kind)), 'command.kind');
      requireValue(['accepted', 'running', 'cancel_requested', 'completed', 'failed', 'canceled', 'interrupted'].includes(String(row.status)), 'command.status');
      requireValue(['user', 'automatic'].includes(String(row.source)), 'command.source');
      for (const key of ['attempt_no', 'created_at', 'updated_at']) numeric(row[key], `command.${key}`);
      requireValue(typeof row.phase === 'number' && Number.isSafeInteger(row.phase) && row.phase >= -1, 'command.phase');
      revision(row.scene_revision, 'command.scene_revision', true);
      nullableObject(row.input, 'command.input');
      break;
    case 'interaction':
      text(row.run_id, 'interaction.run_id');
      requireValue(typeof row.call_id === 'string', 'interaction.call_id');
      requireValue(['question', 'plan', 'command', 'scope', 'resource'].includes(String(row.kind)), 'interaction.kind');
      requireValue(['pending', 'answered', 'applied', 'canceled'].includes(String(row.state)), 'interaction.state');
      revision(row.request_revision, 'interaction.request_revision', true);
      nullableObject(row.request, 'interaction.request');
      nullableObject(row.answer, 'interaction.answer');
      break;
    case 'timeline.group':
      revision(row.order_seq, 'group.order_seq');
      requireValue(['run', 'command'].includes(String(row.kind)), 'group.kind');
      text(row.status, 'group.status');
      numeric(row.item_count, 'group.item_count');
      object(row.summary, 'group.summary');
      requireValue(row.items_mode === undefined || row.items_mode === 'summary' || row.items_mode === 'page', 'group.items_mode');
      requireValue(typeof row.has_more_items === 'boolean', 'group.has_more_items');
      cursor(row.older_item_cursor, 'group.older_item_cursor', true);
      break;
    case 'timeline.item':
      revision(row.created_seq, 'item.created_seq', true);
      text(row.group_id, 'item.group_id');
      revision(row.position, 'item.position', true);
      text(row.kind, 'item.kind');
      object(row.summary, 'item.summary');
      numeric(row.created_at, 'item.created_at');
      break;
  }
}

const entityKinds: readonly EntityKind[] = ['thread', 'run', 'command', 'interaction', 'timeline.group', 'timeline.item'];

export function parseSessionEntity<K extends EntityKind>(value: unknown, kind: K, threadId: string): EntityTypes[K] {
  entity(value, kind, threadId);
  return value as EntityTypes[K];
}

export function parseSessionBatch(value: unknown, threadId: string): SessionBatch {
  const batch = object(value, 'batch');
  requireValue(batch.thread_id === threadId, 'batch.thread_id');
  text(batch.project_id, 'batch.project_id');
  revision(batch.scene_revision, 'batch.scene_revision', true);
  revision(batch.seq, 'batch.seq', true);
  text(batch.commit_id, 'batch.commit_id');
  values(batch.changes, 'batch.changes').forEach(raw => {
    const change = object(raw, 'change');
    if (change.type === 'context_window.invalidate') {
      revision(object(change.entity, 'context_window').context_revision, 'context_window.context_revision');
      return;
    }
    const kind = entityKinds.find(key => change.type === `${key}.upsert` || change.type === `${key}.remove`);
    requireValue(kind, 'change.type');
    if (change.type === `${kind}.upsert`) entity(change.entity, kind, threadId);
    else { text(change.id, 'remove.id'); revision(change.entity_revision, 'remove.entity_revision', true); }
  });
  return value as SessionBatch;
}

function pageBase(row: PublicObject): void {
  revision(row.scene_revision, 'page.scene_revision', true);
  revision(row.read_seq, 'page.read_seq');
  cursor(row.older_cursor, 'page.older_cursor', true);
  requireValue(typeof row.has_more === 'boolean', 'page.has_more');
  requireValue(!row.has_more || row.older_cursor !== null, 'page.continuation');
}

export function parseTimelinePage(value: unknown, threadId: string): TimelinePage {
  const page = object(value, 'timeline');
  pageBase(page);
  requireValue(typeof page.is_empty === 'boolean', 'timeline.is_empty');
  const groups = values(page.groups, 'timeline.groups');
  const groupIds = new Set<string>();
  groups.forEach(raw => {
    entity(raw, 'timeline.group', threadId);
    const group = raw as TimelineGroup;
    requireValue(group.items_mode !== undefined && !groupIds.has(group.id), 'group.page');
    requireValue(group.items_mode === 'summary' || !group.has_more_items || group.older_item_cursor !== null, 'group.continuation');
    groupIds.add(group.id);
  });
  const itemIds = new Set<string>();
  values(page.items, 'timeline.items').forEach(raw => {
    entity(raw, 'timeline.item', threadId);
    const item = raw as TimelineRecord;
    requireValue(groupIds.has(item.group_id) && !itemIds.has(item.id), 'item.page');
    itemIds.add(item.id);
  });
  requireValue(!page.is_empty || groups.length === 0 && itemIds.size === 0 && !page.has_more, 'timeline.is_empty');
  return value as TimelinePage;
}

export function parseSessionView(value: unknown, threadId: string): SessionView {
  const view = object(value, 'view');
  text(view.project_id, 'view.project_id');
  requireValue(view.thread_id === threadId, 'view.thread_id');
  revision(view.scene_revision, 'view.scene_revision', true);
  revision(view.seq, 'view.seq');
  cursor(view.cursor, 'view.cursor');
  entity(view.thread, 'thread', threadId);
  const thread = view.thread as ThreadView;
  requireValue(thread.project_id === view.project_id, 'thread.project_id');
  if (view.active_run !== null) {
    entity(view.active_run, 'run', threadId);
    const run = view.active_run as RunView;
    requireValue(run.scene_revision === view.scene_revision && run.project_id === view.project_id, 'run.scene');
    requireValue(run.id === thread.active_run_id && !['done', 'failed', 'canceled'].includes(run.status), 'view.active_run');
  } else requireValue(thread.active_run_id === null, 'view.active_run');
  values(view.active_commands, 'view.active_commands').forEach(command => entity(command, 'command', threadId));
  values(view.pending_interactions, 'view.pending_interactions').forEach(interaction => entity(interaction, 'interaction', threadId));
  for (const raw of [view.thread, view.active_run, ...(view.active_commands as unknown[]), ...(view.pending_interactions as unknown[])]) {
    if (raw === null) continue;
    const row = raw as Revisioned & { project_id?: string };
    requireValue(compareRevision(row.updated_seq, view.seq as string) <= 0, 'view.updated_seq');
    requireValue(row.project_id === undefined || row.project_id === view.project_id, 'view.entity.project_id');
  }
  const timeline = parseTimelinePage(view.timeline, threadId);
  requireValue(timeline.scene_revision === view.scene_revision && timeline.read_seq === view.seq, 'view.timeline');
  const hasVisibleCommand = (view.active_commands as CommandView[]).some(command => command.kind !== 'rename' && command.kind !== 'suggest');
  requireValue(!timeline.is_empty || view.active_run === null && !hasVisibleCommand && (view.pending_interactions as unknown[]).length === 0, 'view.is_empty');
  nullableObject(view.context_window, 'view.context_window');
  return value as SessionView;
}

function base(threadId: string): string { return `/threads/${encodeURIComponent(threadId)}`; }
const options = (signal?: AbortSignal) => ({ signal, reportError: false, cache: 'no-store' as const });

export const sessionApi = {
  async view(threadId: string, signal?: AbortSignal): Promise<SessionView> {
    return parseSessionView(await fetchClient<unknown>(`${base(threadId)}/view`, options(signal)), threadId);
  },
  async timeline(threadId: string, scene: Revision, before: string, signal?: AbortSignal): Promise<TimelinePage> {
    const query = new URLSearchParams({ scene_revision: scene, before });
    return parseTimelinePage(await fetchClient<unknown>(`${base(threadId)}/timeline?${query}`, options(signal)), threadId);
  },
  async items(threadId: string, scene: Revision, groupId: string, before: string | null, signal?: AbortSignal): Promise<ItemPage> {
    const query = new URLSearchParams({ scene_revision: scene });
    if (before) query.set('before', before);
    const value = await fetchClient<unknown>(`${base(threadId)}/timeline/groups/${encodeURIComponent(groupId)}/items?${query}`, options(signal));
    const page = object(value, 'items');
    pageBase(page);
    requireValue(page.group_id === groupId, 'items.group_id');
    const ids = new Set<string>();
    values(page.items, 'items.items').forEach(raw => {
      entity(raw, 'timeline.item', threadId);
      const item = raw as TimelineRecord;
      requireValue(item.group_id === groupId && !ids.has(item.id), 'items.item');
      ids.add(item.id);
    });
    return value as ItemPage;
  },
  async changes(threadId: string, after: string, signal?: AbortSignal): Promise<ChangePage> {
    const value = await fetchClient<unknown>(`${base(threadId)}/changes?${new URLSearchParams({ cursor: after })}`, options(signal));
    const page = object(value, 'changes');
    cursor(page.cursor, 'changes.cursor');
    requireValue(typeof page.has_more === 'boolean', 'changes.has_more');
    const batches = values(page.batches, 'changes.batches').map(batch => parseSessionBatch(batch, threadId));
    requireValue(!page.has_more || batches.length > 0, 'changes.progress');
    return { batches, cursor: page.cursor as string, has_more: page.has_more as boolean };
  },
  async detail(threadId: string, itemId: string, ref: string, signal?: AbortSignal): Promise<PublicObject> {
    return object(await fetchClient<unknown>(`${base(threadId)}/timeline/items/${encodeURIComponent(itemId)}/detail?${new URLSearchParams({ ref })}`, options(signal)), 'detail');
  },
};
