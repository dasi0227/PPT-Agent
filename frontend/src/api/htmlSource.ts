import { fetchClient } from './client';

export interface HTMLSourceDocument {
  project_id: string;
  slide_id: string;
  path: string;
  content: string;
  source_hash: string;
  scene_revision: number;
}

export const readHTMLSource = (projectId: string, slideId: string) =>
  fetchClient<HTMLSourceDocument>(`/projects/${encodeURIComponent(projectId)}/slides/${encodeURIComponent(slideId)}/source?kind=html`, { reportError: false });
