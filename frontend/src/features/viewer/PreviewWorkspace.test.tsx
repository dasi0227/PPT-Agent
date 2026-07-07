import { render, waitFor, act } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { PreviewWorkspace } from './PreviewWorkspace';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';

const postMessage = vi.fn();

describe('PreviewWorkspace', () => {
  beforeEach(() => {
    postMessage.mockReset();

    Object.defineProperty(window.HTMLIFrameElement.prototype, 'contentWindow', {
      configurable: true,
      get() {
        return { postMessage } as unknown as Window;
      }
    });

    useProjectStore.setState({
      projects: [],
      activeProjectId: 'p1',
      slidesByProjectId: {
        p1: [
          { id: 's1', project_id: 'p1', idx: 0, layout: 'title', title: 'Slide 1', html_path: '/slides/p1/s1.html', json_path: '/slides/p1/s1.json', current_version: 1, order: 10, outline_dirty: false },
          { id: 's2', project_id: 'p1', idx: 1, layout: 'content', title: 'Slide 2', html_path: '/slides/p1/s2.html', json_path: '/slides/p1/s2.json', current_version: 1, order: 20, outline_dirty: false }
        ]
      },
      loadingProjects: false
    });

    useDeckStore.setState({
      currentPage: 0,
      previewMode: 'main'
    });
  });

  it('posts the slide deck once and uses goto for later page switches', async () => {
    render(<PreviewWorkspace />);

    await waitFor(() => {
      expect(postMessage).toHaveBeenCalledWith(
        {
          type: 'update',
          slides: ['/slides/p1/s1.html', '/slides/p1/s2.html'],
          index: 0
        },
        '*'
      );
    });

    postMessage.mockClear();

    await act(async () => {
      useDeckStore.getState().setCurrentPage(1);
    });

    await waitFor(() => {
      expect(postMessage).toHaveBeenCalledTimes(1);
    });

    expect(postMessage).toHaveBeenCalledWith({ type: 'goto', index: 1 }, '*');
  });
});
