import { create } from 'zustand';
import { exportError, exportsApi, subscribeExport, type ExportFormat, type ExportOperation } from '../api/exports';
import { newClientIdentity } from '../lib/clientIdentity';
import { APIError, NetworkError, RequestTimeoutError } from '../api/client';

export interface ExportSession {
  projectId: string;
  format: ExportFormat;
  operation: ExportOperation;
  streamClose: (() => void) | null;
  requestId?: string;
  cancelRequested?: boolean;
  canceling?: boolean;
  cancelError?: string;
  dismissed?: boolean;
}

interface ExportState {
  session: ExportSession | null;
  start: (projectId: string, format: ExportFormat) => Promise<void>;
  download: () => void;
  retry: () => Promise<void>;
  close: () => Promise<void>;
  cancel: () => Promise<void>;
  refresh: (id: string) => Promise<void>;
}

const terminal = new Set(['failed', 'canceled', 'consumed']);

export const useExportStore = create<ExportState>((set, get) => {
  const patchOperation = (operation: ExportOperation) => {
    const session = get().session;
    if (!session || session.operation.id !== operation.id || session.canceling) return;
    if (terminal.has(session.operation.status) && !terminal.has(operation.status)) return;
    if (terminal.has(operation.status)) session.streamClose?.();
    if (operation.status === 'consumed' || (operation.status === 'canceled' && session.cancelRequested)) {
      set({ session: null });
      return;
    }
    if (session.dismissed && operation.status !== 'delivering') {
      // The user already left the download dialog. A failed transfer must not
      // leave a ready artifact reserved indefinitely with no visible owner.
      set({ session: { ...session, operation, cancelRequested: true, canceling: true } });
      void cancelOperation(operation.id);
      return;
    }
    const next = operation.status === 'canceled'
      ? { ...operation, id: '', status: 'failed' as const, error: operation.error ?? { code: 'EXPORT_EXPIRED', message: '导出连接已失效，请重新导出。', retryable: true } }
      : operation;
    set({ session: { ...session, operation: next, streamClose: terminal.has(operation.status) ? null : session.streamClose } });
  };

  const refresh = async (id: string) => {
    if (get().session?.operation.id !== id || get().session?.canceling) return;
    try {
      patchOperation(await exportsApi.get(id));
    } catch (error) {
      const session = get().session;
      if (session?.operation.id !== id || session.canceling) return;
      if (error instanceof APIError && error.status === 410) {
        session.streamClose?.();
        if (session.operation.status === 'delivering' || session.cancelRequested || session.dismissed) {
          set({ session: null });
        } else {
          set({ session: { ...session, streamClose: null, operation: { ...session.operation, id: '', status: 'failed', error: exportError(error) } } });
        }
      }
    }
  };

  const cancelOperation = async (id: string) => {
    try {
      await exportsApi.cancel(id);
    } catch (error) {
      if (!(error instanceof APIError && error.status === 410)) {
        const current = get().session;
        if (current?.operation.id === id) {
          set({ session: { ...current, dismissed: false, canceling: false, cancelError: '终止失败，请检查连接后重试。' } });
          // The response may be lost after the backend has already canceled.
          await refresh(id);
        }
        return;
      }
    }
    const current = get().session;
    if (current?.operation.id !== id) return;
    current.streamClose?.();
    set({ session: null });
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
    const requestId = newClientIdentity('req');
    set({ session: { projectId, format, requestId, operation: placeholder, streamClose: null } });
    const accept = async (operation: ExportOperation) => {
      const current = get().session;
      if (current?.requestId !== requestId) return;
      set({ session: { ...current, operation } });
      subscribe(operation);
      if (current.cancelRequested) await cancelOperation(operation.id);
    };
    try {
      const operation = await exportsApi.create(projectId, format, requestId);
      await accept(operation);
    } catch (error) {
      let finalError = error;
      if (error instanceof NetworkError || error instanceof RequestTimeoutError) {
        try {
          const operation = await exportsApi.create(projectId, format, requestId);
          await accept(operation);
          return;
        } catch (retryError) {
          finalError = retryError;
        }
      }
      if (get().session?.requestId !== requestId) return;
      set({ session: { projectId, format, requestId, streamClose: null, operation: { ...placeholder, status: 'failed', error: exportError(finalError) } } });
    }
  };

  return {
    session: null,
    start,
    refresh,
    cancel: async () => {
      const session = get().session;
      if (!session || session.canceling || terminal.has(session.operation.status) || session.operation.status === 'delivering') return;
      set({ session: { ...session, cancelRequested: true, canceling: true, cancelError: undefined } });
      // Creation may still be freezing the snapshot. Keep its request alive so
      // accept() can cancel the actual ID instead of leaving an orphan task.
      if (session.operation.id) await cancelOperation(session.operation.id);
    },
    download: () => {
      const session = get().session;
      const url = session?.operation.artifact?.download_url;
      if (!session || session.canceling || !url || session.operation.status !== 'ready') return;
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
      if (!session || session.canceling) return;
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
      if (!session || session.canceling) return;
      if (session.operation.status === 'delivering') {
        // Do not delete a file the browser is still receiving. Keep the event
        // subscription until consumption, or cancel if delivery returns to ready.
        set({ session: { ...session, dismissed: true } });
        return;
      }
      if (!terminal.has(session.operation.status)) {
        await get().cancel();
      } else if (session.operation.id) {
        set({ session: { ...session, cancelRequested: true, canceling: true, cancelError: undefined } });
        await cancelOperation(session.operation.id);
      } else {
        session.streamClose?.();
        set({ session: null });
      }
    },
  };
});

export function isProjectExportBlocking(projectId: string | null): boolean {
  const session = useExportStore.getState().session;
  return !!projectId && session?.projectId === projectId && !terminal.has(session.operation.status);
}
