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
}

const terminal = new Set(['failed', 'canceled', 'consumed']);

export const useExportStore = create<ExportState>((set, get) => {
  const patchOperation = (operation: ExportOperation) => set((state) => state.session && state.session.projectId === operation.project_id
    ? { session: { ...state.session, operation } }
    : state);

  const subscribe = (operation: ExportOperation) => {
    get().session?.streamClose?.();
    const close = subscribeExport(operation.id, (next) => {
      patchOperation(next);
      if (terminal.has(next.status)) {
        close();
        if (next.status === 'consumed' || next.status === 'canceled') window.setTimeout(() => set({ session: null }), 250);
      }
    }, () => {
      void exportsApi.get(operation.id).then((latest) => {
        patchOperation(latest);
        if (terminal.has(latest.status)) close();
      }).catch(() => {});
    });
    set((state) => state.session ? { session: { ...state.session, streamClose: close } } : state);
  };

  const start = async (projectId: string, format: ExportFormat) => {
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
        void exportsApi.get(session.operation.id).then(patchOperation).catch((error) => {
          if (error instanceof APIError && error.status === 410) set({ session: null });
        });
      }, 4_000);
    },
    retry: async () => {
      const session = get().session;
      if (!session) return;
      if (session.operation.id) await exportsApi.cancel(session.operation.id).catch(() => {});
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
