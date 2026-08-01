import { fireEvent, render, screen } from '@testing-library/react';
import { describe, it, expect, beforeEach } from 'vitest';
import { Timeline } from './Timeline';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { useRunStore } from '../../stores/runStore';

// Mock ResizeObserver（scrollIntoView 触发需要）
globalThis.ResizeObserver = class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

describe('Timeline user_turn rendering', () => {
  beforeEach(() => {
    useProjectStore.setState({ activeProjectId: 'p1' });
    useThreadStore.setState({ activeThreadIdByProjectId: { p1: 't1' } });
    useRunStore.setState({
      sessions: {
        t1: {
          activeRunId: null,
          status: 'idle' as const,
          target: { artifact: 'presentation' as const, level: 'slide' as const },
          interaction: { intent: 'apply' as const, clarification: 'when_blocked' as const },
          timelineItems: [
            { id: 'u1', type: 'user_turn', text: 'first message', timestamp: 1 },
            { id: 'm1', type: 'markdown', text: 'agent reply', timestamp: 2 },
          ],
          pendingInput: null,
          progress: null,
          eventSourceClose: null,
          plan: null,
        },
      },
    });
  });

  it('renders user_turn text via markdown component', () => {
    render(<Timeline />);
    expect(screen.getByText('first message')).toBeInTheDocument();
    expect(screen.getByText('agent reply')).toBeInTheDocument();
  });

  it('renders user_turn markdown inline formatting', () => {
    useRunStore.setState({
      sessions: {
        t1: {
          activeRunId: null,
          status: 'idle' as const,
          target: { artifact: 'presentation' as const, level: 'slide' as const },
          interaction: { intent: 'apply' as const, clarification: 'when_blocked' as const },
          timelineItems: [
            { id: 'u2', type: 'user_turn', text: 'echo `code`', timestamp: 1 },
          ],
          pendingInput: null,
          progress: null,
          eventSourceClose: null,
          plan: null,
        },
      },
    });
    const { container } = render(<Timeline />);
    // 内联 code 应渲染为 <code>
    expect(container.querySelector('code')?.textContent).toBe('code');
  });
});

describe('Timeline thinking bubble', () => {
  beforeEach(() => {
    useProjectStore.setState({ activeProjectId: 'p1' });
    useThreadStore.setState({ activeThreadIdByProjectId: { p1: 't1' } });
  });

  const sessionWith = (items: any[], overrides: Partial<any> = {}) => ({
    sessions: {
      t1: {
        activeRunId: 'r1',
        status: 'running',
        target: { artifact: 'presentation', level: 'slide' },
        interaction: { intent: 'apply', clarification: 'when_blocked' },
        timelineItems: items,
        pendingInput: null,
        progress: null,
        eventSourceClose: null,
        plan: null,
        ...overrides,
      } as any,
    },
  });

  it('shows ThinkingBubble when running and last item is user_turn (no agent content yet)', () => {
    useRunStore.setState(sessionWith([
      { id: 'u1', type: 'user_turn', text: 'go', timestamp: 1 },
    ]));
    render(<Timeline />);
    expect(screen.getByText('正在思考…')).toBeInTheDocument();
  });

  it('hides ThinkingBubble once strategy is selected', () => {
    useRunStore.setState(sessionWith([
      { id: 'u1', type: 'user_turn', text: 'go', timestamp: 1 },
      { id: 'strategy1', type: 'strategy_status', strategy: 'respond', reason: 'consult', risk: 'low', complexity: 'low', timestamp: 2 },
    ]));
    render(<Timeline />);
    expect(screen.queryByText('正在思考…')).toBeNull();
  });

  it('hides ThinkingBubble after markdown / token arrives', () => {
    useRunStore.setState(sessionWith([
      { id: 'u1', type: 'user_turn', text: 'go', timestamp: 1 },
      { id: 'md1', type: 'markdown', text: 'partial', timestamp: 2 },
    ]));
    render(<Timeline />);
    expect(screen.queryByText('正在思考…')).toBeNull();
  });

  it('does NOT show ThinkingBubble when status=idle', () => {
    useRunStore.setState(sessionWith(
      [{ id: 'u1', type: 'user_turn', text: 'go', timestamp: 1 }],
      { status: 'idle' },
    ));
    render(<Timeline />);
    expect(screen.queryByText('正在思考…')).toBeNull();
  });

  it('does NOT show ThinkingBubble when running but no user_turn (no context to attach)', () => {
    useRunStore.setState(sessionWith([]));
    render(<Timeline />);
    expect(screen.queryByText('正在思考…')).toBeNull();
  });
});

describe('Timeline error friendly messages', () => {
  beforeEach(() => {
    useProjectStore.setState({ activeProjectId: 'p1' });
    useThreadStore.setState({ activeThreadIdByProjectId: { p1: 't1' } });
  });

  const sessionWithError = (code: string | undefined, message: string) => ({
    sessions: {
      t1: {
        activeRunId: null,
        status: 'idle' as const,
        target: { artifact: 'presentation' as const, level: 'slide' as const },
        interaction: { intent: 'apply' as const, clarification: 'when_blocked' as const },
        timelineItems: [
          { id: 'e1', type: 'error', code, message, timestamp: 1 } as any,
        ],
        pendingInput: null,
        progress: null,
        eventSourceClose: null,
        plan: null,
      },
    },
  });

  it('renders friendly Chinese text for LLM_TIMEOUT', () => {
    useRunStore.setState(sessionWithError('LLM_TIMEOUT', 'context deadline exceeded'));
    render(<Timeline />);
    expect(screen.getByText(/AI 响应超时，请稍后重试或降低任务复杂度/)).toBeInTheDocument();
  });

  it('renders friendly Chinese text for LLM_BAD_REQUEST', () => {
    useRunStore.setState(sessionWithError('LLM_BAD_REQUEST', 'bad tool call'));
    render(<Timeline />);
    expect(screen.getByText(/AI 请求未能处理/)).toBeInTheDocument();
  });

  it('falls back to raw message when code is unknown', () => {
    useRunStore.setState(sessionWithError('SOMETHING_ELSE', 'weird failure'));
    render(<Timeline />);
    expect(screen.getByText(/运行未能完成/)).toBeInTheDocument();
    fireEvent.click(screen.getByText('错误详情'));
    expect(screen.getByText(/weird failure/)).toBeInTheDocument();
  });
});
