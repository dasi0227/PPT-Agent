import { fetchClient } from './client';
import type { CreateRunRequest } from './types';

export interface CheckpointInput {
  command: Pick<CreateRunRequest, 'instruction' | 'mode' | 'options'> & { attachments?: { id: string; original_name: string; size_bytes: number; media_type: 'image/png' | 'image/jpeg' | 'image/webp' }[] };
  scope_input: CreateRunRequest['scope'];
  model: string;
  skill_ids: string[];
  component_names: string[];
  mentioned_slide_ids: string[];
  dom_selections: CreateRunRequest['dom_selections'];
  reference_order: CreateRunRequest['reference_order'];
}
export interface HistoryState {
  revision: number;
  scene_revision: number;
  checkpoints: { run_id: string; thread_id: string; time: number; sequence: number }[];
  latest?: string;
  latest_time?: number;
  scene?: { thread_id?: string; input?: CheckpointInput; composer?: Record<string, unknown>; slide_id?: string; active_thread_id?: string; view?: 'html' | 'outline'; preview_mode?: 'main' | 'overview' };
}
export interface HistoryPreview {
  revision: number; time: number; input: string; runs: number;
}
export const projectHistoryApi = {
  state: (id: string) => fetchClient<HistoryState>(`/projects/${id}/history`, { reportError: false }),
  preview: (id: string, runId?: string) => fetchClient<HistoryPreview>(`/projects/${id}/history/preview${runId ? `?run_id=${encodeURIComponent(runId)}` : ''}`),
  switch: (id: string, preview: HistoryPreview, operation: string, scene: unknown, runId?: string) => fetchClient<HistoryState>(`/projects/${id}/history/switch`, {
    method: 'POST', timeoutMs: 120_000,
    body: JSON.stringify({ run_id: runId ?? '', revision: preview.revision, operation_id: operation, scene }),
  }),
};
