export interface Project {
  id: string;
  title: string;
  work_dir: string;
  theme: string;
  status: 'draft' | 'generating' | 'ready';
  design_path: string;
  created_at: number | string;
  updated_at: number | string;
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
  created_at: string;
  updated_at: string;
}

export interface Run {
  id: string;
  thread_id: string;
  status: 'queued' | 'in_progress' | 'requires_action' | 'cancelling' | 'cancelled' | 'failed' | 'completed' | 'expired';
  mode: 'normal' | 'talk' | 'ask';
  scope: string;
  created_at: string;
}

export interface RunPayload {
  kind: 'generate' | 'edit' | 'analyze';
  instruction: string;
  scope: string;
  mode: 'normal' | 'talk' | 'ask';
  page_index?: number | string; // e.g. '/current'
}

export interface NeedsInputPayload {
  content: string;
  reply_to: string;
}

export interface SSEEvent {
  id?: string;
  event: 'run.started' | 'thought' | 'tool_call' | 'tool_result' | 'progress' | 'token' | 'artifact' | 'needs_input' | 'info' | 'done' | 'error';
  data: any;
}
