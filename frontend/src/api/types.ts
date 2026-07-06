export interface Project {
  id: string;
  title: string;
  theme: string;
  status: string;
  created_at: number;
  updated_at: number;
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
}

export interface Thread {
  id: string;
  project_id: string;
  title?: string;
  history_path?: string;
  status?: string;
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

export interface SSEEvent {
  id?: string;
  event: SSEEventName;
  data: any;
}
