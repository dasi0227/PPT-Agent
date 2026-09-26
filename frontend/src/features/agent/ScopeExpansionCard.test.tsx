import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { runsApi } from '../../api/runs';
import type { ProjectContentSnapshot } from '../../api/types';
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
    slide_ids: ['sli_one'],
    source: { kind: 'current_page' },
    include_run_created_slides: false,
    revision: 1,
  },
  requestedAddition: { slide_ids: ['sli_two'] },
  proposedScope: {
    slide_ids: ['sli_one', 'sli_two'],
    source: { kind: 'custom_pages' },
    include_run_created_slides: false,
    revision: 2,
  },
  affectedPageCount: 2,
  reason: '需要同步第三页的设计稿。',
  timestamp: 1,
};

const snapshot: ProjectContentSnapshot = {
  project_id: 'project-1',
  theme: 'clean',
  appearance: null,
  hashes: {},
  manifest: { title: 'Demo', goal: '', audience: '', language: 'zh-CN', pages: '待明确', requirements: [], prohibitions: [] },
  outline: { sections: [{
    id: 'section-1', title: '', purpose: '', subsections: [],
    slides: [
      { slide_id: 'sli_one', title: '开场' },
      { slide_id: 'sli_middle', title: '过渡' },
      { slide_id: 'sli_two', title: '结论' },
    ],
  }] },
  design: { direction: '', layout_preferences: [], decorations: { page_number: 'bottom-right', section_title: 'top-left', deck_title: 'none', key_message: 'none' } },
  slides_by_id: {},
};

afterEach(() => {
  vi.restoreAllMocks();
  act(() => {
    useRunStore.setState({ sessions: {} });
    useProjectStore.setState({ activeProjectId: null, contentByProjectId: {} });
  });
});

describe('ScopeExpansionCard', () => {
  it('submits an all-page adjustment with the pending interaction identity', async () => {
    const submit = vi.spyOn(runsApi, 'submitScopeExpansion').mockResolvedValue(undefined);
    useProjectStore.setState({ activeProjectId: 'project-1', contentByProjectId: { 'project-1': snapshot } });
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
    expect(screen.getByText('第 1 页')).toBeInTheDocument();
    expect(screen.getByText('第 1、3 页')).toBeInTheDocument();
    expect(screen.getByText('需要同步第三页的设计稿。')).toBeInTheDocument();
    expect(screen.queryByText('结论')).not.toBeInTheDocument();
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '允许全部页' }));
    });

    await waitFor(() => {
      expect(submit).toHaveBeenCalledWith('r1', {
        interaction_id: 'se1',
        call_id: 'c1',
        base_revision: 1,
        decision: 'adjust',
        adjusted_scope: { selection: { kind: 'all_pages' } },
      });
    });
  });

  it('expands the approved result to show the old and new page sets', () => {
    useProjectStore.setState({ activeProjectId: 'project-1', contentByProjectId: { 'project-1': snapshot } });
    render(<ScopeExpansionCard item={{ ...item, answer: { decision: 'approve', appliedScope: item.proposedScope } }} />);

    const toggle = screen.getByRole('button', { name: '已批准扩大修改范围' });
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    fireEvent.click(toggle);
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByText('原先：').nextElementSibling).toHaveTextContent('第 1 页');
    expect(screen.getByText('现在：').nextElementSibling).toHaveTextContent('第 1、3 页');
  });

  it('shows all pages as the new scope after an all-page adjustment', () => {
    useProjectStore.setState({ activeProjectId: 'project-1', contentByProjectId: { 'project-1': snapshot } });
    render(<ScopeExpansionCard item={{ ...item, answer: { decision: 'adjust' } }} />);

    fireEvent.click(screen.getByRole('button', { name: '已允许修改全部页' }));
    expect(screen.getByText('现在：').nextElementSibling).toHaveTextContent('全部页');
  });
});
