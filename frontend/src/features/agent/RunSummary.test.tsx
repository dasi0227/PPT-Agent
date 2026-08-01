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
        activeRunId: 'r1',
        projectId: 'p1',
        status,
        streamStatus: status === 'running' ? 'open' : 'closed',
        target: { artifact: 'presentation', level: 'slide' },
        interaction: { intent: 'apply', clarification: 'when_blocked' },
        timelineItems: [],
        pendingInput: status === 'needs_input' ? { id: 'q1', prompt: '确认' } : null,
        progress: { stage: 'verify' },
        eventSourceClose: null,
        strategy: 'compact_workflow',
        plan: {
          id: 'plan',
          title: '执行计划',
          steps: [
            { id: 'one', title: '一步', status: 'completed' },
            { id: 'two', title: '二步', status: 'in_progress' },
          ],
        },
      },
    },
  });
}

describe('RunSummary status presentation', () => {
  it('shows accurate running, input, terminal, stage, and plan states', () => {
    useProjectStore.setState({ activeProjectId: 'p1' });
    useThreadStore.setState({ activeThreadIdByProjectId: { p1: 't1' } });
    setStatus('running');
    render(<RunSummary />);
    expect(screen.getByText('运行中')).toBeInTheDocument();
    expect(screen.getByText('轻量工作流')).toBeInTheDocument();
    expect(screen.getByText('验证结果')).toBeInTheDocument();
    expect(screen.getByText('1 / 2')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /停止/ })).toBeInTheDocument();

    for (const [status, label] of [
      ['needs_input', '等待输入'],
      ['done', '已完成'],
      ['error', '运行失败'],
      ['canceled', '已取消'],
    ] as const) {
      act(() => setStatus(status));
      expect(screen.getByText(label)).toBeInTheDocument();
    }
    expect(screen.queryByRole('button', { name: /停止/ })).toBeNull();
  });
});
