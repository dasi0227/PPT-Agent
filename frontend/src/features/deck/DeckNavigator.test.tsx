import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { DeckNavigator } from './DeckNavigator';
import { useProjectStore } from '../../stores/projectStore';
import { useDeckStore } from '../../stores/deckStore';
import { slidesApi } from '../../api/slides';
import { clearSlideRenderCache } from '../viewer/useSlideRenderCache';
import { BrowserRouter } from 'react-router-dom';
import { useWorkspaceUrlState } from '../workspace/useWorkspaceUrlState';

function RoutedDeckNavigator() {
  useWorkspaceUrlState('p1');
  return <DeckNavigator />;
}

describe('DeckNavigator', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    clearSlideRenderCache();
    useDeckStore.setState({ currentSlideId: 's1', globalView: 'html' });
    useProjectStore.setState({
      projects: [{ id: 'p1', title: '演示项目', work_dir: '', theme: 'swiss', status: 'draft', design_path: '', created_at: 0, updated_at: 0 }],
      activeProjectId: 'p1',
      slidesByProjectId: {
        p1: [
          { id: 's1', project_id: 'p1', position: 0, layout: 'cover', title: '市场分析', html_path: '', spec_path: '/a.json', current_version: 0 },
          { id: 's2', project_id: 'p1', position: 1, layout: 'content', title: '增长趋势', html_path: '', spec_path: '/b.json', current_version: 0 },
        ]
      },
      loadingProjects: false,
      specByProjectId: { p1: {
      outline: { version: '3.0', revision: 1, project_id: 'pro_aaaaaa', title: '演示项目', goal: '', audience: '', language: 'zh-CN', positioning: '核心命题', requirements: [], prohibitions: [], sections: [{ id: 'sec', title: '市场', purpose: '说明市场趋势', subsections: [{ id: 'sub', title: '趋势' }] }], slide_order: ['s1', 's2'], created_at: 1, updated_at: 1 },
      slide_specs: {
        s1: { version: '3.0', revision: 1, project_id: 'p1', slide_id: 's1', section_id: 'sec', role: 'cover', title: '市场分析', key_message: '市场在扩大', elements: [{ type: 'text', intent: '摘要' }], layout: 'cover', created_at: 1, updated_at: 1 },
        s2: { version: '3.0', revision: 2, project_id: 'p1', slide_id: 's2', section_id: 'sec', subsection_id: 'sub', role: 'evidence', title: '增长趋势', key_message: '增长持续', elements: [{ type: 'chart', intent: '趋势图' }], layout: 'chart', created_at: 1, updated_at: 2 },
      },
      design: { version: '3.0', revision: 1, project_id: 'pro_aaaaaa', theme: 'swiss-modern', direction: 'test direction', density: 'medium', chrome: [], created_at: 1, updated_at: 1 },
      materialization: {
        s1: { state: 'fresh', revisions: { slide_html: 1, source_outline: 1, source_spec: 1, source_design: 1 } },
        s2: { state: 'spec_stale', revisions: { slide_html: 1, source_outline: 1, source_spec: 1, source_design: 1 } },
      },
      } },
    });
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
    useProjectStore.setState({ specByProjectId: { p1: {
      outline: {
        version: '3.0', revision: 1, project_id: 'pro_aaaaaa', title: '演示项目',
        goal: '', audience: '', language: 'zh-CN', positioning: '核心命题', requirements: [], prohibitions: [],
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

  function openPageActions(index: number) {
    const trigger = screen.getAllByRole('button', { name: '页面操作' })[index];
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    fireEvent.click(trigger);
  }

  function openSectionActions(index: number) {
    const trigger = screen.getAllByRole('button', { name: '章节操作' })[index];
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    fireEvent.click(trigger);
  }

  function openSubsectionActions(index = 0) {
    const trigger = screen.getAllByRole('button', { name: '子节操作' })[index];
    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    fireEvent.click(trigger);
  }

  it('shows real slide titles instead of generic labels', () => {
    useDeckStore.setState({ globalView: 'outline' });
    render(<DeckNavigator />);
    expect(screen.getByText('市场分析')).toHaveClass('text-[16px]', 'font-semibold');
    expect(screen.getByText('增长趋势')).toHaveClass('text-[16px]', 'font-semibold');
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
    const sectionToggle = screen.getByRole('button', { name: '收起第 1 章 市场' });
    expect(sectionToggle).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByText('市场')).toHaveClass('font-semibold');
    expect(screen.getByText('1.1')).toBeInTheDocument();
    expect(screen.getByText('趋势')).toHaveClass('font-medium');
    expect(sectionToggle).not.toHaveTextContent('页');
  });

  it('limits section hover controls to the section heading row', () => {
    render(<DeckNavigator />);

    const sectionToggle = screen.getByRole('button', { name: '收起第 1 章 市场' });
    expect(sectionToggle.parentElement).toHaveClass('group/section');
    expect(sectionToggle.closest('section')).not.toHaveClass('group/section');
  });

  it('aligns the subsection action in the heading grid', () => {
    render(<DeckNavigator />);

    const subsectionActions = screen.getByRole('button', { name: '子节操作' });
    const actionCell = subsectionActions.parentElement;
    expect(actionCell).toHaveClass('items-center', 'justify-self-end');
    expect(actionCell).not.toHaveClass('absolute');
    expect(actionCell?.parentElement).toHaveClass('grid-cols-[32px_minmax(0,1fr)_20px]', 'items-center');
  });

  it('puts section structure edits behind the overflow menu', () => {
    render(<DeckNavigator />);

    openSectionActions(0);
    expect(screen.getAllByRole('menuitem').map((item) => item.textContent)).toEqual(['重命名', '新增子节', '删除章节']);
  });

  it('puts subsection rename and delete behind one overflow menu', () => {
    render(<DeckNavigator />);

    expect(screen.queryByRole('button', { name: '删除本子节' })).toBeNull();
    openSubsectionActions();
    expect(screen.getAllByRole('menuitem').map((item) => item.textContent)).toEqual(['重命名', '删除子节']);
  });

  it('restores hover-only section controls after a pointer interaction', () => {
    render(<DeckNavigator />);

    const trigger = screen.getByRole('button', { name: '章节操作' });
    const controls = trigger.parentElement;
    expect(controls).not.toBeNull();

    fireEvent.pointerDown(trigger, { button: 0, ctrlKey: false });
    fireEvent.click(trigger);

    expect(controls).toHaveClass('opacity-0');
  });

  it('keeps section controls visible for keyboard navigation', () => {
    render(<DeckNavigator />);

    const trigger = screen.getByRole('button', { name: '章节操作' });
    const controls = trigger.parentElement;
    fireEvent.keyDown(trigger, { key: 'Tab' });
    fireEvent.focus(trigger);

    expect(controls).toHaveClass('opacity-100');
  });

  it('gives page numbers a stronger visual weight than directory numbers', () => {
    render(<DeckNavigator />);

    expect(screen.getByText('01')).toHaveClass('text-[18px]', 'font-bold', 'text-text-900');
  });

  it('expands the current section by default and lets multiple sections stay open', () => {
    setMultiSectionFixture();
    render(<DeckNavigator />);

    const firstSection = screen.getByRole('button', { name: '收起第 1 章 第一章' });
    const secondSection = screen.getByRole('button', { name: '展开第 2 章 第二章' });
    expect(firstSection).toHaveAttribute('aria-expanded', 'true');
    expect(secondSection).toHaveAttribute('aria-expanded', 'false');

    fireEvent.click(secondSection);
    expect(firstSection).toHaveAttribute('aria-expanded', 'true');
    expect(secondSection).toHaveAttribute('aria-expanded', 'true');

    fireEvent.click(firstSection);
    expect(firstSection).toHaveAttribute('aria-expanded', 'false');
    expect(secondSection).toHaveAttribute('aria-expanded', 'true');
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
    const restructureSpy = vi.spyOn(slidesApi, 'restructure').mockImplementation(async () => ({
      slides: useProjectStore.getState().slidesByProjectId.p1,
      spec: useProjectStore.getState().specByProjectId.p1,
    }));
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
    const restructureSpy = vi.spyOn(slidesApi, 'restructure').mockImplementation(async () => ({
      slides: useProjectStore.getState().slidesByProjectId.p1,
      spec: useProjectStore.getState().specByProjectId.p1,
    }));

    render(<DeckNavigator />);

    openPageActions(0);
    expect(screen.getByRole('menuitem', { name: '上移本页' })).toHaveAttribute('data-disabled');
    expect(screen.getByRole('menuitem', { name: '下移本页' })).not.toHaveAttribute('data-disabled');
    fireEvent.keyDown(document, { key: 'Escape' });

    openPageActions(1);
    expect(screen.getByRole('menuitem', { name: '上移本页' })).not.toHaveAttribute('data-disabled');
    expect(screen.getByRole('menuitem', { name: '下移本页' })).toHaveAttribute('data-disabled');
    fireEvent.keyDown(document, { key: 'Escape' });

    openPageActions(0);
    fireEvent.click(screen.getByRole('menuitem', { name: '下移本页' }));

    await waitFor(() => expect(restructureSpy).toHaveBeenCalledWith(
      'p1',
      ['s1', 's2'],
      [
        { slide_id: 's1', section_id: 'sec', subsection_id: 'sub' },
        { slide_id: 's2', section_id: 'sec', subsection_id: 'sub' },
      ],
    ));
  });

  it('moves a 3.1 page into empty 3.2 instead of skipping to section 4', async () => {
    useDeckStore.setState({ globalView: 'outline' });
    setMultiSectionFixture();
    useProjectStore.setState((state) => ({
      specByProjectId: {
        ...state.specByProjectId,
        p1: {
          ...state.specByProjectId.p1,
          slide_specs: {
            ...state.specByProjectId.p1.slide_specs,
            s2: { ...state.specByProjectId.p1.slide_specs.s2, subsection_id: 'sub11' },
          },
        },
      },
    }));
    const restructureSpy = vi.spyOn(slidesApi, 'restructure').mockImplementation(async () => ({
      slides: useProjectStore.getState().slidesByProjectId.p1,
      spec: useProjectStore.getState().specByProjectId.p1,
    }));

    render(<DeckNavigator />);
    openPageActions(1);
    fireEvent.click(screen.getByRole('menuitem', { name: '下移本页' }));

    await waitFor(() => expect(restructureSpy).toHaveBeenCalledWith(
      'p1',
      ['s1', 's2', 's3'],
      [
        { slide_id: 's1', section_id: 'sec1', subsection_id: 'sub11' },
        { slide_id: 's2', section_id: 'sec1', subsection_id: 'sub12' },
        { slide_id: 's3', section_id: 'sec2', subsection_id: 'sub21' },
      ],
    ));
  });

  it('reorders adjacent pages inside one subsection without changing placement', async () => {
    useDeckStore.setState({ globalView: 'outline' });
    setMultiSectionFixture();
    useProjectStore.setState((state) => ({
      specByProjectId: {
        ...state.specByProjectId,
        p1: {
          ...state.specByProjectId.p1,
          slide_specs: {
            ...state.specByProjectId.p1.slide_specs,
            s2: { ...state.specByProjectId.p1.slide_specs.s2, subsection_id: 'sub11' },
          },
        },
      },
    }));
    const restructureSpy = vi.spyOn(slidesApi, 'restructure').mockImplementation(async () => ({
      slides: useProjectStore.getState().slidesByProjectId.p1,
      spec: useProjectStore.getState().specByProjectId.p1,
    }));

    render(<DeckNavigator />);
    openPageActions(1);
    fireEvent.click(screen.getByRole('menuitem', { name: '上移本页' }));

    await waitFor(() => expect(restructureSpy).toHaveBeenCalledWith(
      'p1',
      ['s2', 's1', 's3'],
      [
        { slide_id: 's2', section_id: 'sec1', subsection_id: 'sub11' },
        { slide_id: 's1', section_id: 'sec1', subsection_id: 'sub11' },
        { slide_id: 's3', section_id: 'sec2', subsection_id: 'sub21' },
      ],
    ));
  });

  it('keeps page actions behind a single overflow menu', () => {
    useDeckStore.setState({ globalView: 'outline' });
    render(<DeckNavigator />);

    const firstPage = screen.getByText('市场分析').closest<HTMLElement>('[draggable="true"]');
    expect(firstPage).not.toBeNull();
    expect(within(firstPage!).getAllByRole('button').map((button) => button.getAttribute('aria-label'))).toEqual(['页面操作']);
    openPageActions(0);
    expect(screen.getAllByRole('menuitem').map((item) => item.textContent)).toEqual(['重命名', '上移本页', '下移本页', '删除本页']);
  });

  it('renames a section and applies the authoritative snapshot', async () => {
    const renameSpy = vi.spyOn(slidesApi, 'renameSection').mockImplementation(async (_projectId, sectionId, title) => {
      const state = useProjectStore.getState();
      const view = state.specByProjectId.p1;
      return {
        slides: state.slidesByProjectId.p1,
        spec: {
          ...view,
          outline: {
            ...view.outline,
            revision: view.outline.revision + 1,
            sections: view.outline.sections.map((section) => section.id === sectionId ? { ...section, title } : section),
          },
        },
      };
    });

    render(<DeckNavigator />);
    openSectionActions(0);
    fireEvent.click(screen.getByRole('menuitem', { name: '重命名' }));
    fireEvent.change(screen.getByRole('textbox', { name: '新名称' }), { target: { value: '市场机会' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(renameSpy).toHaveBeenCalledWith('p1', 'sec', '市场机会'));
    expect(await screen.findByText('市场机会')).toBeInTheDocument();
  });

  it('renames a subsection and applies the authoritative snapshot', async () => {
    const renameSpy = vi.spyOn(slidesApi, 'renameSubsection').mockImplementation(async (_projectId, sectionId, subsectionId, title) => {
      const state = useProjectStore.getState();
      const view = state.specByProjectId.p1;
      return {
        slides: state.slidesByProjectId.p1,
        spec: {
          ...view,
          outline: {
            ...view.outline,
            revision: view.outline.revision + 1,
            sections: view.outline.sections.map((section) => section.id === sectionId
              ? {
                  ...section,
                  subsections: section.subsections.map((subsection) => subsection.id === subsectionId
                    ? { ...subsection, title }
                    : subsection),
                }
              : section),
          },
        },
      };
    });

    render(<DeckNavigator />);
    openSubsectionActions();
    fireEvent.click(screen.getByRole('menuitem', { name: '重命名' }));
    fireEvent.change(screen.getByRole('textbox', { name: '新名称' }), { target: { value: '长期趋势' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(renameSpy).toHaveBeenCalledWith('p1', 'sec', 'sub', '长期趋势'));
    expect(await screen.findByText('长期趋势')).toBeInTheDocument();
  });

  it('renames a page and updates both slide projections', async () => {
    useDeckStore.setState({ globalView: 'outline' });
    const renameSpy = vi.spyOn(slidesApi, 'renameSlide').mockImplementation(async (_projectId, slideId, title) => {
      const state = useProjectStore.getState();
      const view = state.specByProjectId.p1;
      return {
        slides: state.slidesByProjectId.p1.map((slide) => slide.id === slideId ? { ...slide, title } : slide),
        spec: {
          ...view,
          slide_specs: {
            ...view.slide_specs,
            [slideId]: { ...view.slide_specs[slideId], title, revision: view.slide_specs[slideId].revision + 1 },
          },
        },
      };
    });

    render(<DeckNavigator />);
    openPageActions(0);
    fireEvent.click(screen.getByRole('menuitem', { name: '重命名' }));
    fireEvent.change(screen.getByRole('textbox', { name: '新名称' }), { target: { value: '市场总览' } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(renameSpy).toHaveBeenCalledWith('p1', 's1', '市场总览'));
    expect(await screen.findByText('市场总览')).toBeInTheDocument();
  });

  it('moves 3.2 down directly into 4.1 when section 4 has no direct pages', async () => {
    useDeckStore.setState({ globalView: 'outline' });
    setMultiSectionFixture();
    const restructureSpy = vi.spyOn(slidesApi, 'restructure').mockImplementation(async () => ({
      slides: useProjectStore.getState().slidesByProjectId.p1,
      spec: useProjectStore.getState().specByProjectId.p1,
    }));

    render(<DeckNavigator />);
    openPageActions(1);
    fireEvent.click(screen.getByRole('menuitem', { name: '下移本页' }));

    await waitFor(() => expect(restructureSpy).toHaveBeenCalledWith(
      'p1',
      ['s1', 's2', 's3'],
      [
        { slide_id: 's1', section_id: 'sec1', subsection_id: 'sub11' },
        { slide_id: 's2', section_id: 'sec2', subsection_id: 'sub21' },
        { slide_id: 's3', section_id: 'sec2', subsection_id: 'sub21' },
      ],
    ));
  });

  it('moves 4.1 up directly into 3.2 when section 4 has no direct pages', async () => {
    useDeckStore.setState({ globalView: 'outline' });
    setMultiSectionFixture();
    const restructureSpy = vi.spyOn(slidesApi, 'restructure').mockImplementation(async () => ({
      slides: useProjectStore.getState().slidesByProjectId.p1,
      spec: useProjectStore.getState().specByProjectId.p1,
    }));

    render(<DeckNavigator />);
    fireEvent.click(screen.getByRole('button', { name: '展开第 2 章 第二章' }));
    openPageActions(2);
    fireEvent.click(screen.getByRole('menuitem', { name: '上移本页' }));

    await waitFor(() => expect(restructureSpy).toHaveBeenCalledWith(
      'p1',
      ['s1', 's2', 's3'],
      [
        { slide_id: 's1', section_id: 'sec1', subsection_id: 'sub11' },
        { slide_id: 's2', section_id: 'sec1', subsection_id: 'sub12' },
        { slide_id: 's3', section_id: 'sec1', subsection_id: 'sub12' },
      ],
    ));
  });

  it('keeps the moved slide selected while the authoritative snapshot changes its index', async () => {
    useDeckStore.setState({ currentSlideId: 's2', globalView: 'outline' });
    setMultiSectionFixture();
    useProjectStore.setState((state) => ({
      specByProjectId: {
        ...state.specByProjectId,
        p1: {
          ...state.specByProjectId.p1,
          slide_specs: {
            ...state.specByProjectId.p1.slide_specs,
            s2: { ...state.specByProjectId.p1.slide_specs.s2, subsection_id: 'sub11' },
          },
        },
      },
    }));
    let resolveRestructure!: (snapshot: Awaited<ReturnType<typeof slidesApi.restructure>>) => void;
    vi.spyOn(slidesApi, 'restructure').mockImplementation(() => new Promise((resolve) => {
      resolveRestructure = resolve;
    }));

    render(<DeckNavigator />);
    openPageActions(1);
    fireEvent.click(screen.getByRole('menuitem', { name: '上移本页' }));

    expect(useDeckStore.getState().currentSlideId).toBe('s2');
    const state = useProjectStore.getState();
    resolveRestructure({
      slides: [state.slidesByProjectId.p1[1], state.slidesByProjectId.p1[0], state.slidesByProjectId.p1[2]],
      spec: state.specByProjectId.p1,
    });

    await waitFor(() => {
      expect(useProjectStore.getState().slidesByProjectId.p1[0].id).toBe('s2');
      expect(useDeckStore.getState().currentSlideId).toBe('s2');
    });
  });

  it('keeps slide 3.2 stable in the real router while moving it up to 3.1', async () => {
    window.history.pushState({}, '', '/projects/p1?slide=s2&view=outline');
    useDeckStore.setState({ currentSlideId: 's2', globalView: 'outline', previewMode: 'main' });
    vi.spyOn(slidesApi, 'restructure').mockImplementation(async () => {
      const state = useProjectStore.getState();
      const currentSpec = state.specByProjectId.p1;
      return {
        slides: state.slidesByProjectId.p1,
        spec: {
          ...currentSpec,
          outline: { ...currentSpec.outline, revision: 2, slide_order: ['s1', 's2'] },
          slide_specs: {
            ...currentSpec.slide_specs,
            s2: { ...currentSpec.slide_specs.s2, subsection_id: undefined, revision: 3 },
          },
        },
      };
    });

    render(<BrowserRouter><RoutedDeckNavigator /></BrowserRouter>);
    openPageActions(1);
    fireEvent.click(screen.getByRole('menuitem', { name: '上移本页' }));

    await waitFor(() => expect(useProjectStore.getState().specByProjectId.p1.slide_specs.s2.subsection_id).toBeUndefined());
    expect(useDeckStore.getState().currentSlideId).toBe('s2');
    expect(new URLSearchParams(window.location.search).get('slide')).toBe('s2');

    await new Promise((resolve) => window.setTimeout(resolve, 20));
    expect(useDeckStore.getState().currentSlideId).toBe('s2');
    expect(new URLSearchParams(window.location.search).get('slide')).toBe('s2');
  });

  it('add page button calls slidesApi.add with last slide as anchor', async () => {
    const addSpy = vi.spyOn(slidesApi, 'add').mockResolvedValue({} as never);
    vi.spyOn(useProjectStore.getState(), 'loadProjectContent').mockResolvedValue();
    render(<DeckNavigator />);
    fireEvent.click(screen.getByRole('button', { name: /新增页面/ }));
    await waitFor(() => {
      expect(addSpy).toHaveBeenCalledWith('p1', { after_slide_id: 's2' });
    });
  });

  it('delete page opens confirm modal then calls slidesApi.remove', async () => {
    const removeSpy = vi.spyOn(slidesApi, 'remove').mockResolvedValue(undefined as never);
    const loadProjectContentSpy = vi.spyOn(useProjectStore.getState(), 'loadProjectContent').mockResolvedValue();

    render(<DeckNavigator />);
    openPageActions(0);
    fireEvent.click(screen.getByRole('menuitem', { name: '删除本页' }));

    // modal should be visible
    expect(screen.getByRole('heading', { name: '删除页面' })).toBeInTheDocument();
    expect(screen.getByText('「市场分析」')).toBeInTheDocument();
    expect(screen.getByText('不可撤销')).toBeInTheDocument();

    // click confirm
    fireEvent.click(screen.getByRole('button', { name: '删除' }));

    await waitFor(() => {
      expect(removeSpy).toHaveBeenCalledWith('s1');
      expect(loadProjectContentSpy).toHaveBeenCalledWith('p1');
    });
  });

  it('delete page does nothing when confirm is canceled', () => {
    const removeSpy = vi.spyOn(slidesApi, 'remove').mockResolvedValue(undefined as never);
    vi.spyOn(window, 'confirm').mockReturnValue(false);
    render(<DeckNavigator />);
    openPageActions(0);
    fireEvent.click(screen.getByRole('menuitem', { name: '删除本页' }));
    expect(removeSpy).not.toHaveBeenCalled();
  });
});
