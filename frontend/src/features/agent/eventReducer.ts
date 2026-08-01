import {
  ArtifactRef,
  JsonRecord,
  SSEEvent,
  PlanState,
  PlanStep,
  PlanStepStatus,
  StructuredOutcome,
  WorkflowIssue,
} from '../../api/types';

export type TimelineItemType =
  | 'markdown'
  | 'tool_call'
  | 'artifact'
  | 'final_result'
  | 'needs_input'
  | 'error'
  | 'user_turn'
  | 'context_status'
  | 'strategy_status'
  | 'verification_status';

export interface BaseTimelineItem {
  id: string;
  type: TimelineItemType;
  timestamp: number;
}

export interface MarkdownMessageItem extends BaseTimelineItem {
  type: 'markdown';
  text: string;
}

export interface UserTurnItem extends BaseTimelineItem {
  type: 'user_turn';
  text: string;
  target?: { artifact: string; level: string; slide_id?: string };
  interaction?: { intent: string; clarification: string };
}

export interface ToolCallItem extends BaseTimelineItem {
  type: 'tool_call';
  call_id: string;
  tool: string;
  args: unknown;
  status: 'running' | 'success' | 'failed';
  observation?: unknown;
  artifacts: ArtifactItem[];
}

export interface ArtifactItem extends BaseTimelineItem {
  type: 'artifact';
  artifact_type: string;
  ref: string;
  artifact_id?: string;
  delivery: 'intermediate' | 'final';
}

export interface FinalResultItem extends BaseTimelineItem {
  type: 'final_result';
  result: StructuredOutcome | string | null;
}

export interface NeedsInputItem extends BaseTimelineItem {
  type: 'needs_input';
  prompt: string;
  choices?: string[];
}

export interface ErrorItem extends BaseTimelineItem {
  type: 'error';
  code?: string;
  message: string;
  technicalMessage?: string;
  requestId?: string;
  retryable?: boolean;
}

export interface ContextStatusItem extends BaseTimelineItem {
  type: 'context_status';
  profile: string;
  warnings: string[];
  readOnly: boolean;
}

export interface StrategyStatusItem extends BaseTimelineItem {
  type: 'strategy_status';
  strategy: 'respond' | 'direct_action' | 'compact_workflow' | 'full_pev';
  reason: string;
  risk: string;
  complexity: string;
}

export interface VerificationStatusItem extends BaseTimelineItem {
  type: 'verification_status';
  verifier: string;
  passed: boolean;
  issueCount: number;
  passedCount: number;
  failedCount: number;
  issues: WorkflowIssue[];
}

export type TimelineItem =
  | MarkdownMessageItem
  | ToolCallItem
  | ArtifactItem
  | FinalResultItem
  | NeedsInputItem
  | ErrorItem
  | ContextStatusItem
  | StrategyStatusItem
  | VerificationStatusItem
  | UserTurnItem;

function normalizeStepStatus(status: unknown): PlanStepStatus {
  if (status === 'running') return 'in_progress';
  if (status === 'completed' || status === 'failed' || status === 'skipped') return status;
  return 'pending';
}

// A plan exists only after plan.created. Respond and DirectAction therefore
// complete normally with a permanently null plan.
export function reducePlan(prev: PlanState | null, event: SSEEvent): PlanState | null {
  if (event.event === 'plan.created') {
    const plan = event.data.plan ?? {};
    const steps: PlanStep[] = Array.isArray(plan.steps)
      ? plan.steps.map((step) => ({
          id: String(step.id ?? ''),
          title: String(step.title ?? step.kind ?? ''),
          status: normalizeStepStatus(step.status),
          detail: step.instruction ? String(step.instruction) : undefined,
        }))
      : [];
    return {
      id: String(plan.id ?? ''),
      title: String(plan.goal ?? '执行计划'),
      steps,
    };
  }
  if (!prev || !['step.started', 'step.completed', 'step.failed'].includes(event.event)) {
    return prev;
  }
  const stepID = String(event.data.step_id ?? '');
  const index = prev.steps.findIndex((step) => step.id === stepID);
  if (index < 0) return prev;
  const steps = prev.steps.slice();
  steps[index] = {
    ...steps[index],
    status: event.event === 'step.started'
      ? 'in_progress'
      : event.event === 'step.completed'
        ? 'completed'
        : 'failed',
    detail: event.data.summary ? String(event.data.summary) : steps[index].detail,
  };
  return { ...prev, steps };
}

function artifactFromEvent(
  id: string,
  data: JsonRecord & { artifact: ArtifactRef },
  timestamp: number,
  delivery: 'intermediate' | 'final',
): ArtifactItem {
  const artifact = data.artifact;
  return {
    id,
    type: 'artifact',
    artifact_type: String(artifact.kind ?? 'artifact'),
    artifact_id: artifact.id ? String(artifact.id) : undefined,
    ref: String(artifact.path ?? artifact.id ?? ''),
    delivery,
    timestamp,
  };
}

export function reduceSSEEvent(state: TimelineItem[], event: SSEEvent): TimelineItem[] {
  const timestamp = Date.now();
  const newId = event.id || `evt_${timestamp}_${Math.random().toString(36).slice(2, 9)}`;
  if (event.id && state.some((item) => item.id === event.id)) return state;

  switch (event.event) {
    case 'run.started':
    case 'plan.created':
    case 'stage.started':
    case 'stage.completed':
    case 'step.started':
    case 'step.completed':
    case 'step.failed':
    case 'repair.started':
    case 'repair.completed':
      return state;

    case 'context.assembled':
      return [...state, {
        id: newId,
        type: 'context_status',
        profile: String(event.data.profile ?? ''),
        warnings: Array.isArray(event.data.warnings) ? event.data.warnings.map(String) : [],
        readOnly: Boolean(event.data.read_only),
        timestamp,
      }];

    case 'strategy.selected':
      return [...state, {
        id: newId,
        type: 'strategy_status',
        strategy: event.data.strategy,
        reason: String(event.data.reason ?? ''),
        risk: String(event.data.risk ?? ''),
        complexity: String(event.data.complexity ?? ''),
        timestamp,
      }];

    case 'tool.called':
      return [...state, {
        id: newId,
        type: 'tool_call',
        call_id: String(event.data.call_id ?? ''),
        tool: String(event.data.tool ?? ''),
        args: event.data.args ?? {},
        status: 'running',
        artifacts: [],
        timestamp,
      }];

    case 'tool.completed':
      return state.map((item) => item.type === 'tool_call' && item.call_id === event.data.call_id
        ? {
            ...item,
            status: event.data.ok ? 'success' : 'failed',
            observation: event.data.summary ?? event.data.issues,
          }
        : item);

    case 'artifact.staged':
      return [...state, artifactFromEvent(newId, event.data, timestamp, 'intermediate')];

    case 'artifact.committed': {
      const next = artifactFromEvent(newId, event.data, timestamp, 'final');
      const existing = state.findIndex((item) =>
        item.type === 'artifact' &&
        item.artifact_type === next.artifact_type &&
        item.artifact_id === next.artifact_id);
      if (existing < 0) return [...state, next];
      const copy = state.slice();
      copy[existing] = { ...next, id: copy[existing].id };
      return copy;
    }

    case 'verification.completed': {
      const issues = Array.isArray(event.data.result?.issues) ? event.data.result.issues : [];
      const passed = Boolean(event.data.result?.passed);
      const next: VerificationStatusItem = {
        id: newId,
        type: 'verification_status',
        verifier: String(event.data.verifier ?? 'verifier'),
        passed,
        issueCount: issues.length,
        passedCount: passed ? 1 : 0,
        failedCount: passed ? 0 : 1,
        issues,
        timestamp,
      };
      const previous = state[state.length - 1];
      if (previous?.type !== 'verification_status') return [...state, next];
      return [
        ...state.slice(0, -1),
        {
          ...previous,
          passed: previous.passed && next.passed,
          issueCount: previous.issueCount + next.issueCount,
          passedCount: previous.passedCount + next.passedCount,
          failedCount: previous.failedCount + next.failedCount,
          issues: [...previous.issues, ...next.issues],
          timestamp,
        },
      ];
    }

    case 'status.summary':
      return [...state, {
        id: newId,
        type: 'markdown',
        text: String(event.data.summary ?? ''),
        timestamp,
      }];

    case 'needs_input':
      return [...state, {
        id: String(event.data.id ?? newId),
        type: 'needs_input',
        prompt: String(event.data.prompt ?? ''),
        choices: event.data.choices,
        timestamp,
      }];

    case 'run.completed':
      return [...state, {
        id: newId,
        type: 'final_result',
        result: event.data.outcome ?? null,
        timestamp,
      }];

    case 'run.failed':
    case 'run.canceled':
      return [...state, {
        id: newId,
        type: 'error',
        code: event.data.outcome?.code,
        message: String(event.data.outcome?.message ?? (event.event === 'run.canceled' ? '运行已取消' : '运行失败')),
        timestamp,
      }];

    default:
      return state;
  }
}
