export interface Project {
  id: string;
  title: string;
  theme: string;
  slide_count: number;
  created_at: string;
  updated_at: string;
}

export interface Slide {
  id: string;
  project_id: string;
  page_index: number;
  content_html: string;
  notes?: string;
  version_no: number;
  updated_at: string;
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
