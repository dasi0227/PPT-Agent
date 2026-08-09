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
  const cancelRun = vi.fn();

  beforeEach(() => {
    createRun.mockReset();
    createRun.mockResolvedValue(true);
    cancelRun.mockReset();
    cancelRun.mockResolvedValue(undefined);
    act(() => {
      useProjectStore.setState({
        activeProjectId: 'p1',
        slidesByProjectId: {
          p1: [
            { id: 'stable-1', project_id: 'p1', position: 0, layout: 'title', title: 'S1', html_path: '', spec_path: '', current_version: 0 },
          ],
        },
      });
      useThreadStore.setState({ activeThreadIdByProjectId: { p1: 't1' }, ensureActiveThread: async () => 't1' });
      useRunStore.setState({
        cancelRun,
        createRun,
        sessions: {
          t1: {
            activeRunId: null, status: 'idle',
            scope: { artifact: 'ppt', level: 'slide' },
            intent: 'execute',
            timelineItems: [], pendingQuestion: null, progress: null, eventSourceClose: null, plan: null,
          },
        },
      });
      useComposerStore.setState({
        artifact: 'ppt', level: 'slide', intent: 'execute',
        modelProfileName: null, userTouchedTarget: false,
      });
      useDeckStore.setState({ currentPage: 0 });
    });
  });

  it('uses default execution and sends the current slide with a stable slide id', async () => {
    render(<CommandComposer />);
    await waitFor(() => expect(screen.getByRole('button', { name: '模型' })).toHaveTextContent('Kimi K3'));
    expect(screen.getByRole('button', { name: '讨论' })).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByRole('button', { name: '盘问' })).toHaveAttribute('aria-pressed', 'false');
	    expect(screen.getByRole('button', { name: '计划' })).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByRole('button', { name: '目标：单页幻灯片' })).toBeInTheDocument();
    expect(screen.getByRole('textbox')).toHaveAttribute('placeholder', '输入你的想法与目标');

    fireEvent.change(screen.getByRole('textbox'), { target: { value: '调整当前页' } });
    fireEvent.click(screen.getByRole('button', { name: '发送' }));

    await waitFor(() => expect(createRun).toHaveBeenCalledWith('t1', expect.objectContaining({
      client_request_id: expect.stringMatching(/^req_/),
      model: 'Kimi K3',
      scope: { artifact: 'ppt', level: 'slide', slide_id: 'stable-1' },
      intent: 'execute',
      instruction: '调整当前页',
    }), 'p1'));
  });

	  it('maps talk, ask and plan buttons mutually exclusively and restores default execution', async () => {
    render(<CommandComposer />);
    const talk = screen.getByRole('button', { name: '讨论' });
    const ask = screen.getByRole('button', { name: '盘问' });
	    const plan = screen.getByRole('button', { name: '计划' });

    await act(async () => fireEvent.click(talk));
    expect(useComposerStore.getState()).toMatchObject({ intent: 'talk' });
    expect(talk).toHaveAttribute('aria-pressed', 'true');
    expect(ask).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByRole('textbox')).toHaveAttribute('placeholder', '输入你的想法与目标');

    await act(async () => fireEvent.click(ask));
    expect(useComposerStore.getState()).toMatchObject({ intent: 'ask' });
    expect(talk).toHaveAttribute('aria-pressed', 'false');
    expect(ask).toHaveAttribute('aria-pressed', 'true');

	    await act(async () => fireEvent.click(plan));
	    expect(useComposerStore.getState()).toMatchObject({ intent: 'plan' });
	    expect(talk).toHaveAttribute('aria-pressed', 'false');
	    expect(ask).toHaveAttribute('aria-pressed', 'false');
	    expect(plan).toHaveAttribute('aria-pressed', 'true');

	    await act(async () => fireEvent.click(plan));
    expect(useComposerStore.getState()).toMatchObject({ intent: 'execute' });
	    expect(plan).toHaveAttribute('aria-pressed', 'false');
  });

	  it('keeps /talk, /ask and /plan shortcuts mapped to their interaction protocols', async () => {
    render(<CommandComposer />);
    await waitFor(() => expect(screen.getByRole('button', { name: '模型' })).toHaveTextContent('Kimi K3'));
    const textarea = screen.getByRole('textbox');

    fireEvent.change(textarea, { target: { value: '/talk 给我建议' } });
    fireEvent.click(screen.getByRole('button', { name: '发送' }));
    await waitFor(() => expect(createRun).toHaveBeenLastCalledWith('t1', expect.objectContaining({
      intent: 'talk',
      instruction: '给我建议',
    }), 'p1'));

    fireEvent.change(textarea, { target: { value: '/ask 先分析方案' } });
    fireEvent.keyDown(textarea, { key: 'Enter', ctrlKey: true });
    await waitFor(() => expect(createRun).toHaveBeenLastCalledWith('t1', expect.objectContaining({
      intent: 'ask',
      instruction: '先分析方案',
    }), 'p1'));

	    fireEvent.change(textarea, { target: { value: '/plan 拆解执行步骤' } });
	    fireEvent.click(screen.getByRole('button', { name: '发送' }));
	    await waitFor(() => expect(createRun).toHaveBeenLastCalledWith('t1', expect.objectContaining({
	      intent: 'plan',
	      instruction: '拆解执行步骤',
	    }), 'p1'));
  });

  it('uses the plan button as progress popover when a plan exists', async () => {
    useRunStore.setState({
      createRun,
      sessions: {
        t1: {
          activeRunId: 'r-plan', status: 'running',
          scope: { artifact: 'ppt', level: 'slide' },
          intent: 'execute',
          timelineItems: [], pendingQuestion: null, progress: null, eventSourceClose: null,
          plan: {
            id: 'p1',
            title: '执行计划',
            revision: 1,
            steps: [
              { id: 's1', title: '完成结构梳理', status: 'completed' },
              { id: 's2', title: '检查视觉结果', status: 'pending' },
            ],
          },
        },
      },
    });
    render(<CommandComposer />);
    const plan = screen.getByRole('button', { name: '计划 1 / 2' });

    await act(async () => {
      fireEvent.pointerDown(plan, { button: 0, ctrlKey: false });
      fireEvent.click(plan);
    });
    expect(useComposerStore.getState()).toMatchObject({ intent: 'execute' });
    expect(screen.getByText('完成结构梳理')).toBeInTheDocument();
    expect(screen.getByText('检查视觉结果')).toBeInTheDocument();
  });

  it('uses the pending question guidance as textarea placeholder', async () => {
    useRunStore.setState({
      cancelRun,
      createRun,
      sessions: {
        t1: {
          activeRunId: 'r1', status: 'waiting',
          scope: { artifact: 'ppt', level: 'slide' },
          intent: 'ask',
          timelineItems: [], pendingQuestion: { id: 'q1', prompt: '选择' },
          progress: null, eventSourceClose: null, plan: null,
        },
      },
    });
    render(<CommandComposer />);
    await waitFor(() => expect(screen.getByRole('button', { name: '模型' })).toHaveTextContent('Kimi K3'));
    expect(screen.getByRole('textbox')).toHaveAttribute('placeholder', '请先回答上方问题');
    expect(screen.queryByText('请先回答上方问题')).toBeNull();
  });

  it('shows a stop button for an active run until the user types steering text', async () => {
    useRunStore.setState({
      cancelRun,
      createRun,
      sessions: {
        t1: {
          activeRunId: 'r1', status: 'running',
          scope: { artifact: 'ppt', level: 'slide' },
          intent: 'execute',
          timelineItems: [], pendingQuestion: null, progress: null, eventSourceClose: null, plan: null,
        },
      },
    });
    render(<CommandComposer />);
    await waitFor(() => expect(screen.getByRole('button', { name: '模型' })).toHaveTextContent('Kimi K3'));
    fireEvent.click(screen.getByRole('button', { name: '终止运行' }));
    await waitFor(() => expect(cancelRun).toHaveBeenCalledWith('t1', 'r1'));

    fireEvent.change(screen.getByRole('textbox'), { target: { value: '补充要求' } });
    expect(screen.getByRole('button', { name: '发送' })).toBeInTheDocument();
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
    await waitFor(() => expect(screen.getByRole('button', { name: '模型' })).toHaveTextContent('Kimi K3'));
    const textarea = screen.getByRole('textbox');
    fireEvent.change(textarea, { target: { value: '保留这段内容' } });
    fireEvent.click(screen.getByRole('button', { name: '发送' }));

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('运行创建失败'));
    expect(textarea).toHaveValue('保留这段内容');
  });

  it('disables text-only profiles for presentation execute but allows them for talk', async () => {
    render(<CommandComposer />);
    const selector = await screen.findByRole('button', { name: '模型' });
    expect(selector).toHaveTextContent('Kimi K3');

    const openMenu = () => {
      fireEvent.pointerDown(selector, { button: 0, ctrlKey: false });
      fireEvent.click(selector);
    };

    openMenu();
    expect(screen.getByRole('menuitem', { name: '模型：DeepSeek V4 Pro' })).toHaveAttribute('data-disabled');
    fireEvent.keyDown(selector, { key: 'Escape' });

    fireEvent.click(screen.getByRole('button', { name: '讨论' }));
    openMenu();
    const textOption = screen.getByRole('menuitem', { name: '模型：DeepSeek V4 Pro' });
    expect(textOption).not.toHaveAttribute('data-disabled');
    fireEvent.click(textOption);
    fireEvent.change(screen.getByRole('textbox'), { target: { value: '只读分析当前页' } });
    fireEvent.click(screen.getByRole('button', { name: '发送' }));

    await waitFor(() => expect(createRun).toHaveBeenCalledWith('t1', expect.objectContaining({
      model: 'DeepSeek V4 Pro',
      intent: 'talk',
    }), 'p1'));
  });

  it('locks the target to 整份设计稿 and surfaces guidance when the project has no pages', async () => {
    useProjectStore.setState({ activeProjectId: 'empty', slidesByProjectId: { empty: [] } });
    useThreadStore.setState({ activeThreadIdByProjectId: { empty: 't-empty' }, ensureActiveThread: async () => 't-empty' });
    render(<CommandComposer />);

    await waitFor(() => expect(screen.getByRole('button', { name: '目标：整份设计稿' })).toBeInTheDocument());
    const targetTrigger = screen.getByRole('button', { name: '目标：整份设计稿' });

    // Locked: clicking must not open the scope/object menu.
    fireEvent.pointerDown(targetTrigger, { button: 0, ctrlKey: false });
    fireEvent.click(targetTrigger);
    expect(screen.queryByRole('group', { name: '范围' })).not.toBeInTheDocument();
    expect(screen.queryByRole('menuitem', { name: '范围：单页' })).not.toBeInTheDocument();

    // Guidance is wired for hover and keyboard focus via aria-describedby.
    const hint = screen.getByRole('tooltip');
    expect(hint).toHaveTextContent('当前为空项目，请先确定整体的设计稿');
    expect(targetTrigger).toHaveAttribute('aria-describedby', hint.id);
  });
});
