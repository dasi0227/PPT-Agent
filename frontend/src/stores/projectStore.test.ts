import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { ProjectContentSnapshot } from '../api/types';

const { getContent, mutate } = vi.hoisted(() => ({ getContent: vi.fn(), mutate: vi.fn() }));
vi.mock('../api/projects', () => ({ projectsApi: { getContent, mutate } }));
vi.mock('./threadStore', () => ({ useThreadStore: { getState: () => ({ loadThreads: vi.fn(), dropProject: vi.fn() }) } }));

import { useProjectStore } from './projectStore';

function snapshot(revision: number): ProjectContentSnapshot {
  return {
    deck: { version: '4.0', revision, project_id: 'pro_1', title: 'Deck', goal: '', audience: '', language: 'zh-CN', requirements: [], prohibitions: [], canvas: { aspect_ratio: '16:9' }, numbering: { enabled: true, hidden_roles: [], format: 'number' }, created_at: 1, updated_at: 1 },
    outline: { version: '4.0', revision, project_id: 'pro_1', sections: [], created_at: 1, updated_at: 1 },
    design: { version: '4.0', revision, project_id: 'pro_1', theme: 'default', direction: '', density: 'medium', chrome: [], created_at: 1, updated_at: 1 },
    slides_by_id: {},
  };
}

describe('projectStore canonical content snapshots', () => {
  beforeEach(() => {
    getContent.mockReset(); mutate.mockReset();
    useProjectStore.setState({ contentByProjectId: {}, contentLoadingByProjectId: {}, contentErrorByProjectId: {}, mutationPendingByProjectId: {}, projectError: null });
  });

  it('publishes deck, outline, design and pages in one store update', async () => {
    getContent.mockResolvedValue(snapshot(1));
    await useProjectStore.getState().loadProjectContent('pro_1');
    expect(useProjectStore.getState().contentByProjectId.pro_1).toEqual(snapshot(1));
  });

  it('applies a mutation response directly without a duplicate refresh', async () => {
    mutate.mockResolvedValue({ mutation: { operation: 'outline.insert' }, content: snapshot(2) });
    await useProjectStore.getState().mutateProject('pro_1', { op: 'outline.insert', node: { kind: 'section', client_ref: 'section', title: 'Section', purpose: 'Purpose', slides: [], subsections: [] }, position: {} });
    expect(mutate).toHaveBeenCalledTimes(1);
    expect(getContent).not.toHaveBeenCalled();
    expect(useProjectStore.getState().contentByProjectId.pro_1.outline.revision).toBe(2);
    expect(useProjectStore.getState().mutationPendingByProjectId.pro_1).toBe(false);
  });
});
