import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { PPTMutation, ProjectContentSnapshot } from '../../api/types';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { useUIStore } from '../../stores/uiStore';
import { DeckNavigator } from './DeckNavigator';

vi.mock('../viewer/IsolatedSlidePreview', () => ({
  IsolatedSlidePreview: () => <div data-testid="slide-thumbnail" />,
}));

function snapshot(): ProjectContentSnapshot {
  return {
    project_title: 'Test', project_id: 'pro_1',
    theme: 'default',
    appearance: null,
    hashes: { outline: "outline-hash" },
    manifest: {
      title: '演示文稿',
      goal: '',
      audience: '',
      language: 'zh-CN',
      slide_count: '待明确',
      requirements: [],
      prohibitions: [],
    },
    outline: {
      sections: [
        {
          id: 'sec_direct',
          title: '开场',
          slides: [{ id: 'slide_1', title: '问题与目标' }],
          subsections: [],
        },
        {
          id: 'sec_grouped',
          title: '方法',
          slides: [],
          subsections: [
            {
              id: 'sec_basics',
              title: '基本概念',
              slides: [{ id: 'slide_2', title: '核心定义' }],
            },
            {
              id: 'sec_examples',
              title: '案例',
              slides: [{ id: 'slide_3', title: '实际案例' }],
            },
          ],
        },
      ],
    },
    design: { requirements: [], prohibitions: [] },
    decorations: { slide_pagination: 'bottom-right', deck_title: 'none', section_title: 'none', logo: 'none' }, logo: null,
    slides_by_id: {
      slide_1: { spec_state: 'pending', spec: null, html_state: 'missing', html_hash: '' },
      slide_2: { spec_state: 'pending', spec: null, html_state: 'missing', html_hash: '' },
      slide_3: { spec_state: 'pending', spec: null, html_state: 'missing', html_hash: '' },
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
    useUIStore.setState({ leftPanelHidden: false });
  });

  it('adds a page directly to an ungrouped section from its overflow menu', async () => {
    const user = userEvent.setup();
    render(<DeckNavigator />);

    await user.click(screen.getByRole('button', { name: '开场操作' }));
    await user.click(await screen.findByText('新增页面'));

    expect(mutateProject).toHaveBeenCalledWith('pro_1', expect.objectContaining({
      op: 'outline.insert',
      expected_hash: 'outline-hash',
      position: { parent_id: 'sec_direct' },
      node: expect.objectContaining({ kind: 'slide', title: '新页面' }),
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
        position: { parent_id: 'sec_basics' },
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

  it('locks structure controls without flashing a warning banner during mutations', () => {
    useProjectStore.setState({ mutationPendingByProjectId: { pro_1: true } });
    render(<DeckNavigator />);

    expect(screen.getByRole('button', { name: '开场操作' })).toBeDisabled();
    expect(screen.queryByText('正在更新目录')).not.toBeInTheDocument();
  });
});
