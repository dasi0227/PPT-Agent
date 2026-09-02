import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { ComponentReference } from '../api/types';
import { useComponentStore } from './componentStore';

const mocks = vi.hoisted(() => ({
  listComponents: vi.fn(),
}));

vi.mock('../api/repositories', () => ({
  repositoriesApi: { listComponents: mocks.listComponents },
}));

const component = (id: string, disabled = false): ComponentReference => ({
  id,
  name: `Component ${id}`,
  description: `Description ${id}`,
  tags: ['card'],
  disabled,
  open_url: '',
});

describe('componentStore', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useComponentStore.setState({
      components: [],
      loading: false,
      loaded: false,
      error: '',
      version: 0,
    });
  });

  it('loads and caches only enabled components', async () => {
    mocks.listComponents.mockResolvedValue({
      components: [component('enabled'), component('disabled', true)],
    });

    await useComponentStore.getState().load();
    await useComponentStore.getState().load();

    expect(mocks.listComponents).toHaveBeenCalledTimes(1);
    expect(useComponentStore.getState()).toMatchObject({
      components: [component('enabled')],
      loaded: true,
      loading: false,
      error: '',
    });
  });

  it('keeps the composer cache empty without marking it loaded when loading fails', async () => {
    mocks.listComponents.mockRejectedValue(new Error('offline'));

    await expect(useComponentStore.getState().load()).rejects.toThrow('offline');

    expect(useComponentStore.getState()).toMatchObject({
      components: [],
      loaded: false,
      loading: false,
      error: 'offline',
    });
  });
});
