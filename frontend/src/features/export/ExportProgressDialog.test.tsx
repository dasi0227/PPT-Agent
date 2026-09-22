import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest';
import { useExportStore } from '../../stores/exportStore';
import { ExportProgressDialog } from './ExportProgressDialog';
import { exportsApi, normalizeExport, type ExportOperation } from '../../api/exports';

function readySession() {
  return {
    projectId: 'pro_one', format: 'pdf' as const, streamClose: null,
    operation: {
      id: 'exp_one', project_id: 'pro_one', format: 'pdf' as const, status: 'ready' as const,
      phase: 'packaging' as const, completed_pages: 3, total_pages: 3, warnings: [], events_url: '/events',
      artifact: { filename: 'Deck.pdf', mime_type: 'application/pdf', size_bytes: 1024, download_url: '/api/v1/exports/exp_one/download' },
    },
  };
}

beforeEach(() => {
  vi.spyOn(exportsApi, 'heartbeat').mockResolvedValue(undefined);
  vi.spyOn(exportsApi, 'get').mockResolvedValue(readySession().operation);
});
afterEach(() => { useExportStore.setState({ session: null }); vi.restoreAllMocks(); });

describe('ExportProgressDialog', () => {
  test.each(['escape', 'close button'])('exits a ready export via %s and cancels its temporary artifact', async (action) => {
    const cancel = vi.spyOn(exportsApi, 'cancel').mockResolvedValue(undefined);
    useExportStore.setState({ session: readySession() });
    render(<ExportProgressDialog />);
    if (action === 'escape') await userEvent.keyboard('{Escape}');
    else await userEvent.click(screen.getByRole('button', { name: '关闭导出' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(cancel).toHaveBeenCalledWith('exp_one');
    expect(useExportStore.getState().session).toBeNull();
  });

  test('renders a downloadable export when the received warning list is null', () => {
    const session = readySession();
    const received = { ...session.operation, warnings: null } as unknown as ExportOperation;
    useExportStore.setState({ session: { ...session, operation: normalizeExport(received) } });
    render(<ExportProgressDialog />);
    expect(screen.getByRole('button', { name: '下载' })).toBeInTheDocument();
  });
  test('does not auto-download and hands the ready artifact to the browser only after a click', async () => {
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
    useExportStore.setState({ session: readySession() });
    render(<ExportProgressDialog />);
    expect(screen.getByText('导出完成')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '终止' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '关闭导出' })).toBeInTheDocument();
    expect(screen.getByText('文件已准备好，点击下载到本地。')).toBeInTheDocument();
    expect(click).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole('button', { name: '下载' }));
    expect(click).toHaveBeenCalledTimes(1);
    expect(useExportStore.getState().session?.operation.status).toBe('delivering');
    click.mockRestore();
  });

  test('offers termination while rendering and closes only after the backend acknowledges it', async () => {
    const session = { ...readySession(), operation: { ...readySession().operation, status: 'running' as const, phase: 'rendering' as const } };
    vi.mocked(exportsApi.get).mockResolvedValue(session.operation);
    let resolveCancel!: () => void;
    const cancel = vi.spyOn(exportsApi, 'cancel').mockReturnValue(new Promise<void>((resolve) => { resolveCancel = resolve; }));
    useExportStore.setState({ session });
    render(<ExportProgressDialog />);
    await userEvent.click(screen.getByRole('button', { name: '终止' }));
    expect(screen.getByRole('button', { name: '正在终止…' })).toBeDisabled();
    expect(screen.getByText('正在终止导出')).toBeInTheDocument();
    expect(cancel).toHaveBeenCalledWith('exp_one');
    resolveCancel();
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });

  test('lists every missing page and never exposes a download action', () => {
    useExportStore.setState({ session: {
      projectId: 'pro_one', format: 'html', streamClose: null,
      operation: { id: '', project_id: 'pro_one', format: 'html', status: 'failed', completed_pages: 0, total_pages: 0, warnings: [], events_url: '', error: {
        code: 'EXPORT_SLIDES_MISSING', message: '有 2 页尚未生成，无法导出完整演示文稿。', retryable: false,
        details: { slides: [{ slide_id: 's1', ordinal: 2, title: '分析' }, { slide_id: 's2', ordinal: 5, title: '结论' }] },
      } },
    } });
    render(<ExportProgressDialog />);
    expect(screen.getByText('第 2 页 · 分析')).toBeInTheDocument();
    expect(screen.getByText('第 5 页 · 结论')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '下载' })).not.toBeInTheDocument();
  });
});
