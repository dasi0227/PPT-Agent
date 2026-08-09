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
  version: '3.0';
  revision: number;
  project: string;
  slide_id: string;
  section_id: string;
  subsection_id?: string;
  role: string;
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
  version: '3.0';
  revision: number;
  project: string;
  title: string;
  goal: string;
  audience: string;
  language: string;
  positioning?: string;
  constraints: { must_include: string[]; must_avoid: string[]; style_limits: string[]; content_limits: string[] };
  sections: Array<{ id: string; title: string; purpose: string; subsections: Array<{ id: string; title: string }> }>;
  slide_order: string[];
  created_at: number;
  updated_at: number;
}

export interface Design {
  version: '3.0';
  revision: number;
  project: string;
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

export type MaterializationState = 'not_materialized' | 'fresh' | 'spec_stale' | 'design_stale' | 'unknown';
export interface Materialization {
  state: MaterializationState;
  revisions: { slide_html: number; source_outline: number; source_spec: number; source_design: number };
}

export interface Slide {
  id: string;
  project_id: string;
  position: number;
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
  status: 'pending' | 'running' | 'waiting' | 'done' | 'failed' | 'canceled';
  target: RunTarget;
  interaction: RunInteraction;
  events_url: string;
  model: string | null;
}

export type Artifact = 'spec' | 'presentation';
export type TargetLevel = 'slide' | 'deck';
export type InteractionIntent = 'talk' | 'ask' | 'plan' | 'execute';
export type ExecutionStrategy = 'talk' | 'ask' | 'plan' | 'execute';

export interface RunTarget { artifact: Artifact; level: TargetLevel; slide_id?: string }
export interface RunInteraction { intent: InteractionIntent }

export interface CreateRunRequest {
  client_request_id?: string;
  model?: string;
  target: RunTarget;
  interaction: RunInteraction;
  instruction: string;
  options?: { language?: string; theme_id?: string; desired_slide_count?: number };
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
  status: 'cancel_requested' | 'pending' | 'running' | 'waiting' | 'done' | 'failed' | 'canceled';
  run_id: string;
}

export interface SpecProjectView {
  outline: Outline;
  slide_specs: Record<string, SlideSpec>;
  design: Design;
  materialization: Record<string, Materialization>;
}

export interface RunInputPayload {
  content: string;
  reply_to: string;
}

export type JsonRecord = Record<string, unknown>;

export type SSEEventName =
  | 'run.started'
  | 'run.progress'
  | 'run.finished'
  | 'plan.updated'
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
  revision: number;
  steps: PlanStep[];
}

export interface PublicEventBase {
  schema_version: 2;
  run_id: string;
  occurred_at: string;
}

export interface PublicTarget {
  type: 'deck' | 'slide';
  slide_id?: string;
  part: 'outline' | 'design' | 'spec' | 'html';
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

export interface ToolPreview {
  slide_id: string;
  image_url: string;
  warnings: string[];
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
  selected_option_ids: string[];
  custom_text: string;
  answers?: QuestionFieldAnswer[];
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
      target: RunTarget;
      interaction: RunInteraction;
      user_input: string;
    }>
  | SSEEventBase<'run.progress', PublicEventBase & {
      stage: RunProgressStage;
      text: string;
      target?: PublicTarget;
      progress?: { current: number; total: number; unit: string };
    }>
  | SSEEventBase<'run.finished', PublicEventBase & {
      status: 'completed' | 'failed' | 'canceled';
      affected_targets?: PublicTarget[];
      duration_ms: number;
      error?: PublicError;
    }>
  | SSEEventBase<'plan.updated', PublicEventBase & {
      plan: JsonRecord & { plan_id: string; revision: number; explanation?: string; steps: JsonRecord[] };
    }>
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
    }>
  | SSEEventBase<'tool.completed', PublicEventBase & {
      call_id: string;
      tool: string;
      status: 'completed' | 'failed';
      target?: PublicTarget;
      display: PublicDisplay;
      preview?: ToolPreview;
      error?: PublicError;
    }>
  | SSEEventBase<'question.asked', PublicEventBase & {
      question_id: string;
      header?: string;
      prompt: string;
      selection: 'single' | 'multiple';
      options: QuestionOption[];
      allow_custom: boolean;
      questions?: QuestionField[];
    }>
  | SSEEventBase<'question.answered', PublicEventBase & {
      question_id: string;
      answer: QuestionAnswer;
      display_text: string;
    }>;
