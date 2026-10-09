import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { runsApi } from '../../api/runs';
import { useRunStore } from '../../stores/runStore';
import { TerminalNotice } from './TerminalNotice';

vi.mock('./useActiveSession', () => ({
  useActiveThreadId: () => 't1',
  useActiveSession: () => ({ status: 'error', activeRunId: null }),
}));
vi.mock('../../api/runs', () => ({ runsApi: { get: vi.fn() } }));

const item = {
  id: 'terminal', type: 'terminal_notice' as const, runId: 'r1',
  status: 'failed' as const, message: '任务已达到执行轮次上限。', affectedTargets: [], timestamp: 1,
  error: { code: 'BUDGET_EXCEEDED', message: '任务已达到执行轮次上限。', retryable: false, details: { secret: 'technical detail' } },
};

describe('TerminalNotice continuation authority', () => {
  const resumeRun = vi.fn();
  beforeEach(() => {
    vi.clearAllMocks();
    resumeRun.mockResolvedValue(true);
    useRunStore.setState({ resumeRun });
    vi.mocked(runsApi.get).mockResolvedValue({ can_continue: true } as Awaited<ReturnType<typeof runsApi.get>>);
  });

  it.each(['failed', 'canceled'] as const)('continues the same eligible %s run only once', async status => {
    render(<TerminalNotice item={{ ...item, status }} />);
    const button = await screen.findByRole('button', { name: '继续执行' });
    fireEvent.click(button);
    fireEvent.click(button);
    expect(resumeRun).toHaveBeenCalledTimes(1);
    expect(resumeRun).toHaveBeenCalledWith('t1', 'r1');
    expect(screen.getByRole('button', { name: '正在继续' })).toBeDisabled();
    if (status === 'canceled') expect(screen.getByText('你已停止本次任务。')).toBeInTheDocument();
  });

  it('hides continuation when the backend rejects eligibility', async () => {
    vi.mocked(runsApi.get).mockResolvedValue({ can_continue: false } as Awaited<ReturnType<typeof runsApi.get>>);
    render(<TerminalNotice item={item} />);
    await waitFor(() => expect(runsApi.get).toHaveBeenCalledWith('r1'));
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('shows only a semantic reason for an ineligible exception, regardless of retryable', async () => {
    vi.mocked(runsApi.get).mockResolvedValue({ can_continue: false } as Awaited<ReturnType<typeof runsApi.get>>);
    render(<TerminalNotice item={{ ...item, status: 'error', error: { ...item.error, retryable: true } }} />);
    await waitFor(() => expect(runsApi.get).toHaveBeenCalledWith('r1'));
    expect(screen.getByText('任务执行异常')).toBeInTheDocument();
    expect(screen.getByText(item.message)).toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
    expect(screen.queryByText(/technical detail|已更改内容|BUDGET_EXCEEDED/)).not.toBeInTheDocument();
  });

  it('uses a terminal provider message and backend continuation authority even when retryable is false', async () => {
    render(<TerminalNotice item={{ ...item,
      message: '模型服务暂时不可用，正在尝试恢复。',
      error: { code: 'PROVIDER_UNAVAILABLE', message: '模型服务暂时不可用，正在尝试恢复。', retryable: false },
    }} />);
    expect(screen.getByText('模型服务请求失败。')).toBeInTheDocument();
    expect(screen.queryByText(/正在尝试恢复/)).not.toBeInTheDocument();
    fireEvent.click(await screen.findByRole('button', { name: '继续执行' }));
    expect(resumeRun).toHaveBeenCalledWith('t1', 'r1');
  });

  it('keeps a rejected continuation stopped and hides the unavailable action', async () => {
    resumeRun.mockResolvedValue(false);
    render(<TerminalNotice item={item} />);
    fireEvent.click(await screen.findByRole('button', { name: '继续执行' }));
    await waitFor(() => expect(screen.queryByRole('button')).not.toBeInTheDocument());
    expect(screen.queryByText('正在继续')).not.toBeInTheDocument();
  });
});
