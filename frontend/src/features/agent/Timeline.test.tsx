import { render, screen } from '@testing-library/react';
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
          status: 'idle',
          mode: 'normal',
          scope: 'current',
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
          status: 'idle',
          mode: 'normal',
          scope: 'current',
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
