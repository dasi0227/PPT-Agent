import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import type { Manifest, PPTMutation, ProjectContentSnapshot } from '../../api/types';
import { useProjectStore } from '../../stores/projectStore';
import { ProjectDocumentView } from './ProjectDocumentView';

const mutate = vi.fn();
const load = vi.fn().mockResolvedValue(undefined);
const initial: ProjectContentSnapshot = {
  project_id: 'p', scene_revision: 1, theme: '', appearance: null, hashes: { manifest: 'original' },
  manifest: { title: '演示标题', language: 'zh-CN', pages: '待明确', goal: '帮助团队理解 Skill', audience: '开发者', requirements: ['解释结构', '展示案例'], prohibitions: [] },
  design: { direction: '', layout_preferences: [], decorations: { page_number: 'bottom-right', section_title: 'none', deck_title: 'none', key_message: 'none' } },
  outline: { sections: [] }, slides_by_id: {},
};
function applyResponse(mutation: PPTMutation) {
  if (mutation.op !== 'manifest.patch') throw new Error('Unexpected resource');
  const current = useProjectStore.getState().contentByProjectId.p;
  const manifest = { ...current.manifest } as Record<keyof Manifest, unknown>;
  for (const patch of mutation.patch) if ('value' in patch) manifest[patch.path.slice(1) as keyof Manifest] = patch.value;
  const next: ProjectContentSnapshot = { ...current, manifest: manifest as unknown as Manifest, hashes: { manifest: `saved-${mutate.mock.calls.length}` } };
  useProjectStore.setState({ contentByProjectId: { p: next } });
  return next;
}
beforeEach(() => {
  mutate.mockReset().mockImplementation(async (_project: string, mutation: PPTMutation) => applyResponse(mutation));
  load.mockClear();
  useProjectStore.setState({ mutateProject: mutate, loadProjectContent: load, contentByProjectId: { p: structuredClone(initial) } });
});
function Editor() {
  const snapshot = useProjectStore(state => state.contentByProjectId.p);
  return <ProjectDocumentView document="manifest" snapshot={snapshot} onRetry={vi.fn()} />;
}

it('edits only from the pencil and submits only the changed field with the captured version', async () => {
  render(<Editor />);
  fireEvent.click(screen.getByText('待明确'));
  expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: '编辑演示页数' }));
  expect(screen.getAllByRole('textbox')).toHaveLength(1);
  expect(screen.getByText('开发者')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'JSON 切换' })).toBeDisabled();
  fireEvent.change(screen.getByLabelText('演示页数'), { target: { value: '11-12' } });
  fireEvent.click(screen.getByRole('button', { name: '保存' }));
  await waitFor(() => expect(screen.queryByRole('textbox')).not.toBeInTheDocument());
  expect(mutate).toHaveBeenCalledWith('p', { op: 'manifest.patch', patch: [{ op: 'replace', path: '/pages', value: '11-12' }], expected_hash: 'original', expected_scene_revision: 1 });
  fireEvent.click(screen.getByRole('button', { name: 'JSON 切换' }));
  expect(screen.getByRole('region', { name: 'JSON 只读预览' })).toHaveTextContent('11-12');
  expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
});

it('retains the draft and rejects a changed history scene even when the hash is unchanged', () => {
  render(<Editor />);
  fireEvent.click(screen.getByRole('button', { name: '编辑演示目标' }));
  fireEvent.change(screen.getByLabelText('演示目标'), { target: { value: '未提交的修改' } });
  act(() => useProjectStore.setState({ contentByProjectId: { p: { ...initial, scene_revision: 2 } } }));
  expect(screen.getByLabelText('演示目标')).toHaveValue('未提交的修改');
  expect(screen.getByRole('button', { name: '保存' })).toBeDisabled();
  expect(screen.getByText('内容已更新，请取消后重新编辑。')).toBeInTheDocument();
  expect(mutate).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: '取消' }));
  expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
});

it('does not persist an empty new row and keeps a failed save available for retry', async () => {
  mutate.mockRejectedValueOnce(new Error('暂时无法保存'));
  render(<Editor />);
  fireEvent.click(screen.getByRole('button', { name: '新增内容要求' }));
  fireEvent.click(screen.getByRole('button', { name: '保存' }));
  expect(mutate).not.toHaveBeenCalled();
  fireEvent.change(screen.getByRole('textbox'), { target: { value: '新增要求' } });
  fireEvent.click(screen.getByRole('button', { name: '保存' }));
  await screen.findByText('暂时无法保存');
  expect(screen.getByRole('textbox')).toHaveValue('新增要求');
  expect(useProjectStore.getState().contentByProjectId.p.manifest.requirements).toEqual(['解释结构', '展示案例']);
  fireEvent.click(screen.getByRole('button', { name: '保存' }));
  await waitFor(() => expect(screen.queryByRole('textbox')).not.toBeInTheDocument());
  expect(useProjectStore.getState().contentByProjectId.p.manifest.requirements).toEqual(['解释结构', '展示案例', '新增要求']);
});

it('restores a deleted row with the deletion response version, then expires undo on a newer resource', async () => {
  render(<Editor />);
  fireEvent.click(screen.getByRole('button', { name: '删除内容要求 1' }));
  await screen.findByRole('button', { name: '撤销' });
  expect(screen.queryByText('解释结构')).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole('button', { name: '撤销' }));
  await screen.findByText('解释结构');
  expect(mutate).toHaveBeenLastCalledWith('p', { op: 'manifest.patch', patch: [{ op: 'replace', path: '/requirements', value: ['解释结构', '展示案例'] }], expected_hash: 'saved-1', expected_scene_revision: 1 });
  fireEvent.click(screen.getByRole('button', { name: '删除内容要求 1' }));
  await screen.findByRole('button', { name: '撤销' });
  act(() => {
    const current = useProjectStore.getState().contentByProjectId.p;
    useProjectStore.setState({ contentByProjectId: { p: { ...current, hashes: { manifest: 'external-update' } } } });
  });
  expect(screen.queryByRole('button', { name: '撤销' })).not.toBeInTheDocument();
});

it('locks other mutations while a request is pending', async () => {
  let finish!: () => void;
  mutate.mockImplementationOnce((_project: string, mutation: PPTMutation) => new Promise<ProjectContentSnapshot>(resolve => {
    finish = () => resolve(applyResponse(mutation));
  }));
  render(<Editor />);
  fireEvent.click(screen.getByRole('button', { name: '删除内容要求 1' }));
  expect(screen.getByRole('button', { name: '新增内容要求' })).toBeDisabled();
  fireEvent.click(screen.getByRole('button', { name: '删除内容要求 2' }));
  expect(mutate).toHaveBeenCalledTimes(1);
  await act(async () => finish());
  expect(screen.getByRole('button', { name: '新增内容要求' })).toBeEnabled();
});
