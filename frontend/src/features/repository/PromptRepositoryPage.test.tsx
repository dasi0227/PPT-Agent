import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Prompt } from '../../api/types';
import { usePromptStore } from '../../stores/promptStore';
import { PromptRepositoryPage } from './PromptRepositoryPage';

const mocks = vi.hoisted(() => ({
  list: vi.fn(),
  get: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  setDisabled: vi.fn(),
  delete: vi.fn(),
}));

vi.mock('../../api/prompts', () => ({ promptsApi: mocks }));

const first: Prompt = {
  id: 'p1',
  name: '高管摘要 / Executive Summary',
  desc: '提炼核心结论',
  value: '提炼核心结论。',
  tags: ['deliverable', 'review'],
  disabled: false,
  created_at: 1,
  updated_at: 2,
};

function renderPage() {
  return render(<MemoryRouter><PromptRepositoryPage /></MemoryRouter>);
}

describe('PromptRepositoryPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    usePromptStore.setState({
      prompts: [first],
      recentIds: [],
      loading: false,
      loaded: true,
      error: '',
      version: 1,
    });
  });

  it('renders prompts with a unified bilingual name and no file link', () => {
    renderPage();
    expect(screen.getByRole('link', { name: '提示词' })).toHaveAttribute('href', '/warehouse/prompt');
    expect(screen.getByRole('heading', { name: first.name })).toBeInTheDocument();
    expect(screen.getAllByText('交付')).toHaveLength(2);
    expect(screen.getAllByText('审查')).toHaveLength(2);
    expect(screen.queryByRole('link', { name: '查看文件' })).not.toBeInTheDocument();
    expect(screen.getAllByText(first.desc)).toHaveLength(2);
    const promptValue = screen.getByText(first.value);
    expect(promptValue).toHaveClass('text-lg', 'font-semibold');
    expect(promptValue.closest('article')).toHaveClass('w-fit');
    expect(screen.getByRole('button', { name: `编辑${first.name}` }).nextElementSibling)
      .toBe(screen.getByRole('button', { name: `删除${first.name}` }));
  });

  it('edits in a dialog and updates the shared cache after saving', async () => {
    const updated = { ...first, value: '更新后的提示词。', updated_at: 3 };
    mocks.update.mockResolvedValue(updated);
    renderPage();

    fireEvent.click(screen.getByRole('button', { name: `编辑${first.name}` }));
    const dialog = screen.getByRole('dialog');
    expect(dialog).toHaveTextContent('编辑提示词');
    fireEvent.change(within(dialog).getByLabelText('提示词 value'), { target: { value: updated.value } });
    fireEvent.click(within(dialog).getByRole('button', { name: '保存' }));

    await waitFor(() => expect(mocks.update).toHaveBeenCalledWith(first.id, {
      name: first.name,
      desc: first.desc,
      value: updated.value,
      tags: first.tags,
    }));
    expect(await screen.findByText(updated.value)).toBeInTheDocument();
    expect(usePromptStore.getState().prompts[0].value).toBe(updated.value);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('toggles prompt availability from the detail header', async () => {
    const disabled = { ...first, disabled: true, updated_at: 3 };
    mocks.setDisabled.mockResolvedValue(disabled);
    renderPage();

    const toggle = screen.getByRole('switch', { name: '切换提示词状态' });
    const directoryItem = screen.getByRole('button', { name: `${first.name}${first.desc}` });
    expect(toggle).toHaveAttribute('aria-checked', 'true');
    expect(directoryItem.querySelector('.lucide-notebook-text')).toBeInTheDocument();
    fireEvent.click(toggle);

    await waitFor(() => expect(mocks.setDisabled).toHaveBeenCalledWith(first.id, true));
    await waitFor(() => expect(toggle).toHaveAttribute('aria-checked', 'false'));
    expect(directoryItem.querySelector('.lucide-pause')).toBeInTheDocument();
  });

  it('filters by tag and creates prompts from the inline form', async () => {
    const created: Prompt = {
      id: 'p2',
      name: '数据分析 / Data Analysis',
      desc: '分析数据并提炼洞察',
      value: '分析数据。',
      tags: ['identity'],
      disabled: false,
      created_at: 3,
      updated_at: 3,
    };
    mocks.create.mockResolvedValue(created);
    renderPage();

    {
      const identityButtons = screen.getAllByRole('button', { name: '身份' });
      fireEvent.click(identityButtons[identityButtons.length - 1]);
    }
    expect(screen.getByText('没有匹配的提示词')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '全部' }));
    fireEvent.click(screen.getByRole('button', { name: '新建提示词' }));
    fireEvent.change(screen.getByLabelText('名称'), { target: { value: created.name } });
    fireEvent.change(screen.getByLabelText('描述'), { target: { value: created.desc } });
    fireEvent.change(screen.getByLabelText('提示词 value'), { target: { value: created.value } });
    {
      const identityButtons = screen.getAllByRole('button', { name: '身份' });
      fireEvent.click(identityButtons[identityButtons.length - 1]);
    }
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(mocks.create).toHaveBeenCalledWith({
      name: created.name,
      desc: created.desc,
      value: created.value,
      tags: ['identity'],
    }));
    expect(await screen.findByRole('heading', { name: created.name })).toBeInTheDocument();
  });
});
