import { fetchClient } from './client';

export type FileOpenWith = 'system' | 'vscode' | 'textedit' | 'finder' | 'custom' | 'inherit';
export interface FileOpenMethod { open_with: FileOpenWith; custom_app_path: string }
export interface FileApplication { name: string; path: string; builtin?: 'finder' | 'textedit' | 'vscode' }
export interface FileSettings {
  default: FileOpenMethod;
  json: FileOpenMethod;
  html: FileOpenMethod;
  revision: number;
  custom_apps: FileApplication[];
}
export interface FileSettingsView extends FileSettings { supported: boolean }
export function fileMethodForPath(value: FileSettings, path: string): FileOpenMethod {
  const extension = path.split('/').pop()?.match(/\.([^.]+)$/)?.[1]?.toLowerCase();
  const method = extension === 'json' ? value.json : extension === 'html' || extension === 'htm' ? value.html : value.default;
  return method.open_with === 'inherit' ? value.default : method;
}
export function fileActionLabel(value: FileSettingsView | null, url: string) {
  if (!value) return '打开文件';
  const method = fileMethodForPath(value, new URL(url, window.location.origin).searchParams.get('path') ?? '');
  if (method.open_with === 'finder') return '在访达中显示';
  const names: Partial<Record<FileOpenWith, string>> = { system: '系统默认应用', vscode: 'VS Code', textedit: '文本编辑' };
  const name = method.open_with === 'custom' ? value.custom_apps.find(app => app.path === method.custom_app_path)?.name ?? '所选应用' : names[method.open_with];
  return `使用${name}打开`;
}
export const filesApi = {
  get: () => fetchClient<FileSettingsView>('/settings/files', { cache: 'no-store', reportError: false }),
  save: (edit: FileSettings) => fetchClient<FileSettingsView>('/settings/files', { method: 'PUT', reportError: false, body: JSON.stringify(edit) }),
  pickApplication: () => fetchClient<{ application: FileApplication | null }>(
    '/settings/files/pick-application', { method: 'POST', reportError: false, timeoutMs: 125000 }),
  open: (url: string) => {
    if (!url.startsWith('/api/v1/files/open?')) return Promise.reject(new Error('文件链接已失效，请刷新页面后重试'));
    return fetchClient<void>(url.slice('/api/v1'.length), { method: 'POST', reportError: false });
  },
};
