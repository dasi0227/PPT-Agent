export interface Project {
  id: string;
  title: string;
  work_dir: string;
  theme: string;
  status: 'draft' | 'generating' | 'ready';
  design_path: string;
  outline_path?: string;
  outline_revision?: number;
  design_revision?: number;
  created_at: number;
  updated_at: number;
}

export interface SlideSpec {
  version: '4.0';
  revision: number;
  project_id: string;
  slide_id: string;
  title: string;
  key_message: string;
  elements: Array<{
    type: 'text' | 'list' | 'metric' | 'quote' | 'table' | 'chart' | 'diagram' | 'code' | 'asset';
    intent: string;
  }>;
  layout?: string;
  created_at: number;
  updated_at: number;
}

export interface Outline {
  version: '4.0';
  revision: number;
  project_id: string;
  sections: OutlineSection[];
  created_at: number;
  updated_at: number;
}

export interface OutlineSlideNode { slide_id: string; label: string; role: string }
export interface OutlineSubsection { id: string; title: string; slides: OutlineSlideNode[] }
export interface OutlineSection { id: string; title: string; purpose: string; slides: OutlineSlideNode[]; subsections: OutlineSubsection[] }

export interface Manifest {
  version: '4.0'; revision: number; project_id: string; title: string; goal: string;
  audience: string; language: string; positioning?: string; requirements: string[]; prohibitions: string[];
  canvas: { aspect_ratio: '16:9' | '4:3' };
  numbering: { enabled: boolean; hidden_roles: string[]; format: 'number' };
  created_at: number; updated_at: number;
}

export interface Design {
  version: '4.0';
  revision: number;
  project_id: string;
  theme: string;
  direction: string;
  density: 'sparse' | 'medium' | 'dense';
  chrome: Array<{
    type: 'page_number' | 'section_marker' | 'key_message' | 'deck_title';
    placement: 'top-left' | 'top-center' | 'top-right' | 'bottom-left' | 'bottom-center' | 'bottom-right' | 'left-edge' | 'right-edge';
    style: string;
  }>;
  created_at: number;
  updated_at: number;
}

export type MaterializationState = 'pending' | 'not_materialized' | 'fresh' | 'spec_stale' | 'design_stale' | 'frame_stale' | 'unknown';
export interface Materialization {
  state: MaterializationState;
  revisions: { slide_html: number; source_outline: number; source_spec: number; source_design: number };
}

export interface Slide {
  id: string;
  project_id: string;
  layout: string;
  title: string;
  html_path: string;
  spec_path: string;
  current_version: number;
  spec_revision?: number;
  html_revision?: number;
  source_outline_revision?: number;
  source_spec_revision?: number;
  source_design_revision?: number;
  spec?: SlideSpec;
  materialization?: Materialization;
  role?: string;
  label?: string;
  sectionId?: string;
  subsectionId?: string;
}

export interface Thread {
  id: string;
  project_id: string;
  title: string;
  status: string;
  history_path: string;
  created_at: number;
  updated_at: number;
}

export interface Run {
  id: string;
  thread_id: string;
  project_id: string;
  status: 'pending' | 'running' | 'waiting' | 'paused' | 'recovering' | 'done' | 'failed' | 'canceled';
  scope: RunScope;
  mode: RunMode;
  events_url: string;
  model: string | null;
  pause_reason?: string;
  paused_at?: number;
}

export type Artifact = 'spec' | 'ppt';
export type ScopeLevel = 'slide' | 'deck';
export type RunMode = 'talk' | 'ask' | 'plan' | 'execute';
export type RunLanguage = 'zh-CN' | 'en-US';
export type SlideRange = '5-8' | '9-15' | '16-25' | '26+';

export interface RunScope { artifact: Artifact; level: ScopeLevel; slide_id?: string }

export interface CreateRunRequest {
  client_request_id?: string;
  model?: string;
  scope: RunScope;
  mode: RunMode;
  instruction: string;
  options?: { language?: RunLanguage; range?: SlideRange };
}

export interface LLMProfileCapabilities {
  vision: boolean;
  tool_calls: boolean;
  multiple_tool_calls: boolean;
}

export interface LLMProfile {
  name: string;
  model: string;
  capabilities: LLMProfileCapabilities;
}

export interface LLMProfilesResponse {
  default: string;
  profiles: LLMProfile[];
}

export interface PolishRequest {
  instruction: string;
  thread_id?: string;
  scope: RunScope;
  mode: RunMode;
  model: string;
}

export interface PolishResponse {
  polished_instruction: string;
  changed: boolean;
  prompt_version: string;
}

export interface SteerRunRequest {
  expected_run_id: string;
  client_message_id: string;
  content: string;
}

export interface SteerRunResponse {
  status: 'accepted';
  run_id: string;
  client_message_id: string;
}

export interface CancelRunResponse {
  status: 'cancel_requested' | 'pending' | 'running' | 'waiting' | 'paused' | 'recovering' | 'done' | 'failed' | 'canceled';
  run_id: string;
}

export type RunCancelReason = 'user_requested' | 'superseded';

export interface ProjectContentSnapshot {
  manifest: Manifest;
  outline: Outline;
  design: Design;
  slides_by_id: Record<string, {
    spec_state: 'pending' | 'ready'; spec: SlideSpec | null; html_state: MaterializationState;
    html_revision: number; materialization: MaterializationRecord | null;
  }>;
}

export interface MaterializationRecord {
  version: '4.0'; artifact: { revision: number; hash: string };
  source: { manifest_revision: number; outline_node_hash: string; spec_revision: number; design_revision: number; hash: string };
  frame: { context_hash: string }; rendered_at: number;
}

export type RestrictedPatch =
  | { op: 'add'; path: string; value: unknown }
  | { op: 'remove'; path: string }
  | { op: 'replace'; path: string; value: unknown };
export interface DraftSlide { client_ref: string; label: string; role: string }
export interface DraftSubsection { client_ref: string; title: string; slides: DraftSlide[] }
export interface DraftSection { client_ref: string; title: string; purpose: string; slides: DraftSlide[]; subsections: DraftSubsection[] }
export type DraftOutlineNode =
  | ({ kind: 'section' } & DraftSection)
  | { kind: 'subsection'; client_ref: string; title: string }
  | ({ kind: 'slide' } & DraftSlide);
export type OutlineNodeChanges = { title?: string; purpose?: string; label?: string; role?: string };
export type PPTMutation =
  | { op: 'manifest.patch'; expected_revision?: number; patch: RestrictedPatch[] }
  | { op: 'outline.init'; expected_revision?: number; structure: DraftSection[] }
  | { op: 'outline.insert'; expected_revision?: number; node: DraftOutlineNode; position: MutationPosition; direct_slides_policy?: 'move_into_new_subsection' }
  | { op: 'outline.move'; expected_revision?: number; node_id: string; position: MutationPosition }
  | { op: 'outline.update'; expected_revision?: number; node_id: string; changes: OutlineNodeChanges }
  | { op: 'outline.remove'; expected_revision?: number; node_id: string; child_policy?: 'promote_to_section' }
  | { op: 'design.write'; expected_revision?: number; design: Partial<Design> }
  | { op: 'design.patch'; expected_revision?: number; patch: RestrictedPatch[] }
  | { op: 'slide.spec.write'; expected_revision?: number; slide_id: string; spec: Partial<SlideSpec> }
  | { op: 'slide.spec.patch'; expected_revision?: number; slide_id: string; patch: RestrictedPatch[] }
  | { op: 'slide.html.write'; slide_id: string; html: string }
  | { op: 'slide.html.patch'; slide_id: string; edits: Array<{ old_text: string; new_text: string }> };
export interface MutationPosition { parent_id?: string; before_id?: string; after_id?: string }
export interface MutationResponse { mutation: { operation: string; revisions: Record<string, number>; created: Record<string, string>; affected_slide_ids: string[]; invalidated_slide_ids: string[] }; content: ProjectContentSnapshot }

export interface RunInputPayload {
  content: string;
  reply_to: string;
}

export interface PlanApprovalRequest {
  interaction_id: string;
  plan_id: string;
  expected_revision: number;
  decision: 'approve' | 'revise' | 'cancel';
  feedback?: string;
  idempotency_key?: string;
}

export interface CommandPermissionRequest {
  interaction_id: string;
  call_id: string;
  command_hash: string;
  decision: 'allow_once' | 'deny';
}

export type JsonRecord = Record<string, unknown>;

export type SSEEventName =
  | 'run.started'
  | 'run.progress'
  | 'run.resumed'
  | 'run.completed'
  | 'run.failed'
  | 'run.error'
  | 'run.canceled'
  | 'plan.updated'
  | 'plan.approval_requested'
  | 'plan.approval_answered'
  | 'command.permission_requested'
  | 'command.permission_answered'
  | 'run.mode_changed'
  | 'message.reasoning'
  | 'message.milestone'
  | 'message.final'
  | 'tool.started'
  | 'tool.completed'
  | 'question.asked'
  | 'question.answered';

export type PlanStepStatus = 'pending' | 'in_progress' | 'completed' | 'failed';

export interface PlanStep {
  id: string;
  title: string;
  status: PlanStepStatus;
  detail?: string;
}

export interface PlanState {
  id: string;
  title: string;
  content: string;
  revision: number;
  approved_revision?: number;
  status: 'awaiting_approval' | 'active' | 'completed' | 'canceled';
  steps: PlanStep[];
}

export interface PublicEventBase {
  schema_version: 3;
  run_id: string;
  occurred_at: string;
}

export interface PublicTarget {
  type: 'deck' | 'slide' | 'file';
  slide_id?: string;
  part: 'manifest' | 'outline' | 'design' | 'spec' | 'html' | 'content';
  display_name?: string;
  insertions?: number;
  deletions?: number;
  local_path?: string;
  open_url?: string;
}

export interface PublicDisplay {
  label: string;
  detail?: string;
}

export interface PublicError {
  code: string;
  message: string;
  retryable: boolean;
}

export interface RunTerminalPayload extends PublicEventBase {
  duration_ms: number;
  affected_targets: PublicTarget[];
  error: PublicError | null;
  trace_id: string;
  reason?: RunCancelReason;
}

export interface ToolPreview {
  slide_id: string;
  image_url: string;
  warnings: string[];
}

export interface CommandProjection {
  text: string;
  status?: 'completed' | 'blocked' | 'failed';
  exit_code?: number;
  duration_ms?: number;
  output_truncated?: boolean;
  stdout_preview?: string;
  stderr_preview?: string;
}

export interface QuestionOption {
  id: string;
  label: string;
  description?: string;
}

export interface QuestionField {
  id: string;
  title: string;
  description?: string;
  options: QuestionOption[];
  allow_custom: boolean;
}

export interface QuestionFieldAnswer {
  question_id: string;
  selected_option_id?: string;
  custom_text?: string;
}

export interface QuestionAnswer {
  answers: QuestionFieldAnswer[];
}

export type RunProgressStage =
  | 'thinking'
  | 'planning'
  | 'reading'
  | 'writing'
  | 'rendering'
  | 'finalizing';

interface SSEEventBase<Name extends SSEEventName, Data> {
  id?: string;
  event: Name;
  data: Data;
}

export type SSEEvent =
  | SSEEventBase<'run.started', PublicEventBase & {
      scope: RunScope;
      mode: RunMode;
      user_input: string;
    }>
  | SSEEventBase<'run.progress', PublicEventBase & {
      stage: RunProgressStage;
      text: string;
      target?: PublicTarget;
      progress?: { current: number; total: number; unit: string };
    }>
  | SSEEventBase<'run.resumed', PublicEventBase>
  | SSEEventBase<'run.completed', RunTerminalPayload>
  | SSEEventBase<'run.failed', RunTerminalPayload>
  | SSEEventBase<'run.error', RunTerminalPayload>
  | SSEEventBase<'run.canceled', RunTerminalPayload>
  | SSEEventBase<'plan.updated', PublicEventBase & {
	  plan: JsonRecord & { plan_id: string; revision: number; title: string; content: string; status: string; steps: JsonRecord[] };
    }>
  | SSEEventBase<'plan.approval_requested', PublicEventBase & { interaction_id: string; plan: JsonRecord & { plan_id: string; revision: number; title: string; content: string; status: string; steps: JsonRecord[] } }>
  | SSEEventBase<'plan.approval_answered', PublicEventBase & { interaction_id: string; plan_id: string; revision: number; decision: 'approve' | 'revise' | 'cancel'; feedback?: string }>
  | SSEEventBase<'command.permission_requested', PublicEventBase & {
      interaction_id: string;
      call_id: string;
      command: string;
      command_hash: string;
      reason_code: string;
      reason: string;
    }>
  | SSEEventBase<'command.permission_answered', PublicEventBase & CommandPermissionRequest>
  | SSEEventBase<'run.mode_changed', PublicEventBase & { previous_mode: RunMode; mode: RunMode }>
  | SSEEventBase<'message.reasoning', PublicEventBase & { message_id: string; text: string }>
  | SSEEventBase<'message.milestone', PublicEventBase & {
      message_id: string;
      text: string;
      completed_step_ids: string[];
    }>
  | SSEEventBase<'message.final', PublicEventBase & {
      message_id: string;
      text: string;
      affected_targets?: PublicTarget[];
    }>
  | SSEEventBase<'tool.started', PublicEventBase & {
      call_id: string;
      tool: string;
      plan_step_id?: string;
      target?: PublicTarget;
      display: PublicDisplay;
      command?: CommandProjection;
    }>
  | SSEEventBase<'tool.completed', PublicEventBase & {
      call_id: string;
      tool: string;
      status: 'completed' | 'blocked' | 'failed';
      target?: PublicTarget;
      display: PublicDisplay;
      preview?: ToolPreview;
      error?: PublicError;
      command?: CommandProjection;
    }>
  | SSEEventBase<'question.asked', PublicEventBase & {
      question_id: string;
      header?: string;
      questions: QuestionField[];
    }>
  | SSEEventBase<'question.answered', PublicEventBase & {
      question_id: string;
      answer: QuestionAnswer;
      display_text: string;
    }>;
