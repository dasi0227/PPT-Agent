import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { useSessionStore } from '../../stores/sessionStore';
import { groupFixture, itemFixture, runFixture, sessionFixture, threadFixture } from '../../test/sessionFixtures';
import { Timeline } from './Timeline';

beforeEach(() => {
  useProjectStore.setState({ activeProjectId: 'p1' });
  useThreadStore.setState({ activeThreadIdByProjectId: { p1: 't1' } });
  useSessionStore.setState({ sessions: {} });
});
afterEach(() => { useSessionStore.getState().drop(['t1']); vi.restoreAllMocks(); });

describe('authoritative timeline display', () => {
  it('shows init only after an empty view succeeds, never on loading or error', () => {
    render(<Timeline />);
    expect(screen.getByLabelText('加载中')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Dasi PPT Agent' })).not.toBeInTheDocument();
    act(() => useSessionStore.setState({ sessions: { t1: sessionFixture({ loadStatus: 'error', isEmpty: false, error: new Error('offline') }) } }));
    expect(screen.getByRole('alert')).toHaveTextContent('offline');
    expect(screen.queryByRole('button', { name: 'Dasi PPT Agent' })).not.toBeInTheDocument();
    act(() => useSessionStore.setState({ sessions: { t1: sessionFixture() } }));
    expect(screen.getByRole('button', { name: 'Dasi PPT Agent' })).toBeInTheDocument();
  });

  it('renders a saved terminal group with lazy process loading even with no inline records', () => {
    const group = groupFixture({ status: 'done', summary: { active_duration_ms: 3000 }, items_mode: 'summary', has_more_items: true });
    useSessionStore.setState({ sessions: { t1: sessionFixture({ isEmpty: false,
      groupsById: { [group.id]: group }, orderedGroupIds: [group.id],
      groupPages: { [group.id]: { mode: 'summary', hasMore: true, olderCursor: null, status: 'idle', error: null } },
    }) } });
    const load = vi.spyOn(useSessionStore.getState(), 'loadGroup').mockResolvedValue();
    const { container } = render(<Timeline />);
    expect(screen.queryByRole('button', { name: 'Dasi PPT Agent' })).not.toBeInTheDocument();
    expect(container.innerHTML).not.toContain('timeline-enter');
    fireEvent.click(screen.getByRole('button', { name: /Dasi 执行完成，耗时 3s/ }));
    expect(load).toHaveBeenCalledWith('t1', group.id);
  });

  it('displays committed usage and terminal state without replaying model calls', () => {
    const run = runFixture(), input = itemFixture(), group = groupFixture({ summary: {
      active_duration_ms: 2000, usage: { calls: 2, unreported_calls: 0, input_tokens: 1400000, output_tokens: 72122, total_tokens: 1472122 },
    } });
    const source = sessionFixture({ isEmpty: false, thread: threadFixture({ active_run_id: run.id }),
      runsById: { [run.id]: run }, groupsById: { [group.id]: group }, orderedGroupIds: [group.id], itemsById: { [input.id]: input },
      groupPages: { [group.id]: { mode: 'page', hasMore: false, olderCursor: null, status: 'idle', error: null } },
    });
    useSessionStore.setState({ sessions: { t1: source } });
    render(<Timeline />);
    expect(screen.getByText(/已进行 2s，已耗费 1\.47M Tokens/)).toBeInTheDocument();
    const final = itemFixture({ id: 'final', kind: 'final', position: '2', created_seq: '2', updated_seq: '2', summary: { text: '已完成' } });
    act(() => useSessionStore.setState({ sessions: { t1: { ...source,
      thread: threadFixture(), groupsById: { [group.id]: { ...group, status: 'done', summary: { ...group.summary, active_duration_ms: 3000 } } },
      itemsById: { ...source.itemsById, final },
    } } }));
    expect(screen.getByRole('button', { name: /Dasi 执行完成，耗时 3s，耗费 1\.47M Tokens/ })).toBeInTheDocument();
    expect(screen.getByText('已完成')).toBeInTheDocument();
    expect(screen.queryByText(/Dasi 正在执行/)).not.toBeInTheDocument();
  });
});
