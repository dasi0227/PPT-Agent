import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useComposerStore } from '../../stores/composerStore';
import { useGitCommitStore } from '../../stores/gitCommitStore';
import { useProjectStore } from '../../stores/projectStore';
import { useRunStore } from '../../stores/runStore';
import { useThreadStore } from '../../stores/threadStore';
import type { BriefingTimelineItem } from './eventReducer';
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
    content: '# Handoff v2\n\n继续完成当前项目。',
    feedback: '',
    created_at: 1,
  }],
};

describe('BriefingActivity', () => {
  beforeEach(() => {
    useProjectStore.setState({ activeProjectId: 'project-1' });
    useComposerStore.setState({ threadDrafts: {} });
    useRunStore.setState({ sessions: {} });
    useGitCommitStore.setState({ sessions: {} });
  });

  it('creates a normal new thread and stages the selected version as its editable draft', async () => {
    const createThread = vi.fn().mockResolvedValue('thread-next');
    useThreadStore.setState({
      activeThreadIdByProjectId: { 'project-1': 'thread-source' },
      createThread,
    });

    render(<BriefingActivity item={briefing} />);
    fireEvent.click(screen.getByRole('button', { name: '以此版本新建会话' }));

    await waitFor(() => expect(createThread).toHaveBeenCalledWith('project-1'));
    expect(useComposerStore.getState().threadDrafts).toEqual({
      'thread-next': '# Handoff v2\n\n继续完成当前项目。',
    });
    expect(screen.queryByText(/来源：/)).not.toBeInTheDocument();
  });
});
