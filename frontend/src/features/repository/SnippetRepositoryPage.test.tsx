import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Snippet } from '../../api/types';
import { useSnippetStore } from '../../stores/snippetStore';
import { SnippetRepositoryPage } from './SnippetRepositoryPage';

const mocks = vi.hoisted(() => ({
  list: vi.fn(),
  get: vi.fn(),
  update: vi.fn(),
  setDisabled: vi.fn(),
  delete: vi.fn(),
}));

vi.mock('../../api/snippets', () => ({ snippetsApi: mocks }));

const first: Snippet = {
  id: 'p1',
  name: '高管摘要 / Executive Summary',
  description: '提炼核心结论',
  content: '提炼核心结论。',
  tags: ['deliverable', 'review'],
  disabled: false, content_state: 'ready', open_url: 'vscode://file/snippets/p1/snippet.txt',
  created_at: 1,
  updated_at: 2,
};

function renderPage() {
  return render(<MemoryRouter><SnippetRepositoryPage /></MemoryRouter>);
}

describe('SnippetRepositoryPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    useSnippetStore.setState({
      snippets: [first],
      loading: false,
      loaded: true,
      error: '',
      version: 1,
    });
  });

  it('renders snippets with a unified bilingual name and an external file link', () => {
    renderPage();
    expect(screen.getByRole('link', { name: '短语' })).toHaveAttribute('href', '/warehouse/snippet');
    expect(screen.getByRole('heading', { name: first.name })).toBeInTheDocument();
    expect(screen.getAllByText('交付')).toHaveLength(2);
    expect(screen.getAllByText('审查')).toHaveLength(2);
    expect(screen.getByRole('link', { name: '查看文件' })).toHaveAttribute('href', first.open_url);
    expect(screen.getAllByText(first.description)).toHaveLength(2);
    const snippetContent = screen.getByText(first.content);
    expect(snippetContent).toHaveClass('text-lg', 'font-semibold');
    expect(snippetContent.closest('article')).toHaveClass('w-fit');
    expect(screen.getByRole('button', { name: `编辑${first.name}` }).nextElementSibling)
      .toBe(screen.getByRole('button', { name: `删除${first.name}` }));
  });

  it('edits metadata in a dialog and keeps the snippet value intact', async () => {
    const updated = { ...first, name: '新名称', description: '新描述', updated_at: 3 };
    mocks.update.mockResolvedValue(updated);
    renderPage();

    fireEvent.click(screen.getByRole('button', { name: `编辑${first.name}` }));
    const dialog = screen.getByRole('dialog');
    expect(dialog).toHaveTextContent('编辑短语');
    fireEvent.change(within(dialog).getByLabelText('名称'), { target: { value: updated.name } });
    fireEvent.change(within(dialog).getByLabelText('描述'), { target: { value: updated.description } });
    fireEvent.click(within(dialog).getByRole('button', { name: '保存' }));

    await waitFor(() => expect(mocks.update).toHaveBeenCalledWith(first.id, {
      name: updated.name,
      description: updated.description,
      tags: first.tags,
    }));
    expect(await screen.findByRole('heading', { name: updated.name })).toBeInTheDocument();
    expect(useSnippetStore.getState().snippets[0].content).toBe(first.content);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('toggles snippet availability from the detail header', async () => {
    const disabled = { ...first, disabled: true, updated_at: 3 };
    mocks.setDisabled.mockResolvedValue(disabled);
    renderPage();

    const toggle = screen.getByRole('switch', { name: '切换短语状态' });
    const directoryItem = screen.getByRole('button', { name: `${first.name}${first.description}` });
    expect(toggle).toHaveAttribute('aria-checked', 'true');
    expect(directoryItem.querySelector('.lucide-notebook-text')).toBeInTheDocument();
    fireEvent.click(toggle);

    await waitFor(() => expect(mocks.setDisabled).toHaveBeenCalledWith(first.id, true));
    await waitFor(() => expect(toggle).toHaveAttribute('aria-checked', 'false'));
    expect(directoryItem.querySelector('.lucide-pause')).toBeInTheDocument();
  });

  it('keeps a missing payload manageable', () => {
    useSnippetStore.setState({ snippets: [{ ...first, content: '', content_state: 'missing', content_error: '资源文件缺失' }] });
    renderPage();
    expect(screen.getByText('资源文件缺失')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: `编辑${first.name}` })).toBeEnabled();
    expect(screen.getByRole('button', { name: `删除${first.name}` })).toBeEnabled();
    expect(screen.getByRole('link', { name: '查看文件' })).toHaveAttribute('href', first.open_url);
  });

  it('filters by tag', async () => {
    renderPage();

    const identityButtons = screen.getAllByRole('button', { name: '身份' });
    fireEvent.click(identityButtons[identityButtons.length - 1]);

    expect(screen.getByText('没有匹配的短语')).toBeInTheDocument();
  });
});
