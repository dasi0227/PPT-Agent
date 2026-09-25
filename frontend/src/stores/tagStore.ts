import { useEffect, useMemo } from 'react';
import { create } from 'zustand';
import { fetchClient } from '../api/client';
import type { ResourceScope, TagDefinition } from '../api/types';
import { showGlobalError } from './toastStore';

interface TagState {
  tags: Partial<Record<ResourceScope, TagDefinition[]>>;
  load: (scope: ResourceScope) => Promise<void>;
}

const inflight = new Map<ResourceScope, Promise<void>>();
const emptyTags: TagDefinition[] = [];

export const useTagStore = create<TagState>((set) => ({
  tags: {},
  load: (scope) => {
    const pending = inflight.get(scope);
    if (pending) return pending;
    const request = fetchClient<{ tags: TagDefinition[] }>(`/tags?scope=${scope}`, { reportError: false })
      .then(({ tags }) => {
        set(state => ({ tags: { ...state.tags, [scope]: tags } }));
      })
      .finally(() => { inflight.delete(scope); });
    inflight.set(scope, request);
    return request;
  },
}));

// Refresh on entry/focus, and expose the same refresh to repository controls.
// Labels and ordering come only from the server response.
export function useResourceTags(scope: ResourceScope, active = true) {
  const tags = useTagStore(state => state.tags[scope] ?? emptyTags);
  const load = useTagStore(state => state.load);
  useEffect(() => {
    if (!active) return;
    const refresh = () => { void load(scope).catch(error => showGlobalError(error instanceof Error ? error.message : '标签加载失败')); };
    refresh();
    window.addEventListener('focus', refresh);
    return () => window.removeEventListener('focus', refresh);
  }, [active, load, scope]);
  return useMemo(() => ({
    labels: Object.fromEntries(tags.map(tag => [tag.key, tag.name])) as Record<string, string>,
    order: tags.map(tag => tag.key),
    refresh: () => load(scope),
  }), [load, scope, tags]);
}
