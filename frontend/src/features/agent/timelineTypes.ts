import type { CommandProgress } from '../../stores/commandRuntime';
import {
  CommandProjection,
  PlanProposal,
  PublicError,
  BriefingKind,
  BriefingVersion,
  ContextCompaction,
  PublicLoadedResource,
  PublicTarget,
  QuestionAnswer,
  QuestionField,
  PublicSkill,
  ToolPreview,
  ToolReadImage,
  ReviewResult,
  RunScope,
  CreateRunScopeInput,
  ApprovalResource,
} from '../../api/types';

export type TimelineItemType =
  | 'deferred'
  | 'user_turn'
  | 'thinking'
  | 'milestone'
  | 'final'
  | 'input_reply'
  | 'tool'
  | 'question'
  | 'plan_approval'
  | 'scope_expansion'
  | 'git_commit'
  | 'command'
  | 'briefing'
  | 'context_compaction'
  | 'terminal_notice';

export interface BaseTimelineItem {
  sceneRevision?: string;
  createdSeq?: string;
  groupId?: string;
  detailRef?: string;
  requestRevision?: string;
  interactionState?: 'pending' | 'answered' | 'applied' | 'canceled';
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
  steering?: boolean;
  domSelections?: import('../../api/types').PublicDOMSelection[];
  referenceOrder?: import('../../api/types').ReferenceOrderItem[];
}

export interface ThinkingItem extends BaseTimelineItem {
  type: 'thinking';
  messageId: string;
  text: string;
}

export interface MilestoneItem extends BaseTimelineItem {
  type: 'milestone';
  messageId: string;
  text: string;
}

export interface FinalMessageItem extends BaseTimelineItem {
  type: 'final';
  messageId: string;
  text: string;
  affectedTargets: PublicTarget[];
  durationMs?: number;
}

export interface InputReplyItem extends BaseTimelineItem {
  type: 'input_reply';
  messageId: string;
  text: string;
  affectedTargets: PublicTarget[];
}

export interface ToolActivityItem extends BaseTimelineItem {
  previewAssessment?: import('../../api/types').PreviewAssessment;
  type: 'tool';
  callId: string;
  tool: string;
  target?: PublicTarget;
  changes?: PublicTarget[];
  label: string;
  detail?: string;
  status: 'running' | 'completed' | 'refuse' | 'blocked' | 'failed' | 'interrupted' | 'canceled';
  preview?: ToolPreview;
  image?: ToolReadImage;
  review?: ReviewResult;
  error?: PublicError;
  command?: CommandProjection;
  commandApproval?: {
    interactionId: string; commandHash: string; changes: PublicTarget[];
    answer?: { decision: 'allow_once' | 'deny'; feedback?: string };
  };
  resources?: PublicLoadedResource[];
  approval?: { interactionId: string; resource: ApprovalResource; revision: string; target: PublicTarget; answer?: { decision: 'approve' | 'reject'; feedback?: string } };
}

export interface QuestionItem extends BaseTimelineItem {
  type: 'question';
  questionId: string;
  questions: QuestionField[];
  answer?: QuestionAnswer;
}

export interface PlanApprovalItem extends BaseTimelineItem {
  type: 'plan_approval'; interactionId: string; plan: PlanProposal;
  answer?: { decision: 'approve' | 'refuse'; feedback?: string };
}

export interface ScopeExpansionItem extends BaseTimelineItem {
  type: 'scope_expansion';
  interactionId: string;
  callId: string;
  baseRevision: string;
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
  reason?: import('../../api/types').RunCancelReason;
  reply?: { messageId: string; text: string };
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

export interface DeferredTimelineItem extends BaseTimelineItem { type: 'deferred' }

export type TimelineItem =
  | DeferredTimelineItem
  | CommandTimelineItem
  | UserTurnItem
  | ThinkingItem
  | MilestoneItem
  | FinalMessageItem
  | InputReplyItem
  | ToolActivityItem
  | QuestionItem
  | PlanApprovalItem
  | ScopeExpansionItem
  | GitCommitTimelineItem
  | BriefingTimelineItem
  | ContextCompactionTimelineItem
  | TerminalNoticeItem;

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
    timestamp: compaction.created_at,
  };
}

