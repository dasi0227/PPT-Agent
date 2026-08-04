import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { ThreadTabs } from './ThreadTabs';

describe('ThreadTabs keyboard access', () => {
  it('switches threads with Enter and exposes selected tab state', () => {
    useProjectStore.setState({ activeProjectId: 'p1' });
    useThreadStore.setState({
      threadsByProjectId: {
        p1: [
          { id: 't1', project_id: 'p1', title: '会话一', status: 'active', history_path: '', created_at: 1, updated_at: 1 },
          { id: 't2', project_id: 'p1', title: '会话二', status: 'active', history_path: '', created_at: 1, updated_at: 1 },
        ],
      },
      openThreadIdsByProjectId: { p1: ['t1', 't2'] },
      activeThreadIdByProjectId: { p1: 't1' },
      errorByProjectId: {},
    });
    render(<ThreadTabs />);
    const first = screen.getByRole('tab', { name: /会话一/ });
    const second = screen.getByRole('tab', { name: /会话二/ });
    expect(first).toHaveAttribute('aria-selected', 'true');
    fireEvent.keyDown(second, { key: 'Enter' });
    expect(useThreadStore.getState().activeThreadIdByProjectId.p1).toBe('t2');
    expect(second).toHaveAttribute('aria-selected', 'true');
  });
});

describe('ThreadTabs create thread', () => {
  it('creates a new thread when the plus button is clicked', async () => {
    const createThread = vi.fn().mockResolvedValue('t-new');
    useProjectStore.setState({ activeProjectId: 'p1' });
    useThreadStore.setState({
      threadsByProjectId: {
        p1: [{ id: 't1', project_id: 'p1', title: '会话一', status: 'active', history_path: '', created_at: 1, updated_at: 1 }],
      },
      openThreadIdsByProjectId: { p1: ['t1'] },
      activeThreadIdByProjectId: { p1: 't1' },
      errorByProjectId: {},
      createThread,
    });
    render(<ThreadTabs />);
    const addButton = screen.getByRole('button', { name: '新建会话' });
    fireEvent.click(addButton);
    expect(createThread).toHaveBeenCalledWith('p1');
    // 等待 createThread 的异步 finally 完成状态回落，避免 act 警告
    await screen.findByRole('button', { name: '新建会话' });
  });

  it('offers the plus button even when there are no threads', () => {
    useProjectStore.setState({ activeProjectId: 'p1' });
    useThreadStore.setState({
      threadsByProjectId: { p1: [] },
      openThreadIdsByProjectId: { p1: [] },
      activeThreadIdByProjectId: { p1: null },
      errorByProjectId: {},
    });
    render(<ThreadTabs />);
    expect(screen.getByText('暂无会话')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '新建会话' })).toBeInTheDocument();
  });
});
