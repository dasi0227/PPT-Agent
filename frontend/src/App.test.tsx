import { render, screen, act } from '@testing-library/react';
import { describe, it, expect, beforeEach } from 'vitest';
import { App } from './App';
import { useProjectStore } from './stores/projectStore';
import { useDeckStore } from './stores/deckStore';
import { useRunStore } from './stores/runStore';
import { useThreadStore } from './stores/threadStore';

// Mock ResizeObserver
globalThis.ResizeObserver = class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

describe('App Level Interactions', () => {
  beforeEach(() => {
    window.history.pushState({}, '', '/projects/p1');
    useProjectStore.setState({
        projects: [
          { id: 'p1', title: 'Project 1', work_dir: '', theme: 'default', status: 'draft', design_path: '', created_at: 0, updated_at: 0 },
          { id: 'p2', title: 'Project 2', work_dir: '', theme: 'default', status: 'draft', design_path: '', created_at: 0, updated_at: 0 }
        ],
        activeProjectId: 'p1',
        slidesByProjectId: {
          'p1': [
            { id: 's1', project_id: 'p1', position: 0, layout: 'title', title: 'Slide 1', html_path: '/slides/p1/s1.html', spec_path: '/slides/p1/s1.json', current_version: 1 },
            { id: 's2', project_id: 'p1', position: 1, layout: 'content', title: 'Slide 2', html_path: '/slides/p1/s2.html', spec_path: '/slides/p1/s2.json', current_version: 1 }
          ]
        },
      loadingProjects: false
    });

    useThreadStore.setState({
      threadsByProjectId: { p1: [{ id: 't1', project_id: 'p1', title: 'Thread', status: 'active', history_path: '', created_at: 0, updated_at: 0 }] },
      openThreadIdsByProjectId: { p1: ['t1'] },
      activeThreadIdByProjectId: { p1: 't1' },
    });

    useDeckStore.setState({
      currentPage: 0,
      previewMode: 'main'
    });

    useRunStore.setState({ sessions: {} });
  });

  it('switches projects and active slide updates', async () => {
    await act(async () => {
      render(<App />);
    });
    
    // Check initial state
    expect(screen.getAllByText('Project 1').length).toBeGreaterThan(0);
    
    // Switch to Project 2
    await act(async () => {
      useProjectStore.getState().selectProject('p2');
    });
    
    expect(useProjectStore.getState().activeProjectId).toBe('p2');
  });

  it('switches current page', async () => {
    await act(async () => {
      useProjectStore.setState({
        projects: [{ id: 'p1', title: 'Project 1', work_dir: '', theme: 'default', status: 'draft', design_path: '', created_at: 0, updated_at: 0 }],
        activeProjectId: 'p1',
        slidesByProjectId: {
          'p1': [
            { id: 's1', project_id: 'p1', position: 0, layout: 'title', title: 'Slide 1', html_path: '/slides/p1/s1.html', spec_path: '/slides/p1/s1.json', current_version: 1 },
            { id: 's2', project_id: 'p1', position: 1, layout: 'content', title: 'Slide 2', html_path: '/slides/p1/s2.html', spec_path: '/slides/p1/s2.json', current_version: 1 }
          ]
        }
      });
    });

    await act(async () => {
      render(<App />);
    });
    
    const slide2Btn = screen.getByRole('button', { name: '下一页' });
    await act(async () => {
      slide2Btn.click();
    });
    
    expect(useDeckStore.getState().currentPage).toBe(1);
  });

  it('restores workspace view state from the project URL after refresh', async () => {
    window.history.pushState({}, '', '/projects/p1?slide=s2&view=outline&mode=overview');
    useDeckStore.setState({
      currentPage: 0,
      globalView: 'html',
      previewMode: 'main',
    });

    await act(async () => {
      render(<App />);
    });

    await act(async () => {
      await Promise.resolve();
    });

    expect(useDeckStore.getState().currentPage).toBe(1);
    expect(useDeckStore.getState().globalView).toBe('outline');
    expect(useDeckStore.getState().previewMode).toBe('overview');
  });

  it('keeps the project URL in sync when the current page changes', async () => {
    window.history.pushState({}, '', '/projects/p1');

    await act(async () => {
      render(<App />);
    });

    await act(async () => {
      useDeckStore.getState().setCurrentPage(1);
    });

    expect(window.location.pathname).toBe('/projects/p1');
    expect(new URLSearchParams(window.location.search).get('slide')).toBe('s2');
  });

  it('updates the URL when switching away from a slide restored from URL', async () => {
    window.history.pushState({}, '', '/projects/p1?slide=s1');

    await act(async () => {
      render(<App />);
    });

    await act(async () => {
      useDeckStore.getState().setCurrentPage(1);
    });

    expect(new URLSearchParams(window.location.search).get('slide')).toBe('s2');
  });

  it('uses the root route as the home state even when a project was previously active', async () => {
    window.history.pushState({}, '', '/');

    await act(async () => {
      render(<App />);
    });

    expect(useProjectStore.getState().activeProjectId).toBeNull();
    expect(screen.getByRole('heading', { name: 'Dasi PPT Agent' })).toBeInTheDocument();
    expect(screen.queryByText('Project 1')).not.toBeInTheDocument();
  });
  
  it('shows run events in agent panel', async () => {
    await act(async () => {
      render(<App />);
    });
    
    await act(async () => {
      useRunStore.setState({
        sessions: {
          t1: {
            activeRunId: 'r1',
            status: 'running',
            target: { artifact: 'presentation', level: 'slide' },
            interaction: { intent: 'execute' },
            timelineItems: [
              { id: '1', type: 'reasoning', messageId: 'm1', text: 'Hello from Agent', timestamp: Date.now() }
            ],
            pendingQuestion: null,
            progress: null,
            eventSourceClose: null,
            plan: null,
          }
        }
      });
    });
    
    expect(screen.getByText('Hello from Agent')).toBeInTheDocument();
  });
});
