import { create } from 'zustand';
import { exportsApi, type ExportFormat } from '../api/exports';

type ExportSession = { projectId: string; format: ExportFormat } & (
  | { status: 'loading' }
  | { status: 'error'; message: string }
);

interface ExportState {
  session: ExportSession | null;
  start: (projectId: string, format: ExportFormat) => Promise<void>;
  retry: () => Promise<void>;
  close: () => void;
}

export const useExportStore = create<ExportState>((set, get) => ({
  session: null,
  start: async (projectId, format) => {
    if (get().session?.status === 'loading') return;
    set({ session: { projectId, format, status: 'loading' } });
    try {
      const { blob, filename } = await exportsApi.create(projectId, format);
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = filename;
      anchor.hidden = true;
      document.body.appendChild(anchor);
      try {
        anchor.click();
      } finally {
        anchor.remove();
        window.setTimeout(() => URL.revokeObjectURL(url), 1_000);
      }
      set({ session: null });
    } catch (error) {
      set({ session: { projectId, format, status: 'error', message: error instanceof Error ? error.message : '导出失败，请重试' } });
    }
  },
  retry: async () => {
    const session = get().session;
    if (session?.status === 'error') await get().start(session.projectId, session.format);
  },
  close: () => {
    if (get().session?.status === 'error') set({ session: null });
  },
}));
