import {
  CommandProjection,
  PlanState,
  PlanStep,
  PlanStepStatus,
  PublicError,
  BriefingKind,
  BriefingVersion,
  PublicLoadedResource,
  PublicTarget,
  QuestionAnswer,
  QuestionField,
  Skill,
  SSEEvent,
  ToolPreview,
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
  | 'git_commit'
  | 'briefing'
  | 'context_compaction'
  | 'terminal_notice';

export interface BaseTimelineItem {
  id: string;
  type: TimelineItemType;
  timestamp: number;
  runId?: string;
}

export interface UserTurnItem extends BaseTimelineItem {
  type: 'user_turn';
  text: string;
  scope?: { artifact: string; level: string; slide_id?: string };
  mode?: string;
  skills?: Skill[];
  components?: PublicLoadedResource[];
  deliveryStatus?: 'sending' | 'accepted' | 'rejected';
  clientMessageId?: string;
  rejectionCode?: string;
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
  type: 'tool';
  callId: string;
  tool: string;
  planStepId?: string;
  target?: PublicTarget;
  label: string;
  detail?: string;
  status: 'running' | 'completed' | 'blocked' | 'failed';
  preview?: ToolPreview;
  error?: PublicError;
  command?: CommandProjection;
  resources?: PublicLoadedResource[];
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
  answer?: { decision: 'approve' | 'revise' | 'cancel'; feedback?: string };
}

export interface CommandPermissionItem extends BaseTimelineItem {
  type: 'command_permission';
  interactionId: string;
  callId: string;
  command: string;
  commandHash: string;
  reasonCode: string;
  reason: string;
  answer?: 'allow_once' | 'deny';
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
  type: 'git_commit';
  operationId: string;
  status: 'completed' | 'failed';
  title?: string;
  items?: string[];
  branch?: string;
  hash?: string;
  filesChanged?: number;
  insertions?: number;
  deletions?: number;
  retryable?: boolean;
}

export interface BriefingTimelineItem extends BaseTimelineItem {
  type: 'briefing';
  briefingId: string;
  kind: BriefingKind;
  status: 'loading' | 'completed';
  versions: BriefingVersion[];
  loadingStartedAt?: number;
}

export interface ContextCompactionTimelineItem extends BaseTimelineItem {
  type: 'context_compaction';
  compactionId: string;
  trigger: 'auto' | 'manual';
  summary: string;
  beforeTokens: number;
  afterTokens: number;
  maxTokens: number;
  reclaimedTokens: number;
  durationMs: number;
}

export type TimelineItem =
  | UserTurnItem
  | RunLifecycleItem
  | ReasoningItem
  | MilestoneItem
  | FinalMessageItem
  | ToolActivityItem
  | QuestionItem
  | PlanApprovalItem
  | CommandPermissionItem
  | GitCommitTimelineItem
  | BriefingTimelineItem
  | ContextCompactionTimelineItem
  | TerminalNoticeItem;

function normalizeStepStatus(status: unknown): PlanStepStatus {
  if (status === 'completed' || status === 'failed' || status === 'in_progress') return status;
  return 'pending';
}

export function reducePlan(prev: PlanState | null, event: SSEEvent): PlanState | null {
	if (event.event !== 'plan.updated' && event.event !== 'plan.approval_requested') return prev;
	const plan = event.data.plan;
  const revision = Number(plan.revision);
  if (prev && revision <= prev.revision) return prev;
  const steps: PlanStep[] = plan.steps.map((step) => ({
    id: String(step.id ?? ''),
    title: String(step.title ?? ''),
    status: normalizeStepStatus(step.status),
  }));
  return {
    id: String(plan.plan_id),
		title: String(plan.title ?? '执行计划'), content: String(plan.content ?? ''),
		approved_revision: Number(plan.approved_revision || 0) || undefined,
		status: (String(plan.status) as PlanState['status']),
    revision,
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

function upsertById(state: TimelineItem[], item: TimelineItem): TimelineItem[] {
  const index = state.findIndex((candidate) => candidate.id === item.id);
  if (index < 0) return [...state, item];
  const copy = state.slice();
  copy[index] = item;
  return copy;
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

export function reduceSSEEvent(state: TimelineItem[], event: SSEEvent): TimelineItem[] {
  const timestamp = timestampOf(event);
  const runId = event.data.run_id;

  switch (event.event) {
    case 'run.started':
    case 'run.progress':
	case 'plan.updated':
	case 'run.mode_changed':
    case 'context.window.updated':
		return state;
    case 'context.compacted': {
      const compaction = event.data.compaction;
      return upsertById(state, {
        id: `context-compaction:${compaction.id}`,
        type: 'context_compaction',
        compactionId: compaction.id,
        trigger: compaction.trigger,
        summary: compaction.summary,
        beforeTokens: compaction.before_tokens,
        afterTokens: compaction.after_tokens,
        maxTokens: compaction.max_tokens,
        reclaimedTokens: compaction.reclaimed_tokens,
        durationMs: compaction.duration_ms,
        timestamp: compaction.created_at * 1000,
      });
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
      return upsertById(settleInterruptedTools(state, runId), item);
    }

	case 'plan.approval_requested': {
		const plan = reducePlan(null, event)!;
		return upsertById(state, { id: `${runId}:plan-approval:${event.data.interaction_id}`, type: 'plan_approval', runId, interactionId: event.data.interaction_id, plan, timestamp });
	}
	case 'plan.approval_answered': {
		const id = `${runId}:plan-approval:${event.data.interaction_id}`;
		const existing = state.find((item): item is PlanApprovalItem => item.type === 'plan_approval' && item.id === id);
		return existing ? upsertById(state, { ...existing, answer: { decision: event.data.decision, feedback: event.data.feedback } }) : state;
	}
    case 'command.permission_requested': {
      const id = `${runId}:command-permission:${event.data.interaction_id}`;
      const existing = state.find((item): item is CommandPermissionItem =>
        item.type === 'command_permission' && item.id === id);
      return upsertById(state, {
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
      return upsertById(state, { ...existing, answer: event.data.decision });
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
      return upsertById(state, item);
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
      return upsertById(state, item);
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
        affectedTargets: event.data.affected_targets ?? [],
        durationMs: existing?.durationMs,
        timestamp,
      };
      return upsertById(state, item);
    }

    case 'tool.started': {
      const id = `${runId}:tool:${event.data.call_id}`;
      const existing = state.find((item): item is ToolActivityItem =>
        item.type === 'tool' && item.id === id);
      const item: ToolActivityItem = {
        id,
        type: 'tool',
        runId,
        callId: event.data.call_id,
        tool: event.data.tool,
        planStepId: event.data.plan_step_id,
        target: event.data.target,
        label: event.data.display.label,
        detail: event.data.display.detail,
        status: existing?.status ?? 'running',
        preview: existing?.preview,
        error: existing?.error,
        command: event.data.command ?? existing?.command,
        resources: existing?.resources,
        timestamp: existing?.timestamp ?? timestamp,
      };
      return upsertById(state, item);
    }

    case 'tool.completed': {
      const id = `${runId}:tool:${event.data.call_id}`;
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
        label: event.data.display.label,
        detail: event.data.display.detail,
        status: event.data.status,
        preview: event.data.preview,
        error: event.data.error,
        command: event.data.command ?? existing?.command,
        resources: event.data.resources,
        timestamp: existing?.timestamp ?? timestamp,
      };
      return upsertById(state, item);
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
      return upsertById(state, item);
    }

    case 'question.answered': {
      const id = `${runId}:question:${event.data.question_id}`;
      const existing = state.find((item): item is QuestionItem =>
        item.type === 'question' && item.id === id);
      if (!existing) return state;
      return upsertById(state, {
        ...existing,
        answer: event.data.answer,
        displayText: event.data.display_text,
      });
    }

    case 'run.completed': {
      return state.map((item) =>
        item.type === 'final' && item.runId === runId
          ? {
              ...item,
              affectedTargets: event.data.affected_targets.length > 0 ? event.data.affected_targets : item.affectedTargets,
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
            ? '此前任务因服务中断而暂停，已停止执行。'
            : '运行已取消'
          : status === 'error'
            ? (error?.message ?? '系统运行异常，请稍后重试。')
          : (error?.message ?? '运行未能完成，请稍后重试。'),
        durationMs: event.data.duration_ms,
        reason: event.data.reason,
        timestamp,
      };
      const settledState = event.data.reason === 'superseded' ? settleInterruptedTools(state, runId) : state;
      return upsertById(settledState, item);
    }
  }
}
