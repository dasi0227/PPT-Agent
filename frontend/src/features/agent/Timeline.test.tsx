import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useProjectStore } from '../../stores/projectStore';
import { IDLE_SESSION, useRunStore } from '../../stores/runStore';
import { useThreadStore } from '../../stores/threadStore';
import type { TimelineItem } from './eventReducer';
import { Timeline } from './Timeline';

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

  afterEach(() => vi.restoreAllMocks());

  it('places a newly sent message at the top and keeps it there as the agent replies', () => {
    render(<Timeline />);
    const scroll = screen.getByTestId('timeline-scroll');
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
          id: 'reply', type: 'reasoning', messageId: 'reply', text: '正在处理', timestamp: 3,
        }],
      } } }));
    });

    expect(scroll.scrollTop).toBe(188);
  });
});
