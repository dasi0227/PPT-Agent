export const homeRoute = '/';

export function projectRoute(projectId: string): string {
  return `/projects/${encodeURIComponent(projectId)}`;
}
