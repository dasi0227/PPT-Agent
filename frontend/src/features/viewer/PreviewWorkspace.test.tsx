import { render, waitFor, act, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { PreviewWorkspace } from './PreviewWorkspace';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { useBlueprintStore } from '../../stores/blueprintStore';

const blueprint = (id: string, title = '封面标题') => ({
  schema_version: '2.0' as const, revision: 1, slide_id: id, section_id: 'main', role: 'cover',
  title, key_message: title, content: { summary: title, points: ['要点一'] },
  visual_intent: { archetype: 'hero', description: '主视觉', asset_queries: [] },
  speaker_notes: '', created_at: 1, updated_at: 1,
});
const setBlueprints = () => useBlueprintStore.setState({ byProjectId: { p1: {
  deck: { schema_version: '2.0', revision: 1, project_id: 'p1', title: 'Deck', goal: '', audience: '', language: 'zh-CN', core_thesis: '', narrative_arc: '', sections: [], slide_order: ['s1', 's2'], created_at: 1, updated_at: 1 },
  slides: { s1: blueprint('s1'), s2: blueprint('s2', '第二页') },
  design_spec: { schema_version: '2.0', revision: 1, canvas: {}, palette: [], typography: {}, spacing: {}, radius: {}, shadows: {}, layout_system: {}, signature: '', motion: {} },
  materialization: {
    s1: { state: 'not_materialized', revisions: { presentation: 0, source_deck: 0, source_blueprint: 0, source_design: 0 } },
    s2: { state: 'not_materialized', revisions: { presentation: 0, source_deck: 0, source_blueprint: 0, source_design: 0 } },
  },
} } });

const postMessage = vi.fn();

describe('PreviewWorkspace', () => {
  beforeEach(() => {
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
    setBlueprints();
  });

  it('fetches stable render endpoints, posts HTML content, and uses goto for page switches', async () => {
    render(<PreviewWorkspace />);

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
    expect(fetch).toHaveBeenCalledWith('/api/v1/slides/s1/render', expect.any(Object));
    expect(fetch).toHaveBeenCalledWith('/api/v1/slides/s2/render', expect.any(Object));

    postMessage.mockClear();

    await act(async () => {
      useDeckStore.getState().setCurrentPage(1);
    });

    await waitFor(() => {
      expect(postMessage).toHaveBeenCalledTimes(1);
    });

    expect(postMessage).toHaveBeenCalledWith({ type: 'gotoSlide', index: 1 }, '*');
  });

  it('keeps the main runtime sandboxed without same-origin privilege', async () => {
    render(<PreviewWorkspace />);
    await waitFor(() => expect(document.querySelector('iframe')).not.toBeNull());
    const iframe = document.querySelector('iframe');
    expect(iframe).toHaveAttribute('sandbox', 'allow-scripts');
    expect(iframe?.getAttribute('sandbox')).not.toContain('allow-same-origin');
  });
});

describe('PreviewWorkspace dual view (globalView)', () => {
  beforeEach(() => {
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
    useDeckStore.setState({ currentPage: 0, previewMode: 'main', globalView: 'html', viewByPage: {} });
    setBlueprints();
  });

  it('renders SlideBlueprintCard (not iframe) when current page has no html', () => {
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
    expect(screen.getAllByText('封面标题').length).toBeGreaterThan(0);
    expect(screen.getByText('要点一')).toBeInTheDocument();
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

  it('renders iframe by default when current page has html', async () => {
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
    await waitFor(() => expect(document.querySelector('iframe')).not.toBeNull());
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
          { id: 's1', project_id: 'p1', idx: 0, layout: 'title', title: 'S1', html_path: '', json_path: '', current_version: 0, order: 10, outline_dirty: false,
            content: { layout: 'title', title: 'S1' } },
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
