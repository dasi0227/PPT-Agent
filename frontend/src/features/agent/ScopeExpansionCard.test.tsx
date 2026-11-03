import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { runsApi } from '../../api/runs';
import type { ProjectContentSnapshot } from '../../api/types';
import { useProjectStore } from '../../stores/projectStore';
import { useSessionStore } from '../../stores/sessionStore';
import { sessionFixture, interactionFixture, receiptFixture } from '../../test/sessionFixtures';
import { ScopeExpansionCard } from './ScopeExpansionCard';
import type { ScopeExpansionItem } from './timelineTypes';

vi.mock('./useActiveSession', () => ({
  useActiveThreadId: () => 'thread-1',
}));

const item: ScopeExpansionItem = {
  sceneRevision: '1', requestRevision: '1', interactionState: 'pending',
  id: 'r1:scope-expansion:se1',
  type: 'scope_expansion',
  runId: 'r1',
  interactionId: 'se1',
  callId: 'c1',
  baseRevision: '1',
  currentScope: {
    slide_ids: ['sli_one'],
    source: { kind: 'current_page' },
    include_run_created_slides: false,
    revision: '1',
  },
  requestedAddition: { slide_ids: ['sli_two'] },
  proposedScope: {
    slide_ids: ['sli_one', 'sli_two'],
    source: { kind: 'custom_pages' },
    include_run_created_slides: false,
    revision: '2',
  },
  affectedPageCount: 2,
  reason: '需要同步第三页的设计稿。',
  timestamp: 1,
};

const snapshot: ProjectContentSnapshot = {
  project_title: 'Test', project_id: 'project-1',
  theme: 'clean',
  appearance: null,
  hashes: {},
  meta: { title: 'Demo', goal: '', audience: '', language: 'zh-CN', count: '待明确' },
  outline: { sections: [{
    id: 'section-1', title: '', subsections: [],
    slides: [
      { id: 'sli_one', title: '开场' },
      { id: 'sli_middle', title: '过渡' },
      { id: 'sli_two', title: '结论' },
    ],
  }] },
  constraints: { content: { requirements: [], prohibitions: [] }, visual: { requirements: [], prohibitions: [] } },
  decorations: { slide_pagination: 'bottom-right', section_title: 'top-left', deck_title: 'none', logo: 'none' }, logo: null,
  slides_by_id: {},
};

afterEach(() => {
  vi.restoreAllMocks();
  act(() => {
    useSessionStore.setState({ sessions: {} });
    useProjectStore.setState({ activeProjectId: null, contentByProjectId: {} });
  });
});

describe('ScopeExpansionCard', () => {
  it('submits an all-page adjustment with the pending interaction identity', async () => {
    const submit = vi.spyOn(runsApi, 'submitScopeExpansion').mockResolvedValue(receiptFixture());
    useProjectStore.setState({ activeProjectId: 'project-1', contentByProjectId: { 'project-1': snapshot } });
    useSessionStore.setState({ sessions: { 'thread-1': sessionFixture({
      threadId: 'thread-1', interactionsById: { se1: interactionFixture({ id: 'se1', kind: 'scope', thread_id: 'thread-1' }) },
    }) } });

    render(<ScopeExpansionCard item={item} />);
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '允许全部页' }));
      expect(submit).not.toHaveBeenCalled();
    });
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '继续' }));
    });

    await waitFor(() => {
      expect(submit).toHaveBeenCalledWith('r1', {
        scene_revision: '1', request_revision: '1',
        interaction_id: 'se1',
        call_id: 'c1',
        base_revision: '1',
        decision: 'revise',
        feedback: undefined,
      });
    });
  });
});
