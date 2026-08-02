import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useComposerStore } from '../../stores/composerStore';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { useRunStore } from '../../stores/runStore';
import { useThreadStore } from '../../stores/threadStore';
import { CommandComposer } from './CommandComposer';

vi.mock('../../lib/platform', () => ({ isMac: () => false, submitShortcutLabel: () => 'Ctrl + Enter' }));

describe('CommandComposer', () => {
  const createRun = vi.fn();

  beforeEach(() => {
    createRun.mockReset();
    createRun.mockResolvedValue(true);
    act(() => {
      useProjectStore.setState({
        activeProjectId: 'p1',
        slidesByProjectId: {
          p1: [
            { id: 'stable-1', project_id: 'p1', position: 0, layout: 'title', title: 'S1', html_path: '', json_path: '', current_version: 0 },
          ],
        },
      });
      useThreadStore.setState({ activeThreadIdByProjectId: { p1: 't1' }, ensureActiveThread: async () => 't1' });
      useRunStore.setState({
        createRun,
        sessions: {
          t1: {
            activeRunId: null, status: 'idle',
            target: { artifact: 'presentation', level: 'slide' },
            interaction: { intent: 'execute' },
            timelineItems: [], pendingQuestion: null, progress: null, eventSourceClose: null, plan: null,
          },
        },
      });
      useComposerStore.setState({
        artifact: 'presentation', level: 'slide', intent: 'execute', userTouchedTarget: false,
      });
      useDeckStore.setState({ currentPage: 0 });
    });
  });

  it('uses default execution and sends the current slide with a stable slide id', async () => {
    render(<CommandComposer />);
    expect(screen.getByRole('button', { name: '讨论' })).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByRole('button', { name: '询问' })).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByRole('button', { name: '目标：单页幻灯片' })).toBeInTheDocument();
    expect(screen.getByRole('textbox')).toHaveAttribute('placeholder', '输入你的想法与目标');

    fireEvent.change(screen.getByRole('textbox'), { target: { value: '调整当前页' } });
    fireEvent.click(screen.getByRole('button', { name: '发送' }));

    await waitFor(() => expect(createRun).toHaveBeenCalledWith('t1', {
      target: { artifact: 'presentation', level: 'slide', slide_id: 'stable-1' },
      interaction: { intent: 'execute' },
      instruction: '调整当前页',
    }, 'p1'));
  });

  it('maps talk and ask buttons mutually exclusively and restores default execution', async () => {
    render(<CommandComposer />);
    const talk = screen.getByRole('button', { name: '讨论' });
    const ask = screen.getByRole('button', { name: '询问' });

    await act(async () => fireEvent.click(talk));
    expect(useComposerStore.getState()).toMatchObject({ intent: 'talk' });
    expect(talk).toHaveAttribute('aria-pressed', 'true');
    expect(ask).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByRole('textbox')).toHaveAttribute('placeholder', '输入你的想法与目标');

    await act(async () => fireEvent.click(ask));
    expect(useComposerStore.getState()).toMatchObject({ intent: 'ask' });
    expect(talk).toHaveAttribute('aria-pressed', 'false');
    expect(ask).toHaveAttribute('aria-pressed', 'true');

    await act(async () => fireEvent.click(ask));
    expect(useComposerStore.getState()).toMatchObject({ intent: 'execute' });
    expect(ask).toHaveAttribute('aria-pressed', 'false');
  });

  it('keeps /talk and /ask shortcuts mapped to their existing interaction protocols', async () => {
    render(<CommandComposer />);
    const textarea = screen.getByRole('textbox');

    fireEvent.change(textarea, { target: { value: '/talk 给我建议' } });
    fireEvent.click(screen.getByRole('button', { name: '发送' }));
    await waitFor(() => expect(createRun).toHaveBeenLastCalledWith('t1', expect.objectContaining({
      interaction: { intent: 'talk' },
      instruction: '给我建议',
    }), 'p1'));

    fireEvent.change(textarea, { target: { value: '/ask 先分析方案' } });
    fireEvent.keyDown(textarea, { key: 'Enter', ctrlKey: true });
    await waitFor(() => expect(createRun).toHaveBeenLastCalledWith('t1', expect.objectContaining({
      interaction: { intent: 'ask' },
      instruction: '先分析方案',
    }), 'p1'));
  });

  it('does not render the removed materialization control or duplicate status line', async () => {
    const { container } = render(<CommandComposer />);
    await act(async () => {});
    expect(screen.queryByRole('button', { name: '物化整份' })).not.toBeInTheDocument();
    expect(screen.queryByText(/本次作用于/)).not.toBeInTheDocument();
    expect(container.querySelector('.ring-accent')).not.toBeInTheDocument();
    expect(screen.getByRole('textbox')).toHaveClass('focus-visible:ring-0', 'focus-visible:ring-offset-0');
  });

  it('keeps typed content when run creation fails', async () => {
    createRun.mockResolvedValue(false);
    render(<CommandComposer />);
    const textarea = screen.getByRole('textbox');
    fireEvent.change(textarea, { target: { value: '保留这段内容' } });
    fireEvent.click(screen.getByRole('button', { name: '发送' }));

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('运行创建失败'));
    expect(textarea).toHaveValue('保留这段内容');
  });

  it('keeps the single-page choice available even when the project has no pages', async () => {
    useProjectStore.setState({ activeProjectId: 'empty', slidesByProjectId: { empty: [] } });
    useThreadStore.setState({ activeThreadIdByProjectId: { empty: 't-empty' }, ensureActiveThread: async () => 't-empty' });
    render(<CommandComposer />);

    await waitFor(() => expect(screen.getByRole('button', { name: '目标：整份蓝图' })).toBeInTheDocument());
    const targetTrigger = screen.getByRole('button', { name: '目标：整份蓝图' });
    fireEvent.pointerDown(targetTrigger, { button: 0, ctrlKey: false });
    fireEvent.click(targetTrigger);
    expect(screen.getByRole('menuitem', { name: '范围：单页' })).not.toHaveAttribute('data-disabled');
  });
});
