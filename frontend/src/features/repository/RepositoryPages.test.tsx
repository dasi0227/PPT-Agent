import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useToastStore } from '../../stores/toastStore';
import { ComponentRepositoryPage } from './ComponentRepositoryPage';
import { SkillRepositoryPage } from './SkillRepositoryPage';
import { ThemeRepositoryPage } from './ThemeRepositoryPage';

const mocks = vi.hoisted(() => ({
  listThemes: vi.fn(),
  getTheme: vi.fn(),
  listComponents: vi.fn(),
  getComponent: vi.fn(),
  getSkill: vi.fn(),
  setSkillDisabled: vi.fn(),
  listSkills: vi.fn(),
}));

vi.mock('../../api/repositories', () => ({
  repositoriesApi: {
    listThemes: mocks.listThemes,
    getTheme: mocks.getTheme,
    listComponents: mocks.listComponents,
    getComponent: mocks.getComponent,
    getSkill: mocks.getSkill,
    setSkillDisabled: mocks.setSkillDisabled,
  },
}));

vi.mock('../../api/skills', () => ({
  skillsApi: { list: mocks.listSkills },
}));

vi.mock('../../api/projects', () => ({
  projectsApi: {
    list: vi.fn(),
    create: vi.fn(),
    patch: vi.fn(),
    get: vi.fn(),
    getContent: vi.fn(),
    mutate: vi.fn(),
    setTheme: vi.fn(),
    delete: vi.fn(),
  },
}));

function renderPage(page: React.ReactNode) {
  return render(<MemoryRouter>{page}</MemoryRouter>);
}

describe('personal repository pages', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useToastStore.getState().clearToasts();
  });

  it('renders repository navigation and filters themes from API data', async () => {
    const themes = [
      {
        id: 'swiss-modern',
        name: 'Swiss Modern',
        description: 'Clean grid',
        tags: ['minimal'],
        css: ':root{--color-bg:#fff;--font-sans:Aptos}',
        css_url: '/api/v1/themes/swiss-modern/css',
        open_url: 'vscode://file/themes/swiss-modern/theme.css',
      },
      {
        id: 'tokyo-night',
        name: 'Tokyo Night',
        description: 'Dark presentation',
        tags: ['dark'],
        css: ':root{--color-bg:#111;--font-sans:Inter}',
        css_url: '/api/v1/themes/tokyo-night/css',
        open_url: 'vscode://file/themes/tokyo-night/theme.css',
      },
    ];
    mocks.listThemes.mockResolvedValue({ themes });
    mocks.getTheme.mockImplementation(async (id: string) => themes.find((theme) => theme.id === id));

    renderPage(<ThemeRepositoryPage />);

    expect(await screen.findByTitle('Swiss Modern 主题预览')).toHaveAttribute('sandbox', '');
    expect(screen.getByRole('link', { name: '组件' })).toHaveAttribute('href', '/warehouse/component');
    fireEvent.change(screen.getByLabelText('搜索主题'), { target: { value: 'dark' } });
    expect(screen.getByTitle('Tokyo Night 主题预览')).toBeInTheDocument();
    expect(screen.queryByTitle('Swiss Modern 主题预览')).not.toBeInTheDocument();
  });

  it('filters component kinds and previews HTML only in sandboxed iframes', async () => {
    const components = [
      {
        id: 'feature-card',
        name: 'Feature Card',
        description: 'Feature summary',
        tags: ['card'],
        kind: 'content',
        html: '<article><h2>Feature</h2><script>window.parent.bad=true</script></article>',
        open_url: 'vscode://file/components/feature-card/index.html',
      },
      {
        id: 'quote-block',
        name: 'Quote Block',
        description: 'Editorial quote',
        tags: ['quote'],
        kind: 'editorial',
        html: '<blockquote>Quote</blockquote>',
        open_url: 'vscode://file/components/quote-block/index.html',
      },
    ];
    mocks.listComponents.mockResolvedValue({ components });
    mocks.getComponent.mockImplementation(async (id: string) => components.find((component) => component.id === id));

    renderPage(<ComponentRepositoryPage />);

    const detail = await screen.findByTitle('Feature Card 组件预览');
    expect(detail).toHaveAttribute('sandbox', '');
    expect(detail.getAttribute('srcdoc')).toContain('<script>window.parent.bad=true</script>');
    fireEvent.click(screen.getByRole('button', { name: 'editorial' }));
    expect(screen.getByTitle('Quote Block 组件预览')).toHaveAttribute('sandbox', '');
    expect(screen.queryByTitle('Feature Card 组件预览')).not.toBeInTheDocument();
  });

  it('rolls back a failed optimistic Skill toggle and reports the error', async () => {
    const skill = {
      id: 'story',
      name: '演示叙事',
      description: '梳理页面叙事。',
      content: '# 演示叙事',
      disabled: false,
      open_url: 'vscode://file/skills/story/SKILL.md',
    };
    mocks.listSkills.mockResolvedValue({ skills: [skill] });
    mocks.getSkill.mockResolvedValue(skill);
    mocks.setSkillDisabled.mockRejectedValue(new Error('registry write failed'));

    renderPage(<SkillRepositoryPage />);

    const toggle = await screen.findByRole('switch', { name: '切换技能状态' });
    expect(toggle).toHaveAttribute('aria-checked', 'true');
    fireEvent.click(toggle);
    await waitFor(() => expect(mocks.setSkillDisabled).toHaveBeenCalledWith('story', true));
    await waitFor(() => expect(toggle).toHaveAttribute('aria-checked', 'true'));
    expect(useToastStore.getState().toasts).toEqual([
      expect.objectContaining({ message: 'registry write failed', tone: 'error' }),
    ]);
  });
});
