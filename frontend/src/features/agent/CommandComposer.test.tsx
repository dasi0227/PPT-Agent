import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useComposerStore } from '../../stores/composerStore';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { useRunStore } from '../../stores/runStore';
import { useThreadStore } from '../../stores/threadStore';
import { CommandComposer } from './CommandComposer';

vi.mock('../../lib/platform', () => ({ isMac: () => false, submitShortcutLabel: () => 'Ctrl + Enter' }));

describe('CommandComposer target protocol', () => {
  beforeEach(() => {
    useProjectStore.setState({
      activeProjectId: 'p1',
      slidesByProjectId: {
        p1: [
          { id: 'stable-1', project_id: 'p1', position: 0, layout: 'title', title: 'S1', html_path: '', json_path: '', current_version: 0 },
        ],
      },
    });
    useThreadStore.setState({ activeThreadIdByProjectId: { p1: 't1' }, ensureActiveThread: async () => 't1' });
    useRunStore.setState({ sessions: {
      t1: {
        activeRunId: null, status: 'idle',
        target: { artifact: 'presentation', level: 'slide' },
        interaction: { intent: 'apply', clarification: 'when_blocked' },
        timelineItems: [], pendingInput: null, progress: null, eventSourceClose: null, plan: null,
      },
    } });
    useComposerStore.setState({
      artifact: 'presentation', level: 'slide', intent: 'apply',
      clarification: 'when_blocked', userTouchedTarget: true,
    });
    useDeckStore.setState({ currentPage: 0 });
  });

  it('sends a stable slide id with the new payload', async () => {
    const createRun = vi.fn().mockResolvedValue(undefined);
    useRunStore.setState({ createRun });
    render(<CommandComposer />);
    fireEvent.change(screen.getByRole('textbox'), { target: { value: '调整当前页' } });
    fireEvent.click(screen.getByRole('button', { name: '发送' }));
    await waitFor(() => expect(createRun).toHaveBeenCalledWith('t1', {
      target: { artifact: 'presentation', level: 'slide', slide_id: 'stable-1' },
      interaction: { intent: 'apply', clarification: 'when_blocked' },
      instruction: '调整当前页',
    }));
  });

  it('switches all four artifact and level combinations', () => {
    render(<CommandComposer />);
    expect(screen.getByRole('button', { name: '演示' })).toHaveAttribute('aria-pressed', 'true');
    fireEvent.click(screen.getByRole('button', { name: '蓝图' }));
    fireEvent.click(screen.getByRole('button', { name: '整份' }));
    expect(useComposerStore.getState()).toMatchObject({ artifact: 'blueprint', level: 'deck' });
  });

  it('maps discussion to consult', () => {
    render(<CommandComposer />);
    fireEvent.click(screen.getByRole('button', { name: '讨论' }));
    expect(useComposerStore.getState().intent).toBe('consult');
  });

  it('maps execution confirmation to before_apply', () => {
    render(<CommandComposer />);
    fireEvent.click(screen.getByRole('button', { name: '执行前确认' }));
    expect(useComposerStore.getState().clarification).toBe('before_apply');
  });

  it('materializes the whole deck without a slide id', async () => {
    const createRun = vi.fn().mockResolvedValue(undefined);
    useRunStore.setState({ createRun });
    render(<CommandComposer />);
    fireEvent.click(screen.getByRole('button', { name: '物化整份' }));
    await waitFor(() => expect(createRun).toHaveBeenCalledWith('t1', expect.objectContaining({
      target: { artifact: 'presentation', level: 'deck' },
    })));
  });
});
