import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { runsApi } from '../../api/runs';
import { useProjectStore } from '../../stores/projectStore';
import { IDLE_SESSION, useRunStore } from '../../stores/runStore';
import { ScopeExpansionCard } from './ScopeExpansionCard';
import type { ScopeExpansionItem } from './eventReducer';

vi.mock('./useActiveSession', () => ({
  useActiveThreadId: () => 'thread-1',
}));

const item: ScopeExpansionItem = {
  id: 'r1:scope-expansion:se1',
  type: 'scope_expansion',
  runId: 'r1',
  interactionId: 'se1',
  callId: 'c1',
  baseRevision: 1,
  currentScope: {
    object: 'html',
    slide_ids: ['sli_one'],
    source: { kind: 'current_page' },
    include_run_created_slides: false,
    revision: 1,
  },
  requestedAddition: { slide_ids: ['sli_two'], object: 'spec' },
  proposedScope: {
    object: 'presentation',
    slide_ids: ['sli_one', 'sli_two'],
    source: { kind: 'custom_pages' },
    include_run_created_slides: false,
    revision: 2,
  },
  affectedPageCount: 2,
  reason: '需要同步第二页的设计稿。',
  timestamp: 1,
};

afterEach(() => {
  vi.restoreAllMocks();
  useRunStore.setState({ sessions: {} });
  useProjectStore.setState({ activeProjectId: null, contentByProjectId: {} });
});

describe('ScopeExpansionCard', () => {
  it('submits a global adjustment with the pending interaction identity', async () => {
    const submit = vi.spyOn(runsApi, 'submitScopeExpansion').mockResolvedValue(undefined);
    useRunStore.setState({
      sessions: {
        'thread-1': {
          ...IDLE_SESSION,
          activeRunId: 'r1',
          status: 'waiting',
          timelineItems: [item],
        },
      },
    });

    render(<ScopeExpansionCard item={item} />);
    fireEvent.click(screen.getByRole('button', { name: '调整为全局' }));

    await waitFor(() => {
      expect(submit).toHaveBeenCalledWith('r1', {
        interaction_id: 'se1',
        call_id: 'c1',
        base_revision: 1,
        decision: 'adjust',
        adjusted_scope: { object: 'global', selection: { kind: 'all_pages' } },
      });
    });
  });
});
