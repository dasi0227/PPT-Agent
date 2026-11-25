export const homeRoute = '/';
export type RepositorySection = 'theme' | 'component' | 'skill' | 'snippet';
export const settingsSections = ['models', 'routing', 'agent', 'files', 'shortcuts'] as const;
export type SettingsSection = typeof settingsSections[number];

export function settingsRoute(section: SettingsSection): string {
  return `/settings/${section}`;
}

export function getSettingsSection(pathname: string): SettingsSection | undefined {
  const path = pathname.replace(/\/+$/, '').toLowerCase();
  return path === '/settings' ? 'models' : settingsSections.find(section => settingsRoute(section) === path);
}

export function repositoryRoute(section: RepositorySection): string {
  return `/assets/${section}`;
}

export function projectRoute(projectId: string): string {
  return `/projects/${encodeURIComponent(projectId)}`;
}

export function projectWorkspaceRoute(
  projectId: string,
  params: { slideId?: string; view?: 'html' | 'outline'; mode?: 'main' | 'overview'; content?: 'preview' | 'source'; document?: 'meta' | 'constraint' | 'outline' | null } = {},
): string {
  const search = new URLSearchParams();
  if (params.document) {
    search.set('document', params.document);
  } else if (params.mode === 'overview') {
    search.set('mode', 'overview');
  } else {
    if (params.slideId) search.set('slide', params.slideId);
  }
  // Retain the selected page view while a project document or overview is open.
  if (params.view === 'outline') search.set('view', 'spec');
  else if (!params.document && params.mode !== 'overview' && params.content === 'source') search.set('view', 'html');
  if (params.content === 'source') search.set('content', 'source');
  const query = search.toString();
  return `${projectRoute(projectId)}${query ? `?${query}` : ''}`;
}
