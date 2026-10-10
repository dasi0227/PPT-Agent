import type { CommandProgress } from '../../stores/commandRuntime';
import {
  CommandProjection,
  PlanState,
  PlanStep,
  PlanStepStatus,
  PublicError,
  BriefingKind,
  BriefingVersion,
  ContextCompaction,
  PublicLoadedResource,
  PublicTarget,
  QuestionAnswer,
  QuestionField,
  PublicSkill,
  SSEEvent,
  ToolPreview,
  ToolReadImage,
  ReviewResult,
  RunScope,
  CreateRunScopeInput,
  ApprovalResource,
} from '../../api/types';

export type TimelineItemType =
  | 'user_turn'
  | 'run_lifecycle'
  | 'reasoning'
  | 'milestone'
  | 'final'
  | 'tool'
  | 'question'
  | 'plan_approval'
  | 'command_permission'
  | 'scope_expansion'
  | 'git_commit'
  | 'command'
  | 'briefing'
  | 'context_compaction'
  | 'terminal_notice';

export interface BaseTimelineItem {
  commandError?: string;
  commandSource?: 'user' | 'automatic';
  commandRecord?: import('../../api/types').CommandActivityRecord;
  id: string;
  type: TimelineItemType;
  timestamp: number;
  runId?: string;
}

export interface UserTurnItem extends BaseTimelineItem {
  type: 'user_turn';
  text: string;
  scope?: RunScope | CreateRunScopeInput;
  mode?: string;
  skills?: PublicSkill[];
  components?: PublicLoadedResource[];
  deliveryStatus?: 'sending' | 'accepted' | 'rejected';
  clientMessageId?: string;
  sourceMessageIds?: string[];
  rejectionCode?: string;
  domSelections?: import('../../api/types').PublicDOMSelection[];
  referenceOrder?: import('../../api/types').ReferenceOrderItem[];
}

export interface ReasoningItem extends BaseTimelineItem {
  type: 'reasoning';
  messageId: string;
  text: string;
}

export interface RunLifecycleItem extends BaseTimelineItem {
  type: 'run_lifecycle';
  state: 'resumed';
  text: string;
}

export interface MilestoneItem extends BaseTimelineItem {
  type: 'milestone';
  messageId: string;
  text: string;
  completedStepIds: string[];
}

export interface FinalMessageItem extends BaseTimelineItem {
  type: 'final';
  messageId: string;
  text: string;
  affectedTargets: PublicTarget[];
  durationMs?: number;
}

export interface ToolActivityItem extends BaseTimelineItem {
  contentPrecheck?: import('../../api/types').ContentPrecheck[];
  type: 'tool';
  callId: string;
  tool: string;
  planStepId?: string;
  target?: PublicTarget;
  changes?: PublicTarget[];
  label: string;
  detail?: string;
  status: 'running' | 'completed' | 'blocked' | 'failed';
  preview?: ToolPreview;
  image?: ToolReadImage;
  review?: ReviewResult;
  error?: PublicError;
  command?: CommandProjection;
  resources?: PublicLoadedResource[];
  approval?: { interactionId: string; resource: ApprovalResource; revision: number; target: PublicTarget; answer?: { decision: 'approve' | 'reject'; feedback?: string } };
}

export interface QuestionItem extends BaseTimelineItem {
  type: 'question';
  questionId: string;
  header?: string;
  questions: QuestionField[];
  answer?: QuestionAnswer;
  displayText?: string;
}

export interface PlanApprovalItem extends BaseTimelineItem {
  type: 'plan_approval'; interactionId: string; plan: PlanState;
  answer?: { decision: 'approve' | 'refuse'; feedback?: string };
}

export interface CommandPermissionItem extends BaseTimelineItem {
  type: 'command_permission';
  interactionId: string;
  callId: string;
  command: string;
  commandHash: string;
  reasonCode: string;
  reason: string;
  answer?: { decision: 'allow_once' | 'deny'; feedback?: string };
}

export interface ScopeExpansionItem extends BaseTimelineItem {
  type: 'scope_expansion';
  interactionId: string;
  callId: string;
  baseRevision: number;
  currentScope: RunScope;
  requestedAddition: { slide_ids?: string[] };
  proposedScope: RunScope;
  affectedPageCount: number;
  reason: string;
  answer?: { decision: 'approve' | 'refuse' | 'revise'; appliedScope?: RunScope; feedback?: string };
}

export interface TerminalNoticeItem extends BaseTimelineItem {
  type: 'terminal_notice';
  status: 'failed' | 'canceled' | 'error';
  error?: PublicError | null;
  message: string;
  affectedTargets: PublicTarget[];
  durationMs?: number;
  traceId?: string | null;
  technicalMessage?: string;
  requestId?: string;
  retryable?: boolean;
  reason?: 'user_requested' | 'superseded';
}

export interface GitCommitTimelineItem extends BaseTimelineItem {
phase?: number;
cancellable?: boolean;
  type: 'git_commit';
  operationId: string;
  status: 'loading' | 'completed' | 'failed' | 'canceled';
  title?: string;
  items?: string[];
  branch?: string;
  hash?: string;
  filesChanged?: number;
  insertions?: number;
  deletions?: number;
  retryable?: boolean;
}

export interface BriefingTimelineItem extends BaseTimelineItem, CommandProgress {
  type: 'briefing';
  briefingId: string;
  kind: BriefingKind;
  versions: BriefingVersion[];
}

export interface ContextCompactionTimelineItem extends BaseTimelineItem {
  type: 'context_compaction';
  compactionId: string;
  trigger: 'auto' | 'manual';
  title: string;
  content: string;
  beforeTokens: number;
  afterTokens: number;
  maxTokens: number;
  reclaimedTokens: number;
  durationMs: number;
}

export interface CommandTimelineItem extends BaseTimelineItem, CommandProgress {
  type: 'command';
  kind: 'rename' | 'polish' | 'compact';
  title: string;
  content?: string;
  method?: 'manual' | 'auto';
}

export type TimelineItem =
  | CommandTimelineItem
  | UserTurnItem
  | RunLifecycleItem
  | ReasoningItem
  | MilestoneItem
  | FinalMessageItem
  | ToolActivityItem
  | QuestionItem
  | PlanApprovalItem
  | CommandPermissionItem
  | ScopeExpansionItem
  | GitCommitTimelineItem
  | BriefingTimelineItem
  | ContextCompactionTimelineItem
  | TerminalNoticeItem;

function normalizeStepStatus(status: unknown): PlanStepStatus {
  if (status === 'completed' || status === 'failed' || status === 'processing') return status;
  return 'pending';
}

export function reducePlan(prev: PlanState | null, event: SSEEvent): PlanState | null {
	if (event.event !== 'plan.updated' && event.event !== 'plan.approval_requested') return prev;
	const plan = event.data.plan;
  const eventSequence = event.id ? Number(event.id) : undefined;
  if (prev?.eventRunId === event.data.run_id && eventSequence !== undefined && prev.eventSequence !== undefined && eventSequence <= prev.eventSequence) return prev;
  const steps: PlanStep[] = plan.steps.map((step) => ({
    id: String(step.id ?? ''),
    title: String(step.title ?? ''),
    status: normalizeStepStatus(step.status),
  }));
  return {
    id: String(plan.plan_id),
		title: String(plan.title ?? '执行计划'), content: String(plan.content ?? ''),
		status: (String(plan.status) as PlanState['status']),
    eventRunId: event.data.run_id,
    eventSequence,
    steps,
  };
}

function timestampOf(event: SSEEvent): number {
  const timestamp = Date.parse(event.data.occurred_at);
  return Number.isNaN(timestamp) ? Date.now() : timestamp;
}

function eventItemId(event: SSEEvent, fallback: string): string {
  return event.id ? `${event.data.run_id}:${event.id}` : `${event.data.run_id}:${fallback}`;
}

export function upsertTimelineItem(state: TimelineItem[], item: TimelineItem): TimelineItem[] {
  const index = state.findIndex((candidate) => candidate.id === item.id);
  if (index < 0) return [...state, item];
  const copy = state.slice();
  copy[index] = item;
  return copy;
}

export function contextCompactionTimelineItem(
  compaction: ContextCompaction,
): ContextCompactionTimelineItem {
  return {
    id: `context-compaction:${compaction.id}`,
    type: 'context_compaction',
    compactionId: compaction.id,
    ...(compaction.trigger === 'auto' && compaction.run_id ? { runId: compaction.run_id } : {}),
    commandSource: compaction.trigger === 'auto' ? 'automatic' : 'user',
    trigger: compaction.trigger,
    title: compaction.title,
    content: compaction.content,
    beforeTokens: compaction.before_tokens,
    afterTokens: compaction.after_tokens,
    maxTokens: compaction.max_tokens,
    reclaimedTokens: compaction.reclaimed_tokens,
    durationMs: compaction.duration_ms,
    timestamp: compaction.created_at * 1000,
  };
}

function settleInterruptedTools(state: TimelineItem[], runId: string): TimelineItem[] {
  return state.map((candidate): TimelineItem =>
    candidate.type === 'tool' && candidate.runId === runId && candidate.status === 'running'
      ? {
          ...candidate,
          status: 'failed',
          error: {
            code: 'RUN_INTERRUPTED',
            message: '任务中断时，此操作尚未完成。',
            retryable: false,
          },
        }
      : candidate);
}

function settleCompaction(state: TimelineItem[], runId: string, status: 'failed' | 'canceled'): TimelineItem[] {
  return state.map(item => item.type === 'command' && item.kind === 'compact'
    && item.commandSource === 'automatic' && item.runId === runId && item.status === 'loading'
    ? { ...item, status, cancellable: false } : item);
}

export function reduceSSEEvent(state: TimelineItem[], event: SSEEvent): TimelineItem[] {
  const timestamp = timestampOf(event);
  const runId = event.data.run_id;

  switch (event.event) {
    case 'run.started':
    case 'run.progress':
	case 'plan.updated':
	case 'run.mode_changed':
    case 'scope.updated':
		return state;
    case 'context.window.updated': {
      const progress = event.data.compaction;
      if (!progress) return state;
      const existing = state.find(item => item.id === `context-compaction:${progress.id}`);
      if (existing && (existing.type === 'context_compaction' || (existing.type === 'command' && existing.status !== 'loading'))) return state;
      return upsertTimelineItem(state, {
        id: `context-compaction:${progress.id}`, type: 'command', kind: 'compact',
        runId, commandSource: 'automatic', method: 'auto', title: '压缩上下文',
        status: 'loading', phase: progress.phase, cancellable: false, timestamp: existing?.timestamp ?? timestamp,
      });
    }
    case 'context.compacted': {
      const compaction = event.data.compaction;
      const item = contextCompactionTimelineItem({ ...compaction, run_id: compaction.run_id || runId });
      const existing = state.find(value => value.id === item.id);
      return upsertTimelineItem(state, { ...item, timestamp: existing?.timestamp ?? item.timestamp });
    }

    case 'run.resumed': {
      const item: RunLifecycleItem = {
        id: `${runId}:resumed`,
        type: 'run_lifecycle',
        runId,
        state: 'resumed',
        text: '已从中断处恢复，继续执行',
        timestamp,
      };
      return upsertTimelineItem(settleInterruptedTools(state.filter(entry => !(entry.runId === runId && entry.type === 'terminal_notice')), runId), item);
    }

	case 'plan.approval_requested': {
		const plan = reducePlan(null, event)!;
		const id = `${runId}:plan-approval:${event.data.interaction_id}`;
		const existing = state.find((item): item is PlanApprovalItem => item.type === 'plan_approval' && item.id === id);
		if (existing?.answer) return state;
		return upsertTimelineItem(state, { id, type: 'plan_approval', runId, interactionId: event.data.interaction_id, plan, answer: existing?.answer, timestamp: existing?.timestamp ?? timestamp });
	}
	case 'plan.approval_answered': {
		const id = `${runId}:plan-approval:${event.data.interaction_id}`;
		const existing = state.find((item): item is PlanApprovalItem => item.type === 'plan_approval' && item.id === id);
		return existing ? upsertTimelineItem(state, { ...existing, answer: { decision: event.data.decision, feedback: event.data.feedback } }) : state;
	}
    case 'command.permission_requested': {
      const id = `${runId}:command-permission:${event.data.interaction_id}`;
      const existing = state.find((item): item is CommandPermissionItem =>
        item.type === 'command_permission' && item.id === id);
      return upsertTimelineItem(state, {
        id,
        type: 'command_permission',
        runId,
        interactionId: event.data.interaction_id,
        callId: event.data.call_id,
        command: event.data.command,
        commandHash: event.data.command_hash,
        reasonCode: event.data.reason_code,
        reason: event.data.reason,
        answer: existing?.answer,
        timestamp: existing?.timestamp ?? timestamp,
      });
    }
    case 'command.permission_answered': {
      const id = `${runId}:command-permission:${event.data.interaction_id}`;
      const existing = state.find((item): item is CommandPermissionItem =>
        item.type === 'command_permission' && item.id === id);
      if (!existing || existing.callId !== event.data.call_id || existing.commandHash !== event.data.command_hash) {
        return state;
      }
      return upsertTimelineItem(state, { ...existing, answer: { decision: event.data.decision, feedback: event.data.feedback } });
    }
    case 'scope.expansion_requested': {
      const id = `${runId}:scope-expansion:${event.data.interaction_id}`;
      const existing = state.find((item): item is ScopeExpansionItem => item.type === 'scope_expansion' && item.id === id);
      return upsertTimelineItem(state, {
        id, type: 'scope_expansion', runId, interactionId: event.data.interaction_id,
        callId: event.data.call_id, baseRevision: event.data.base_revision,
        currentScope: event.data.current_scope, requestedAddition: event.data.requested_addition,
        proposedScope: event.data.proposed_scope, affectedPageCount: event.data.affected_page_count,
        reason: event.data.reason, answer: existing?.answer, timestamp: existing?.timestamp ?? timestamp,
      });
    }
    case 'scope.expansion_answered': {
      const id = `${runId}:scope-expansion:${event.data.interaction_id}`;
      const existing = state.find((item): item is ScopeExpansionItem => item.type === 'scope_expansion' && item.id === id);
      if (!existing || existing.callId !== event.data.call_id || existing.baseRevision !== event.data.base_revision) return state;
      return upsertTimelineItem(state, { ...existing, answer: { decision: event.data.decision, appliedScope: event.data.applied_scope, feedback: event.data.feedback } });
    }

    case 'message.reasoning': {
      const item: ReasoningItem = {
        id: eventItemId(event, `reasoning:${event.data.message_id}`),
        type: 'reasoning',
        runId,
        messageId: event.data.message_id,
        text: event.data.text,
        timestamp,
      };
      return upsertTimelineItem(state, item);
    }

    case 'message.milestone': {
      const item: MilestoneItem = {
        id: eventItemId(event, `milestone:${event.data.message_id}`),
        type: 'milestone',
        runId,
        messageId: event.data.message_id,
        text: event.data.text,
        completedStepIds: event.data.completed_step_ids,
        timestamp,
      };
      return upsertTimelineItem(state, item);
    }

    case 'message.final': {
      const existing = state.find((item): item is FinalMessageItem =>
        item.type === 'final' && item.runId === runId && item.messageId === event.data.message_id);
      const item: FinalMessageItem = {
        id: existing?.id ?? eventItemId(event, `final:${event.data.message_id}`),
        type: 'final',
        runId,
        messageId: event.data.message_id,
        text: event.data.text,
        affectedTargets: event.data.affected_targets,
        durationMs: existing?.durationMs,
        timestamp,
      };
      return upsertTimelineItem(state, item);
    }

    case 'tool.started': {
      const id = `${runId}:tool:${event.data.call_id}`;
      if (event.data.tool === 'git_commit' && state.some(item => item.id === id && item.type === 'git_commit')) return state;
      const existing = state.find((item): item is ToolActivityItem =>
        item.type === 'tool' && item.id === id);
      const item: ToolActivityItem = {
        id,
        type: 'tool',
        runId,
        callId: event.data.call_id,
        tool: event.data.tool,
        planStepId: event.data.plan_step_id,
        target: existing?.changes ? existing.target : event.data.target,
        changes: existing?.changes,
        label: existing?.changes ? existing.label : event.data.display.label,
        detail: existing?.changes ? existing.detail : event.data.display.detail,
        status: existing?.status ?? 'running',
        preview: existing?.preview,
        image: existing?.image,
        review: existing?.review,
        error: existing?.error,
        command: event.data.command ?? existing?.command,
        resources: existing?.resources,
        approval: existing?.approval,
        timestamp: existing?.timestamp ?? timestamp,
      };
      return upsertTimelineItem(state, item);
    }

    case 'tool.content_prechecked': {
      return state.map(item => {
        if (item.type !== 'tool') return item;
        const matchesAssessment = item.contentPrecheck?.some(value => event.data.content_precheck.some(update => update.assessment_id === value.assessment_id));
        if (!matchesAssessment && (item.callId !== event.data.call_id || item.runId !== runId)) return item;
        const updated = new Map((item.contentPrecheck ?? []).map(value => [value.assessment_id, value]));
        for (const value of event.data.content_precheck) updated.set(value.assessment_id, value);
        return { ...item, contentPrecheck: [...updated.values()] };
      });
    }
    case 'resource.edit_approval_requested':
    case 'resource.edit_approval_updated': {
      const id = `${runId}:tool:${event.data.call_id}`;
      const existing = state.find((item): item is ToolActivityItem => item.type === 'tool' && item.id === id);
      const labels = { manifest: '内容要求', design: '视觉要求', outline: '目录结构' };
      if (existing?.approval?.answer) return state;
      if (existing?.approval?.interactionId === event.data.interaction_id && existing.approval.revision > event.data.revision) return state;
      return upsertTimelineItem(state, {
        id, type: 'tool', runId, callId: event.data.call_id,
        tool: existing?.tool ?? `edit_${event.data.resource}`,
        label: `编辑${labels[event.data.resource]}`,
        status: 'running', timestamp: existing?.timestamp ?? timestamp,
        target: event.data.target,
        approval: { interactionId: event.data.interaction_id, resource: event.data.resource,
          revision: event.data.revision, target: event.data.target, answer: existing?.approval?.answer },
      });
    }
    case 'resource.edit_approval_answered': {
      const id = `${runId}:tool:${event.data.call_id}`;
      return state.map(item => item.type === 'tool' && item.id === id && item.approval?.interactionId === event.data.interaction_id
        ? { ...item, approval: { ...item.approval, answer: { decision: event.data.decision, feedback: event.data.feedback } } } : item);
    }
    case 'tool.completed': {
      const id = `${runId}:tool:${event.data.call_id}`;
      if (event.data.tool === 'git_commit' && state.some(item => item.id === id && item.type === 'git_commit')) return state;
      const existing = state.find((item): item is ToolActivityItem =>
        item.type === 'tool' && item.id === id);
      const item: ToolActivityItem = {
        id,
        type: 'tool',
        runId,
        callId: event.data.call_id,
        tool: event.data.tool,
        planStepId: existing?.planStepId,
        target: event.data.target ?? existing?.target,
        changes: event.data.changes,
        label: event.data.display.label,
        detail: event.data.display.detail,
        status: event.data.status,
        preview: event.data.preview,
        image: event.data.image,
        review: event.data.review,
        contentPrecheck: event.data.content_precheck ?? existing?.contentPrecheck,
        error: event.data.error,
        command: event.data.command ?? existing?.command,
        resources: event.data.resources,
        approval: existing?.approval,
        timestamp: existing?.timestamp ?? timestamp,
      };
      return upsertTimelineItem(state, item);
    }

    case 'question.asked': {
      const id = `${runId}:question:${event.data.question_id}`;
      const existing = state.find((item): item is QuestionItem =>
        item.type === 'question' && item.id === id);
      const item: QuestionItem = {
        id,
        type: 'question',
        runId,
        questionId: event.data.question_id,
        header: event.data.header,
        questions: event.data.questions,
        answer: existing?.answer,
        displayText: existing?.displayText,
        timestamp: existing?.timestamp ?? timestamp,
      };
      return upsertTimelineItem(state, item);
    }

    case 'question.answered': {
      const id = `${runId}:question:${event.data.question_id}`;
      const existing = state.find((item): item is QuestionItem =>
        item.type === 'question' && item.id === id);
      if (!existing) return state;
      return upsertTimelineItem(state, {
        ...existing,
        answer: event.data.answer,
        displayText: event.data.display_text,
      });
    }

    case 'run.completed': {
      return settleCompaction(state, runId, 'failed').map((item) =>
        item.type === 'final' && item.runId === runId
          ? {
              ...item,
              affectedTargets: event.data.affected_targets,
              durationMs: event.data.duration_ms,
            }
          : item);
    }

    case 'run.failed':
    case 'run.error':
    case 'run.canceled': {
      const error = event.data.error;
      const status = event.event === 'run.canceled'
        ? 'canceled'
        : event.event === 'run.error'
          ? 'error'
          : 'failed';
      const item: TerminalNoticeItem = {
        id: `${runId}:terminal`,
        type: 'terminal_notice',
        runId,
        status,
        error,
        affectedTargets: event.data.affected_targets,
        traceId: event.data.trace_id,
        message: status === 'canceled'
          ? event.data.reason === 'superseded'
            ? '此前任务因服务中断而结束。'
            : '你已停止本次任务。'
          : status === 'error'
            ? (error?.message ?? '任务执行时发生异常，未能完成。')
          : (error?.message ?? '任务未能完成全部要求。'),
        durationMs: event.data.duration_ms,
        reason: event.data.reason,
        timestamp,
      };
      const settledState = event.data.reason === 'superseded' ? settleInterruptedTools(state, runId) : state;
      return upsertTimelineItem(settleCompaction(settledState, runId, status === 'canceled' ? 'canceled' : 'failed'), item);
    }
  }
}
