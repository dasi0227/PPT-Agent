import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { CommandComposer } from './CommandComposer';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { useRunStore } from '../../stores/runStore';
import { useComposerStore } from '../../stores/composerStore';
import { useDeckStore } from '../../stores/deckStore';
import * as platform from '../../lib/platform';

vi.mock('../../lib/platform', () => ({
  isMac: vi.fn(),
  submitShortcutLabel: vi.fn(() => 'Ctrl + Enter')
}));

describe('CommandComposer', () => {
  beforeEach(() => {
    useProjectStore.setState({
      activeProjectId: 'p1',
      slidesByProjectId: {
        p1: [
          { id: 's1', project_id: 'p1', idx: 0, layout: 'title', title: 'S1', html_path: '', json_path: 'slides/s1/slide.json', current_version: 0, order: 10, outline_dirty: false },
          { id: 's2', project_id: 'p1', idx: 1, layout: 'bullets', title: 'S2', html_path: 'slides/s2/index.html', json_path: 'slides/s2/slide.json', current_version: 1, order: 20, outline_dirty: false },
        ],
      },
    });
    useThreadStore.setState({ 
      activeThreadIdByProjectId: { p1: 't1' },
      ensureActiveThread: async () => 't1'
    });
    useRunStore.setState({
      sessions: {
        t1: { activeRunId: null, status: 'idle' as any, mode: 'normal' as any, scope: 'current' as any, timelineItems: [], pendingInput: null, progress: null, eventSourceClose: null, plan: null }
      }
    });
    useComposerStore.setState({ interactionMode: 'page', subMode: 'normal', userTouchedMode: false });
    useDeckStore.setState({ currentPage: 0 });
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

  it('generates the current page when its real html_path is empty', async () => {
    const mockCreateRun = vi.fn().mockResolvedValue(undefined);
    useRunStore.setState({ createRun: mockCreateRun });
    useDeckStore.setState({ currentPage: 0 });

    render(<CommandComposer />);
    fireEvent.change(screen.getByRole('textbox'), { target: { value: '生成这一页' } });
    fireEvent.click(screen.getByRole('button', { name: '发送' }));

    await waitFor(() => expect(mockCreateRun).toHaveBeenCalledWith('t1', {
      kind: 'generate',
      scope: 'current',
      mode: 'normal',
      page_index: 0,
      instruction: '生成这一页',
    }));
  });

  it('edits the current page when its real html_path exists', async () => {
    const mockCreateRun = vi.fn().mockResolvedValue(undefined);
    useRunStore.setState({ createRun: mockCreateRun });
    useDeckStore.setState({ currentPage: 1 });

    render(<CommandComposer />);
    fireEvent.change(screen.getByRole('textbox'), { target: { value: '放大标题' } });
    fireEvent.click(screen.getByRole('button', { name: '发送' }));

    await waitFor(() => expect(mockCreateRun).toHaveBeenCalledWith('t1', {
      kind: 'edit',
      scope: 'current',
      mode: 'normal',
      page_index: 1,
      instruction: '放大标题',
    }));
  });

  it.each(['/page 2 改标题', '/overview 统一配色', '/repo 保存主题', '/talk 讨论节奏', '/ask 帮我判断'])(
    'passes raw slash command %s without conflicting parsed fields',
    async (instruction) => {
      const mockCreateRun = vi.fn().mockResolvedValue(undefined);
      useRunStore.setState({ createRun: mockCreateRun });

      render(<CommandComposer />);
      fireEvent.change(screen.getByRole('textbox'), { target: { value: instruction } });
      fireEvent.click(screen.getByRole('button', { name: '发送' }));

      await waitFor(() => expect(mockCreateRun).toHaveBeenCalledWith('t1', {
        kind: 'edit',
        instruction,
      }));
    }
  );

  it('offers an explicit whole-deck generate action with overview scope and no page_index', async () => {
    const mockCreateRun = vi.fn().mockResolvedValue(undefined);
    useRunStore.setState({ createRun: mockCreateRun });

    render(<CommandComposer />);
    fireEvent.click(screen.getByRole('button', { name: '整套生成' }));

    await waitFor(() => expect(mockCreateRun).toHaveBeenCalledWith('t1', {
      kind: 'generate',
      scope: 'overview',
      instruction: '基于当前大纲生成整套 HTML PPT',
    }));
    expect(mockCreateRun.mock.calls[0][1]).not.toHaveProperty('page_index');
  });
});
