export const homeRoute = '/';
export type RepositorySection = 'theme' | 'component' | 'skill' | 'snippet';

export function repositoryRoute(section: RepositorySection): string {
  return `/warehouse/${section}`;
}

export function projectRoute(projectId: string): string {
  return `/projects/${encodeURIComponent(projectId)}`;
}

export function projectWorkspaceRoute(
  projectId: string,
  params: { slideId?: string; view?: 'html' | 'outline'; mode?: 'main' | 'overview'; content?: 'preview' | 'source'; document?: 'manifest' | 'design' | null } = {},
): string {
  const search = new URLSearchParams();
  if (params.slideId) search.set('slide', params.slideId);
  if (params.view && params.view !== 'html') search.set('view', params.view);
  if (params.mode && params.mode !== 'main') search.set('mode', params.mode);
  if (params.content === 'source') search.set('content', 'source');
  if (params.document) search.set('document', params.document);
  const query = search.toString();
  return `${projectRoute(projectId)}${query ? `?${query}` : ''}`;
}
