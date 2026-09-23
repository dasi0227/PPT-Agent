import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Project, ProjectContentSnapshot, Theme } from '../../api/types';
import { useProjectStore } from '../../stores/projectStore';
import { useToastStore } from '../../stores/toastStore';
import { ComponentRepositoryPage } from './ComponentRepositoryPage';
import { SkillRepositoryPage } from './SkillRepositoryPage';
import { ThemeRepositoryPage } from './ThemeRepositoryPage';
import { clearThemeExampleCache } from './ThemePreview';

const mocks = vi.hoisted(() => ({
  themeExample: vi.fn(),
  listThemes: vi.fn(),
  getTheme: vi.fn(),
  listProjects: vi.fn(),
  listComponents: vi.fn(),
  getComponent: vi.fn(),
  setComponentDisabled: vi.fn(),
  updateComponent: vi.fn(),
  getSkill: vi.fn(),
  setSkillDisabled: vi.fn(),
  updateSkill: vi.fn(),
  deleteTheme: vi.fn(),
  deleteComponent: vi.fn(),
  deleteSkill: vi.fn(),
  listSkills: vi.fn(),
  setTheme: vi.fn(),
  getContent: vi.fn(),
  updateTheme: vi.fn(),
}));

vi.mock('../../api/repositories', () => ({
  repositoriesApi: {
    themeExample:mocks.themeExample,
    listThemes: mocks.listThemes,
    getTheme: mocks.getTheme,
    listComponents: mocks.listComponents,
    getComponent: mocks.getComponent,
    setComponentDisabled: mocks.setComponentDisabled,
    updateComponent: mocks.updateComponent,
    getSkill: mocks.getSkill,
    setSkillDisabled: mocks.setSkillDisabled,
    updateSkill: mocks.updateSkill,
    deleteTheme: mocks.deleteTheme,
    deleteComponent: mocks.deleteComponent,
    deleteSkill: mocks.deleteSkill,
    updateTheme: mocks.updateTheme,
  },
}));

vi.mock('../../api/skills', () => ({
  skillsApi: { list: mocks.listSkills },
}));

vi.mock('../../api/projects', () => ({
  projectsApi: {
    list: mocks.listProjects,
    create: vi.fn(),
    patch: vi.fn(),
    get: vi.fn(),
    getContent: mocks.getContent,
    mutate: vi.fn(),
    setTheme: mocks.setTheme,
    delete: vi.fn(),
  },
}));

function LocationProbe() {
  const location = useLocation();
  return <output aria-label="location">{location.pathname}{location.search}</output>;
}

function renderPage(page: React.ReactNode, initialEntry: string | { pathname: string; state?: unknown } = '/warehouse/theme') {
  return render(<MemoryRouter initialEntries={[initialEntry]}>{page}<LocationProbe /></MemoryRouter>);
}

function themeFixtures(): Theme[] {
  return [
    {
      id: 'editorial-serif',
      style_hash:'css-1', appearance:{hash:'appearance-1',theme_css_url:'/api/v1/themes/editorial-serif/css',chrome_tokens:{}},
      name: 'Editorial Serif',
      description: 'Clean grid',
      tags: ['minimal'],
      css: ':root{--color-bg:#fff;--color-fg:#111;--color-primary:#d0021b;--color-accent:#1c1c1c;--font-sans:Aptos;--font-serif:Georgia}',
      css_url: '/api/v1/themes/editorial-serif/css',
      open_url: 'vscode://file/themes/editorial-serif/theme.css',
    },
    {
      id: 'blueprint',
      style_hash:'css-2', appearance:{hash:'appearance-2',theme_css_url:'/api/v1/themes/blueprint/css',chrome_tokens:{}},
      name: 'Blueprint',
      description: 'Dark presentation',
      tags: ['cool'],
      css: ':root{--color-bg:#111;--color-fg:#eee;--color-primary:#7aa2f7;--color-accent:#bb9af7;--font-sans:Inter;--font-serif:Georgia}',
      css_url: '/api/v1/themes/blueprint/css',
      open_url: 'vscode://file/themes/blueprint/theme.css',
    },
  ];
}

function project(theme: string): Project {
  return {
    id: 'project-7',
    title: 'Project 7',
    work_dir: '/projects/project-7',
    theme,
    status: 'ready',
    design_path: 'design.json',
    outline_path: 'outline.json',

    created_at: 1,
    updated_at: theme === 'editorial-serif' ? 1 : 2,
  };
}

function projectContent(theme: string): ProjectContentSnapshot {
  return {
    appearance: null,
    hashes: { outline: "outline-hash", design: `design-${theme}` },
    manifest: { version: '5.0', project_id: 'project-7', title: 'Deck', goal: '', audience: '', language: 'zh-CN', requirements: [], prohibitions: [], canvas: { aspect_ratio: '16:9' }, numbering: { enabled: true, hidden_roles: [], format: 'number' }, created_at: 1, updated_at: 1 },
    outline: { version: '5.0', project_id: 'project-7', sections: [], created_at: 1, updated_at: 1 },
    design: { version: '5.0',  project_id: 'project-7', theme, direction: '', chrome: [], created_at: 1, updated_at: theme === 'editorial-serif' ? 1 : 2 },
    slides_by_id: {},
  };
}

describe('personal repository pages', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    clearThemeExampleCache();
    mocks.themeExample.mockImplementation(async (name:string)=>({html:`<main class="slide-stage">${name}</main>`}));
    mocks.getTheme.mockImplementation(async(id:string)=>themeFixtures().find(theme=>theme.id===id));
    vi.stubGlobal('IntersectionObserver', class { observe() {} disconnect() {} unobserve() {} });
    mocks.listProjects.mockResolvedValue([]);
    useToastStore.getState().clearToasts();
    useProjectStore.setState({
      projects: [],
      openProjectIds: [],
      activeProjectId: null,
      contentByProjectId: {},
      contentLoadingByProjectId: {},
      contentErrorByProjectId: {},
      mutationPendingByProjectId: {},
      loadingProjects: false,
      projectError: null,
    });
  });

  it('renders repository navigation and filters themes from API data', async () => {
    const themes = themeFixtures();
    mocks.listThemes.mockResolvedValue({ themes });
    mocks.getTheme.mockImplementation(async (id: string) => themes.find((theme) => theme.id === id));

    renderPage(<ThemeRepositoryPage />);

    const preview = await screen.findByTitle('Editorial Serif 主题预览');
    expect(preview).toHaveAttribute('sandbox', 'allow-scripts');
    expect(preview).toHaveAttribute('src', '/slide-runtime/index.html');
    expect(screen.getByRole('main')).toHaveClass('h-[100dvh]', 'overflow-hidden');
    expect(screen.queryByText('色板')).not.toBeInTheDocument();
    expect(screen.queryByText('字体')).not.toBeInTheDocument();
    const editButton = screen.getByRole('button', { name: '编辑Editorial Serif' });
    const deleteButton = screen.getByRole('button', { name: '删除Editorial Serif' });
    const fileLink = screen.getByRole('link', { name: '查看文件' });
    expect(editButton.nextElementSibling).toBe(deleteButton);
    expect(deleteButton.nextElementSibling).toBe(fileLink);
    expect(screen.getByRole('region', { name: '主题详情' }).querySelector('footer')).not.toBeInTheDocument();
    expect(screen.getByLabelText('标签')).toHaveTextContent('极简');
    expect(screen.queryByText('Aa')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '封面页' })).toBeInTheDocument();
    expect(screen.getAllByText('Clean grid')).toHaveLength(2);
    expect(document.querySelector('[data-repository-workspace]')).toBeInTheDocument();
    const minimalFilter = screen.getByRole('button', { name: '极简' });
    expect(minimalFilter.parentElement).toHaveClass('overflow-x-auto', 'scrollbar-none');
    expect(screen.getByText('仓库')).toHaveClass('text-base', 'font-bold', 'text-text-900');
    expect(screen.queryByText('个人仓库')).not.toBeInTheDocument();
    const repositoryBrand = screen.getByRole('button', { name: '返回项目' });
    expect(repositoryBrand).toHaveClass('px-2', 'text-base');
    expect(repositoryBrand.querySelector('img')).toHaveClass('h-10', 'w-10', 'rounded-sm');
    expect(screen.getByRole('link', { name: '组件' })).toHaveAttribute('href', '/warehouse/component');
    fireEvent.click(screen.getByRole('button', { name: /Blueprint/ }));
    expect(mocks.setTheme).not.toHaveBeenCalled();
    expect(useToastStore.getState().toasts).toEqual([]);
    fireEvent.click(screen.getByRole('button', { name: '内容页' }));
    await waitFor(()=>expect(mocks.themeExample).toHaveBeenCalledWith('content'));
    fireEvent.click(screen.getByRole('button', { name: '图表页' }));
    await waitFor(()=>expect(mocks.themeExample).toHaveBeenCalledWith('chart'));
    const activePreview = () => document.querySelector('[data-preview-active="true"] iframe');
    await waitFor(() => expect(activePreview()).toHaveAttribute('title', 'Blueprint 主题预览'));
    fireEvent.change(screen.getByLabelText('搜索主题'), { target: { value: 'dark' } });
    expect(activePreview()).toHaveAttribute('title', 'Blueprint 主题预览');
    expect(preview).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('搜索主题'), { target: { value: 'minimal' } });
    await waitFor(() => expect(activePreview()).toHaveAttribute('title', 'Editorial Serif 主题预览'));
    fireEvent.click(screen.getByRole('button', { name: '封面页' }));
    expect(activePreview()).toBe(preview);
  });

  it('retains the selected preview while revalidating after a return or refresh', async () => {
    const themes = themeFixtures();
    mocks.listThemes.mockResolvedValue({ themes });
    const { rerender } = render(<MemoryRouter><ThemeRepositoryPage active /></MemoryRouter>);
    await screen.findByTitle('Editorial Serif 主题预览');
    fireEvent.click(screen.getByRole('button', { name: /Blueprint/ }));
    const preview = await screen.findByTitle('Blueprint 主题预览');
    let finish!: (value: { themes: Theme[] }) => void;
    mocks.listThemes.mockImplementation(() => new Promise(resolve => { finish = resolve; }));
    rerender(<MemoryRouter><ThemeRepositoryPage active={false} /></MemoryRouter>);
    rerender(<MemoryRouter><ThemeRepositoryPage active /></MemoryRouter>);
    expect(screen.getByTitle('Blueprint 主题预览')).toBe(preview);
    finish({ themes: themes.map(theme => ({ ...theme })) });
    await waitFor(() => expect(mocks.listThemes).toHaveBeenCalledTimes(2));
    expect(screen.getByTitle('Blueprint 主题预览')).toBe(preview);
    fireEvent.click(screen.getByRole('button', { name: '刷新仓库' }));
    expect(screen.getByTitle('Blueprint 主题预览')).toBe(preview);
    finish({ themes });
  });

  it('applies the previewed theme to the active project and synchronizes project state', async () => {
    const themes = themeFixtures();
    mocks.listThemes.mockResolvedValue({ themes });
    mocks.getTheme.mockImplementation(async (id: string) => themes.find((theme) => theme.id === id));
    mocks.setTheme.mockResolvedValue(project('blueprint'));
    mocks.getContent.mockResolvedValue(projectContent('blueprint'));
    useProjectStore.setState({
      projects: [project('editorial-serif')],
      openProjectIds: ['project-7'],
      activeProjectId: null,
      contentByProjectId: { 'project-7': projectContent('editorial-serif') },
    });

    renderPage(<ThemeRepositoryPage />, {
      pathname: '/warehouse/theme',
      state: { returnTo: '/projects/project-7?slide=slide-2' },
    });

    const currentButton = await screen.findByRole('button', { name: '已保存主题' });
    expect(currentButton).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: /Blueprint/ }));
    expect(mocks.setTheme).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: '应用主题' }));

    await waitFor(() => expect(mocks.setTheme).toHaveBeenCalledWith('project-7', 'blueprint'));
    await waitFor(() => expect(screen.getByRole('button', { name: '已保存主题' })).toBeDisabled());
    expect(useProjectStore.getState().projects[0]).toMatchObject({
      id: 'project-7',
      theme: 'blueprint',
      });
    expect(useProjectStore.getState().contentByProjectId['project-7'].design).toMatchObject({
      theme: 'blueprint',
      });
    expect(useToastStore.getState().toasts).toEqual([
      expect.objectContaining({ message: '已保存「Blueprint」主题，画布将加载新外观', tone: 'success' }),
    ]);
  });

  it('edits theme metadata and tags in a dialog', async () => {
    const themes = themeFixtures();
    const updated = {
      ...themes[0],
      name: 'Swiss Compact',
      description: 'Tighter presentation grid',
      tags: ['minimal', 'business'] as Theme['tags'],
    };
    mocks.listThemes.mockResolvedValue({ themes });
    mocks.getTheme.mockImplementation(async (id: string) => themes.find((theme) => theme.id === id));
    mocks.updateTheme.mockResolvedValue(updated);

    renderPage(<ThemeRepositoryPage />);

    fireEvent.click(await screen.findByRole('button', { name: '编辑Editorial Serif' }));
    const dialog = screen.getByRole('dialog');
    expect(dialog).toHaveTextContent('编辑主题');
    fireEvent.change(within(dialog).getByLabelText('名称'), { target: { value: updated.name } });
    fireEvent.change(within(dialog).getByLabelText('描述'), { target: { value: updated.description } });
    fireEvent.click(within(dialog).getByRole('button', { name: '商务' }));
    fireEvent.click(within(dialog).getByRole('button', { name: '保存' }));

    await waitFor(() => expect(mocks.updateTheme).toHaveBeenCalledWith('editorial-serif', {
      name: updated.name,
      description: updated.description,
      tags: updated.tags,
    }));
    expect(await screen.findByRole('heading', { name: updated.name })).toBeInTheDocument();
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('keeps the preview and original project theme when applying fails, and blocks duplicate submissions', async () => {
    const themes = themeFixtures();
    let rejectRequest: (reason: Error) => void = () => undefined;
    const request = new Promise<Project>((_, reject) => { rejectRequest = reject; });
    mocks.listThemes.mockResolvedValue({ themes });
    mocks.getTheme.mockImplementation(async (id: string) => themes.find((theme) => theme.id === id));
    mocks.setTheme.mockReturnValue(request);
    useProjectStore.setState({
      projects: [project('editorial-serif')],
      openProjectIds: ['project-7'],
      activeProjectId: 'project-7',
      contentByProjectId: { 'project-7': projectContent('editorial-serif') },
    });

    renderPage(<ThemeRepositoryPage />);

    await screen.findByRole('button', { name: '已保存主题' });
    fireEvent.click(screen.getByRole('button', { name: /Blueprint/ }));
    const applyButton = screen.getByRole('button', { name: '应用主题' });
    fireEvent.click(applyButton);
    fireEvent.click(applyButton);

    expect(mocks.setTheme).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('button', { name: '应用中' })).toBeDisabled();
    rejectRequest(new Error('theme write failed'));

    await waitFor(() => expect(screen.getByRole('button', { name: '应用主题' })).toBeEnabled());
    expect(screen.getByTitle('Blueprint 主题预览')).toBeInTheDocument();
    expect(useProjectStore.getState().projects[0].theme).toBe('editorial-serif');
    expect(useProjectStore.getState().contentByProjectId['project-7'].design.theme).toBe('editorial-serif');
    expect(useToastStore.getState().toasts).toEqual([
      expect.objectContaining({ message: 'theme write failed', tone: 'error' }),
    ]);
  });

  it('returns to the project route used to enter the warehouse', async () => {
    mocks.listThemes.mockResolvedValue({ themes: [] });
    renderPage(<ThemeRepositoryPage />, {
      pathname: '/warehouse/theme',
      state: { returnTo: '/projects/project-7?slide=slide-2' },
    });

    await screen.findByText('没有匹配的主题');
    fireEvent.click(screen.getByRole('link', { name: '返回主页' }));
    expect(screen.getByLabelText('location')).toHaveTextContent('/projects/project-7?slide=slide-2');
  });

  it('filters components by tags and previews HTML only in sandboxed iframes', async () => {
    vi.stubGlobal('IntersectionObserver', undefined);
    const components = [
      {
        id: 'feature-card',
        name: 'Feature Card',
        description: 'Feature summary',
        tags: ['card'],
        disabled: false,
        html: '<article><h2>Feature</h2><script>window.parent.bad=true</script></article>',
        open_url: 'vscode://file/components/feature-card/index.html',
      },
      {
        id: 'quote-block',
        name: 'Quote Block',
        description: 'Editorial quote',
        tags: ['other'],
        disabled: false,
        html: '<blockquote>Quote</blockquote>',
        open_url: 'vscode://file/components/quote-block/index.html',
      },
    ];
    mocks.listComponents.mockResolvedValue({ components });
    mocks.getComponent.mockImplementation(async (id: string) => components.find((component) => component.id === id));

    renderPage(<ComponentRepositoryPage />);

    const detail = await screen.findByTitle('Feature Card 组件预览');
    const thumbnail = screen.getByTitle('Feature Card 缩略预览');
    expect(thumbnail).toHaveAttribute('srcdoc');
    expect(mocks.getTheme).not.toHaveBeenCalled();
    expect(mocks.listThemes).not.toHaveBeenCalled();
    expect(detail.getAttribute('srcdoc')).toContain(components[0].html);
    expect(detail.getAttribute('srcdoc')).not.toContain('/api/v1/themes/');
    expect(detail.getAttribute('srcdoc')).not.toContain('/api/v1/runtime/base.css');
    expect(screen.getByPlaceholderText('搜索组件')).toBeInTheDocument();
    expect(detail).toHaveAttribute('sandbox', 'allow-scripts');
    expect(detail.closest('.aspect-video')).toBeInTheDocument();
    expect(document.querySelector('[data-repository-workspace]')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '卡片' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '其它' }));
    expect(screen.getByTitle('Quote Block 组件预览')).toHaveAttribute('sandbox', 'allow-scripts');
    expect(detail.closest('[data-preview-active]')).toHaveAttribute('aria-hidden', 'true');
    fireEvent.click(screen.getByRole('button', { name: '卡片' }));
    expect(screen.getByTitle('Feature Card 组件预览')).toBe(detail);
    expect(detail.closest('[data-preview-active]')).toHaveAttribute('data-preview-active', 'true');
  });

  it('rolls back a failed optimistic Skill toggle and reports the error', async () => {
    const skill = {
      id: 'story',
      name: '演示叙事',
      description: '梳理页面叙事。',
      content: '# 演示叙事',
      tags: ['methodology'],
      disabled: false,
      open_url: 'vscode://file/skills/story/SKILL.md',
    };
    mocks.listSkills.mockResolvedValue({ skills: [skill] });
    mocks.getSkill.mockResolvedValue(skill);
    mocks.setSkillDisabled.mockRejectedValue(new Error('registry write failed'));

    renderPage(<SkillRepositoryPage />);

    const toggle = await screen.findByRole('switch', { name: '切换技能状态' });
    expect(document.querySelector('[data-repository-workspace]')).toBeInTheDocument();
    expect(screen.queryByText('SKILL.md')).not.toBeInTheDocument();
    expect(toggle).toHaveAttribute('aria-checked', 'true');
    fireEvent.click(toggle);
    await waitFor(() => expect(mocks.setSkillDisabled).toHaveBeenCalledWith('story', true));
    await waitFor(() => expect(toggle).toHaveAttribute('aria-checked', 'true'));
    expect(useToastStore.getState().toasts).toEqual([
      expect.objectContaining({ message: 'registry write failed', tone: 'error' }),
    ]);
  });

  it('confirms component deletion and removes it from the directory', async () => {
    const component = {
      id: 'feature-card',
      name: 'Feature Card',
      description: 'Feature summary',
      tags: ['card', 'metric', 'chart', 'table', 'list'],
      disabled: false,
      html: '<article><h2>Feature</h2></article>',
      open_url: 'vscode://file/components/feature-card/index.html',
    };
    mocks.listComponents.mockResolvedValue({ components: [component] });
    mocks.getComponent.mockResolvedValue(component);
    mocks.deleteComponent.mockResolvedValue(undefined);

    renderPage(<ComponentRepositoryPage />);

    await screen.findByTitle('Feature Card 组件预览');
    expect(screen.getByLabelText('标签')).toHaveClass('flex-nowrap', 'overflow-x-auto', 'scrollbar-none');
    expect(screen.getByLabelText('标签').children).toHaveLength(5);
    fireEvent.click(screen.getByRole('button', { name: '删除Feature Card' }));
    expect(screen.getByRole('dialog')).toHaveTextContent('确定删除「Feature Card」吗？');
    fireEvent.click(screen.getByRole('button', { name: '删除' }));

    await waitFor(() => expect(mocks.deleteComponent).toHaveBeenCalledWith('feature-card'));
    await waitFor(() => expect(screen.queryByText('Feature Card')).not.toBeInTheDocument());
  });
});
