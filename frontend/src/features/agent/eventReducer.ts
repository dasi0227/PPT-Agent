import {
  PlanState,
  PlanStep,
  PlanStepStatus,
  PublicError,
  PublicTarget,
  QuestionAnswer,
  QuestionOption,
  SSEEvent,
  ToolPreview,
} from '../../api/types';

export type TimelineItemType =
  | 'user_turn'
  | 'reasoning'
  | 'milestone'
  | 'final'
  | 'tool'
  | 'question'
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
  target?: { artifact: string; level: string; slide_id?: string };
  interaction?: { intent: string };
}

export interface ReasoningItem extends BaseTimelineItem {
  type: 'reasoning';
  messageId: string;
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
}

export interface ToolActivityItem extends BaseTimelineItem {
  type: 'tool';
  callId: string;
  tool: string;
  planStepId?: string;
  target?: PublicTarget;
  label: string;
  detail?: string;
  status: 'running' | 'completed' | 'failed';
  preview?: ToolPreview;
  error?: PublicError;
}

export interface QuestionItem extends BaseTimelineItem {
  type: 'question';
  questionId: string;
  header?: string;
  prompt: string;
  selection: 'single' | 'multiple';
  options: QuestionOption[];
  allowCustom: boolean;
  answer?: QuestionAnswer;
  displayText?: string;
}

export interface TerminalNoticeItem extends BaseTimelineItem {
  type: 'terminal_notice';
  status: 'failed' | 'canceled';
  error?: PublicError;
  message: string;
  technicalMessage?: string;
  requestId?: string;
}

export type TimelineItem =
  | UserTurnItem
  | ReasoningItem
  | MilestoneItem
  | FinalMessageItem
  | ToolActivityItem
  | QuestionItem
  | TerminalNoticeItem;

function normalizeStepStatus(status: unknown): PlanStepStatus {
  if (status === 'completed' || status === 'failed' || status === 'in_progress') return status;
  return 'pending';
}

export function reducePlan(prev: PlanState | null, event: SSEEvent): PlanState | null {
  if (event.event !== 'plan.updated') return prev;
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
    title: String(plan.explanation ?? '执行计划'),
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

export function reduceSSEEvent(state: TimelineItem[], event: SSEEvent): TimelineItem[] {
  const timestamp = timestampOf(event);
  const runId = event.data.run_id;

  switch (event.event) {
    case 'run.started':
    case 'run.progress':
    case 'plan.updated':
      return state;

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
      const existing = state.find((item) =>
        item.type === 'final' && item.runId === runId && item.messageId === event.data.message_id);
      const item: FinalMessageItem = {
        id: existing?.id ?? eventItemId(event, `final:${event.data.message_id}`),
        type: 'final',
        runId,
        messageId: event.data.message_id,
        text: event.data.text,
        affectedTargets: event.data.affected_targets ?? [],
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
        target: existing?.target,
        label: event.data.display.label,
        detail: event.data.display.detail,
        status: event.data.status,
        preview: event.data.preview,
        error: event.data.error,
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
        prompt: event.data.prompt,
        selection: event.data.selection,
        options: event.data.options,
        allowCustom: event.data.allow_custom,
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

    case 'run.finished': {
      if (event.data.status === 'completed') return state;
      const error = event.data.error;
      const item: TerminalNoticeItem = {
        id: `${runId}:terminal`,
        type: 'terminal_notice',
        runId,
        status: event.data.status,
        error,
        message: event.data.status === 'canceled'
          ? '运行已取消'
          : (error?.message ?? '运行未能完成，请稍后重试。'),
        timestamp,
      };
      return upsertById(state, item);
    }
  }
}
