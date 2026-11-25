import { fetchClient, postJson } from './client';
import type { Constraint, Decorations, Meta, Outline, Project, RuntimeLogo, SlideSpec } from './types';

export interface ProjectFiles {
  meta: Meta | null;
  constraint: Constraint | null;
  outline: Outline | null;
  slides: Record<string, { spec: SlideSpec | null; html: string | null }>;
  hashes: {
    meta: string | null;
    constraint: string | null;
    outline: string | null;
    slides: Record<string, { spec: string | null; html: string | null }>;
  };
}

export interface ProjectAppearance {
  theme_id: string | null;
  positions: Decorations;
  logo: boolean;
}

export type ProjectFileWrite =
  | { part: 'meta'; content: Meta }
  | { part: 'constraint'; content: Constraint }
  | { part: 'outline'; content: Outline }
  | { part: 'spec'; slideId: string; content: SlideSpec }
  | { part: 'html'; slideId: string; content: string };

const projectPath = (id: string) => '/projects/' + encodeURIComponent(id);
const read = { cache: 'no-store', reportError: false } as const;

async function getLogo(id: string, previous: RuntimeLogo | null): Promise<RuntimeLogo | null> {
  const response = await fetch('/api' + projectPath(id) + '/logo', {
    cache: 'no-cache',
    headers: previous ? { 'If-None-Match': previous.hash } : undefined,
  });
  if (response.status === 404) return null;
  if (response.status === 304) return previous;
  const hash = response.headers.get('ETag');
  if (!response.ok || !hash) throw new Error('标志图读取失败');
  if (previous?.hash === hash) {
    await response.body?.cancel();
    return previous;
  }
  const image = await response.blob();
  const src = await new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result as string);
    reader.onerror = () => reject(reader.error);
    reader.readAsDataURL(image);
  });
  return { hash, src };
}

export const projectsApi = {
  list: () => fetchClient<{ items: Project[] }>('/projects', { cache: 'no-store' }).then(response => response.items),
  create: (title: string) => postJson<{ id: string }>('/projects/create', { title }),
  update: (project_id: string, title: string) => postJson<void>('/projects/update', { project_id, title }),
  delete: (project_id: string) => postJson<void>('/projects/delete', { project_id }),
  getContent: (id: string) => fetchClient<ProjectFiles>(projectPath(id), read),
  saveFile: (project_id: string, base_hash: string | null, file: ProjectFileWrite) =>
    postJson<{ hash: string }>('slideId' in file ? '/slides/' + file.part + '/save' : '/projects/' + file.part + '/save', {
      project_id,
      base_hash,
      ...('slideId' in file && { slide_id: file.slideId }),
      [file.part]: file.content,
    }),
  getAppearance: (id: string) => fetchClient<ProjectAppearance>(projectPath(id) + '/appearance', read),
  setTheme: (project_id: string, theme_id: string) => postJson<void>('/projects/theme/save', { project_id, theme_id }),
  setPositions: (project_id: string, positions: Decorations) => postJson<void>('/projects/positions/save', { project_id, ...positions }),
  getLogo,
  uploadLogo: (id: string, file: File) => {
    const body = new FormData();
    body.append('project_id', id);
    body.append('file', file, file.name);
    return fetchClient<void>('/projects/logo/save', { method: 'POST', body, reportError: false, timeoutMs: 30_000 });
  },
  deleteLogo: (project_id: string) => postJson<void>('/projects/logo/delete', { project_id }),
  thumbnailUrl: (projectId: string, slideId: string) => '/api' + projectPath(projectId) + '/slides/' + encodeURIComponent(slideId) + '/thumbnail',
};
