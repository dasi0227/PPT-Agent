import { fetchClient } from './client';

export type FileOpenWith = 'system' | 'vscode' | 'textedit' | 'finder' | 'custom';
export interface FileApplication { name: string; path: string }
export interface FileSettings {
  open_with: FileOpenWith;
  custom_app_path: string;
  revision: number;
  custom_apps: FileApplication[];
}
export interface FileSettingsView extends FileSettings {
  supported: boolean;
  custom_app_name?: string;
}
export const fileOpenOptions: { value: FileOpenWith; label: string }[] = [
  { value: 'system', label: '系统默认应用' },
  { value: 'vscode', label: 'VS Code' },
  { value: 'textedit', label: '文本编辑' },
  { value: 'finder', label: '在访达中显示' },
];
export function fileActionLabel(value: FileSettingsView | null) {
  if (!value) return '打开文件';
  if (value.open_with === 'finder') return '在访达中显示';
  const name = value.open_with === 'custom' ? value.custom_app_name || '所选应用'
    : fileOpenOptions.find(option => option.value === value.open_with)?.label;
  return `使用${name}打开`;
}
export const filesApi = {
  get: () => fetchClient<FileSettingsView>('/settings/files', { cache: 'no-store', reportError: false }),
  save: (edit: FileSettings) => fetchClient<FileSettingsView>('/settings/files', { method: 'PUT', reportError: false, body: JSON.stringify(edit) }),
  pickApplication: () => fetchClient<{ application: { name: string; path: string } | null }>(
    '/settings/files/pick-application', { method: 'POST', reportError: false, timeoutMs: 125000 }),
  open: (url: string) => {
    if (!url.startsWith('/api/v1/files/open?')) return Promise.reject(new Error('文件链接已失效，请刷新页面后重试'));
    return fetchClient<void>(url.slice('/api/v1'.length), { method: 'POST', reportError: false });
  },
};
