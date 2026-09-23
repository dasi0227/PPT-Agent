import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { projectHistoryApi } from '../../api/projectHistory';
import { projectsApi } from '../../api/projects';
import { loadProjectComposer, useComposerStore } from '../../stores/composerStore';
import { useProjectStore } from '../../stores/projectStore';
import { useProjectHistoryStore } from '../../stores/projectHistoryStore';
import { IDLE_SESSION, useRunStore } from '../../stores/runStore';
import { useThreadStore } from '../../stores/threadStore';
import { CommandComposer } from './CommandComposer';
import { hydrateRunFromHistory, type HistoryEntry } from './historyHydrator';

const finalHistory = (runId: string, suggestion: string): HistoryEntry[] => {
  const base = { schema_version: 6, run_id: runId, occurred_at: '2026-09-23T00:00:00Z' };
  return [
    { seq: 1, ts: 1, run_id: runId, turn: 'agent', type: 'message.final', data: {
      ...base, message_id: `final-${runId}`, text: '完成', affected_targets: [], suggested_next_inputs: [suggestion],
    } },
    { seq: 2, ts: 2, run_id: runId, turn: 'agent', type: 'run.completed', data: {
      ...base, duration_ms: 1, affected_targets: [], error: null, trace_id: runId,
    } },
  ];
};

function restoreThread(threadId: string, runId: string, suggestion: string) {
  const hydrated = hydrateRunFromHistory(finalHistory(runId, suggestion));
  useRunStore.setState((state) => ({ sessions: { ...state.sessions, [threadId]: {
    ...IDLE_SESSION, ...hydrated.session, projectId: 'p1', timelineItems: hydrated.items,
  } } }));
}

describe('CommandComposer suggestions', () => {
  beforeEach(() => {
    localStorage.clear();
    loadProjectComposer(null);
    useRunStore.setState({ sessions: {} });
    useProjectStore.setState({ activeProjectId: 'p1', contentByProjectId: {}, contentLoadingByProjectId: { p1: true }, contentErrorByProjectId: {} });
    useProjectHistoryStore.setState({ states: {}, stateErrorByProjectId: { p1: true } });
    useThreadStore.setState({ activeThreadIdByProjectId: { p1: 't1' } });
    restoreThread('t1', 'r1', '优化第一页');
  });
  afterEach(() => vi.restoreAllMocks());

  it('shows persisted suggestions without project validation, survives project changes, and fills only a draft', async () => {
    const history = vi.spyOn(projectHistoryApi, 'state');
    const content = vi.spyOn(projectsApi, 'getContent');
    const create = vi.spyOn(useRunStore.getState(), 'createRun');
    render(<CommandComposer />);
    const suggestion = () => screen.getByRole('button', { name: /1\. 优化第一页/ });
    await waitFor(() => expect(suggestion()).toBeVisible());
    act(() => {
      useProjectHistoryStore.setState({ states: { p1: { revision: 999, scene_revision: 50, checkpoints: [] } } });
      useProjectStore.setState({ contentLoadingByProjectId: { p1: false }, contentErrorByProjectId: { p1: '内容刷新失败' } });
      restoreThread('t2', 'r2', '检查结论');
    });
    expect(suggestion()).toBeVisible();
    expect(history).not.toHaveBeenCalled();
    expect(content).not.toHaveBeenCalled();
    fireEvent.click(suggestion());
    expect(useComposerStore.getState().threadDrafts.t1).toBe('优化第一页');
    expect(screen.queryByRole('button', { name: /1\. 优化第一页/ })).not.toBeInTheDocument();
    expect(create).not.toHaveBeenCalled();
    const editor = screen.getByRole('textbox');
    editor.textContent = '';
    fireEvent.input(editor);
    expect(suggestion()).toBeVisible();
  });

  it('keeps each thread’s suggestions and restores the suggestions from the selected checkpoint history', async () => {
    restoreThread('t2', 'r2', '检查结论');
    render(<CommandComposer />);
    await waitFor(() => expect(screen.getByRole('button', { name: /1\. 优化第一页/ })).toBeVisible());
    act(() => useThreadStore.setState({ activeThreadIdByProjectId: { p1: 't2' } }));
    expect(screen.getByRole('button', { name: /1\. 检查结论/ })).toBeVisible();
    act(() => {
      useThreadStore.setState({ activeThreadIdByProjectId: { p1: 't1' } });
      useRunStore.setState({ sessions: {} });
      useProjectHistoryStore.setState({ states: { p1: { revision: 1000, scene_revision: 1000, checkpoints: [] } } });
      restoreThread('t1', 'checkpoint-run', '补充历史页面的说明');
    });
    expect(screen.getByRole('button', { name: /1\. 补充历史页面的说明/ })).toBeVisible();
    expect(screen.queryByRole('button', { name: /1\. 优化第一页/ })).not.toBeInTheDocument();
  });
});
