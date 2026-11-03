import { fetchClient } from './client';
import type { CreateRunRequest } from './types';
import { isRevision, type Revision } from './session';

export interface CheckpointInput {
  command: Pick<CreateRunRequest, 'instruction' | 'mode'> & { attachments?: { id: string; original_name: string; size_bytes: number; media_type: 'image/png' | 'image/jpeg' }[] };
  scope_input: CreateRunRequest['scope'];
  model: string;
  skill_ids: string[];
  component_names: string[];
  mentioned_slide_ids: string[];
  dom_selections: CreateRunRequest['dom_selections'];
  reference_order: CreateRunRequest['reference_order'];
}
export interface HistoryState {
  revision: Revision;
  scene_revision: Revision;
  checkpoints: { run_id: string; thread_id: string; time: number }[];
  latest?: string;
  latest_time?: number;
  scene?: { thread_id?: string; input?: CheckpointInput; composer?: Record<string, unknown>; slide_id?: string; active_thread_id?: string; view?: 'html' | 'outline'; preview_mode?: 'main' | 'overview'; content_mode?: 'preview' | 'source' };
}
export interface HistoryPreview {
  revision: Revision; scene_revision: Revision; time: number; input: string; runs: number;
}
function historyVersions<T extends { revision: Revision; scene_revision: Revision }>(value: T): T {
  if (!isRevision(value.revision) || value.revision === '0' || !isRevision(value.scene_revision) || value.scene_revision === '0') {
    throw new Error('Invalid project history version');
  }
  return value;
}
export const projectHistoryApi = {
  state: (id: string) => fetchClient<HistoryState>(`/projects/${id}/history`, { reportError: false }).then(historyVersions),
  preview: (id: string, runId?: string) => fetchClient<HistoryPreview>(`/projects/${id}/history/preview${runId ? `?run_id=${encodeURIComponent(runId)}` : ''}`).then(historyVersions),
  switch: (id: string, preview: HistoryPreview, operation: string, scene: unknown, runId?: string) => fetchClient<HistoryState>(`/projects/${id}/history/switch`, {
    method: 'POST', timeoutMs: 120_000,
    body: JSON.stringify({ run_id: runId ?? '', revision: preview.revision, scene_revision: preview.scene_revision, operation_id: operation, scene }),
  }).then(historyVersions),
};
