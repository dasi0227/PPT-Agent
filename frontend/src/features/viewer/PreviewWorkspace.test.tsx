import { render, waitFor, act, screen, fireEvent } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { PreviewWorkspace } from './PreviewWorkspace';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { clearSlideRenderCache } from './useSlideRenderCache';

const slideSpec = (id: string, title = '封面标题') => ({
  version: '3.0' as const, revision: 1, project_id: 'p1', slide_id: id,
  section_id: 'main', role: 'cover',
  title, key_message: title, elements: [{ type: 'text' as const, intent: '要点一' }],
  layout: 'hero', created_at: 1, updated_at: 1,
});
const setSpecs = () => useProjectStore.setState({
  specByProjectId: { p1: {
    outline: { version: '3.0', revision: 1, project_id: 'pro_aaaaaa', title: 'Deck', goal: '', audience: '', language: 'zh-CN', positioning: '', requirements: [], prohibitions: [], sections: [], slide_order: ['s1', 's2'], created_at: 1, updated_at: 1 },
    slide_specs: { s1: slideSpec('s1'), s2: slideSpec('s2', '第二页') },
    design: { version: '3.0', revision: 1, project_id: 'pro_aaaaaa', theme: 'swiss-modern', direction: 'test direction', density: 'medium', chrome: [], created_at: 1, updated_at: 1 },
    materialization: {
      s1: { state: 'not_materialized', revisions: { slide_html: 0, source_outline: 0, source_spec: 0, source_design: 0 } },
      s2: { state: 'not_materialized', revisions: { slide_html: 0, source_outline: 0, source_spec: 0, source_design: 0 } },
    },
  } },
  contentLoadingByProjectId: {},
  contentErrorByProjectId: {},
});

const postMessage = vi.fn();

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('PreviewWorkspace', () => {
  beforeEach(() => {
    clearSlideRenderCache();
    postMessage.mockReset();
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => ({
      ok: true,
      status: 200,
      text: async () => input.toString().includes('/s1/') ? '<h1>Slide 1</h1>' : '<h1>Slide 2</h1>',
    } as Response)));

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
          { id: 's1', project_id: 'p1', position: 0, layout: 'title', title: 'Slide 1', html_path: '/slides/p1/s1.html', spec_path: '/slides/p1/s1.json', current_version: 1 },
          { id: 's2', project_id: 'p1', position: 1, layout: 'content', title: 'Slide 2', html_path: '/slides/p1/s2.html', spec_path: '/slides/p1/s2.json', current_version: 1 }
        ]
      },
      loadingProjects: false
    });

    useDeckStore.setState({
      currentSlideId: 's1',
      previewMode: 'main',
      globalView: 'html',
    });
    setSpecs();
  });

  it('loads the current slide, prefetches adjacent slides, then switches without rebuilding a single-slide deck', async () => {
    render(<PreviewWorkspace />);

    await waitFor(() => expect(fetch).toHaveBeenCalled());
    expect(vi.mocked(fetch).mock.calls[0]?.[0]).toBe('/api/v1/slides/s1/render');
    await waitFor(() => {
      expect(fetch).toHaveBeenCalledWith('/api/v1/slides/s2/render', expect.any(Object));
    });
    await waitFor(() => {
      expect(postMessage).toHaveBeenCalledWith(
        {
          type: 'updateDeck',
          slides: [
            { id: 's1', html: '<h1>Slide 1</h1>' },
            { id: 's2', html: '<h1>Slide 2</h1>' },
          ],
          index: 0,
        },
        '*'
      );
    });

    postMessage.mockClear();
    await act(async () => {
      useDeckStore.getState().setCurrentSlideId('s2');
    });

    await waitFor(() => {
      expect(postMessage).toHaveBeenCalledWith(
        {
          type: 'gotoSlide',
          index: 1,
        },
        '*'
      );
    });
    expect(postMessage).not.toHaveBeenCalledWith(
      {
        type: 'updateDeck',
        slides: [{ id: 's2', html: '<h1>Slide 2</h1>' }],
        index: 0,
      },
      '*'
    );
  });

  it('keeps the main runtime sandboxed without same-origin privilege', async () => {
    render(<PreviewWorkspace />);
    await waitFor(() => expect(document.querySelector('iframe')).not.toBeNull());
    const iframe = document.querySelector('iframe');
    expect(iframe).toHaveAttribute('sandbox', 'allow-scripts');
    expect(iframe?.getAttribute('sandbox')).not.toContain('allow-same-origin');
  });

  it('uses a new cache key when the presentation revision changes', async () => {
    useProjectStore.setState({
      slidesByProjectId: {
        p1: [
          { id: 's1', project_id: 'p1', position: 0, layout: 'title', title: 'Slide 1', html_path: '/slides/p1/s1.html', spec_path: '', current_version: 1 },
        ],
      },
    });
    let renderedRevision = 1;
    vi.stubGlobal('fetch', vi.fn(async () => ({
      ok: true,
      status: 200,
      text: async () => `<h1>Revision ${renderedRevision}</h1>`,
    } as Response)));
    render(<PreviewWorkspace />);
    await waitFor(() => expect(postMessage).toHaveBeenCalledWith({
      type: 'updateDeck',
      slides: [{ id: 's1', html: '<h1>Revision 1</h1>' }],
      index: 0,
    }, '*'));

    renderedRevision = 2;
    await act(async () => {
      useProjectStore.setState((state) => ({
        slidesByProjectId: {
          ...state.slidesByProjectId,
          p1: [{ ...state.slidesByProjectId.p1[0], current_version: 2 }],
        },
      }));
    });

    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
    await waitFor(() => expect(postMessage).toHaveBeenCalledWith({
      type: 'updateDeck',
      slides: [{ id: 's1', html: '<h1>Revision 2</h1>' }],
      index: 0,
    }, '*'));
  });

  it('loads only overview slides that enter the visible region', async () => {
    const observers: Array<(entries: IntersectionObserverEntry[]) => void> = [];
    vi.stubGlobal('IntersectionObserver', class {
      constructor(callback: (entries: IntersectionObserverEntry[]) => void) {
        observers.push(callback);
      }
      observe() {}
      unobserve() {}
      disconnect() {}
    });
    useDeckStore.setState({ previewMode: 'overview', globalView: 'html', currentSlideId: 's1' });
    render(<PreviewWorkspace />);

    expect(observers).toHaveLength(2);
    expect(fetch).not.toHaveBeenCalled();
    await act(async () => {
      observers[0]([{ isIntersecting: true } as IntersectionObserverEntry]);
    });
    await waitFor(() => expect(fetch).toHaveBeenCalledTimes(1));
    expect(fetch).toHaveBeenCalledWith('/api/v1/slides/s1/render', expect.any(Object));
  });

  it('shows only an empty state in overview when the project has no pages', () => {
    useProjectStore.setState({ slidesByProjectId: { p1: [] } });
    useDeckStore.setState({ previewMode: 'overview', globalView: 'html', currentSlideId: null });
    render(<PreviewWorkspace />);

    expect(screen.getByText('暂无页面')).toBeInTheDocument();
    expect(screen.queryByText('全局视觉规范')).not.toBeInTheDocument();
    expect(screen.queryAllByTestId(/overview-slide-/)).toHaveLength(0);
  });

  it('aligns the main preview empty project state with overview', () => {
    useProjectStore.setState({ slidesByProjectId: { p1: [] } });
    useDeckStore.setState({ previewMode: 'main', globalView: 'html', currentSlideId: null });
    render(<PreviewWorkspace />);

    expect(screen.getByText('暂无页面')).toBeInTheDocument();
    expect(screen.queryByText('暂无内容')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '设计稿' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '幻灯片' })).toBeInTheDocument();
  });

  it('uses the no-content state only when a page exists without rendered content', () => {
    useProjectStore.setState((state) => ({
      slidesByProjectId: {
        ...state.slidesByProjectId,
        p1: [{ ...state.slidesByProjectId.p1[0], html_path: '' }],
      },
    }));
    useProjectStore.setState((state) => ({
      specByProjectId: {
        ...state.specByProjectId,
        p1: { ...state.specByProjectId.p1, slide_specs: {} },
      },
    }));
    useDeckStore.setState({ previewMode: 'main', globalView: 'html', currentSlideId: 's1' });
    render(<PreviewWorkspace />);

    expect(screen.getByText('暂时没有幻灯片内容')).toBeInTheDocument();
    expect(screen.queryByText('暂无页面')).not.toBeInTheDocument();
  });

  it('shows an actionable HTML error and retries without side effects', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('offline'); }));
    render(<PreviewWorkspace />);
    expect(await screen.findByText(/HTML 加载失败/)).toBeInTheDocument();

    vi.stubGlobal('fetch', vi.fn(async () => ({
      ok: true,
      status: 200,
      text: async () => '<h1>Recovered</h1>',
    } as Response)));
    fireEvent.click(screen.getByRole('button', { name: '重试' }));
    await waitFor(() => expect(postMessage).toHaveBeenCalledWith({
      type: 'updateDeck',
      slides: [{ id: 's1', html: '<h1>Recovered</h1>' }],
      index: 0,
    }, '*'));
  });

  it('supports keyboard paging and overview without hijacking text input', async () => {
    vi.stubGlobal('IntersectionObserver', class {
      observe() {}
      unobserve() {}
      disconnect() {}
    });
    render(<PreviewWorkspace />);
    await waitFor(() => expect(document.querySelector('iframe')).not.toBeNull());
    await act(async () => {
      fireEvent.keyDown(window, { key: 'ArrowRight' });
    });
    expect(useDeckStore.getState().currentSlideId).toBe('s2');

    const input = document.createElement('input');
    document.body.appendChild(input);
    fireEvent.keyDown(input, { key: 'ArrowLeft' });
    expect(useDeckStore.getState().currentSlideId).toBe('s2');
    input.remove();

    fireEvent.keyDown(screen.getByRole('button', { name: '上一页' }), { key: 'ArrowLeft' });
    expect(useDeckStore.getState().currentSlideId).toBe('s2');

    await act(async () => {
      fireEvent.keyDown(window, { key: 'o' });
    });
    expect(useDeckStore.getState().previewMode).toBe('overview');
  });

  it('connects presentation mode to fullscreen and Escape exits it', async () => {
    const fullscreenDescriptor = Object.getOwnPropertyDescriptor(document, 'fullscreenElement');
    const requestDescriptor = Object.getOwnPropertyDescriptor(HTMLElement.prototype, 'requestFullscreen');
    const exitDescriptor = Object.getOwnPropertyDescriptor(document, 'exitFullscreen');
    const requestFullscreen = vi.fn(async () => {});
    const exitFullscreen = vi.fn(async () => {});
    Object.defineProperty(HTMLElement.prototype, 'requestFullscreen', { configurable: true, value: requestFullscreen });
    Object.defineProperty(document, 'exitFullscreen', { configurable: true, value: exitFullscreen });
    Object.defineProperty(document, 'fullscreenElement', { configurable: true, value: document.body });

    render(<PreviewWorkspace />);
    await waitFor(() => expect(document.querySelector('iframe')).not.toBeNull());
    fireEvent.click(screen.getByRole('button', { name: '全屏放映' }));
    expect(requestFullscreen).toHaveBeenCalledTimes(1);
    fireEvent.keyDown(window, { key: 'Escape' });
    expect(exitFullscreen).toHaveBeenCalledTimes(1);

    if (requestDescriptor) Object.defineProperty(HTMLElement.prototype, 'requestFullscreen', requestDescriptor);
    else Reflect.deleteProperty(HTMLElement.prototype, 'requestFullscreen');
    if (exitDescriptor) Object.defineProperty(document, 'exitFullscreen', exitDescriptor);
    else Reflect.deleteProperty(document, 'exitFullscreen');
    if (fullscreenDescriptor) Object.defineProperty(document, 'fullscreenElement', fullscreenDescriptor);
    else Reflect.deleteProperty(document, 'fullscreenElement');
  });
});

describe('PreviewWorkspace dual view (globalView)', () => {
  beforeEach(() => {
    clearSlideRenderCache();
    postMessage.mockReset();
    vi.stubGlobal('fetch', vi.fn(async () => ({
      ok: true,
      status: 200,
      text: async () => '<h1>Rendered slide</h1>',
    } as Response)));
    Object.defineProperty(window.HTMLIFrameElement.prototype, 'contentWindow', {
      configurable: true,
      get() {
        return { postMessage } as unknown as Window;
      }
    });
    useDeckStore.setState({ currentSlideId: 's1', previewMode: 'main', globalView: 'html' });
    setSpecs();
  });

  it('renders an empty slide state when current page has no html in presentation view', () => {
    useProjectStore.setState({
      projects: [],
      activeProjectId: 'p1',
      slidesByProjectId: {
        p1: [
          { id: 's1', project_id: 'p1', position: 0, layout: 'bullets', title: '封面标题', html_path: '', spec_path: '/slides/p1/s1.json', current_version: 0 },
        ]
      },
      loadingProjects: false
    });
    render(<PreviewWorkspace />);
    expect(screen.getByText('暂时没有幻灯片内容')).toBeInTheDocument();
    expect(screen.queryByText('要点一')).toBeNull();
    expect(document.querySelector('iframe')).toBeNull();
  });

  it('renders SlideSpecCard when switching to design view', () => {
    useProjectStore.setState({
      projects: [],
      activeProjectId: 'p1',
      slidesByProjectId: {
        p1: [
          { id: 's1', project_id: 'p1', position: 0, layout: 'bullets', title: '封面标题', html_path: '', spec_path: '/slides/p1/s1.json', current_version: 0 },
        ]
      },
      loadingProjects: false
    });
    useDeckStore.setState({ globalView: 'outline' });
    render(<PreviewWorkspace />);
    expect(screen.getAllByText('封面标题').length).toBeGreaterThan(0);
    expect(screen.getByText('要点一')).toBeInTheDocument();
    expect(document.querySelector('iframe')).toBeNull();
  });

  it('幻灯片 segment button is NOT disabled even when current page has no html (globalView 全局)', () => {
    useProjectStore.setState({
      projects: [],
      activeProjectId: 'p1',
      slidesByProjectId: {
        p1: [
          { id: 's1', project_id: 'p1', position: 0, layout: 'bullets', title: '封面标题', html_path: '', spec_path: '/slides/p1/s1.json', current_version: 0 },
        ]
      },
      loadingProjects: false
    });
    render(<PreviewWorkspace />);
    const presentationButton = screen.getByRole('button', { name: '幻灯片' });
    expect(presentationButton).not.toBeDisabled();
  });

  it('renders iframe by default when current page has html', async () => {
    useProjectStore.setState({
      projects: [],
      activeProjectId: 'p1',
      slidesByProjectId: {
        p1: [
          { id: 's1', project_id: 'p1', position: 0, layout: 'title', title: 'Slide 1', html_path: '/slides/p1/s1.html', spec_path: '/slides/p1/s1.json', current_version: 1 },
        ]
      },
      loadingProjects: false
    });
    render(<PreviewWorkspace />);
    await waitFor(() => expect(document.querySelector('iframe')).not.toBeNull());
  });

  it('globalView=outline forces OutlineCard even when hasHtml=true', async () => {
    useProjectStore.setState({
      projects: [],
      activeProjectId: 'p1',
      slidesByProjectId: {
        p1: [
          { id: 's1', project_id: 'p1', position: 0, layout: 'title', title: 'S1', html_path: '/slides/p1/s1.html', spec_path: '/slides/p1/s1.json', current_version: 1 },
        ]
      },
      loadingProjects: false,
    });
    useDeckStore.setState({ globalView: 'outline', currentSlideId: 's1', previewMode: 'main' });
    render(<PreviewWorkspace />);
    // 全局 outline：主区应显示 OutlineCard 而非 iframe。
    expect(document.querySelector('iframe')).toBeNull();
  });

  it('grid mode marks slides without generated HTML', async () => {
    useProjectStore.setState({
      projects: [],
      activeProjectId: 'p1',
      slidesByProjectId: {
        p1: [
          { id: 's1', project_id: 'p1', position: 0, layout: 'title', title: 'S1', html_path: '/slides/p1/s1.html', spec_path: '', current_version: 1 },
          { id: 's2', project_id: 'p1', position: 1, layout: 'bullets', title: '', html_path: '', spec_path: '', current_version: 0 },
        ]
      },
      loadingProjects: false,
    });
    useDeckStore.setState({ previewMode: 'overview', globalView: 'html', currentSlideId: 's1' });
    render(<PreviewWorkspace />);
    // 网格模式下 s2 无 html_path，应有徽标。
    expect(screen.getByText('未生成 HTML')).toBeInTheDocument();
    await waitFor(() => expect(document.querySelector('iframe')).not.toBeNull());
    for (const iframe of Array.from(document.querySelectorAll('iframe'))) {
      expect(iframe).toHaveAttribute('sandbox', 'allow-scripts');
      expect(iframe.getAttribute('sandbox')).not.toContain('allow-same-origin');
    }
  });

  it('refreshes from outline fallback to isolated preview after whole-deck generation updates slides', async () => {
    useProjectStore.setState({
      projects: [],
      activeProjectId: 'p1',
      slidesByProjectId: {
        p1: [
          { id: 's1', project_id: 'p1', position: 0, layout: 'title', title: 'S1', html_path: '', spec_path: '', current_version: 0 },
        ],
      },
      loadingProjects: false,
    });
    render(<PreviewWorkspace />);
    expect(document.querySelector('iframe')).toBeNull();

    await act(async () => {
      useProjectStore.setState((state) => ({
        slidesByProjectId: {
          ...state.slidesByProjectId,
          p1: [{
            ...state.slidesByProjectId.p1[0],
            html_path: 'slides/s1/index.html',
            current_version: 1,
          }],
        },
      }));
    });

    await waitFor(() => expect(document.querySelector('iframe')).not.toBeNull());
    await waitFor(() => expect(postMessage).toHaveBeenCalledWith({
      type: 'updateDeck',
      slides: [{ id: 's1', html: '<h1>Rendered slide</h1>' }],
      index: 0,
    }, '*'));
  });
});
