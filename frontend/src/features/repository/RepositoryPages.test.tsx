import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Project, ProjectContentSnapshot, Theme } from '../../api/types';
import { useProjectStore } from '../../stores/projectStore';
import { useToastStore } from '../../stores/toastStore';
import { ComponentRepositoryPage } from './ComponentRepositoryPage';
import { SkillRepositoryPage } from './SkillRepositoryPage';
import { ThemeRepositoryPage } from './ThemeRepositoryPage';

const mocks = vi.hoisted(() => ({
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
  updateTheme: vi.fn(),
}));

vi.mock('../../api/repositories', () => ({
  repositoriesApi: {
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
    getContent: vi.fn(),
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
      id: 'swiss-modern',
      name: 'Swiss Modern',
      description: 'Clean grid',
      tags: ['minimal'],
      css: ':root{--color-bg:#fff;--color-fg:#111;--color-primary:#d0021b;--color-accent:#1c1c1c;--font-sans:Aptos;--font-serif:Georgia}',
      css_url: '/api/v1/themes/swiss-modern/css',
      open_url: 'vscode://file/themes/swiss-modern/theme.css',
    },
    {
      id: 'tokyo-night',
      name: 'Tokyo Night',
      description: 'Dark presentation',
      tags: ['cool'],
      css: ':root{--color-bg:#111;--color-fg:#eee;--color-primary:#7aa2f7;--color-accent:#bb9af7;--font-sans:Inter;--font-serif:Georgia}',
      css_url: '/api/v1/themes/tokyo-night/css',
      open_url: 'vscode://file/themes/tokyo-night/theme.css',
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
    outline_revision: 1,
    design_revision: theme === 'swiss-modern' ? 1 : 2,
    created_at: 1,
    updated_at: theme === 'swiss-modern' ? 1 : 2,
  };
}

function projectContent(theme: string): ProjectContentSnapshot {
  return {
    manifest: { version: '4.0', revision: 1, project_id: 'project-7', title: 'Deck', goal: '', audience: '', language: 'zh-CN', requirements: [], prohibitions: [], canvas: { aspect_ratio: '16:9' }, numbering: { enabled: true, hidden_roles: [], format: 'number' }, created_at: 1, updated_at: 1 },
    outline: { version: '4.0', revision: 1, project_id: 'project-7', sections: [], created_at: 1, updated_at: 1 },
    design: { version: '4.0', revision: theme === 'swiss-modern' ? 1 : 2, project_id: 'project-7', theme, direction: '', density: 'medium', chrome: [], created_at: 1, updated_at: theme === 'swiss-modern' ? 1 : 2 },
    slides_by_id: {},
  };
}

describe('personal repository pages', () => {
  beforeEach(() => {
    vi.clearAllMocks();
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

    const preview = await screen.findByTitle('Swiss Modern 主题预览');
    expect(preview).toHaveAttribute('sandbox', '');
    expect(preview.getAttribute('srcdoc')).toContain('IDEA<br>TO<br>SLIDES');
    expect(preview.getAttribute('srcdoc')).toContain('class="specimen-word">Dasi');
    expect(preview.getAttribute('srcdoc')).toContain('transform:translateY(-2%)');
    expect(preview.getAttribute('srcdoc')).not.toContain('>Aa<');
    expect(screen.getByRole('main')).toHaveClass('h-[100dvh]', 'overflow-hidden');
    expect(screen.queryByText('色板')).not.toBeInTheDocument();
    expect(screen.queryByText('字体')).not.toBeInTheDocument();
    const editButton = screen.getByRole('button', { name: '编辑Swiss Modern' });
    const deleteButton = screen.getByRole('button', { name: '删除Swiss Modern' });
    const fileLink = screen.getByRole('link', { name: '查看文件' });
    expect(editButton.nextElementSibling).toBe(deleteButton);
    expect(deleteButton.nextElementSibling).toBe(fileLink);
    expect(screen.getByRole('region', { name: '主题详情' }).querySelector('footer')).not.toBeInTheDocument();
    expect(screen.getByLabelText('标签')).toHaveTextContent('极简');
    expect(screen.getAllByText('Dasi')).toHaveLength(2);
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
    fireEvent.click(screen.getByRole('button', { name: /Tokyo Night/ }));
    expect(mocks.setTheme).not.toHaveBeenCalled();
    expect(useToastStore.getState().toasts).toEqual([]);
    fireEvent.click(screen.getByRole('button', { name: '内容页' }));
    expect(screen.getByTitle('Tokyo Night 主题预览').getAttribute('srcdoc')).toContain('好页面先回答一个问题');
    expect(screen.getByTitle('Tokyo Night 主题预览').getAttribute('srcdoc')).not.toContain('<main class="showcase chart-page">');
    fireEvent.click(screen.getByRole('button', { name: '图表页' }));
    expect(screen.getByTitle('Tokyo Night 主题预览').getAttribute('srcdoc')).toContain('交付节奏持续提升');
    expect(screen.getByTitle('Tokyo Night 主题预览').getAttribute('srcdoc')).toContain('accent-bar');
    fireEvent.change(screen.getByLabelText('搜索主题'), { target: { value: 'dark' } });
    expect(screen.getByTitle('Tokyo Night 主题预览')).toBeInTheDocument();
    expect(screen.queryByTitle('Swiss Modern 主题预览')).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('搜索主题'), { target: { value: 'minimal' } });
    expect(screen.getByTitle('Swiss Modern 主题预览')).toBeInTheDocument();
  });

  it('applies the previewed theme to the active project and synchronizes project state', async () => {
    const themes = themeFixtures();
    mocks.listThemes.mockResolvedValue({ themes });
    mocks.getTheme.mockImplementation(async (id: string) => themes.find((theme) => theme.id === id));
    mocks.setTheme.mockResolvedValue(project('tokyo-night'));
    useProjectStore.setState({
      projects: [project('swiss-modern')],
      openProjectIds: ['project-7'],
      activeProjectId: null,
      contentByProjectId: { 'project-7': projectContent('swiss-modern') },
    });

    renderPage(<ThemeRepositoryPage />, {
      pathname: '/warehouse/theme',
      state: { returnTo: '/projects/project-7?slide=slide-2' },
    });

    const currentButton = await screen.findByRole('button', { name: '当前主题' });
    expect(currentButton).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: /Tokyo Night/ }));
    expect(mocks.setTheme).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole('button', { name: '应用主题' }));

    await waitFor(() => expect(mocks.setTheme).toHaveBeenCalledWith('project-7', 'tokyo-night'));
    await waitFor(() => expect(screen.getByRole('button', { name: '当前主题' })).toBeDisabled());
    expect(useProjectStore.getState().projects[0]).toMatchObject({
      id: 'project-7',
      theme: 'tokyo-night',
      design_revision: 2,
    });
    expect(useProjectStore.getState().contentByProjectId['project-7'].design).toMatchObject({
      theme: 'tokyo-night',
      revision: 2,
    });
    expect(useToastStore.getState().toasts).toEqual([
      expect.objectContaining({ message: '已应用「Tokyo Night」主题', tone: 'success' }),
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

    fireEvent.click(await screen.findByRole('button', { name: '编辑Swiss Modern' }));
    const dialog = screen.getByRole('dialog');
    expect(dialog).toHaveTextContent('编辑主题');
    fireEvent.change(within(dialog).getByLabelText('名称'), { target: { value: updated.name } });
    fireEvent.change(within(dialog).getByLabelText('描述'), { target: { value: updated.description } });
    fireEvent.click(within(dialog).getByRole('button', { name: '商务' }));
    fireEvent.click(within(dialog).getByRole('button', { name: '保存' }));

    await waitFor(() => expect(mocks.updateTheme).toHaveBeenCalledWith('swiss-modern', {
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
      projects: [project('swiss-modern')],
      openProjectIds: ['project-7'],
      activeProjectId: 'project-7',
      contentByProjectId: { 'project-7': projectContent('swiss-modern') },
    });

    renderPage(<ThemeRepositoryPage />);

    await screen.findByRole('button', { name: '当前主题' });
    fireEvent.click(screen.getByRole('button', { name: /Tokyo Night/ }));
    const applyButton = screen.getByRole('button', { name: '应用主题' });
    fireEvent.click(applyButton);
    fireEvent.click(applyButton);

    expect(mocks.setTheme).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('button', { name: '应用中' })).toBeDisabled();
    rejectRequest(new Error('theme write failed'));

    await waitFor(() => expect(screen.getByRole('button', { name: '应用主题' })).toBeEnabled());
    expect(screen.getByTitle('Tokyo Night 主题预览')).toBeInTheDocument();
    expect(useProjectStore.getState().projects[0].theme).toBe('swiss-modern');
    expect(useProjectStore.getState().contentByProjectId['project-7'].design.theme).toBe('swiss-modern');
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
    expect(screen.getByPlaceholderText('搜索组件')).toBeInTheDocument();
    expect(detail).toHaveAttribute('sandbox', '');
    expect(detail).toHaveAttribute('width', '960');
    expect(detail).toHaveAttribute('height', '540');
    expect(thumbnail).toHaveAttribute('width', '960');
    expect(thumbnail).toHaveAttribute('height', '540');
    expect(thumbnail.getAttribute('srcdoc')).toBe(detail.getAttribute('srcdoc'));
    expect(detail.getAttribute('srcdoc')).toContain('<script>window.parent.bad=true</script>');
    expect(detail.getAttribute('srcdoc')).toContain('.component-stage{display:grid;place-items:center');
    expect(detail.getAttribute('srcdoc')).toContain('--text-h1:36px');
    expect(detail.parentElement).toHaveClass('aspect-video');
    expect(document.querySelector('[data-repository-workspace]')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: '卡片' })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '其它' }));
    expect(screen.getByTitle('Quote Block 组件预览')).toHaveAttribute('sandbox', '');
    expect(screen.queryByTitle('Feature Card 组件预览')).not.toBeInTheDocument();
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
