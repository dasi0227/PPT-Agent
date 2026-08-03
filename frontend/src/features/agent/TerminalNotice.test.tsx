import { act, fireEvent, render, screen } from '@testing-library/react';
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
});
