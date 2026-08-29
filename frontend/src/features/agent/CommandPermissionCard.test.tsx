import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { runsApi } from '../../api/runs';
import { IDLE_SESSION, useRunStore } from '../../stores/runStore';
import { CommandPermissionCard } from './CommandPermissionCard';
import type { CommandPermissionItem } from './eventReducer';

vi.mock('./useActiveSession', () => ({
  useActiveThreadId: () => 'thread-1',
}));

const item: CommandPermissionItem = {
  id: 'r1:command-permission:cp1',
  type: 'command_permission',
  runId: 'r1',
  interactionId: 'cp1',
  callId: 'c1',
  command: "sed -i '' 's/old/new/g' notes.txt",
  commandHash: 'sha256:abc',
  reasonCode: 'PROJECT_FILE_EDIT',
  reason: '该命令将修改项目文件，需要经过批准后才能执行。',
  timestamp: 1,
};

afterEach(() => {
  vi.restoreAllMocks();
  useRunStore.setState({ sessions: {} });
});

describe('CommandPermissionCard', () => {
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

    render(<CommandPermissionCard item={item} />);
    expect(screen.getByText(item.command)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '允许一次' }));

    await waitFor(() => {
      expect(submit).toHaveBeenCalledWith('r1', {
        interaction_id: 'cp1',
        call_id: 'c1',
        command_hash: 'sha256:abc',
        decision: 'allow_once',
      });
    });
  });

  it('renders a compact denied record after history hydration', () => {
    render(<CommandPermissionCard item={{ ...item, answer: 'deny' }} />);
    expect(screen.getByText('已拒绝命令执行，Agent 将选择其他方式')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '允许一次' })).toBeNull();
  });
});
