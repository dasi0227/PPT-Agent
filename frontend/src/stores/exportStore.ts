import { create } from 'zustand';
import { exportError, exportsApi, subscribeExport, type ExportFormat, type ExportOperation } from '../api/exports';
import { newClientIdentity } from '../lib/clientIdentity';
import { APIError, NetworkError, RequestTimeoutError } from '../api/client';

export interface ExportSession {
  projectId: string;
  format: ExportFormat;
  operation: ExportOperation;
  streamClose: (() => void) | null;
}

interface ExportState {
  session: ExportSession | null;
  start: (projectId: string, format: ExportFormat) => Promise<void>;
  download: () => void;
  retry: () => Promise<void>;
  close: () => Promise<void>;
  refresh: (id: string) => Promise<void>;
}

const terminal = new Set(['failed', 'canceled', 'consumed']);

export const useExportStore = create<ExportState>((set, get) => {
  const patchOperation = (operation: ExportOperation) => {
    const session = get().session;
    if (!session || session.operation.id !== operation.id) return;
    if (terminal.has(session.operation.status) && !terminal.has(operation.status)) return;
    if (terminal.has(operation.status)) session.streamClose?.();
    if (operation.status === 'consumed') {
      set({ session: null });
      return;
    }
    const next = operation.status === 'canceled'
      ? { ...operation, id: '', status: 'failed' as const, error: operation.error ?? { code: 'EXPORT_EXPIRED', message: '导出连接已失效，请重新导出。', retryable: true } }
      : operation;
    set({ session: { ...session, operation: next, streamClose: terminal.has(operation.status) ? null : session.streamClose } });
  };

  const refresh = async (id: string) => {
    if (get().session?.operation.id !== id) return;
    try {
      patchOperation(await exportsApi.get(id));
    } catch (error) {
      const session = get().session;
      if (session?.operation.id !== id) return;
      if (error instanceof APIError && error.status === 410) {
        session.streamClose?.();
        if (session.operation.status === 'delivering') {
          set({ session: null });
        } else {
          set({ session: { ...session, streamClose: null, operation: { ...session.operation, id: '', status: 'failed', error: exportError(error) } } });
        }
      }
    }
  };

  const subscribe = (operation: ExportOperation) => {
    get().session?.streamClose?.();
    if (terminal.has(operation.status)) { patchOperation(operation); return; }
    const close = subscribeExport(operation.id, patchOperation, () => { void refresh(operation.id); });
    set((state) => state.session ? { session: { ...state.session, streamClose: close } } : state);
  };

  const start = async (projectId: string, format: ExportFormat) => {
    const current = get().session;
    if (current && !terminal.has(current.operation.status)) return;
    get().session?.streamClose?.();
    const placeholder: ExportOperation = { id: '', project_id: projectId, format, status: 'accepted', phase: 'snapshotting', completed_pages: 0, total_pages: 0, warnings: [], events_url: '' };
    set({ session: { projectId, format, operation: placeholder, streamClose: null } });
    const requestId = newClientIdentity('req');
    try {
      const operation = await exportsApi.create(projectId, format, requestId);
      set({ session: { projectId, format, operation, streamClose: null } });
      subscribe(operation);
    } catch (error) {
      let finalError = error;
      if (error instanceof NetworkError || error instanceof RequestTimeoutError) {
        try {
          const operation = await exportsApi.create(projectId, format, requestId);
          set({ session: { projectId, format, operation, streamClose: null } });
          subscribe(operation);
          return;
        } catch (retryError) {
          finalError = retryError;
        }
      }
      set({ session: { projectId, format, streamClose: null, operation: { ...placeholder, status: 'failed', error: exportError(finalError) } } });
    }
  };

  return {
    session: null,
    start,
    refresh,
    download: () => {
      const session = get().session;
      const url = session?.operation.artifact?.download_url;
      if (!session || !url || session.operation.status !== 'ready') return;
      patchOperation({ ...session.operation, status: 'delivering' });
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.style.display = 'none';
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      window.setTimeout(() => {
        const current = get().session;
        if (!current || current.operation.id !== session.operation.id || current.operation.status !== 'delivering') return;
        void refresh(session.operation.id);
      }, 4_000);
    },
    retry: async () => {
      const session = get().session;
      if (!session) return;
      session.streamClose?.();
      if (session.operation.id) {
        try { await exportsApi.cancel(session.operation.id); }
        catch (error) {
          if (!(error instanceof APIError && error.status === 410)) {
            patchOperation({ ...session.operation, status: 'failed', error: { ...exportError(error), message: '未能清理上次导出，请检查连接后重试。' } });
            return;
          }
        }
      }
      const current = get().session;
      if (!current || current.projectId !== session.projectId || current.operation.id !== session.operation.id) return;
      await start(session.projectId, session.format);
    },
    close: async () => {
      const session = get().session;
      session?.streamClose?.();
      set({ session: null });
      if (session?.operation.id) await exportsApi.cancel(session.operation.id).catch(() => {});
    },
  };
});

export function isProjectExportBlocking(projectId: string | null): boolean {
  const session = useExportStore.getState().session;
  return !!projectId && session?.projectId === projectId && !terminal.has(session.operation.status);
}
