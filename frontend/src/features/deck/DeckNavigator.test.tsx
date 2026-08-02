import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { DeckNavigator } from './DeckNavigator';
import { useProjectStore } from '../../stores/projectStore';
import { useDeckStore } from '../../stores/deckStore';
import { slidesApi } from '../../api/slides';
import { useSpecStore } from '../../stores/specStore';

describe('DeckNavigator', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    useDeckStore.setState({ currentPage: 0 });
    useProjectStore.setState({
      projects: [{ id: 'p1', title: '演示项目', work_dir: '', theme: 'swiss', status: 'draft', design_path: '', created_at: 0, updated_at: 0 }],
      activeProjectId: 'p1',
      slidesByProjectId: {
        p1: [
          { id: 's1', project_id: 'p1', position: 0, layout: 'cover', title: '市场分析', html_path: '/a.html', spec_path: '/a.json', current_version: 1 },
          { id: 's2', project_id: 'p1', position: 1, layout: 'content', title: '增长趋势', html_path: '/b.html', spec_path: '/b.json', current_version: 1 },
        ]
      },
      loadingProjects: false
    });
    useSpecStore.setState({ byProjectId: { p1: {
      outline: { schema_version: '3.0', revision: 1, project_id: 'p1', title: '演示项目', goal: '', audience: '', language: 'zh-CN', core_thesis: '核心命题', narrative_arc: '', sections: [{ id: 'sec', number: '01', title: '市场', subsections: [{ id: 'sub', number: '1.1', title: '趋势' }] }], slide_order: ['s1', 's2'], created_at: 1, updated_at: 1 },
      slide_specs: {
        s1: { schema_version: '3.0', revision: 1, project_id: 'p1', slide_id: 's1', source_outline_revision: 1, section_id: 'sec', role: 'cover', title: '市场分析', key_message: '市场在扩大', content: { summary: '摘要', points: [] }, visual_intent: { archetype: 'cover', description: '封面', asset_queries: [] }, speaker_notes: '', created_at: 1, updated_at: 1 },
        s2: { schema_version: '3.0', revision: 2, project_id: 'p1', slide_id: 's2', source_outline_revision: 1, section_id: 'sec', subsection_id: 'sub', role: 'evidence', title: '增长趋势', key_message: '增长持续', content: { summary: '摘要', points: [] }, visual_intent: { archetype: 'chart', description: '趋势图', asset_queries: [] }, speaker_notes: '', created_at: 1, updated_at: 2 },
      },
      design: { schema_version: '3.0', revision: 1, project_id: 'p1', canvas: {}, palette: [], typography: {}, spacing: {}, radius: {}, shadows: {}, layout_system: {}, signature: '', motion: {}, created_at: 1, updated_at: 1 },
      materialization: {
        s1: { state: 'fresh', revisions: { slide_html: 1, source_outline: 1, source_spec: 1, source_design: 1 } },
        s2: { state: 'spec_stale', revisions: { slide_html: 1, source_outline: 1, source_spec: 1, source_design: 1 } },
      },
    } } });
  });

  it('shows real slide titles instead of generic labels', () => {
    render(<DeckNavigator />);
    expect(screen.getByText('市场分析')).toBeInTheDocument();
    expect(screen.getByText('增长趋势')).toBeInTheDocument();
    expect(screen.queryByText('Slide 1')).not.toBeInTheDocument();
    expect(screen.queryByText('核心命题')).not.toBeInTheDocument();
  });

  it('marks stale pages with a materialization badge', () => {
    render(<DeckNavigator />);
    expect(screen.getByText('设计稿有更新')).toBeInTheDocument();
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
    expect(screen.getByRole('heading', { name: '删除页面' })).toBeInTheDocument();
    expect(screen.getByText('「市场分析」')).toBeInTheDocument();
    expect(screen.getByText('不可撤销')).toBeInTheDocument();

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
