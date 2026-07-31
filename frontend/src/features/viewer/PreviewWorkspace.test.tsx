import { render, waitFor, act, screen } from '@testing-library/react';
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
      previewMode: 'main',
      globalView: 'html',
      viewByPage: {},
    });
  });

  it('posts the slide deck once and uses goto for later page switches', async () => {
    render(<PreviewWorkspace />);

    await waitFor(() => {
      expect(postMessage).toHaveBeenCalledWith(
        {
          type: 'update',
          slides: ['/api/v1/slides/s1/render', '/api/v1/slides/s2/render'],
          index: 0,
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

describe('PreviewWorkspace dual view (globalView)', () => {
  beforeEach(() => {
    postMessage.mockReset();
    Object.defineProperty(window.HTMLIFrameElement.prototype, 'contentWindow', {
      configurable: true,
      get() {
        return { postMessage } as unknown as Window;
      }
    });
    useDeckStore.setState({ currentPage: 0, previewMode: 'main', globalView: 'html', viewByPage: {} });
  });

  it('renders OutlineCard (not iframe) when current page has no html', () => {
    useProjectStore.setState({
      projects: [],
      activeProjectId: 'p1',
      slidesByProjectId: {
        p1: [
          { id: 's1', project_id: 'p1', idx: 0, layout: 'bullets', title: '封面标题', html_path: '', json_path: '/slides/p1/s1.json', current_version: 0, order: 10, outline_dirty: false,
            content: { layout: 'bullets', title: '封面标题', bullets: ['要点一'] } },
        ]
      },
      loadingProjects: false
    });
    render(<PreviewWorkspace />);
    // idle session → editable OutlineCard：title 为输入框、bullets 为文本域，均非 iframe。
    expect(screen.getByDisplayValue('封面标题')).toBeInTheDocument();
    expect((screen.getByLabelText('slide-bullets') as HTMLTextAreaElement).value).toContain('要点一');
    expect(document.querySelector('iframe')).toBeNull();
  });

  it('HTML segment button is NOT disabled even when current page has no html (globalView 全局)', () => {
    useProjectStore.setState({
      projects: [],
      activeProjectId: 'p1',
      slidesByProjectId: {
        p1: [
          { id: 's1', project_id: 'p1', idx: 0, layout: 'bullets', title: '封面标题', html_path: '', json_path: '/slides/p1/s1.json', current_version: 0, order: 10, outline_dirty: false,
            content: { layout: 'bullets', title: '封面标题', bullets: ['要点一'] } },
        ]
      },
      loadingProjects: false
    });
    render(<PreviewWorkspace />);
    const htmlBtn = screen.getByRole('button', { name: 'HTML' });
    expect(htmlBtn).not.toBeDisabled();
  });

  it('renders iframe by default when current page has html', () => {
    useProjectStore.setState({
      projects: [],
      activeProjectId: 'p1',
      slidesByProjectId: {
        p1: [
          { id: 's1', project_id: 'p1', idx: 0, layout: 'title', title: 'Slide 1', html_path: '/slides/p1/s1.html', json_path: '/slides/p1/s1.json', current_version: 1, order: 10, outline_dirty: false },
        ]
      },
      loadingProjects: false
    });
    render(<PreviewWorkspace />);
    expect(document.querySelector('iframe')).not.toBeNull();
  });

  it('globalView=outline forces OutlineCard even when hasHtml=true', async () => {
    useProjectStore.setState({
      projects: [],
      activeProjectId: 'p1',
      slidesByProjectId: {
        p1: [
          { id: 's1', project_id: 'p1', idx: 0, layout: 'title', title: 'S1', html_path: '/slides/p1/s1.html', json_path: '/slides/p1/s1.json', current_version: 1, order: 10, outline_dirty: false,
            content: { layout: 'title', title: 'S1' } },
        ]
      },
      loadingProjects: false,
    });
    useDeckStore.setState({ globalView: 'outline', currentPage: 0, previewMode: 'main', viewByPage: {} });
    render(<PreviewWorkspace />);
    // 全局 outline：主区应显示 OutlineCard 而非 iframe。
    expect(document.querySelector('iframe')).toBeNull();
  });

  it('grid mode shows amber 暂无 HTML badge for slides without html', async () => {
    useProjectStore.setState({
      projects: [],
      activeProjectId: 'p1',
      slidesByProjectId: {
        p1: [
          { id: 's1', project_id: 'p1', idx: 0, layout: 'title', title: 'S1', html_path: '/slides/p1/s1.html', json_path: '', current_version: 1, order: 10, outline_dirty: false },
          { id: 's2', project_id: 'p1', idx: 1, layout: 'bullets', title: '', html_path: '', json_path: '', current_version: 0, order: 20, outline_dirty: false },
        ]
      },
      loadingProjects: false,
    });
    useDeckStore.setState({ previewMode: 'overview', globalView: 'html', currentPage: 0, viewByPage: {} });
    render(<PreviewWorkspace />);
    // 网格模式下 s2 无 html_path，应有徽标。
    expect(screen.getByText('暂无 HTML')).toBeInTheDocument();
  });
});
