import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { ToolActivityItem } from './eventReducer';
import { ToolActivityRow, ToolGroupRow } from './ActivityRows';

function commandItem(overrides: Partial<ToolActivityItem> = {}): ToolActivityItem {
  return {
    id: 'r1:tool:c1',
    type: 'tool',
    runId: 'r1',
    callId: 'c1',
    tool: 'run_command',
    label: '正在执行命令',
    status: 'running',
    timestamp: 1_000,
    command: { text: 'git status --short' },
    ...overrides,
  };
}

afterEach(() => {
  vi.useRealTimers();
});

describe('run command activity', () => {
  it('delays only the running command row for 300 ms', () => {
    vi.useFakeTimers();
    vi.setSystemTime(1_000);
    render(<ToolActivityRow item={commandItem()} />);

    expect(screen.queryByText('正在执行命令')).toBeNull();
    act(() => vi.advanceTimersByTime(299));
    expect(screen.queryByText('正在执行命令')).toBeNull();
    act(() => vi.advanceTimersByTime(1));
    expect(screen.getByText('正在执行命令')).toBeInTheDocument();
  });

  it('renders completed command details in one collapsed box', () => {
    render(<ToolActivityRow item={commandItem({
      label: '已确认当前仓库状态',
      status: 'completed',
      command: {
        text: 'git status --short',
        status: 'completed',
        stdout_preview: 'M notes.txt',
      },
    })} />);

    expect(screen.queryByText('M notes.txt')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: /已确认当前仓库状态/ }));
    expect(screen.getByText('git status --short')).toBeInTheDocument();
    expect(screen.getByText('M notes.txt')).toBeInTheDocument();
    expect(screen.queryByText('命令')).toBeNull();
    expect(screen.queryByText('结果')).toBeNull();
  });

  it('shows a stable relative file name for linked PPT targets', () => {
    render(<ToolActivityRow item={commandItem({
      tool: 'mutate_ppt',
      label: '已更新演示内容',
      detail: '/Users/test/project/manifest.json',
      status: 'completed',
      command: undefined,
      target: {
        type: 'deck',
        part: 'manifest',
        local_path: '/Users/test/project/manifest.json',
        open_url: 'vscode://file/Users/test/project/manifest.json',
      },
    })} />);

    fireEvent.click(screen.getByRole('button', { name: /已更新演示内容/ }));

    expect(screen.getByRole('link', { name: /manifest.json/ })).toHaveAttribute(
      'href',
      'vscode://file/Users/test/project/manifest.json',
    );
    expect(screen.queryByText('/Users/test/project/manifest.json')).toBeNull();
  });

  it('uses the confirmed command-count title for grouped rows', () => {
    const items = ['1', '2', '3'].map((id) => commandItem({
      id: `r1:tool:${id}`,
      callId: id,
      status: 'completed',
      label: '已执行命令',
      command: { text: 'pwd', status: 'completed' },
    }));
    render(<ToolGroupRow items={items} />);
    expect(screen.getByText('已执行 3 条命令')).toBeInTheDocument();
  });
});
