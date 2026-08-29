import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { PPTMutation, ProjectContentSnapshot } from '../../api/types';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { DeckNavigator } from './DeckNavigator';

vi.mock('../viewer/IsolatedSlidePreview', () => ({
  IsolatedSlidePreview: () => <div data-testid="slide-thumbnail" />,
}));

function snapshot(): ProjectContentSnapshot {
  return {
    deck: {
      version: '4.0',
      revision: 1,
      project_id: 'pro_1',
      title: '演示文稿',
      goal: '',
      audience: '',
      language: 'zh-CN',
      requirements: [],
      prohibitions: [],
      canvas: { aspect_ratio: '16:9' },
      numbering: { enabled: true, hidden_roles: [], format: 'number' },
      created_at: 1,
      updated_at: 1,
    },
    outline: {
      version: '4.0',
      revision: 4,
      project_id: 'pro_1',
      sections: [
        {
          id: 'sec_direct',
          title: '开场',
          purpose: '',
          slides: [{ slide_id: 'slide_1', label: '问题与目标', role: 'cover' }],
          subsections: [],
        },
        {
          id: 'sec_grouped',
          title: '方法',
          purpose: '',
          slides: [],
          subsections: [
            {
              id: 'sub_basics',
              title: '基本概念',
              slides: [{ slide_id: 'slide_2', label: '核心定义', role: 'content' }],
            },
            {
              id: 'sub_examples',
              title: '案例',
              slides: [{ slide_id: 'slide_3', label: '实际案例', role: 'content' }],
            },
          ],
        },
      ],
      created_at: 1,
      updated_at: 1,
    },
    design: {
      version: '4.0',
      revision: 1,
      project_id: 'pro_1',
      theme: 'default',
      direction: '',
      density: 'medium',
      chrome: [],
      created_at: 1,
      updated_at: 1,
    },
    slides_by_id: {
      slide_1: { spec_state: 'pending', spec: null, html_state: 'not_materialized', html_revision: 0, materialization: null },
      slide_2: { spec_state: 'pending', spec: null, html_state: 'not_materialized', html_revision: 0, materialization: null },
      slide_3: { spec_state: 'pending', spec: null, html_state: 'not_materialized', html_revision: 0, materialization: null },
    },
  };
}

describe('DeckNavigator', () => {
  const mutateProject = vi.fn<(projectId: string, mutation: PPTMutation) => Promise<ProjectContentSnapshot>>();

  beforeEach(() => {
    mutateProject.mockReset();
    mutateProject.mockResolvedValue(snapshot());
    useProjectStore.setState({
      activeProjectId: 'pro_1',
      contentByProjectId: { pro_1: snapshot() },
      contentLoadingByProjectId: { pro_1: false },
      contentErrorByProjectId: { pro_1: undefined },
      mutationPendingByProjectId: { pro_1: false },
      mutateProject,
    });
    useDeckStore.setState({ currentSlideId: 'slide_1', globalView: 'outline' });
  });

  it('shows page titles in design view and removes the footer create action', () => {
    render(<DeckNavigator />);

    expect(screen.getByText('问题与目标')).toBeInTheDocument();
    expect(screen.getByText('核心定义')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '新增页面' })).not.toBeInTheDocument();
  });

  it('shows rendered-page placeholders instead of titles in slide view', () => {
    useDeckStore.setState({ globalView: 'html' });
    render(<DeckNavigator />);

    expect(screen.queryByText('问题与目标')).not.toBeInTheDocument();
    expect(screen.getAllByText('未生成')).toHaveLength(3);
  });

  it('adds a page directly to an ungrouped section from its overflow menu', async () => {
    const user = userEvent.setup();
    render(<DeckNavigator />);

    await user.click(screen.getByRole('button', { name: '开场操作' }));
    await user.click(await screen.findByText('新增页面'));

    expect(mutateProject).toHaveBeenCalledWith('pro_1', expect.objectContaining({
      op: 'outline.insert',
      expected_revision: 4,
      position: { parent_id: 'sec_direct' },
      node: expect.objectContaining({ kind: 'slide', label: '新页面' }),
    }));
  });

  it('requires a subsection target when adding a page to a grouped section', async () => {
    const user = userEvent.setup();
    render(<DeckNavigator />);

    await user.click(screen.getByRole('button', { name: '方法操作' }));
    await user.hover(await screen.findByText('新增页面'));
    const target = await screen.findByRole('menuitem', { name: '基本概念' });
    target.focus();
    await user.keyboard('{Enter}');

    await waitFor(() => {
      expect(mutateProject).toHaveBeenCalledWith('pro_1', expect.objectContaining({
        op: 'outline.insert',
        position: { parent_id: 'sub_basics' },
        node: expect.objectContaining({ kind: 'slide' }),
      }));
    });
  });

  it('moves direct pages into the first newly created subsection', async () => {
    const user = userEvent.setup();
    render(<DeckNavigator />);

    await user.click(screen.getByRole('button', { name: '开场操作' }));
    await user.click(await screen.findByText('新增子节'));
    await user.click(screen.getByRole('button', { name: '确认' }));

    expect(mutateProject).toHaveBeenCalledWith('pro_1', expect.objectContaining({
      op: 'outline.insert',
      position: { parent_id: 'sec_direct' },
      direct_slides_policy: 'move_into_new_subsection',
      node: expect.objectContaining({ kind: 'subsection', title: '新子节' }),
    }));
  });

  it('adds an empty section at the end from the header action', async () => {
    const user = userEvent.setup();
    render(<DeckNavigator />);

    await user.click(screen.getByRole('button', { name: '新增章节' }));

    expect(mutateProject).toHaveBeenCalledWith('pro_1', expect.objectContaining({
      op: 'outline.insert',
      position: {},
      node: expect.objectContaining({ kind: 'section', slides: [], subsections: [] }),
    }));
  });
});
