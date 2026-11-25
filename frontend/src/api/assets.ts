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
}
export interface AssetDetail extends AssetInfo { content: string }
export interface AssetTag { id: string; name: string }
export type AssetDraft = Pick<AssetInfo, 'name' | 'description' | 'tag_ids'> & { content: string };
type Changes = Partial<AssetDraft & { enabled: boolean }>;
const read = { reportError: false, cache: 'no-store' } as const;
export const assetsApi = {
  list: (type: ResourceScope, query: string) => fetchClient<{ items: AssetInfo[] }>('/assets?' + new URLSearchParams({ type, query }), read),
  detail: (id: string) => fetchClient<AssetDetail>('/assets/' + encodeURIComponent(id), read),
  create: (type: ResourceScope, draft: AssetDraft) => postJson<{ id: string }>('/assets/create', { type, ...draft }),
  update: (id: string, type: ResourceScope, changes: Changes) => postJson<void>('/assets/update', { asset_id: id, type, ...changes }),
  delete: (asset_id: string) => postJson<void>('/assets/delete', { asset_id }),
  tags: (type: ResourceScope) => fetchClient<{ items: AssetTag[] }>('/assets/tags?type=' + type, read),
  createTag: (type: ResourceScope, name: string) => postJson<{ id: string }>('/assets/tags/create', { type, name }),
  deleteTag: (tag_id: string) => postJson<void>('/assets/tags/delete', { tag_id }),
};
