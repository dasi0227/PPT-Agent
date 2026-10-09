import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useProjectStore } from '../../stores/projectStore';
import { IDLE_SESSION, useRunStore } from '../../stores/runStore';
import { useThreadStore } from '../../stores/threadStore';
import type { TimelineItem, ToolActivityItem } from './eventReducer';
import { Timeline } from './Timeline';
import { ToolActivityRow } from './ActivityRows';
import { useDeckStore } from '../../stores/deckStore';
import { useResourceApprovalStore } from '../../stores/resourceApprovalStore';

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
    act(() => {
      useResourceApprovalStore.getState().close();
      useDeckStore.getState().setActiveDocument(null);
    });
  });

  it('keeps edit approval on the yellow tool row and opens the candidate canvas', () => {
    const target = { type: 'deck' as const, part: 'manifest' as const, diff: {
      kind: 'fields' as const, status: 'modified' as const, filename: '.manifest.json', fields: [],
    } };
    const item: ToolActivityItem = {
      id: 'r1:tool:c1', type: 'tool', runId: 'r1', callId: 'c1', tool: 'edit_manifest',
      label: '编辑内容要求', status: 'running', timestamp: 1, target,
      approval: { interactionId: 'resa_1', resource: 'manifest', revision: 1, target },
    };
    render(<ToolActivityRow item={item} />);
    expect(screen.getByText('待人工介入')).toBeInTheDocument();
    expect(screen.getByRole('region', { name: '变更差异' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '手动编辑' }));
    expect(useResourceApprovalStore.getState().active).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '继续' }));
    expect(useResourceApprovalStore.getState().active).toMatchObject({ runId: 'r1', interactionId: 'resa_1', resource: 'manifest' });
    expect(useDeckStore.getState().activeDocument).toBe('manifest');
  });

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
