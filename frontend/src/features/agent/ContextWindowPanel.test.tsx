import { fireEvent, render, screen, within } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';
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
    system_prompt: 0, runtime: 0, chat_history: 0, read_file: 0, run_command: 0, other: 0,
  },
  details: {
    system_prompt: [{ name: 'system prompts', tokens: 0 }, { name: 'tool definitions', tokens: 0 }],
    runtime: [{ name: 'runtime state', tokens: 0 }, { name: 'runtime resources', tokens: 0 }, { name: 'runtime messages', tokens: 0 }],
    chat_history: [{ name: 'user messages', tokens: 0 }, { name: 'assistant messages', tokens: 0 }, { name: 'other tools', tokens: 0 }, { name: 'context summary', tokens: 0 }],
    read_file: [{ name: 'read_ppt', tokens: 0 }, { name: 'read_image', tokens: 0 }, { name: 'read_project', tokens: 0 }],
    run_command: [{ name: 'run_command', tokens: 0 }],
    other: [{ name: 'other', tokens: 0 }],
  },
};

describe('ContextWindowPanel', () => {
  beforeEach(() => {
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

  it('shows all six buckets in the fixed order, including zero values', () => {
    render(<ContextWindowPanel />);
    fireEvent.click(screen.getByRole('button', { name: /上下文窗口/ }));

    expect(screen.getAllByRole('tab').map((tab) => tab.textContent)).toEqual([
      '系统提示词0.0 k',
      '运行时0.0 k',
      '对话历史0.0 k',
      '读文件0.0 k',
      '跑命令0.0 k',
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
    fireEvent.click(screen.getByRole('tab', { name: /读文件/ }));

    expect(screen.getByRole('tabpanel', { name: '读文件明细' })).toBeInTheDocument();
    expect(screen.getByText('read ppt')).toBeInTheDocument();
    expect(screen.getByText('read image')).toBeInTheDocument();
    expect(screen.getByText('read project')).toBeInTheDocument();
    expect(screen.queryByText('read_ppt')).not.toBeInTheDocument();
    expect(screen.getByText('读取或上传并送入模型的图片')).toBeInTheDocument();
    expect(screen.getAllByText('0.00 k')).toHaveLength(3);

    fireEvent.click(screen.getByRole('tab', { name: /跑命令/ }));
    expect(screen.getByText('run command')).toBeInTheDocument();
    expect(screen.queryByText('run_command')).not.toBeInTheDocument();
  });

  it('shows ranked command details and the aggregated remainder', () => {
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
              chat_history: 0,
              read_file: 0,
              run_command: 1000,
              other: 0,
            },
            details: {
              system_prompt: [{ name: 'system prompts', tokens: 0 }, { name: 'tool definitions', tokens: 0 }],
              runtime: [{ name: 'runtime state', tokens: 0 }, { name: 'runtime resources', tokens: 0 }, { name: 'runtime messages', tokens: 0 }],
              chat_history: [{ name: 'user messages', tokens: 0 }, { name: 'assistant messages', tokens: 0 }, { name: 'other tools', tokens: 0 }, { name: 'context summary', tokens: 0 }],
              read_file: [{ name: 'read_ppt', tokens: 0 }, { name: 'read_image', tokens: 0 }, { name: 'read_project', tokens: 0 }],
              run_command: [{ name: 'ls', tokens: 400 }, { name: 'rg', tokens: 300 }, { name: 'git', tokens: 200 }, { name: 'other command', tokens: 100 }],
              other: [{ name: 'other', tokens: 0 }],
            },
          },
        },
      },
    });

    render(<ContextWindowPanel />);
    fireEvent.click(screen.getByRole('button', { name: /上下文窗口/ }));
    expect(screen.getByRole('button', { name: '压缩' })).toBeEnabled();
    fireEvent.click(screen.getByRole('tab', { name: /跑命令/ }));

    const panel = screen.getByRole('tabpanel', { name: '跑命令明细' });
    expect(within(panel).getByText('ls')).toBeInTheDocument();
    expect(within(panel).getByText('rg')).toBeInTheDocument();
    expect(within(panel).getByText('git')).toBeInTheDocument();
    expect(within(panel).getByText('other command')).toBeInTheDocument();
    expect(within(panel).getByText('ls 命令的调用与返回结果')).toBeInTheDocument();
    expect(within(panel).getByText('其余命令的调用与返回结果')).toBeInTheDocument();
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
