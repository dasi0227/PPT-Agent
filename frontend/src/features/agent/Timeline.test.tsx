import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useProjectStore } from '../../stores/projectStore';
import { IDLE_SESSION, useRunStore } from '../../stores/runStore';
import { useThreadStore } from '../../stores/threadStore';
import { reduceSSEEvent, type TimelineItem, type ToolActivityItem } from './eventReducer';
import { Timeline } from './Timeline';
import { ToolActivityRow } from './ActivityRows';
import { useDeckStore } from '../../stores/deckStore';
import { useResourceApprovalStore } from '../../stores/resourceApprovalStore';
import { reduceRunTokenUsage } from './runTokenUsage';

vi.mock('./useCommandHistoryRecovery', () => ({ useCommandHistoryRecovery: () => undefined }));

describe('Timeline scrolling after sending', () => {
  beforeEach(() => {
    useProjectStore.setState({ activeProjectId: 'p1' });
    useThreadStore.setState({ activeThreadIdByProjectId: { p1: 't1' } });
    useRunStore.setState({ sessions: { t1: {
      ...IDLE_SESSION,
      projectId: 'p1',
      activeRunId: 'r1',
      status: 'running',
      timelineItems: [{ id: 'old', type: 'user_turn', text: '之前的消息', timestamp: 1 }],
    } } });
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
      return new DOMRect(0, this.dataset.testid === 'latest-turn' ? 200 : 0, 100, 100);
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    act(() => {
      useResourceApprovalStore.getState().close();
      useDeckStore.getState().setActiveDocument(null);
    });
  });

  it.each([
    ['input_rejected', 'Dasi 执行完成'], ['user_requested', '执行中断'], ['input_detection_failed', '系统异常'],
  ] as const)('renders %s with its correct result and preserves time and Tokens', (reason, label) => {
    const timelineItems = reduceSSEEvent([
      { id: 'user', type: 'user_turn', runId: 'r1', text: '请求', timestamp: 1 },
    ], { event: 'run.canceled', data: {
      schema_version: 6, run_id: 'r1', occurred_at: '2026-11-01T00:00:00Z', duration_ms: 1000,
      affected_targets: [], error: null, trace_id: 'r1', reason,
      ...(reason !== 'user_requested' ? { reply: { input_id: 'request', text: '固定答复' } } : {}),
    } });
    useRunStore.setState(state => ({ sessions: { ...state.sessions, t1: {
      ...state.sessions.t1, status: 'canceled', timelineItems,
      tokenUsageByRun: reduceRunTokenUsage({}, { event: 'run.usage.updated', data: {
        schema_version: 6, run_id: 'r1', occurred_at: '2026-11-01T00:00:00Z', call: {
          call_id: 'decision', provider: 'jev', model: 'model', duration_ms: 1000,
          usage: { input_tokens: 900, output_tokens: 67, total_tokens: 967 },
        },
      } }),
    } } }));
    render(<Timeline />);
    expect(screen.getByText(`${label}，耗时 1s，耗费 967 Tokens`)).toBeInTheDocument();
    if (reason === 'input_rejected') {
      expect(screen.queryByText(/执行中断|系统异常/)).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: '复制回复' })).toBeInTheDocument();
    }
  });

  it('updates Run consumption after model calls and retains it in the terminal summary', () => {
    useRunStore.setState((state) => ({ sessions: { ...state.sessions, t1: {
      ...state.sessions.t1,
      runClock: { elapsedMs: 2000, runningSince: null },
      timelineItems: [{ id: 'user', type: 'user_turn', runId: 'r1', text: '生成演示文稿', timestamp: 1 }],
    } } }));
    render(<Timeline />);
    expect(screen.getByText('Dasi 正在执行，已进行 2s')).toBeInTheDocument();
    expect(screen.queryByText(/Tokens/)).not.toBeInTheDocument();

    for (const [callId, provider, input, output] of [
      ['main', 'openai', 1_400_000, 72_088], ['decision', 'jev', 20, 14],
    ] as const) {
      act(() => {
        useRunStore.setState((state) => ({ sessions: { ...state.sessions, t1: {
          ...state.sessions.t1,
          tokenUsageByRun: reduceRunTokenUsage(state.sessions.t1.tokenUsageByRun, {
            event: 'run.usage.updated', data: {
              schema_version: 6, run_id: 'r1', occurred_at: '2026-11-01T00:00:00Z',
              call: { call_id: callId, provider, model: 'model', duration_ms: 10,
                usage: { input_tokens: input, output_tokens: output, total_tokens: input + output } },
            },
          }),
        } } }));
      });
    }
    expect(screen.getByText(/已进行 2s，已耗费 1\.47M Tokens/)).toBeInTheDocument();

    act(() => {
      useRunStore.setState(state => ({ sessions: { ...state.sessions, t1: { ...state.sessions.t1, status: 'waiting' } } }));
    });
    expect(screen.getByText('Dasi 等待你的确认，已进行 2s，已耗费 1.47M Tokens')).toBeInTheDocument();
    expect(screen.queryByText(/Dasi 正在执行/)).not.toBeInTheDocument();

    act(() => {
      useRunStore.setState((state) => ({ sessions: { ...state.sessions, t1: {
        ...state.sessions.t1,
        status: 'done',
        timelineItems: [...state.sessions.t1.timelineItems, {
          id: 'final', type: 'final', runId: 'r1', messageId: 'final', text: '已完成',
          timestamp: 2, durationMs: 3000, affectedTargets: [],
        }],
      } } }));
    });
    expect(screen.getByText('Dasi 执行完成，耗时 3s，耗费 1.47M Tokens')).toBeInTheDocument();
    expect(screen.queryByText(/已耗费/)).not.toBeInTheDocument();
  });

  it('keeps edit approval on the yellow tool row and opens the candidate canvas', () => {
    const target = { type: 'deck' as const, part: 'meta' as const, diff: {
      kind: 'fields' as const, status: 'modified' as const, filename: '.meta.json', fields: [],
    } };
    const item: ToolActivityItem = {
      id: 'r1:tool:c1', type: 'tool', runId: 'r1', callId: 'c1', tool: 'edit_meta',
      label: '编辑基本信息', status: 'running', timestamp: 1, target,
      approval: { interactionId: 'resa_1', resource: 'meta', revision: 1, target },
    };
    render(<ToolActivityRow item={item} />);
    expect(screen.getByText('待人工介入')).toBeInTheDocument();
    expect(screen.getByRole('region', { name: '变更差异' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '手动编辑' }));
    expect(useResourceApprovalStore.getState().active).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '继续' }));
    expect(useResourceApprovalStore.getState().active).toMatchObject({ runId: 'r1', interactionId: 'resa_1', resource: 'meta' });
    expect(useDeckStore.getState().activeDocument).toBe('meta');
  });

  it('keeps a new message aligned during replies and resizing until the user scrolls away', () => {
    const resizeCallbacks = new Map<Element, () => void>();
    vi.stubGlobal('ResizeObserver', class {
      private readonly callback: () => void;
      constructor(callback: () => void) { this.callback = callback; }
      observe(target: Element) { resizeCallbacks.set(target, this.callback); }
      disconnect() {}
    });
    let turnTop = 200;
    vi.mocked(HTMLElement.prototype.getBoundingClientRect).mockImplementation(function (this: HTMLElement) {
      const scrollTop = screen.queryByTestId('timeline-scroll')?.scrollTop ?? 0;
      return new DOMRect(0, this.dataset.testid === 'latest-turn' ? turnTop - scrollTop : 0, 100, 100);
    });
    render(<Timeline />);
    const scroll = screen.getByTestId('timeline-scroll');
    let viewportWidth = 360;
    Object.defineProperties(scroll, {
      clientWidth: { configurable: true, get: () => viewportWidth },
      clientHeight: { configurable: true, value: 400 },
      scrollHeight: { configurable: true, value: 1200 },
    });
    act(() => resizeCallbacks.get(scroll)?.());
    const newMessage: TimelineItem = { id: 'user_new', type: 'user_turn', text: '刚发送的消息', timestamp: 2 };

    act(() => {
      useRunStore.setState((state) => ({ sessions: { ...state.sessions, t1: {
        ...state.sessions.t1,
        timelineItems: [...state.sessions.t1.timelineItems, newMessage],
      } } }));
    });

    expect(screen.getByTestId('latest-turn')).toHaveClass('min-h-full');
    expect(scroll.scrollTop).toBe(188);
    fireEvent.scroll(scroll);

    act(() => {
      useRunStore.setState((state) => ({ sessions: { ...state.sessions, t1: {
        ...state.sessions.t1,
        timelineItems: [...state.sessions.t1.timelineItems, {
          id: 'reply', type: 'thinking', messageId: 'reply', text: '正在处理', timestamp: 3,
        }],
      } } }));
    });

    expect(scroll.scrollTop).toBe(188);

    viewportWidth = 720;
    turnTop = 160;
    scroll.scrollTop = 180;
    fireEvent.scroll(scroll);
    act(() => resizeCallbacks.get(scroll)?.());
    expect(scroll.scrollTop).toBe(148);

    scroll.scrollTop = 40;
    fireEvent.scroll(scroll);
    viewportWidth = 360;
    turnTop = 200;
    act(() => resizeCallbacks.get(scroll)?.());
    expect(scroll.scrollTop).toBe(40);
  });

  it('shows activity while a resumed run has not emitted its next progress event', () => {
    render(<Timeline />);
    const activeLabel = 'Dasi 正在推进任务';
    expect(screen.getByRole('img', { name: activeLabel })).toBeInTheDocument();

    act(() => {
      useRunStore.setState((state) => ({ sessions: { ...state.sessions, t1: {
        ...state.sessions.t1, status: 'waiting', progress: null,
      } } }));
    });
    expect(screen.queryByRole('img', { name: activeLabel })).not.toBeInTheDocument();

    act(() => {
      useRunStore.setState((state) => ({ sessions: { ...state.sessions, t1: {
        ...state.sessions.t1, status: 'running', progress: null,
      } } }));
    });
    expect(screen.getByRole('img', { name: activeLabel })).toBeInTheDocument();
  });
});
