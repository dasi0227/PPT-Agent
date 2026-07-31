import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { DeckNavigator } from './DeckNavigator';
import { useProjectStore } from '../../stores/projectStore';
import { useDeckStore } from '../../stores/deckStore';
import { slidesApi } from '../../api/slides';
import { useBlueprintStore } from '../../stores/blueprintStore';

describe('DeckNavigator', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    useDeckStore.setState({ currentPage: 0 });
    useProjectStore.setState({
      projects: [{ id: 'p1', title: '演示项目', work_dir: '', theme: 'swiss', status: 'draft', design_path: '', created_at: 0, updated_at: 0 }],
      activeProjectId: 'p1',
      slidesByProjectId: {
        p1: [
          { id: 's1', project_id: 'p1', idx: 0, layout: 'cover', title: '市场分析', html_path: '/a.html', json_path: '/a.json', current_version: 1, order: 10, outline_dirty: false },
          { id: 's2', project_id: 'p1', idx: 1, layout: 'content', title: '增长趋势', html_path: '/b.html', json_path: '/b.json', current_version: 1, order: 20, outline_dirty: true },
        ]
      },
      loadingProjects: false
    });
    useBlueprintStore.setState({ byProjectId: { p1: {
      deck: { schema_version: '2.0', revision: 1, project_id: 'p1', title: '演示项目', goal: '', audience: '', language: 'zh-CN', core_thesis: '核心命题', narrative_arc: '', sections: [{ id: 'sec', number: '01', title: '市场', subsections: [{ id: 'sub', number: '1.1', title: '趋势' }] }], slide_order: ['s1', 's2'], created_at: 1, updated_at: 1 },
      slides: {
        s1: { schema_version: '2.0', revision: 1, slide_id: 's1', section_id: 'sec', role: 'cover', title: '市场分析', key_message: '市场在扩大', content: { summary: '摘要', points: [] }, visual_intent: { archetype: 'cover', description: '封面', asset_queries: [] }, speaker_notes: '', created_at: 1, updated_at: 1 },
        s2: { schema_version: '2.0', revision: 2, slide_id: 's2', section_id: 'sec', subsection_id: 'sub', role: 'evidence', title: '增长趋势', key_message: '增长持续', content: { summary: '摘要', points: [] }, visual_intent: { archetype: 'chart', description: '趋势图', asset_queries: [] }, speaker_notes: '', created_at: 1, updated_at: 2 },
      },
      design_spec: { schema_version: '2.0', revision: 1, canvas: {}, palette: [], typography: {}, spacing: {}, radius: {}, shadows: {}, layout_system: {}, signature: '', motion: {} },
      materialization: {
        s1: { state: 'fresh', revisions: { presentation: 1, source_deck: 1, source_blueprint: 1, source_design: 1 } },
        s2: { state: 'blueprint_stale', revisions: { presentation: 1, source_deck: 1, source_blueprint: 1, source_design: 1 } },
      },
    } } });
  });

  it('shows real slide titles instead of generic labels', () => {
    render(<DeckNavigator />);
    expect(screen.getByText('市场分析')).toBeInTheDocument();
    expect(screen.getByText('增长趋势')).toBeInTheDocument();
    expect(screen.queryByText('Slide 1')).not.toBeInTheDocument();
  });

  it('marks stale pages with a materialization badge', () => {
    render(<DeckNavigator />);
    expect(screen.getByText('蓝图有更新')).toBeInTheDocument();
  });

  it('renders section and subsection directory hierarchy', () => {
    render(<DeckNavigator />);
    expect(screen.getByText('01 市场')).toBeInTheDocument();
    expect(screen.getByText('1.1 趋势')).toBeInTheDocument();
  });

  it('add page button calls slidesApi.add with last slide as anchor', async () => {
    const addSpy = vi.spyOn(slidesApi, 'add').mockResolvedValue({} as never);
    vi.spyOn(useProjectStore.getState(), 'loadProjectSlides').mockResolvedValue();
    render(<DeckNavigator />);
    fireEvent.click(screen.getByRole('button', { name: /加页/ }));
    await waitFor(() => {
      expect(addSpy).toHaveBeenCalledWith('p1', { after_slide_id: 's2' });
    });
  });

  it('delete page opens confirm modal then calls slidesApi.remove', async () => {
    const removeSpy = vi.spyOn(slidesApi, 'remove').mockResolvedValue(undefined as never);
    const loadProjectSlidesSpy = vi.spyOn(useProjectStore.getState(), 'loadProjectSlides').mockResolvedValue();

    render(<DeckNavigator />);
    fireEvent.click(screen.getAllByRole('button', { name: /删除本页/ })[0]);

    // modal should be visible
    expect(screen.getByText('确认删除「市场分析」这一页吗？此操作不可撤销。')).toBeInTheDocument();

    // click confirm
    fireEvent.click(screen.getByRole('button', { name: '删除' }));

    await waitFor(() => {
      expect(removeSpy).toHaveBeenCalledWith('s1');
      expect(loadProjectSlidesSpy).toHaveBeenCalledWith('p1');
    });
  });

  it('delete page does nothing when confirm is canceled', () => {
    const removeSpy = vi.spyOn(slidesApi, 'remove').mockResolvedValue(undefined as never);
    vi.spyOn(window, 'confirm').mockReturnValue(false);
    render(<DeckNavigator />);
    fireEvent.click(screen.getAllByRole('button', { name: /删除本页/ })[0]);
    expect(removeSpy).not.toHaveBeenCalled();
  });
});
