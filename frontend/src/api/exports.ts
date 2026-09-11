import { APIError, fetchClient } from './client';

export type ExportFormat = 'png' | 'pdf' | 'html';
export type ExportStatus = 'accepted' | 'running' | 'ready' | 'delivering' | 'consumed' | 'failed' | 'canceled';
export type ExportPhase = 'snapshotting' | 'rendering' | 'packaging';

export interface ExportSlideIssue { slide_id: string; ordinal: number; title: string }
export interface ExportPublicError { code: string; message: string; details?: { slides?: ExportSlideIssue[] }; retryable: boolean }
export interface ExportArtifact { filename: string; mime_type: string; size_bytes: number; download_url: string }
export interface ExportOperation {
  id: string;
  project_id: string;
  format: ExportFormat;
  status: ExportStatus;
  phase?: ExportPhase;
  completed_pages: number;
  total_pages: number;
  current_slide_id?: string;
  current_ordinal?: number;
  warnings: string[];
  artifact?: ExportArtifact;
  error?: ExportPublicError;
  events_url: string;
}

export const exportsApi = {
  create: (projectId: string, format: ExportFormat, clientRequestId: string) => fetchClient<ExportOperation>(`/projects/${projectId}/exports`, {
    method: 'POST', body: JSON.stringify({ format, client_request_id: clientRequestId }), timeoutMs: 30_000,
  }),
  get: (id: string) => fetchClient<ExportOperation>(`/exports/${id}`, { reportError: false }),
  cancel: (id: string, keepalive = false) => fetch(`/api/v1/exports/${encodeURIComponent(id)}`, { method: 'DELETE', keepalive }),
};

const eventNames = ['export.progress', 'export.ready', 'export.delivery_started', 'export.download_failed', 'export.consumed', 'export.failed', 'export.canceled'] as const;

export function subscribeExport(id: string, onMessage: (operation: ExportOperation) => void, onError: () => void): () => void {
  const source = new EventSource(`/api/v1/exports/${encodeURIComponent(id)}/events`);
  for (const name of eventNames) {
    source.addEventListener(name, (event) => {
      try { onMessage(JSON.parse((event as MessageEvent).data) as ExportOperation); } catch { onError(); }
    });
  }
  source.onerror = onError;
  return () => source.close();
}

export function exportError(error: unknown): ExportPublicError {
  if (error instanceof APIError) {
    const details = error.details && typeof error.details === 'object' ? error.details as { slides?: ExportSlideIssue[] } : undefined;
    return { code: error.code ?? 'EXPORT_FAILED', message: error.message, details, retryable: error.retryable };
  }
  return { code: 'EXPORT_FAILED', message: error instanceof Error ? error.message : '导出失败，请重试。', retryable: true };
}
