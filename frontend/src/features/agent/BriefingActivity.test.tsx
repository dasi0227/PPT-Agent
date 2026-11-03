import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useComposerStore } from '../../stores/composerStore';
import { useProjectStore } from '../../stores/projectStore';
import { useSessionStore } from '../../stores/sessionStore';
import { useThreadStore } from '../../stores/threadStore';
import type { BriefingTimelineItem } from './timelineTypes';
import { BriefingActivity } from './BriefingActivity';

const briefing: BriefingTimelineItem = {
  id: 'briefing:1',
  type: 'briefing',
  briefingId: 'briefing-1',
  kind: 'handoff',
  status: 'completed',
  timestamp: 1,
  versions: [{
    briefing_id: 'briefing-1',
    thread_id: 'thread-source',
    project_id: 'project-1',
    kind: 'handoff',
    version_no: 2,
    title: '交接剩余页面',
    content: '# Handoff v2\n\n继续完成当前项目。',
    feedback: '',
    created_at: 1,
  }],
};

describe('BriefingActivity', () => {
  beforeEach(() => {
    useProjectStore.setState({ activeProjectId: 'project-1' });
    useComposerStore.setState({ threadDrafts: {} });
    useSessionStore.setState({ sessions: {} });
  });

  it('creates a normal new thread and stages the latest content as its editable draft', async () => {
    const createThread = vi.fn().mockResolvedValue('thread-next');
    useThreadStore.setState({
      activeThreadIdByProjectId: { 'project-1': 'thread-source' },
      createThread,
    });

    render(<BriefingActivity item={briefing} />);
    fireEvent.click(screen.getByRole('button', { name: /handoff: 交接剩余页面/ }));
    fireEvent.click(screen.getByRole('button', { name: '新建会话' }));

    await waitFor(() => expect(createThread).toHaveBeenCalledWith('project-1'));
    expect(useComposerStore.getState().threadDrafts).toEqual({
      'thread-next': '# Handoff v2\n\n继续完成当前项目。',
    });
    expect(screen.queryByText(/来源：/)).not.toBeInTheDocument();
  });

  it('keeps long briefing history content collapsible without covering its footer actions', () => {
    const scrollHeight = vi.spyOn(HTMLElement.prototype, 'scrollHeight', 'get').mockImplementation(function (this: HTMLElement) {
      return this.dataset.testid === 'command-content-preview' ? 640 : 0;
    });
    const clientHeight = vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockImplementation(function (this: HTMLElement) {
      return this.dataset.testid === 'command-content-preview' ? 320 : 0;
    });

    try {
      render(<BriefingActivity item={briefing} />);
      fireEvent.click(screen.getByRole('button', { name: /handoff: 交接剩余页面/ }));

      fireEvent.click(screen.getByRole('button', { name: '展开' }));
      const collapse = screen.getByRole('button', { name: '收起' });
      expect(collapse).toBeInTheDocument();
      expect(collapse).toHaveAttribute('aria-expanded', 'true');
      const controlled = document.getElementById(collapse.getAttribute('aria-controls')!);
      expect(screen.getByTestId('command-content-preview')).toContainElement(controlled);
      expect(screen.getByRole('button', { name: '复制' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: '新建会话' })).toBeInTheDocument();
      fireEvent.click(collapse);
      expect(screen.getByRole('button', { name: '展开' })).toHaveAttribute('aria-expanded', 'false');
    } finally {
      scrollHeight.mockRestore();
      clientHeight.mockRestore();
    }
  });
});
