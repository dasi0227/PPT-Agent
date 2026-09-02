import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Prompt } from '../api/types';
import { usePromptStore } from './promptStore';

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
  name: `提示 ${id} / Prompt ${id}`,
  desc: `description ${id}`,
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
      loading: false,
      loaded: false,
      error: '',
      version: 0,
    });
  });

  it('loads prompts into the shared cache', async () => {
    mocks.list.mockResolvedValue({ prompts: [prompt('p1'), prompt('p2')] });

    await usePromptStore.getState().load();

    expect(usePromptStore.getState()).toMatchObject({
      prompts: [prompt('p1'), prompt('p2')],
      loaded: true,
      loading: false,
    });
  });

  it('updates cache only after CRUD requests succeed', async () => {
    const first = prompt('p1');
    const updated = { ...first, value: 'updated', updated_at: 2 };
    mocks.create.mockResolvedValue(first);
    mocks.update.mockResolvedValue(updated);
    mocks.delete.mockResolvedValue(undefined);

    await usePromptStore.getState().create({
      name: first.name, desc: first.desc, value: first.value, tags: first.tags,
    });
    await usePromptStore.getState().update(first.id, {
      name: updated.name, desc: updated.desc, value: updated.value, tags: updated.tags,
    });
    expect(usePromptStore.getState().prompts).toEqual([updated]);
    await usePromptStore.getState().delete(first.id);
    expect(usePromptStore.getState().prompts).toEqual([]);
  });

});
