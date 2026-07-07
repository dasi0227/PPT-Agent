import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';
import { DeckNavigator } from './DeckNavigator';
import { useProjectStore } from '../../stores/projectStore';
import { useDeckStore } from '../../stores/deckStore';

describe('DeckNavigator', () => {
  beforeEach(() => {
    useDeckStore.setState({ currentPage: 0 });
    useProjectStore.setState({
      projects: [{ id: 'p1', title: '演示项目', theme: 'swiss', status: 'draft', created_at: 0, updated_at: 0 }],
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
});
