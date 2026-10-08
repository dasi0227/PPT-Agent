import { fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import userEvent from '@testing-library/user-event';
import { useComposerStore } from '../../stores/composerStore';
import { useContextWindowStore } from '../../stores/contextWindowStore';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { ContextWindowPanel } from './ContextWindowPanel';

const EMPTY_TEST_SNAPSHOT = {
  total: 0,
  max: 65536,
  ratio: 0,
  compactable_tokens: 0,
  compact_threshold_tokens: 12000,
  status: 'idle' as const,
  buckets: {
    system_prompt: 0, runtime: 0, chat_history: 0, read_file: 0, other: 0,
  },
  details: {
    system_prompt: [{ name: 'system prompts', tokens: 0 }, { name: 'tool definitions', tokens: 0 }],
    runtime: [{ name: 'runtime context', tokens: 0 }, { name: 'runtime messages', tokens: 0 }],
    chat_history: [{ name: 'user messages', tokens: 0 }, { name: 'assistant messages', tokens: 0 }, { name: 'tools execution', tokens: 0 }],
    read_file: [{ name: 'read_resource', tokens: 0 }, { name: 'read_image', tokens: 0 }],
    other: [{ name: 'other', tokens: 0 }],
  },
};

describe('ContextWindowPanel', () => {
  afterEach(() => vi.unstubAllGlobals());
  beforeEach(() => {
    vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} });
    useProjectStore.setState({ activeProjectId: null });
    useThreadStore.setState({ activeThreadIdByProjectId: {} });
    useContextWindowStore.setState({ sessions: {} });
    useComposerStore.setState({ modelProfileName: null });
  });

  it('closes with Escape without retaining focus on its trigger', () => {
    render(<ContextWindowPanel />);
    const trigger = screen.getByRole('button', { name: /上下文窗口/ });
    trigger.focus();
    fireEvent.click(trigger);

    expect(screen.getByRole('dialog', { name: '上下文窗口' })).toBeInTheDocument();
    fireEvent.keyDown(document, { key: 'Escape' });

    expect(screen.queryByRole('dialog', { name: '上下文窗口' })).not.toBeInTheDocument();
    expect(trigger).not.toHaveFocus();
  });

  it('escapes the clipped panel while keeping inside controls usable and outside dismissal working', async () => {
    const user = userEvent.setup();
    const { container } = render(<div style={{ width: 200, overflow: 'hidden' }}><ContextWindowPanel /></div>);
    await user.click(screen.getByRole('button', { name: /上下文窗口/ }));
    const dialog = screen.getByRole('dialog', { name: '上下文窗口' });
    expect(document.body).toContainElement(dialog);
    expect(container).not.toContainElement(dialog);
    await user.click(screen.getByRole('tab', { name: /读取/ }));
    expect(screen.getByRole('tabpanel', { name: '读取明细' })).toBeInTheDocument();
    await user.click(document.body);
    expect(screen.queryByRole('dialog', { name: '上下文窗口' })).not.toBeInTheDocument();
  });

  it('shows all five buckets in the fixed order, including zero values', () => {
    render(<ContextWindowPanel />);
    fireEvent.click(screen.getByRole('button', { name: /上下文窗口/ }));

    expect(screen.getAllByRole('tab').map((tab) => tab.textContent)).toEqual([
      '系统提示词0.0 k',
      '运行时0.0 k',
      '历史记录0.0 k',
      '读取0.0 k',
      '其它0.0 k',
    ]);
    expect(screen.queryByText('空闲')).not.toBeInTheDocument();
    expect(screen.getByText('系统提示词').parentElement).toHaveClass('items-baseline');
    expect(screen.getByText('系统提示词')).toHaveClass('leading-none');
    expect(screen.getAllByText('0.0 k')[0]).toHaveClass('leading-none');
    expect(screen.getByText('system prompts')).toBeInTheDocument();
    expect(screen.getByText('定义 Agent 行为、模式与任务约束')).toBeInTheDocument();
    expect(screen.getAllByText('0.00 k')).toHaveLength(2);
  });

  it('keeps fixed zero-token details visible when switching buckets', () => {
    render(<ContextWindowPanel />);
    fireEvent.click(screen.getByRole('button', { name: /上下文窗口/ }));
    fireEvent.click(screen.getByRole('tab', { name: /读取/ }));

    expect(screen.getByRole('tabpanel', { name: '读取明细' })).toBeInTheDocument();
    expect(screen.getByText('read resource')).toBeInTheDocument();
    expect(screen.getByText('read image')).toBeInTheDocument();
    expect(screen.queryByText('read project')).not.toBeInTheDocument();
    expect(screen.queryByText('read_resource')).not.toBeInTheDocument();
    expect(screen.getByText('读取或上传并送入模型的图片')).toBeInTheDocument();
    expect(screen.getAllByText('0.00 k')).toHaveLength(2);

  });

  it('includes command execution in history', () => {
    useProjectStore.setState({ activeProjectId: 'p1' });
    useThreadStore.setState({ activeThreadIdByProjectId: { p1: 't1' } });
    useContextWindowStore.setState({
      sessions: {
        t1: {
          loading: false,
          compacting: false,
          snapshot: {
            total: 1000,
            max: 65536,
            ratio: 1000 / 65536,
            compactable_tokens: 12000,
            compact_threshold_tokens: 12000,
            status: 'idle',
            buckets: {
              system_prompt: 0,
              runtime: 0,
              chat_history: 1000,
              read_file: 0,
              other: 0,
            },
            details: {
              system_prompt: [{ name: 'system prompts', tokens: 0 }, { name: 'tool definitions', tokens: 0 }],
              runtime: [{ name: 'runtime context', tokens: 0 }, { name: 'runtime messages', tokens: 0 }],
              chat_history: [{ name: 'user messages', tokens: 0 }, { name: 'assistant messages', tokens: 0 }, { name: 'tools execution', tokens: 1000 }],
              read_file: [{ name: 'read_resource', tokens: 0 }, { name: 'read_image', tokens: 0 }],
              other: [{ name: 'other', tokens: 0 }],
            },
          },
        },
      },
    });

    render(<ContextWindowPanel />);
    fireEvent.click(screen.getByRole('button', { name: /上下文窗口/ }));
    expect(screen.getByRole('button', { name: '压缩' })).toBeEnabled();
    fireEvent.click(screen.getByRole('tab', { name: /历史记录/ }));
    const panel = screen.getByRole('tabpanel', { name: '历史记录明细' });
    expect(within(panel).getByText('tools execution')).toBeInTheDocument();
    expect(within(panel).getByText('1.00 k')).toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: /跑命令/ })).not.toBeInTheDocument();
  });

  it('disables manual compaction until the compactable transcript reaches 12k tokens', () => {
    useProjectStore.setState({ activeProjectId: 'p1' });
    useThreadStore.setState({ activeThreadIdByProjectId: { p1: 't1' } });
    const snapshot = {
      ...EMPTY_TEST_SNAPSHOT,
      compactable_tokens: 11_999,
      compact_threshold_tokens: 12_000,
    };
    useContextWindowStore.setState({
      sessions: { t1: { loading: false, compacting: false, snapshot } },
    });

    render(<ContextWindowPanel />);
    fireEvent.click(screen.getByRole('button', { name: /上下文窗口/ }));

    const compactButton = screen.getByRole('button', { name: '压缩' });
    expect(compactButton).toBeDisabled();
    expect(compactButton).toHaveClass('disabled:opacity-40');
    expect(compactButton).toHaveAttribute('title', '可压缩历史达到 12.0 k 后可用，当前 11999 Token');
  });
});
