import { act, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { projectsApi } from '../../api/projects';
import type { MutationResponse, ProjectContentSnapshot } from '../../api/types';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { useResourceApprovalStore } from '../../stores/resourceApprovalStore';
import { ProjectDocumentView } from './ProjectDocumentView';

const missing: ProjectContentSnapshot = {
  project_id: 'p1', project_title: '新项目', scene_revision: 1, theme: '', appearance: null,
  hashes: {}, manifest: null, design: null, outline: { sections: [] }, slides_by_id: {},
};
const created: ProjectContentSnapshot = { ...missing, scene_revision: 2, hashes: { outline: 'empty' } };
function response(content: ProjectContentSnapshot): MutationResponse {
  return { content, mutation: { operation: 'outline.create', hashes: content.hashes, created: {}, affected_slide_ids: [] } };
}
function Harness() {
  const snapshot = useProjectStore(state => state.contentByProjectId.p1);
  return <ProjectDocumentView document="outline" snapshot={snapshot} onRetry={() => {}} />;
}
beforeEach(() => {
  useDeckStore.setState({ contentMode: 'preview', sourceBlocked: false });
  useResourceApprovalStore.setState({ active: null, editStates: {} });
  useProjectStore.setState({ activeProjectId: 'p1', contentByProjectId: { p1: missing }, mutationPendingByProjectId: {}, contentErrorByProjectId: {} });
  HTMLElement.prototype.scrollIntoView = vi.fn();
});
afterEach(() => { vi.restoreAllMocks(); });

it('creates a real empty outline on demand, then persists confirmed rows without ordinary footer actions', async () => {
  let finish!: (result: MutationResponse) => void;
  const mutation = vi.spyOn(projectsApi, 'mutate').mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  const { container } = render(<Harness />);
  expect(screen.getByText('暂无文件')).toBeInTheDocument();
  expect(container.querySelector('.management-footer')).toBeNull();
  expect(useDeckStore.getState().sourceBlocked).toBe(true);
  act(() => useDeckStore.getState().setContentMode('source'));
  expect(useDeckStore.getState().contentMode).toBe('preview');
  const create = screen.getByRole('button', { name: '新建' });
  fireEvent.click(create); fireEvent.click(create);
  expect(mutation).toHaveBeenCalledTimes(1);
  expect(mutation).toHaveBeenCalledWith('p1', { op: 'outline.create', expected_scene_revision: 1 });
  await act(async () => finish(response(created)));
  expect(screen.queryByText('暂无文件')).not.toBeInTheDocument();
  expect(container.querySelector('.management-footer')).toBeNull();
  expect(useDeckStore.getState().sourceBlocked).toBe(false);
  fireEvent.click(screen.getByRole('button', { name: '新增章节' }));
  fireEvent.change(screen.getByRole('textbox', { name: '章节名称' }), { target: { value: '第一章' } });
  const next = { ...created, scene_revision: 3, hashes: { outline: 'updated' }, outline: { sections: [{ id: 'sec_real', title: '第一章', purpose: '待明确', slides: [], subsections: [] }] } };
  mutation.mockImplementationOnce(() => new Promise(resolve => { finish = resolve; }));
  fireEvent.click(screen.getByRole('button', { name: '确认编辑' }));
  expect(await screen.findByRole('status')).toHaveTextContent('正在保存…');
  expect(mutation).toHaveBeenLastCalledWith('p1', expect.objectContaining({ op: 'outline.insert', expected_hash: 'empty', expected_scene_revision: 2 }));
  await act(async () => finish(response(next)));
  expect(screen.getByRole('status')).toHaveTextContent('已保存');
  expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: '恢复' })).not.toBeInTheDocument();
  expect(screen.queryByRole('button', { name: '保存' })).not.toBeInTheDocument();
  act(() => useDeckStore.getState().setContentMode('source'));
  expect(container.querySelector('code')).toHaveTextContent('"id": "sec_real"');
  expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
});

it('refreshes after a failed creation and allows retry without presenting a virtual empty file as JSON', async () => {
  const mutation = vi.spyOn(projectsApi, 'mutate').mockRejectedValueOnce(new Error('创建冲突')).mockResolvedValueOnce(response(created));
  const refresh = vi.spyOn(projectsApi, 'getContent').mockResolvedValue(missing);
  render(<Harness />);
  fireEvent.click(screen.getByRole('button', { name: '新建' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('创建冲突');
  expect(refresh).toHaveBeenCalledWith('p1');
  expect(useDeckStore.getState().sourceBlocked).toBe(true);
  fireEvent.click(screen.getByRole('button', { name: '新建' }));
  await waitFor(() => expect(screen.getByRole('button', { name: '新增章节' })).toBeInTheDocument());
  expect(mutation).toHaveBeenCalledTimes(2);
});
