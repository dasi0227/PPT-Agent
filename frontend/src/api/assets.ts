import { fetchClient, postJson } from './client';
import type { ResourceScope } from './types';

export interface AssetInfo {
  id: string;
  type: ResourceScope;
  name: string;
  description: string;
  enabled: boolean;
  tag_ids: string[];
  updated_at: number;
  hash: string | null;
}
export interface AssetContent { content: string; hash: string }
export interface AssetTag { id: string; name: string }
export type AssetDraft = Pick<AssetInfo, 'name' | 'description' | 'tag_ids'> & { content: string };
type SavedAsset = AssetInfo & { hash: string };
type Changes = Partial<AssetDraft & { enabled: boolean; base_hash: string }>;
const read = { reportError: false, cache: 'no-store' } as const;
export const assetsApi = {
  list: (type: ResourceScope, query: string) => fetchClient<{ items: AssetInfo[] }>('/assets?' + new URLSearchParams({ type, query }), read),
  content: (id: string) => fetchClient<AssetContent>('/assets/' + encodeURIComponent(id), read),
  create: (type: ResourceScope, draft: AssetDraft) => postJson<SavedAsset>('/assets/create', { type, ...draft }),
  update: (id: string, changes: Changes) => postJson<SavedAsset>('/assets/update', { asset_id: id, ...changes }),
  delete: (asset_id: string) => postJson<void>('/assets/delete', { asset_id }),
  tags: (type: ResourceScope) => fetchClient<{ items: AssetTag[] }>('/assets/tags?type=' + type, read),
  createTag: (type: ResourceScope, name: string) => postJson<AssetTag>('/assets/tags/create', { type, name }),
  deleteTag: (tag_id: string) => postJson<void>('/assets/tags/delete', { tag_id }),
};
