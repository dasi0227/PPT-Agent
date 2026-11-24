import { fetchClient, postJson } from './client';

export interface FileSettings {
  default_open: string;
  json_open: string;
  html_open: string;
  custom_apps: string[];
}
export function applicationName(path: string) {
  return path.split(/[/\\]/).pop()?.replace(/\.(app|exe)$/i, '') || path;
}
export function fileActionLabel(value: FileSettings | null, url: string) {
  if (!value) return '打开文件';
  const path = new URL(url, window.location.origin).searchParams.get('path') ?? '';
  let method = /\.json$/i.test(path) ? value.json_open : /\.html?$/i.test(path) ? value.html_open : value.default_open;
  if (method === 'inherit') method = value.default_open;
  return `使用${method === 'system' ? '系统默认应用' : applicationName(method)}打开`;
}
export const filesApi = {
  get: () => fetchClient<FileSettings>('/settings/fileopen', { cache: 'no-store', reportError: false }),
  save: (edit: FileSettings) => postJson<FileSettings>('/settings/fileopen/save', edit),
  pickApplication: () => fetchClient<{ path: string | null }>('/settings/fileopen/pick', { method: 'POST', reportError: false, timeoutMs: 125000 }),
  open: (url: string) => {
    const link = new URL(url, location.origin);
    if (link.origin !== location.origin || link.pathname !== '/api/files/open') return Promise.reject(new Error('文件链接已失效，请刷新页面后重试'));
    return postJson<void>('/files/open', { project_id: link.searchParams.get('project_id'), path: link.searchParams.get('path') });
  },
};
