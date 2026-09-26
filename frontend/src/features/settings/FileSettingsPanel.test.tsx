import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { useFileSettingsStore } from '../../stores/fileSettingsStore';
import { FileSettingsPanel } from './FileSettingsPanel';
const { fetchClient } = vi.hoisted(() => ({ fetchClient: vi.fn() }));
vi.mock('../../api/client', () => ({ fetchClient }));
vi.mock('../../components/ui/select', () => ({ Select: ({ value, onValueChange, options, ...props }: {
  value: string; onValueChange: (value: string) => void; options: { value: string; label: string }[];
}) => <select {...props} value={value} onChange={event => onValueChange(event.target.value)}>
  {options.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}
</select> }));
const initial = { open_with: 'system', custom_app_path: '', custom_apps: [], revision: 0, supported: true };
const editor = { name: 'My Editor', path: '/Applications/My Editor.app' };
const other = { name: 'Other Editor', path: '/Applications/Other Editor.app' };
const saves = () => fetchClient.mock.calls.filter(([, options]) => options?.method === 'PUT');
beforeEach(() => {
  useFileSettingsStore.setState({ value: null }); fetchClient.mockReset();
  fetchClient.mockImplementation(async (_url, options) => {
    if (options?.method !== 'PUT') return initial;
    const edit = JSON.parse(options.body);
    return { ...edit, supported: true, revision: edit.revision + 1 };
  });
});
it('persists a default and keeps it selected when a subsequent save fails', async () => {
  render(<FileSettingsPanel refreshKey={0} onSavingChange={vi.fn()} />);
  const select = await screen.findByLabelText('默认打开方式');
  fireEvent.change(select, { target: { value: 'finder' } });
  await waitFor(() => expect(select).toHaveValue('finder'));
  expect(JSON.parse(saves()[0][1].body)).toEqual({ open_with: 'finder', custom_app_path: '', custom_apps: [], revision: 0 });
  fetchClient.mockRejectedValueOnce(new Error('配置已被更新'));
  fireEvent.change(select, { target: { value: 'vscode' } });
  expect(await screen.findByRole('alert')).toHaveTextContent('配置已被更新');
  expect(select).toHaveValue('finder');
});
it('adds multiple applications without changing the default, and ignores cancellation and duplicates', async () => {
  render(<FileSettingsPanel refreshKey={0} onSavingChange={vi.fn()} />);
  const select = await screen.findByLabelText('默认打开方式');
  const add = screen.getByRole('button', { name: '添加应用' });
  expect(within(select).getAllByRole('option')).toHaveLength(4);
  expect(screen.queryByText('其他应用')).not.toBeInTheDocument();
  fetchClient.mockResolvedValueOnce({ application: null });
  fireEvent.click(add);
  await waitFor(() => expect(add).not.toBeDisabled());
  expect(saves()).toHaveLength(0);
  for (const application of [editor, other, editor]) {
    fetchClient.mockResolvedValueOnce({ application });
    fireEvent.click(add);
    await waitFor(() => expect(add).not.toBeDisabled());
  }
  expect(saves()).toHaveLength(2);
  expect(select).toHaveValue('system');
  expect(within(select).getAllByRole('option')).toHaveLength(6);
  expect(screen.getByRole('button', { name: '删除应用 My Editor' })).toBeEnabled();
  expect(screen.getByRole('button', { name: '删除应用 Other Editor' })).toBeEnabled();
  expect(screen.queryByRole('button', { name: '更换应用' })).not.toBeInTheDocument();
  fireEvent.change(select, { target: { value: `app:${other.path}` } });
  await waitFor(() => expect(select).toHaveValue(`app:${other.path}`));
  expect(useFileSettingsStore.getState().value?.custom_app_path).toBe(other.path);
});
it('preserves entries on failed deletion, then removes the selected app and resets the default atomically', async () => {
  fetchClient.mockResolvedValueOnce({ ...initial, open_with: 'custom', custom_app_path: editor.path, custom_apps: [editor, other] });
  render(<FileSettingsPanel refreshKey={0} onSavingChange={vi.fn()} />);
  const select = await screen.findByLabelText('默认打开方式');
  fetchClient.mockRejectedValueOnce(new Error('保存失败'));
  fireEvent.click(screen.getByRole('button', { name: '删除应用 My Editor' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('保存失败');
  expect(select).toHaveValue(`app:${editor.path}`);
  expect(within(select).getAllByRole('option')).toHaveLength(6);
  fireEvent.click(screen.getByRole('button', { name: '删除应用 My Editor' }));
  await waitFor(() => expect(select).toHaveValue('system'));
  expect(within(select).getAllByRole('option')).toHaveLength(5);
  expect(screen.queryByRole('button', { name: '删除应用 My Editor' })).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: '删除应用 Other Editor' })).toBeEnabled();
  expect(JSON.parse(saves()[1][1].body)).toMatchObject({ open_with: 'system', custom_app_path: '', custom_apps: [other] });
});
