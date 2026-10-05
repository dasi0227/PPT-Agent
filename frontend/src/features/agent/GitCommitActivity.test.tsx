import { fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, test } from 'vitest';
import { useGitCommitStore } from '../../stores/gitCommitStore';
import { useProjectStore } from '../../stores/projectStore';
import type { GitCommitTimelineItem } from './eventReducer';
import { GitCommitEvent, GitCommitProgress } from './GitCommitActivity';

beforeEach(() => {
  useProjectStore.setState({ activeProjectId: 'p1' });
  useGitCommitStore.setState({ sessions: {} });
});

describe('Git commit timeline presentation', () => {
  test('renders real progress phases with centered labels', () => {
    useGitCommitStore.setState({ sessions: { p1: {
      operationId: 'cmd_1', sourceThreadId: 't1', status: 'running', phase: 'analyzing',
      displayPhase: 1, streamClose: null,
    } } });
    render(<GitCommitProgress phase="analyzing" />);
    expect(screen.getByText('生成说明').closest('[aria-current="step"]')).not.toBeNull();
    expect(screen.getByText('整理变更')).toBeInTheDocument();
    expect(screen.getByText('写入版本')).toBeInTheDocument();
  });

  test('shows grouped metadata and reveals only summary items', () => {
    const item: GitCommitTimelineItem = {
      id: 'git-commit:gco_1',
      type: 'git_commit',
      operationId: 'gco_1',
      status: 'completed',
      title: 'fix: 优化增长图表标签布局',
      items: ['强化季度增长趋势的视觉层级', '消除数据标签重叠'],
      branch: 'main',
      hash: '8af42d9',
      filesChanged: 3,
      insertions: 46,
      deletions: 18,
      timestamp: new Date('2026-08-30T06:29:08.000Z').getTime(),
    };
    render(<GitCommitEvent item={item} />);
    expect(screen.getByText(item.title!)).toBeInTheDocument();
    expect(screen.getByText(/main 8af42d9/)).toBeInTheDocument();
    expect(screen.queryByText('消除数据标签重叠')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /fix: 优化增长图表标签布局/ }));
    expect(screen.getByText('消除数据标签重叠')).toBeInTheDocument();
  });

  test('uses the compact retryable failure state', () => {
    render(<GitCommitEvent item={{
      id: 'git-commit:gco_2',
      type: 'git_commit',
      operationId: 'gco_2',
      status: 'failed',
      retryable: true,
      timestamp: Date.now(),
    }} />);
    expect(screen.getByText('项目版本提交失败')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '重试' })).toBeInTheDocument();
  });
});
