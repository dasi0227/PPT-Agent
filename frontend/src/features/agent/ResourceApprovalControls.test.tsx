import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';
import { runsApi } from '../../api/runs';
import type { Manifest, Outline, ResourceEditApproval } from '../../api/types';
import { useProjectStore } from '../../stores/projectStore';
import { useDeckStore } from '../../stores/deckStore';
import { resourceApprovalKey, useResourceApprovalStore } from '../../stores/resourceApprovalStore';
import { ApprovalDraftEditor } from '../viewer/ApprovalDraftEditor';
import { ToolActivityRow } from './ActivityRows';
import type { ToolActivityItem } from './eventReducer';

function Harness({ item }: { item: ToolActivityItem }) {
  const active = useResourceApprovalStore(state => state.active);
  const activeDocument = useDeckStore(state => state.activeDocument);
  return <><ToolActivityRow item={item} />{active && activeDocument === active.resource && <ApprovalDraftEditor active={active} />}</>;
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  useResourceApprovalStore.setState({ active: null, editStates: {} });
  useDeckStore.setState({ activeDocument: null, contentMode: 'preview', sourceBlocked: false });
});

it('persists every outline operation to the isolated draft, restores the proposal, and reviews only the saved revision', async () => {
  const user = userEvent.setup();
  HTMLElement.prototype.scrollIntoView = vi.fn();
  const proposal: Outline = { sections: [{ id: 'sec_a', title: '封面', purpose: '说明主题', subsections: [], slides: [{ id: 'sli_a', title: '第一页' }] }] };
  const target = { type: 'deck' as const, part: 'outline' as const, diff: { kind: 'outline' as const, status: 'modified' as const, filename: '.outline.json', sections: [] } };
  let record: ResourceEditApproval<Outline> = { run_id: 'r2', call_id: 'c2', interaction_id: 'resa_2', resource: 'outline', revision: 1,
    base_exists: false, base: null, proposal, draft: proposal, target, state: 'pending' };
  let item: ToolActivityItem = { id: 'r2:tool:c2', type: 'tool', runId: 'r2', callId: 'c2', tool: 'edit_outline', label: '编辑目录结构', status: 'running', timestamp: 1,
    approval: { interactionId: record.interaction_id, resource: 'outline', revision: 1, target } };
  useProjectStore.setState({ activeProjectId: 'p1' });
  const mutate = vi.spyOn(useProjectStore.getState(), 'mutateProject');
  const decide = vi.spyOn(runsApi, 'decideResourceEditApproval').mockResolvedValue(undefined);
  vi.spyOn(runsApi, 'getResourceEditApproval').mockImplementation(async () => record);
  const persist = vi.spyOn(runsApi, 'updateResourceEditDraft').mockImplementation(async (_run, _id, revision, draft) => {
    expect(revision).toBe(record.revision);
    record = { ...record, revision: revision + 1, draft: draft as Outline };
    return record;
  });
  const view = render(<Harness item={item} />);
  await user.click(screen.getByRole('button', { name: '手动编辑' }));
  await user.click(screen.getByRole('button', { name: '继续' }));
  await screen.findByRole('button', { name: '封面' });
  const flow = () => useResourceApprovalStore.getState().editStates[resourceApprovalKey('r2', 'resa_2')];
  const footer = () => within(document.querySelector('.management-footer') as HTMLElement);
  const menu = async (title: string, action: string) => {
    await user.click(screen.getByRole('button', { name: `${title}操作` }));
    await user.click(await screen.findByRole('menuitem', { name: action }));
  };
  await user.click(screen.getByRole('button', { name: '封面' }));
  fireEvent.change(screen.getByRole('textbox', { name: '章节名称' }), { target: { value: '新封面' } });
  persist.mockRejectedValueOnce(new Error('草稿保存失败'));
  await user.click(screen.getByRole('button', { name: '确认编辑' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('草稿保存失败');
  expect(screen.getByRole('textbox', { name: '章节名称' })).toHaveValue('新封面');
  expect(record.revision).toBe(1);
  expect(flow().phase).toBe('editing');
  await user.click(screen.getByRole('button', { name: '确认编辑' }));
  await waitFor(() => expect(record.draft?.sections[0].title).toBe('新封面'));
  await waitFor(() => expect(screen.queryByRole('textbox')).not.toBeInTheDocument());
  expect(flow()).toMatchObject({ phase: 'editing', dirty: false, busy: false });
  expect(screen.getByRole('button', { name: '继续' })).toBeDisabled();
  await menu('新封面', '新增页面');
  fireEvent.change(screen.getByRole('textbox', { name: '页面名称' }), { target: { value: '第二页' } });
  await user.click(screen.getByRole('button', { name: '确认编辑' }));
  await screen.findByRole('button', { name: '第二页' });
  expect(record.draft?.sections[0].slides).toHaveLength(2);
  await menu('第二页', '上移');
  await waitFor(() => expect(record.draft?.sections[0].slides[0].title).toBe('第二页'));
  await menu('第二页', '删除页面');
  const dialog = screen.getByRole('dialog');
  await user.click(within(dialog).getByRole('button', { name: '删除' }));
  await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  expect(record.draft?.sections[0].slides.map(page => page.id)).toEqual(['sli_a']);
  expect(record.revision).toBe(5);
  expect(mutate).not.toHaveBeenCalled();
  expect(decide).not.toHaveBeenCalled();
  expect(proposal.sections[0].title).toBe('封面');
  act(() => useDeckStore.getState().setActiveDocument('design'));
  expect(screen.queryByRole('button', { name: '新封面' })).not.toBeInTheDocument();
  act(() => useDeckStore.getState().setActiveDocument('outline'));
  await screen.findByRole('button', { name: '新封面' });
  expect(record.revision).toBe(5);
  act(() => useDeckStore.getState().setContentMode('source'));
  expect(document.querySelector('code')).toHaveTextContent('"title": "新封面"');
  act(() => useDeckStore.getState().setContentMode('preview'));
  await user.click(footer().getByRole('button', { name: '保存' }));
  await waitFor(() => expect(flow()).toMatchObject({ phase: 'review', revision: 5, busy: false }));
  expect(persist).toHaveBeenCalledTimes(5);
  await user.click(footer().getByRole('button', { name: '恢复' }));
  await screen.findByRole('button', { name: '封面' });
  expect(record.draft).toEqual(proposal);
  expect(record.revision).toBe(6);
  await waitFor(() => expect(flow().phase).toBe('editing'));
  await user.click(screen.getByRole('button', { name: '封面' }));
  fireEvent.change(screen.getByRole('textbox', { name: '章节名称' }), { target: { value: '最终封面' } });
  await user.click(footer().getByRole('button', { name: '保存' }));
  await waitFor(() => expect(flow()).toMatchObject({ phase: 'review', revision: 7, dirty: false, busy: false }));
  expect(record.draft?.sections[0].title).toBe('最终封面');
  expect(mutate).not.toHaveBeenCalled();
  expect(decide).not.toHaveBeenCalled();
  await user.click(screen.getByRole('button', { name: '继续' }));
  expect(await screen.findByRole('alert')).toHaveTextContent('草稿已更新');
  expect(decide).not.toHaveBeenCalled();
  item = { ...item, approval: { ...item.approval!, revision: record.revision } };
  view.rerender(<Harness item={item} />);
  await user.click(screen.getByRole('button', { name: '继续' }));
  await waitFor(() => expect(decide).toHaveBeenCalledWith('r2', 'resa_2', 'c2', 7, 'approve', undefined));
});

it('saves and restores the isolated draft, then approves the reviewed revision only from Timeline', async () => {
  const proposal: Manifest = { title: '演示标题', language: 'zh-CN', pages: '待明确', goal: 'Agent 原始提案', audience: '开发者', requirements: [], prohibitions: [] };
  const target = { type: 'deck' as const, part: 'manifest' as const, diff: { kind: 'fields' as const, status: 'modified' as const, filename: '.manifest.json', fields: [] } };
  let record: ResourceEditApproval<Manifest> = { run_id: 'r1', call_id: 'c1', interaction_id: 'resa_1', resource: 'manifest', revision: 1,
    base_exists: true, base: { ...proposal, goal: '审批前基线' }, proposal, draft: proposal, target, state: 'pending' };
  let item: ToolActivityItem = { id: 'r1:tool:c1', type: 'tool', runId: 'r1', callId: 'c1', tool: 'edit_manifest', label: '编辑内容要求', status: 'running', timestamp: 1,
    approval: { interactionId: record.interaction_id, resource: 'manifest', revision: 1, target } };
  useProjectStore.setState({ activeProjectId: 'p1' });
  const decide = vi.spyOn(runsApi, 'decideResourceEditApproval').mockResolvedValue(undefined);
  vi.spyOn(runsApi, 'getResourceEditApproval').mockImplementation(async () => record);
  const save = vi.spyOn(runsApi, 'updateResourceEditDraft').mockImplementation(async (_run, _id, revision, draft) => {
    expect(revision).toBe(record.revision);
    record = { ...record, revision: revision + 1, draft: draft as Manifest };
    return record;
  });
  const view = render(<Harness item={item} />);
  fireEvent.click(screen.getByRole('button', { name: '手动编辑' }));
  expect(decide).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: '继续' }));
  await screen.findByRole('button', { name: '编辑演示目标' });
  expect(screen.getByRole('button', { name: '继续' })).toBeDisabled();
  act(() => { useDeckStore.getState().setActiveDocument('design'); });
  expect(screen.getByRole('button', { name: '继续' })).toBeEnabled();
  fireEvent.click(screen.getByRole('button', { name: '继续' }));
  await screen.findByRole('button', { name: '编辑演示目标' });
  fireEvent.click(screen.getByRole('button', { name: '编辑演示目标' }));
  fireEvent.change(screen.getByLabelText('演示目标'), { target: { value: '用户修改草稿' } });
  const footer = () => within(document.querySelector('.management-footer') as HTMLElement);
  fireEvent.click(footer().getByRole('button', { name: '保存' }));
  await waitFor(() => expect(record.draft?.goal).toBe('用户修改草稿'));
  await waitFor(() => expect(useResourceApprovalStore.getState().editStates[resourceApprovalKey('r1', 'resa_1')]).toMatchObject({ phase: 'review', revision: 2, dirty: false }));
  expect(decide).not.toHaveBeenCalled();
  expect(record.base?.goal).toBe('审批前基线');
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: '继续' })); });
  expect(screen.getByRole('alert')).toHaveTextContent('草稿已更新');
  expect(decide).not.toHaveBeenCalled();
  fireEvent.click(footer().getByRole('button', { name: '恢复' }));
  await waitFor(() => expect(record.draft?.goal).toBe('Agent 原始提案'));
  await waitFor(() => expect(footer().getByRole('button', { name: '保存' })).toBeEnabled());
  expect(screen.getByRole('button', { name: '继续' })).toBeDisabled();
  fireEvent.click(footer().getByRole('button', { name: '保存' }));
  await waitFor(() => expect(useResourceApprovalStore.getState().editStates[resourceApprovalKey('r1', 'resa_1')]).toMatchObject({ phase: 'review', revision: 3, dirty: false }));
  item = { ...item, approval: { ...item.approval!, revision: record.revision } };
  view.rerender(<Harness item={item} />);
  expect(screen.getByRole('button', { name: '手动编辑' })).toHaveAttribute('aria-pressed', 'true');
  expect(save).toHaveBeenCalledTimes(2);
  expect(decide).not.toHaveBeenCalled();
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: '继续' })); });
  expect(decide).toHaveBeenCalledWith('r1', 'resa_1', 'c1', 3, 'approve', undefined);
});
