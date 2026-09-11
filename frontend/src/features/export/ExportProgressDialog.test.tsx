import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, test, vi } from 'vitest';
import { useExportStore } from '../../stores/exportStore';
import { ExportProgressDialog } from './ExportProgressDialog';

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

afterEach(() => useExportStore.setState({ session: null }));

describe('ExportProgressDialog', () => {
  test('does not auto-download and hands the ready artifact to the browser only after a click', async () => {
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
    useExportStore.setState({ session: readySession() });
    render(<ExportProgressDialog />);
    expect(screen.getByText('导出完成')).toBeInTheDocument();
    expect(click).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole('button', { name: '下载' }));
    expect(click).toHaveBeenCalledTimes(1);
    expect(useExportStore.getState().session?.operation.status).toBe('delivering');
    click.mockRestore();
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
