import { fetchClient } from './client';

export type SourceKind = 'spec' | 'html';
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
  `/projects/${encodeURIComponent(projectId)}/slides/${encodeURIComponent(slideId)}/source?kind=${kind}`;

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
