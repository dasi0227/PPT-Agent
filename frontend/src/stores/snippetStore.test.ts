import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { Snippet } from '../api/types';
import { useSnippetStore } from './snippetStore';

const mocks = vi.hoisted(() => ({
  list: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  setDisabled: vi.fn(),
  delete: vi.fn(),
}));

vi.mock('../api/snippets', () => ({ snippetsApi: mocks }));

const snippet = (id: string): Snippet => ({
  id,
  name: `提示 ${id} / Snippet ${id}`,
  description: `description ${id}`,
  content: `value ${id}`,
  tags: ['deliverable'],
  disabled: false, content_state: 'ready', open_url: 'vscode://file/snippets/p1/snippet.txt',
  created_at: 1,
  updated_at: 1,
});

describe('snippetStore', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    useSnippetStore.setState({
      snippets: [],
      loading: false,
      loaded: false,
      error: '',
      version: 0,
    });
  });

  it('loads snippets into the shared cache', async () => {
    mocks.list.mockResolvedValue({ snippets: [snippet('p1'), snippet('p2')] });

    await useSnippetStore.getState().load();

    expect(useSnippetStore.getState()).toMatchObject({
      snippets: [snippet('p1'), snippet('p2')],
      loaded: true,
      loading: false,
    });
  });

  it('updates cache only after CRUD requests succeed', async () => {
    const first = snippet('p1');
    const updated = { ...first, description: 'updated', updated_at: 2 };
    mocks.create.mockResolvedValue(first);
    mocks.update.mockResolvedValue(updated);
    mocks.delete.mockResolvedValue(undefined);

    await useSnippetStore.getState().create({
      name: first.name, description: first.description, content: first.content, tags: first.tags,
    });
    await useSnippetStore.getState().update(first.id, {
      name: updated.name, description: updated.description, tags: updated.tags,
    });
    expect(useSnippetStore.getState().snippets).toEqual([updated]);
    await useSnippetStore.getState().delete(first.id);
    expect(useSnippetStore.getState().snippets).toEqual([]);
  });

});
