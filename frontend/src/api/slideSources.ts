import { fetchClient } from './client';

export type SourceKind = 'manifest' | 'design' | 'spec' | 'html';
export const SOURCE_META = {
  manifest: { filename: 'manifest.json', label: '内容要求', language: 'JSON' },
  design: { filename: 'design.json', label: '视觉要求', language: 'JSON' },
  spec: { filename: 'spec.json', label: '设计稿', language: 'JSON' },
  html: { filename: 'index.html', label: '幻灯片', language: 'HTML' },
} as const;
export const isProjectSource = (kind: SourceKind): kind is 'manifest' | 'design' => kind === 'manifest' || kind === 'design';
export function sourceOrder(file: { kind: SourceKind; slideId: string }, slideIds: string[]) {
  if (file.kind === 'manifest') return -2;
  if (file.kind === 'design') return -1;
  const index = slideIds.indexOf(file.slideId);
  return (index < 0 ? slideIds.length : index) * 2 + (file.kind === 'html' ? 1 : 0);
}
export interface SlideSourceDocument {
  project_id: string;
  slide_id: string;
  kind: SourceKind;
  path: string;
  language: 'json' | 'html';
  content: string;
  source_hash: string;
  content_hash: string | null;
  scene_revision: number;
  writable: boolean;
  readonly_reason: string | null;
}

const path = (projectId: string, slideId: string, kind: SourceKind) =>
  isProjectSource(kind) ? `/projects/${encodeURIComponent(projectId)}/source?kind=${kind}`
    : `/projects/${encodeURIComponent(projectId)}/slides/${encodeURIComponent(slideId)}/source?kind=${kind}`;

export const slideSourcesApi = {
  get: (projectId: string, slideId: string, kind: SourceKind) =>
    fetchClient<SlideSourceDocument>(path(projectId, slideId, kind), { reportError: false }),
  save: (projectId: string, slideId: string, kind: SourceKind, content: string, expectedSourceHash: string, expectedSceneRevision: number) =>
    fetchClient<{ changed: boolean; document: SlideSourceDocument }>(path(projectId, slideId, kind), {
      method: 'PUT',
      body: JSON.stringify({ content, expected_source_hash: expectedSourceHash, expected_scene_revision: expectedSceneRevision }),
      reportError: false,
      timeoutMs: 30_000,
    }),
};
