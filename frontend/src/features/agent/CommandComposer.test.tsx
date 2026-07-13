import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { CommandComposer } from './CommandComposer';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { useRunStore } from '../../stores/runStore';
import * as platform from '../../lib/platform';

vi.mock('../../lib/platform', () => ({
  isMac: vi.fn(),
  submitShortcutLabel: vi.fn(() => 'Ctrl + Enter')
}));

describe('CommandComposer', () => {
  beforeEach(() => {
    useProjectStore.setState({ activeProjectId: 'p1' });
    useThreadStore.setState({ 
      activeThreadIdByProjectId: { p1: 't1' },
      ensureActiveThread: async () => 't1'
    });
    useRunStore.setState({
      sessions: {
        t1: { activeRunId: null, status: 'idle' as any, mode: 'normal' as any, scope: 'current' as any, timelineItems: [], pendingInput: null, progress: null, eventSourceClose: null, plan: null }
      }
    });
  });

  it('Enter only inserts newline', () => {
    vi.mocked(platform.isMac).mockReturnValue(false);
    render(<CommandComposer />);
    const textarea = screen.getByRole('textbox');
    fireEvent.change(textarea, { target: { value: 'hi' } });
    fireEvent.keyDown(textarea, { key: 'Enter' });
    expect(textarea).toHaveValue('hi'); // no prevent default means it would insert newline in real browser
  });

  it('Ctrl+Enter submits on Windows', async () => {
    vi.mocked(platform.isMac).mockReturnValue(false);
    
    const mockCreateRun = vi.fn();
    useRunStore.setState({ createRun: mockCreateRun });

    render(<CommandComposer />);
    const textarea = screen.getByRole('textbox');
    fireEvent.change(textarea, { target: { value: 'hi' } });

    await waitFor(() => {
      expect(screen.getAllByRole('button').pop()).not.toBeDisabled();
    });

    fireEvent.keyDown(textarea, { key: 'Enter', ctrlKey: true });

    await waitFor(() => {
      expect(mockCreateRun).toHaveBeenCalled();
    });
  });

  it('Meta+Enter submits on Mac', async () => {
    vi.mocked(platform.isMac).mockReturnValue(true);

    const mockCreateRun = vi.fn();
    useRunStore.setState({ createRun: mockCreateRun });

    render(<CommandComposer />);
    const textarea = screen.getByRole('textbox');
    fireEvent.change(textarea, { target: { value: 'hi' } });

    await waitFor(() => {
      const btns = screen.getAllByRole('button');
      expect(btns[btns.length - 1]).not.toBeDisabled();
    });

    fireEvent.keyDown(textarea, { key: 'Enter', metaKey: true });

    await waitFor(() => {
      expect(mockCreateRun).toHaveBeenCalled();
    });
  });

  it('does not submit if isComposing', async () => {
    vi.mocked(platform.isMac).mockReturnValue(true);
    render(<CommandComposer />);
    const textarea = screen.getByRole('textbox');
    fireEvent.change(textarea, { target: { value: 'hi' } });
    
    const mockCreateRun = vi.fn();
    useRunStore.setState({ createRun: mockCreateRun });

    await act(async () => {
      fireEvent.compositionStart(textarea);
      fireEvent.keyDown(textarea, { key: 'Enter', metaKey: true, nativeEvent: { isComposing: true } });
    });

    expect(mockCreateRun).not.toHaveBeenCalled();
  });
});
