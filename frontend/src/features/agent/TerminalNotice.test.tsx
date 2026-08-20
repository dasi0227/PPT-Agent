import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useProjectStore } from '../../stores/projectStore';
import { useRunStore } from '../../stores/runStore';
import { useThreadStore } from '../../stores/threadStore';
import { TerminalNotice } from './TerminalNotice';

describe('TerminalNotice retry authority', () => {
  const retryRun = vi.fn();

  beforeEach(() => {
    retryRun.mockReset();
    retryRun.mockResolvedValue(true);
    act(() => {
      useProjectStore.setState({ activeProjectId: 'p1' });
      useThreadStore.setState({ activeThreadIdByProjectId: { p1: 't1' } });
      useRunStore.setState({ retryRun });
    });
  });

  it('shows retry only when the authoritative error says retryable=true', () => {
    render(<TerminalNotice item={{
      id: 'terminal',
      type: 'terminal_notice',
      status: 'failed',
      message: '模型服务暂时不可用',
      error: { code: 'PROVIDER_UNAVAILABLE', message: '模型服务暂时不可用', retryable: true },
      timestamp: 1,
    }} />);

    fireEvent.click(screen.getByRole('button', { name: '重试（创建新任务）' }));
    expect(retryRun).toHaveBeenCalledWith('t1');
  });

  it.each([
    { label: 'false', retryable: false },
    { label: 'missing', retryable: undefined },
  ])('hides retry when retryable is $label', ({ retryable }) => {
    render(<TerminalNotice item={{
      id: `terminal-${String(retryable)}`,
      type: 'terminal_notice',
      status: 'failed',
      message: '页面检查未通过',
      error: retryable === undefined
        ? undefined
        : { code: 'RENDER_FAILED', message: '页面检查未通过', retryable },
      timestamp: 1,
    }} />);

    expect(screen.queryByRole('button', { name: '重试（创建新任务）' })).not.toBeInTheDocument();
    expect(screen.getByText('页面检查未通过')).toBeInTheDocument();
  });

  it('shows copy and a hover-only timestamp for terminal messages', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText } });
    render(<TerminalNotice item={{
      id: 'terminal-copy', type: 'terminal_notice', status: 'failed',
      message: '连续修正未成功，任务已停止。', timestamp: new Date(2026, 7, 11, 14, 5).getTime(),
    }} />);

    const copy = screen.getByRole('button', { name: '复制消息' });
    expect(screen.getByText('08-11 14-05')).toBeInTheDocument();
    expect(copy.parentElement).toHaveClass('opacity-0', 'group-hover:opacity-100');
    fireEvent.click(copy);
    await waitFor(() => expect(writeText).toHaveBeenCalledWith('连续修正未成功，任务已停止。'));
  });
});
