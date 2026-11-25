import { useEffect, useMemo } from 'react';
import { create } from 'zustand';
import { assetsApi, type AssetTag } from '../api/assets';
import type { ResourceScope } from '../api/types';
import { showGlobalError } from './toastStore';

interface TagState {
  tags: Partial<Record<ResourceScope, AssetTag[]>>;
  load: (scope: ResourceScope, fresh?: boolean) => Promise<void>;
}

const inflight = new Map<ResourceScope, Promise<void>>();
const emptyTags: AssetTag[] = [];

export const useTagStore = create<TagState>((set) => ({
  tags: {},
  load: (scope, fresh = false) => {
    const pending = inflight.get(scope);
    if (pending && !fresh) return pending;
    const request: Promise<void> = assetsApi.tags(scope)
      .then(({ items: tags }) => {
        if (inflight.get(scope) === request) set(state => ({ tags: { ...state.tags, [scope]: tags } }));
      })
      .finally(() => {
        if (inflight.get(scope) === request) inflight.delete(scope);
      });
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
    labels: Object.fromEntries(tags.map(tag => [tag.id, tag.name])) as Record<string, string>,
    order: tags.map(tag => tag.id),
    refresh: () => load(scope),
  }), [load, scope, tags]);
}
