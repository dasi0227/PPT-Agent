import { render, screen, act } from '@testing-library/react';
import { describe, it, expect, beforeEach } from 'vitest';
import { App } from './App';
import { useProjectStore } from './stores/projectStore';
import { useDeckStore } from './stores/deckStore';
import { useRunStore } from './stores/runStore';

// Mock ResizeObserver
globalThis.ResizeObserver = class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

describe('App Level Interactions', () => {
  beforeEach(() => {
    useProjectStore.setState({
      projects: [
          { id: 'p1', topic: 'Project 1', brief: '', theme: 'default', language: 'zh', slide_count: 2, created_at: '', updated_at: '' },
          { id: 'p2', topic: 'Project 2', brief: '', theme: 'default', language: 'zh', slide_count: 1, created_at: '', updated_at: '' }
        ],
        activeProjectId: 'p1',
        slidesByProjectId: {
          'p1': [
            { id: 's1', project_id: 'p1', idx: 0, html_path: '/slides/p1/s1.html', notes: '', current_version: 1, created_at: '', updated_at: '' },
            { id: 's2', project_id: 'p1', idx: 1, html_path: '/slides/p1/s2.html', notes: '', current_version: 1, created_at: '', updated_at: '' }
          ]
        },
      threadsByProjectId: {},
      loadingProjects: false
    });
    
    useDeckStore.setState({
      currentPage: 0,
      previewMode: 'main'
    });

    useRunStore.setState({
      activeRunId: null,
      status: 'idle',
      timelineItems: [],
      pendingInput: null
    });
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
        projects: [{ id: 'p1', topic: 'Project 1', brief: '', theme: 'default', language: 'zh', slide_count: 2, created_at: '', updated_at: '' }],
        activeProjectId: 'p1',
        slidesByProjectId: {
          'p1': [
            { id: 's1', project_id: 'p1', idx: 0, html_path: '/slides/p1/s1.html', notes: '', current_version: 1, created_at: '', updated_at: '' },
            { id: 's2', project_id: 'p1', idx: 1, html_path: '/slides/p1/s2.html', notes: '', current_version: 1, created_at: '', updated_at: '' }
          ]
        }
      });
    });

    await act(async () => {
      render(<App />);
    });
    
    const slide2Btn = screen.getByText('Slide 2');
    await act(async () => {
      slide2Btn.click();
    });
    
    expect(useDeckStore.getState().currentPage).toBe(1);
  });
  
  it('shows run events in agent panel', async () => {
    await act(async () => {
      render(<App />);
    });
    
    await act(async () => {
      useRunStore.setState({
        status: 'running',
        timelineItems: [
          { id: '1', type: 'markdown', text: 'Hello from Agent', timestamp: Date.now() }
        ]
      });
    });
    
    expect(screen.getByText('Hello from Agent')).toBeInTheDocument();
    expect(screen.getByText('Thinking...')).toBeInTheDocument(); // because status='running'
  });
});
