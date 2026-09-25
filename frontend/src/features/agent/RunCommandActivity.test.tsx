import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import type { Slide } from '../../api/types';
import type { ToolActivityItem } from './eventReducer';
import { presentActivityText, ToolActivityRow, ToolGroupRow } from './ActivityRows';

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
  it('resolves generic and stale slide labels from the current outline order', () => {
    const slides = [{ id: 'sli_first' }, { id: 'sli_random4' }] as Slide[];
    const target = { type: 'slide', slide_id: 'sli_random4', part: 'spec', display_name: '页面' } as const;

    expect(presentActivityText('已读取页面设计稿', target, slides)).toBe('已读取第 2 页设计稿');
    expect(presentActivityText('已读取第 4 页设计稿', { ...target, display_name: '第 4 页' }, slides))
      .toBe('已读取第 2 页设计稿');
  });

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

  it('renders tool results as normal-weight black text', () => {
    render(<ToolActivityRow item={commandItem({ tool: 'read_ppt', command: undefined, label: '已读取演示内容', status: 'completed' })} />);

    expect(screen.getByText('已读取演示内容')).toHaveClass(
      'font-normal',
      'text-text-900',
    );
  });

  it('renders completed command details in one collapsed box', () => {
    render(<ToolActivityRow item={commandItem({
      label: '已执行 1 条命令',
      status: 'completed',
      command: {
        text: 'git status --short',
        status: 'completed',
        stdout_preview: 'M notes.txt',
      },
    })} />);

    expect(screen.queryByText('M notes.txt')).toBeNull();
    expect(screen.queryByText('已执行 1 条命令')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '已执行 git 命令' }));
    expect(screen.getByText('git status --short')).toBeInTheDocument();
    expect(screen.getByText('M notes.txt')).toBeInTheDocument();
    expect(screen.queryByText('命令')).toBeNull();
    expect(screen.queryByText('结果')).toBeNull();
  });

  it('does not expose a JSON source link from a stored PPT target', () => {
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

    fireEvent.click(screen.getByRole('button', { name: /已编辑演示内容/ }));

    expect(screen.queryByRole('link')).not.toBeInTheDocument();
    expect(screen.queryByText('/Users/test/project/manifest.json')).toBeNull();
  });

  it('shows the count for identical grouped commands and names for their individual rows', () => {
    const items = ['1', '2', '3'].map((id) => commandItem({
      id: `r1:tool:${id}`,
      callId: id,
      status: 'completed',
      label: '已执行命令',
      command: { text: 'pwd', status: 'completed' },
    }));
    render(<ToolGroupRow items={items} />);
    fireEvent.click(screen.getByRole('button', { name: '已执行 3 条命令' }));
    expect(screen.getAllByRole('button', { name: '已执行 pwd 命令' })).toHaveLength(3);
  });

  it('shows each command name when a mixed command group is expanded', () => {
    const items = ['1', '2', '3'].map((id, index) => commandItem({
      id: `r1:tool:${id}`,
      callId: id,
      status: 'completed',
      label: '已执行命令',
      command: { text: index === 0 ? 'pwd' : 'ls', status: 'completed' },
    }));
    render(<ToolGroupRow items={items} />);
    expect(screen.getByText('已执行 3 条命令')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '已执行 3 条命令' }));
    expect(screen.getByRole('button', { name: '已执行 pwd 命令' })).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: '已执行 ls 命令' })).toHaveLength(2);
  });

  it('preserves the complete read label for deck targets', () => {
    const { container } = render(<ToolActivityRow item={commandItem({
      tool: 'read_ppt',
      label: '已读取演示内容',
      status: 'completed',
      command: undefined,
      target: { type: 'deck', part: 'manifest' },
    })} />);
    expect(container.textContent).toContain('已读取演示内容');
  });

  it('uses a monitor icon for completed slide renders', () => {
    const { container } = render(<ToolActivityRow item={commandItem({
      tool: 'render_slide',
      label: '第 3 页渲染通过',
      status: 'completed',
      command: undefined,
      target: { type: 'slide', slide_id: 'sli_three', part: 'html' },
    })} />);

    expect(container.querySelector('.lucide-monitor')).toBeInTheDocument();
    expect(container.querySelector('.lucide-circle-check-big')).not.toBeInTheDocument();
  });

  it('shows only the screenshot when a slide render passes', () => {
    render(<ToolActivityRow item={commandItem({
      tool: 'render_slide',
      label: '第 3 页渲染通过',
      detail: 'sli_three.html',
      status: 'completed',
      command: undefined,
      target: {
        type: 'slide', slide_id: 'sli_three', part: 'html',
        open_url: 'vscode://file/project/sli_three.html',
      },
      preview: {
        slide_id: 'sli_three',
        image_url: '/api/v1/runs/run-1/screenshots/shot-1',
        warnings: [],
      },
    })} />);

    expect(screen.queryByText('第 3 页渲染通过')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /^已渲染/ }));

    expect(screen.getByRole('img', { name: /渲染预览/ })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /index.html/ })).not.toBeInTheDocument();
    expect(screen.queryByText(/slides\/sli_three\/index.html/)).not.toBeInTheDocument();
  });

  it('expands loaded resources as names with ExternalLink actions only', () => {
    render(<ToolActivityRow item={commandItem({
      tool: 'load_component',
      label: '已加载 2 个组件',
      status: 'completed',
      command: undefined,
      resources: [
        { kind: 'component', id: 'feature-card', name: 'Feature Card', open_url: 'vscode://file/components/feature-card/index.html' },
        { kind: 'component', id: 'quote-block', name: 'Quote Block', open_url: 'vscode://file/components/quote-block/index.html' },
      ],
    })} />);

    fireEvent.click(screen.getByRole('button', { name: /已加载 2 个组件/ }));
    expect(screen.getByRole('link', { name: 'Feature Card' })).toHaveAttribute(
      'href',
      'vscode://file/components/feature-card/index.html',
    );
    expect(screen.getByRole('link', { name: 'Quote Block' })).toBeInTheDocument();
    expect(screen.queryByText('component')).not.toBeInTheDocument();
    expect(screen.queryByText(/components\/feature-card/)).not.toBeInTheDocument();
    expect(screen.queryByRole('img')).not.toBeInTheDocument();
  });
});
