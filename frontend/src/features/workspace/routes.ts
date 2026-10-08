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
  params: { slideId?: string; view?: 'html' | 'outline'; mode?: 'main' | 'overview'; content?: 'preview' | 'source'; document?: 'manifest' | 'design' | 'outline' | null } = {},
): string {
  const search = new URLSearchParams();
  if (params.document) {
    search.set('document', params.document);
    if (params.content === 'source' && params.document !== 'outline') search.set('content', 'source');
  } else if (params.mode === 'overview') {
    search.set('mode', 'overview');
  } else {
    if (params.slideId) search.set('slide', params.slideId);
    if (params.view === 'outline') search.set('view', 'spec');
    else if (params.content === 'source') search.set('view', 'html');
    if (params.content === 'source') search.set('content', 'source');
  }
  const query = search.toString();
  return `${projectRoute(projectId)}${query ? `?${query}` : ''}`;
}
