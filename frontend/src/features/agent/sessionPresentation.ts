import { compareRevision, isPublicObject, type CommandView, type InteractionView, type PublicObject, type TimelineGroup, type TimelineRecord } from '../../api/session';
import type { CommandActivityRecord, CreateRunScopeInput, PlanState, RunMode, RunProgress, RunScope } from '../../api/types';
import { selectActiveRun, type SessionLoadStatus, type SessionState } from '../../stores/sessionStore';
import { contextCompactionTimelineItem, type TimelineItem, type ToolActivityItem } from './timelineTypes';
import type { NextInputSuggestionsState } from './nextInputSuggestions';
import type { RunClock } from './runClock';
import type { RunTokenUsageByRun } from './runTokenUsage';
import type { CommandStatus } from '../../stores/commandRuntime';

export type RunStatus = 'idle' | 'creating' | 'running' | 'waiting' | 'paused' | 'recovering' | 'canceling' | 'done' | 'error' | 'canceled';

export interface RunSession {
  loadStatus: SessionLoadStatus;
  isEmpty: boolean;
  error: Error | null;
  projectId: string | null;
  activeRunId: string | null;
  activeRunModel: string | null;
  status: RunStatus;
  scope: RunScope | CreateRunScopeInput;
  mode: RunMode;
  originalRequest?: { mode: RunMode };
  timelineItems: TimelineItem[];
  groups: TimelineGroup[];
  pendingQuestion: { id: string } | null;
  progress: RunProgress | null;
  plan: PlanState | null;
  planRevision: string;
  nextInputSuggestions: NextInputSuggestionsState | null;
  tokenUsageByRun: RunTokenUsageByRun;
  runClock?: RunClock;
}

export const UNLOADED_SESSION: RunSession = Object.freeze({
  loadStatus: 'uninitialized', isEmpty: false, error: null, projectId: null,
  activeRunId: null, activeRunModel: null, status: 'idle', scope: { selection: { kind: 'current_page' } },
  mode: 'create', timelineItems: [], groups: [], pendingQuestion: null, progress: null,
  plan: null, planRevision: '0', nextInputSuggestions: null, tokenUsageByRun: {},
} as RunSession);

const string = (value: unknown, fallback = '') => typeof value === 'string' ? value : fallback;
const number = (value: unknown) => typeof value === 'number' && Number.isFinite(value) ? value : 0;
const object = (value: unknown): PublicObject => isPublicObject(value) ? value : {};
const array = <T,>(value: unknown): T[] => Array.isArray(value) ? value as T[] : [];

function commandItem(record: TimelineRecord, row: CommandView): TimelineItem | null {
  if (row.kind === 'rename' || row.kind === 'suggest') return null;
  const result = object(row.result ?? row.latest_success?.result);
  const status: CommandStatus = row.status === 'completed' ? 'completed' : row.status === 'canceled' ? 'canceled'
    : row.status === 'failed' || row.status === 'interrupted' ? 'failed' : 'loading';
  const base = { id: row.command_id, groupId: record.group_id, timestamp: row.created_at, status,
    commandSource: row.source, commandError: row.error?.message,
    phase: row.phase, cancellable: status === 'loading' && row.status !== 'cancel_requested',
    runId: row.source === 'automatic' ? row.run_id : undefined };
  if (row.kind === 'commit') return {
    ...base, type: 'git_commit', operationId: row.command_id, title: result.empty ? '当前项目没有可提交的变更' : string(result.title, '提交项目版本'),
    items: array<string>(result.items), hash: string(result.hash), branch: string(result.branch),
    filesChanged: number(result.files_changed), insertions: number(result.insertions), deletions: number(result.deletions), retryable: row.error?.retryable,
  };
  const commandRecord: CommandActivityRecord = {
    id: row.command_id, thread_id: row.thread_id, project_id: row.project_id,
    kind: row.kind, method: row.source === 'automatic' ? 'auto' : row.input?.mode === 'manual' || row.kind === 'compact' ? 'manual' : 'auto',
    status, phase: row.phase, previous_title: row.previous_title ?? '', request: row.input ?? {},
    result, created_at: row.created_at, updated_at: row.updated_at,
  };
  const activity = { ...base, commandRecord };
  if (row.kind === 'handoff') {
    const briefing = object(result.briefing);
    return { ...activity, type: 'briefing', kind: 'handoff', briefingId: string(briefing.briefing_id),
      versions: array<import('../../api/types').BriefingVersion>(briefing.versions).slice(-1) };
  }
  if (row.kind === 'compact' && status === 'completed' && isPublicObject(result.compaction)) {
    return { ...contextCompactionTimelineItem(result.compaction as unknown as import('../../api/types').ContextCompaction), ...activity };
  }
  return { ...activity, type: 'command', kind: row.kind, title: string(result.title, row.kind === 'compact' ? '压缩上下文' : '润色输入内容'),
    content: string(result.content), method: commandRecord.method };
}

function interactionItem(record: TimelineRecord, row: InteractionView, runId: string): TimelineItem {
  const request = object(row.request), answer = row.answer;
  const base = { id: record.id, groupId: record.group_id, timestamp: record.created_at, runId,
    requestRevision: row.request_revision, interactionState: row.state,
    detailRef: record.detail_ref ?? row.detail_ref };
  if (!row.request) return { ...base, type: 'deferred', detailRef: record.detail_ref ?? row.detail_ref };
  const display = { ...base, detailRef: undefined };
  switch (row.kind) {
    case 'question': return { ...display, type: 'question', questionId: row.id,
      questions: array(request.questions), answer: answer?.answer as import('../../api/types').QuestionAnswer | undefined };
    case 'plan': return { ...display, type: 'plan_approval', interactionId: row.id,
      plan: request.plan as import('../../api/types').PlanProposal,
      answer: answer as { decision: 'approve' | 'refuse'; feedback?: string } | undefined ?? undefined };
    case 'scope': return { ...display, type: 'scope_expansion', interactionId: row.id, callId: row.call_id,
      baseRevision: string(request.base_revision), currentScope: request.current_scope as RunScope,
      requestedAddition: object(request.requested_addition), proposedScope: request.proposed_scope as RunScope,
      affectedPageCount: number(request.affected_page_count), reason: string(request.reason),
      answer: answer ? { decision: answer.decision as 'approve' | 'refuse' | 'revise', feedback: string(answer.feedback), appliedScope: answer.applied_scope as RunScope | undefined } : undefined };
    case 'command': return { ...display, type: 'tool', callId: row.call_id, tool: 'run_command', label: '执行命令',
      status: row.state === 'canceled' ? 'canceled' : 'running',
      command: { text: string(request.command), stdout_preview: string(request.stdout), stderr_preview: string(request.stderr) },
      commandApproval: { interactionId: row.id, commandHash: string(request.command_hash), changes: array(request.changes),
        answer: answer ? { decision: answer.decision as 'allow_once' | 'deny', feedback: string(answer.feedback) } : undefined } };
    case 'resource': {
      const resource = request.resource as import('../../api/types').ApprovalResource;
      const labels = { meta: '基本信息', constraints: '全局约束', outline: '目录结构' };
      return { ...display, type: 'tool', callId: row.call_id, tool: `edit_${resource}`, label: `编辑${labels[resource]}`,
        status: row.state === 'canceled' ? 'canceled' : 'running', target: request.target as ToolActivityItem['target'],
        approval: { interactionId: row.id, resource, revision: row.request_revision, target: request.target as import('../../api/types').PublicTarget,
          answer: answer ? { decision: answer.decision as 'approve' | 'reject', feedback: string(answer.feedback) } : undefined } };
    }
  }
}

function displayRecord(session: SessionState, record: TimelineRecord): TimelineItem | null {
  const ref = record.detail_ref ?? record.body_ref;
  const data = ref && session.detailsByRef[ref] || record.summary;
  const group = session.groupsById[record.group_id];
  const runId = group?.run_id ?? (string(data.run_id ?? data.runId) || undefined);
  const base = { id: record.id, groupId: record.group_id, runId, timestamp: record.created_at,
    detailRef: ref && !session.detailsByRef[ref] ? ref : undefined };
  const terminal = object(group?.summary.terminal);
  if (record.kind === 'command') {
    const stored = data as unknown as CommandView;
    const current = session.commandsById[stored.command_id];
    const row = current && compareRevision(current.entity_revision, stored.entity_revision) > 0 ? current : stored;
    if (row.content_ref && !session.detailsByRef[row.content_ref]) return { ...base, type: 'deferred', detailRef: row.content_ref };
    const detail = row.content_ref && session.detailsByRef[row.content_ref];
    return commandItem(record, detail ? detail as unknown as CommandView : row);
  }
  if (record.kind === 'interaction') {
    const stored = data as unknown as InteractionView;
    const current = session.interactionsById[stored.id];
    const row = current && compareRevision(current.entity_revision, stored.entity_revision) > 0 ? current : stored;
    const detail = row.detail_ref && session.detailsByRef[row.detail_ref];
    return interactionItem(record, detail ? detail as unknown as InteractionView : row, runId ?? row.run_id);
  }
  switch (record.kind) {
    case 'user_turn': return { ...base, type: 'user_turn', text: string(data.text ?? data.excerpt),
      scope: data.scope as RunScope | CreateRunScopeInput | undefined, mode: string(data.mode),
      skills: array(data.skills), components: array(data.components), domSelections: array(data.domSelections), referenceOrder: array(data.referenceOrder),
      clientMessageId: string(data.clientMessageId) || undefined,
      steering: data.steering === true,
      deliveryStatus: data.deliveryStatus === 'rejected' ? 'rejected' : 'accepted' };
    case 'thinking': case 'milestone': return { ...base, type: record.kind, messageId: record.message_id ?? string(data.message_id), text: string(data.text ?? data.excerpt) };
    case 'message': return { ...base, type: 'input_reply', messageId: record.message_id ?? string(data.message_id), text: string(data.text ?? data.excerpt), affectedTargets: array(data.affected_targets) };
    case 'final': return { ...base, type: 'final', messageId: record.message_id ?? string(data.message_id), text: string(data.text ?? data.excerpt),
      affectedTargets: array(terminal.affected_targets ?? data.affected_targets), durationMs: number(group?.summary.active_duration_ms) };
    case 'tool': {
      const display = object(data.display);
      return { ...base, type: 'tool', callId: record.tool_call_id ?? string(data.call_id), tool: string(data.tool),
        status: data.status as ToolActivityItem['status'], label: string(display.label, string(data.tool)), detail: string(display.detail) || undefined,
        target: data.target as ToolActivityItem['target'], changes: array(data.changes),
        preview: data.preview as ToolActivityItem['preview'], previewAssessment: data.preview_assessment as ToolActivityItem['previewAssessment'],
        image: data.image as ToolActivityItem['image'], review: data.review as ToolActivityItem['review'], error: data.error as ToolActivityItem['error'],
        command: data.command as ToolActivityItem['command'], resources: array(data.resources) };
    }
    case 'compaction':
      return base.detailRef ? { ...base, type: 'deferred' } : { ...contextCompactionTimelineItem(data.compaction as import('../../api/types').ContextCompaction), ...base };
    case 'terminal_notice': {
      if (group?.status === 'done') return null;
      const error = data.error as import('../../api/types').PublicError | undefined;
      const reason = data.reason as import('../../api/types').RunCancelReason | undefined;
      const status = group?.status === 'canceled' ? 'canceled' : 'failed';
      const reply = object(data.reply);
      return { ...base, type: 'terminal_notice', status, error, reason, affectedTargets: array(data.affected_targets),
        durationMs: number(data.duration_ms), traceId: string(data.trace_id),
        message: error?.message ?? (status === 'canceled' ? reason === 'superseded' ? '此前任务因服务中断而结束。'
          : reason === 'input_rejected' ? '请求被拒绝，任务已停止。' : reason === 'input_detection_failed' ? '请求识别失败，任务已停止。'
            : reason === 'user_requested' ? '你已停止本次任务。' : '任务已停止。' : '任务未能完成全部要求。'),
        reply: typeof reply.text === 'string' ? { messageId: string(reply.message_id ?? reply.input_id), text: reply.text } : undefined };
    }
    default: return { ...base, type: 'deferred' };
  }
}

const cache = new WeakMap<SessionState, RunSession>();
export function presentSession(source: SessionState): RunSession {
  const cached = cache.get(source);
  if (cached) return cached;
  const groups = source.orderedGroupIds.map(id => source.groupsById[id]).filter(Boolean);
  const order = new Map(groups.map((group, index) => [group.id, index]));
  const records = Object.values(source.itemsById).filter(item => order.has(item.group_id))
    .sort((a, b) => (order.get(a.group_id)! - order.get(b.group_id)!) || compareRevision(a.position, b.position) || (a.id < b.id ? -1 : 1));
  const timelineItems: TimelineItem[] = records.flatMap(record => {
    const item = displayRecord(source, record);
    return item ? [{ ...item, createdSeq: record.created_seq, sceneRevision: source.sceneRevision ?? undefined }] : [];
  });
  for (const row of Object.values(source.commandsById)) {
    if (!['accepted', 'running', 'cancel_requested'].includes(row.status)
      || records.some(item => item.id === `command:${row.command_id}`)) continue;
    const record: TimelineRecord = { id: `command:${row.command_id}`, thread_id: source.threadId,
      group_id: `command:${row.command_id}`, kind: 'command', position: '1', created_at: row.created_at,
      summary: row as unknown as PublicObject, entity_revision: row.entity_revision,
      created_seq: row.updated_seq, updated_seq: row.updated_seq, detail_ref: row.content_ref };
    const item = displayRecord(source, record);
    if (item) timelineItems.push({ ...item, createdSeq: record.created_seq, sceneRevision: source.sceneRevision ?? undefined });
  }
  for (const row of Object.values(source.interactionsById)) {
    if (!['pending', 'answered'].includes(row.state) || records.some(item => item.interaction_id === row.id)) continue;
    const id = `interaction:${row.id}`;
    const record = source.itemsById[id] ?? { id, thread_id: source.threadId, group_id: `run:${row.run_id}`, kind: 'interaction',
      position: '0', created_at: source.runsById[row.run_id]?.updated_at ?? 0, summary: row as unknown as PublicObject,
      entity_revision: row.entity_revision, created_seq: row.updated_seq, updated_seq: row.updated_seq, detail_ref: row.detail_ref };
    const resolved = row.detail_ref ? source.detailsByRef[row.detail_ref] as unknown as InteractionView | undefined : undefined;
    timelineItems.push({ ...interactionItem(record, resolved || row, row.run_id),
      createdSeq: record.created_seq, sceneRevision: source.sceneRevision ?? undefined });
  }
  // Join separately saved tool and approval entities by their stable call ID.
  // Deferred approvals stay separate so their own detail reference can be read.
  const toolIndex = new Map<string, number>();
  timelineItems.forEach((item, index) => { if (item.type === 'tool' && !item.interactionState) toolIndex.set(`${item.runId}:${item.callId}`, index); });
  const joined = new Set<string>();
  for (const item of timelineItems) {
    if (item.type !== 'tool' || !item.interactionState) continue;
    const index = toolIndex.get(`${item.runId}:${item.callId}`);
    if (index === undefined) continue;
    const tool = timelineItems[index] as ToolActivityItem;
    timelineItems[index] = { ...tool, approval: item.approval, commandApproval: item.commandApproval,
      requestRevision: item.requestRevision, interactionState: item.interactionState,
      command: tool.command ?? item.command, target: tool.target ?? item.target };
    joined.add(item.id);
  }
  const items = timelineItems.filter(item => !joined.has(item.id));
  for (const input of Object.values(source.optimisticInputs)) items.push({
    id: `pending:${input.clientMessageId}`, type: 'user_turn', text: input.text, timestamp: input.createdAt,
    runId: input.runId, clientMessageId: input.clientMessageId, scope: input.request?.scope, mode: input.request?.mode,
    steering: input.request === undefined,
    domSelections: input.request?.dom_selections?.map(({ selection_id, marker_no, comment, status }) => ({ selection_id, marker_no, comment, status })),
    referenceOrder: input.request?.reference_order, deliveryStatus: input.status === 'error' ? 'rejected' : input.status,
  });
  const run = selectActiveRun(source);
  const status: RunStatus = run ? run.cancel_requested_at ? 'canceling' : run.status === 'pending' ? 'running' : run.status === 'failed' ? 'error' : run.status
    : Object.values(source.optimisticInputs).some(input => input.status === 'sending' && !input.runId) ? 'creating' : 'idle';
  const firstInput = items.find(item => item.type === 'user_turn' && item.runId === run?.id && item.mode);
  const pending = Object.values(source.interactionsById).find(row => row.run_id === run?.id && row.kind === 'question' && row.state === 'pending');
  const tokenUsageByRun: RunTokenUsageByRun = {};
  for (const group of groups) {
    if (!group.run_id || !isPublicObject(group.summary.usage)) continue;
    const usage = group.summary.usage;
    tokenUsageByRun[group.run_id] = { calls: number(usage.calls), unknown_calls: number(usage.unreported_calls),
      input_tokens: number(usage.input_tokens), output_tokens: number(usage.output_tokens), total_tokens: number(usage.total_tokens) };
  }
  const requests = array<string>(source.thread?.suggestions?.requests);
  const suggestion = Object.values(source.commandsById).find(command => command.kind === 'suggest'
    && ['accepted', 'running', 'cancel_requested'].includes(command.status));
  const view: RunSession = {
    loadStatus: source.loadStatus, isEmpty: source.isEmpty && items.length === 0, error: source.error,
    projectId: source.projectId, activeRunId: run?.id ?? null, activeRunModel: run?.model ?? null, status,
    scope: run?.scope ?? UNLOADED_SESSION.scope, mode: run?.mode ?? 'create',
    originalRequest: firstInput?.type === 'user_turn' && firstInput.mode ? { mode: firstInput.mode as RunMode } : undefined,
    timelineItems: items, groups, pendingQuestion: pending ? { id: pending.id } : null,
    progress: status === 'canceling' ? { activity: 'run.canceling' } : status === 'recovering' ? { activity: 'run.recovering' }
      : status === 'running' ? run?.progress ?? null : null,
    plan: source.thread?.plan ? { ...source.thread.plan, eventRunId: source.thread.plan_run_id ?? undefined } : null,
    planRevision: source.thread?.plan_revision ?? '0',
    nextInputSuggestions: !run && (suggestion || requests.length) ? {
      runId: suggestion?.run_id ?? string(source.thread?.suggestions?.run_id),
      attemptId: suggestion?.attempt_id ?? string(source.thread?.suggestions?.attempt_id),
      items: requests, status: suggestion ? 'pending' : 'eligible',
    } : null,
    tokenUsageByRun, runClock: run ? { elapsedMs: run.active_duration_ms, runningSince: run.active_since } : undefined,
  };
  cache.set(source, view);
  return view;
}
