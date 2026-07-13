import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { DeckNavigator } from './DeckNavigator';
import { useProjectStore } from '../../stores/projectStore';
import { useDeckStore } from '../../stores/deckStore';
import { slidesApi } from '../../api/slides';

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
  });

  it('shows real slide titles instead of generic labels', () => {
    render(<DeckNavigator />);
    expect(screen.getByText('市场分析')).toBeInTheDocument();
    expect(screen.getByText('增长趋势')).toBeInTheDocument();
    expect(screen.queryByText('Slide 1')).not.toBeInTheDocument();
  });

  it('marks dirty pages with a badge', () => {
    render(<DeckNavigator />);
    const badge = screen.getByTitle('大纲已改，待更新');
    expect(badge).toBeInTheDocument();
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

  it('delete page asks confirm then calls slidesApi.remove', async () => {
    const removeSpy = vi.spyOn(slidesApi, 'remove').mockResolvedValue(undefined as never);
    vi.spyOn(useProjectStore.getState(), 'loadProjectSlides').mockResolvedValue();
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    render(<DeckNavigator />);
    fireEvent.click(screen.getAllByRole('button', { name: /删除本页/ })[0]);
    expect(window.confirm).toHaveBeenCalled();
    await waitFor(() => {
      expect(removeSpy).toHaveBeenCalledWith('s1');
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
