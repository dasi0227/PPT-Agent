import { fireEvent, render, screen, waitFor } from '@testing-library/react';
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
  delete: vi.fn(),
}));

vi.mock('../../api/prompts', () => ({ promptsApi: mocks }));

const first: Prompt = {
  id: 'p1',
  key_zh: '高管摘要',
  key_en: 'executive-summary',
  value: '提炼核心结论。',
  tags: ['summarize', 'rewrite'],
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

  it('renders the fourth repository with equal bilingual keys and no file link', () => {
    renderPage();
    expect(screen.getByRole('link', { name: '提示词' })).toHaveAttribute('href', '/warehouse/prompt');
    expect(screen.getByRole('heading', { name: '高管摘要 / executive-summary' })).toBeInTheDocument();
    expect(screen.getAllByText('总结')).toHaveLength(2);
    expect(screen.getAllByText('改写')).toHaveLength(2);
    expect(screen.queryByRole('link', { name: '查看文件' })).not.toBeInTheDocument();
    expect(screen.getAllByText('提炼核心结论。')).toHaveLength(2);
  });

  it('edits in place and updates the shared cache after saving', async () => {
    const updated = { ...first, value: '更新后的提示词。', updated_at: 3 };
    mocks.update.mockResolvedValue(updated);
    renderPage();

    fireEvent.click(screen.getByRole('button', { name: '编辑' }));
    fireEvent.change(screen.getByLabelText('提示词 value'), { target: { value: updated.value } });
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(mocks.update).toHaveBeenCalledWith(first.id, {
      key_zh: first.key_zh,
      key_en: first.key_en,
      value: updated.value,
      tags: first.tags,
    }));
    expect(await screen.findAllByText(updated.value)).toHaveLength(2);
    expect(usePromptStore.getState().prompts[0].value).toBe(updated.value);
  });

  it('filters by tag and creates prompts from the inline form', async () => {
    const created: Prompt = {
      id: 'p2',
      key_zh: '数据分析',
      key_en: 'data-analysis',
      value: '分析数据。',
      tags: ['data'],
      created_at: 3,
      updated_at: 3,
    };
    mocks.create.mockResolvedValue(created);
    renderPage();

    {
      const dataButtons = screen.getAllByRole('button', { name: '数据' });
      fireEvent.click(dataButtons[dataButtons.length - 1]);
    }
    expect(screen.getByText('没有匹配的提示词')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '全部' }));
    fireEvent.click(screen.getByRole('button', { name: '新建提示词' }));
    fireEvent.change(screen.getByLabelText('中文 key'), { target: { value: created.key_zh } });
    fireEvent.change(screen.getByLabelText('英文 key'), { target: { value: created.key_en } });
    fireEvent.change(screen.getByLabelText('提示词 value'), { target: { value: created.value } });
    {
      const dataButtons = screen.getAllByRole('button', { name: '数据' });
      fireEvent.click(dataButtons[dataButtons.length - 1]);
    }
    fireEvent.click(screen.getByRole('button', { name: '保存' }));

    await waitFor(() => expect(mocks.create).toHaveBeenCalledWith({
      key_zh: created.key_zh,
      key_en: created.key_en,
      value: created.value,
      tags: ['data'],
    }));
    expect(await screen.findByRole('heading', { name: '数据分析 / data-analysis' })).toBeInTheDocument();
  });
});
