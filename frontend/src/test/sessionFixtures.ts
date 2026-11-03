import type { CommitReceipt, InteractionView, CommandView, RunView, SessionView, ThreadView, TimelineGroup, TimelineRecord } from '../api/session';
import type { SessionState } from '../stores/sessionStore';

export const threadFixture = (overrides: Partial<ThreadView> = {}): ThreadView => ({
  id: 't1', project_id: 'p1', title: '会话', auto_rename_enabled: true, auto_suggest_enabled: true,
  naming_revision: '1', entity_revision: '1', context_revision: '0', updated_seq: '1',
  plan: null, plan_revision: '0', plan_run_id: null, active_run_id: null, suggestions: null,
  created_at: 1000, updated_at: 1000, ...overrides,
});
export const runFixture = (overrides: Partial<RunView> = {}): RunView => ({
  id: 'r1', thread_id: 't1', project_id: 'p1', scene_revision: '1', entity_revision: '1', updated_seq: '1',
  status: 'running', mode: 'create', model: 'model', can_continue: false,
  scope: { slide_ids: [], source: { kind: 'all_pages' }, include_run_created_slides: true, revision: '1' },
  progress: null, usage: null, terminal: null, context_window: null, active_duration_ms: 2000, active_since: null,
  created_at: 1000, updated_at: 1000, ...overrides,
});
export const groupFixture = (overrides: Partial<TimelineGroup> = {}): TimelineGroup => ({
  id: 'run:r1', thread_id: 't1', kind: 'run', run_id: 'r1', status: 'running', order_seq: '1',
  entity_revision: '1', updated_seq: '1', summary: {}, item_count: 1, items_mode: 'page',
  has_more_items: false, older_item_cursor: null, ...overrides,
});
export const itemFixture = (overrides: Partial<TimelineRecord> = {}): TimelineRecord => ({
  id: 'input:i1', thread_id: 't1', group_id: 'run:r1', kind: 'user_turn', position: '1',
  summary: { text: '生成演示文稿', mode: 'create', clientMessageId: 'req1', steering: false }, created_seq: '1',
  entity_revision: '1', updated_seq: '1', created_at: 1000, ...overrides,
});
export const commandFixture = (overrides: Partial<CommandView> = {}): CommandView => ({
  command_id: 'cmd1', attempt_id: 'attempt1', attempt_no: 1, thread_id: 't1', project_id: 'p1',
  scene_revision: '1', entity_revision: '1', updated_seq: '1', kind: 'polish', source: 'user',
  status: 'accepted', phase: -1, input: {}, created_at: 1000, updated_at: 1000, ...overrides,
});
export const viewFixture = (overrides: Partial<SessionView> = {}): SessionView => ({
  project_id: 'p1', thread_id: 't1', scene_revision: '1', seq: '1', cursor: 'cursor1',
  thread: threadFixture(), active_run: null, active_commands: [], pending_interactions: [], context_window: null,
  timeline: { scene_revision: '1', read_seq: '1', groups: [], items: [], has_more: false, older_cursor: null, is_empty: true },
  ...overrides,
});
export const sessionFixture = (overrides: Partial<SessionState> = {}): SessionState => ({
  projectId: 'p1', threadId: 't1', generation: 0, loadStatus: 'ready', connectionStatus: 'closed',
  sceneRevision: '1', appliedSeq: '1', snapshotSeq: '1', cursor: 'cursor1', thread: threadFixture(),
  runsById: {}, commandsById: {}, interactionsById: {}, groupsById: {}, itemsById: {}, orderedGroupIds: [],
  removedVersions: {}, groupPages: {}, detailsByRef: {}, detailStates: {}, optimisticInputs: {},
  olderCursor: null, hasMore: false, isEmpty: true, historyPage: { status: 'idle', error: null }, contextWindow: null, error: null,
  ...overrides,
});

export const receiptFixture = (overrides: Partial<CommitReceipt> = {}): CommitReceipt => ({
  commit_id: 'commit2', cursor: 'cursor2', scene_revision: '1', seq: '2', context_revision: '0', checkpoint_revision: '1', ...overrides,
});
export const interactionFixture = (overrides: Partial<InteractionView> = {}): InteractionView => ({
  id: 'q1', run_id: 'r1', thread_id: 't1', call_id: 'c1', kind: 'question', state: 'pending',
  request_revision: '1', entity_revision: '1', updated_seq: '1', request: { questions: [] }, answer: null, ...overrides,
});
