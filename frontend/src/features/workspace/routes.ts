export const homeRoute = '/';
export type RepositorySection = 'theme' | 'component' | 'skill';

export function repositoryRoute(section: RepositorySection): string {
  return `/warehouse/${section}`;
}

export function projectRoute(projectId: string): string {
  return `/projects/${encodeURIComponent(projectId)}`;
}

export function projectWorkspaceRoute(
  projectId: string,
  params: { slideId?: string; view?: 'html' | 'outline'; mode?: 'main' | 'overview' } = {},
): string {
  const search = new URLSearchParams();
  if (params.slideId) search.set('slide', params.slideId);
  if (params.view && params.view !== 'html') search.set('view', params.view);
  if (params.mode && params.mode !== 'main') search.set('mode', params.mode);
  const query = search.toString();
  return `${projectRoute(projectId)}${query ? `?${query}` : ''}`;
}
