import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Prompt } from '../api/types';
import { PROMPT_RECENT_STORAGE_KEY, usePromptStore } from './promptStore';

const mocks = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  setDisabled: vi.fn(),
  delete: vi.fn(),
}));

vi.mock('../api/prompts', () => ({ promptsApi: mocks }));

const prompt = (id: string): Prompt => ({
  id,
  key_zh: `提示${id}`,
  key_en: `prompt-${id}`,
  value: `value ${id}`,
  tags: ['deliverable'],
  disabled: false,
  created_at: 1,
  updated_at: 1,
});

describe('promptStore', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    usePromptStore.setState({
      prompts: [],
      recentIds: [],
      loading: false,
      loaded: false,
      error: '',
      version: 0,
    });
  });

  it('loads prompts and removes stale recent IDs', async () => {
    localStorage.setItem(PROMPT_RECENT_STORAGE_KEY, JSON.stringify(['p2', 'missing', 'p1']));
    usePromptStore.setState({ recentIds: ['p2', 'missing', 'p1'] });
    mocks.list.mockResolvedValue({ prompts: [prompt('p1'), prompt('p2')] });

    await usePromptStore.getState().load();

    expect(usePromptStore.getState()).toMatchObject({
      prompts: [prompt('p1'), prompt('p2')],
      recentIds: ['p2', 'p1'],
      loaded: true,
      loading: false,
    });
    expect(JSON.parse(localStorage.getItem(PROMPT_RECENT_STORAGE_KEY) ?? '[]')).toEqual(['p2', 'p1']);
  });

  it('updates cache only after CRUD requests succeed', async () => {
    const first = prompt('p1');
    const updated = { ...first, value: 'updated', updated_at: 2 };
    mocks.create.mockResolvedValue(first);
    mocks.update.mockResolvedValue(updated);
    mocks.delete.mockResolvedValue(undefined);

    await usePromptStore.getState().create({
      key_zh: first.key_zh, key_en: first.key_en, value: first.value, tags: first.tags,
    });
    await usePromptStore.getState().update(first.id, {
      key_zh: updated.key_zh, key_en: updated.key_en, value: updated.value, tags: updated.tags,
    });
    expect(usePromptStore.getState().prompts).toEqual([updated]);
    await usePromptStore.getState().delete(first.id);
    expect(usePromptStore.getState().prompts).toEqual([]);
  });

  it('deduplicates and caps recent usage', () => {
    for (let index = 0; index < 24; index += 1) {
      usePromptStore.getState().recordRecent(`p${index}`);
    }
    usePromptStore.getState().recordRecent('p20');
    expect(usePromptStore.getState().recentIds).toHaveLength(20);
    expect(usePromptStore.getState().recentIds[0]).toBe('p20');
    expect(usePromptStore.getState().recentIds.filter((id) => id === 'p20')).toHaveLength(1);
  });
});
