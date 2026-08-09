import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { DeckNavigator } from './DeckNavigator';
import { useProjectStore } from '../../stores/projectStore';
import { useDeckStore } from '../../stores/deckStore';
import { slidesApi } from '../../api/slides';
import { useSpecStore } from '../../stores/specStore';
import { clearSlideRenderCache } from '../viewer/useSlideRenderCache';

describe('DeckNavigator', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    clearSlideRenderCache();
    useDeckStore.setState({ currentPage: 0, globalView: 'html' });
    useProjectStore.setState({
      projects: [{ id: 'p1', title: '演示项目', work_dir: '', theme: 'swiss', status: 'draft', design_path: '', created_at: 0, updated_at: 0 }],
      activeProjectId: 'p1',
      slidesByProjectId: {
        p1: [
          { id: 's1', project_id: 'p1', position: 0, layout: 'cover', title: '市场分析', html_path: '', spec_path: '/a.json', current_version: 0 },
          { id: 's2', project_id: 'p1', position: 1, layout: 'content', title: '增长趋势', html_path: '', spec_path: '/b.json', current_version: 0 },
        ]
      },
      loadingProjects: false
    });
    useSpecStore.setState({ byProjectId: { p1: {
      outline: { version: '3.0', revision: 1, project_id: 'pro_aaaaaa', title: '演示项目', goal: '', audience: '', language: 'zh-CN', positioning: '核心命题', constraints: { must_include: [], must_avoid: [], style_limits: [], content_limits: [] }, sections: [{ id: 'sec', title: '市场', purpose: '说明市场趋势', subsections: [{ id: 'sub', title: '趋势' }] }], slide_order: ['s1', 's2'], created_at: 1, updated_at: 1 },
      slide_specs: {
        s1: { version: '3.0', revision: 1, project_id: 'p1', slide_id: 's1', section_id: 'sec', role: 'cover', title: '市场分析', key_message: '市场在扩大', elements: [{ type: 'text', intent: '摘要' }], layout: 'cover', created_at: 1, updated_at: 1 },
        s2: { version: '3.0', revision: 2, project_id: 'p1', slide_id: 's2', section_id: 'sec', subsection_id: 'sub', role: 'evidence', title: '增长趋势', key_message: '增长持续', elements: [{ type: 'chart', intent: '趋势图' }], layout: 'chart', created_at: 1, updated_at: 2 },
      },
      design: { version: '3.0', revision: 1, project_id: 'pro_aaaaaa', theme: 'swiss-modern', direction: 'test direction', density: 'medium', chrome: [], created_at: 1, updated_at: 1 },
      materialization: {
        s1: { state: 'fresh', revisions: { slide_html: 1, source_outline: 1, source_spec: 1, source_design: 1 } },
        s2: { state: 'spec_stale', revisions: { slide_html: 1, source_outline: 1, source_spec: 1, source_design: 1 } },
      },
    } } });
  });

  function setMultiSectionFixture() {
    useProjectStore.setState((state) => ({
      slidesByProjectId: {
        ...state.slidesByProjectId,
        p1: [
          { id: 's1', project_id: 'p1', position: 0, layout: 'content', title: '章节一第一项', html_path: '', spec_path: '/s1.json', current_version: 0 },
          { id: 's2', project_id: 'p1', position: 1, layout: 'content', title: '章节一第二项', html_path: '', spec_path: '/s2.json', current_version: 0 },
          { id: 's3', project_id: 'p1', position: 2, layout: 'content', title: '章节二第一项', html_path: '', spec_path: '/s3.json', current_version: 0 },
        ],
      },
    }));
    useSpecStore.setState({ byProjectId: { p1: {
      outline: {
        version: '3.0', revision: 1, project_id: 'pro_aaaaaa', title: '演示项目',
        goal: '', audience: '', language: 'zh-CN', positioning: '核心命题', constraints: { must_include: [], must_avoid: [], style_limits: [], content_limits: [] },
        sections: [
          {
            id: 'sec1', title: '第一章', purpose: '第一章定位',
            subsections: [
              { id: 'sub11', title: '第一节' },
              { id: 'sub12', title: '第二节' },
            ],
          },
          {
            id: 'sec2', title: '第二章', purpose: '第二章定位',
            subsections: [{ id: 'sub21', title: '第一节' }],
          },
        ],
        slide_order: ['s1', 's2', 's3'], created_at: 1, updated_at: 1,
      },
      slide_specs: {
        s1: { version: '3.0', revision: 1, project_id: 'p1', slide_id: 's1', section_id: 'sec1', subsection_id: 'sub11', role: 'context', title: '章节一第一项', key_message: 'A', elements: [{ type: 'text', intent: 'A' }], layout: 'content', created_at: 1, updated_at: 1 },
        s2: { version: '3.0', revision: 1, project_id: 'p1', slide_id: 's2', section_id: 'sec1', subsection_id: 'sub12', role: 'context', title: '章节一第二项', key_message: 'B', elements: [{ type: 'text', intent: 'B' }], layout: 'content', created_at: 1, updated_at: 1 },
        s3: { version: '3.0', revision: 1, project_id: 'p1', slide_id: 's3', section_id: 'sec2', subsection_id: 'sub21', role: 'context', title: '章节二第一项', key_message: 'C', elements: [{ type: 'text', intent: 'C' }], layout: 'content', created_at: 1, updated_at: 1 },
      },
      design: { version: '3.0', revision: 1, project_id: 'pro_aaaaaa', theme: 'swiss-modern', direction: 'test direction', density: 'medium', chrome: [], created_at: 1, updated_at: 1 },
      materialization: {},
    } } });
  }

  it('shows real slide titles instead of generic labels', () => {
    useDeckStore.setState({ globalView: 'outline' });
    render(<DeckNavigator />);
    expect(screen.getByText('市场分析')).toBeInTheDocument();
    expect(screen.getByText('增长趋势')).toBeInTheDocument();
    expect(screen.queryByText('Slide 1')).not.toBeInTheDocument();
    expect(screen.queryByText('核心命题')).not.toBeInTheDocument();
    expect(screen.queryByText('暂无')).toBeNull();
  });

  it('hides materialization badges and layout machine fields from the directory', () => {
    render(<DeckNavigator />);
    expect(screen.queryByText('设计稿有更新')).toBeNull();
    expect(screen.queryByText('未生成')).toBeNull();
    expect(screen.queryByText('cover')).toBeNull();
    expect(screen.queryByText('content')).toBeNull();
    expect(screen.getAllByText('暂无')).toHaveLength(2);
    expect(screen.queryByText('市场分析')).toBeNull();
    expect(screen.queryByText('增长趋势')).toBeNull();
  });

  it('renders section and subsection directory hierarchy', () => {
    render(<DeckNavigator />);
    const section = screen.getByText('1. 市场');
    const subsection = screen.getByText('1.1 趋势');
    expect(section).toBeInTheDocument();
    expect(subsection).toBeInTheDocument();
    expect(section).toHaveClass('font-normal');
    expect(subsection).toHaveClass('font-normal');
    expect(section).not.toHaveClass('font-semibold');
    expect(subsection).not.toHaveClass('font-medium');
  });

  it('renders low-resolution HTML thumbnails when a page has HTML', async () => {
    useProjectStore.setState((state) => ({
      slidesByProjectId: {
        ...state.slidesByProjectId,
        p1: [
          { ...state.slidesByProjectId.p1[0], html_path: '/slides/s1/index.html', current_version: 1, html_revision: 1 },
          state.slidesByProjectId.p1[1],
        ],
      },
    }));
    vi.spyOn(slidesApi, 'render').mockResolvedValue('<!doctype html><html><body><section>Preview</section></body></html>');

    render(<DeckNavigator />);

    await waitFor(() => expect(slidesApi.render).toHaveBeenCalledWith('s1', expect.any(AbortSignal)));
    expect(await screen.findByTitle('第 1 页缩略图')).toBeInTheDocument();
    expect(screen.getAllByText('暂无')).toHaveLength(1);
  });

  it('prefetches directory thumbnails before switching into HTML view', async () => {
    useDeckStore.setState({ globalView: 'outline' });
    useProjectStore.setState((state) => ({
      slidesByProjectId: {
        ...state.slidesByProjectId,
        p1: [
          { ...state.slidesByProjectId.p1[0], html_path: '/slides/s1/index.html', current_version: 1, html_revision: 1 },
          state.slidesByProjectId.p1[1],
        ],
      },
    }));
    vi.spyOn(slidesApi, 'render').mockResolvedValue('<!doctype html><html><body><section>Preview</section></body></html>');

    render(<DeckNavigator />);

    expect(screen.getByText('市场分析')).toBeInTheDocument();
    await waitFor(() => expect(slidesApi.render).toHaveBeenCalledWith('s1', expect.any(AbortSignal)));
  });

  it('dragging a page onto a subsection page updates its placement', async () => {
    useDeckStore.setState({ globalView: 'outline' });
    const restructureSpy = vi.spyOn(slidesApi, 'restructure').mockResolvedValue(undefined as never);
    vi.spyOn(useProjectStore.getState(), 'loadProjectSlides').mockResolvedValue();
    const transfer = {
      value: '',
      setData: vi.fn((_type: string, value: string) => { transfer.value = value; }),
      getData: vi.fn(() => transfer.value),
      effectAllowed: '',
      dropEffect: '',
    };

    render(<DeckNavigator />);
    fireEvent.dragStart(screen.getByText('市场分析').closest('[draggable="true"]')!, { dataTransfer: transfer });
    fireEvent.drop(screen.getByText('增长趋势').closest('[draggable="true"]')!, { dataTransfer: transfer });

    await waitFor(() => expect(restructureSpy).toHaveBeenCalledWith(
      'p1',
      ['s1', 's2'],
      [
        { slide_id: 's1', section_id: 'sec', subsection_id: 'sub' },
        { slide_id: 's2', section_id: 'sec', subsection_id: 'sub' },
      ],
    ));
  });

  it('moves pages across subsection boundaries with the same restructure contract as dragging', async () => {
    useDeckStore.setState({ globalView: 'outline' });
    const restructureSpy = vi.spyOn(slidesApi, 'restructure').mockResolvedValue(undefined as never);
    vi.spyOn(useProjectStore.getState(), 'loadProjectSlides').mockResolvedValue();

    render(<DeckNavigator />);

    const upButtons = screen.getAllByRole('button', { name: '上移本页' });
    const downButtons = screen.getAllByRole('button', { name: '下移本页' });
    expect(upButtons[0]).toBeDisabled();
    expect(downButtons[0]).not.toBeDisabled();
    expect(upButtons[1]).not.toBeDisabled();
    expect(downButtons[1]).toBeDisabled();

    fireEvent.click(downButtons[0]);

    await waitFor(() => expect(restructureSpy).toHaveBeenCalledWith(
      'p1',
      ['s2', 's1'],
      [
        { slide_id: 's2', section_id: 'sec', subsection_id: 'sub' },
        { slide_id: 's1', section_id: 'sec', subsection_id: 'sub' },
      ],
    ));
  });

  it('moves the last page of a section down as the next section direct boundary item', async () => {
    useDeckStore.setState({ globalView: 'outline' });
    setMultiSectionFixture();
    const restructureSpy = vi.spyOn(slidesApi, 'restructure').mockResolvedValue(undefined as never);
    vi.spyOn(useProjectStore.getState(), 'loadProjectSlides').mockResolvedValue();

    render(<DeckNavigator />);
    fireEvent.click(screen.getAllByRole('button', { name: '下移本页' })[1]);

    await waitFor(() => expect(restructureSpy).toHaveBeenCalledWith(
      'p1',
      ['s1', 's2', 's3'],
      [
        { slide_id: 's1', section_id: 'sec1', subsection_id: 'sub11' },
        { slide_id: 's2', section_id: 'sec2' },
        { slide_id: 's3', section_id: 'sec2', subsection_id: 'sub21' },
      ],
    ));
  });

  it('moves the first page of a section up as the previous section direct boundary item', async () => {
    useDeckStore.setState({ globalView: 'outline' });
    setMultiSectionFixture();
    const restructureSpy = vi.spyOn(slidesApi, 'restructure').mockResolvedValue(undefined as never);
    vi.spyOn(useProjectStore.getState(), 'loadProjectSlides').mockResolvedValue();

    render(<DeckNavigator />);
    fireEvent.click(screen.getAllByRole('button', { name: '上移本页' })[2]);

    await waitFor(() => expect(restructureSpy).toHaveBeenCalledWith(
      'p1',
      ['s1', 's2', 's3'],
      [
        { slide_id: 's1', section_id: 'sec1', subsection_id: 'sub11' },
        { slide_id: 's2', section_id: 'sec1', subsection_id: 'sub12' },
        { slide_id: 's3', section_id: 'sec1' },
      ],
    ));
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
