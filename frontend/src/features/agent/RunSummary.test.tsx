import { act, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { RunStatus } from '../../stores/runStore';
import { useRunStore } from '../../stores/runStore';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { RunSummary } from './RunSummary';

function setStatus(status: RunStatus) {
  useRunStore.setState({
    sessions: {
      t1: {
        activeRunId: 'r1', projectId: 'p1', status,
        streamStatus: status === 'running' ? 'open' : 'closed',
        target: { artifact: 'presentation', level: 'slide' },
        interaction: { intent: 'execute' },
        timelineItems: [],
        pendingQuestion: status === 'waiting' ? { id: 'q1', prompt: '确认' } : null,
        progress: { stage: 'finalizing', text: '正在完成最终检查' },
        eventSourceClose: null,
        plan: {
          id: 'plan', title: '执行计划', revision: 1,
          steps: [
            { id: 'one', title: '一步', status: 'completed' },
            { id: 'two', title: '二步', status: 'in_progress' },
          ],
        },
      },
    },
  });
}

describe('RunSummary product status', () => {
  it('shows target, status, plan count, and reconnect state without runtime internals', () => {
    useProjectStore.setState({ activeProjectId: 'p1' });
    useThreadStore.setState({ activeThreadIdByProjectId: { p1: 't1' } });
    setStatus('running');
    render(<RunSummary />);
    expect(screen.getByText('运行中')).toBeInTheDocument();
    expect(screen.getByText('1 / 2')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /停止/ })).toBeInTheDocument();
    expect(screen.queryByText('计划执行')).toBeNull();
    expect(screen.queryByText('完成检查')).toBeNull();

    for (const [status, label] of [
      ['waiting', '等待回答'],
      ['done', '已完成'],
      ['error', '运行失败'],
      ['canceled', '已取消'],
    ] as const) {
      act(() => setStatus(status));
      expect(screen.getByText(label)).toBeInTheDocument();
    }
  });
});
