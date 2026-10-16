import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { runsApi } from '../../api/runs';
import { IDLE_SESSION, useRunStore } from '../../stores/runStore';
import { RunCommandActivity } from './RunCommandActivity';
import type { ToolActivityItem } from './eventReducer';

vi.mock('./useActiveSession', () => ({
  useActiveThreadId: () => 'thread-1',
}));

const item: ToolActivityItem = {
  id: 'r1:tool:c1', type: 'tool', runId: 'r1', callId: 'c1', tool: 'run_command',
  label: '执行命令', status: 'running', timestamp: 1,
  command: { text: "sed -i '' 's/old/new/g' notes.txt", stdout_preview: 'saved' },
  commandApproval: { interactionId: 'cp1', commandHash: 'sha256:abc', changes: [] },
};

afterEach(() => {
  vi.restoreAllMocks();
  useRunStore.setState({ sessions: {} });
});

describe('RunCommandActivity', () => {
  it('submits an exact allow-once answer', async () => {
    const submit = vi.spyOn(runsApi, 'submitCommandPermission').mockResolvedValue(undefined);
    useRunStore.setState({
      sessions: {
        'thread-1': {
          ...IDLE_SESSION,
          activeRunId: 'r1',
          status: 'waiting',
          timelineItems: [item],
        },
      },
    });

    render(<RunCommandActivity item={item} />);
    expect(screen.getByText(item.command!.text)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '批准' }));
    expect(submit).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: '继续' }));

    await waitFor(() => {
      expect(submit).toHaveBeenCalledWith('r1', {
        interaction_id: 'cp1',
        call_id: 'c1',
        command_hash: 'sha256:abc',
        decision: 'allow_once',
        feedback: undefined,
      });
    });
  });

  it('renders a compact denied record after history hydration', () => {
    render(<RunCommandActivity item={{ ...item, status: 'refuse', commandApproval: { ...item.commandApproval!, answer: { decision: 'deny' } } }} />);
    expect(screen.getByText('未应用 sed 命令的更改')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '批准' })).toBeNull();
  });
});
