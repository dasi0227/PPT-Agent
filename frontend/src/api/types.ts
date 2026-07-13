export interface Project {
  id: string;
  title: string;
  work_dir: string;
  theme: string;
  status: 'draft' | 'generating' | 'ready';
  design_path: string;
  created_at: number;
  updated_at: number;
}

export interface SlideContent {
  layout: string;
  title: string;
  subtitle?: string;
  bullets?: string[];
  content_intent?: string;
  chart_intent?: { type: string; data_hint?: string };
  steps?: number;
  notes?: string;
}

export interface Slide {
  id: string;
  project_id: string;
  idx: number;
  layout: string;
  title: string;
  html_path: string;
  json_path: string;
  current_version: number;
  order: number;
  outline_dirty: boolean;
  content?: SlideContent;
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
  status: 'queued' | 'in_progress' | 'requires_action' | 'cancelling' | 'cancelled' | 'failed' | 'completed' | 'expired';
  mode: 'normal' | 'talk' | 'ask';
  scope: RunScope;
  created_at: string;
}

export type RunScope = 'current' | 'page' | 'overview' | 'repo';

export type RunKind = 'outline' | 'generate' | 'edit' | 'command';

export type RunMode = 'normal' | 'talk' | 'ask';

export type RunCommand = 'talk' | 'ask' | 'prompt' | 'recap';

export interface RunPayload {
  kind: RunKind;
  instruction: string;
  scope?: RunScope;                 // outline 首次可省，后端缺省为 current
  mode?: RunMode;
  page_index?: number;
  command?: RunCommand;
  brief?: string;                   // outline
  slide_count?: number;             // outline
  language?: string;                // outline
  theme?: string;                   // generate
}

export interface NeedsInputPayload {
  content: string;
  reply_to: string;
}

export type SSEEventName =
  | 'run.started' | 'thought' | 'tool_call' | 'tool_result' | 'progress'
  | 'token' | 'artifact' | 'needs_input' | 'info' | 'done' | 'error'
  | 'plan' | 'plan.update';

export type PlanStepStatus = 'pending' | 'in_progress' | 'completed' | 'failed' | 'skipped';

export interface PlanStep {
  id: string;
  title: string;
  status: PlanStepStatus;
  detail?: string;
}

export interface PlanState {
  id: string;
  title: string;
  steps: PlanStep[];
}

export interface SSEEvent {
  id?: string;
  event: SSEEventName;
  data: any;
}
