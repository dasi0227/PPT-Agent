import { act, fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { useRunStore } from '../../stores/runStore';
import { useThreadStore } from '../../stores/threadStore';
import type { TimelineItem, ToolActivityItem } from './eventReducer';
import { Timeline } from './Timeline';
import { groupTimelineItems } from './timelineGrouping';

const scrollTo = vi.fn();
Object.defineProperty(HTMLElement.prototype, 'scrollTo', { configurable: true, value: scrollTo });

function setSession(items: TimelineItem[], overrides: Record<string, unknown> = {}) {
  useRunStore.setState({
    sessions: {
      t1: {
        activeRunId: 'r1',
        status: 'running',
        target: { artifact: 'presentation', level: 'deck' },
        interaction: { intent: 'execute' },
        timelineItems: items,
        pendingQuestion: null,
        progress: null,
        eventSourceClose: null,
        plan: null,
        ...overrides,
      } as any,
    },
  });
}

function tool(id: string, overrides: Partial<ToolActivityItem> = {}): ToolActivityItem {
  return {
    id, type: 'tool', runId: 'r1', callId: id, tool: 'write_ppt',
    planStepId: 'build', label: `已生成 ${id}`, status: 'completed', timestamp: 1,
    target: { type: 'slide', slide_id: id, part: 'html' },
    ...overrides,
  };
}

describe('Timeline', () => {
  beforeEach(() => {
    scrollTo.mockClear();
    useProjectStore.setState({
      activeProjectId: 'p1',
      slidesByProjectId: { p1: [{ id: 'current', title: 'Current', order_index: 0 } as any] },
    });
    useDeckStore.setState({ currentPage: 0 });
    useThreadStore.setState({ activeThreadIdByProjectId: { p1: 't1' } });
  });

  it('renders distinct reasoning, milestone, final, and a plan after the user turn', () => {
    setSession([
      { id: 'u1', type: 'user_turn', text: '生成 PPT', timestamp: 1 },
      { id: 'r1', type: 'reasoning', messageId: 'm1', text: '我先确认全局设计。', timestamp: 2 },
      { id: 'm1', type: 'milestone', messageId: 'm2', text: '全局设计已经完成。', completedStepIds: ['s1'], timestamp: 3 },
      { id: 'f1', type: 'final', messageId: 'm3', text: '整份演示文稿已经完成。', affectedTargets: [], timestamp: 4 },
    ], {
      status: 'done',
      plan: {
        id: 'p1', title: '执行计划', revision: 2,
        steps: [{ id: 's1', title: '设计', status: 'completed' }],
      },
    });
    render(<Timeline />);
    expect(screen.getByText('生成 PPT')).toBeInTheDocument();
    expect(screen.getByText('我先确认全局设计。')).toBeInTheDocument();
    expect(screen.getByText('全局设计已经完成。')).toBeInTheDocument();
    expect(screen.getByText('整份演示文稿已经完成。')).toBeInTheDocument();
    expect(screen.getByText('执行计划')).toBeInTheDocument();
    expect(screen.queryByText(/Context|Strategy|Completion Gate/)).toBeNull();
  });

  it('groups 3+ consecutive safe completions but never hides running, failed, warning, or current-page rows', () => {
    const grouped = groupTimelineItems([tool('s1'), tool('s2'), tool('s3')], 'current');
    expect(grouped).toHaveLength(1);
    expect(grouped[0].kind).toBe('tool_group');

    const visible = groupTimelineItems([
      tool('s1'),
      tool('running', { status: 'running' }),
      tool('failed', { status: 'failed' }),
      tool('warning', { preview: { slide_id: 'warning', image_url: '/api/v1/runs/r/screenshots/x', warnings: ['overflow'] } }),
      tool('current'),
    ], 'current');
    expect(visible.every((entry) => entry.kind === 'item')).toBe(true);
  });

  it('shows live progress only while not waiting for a question', () => {
    setSession([{ id: 'u1', type: 'user_turn', text: '开始', timestamp: 1 }], {
      progress: { stage: 'rendering', text: '正在检查第 6 页' },
    });
    const { rerender } = render(<Timeline />);
    expect(screen.getByText('正在检查第 6 页')).toBeInTheDocument();
    act(() => {
      setSession([{ id: 'u1', type: 'user_turn', text: '开始', timestamp: 1 }], {
        status: 'waiting',
        pendingQuestion: { id: 'q1', prompt: '选择' },
        progress: { stage: 'rendering', text: '不应显示' },
      });
    });
    rerender(<Timeline />);
    expect(screen.queryByText('不应显示')).toBeNull();
  });

  it('stops auto-follow when the user leaves the bottom and offers return to latest', () => {
    setSession([{ id: 'u1', type: 'user_turn', text: '开始', timestamp: 1 }]);
    const { container, rerender } = render(<Timeline />);
    const scroller = container.querySelector('.overflow-y-auto') as HTMLDivElement;
    Object.defineProperties(scroller, {
      scrollHeight: { configurable: true, value: 1000 },
      clientHeight: { configurable: true, value: 300 },
      scrollTop: { configurable: true, writable: true, value: 100 },
    });
    fireEvent.scroll(scroller);
    act(() => {
      setSession([
        { id: 'u1', type: 'user_turn', text: '开始', timestamp: 1 },
        { id: 'r1', type: 'reasoning', messageId: 'm1', text: '新消息', timestamp: 2 },
      ]);
    });
    rerender(<Timeline />);
    expect(screen.getByRole('button', { name: /回到最新/ })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /回到最新/ }));
    expect(scrollTo).toHaveBeenCalled();
  });

  it('uses instant scrolling when reduced motion is requested', () => {
    const previous = window.matchMedia;
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      value: vi.fn().mockReturnValue({ matches: true }),
    });
    setSession([{ id: 'u1', type: 'user_turn', text: '开始', timestamp: 1 }]);
    render(<Timeline />);
    expect(scrollTo).toHaveBeenCalledWith(expect.objectContaining({ behavior: 'auto' }));
    Object.defineProperty(window, 'matchMedia', { configurable: true, value: previous });
  });
});
