import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { runsApi } from '../../api/runs';
import { receiptFixture } from '../../test/sessionFixtures';
import type { Meta, ResourceEditApproval } from '../../api/types';
import { useProjectStore } from '../../stores/projectStore';
import { useDeckStore } from '../../stores/deckStore';
import { resourceApprovalKey, useResourceApprovalStore } from '../../stores/resourceApprovalStore';
import { ApprovalDraftEditor } from '../viewer/ApprovalDraftEditor';
import { ToolActivityRow } from './ActivityRows';
import type { ToolActivityItem } from './timelineTypes';

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

it('saves and restores the isolated draft, then approves the reviewed revision only from Timeline', async () => {
  const proposal: Meta = { title: '演示标题', language: 'zh-CN', count: '待明确', goal: 'Agent 原始提案', audience: '开发者' };
  const target = { type: 'deck' as const, part: 'meta' as const, diff: { kind: 'fields' as const, status: 'modified' as const, filename: '.meta.json', fields: [] } };
  let record: ResourceEditApproval<Meta> = { run_id: 'r1', call_id: 'c1', interaction_id: 'resa_1', resource: 'meta', revision: '1',
    base_exists: true, base: { ...proposal, goal: '审批前基线' }, proposal, draft: proposal, target, state: 'pending' };
  let item: ToolActivityItem = { id: 'r1:tool:c1', type: 'tool', runId: 'r1', callId: 'c1', tool: 'edit_meta', label: '编辑基本信息', status: 'running', timestamp: 1,
    sceneRevision: '1', requestRevision: '1', interactionState: 'pending',
    approval: { interactionId: record.interaction_id, resource: 'meta', revision: '1', target } };
  useProjectStore.setState({ activeProjectId: 'p1' });
  const decide = vi.spyOn(runsApi, 'decideResourceEditApproval').mockResolvedValue(receiptFixture());
  vi.spyOn(runsApi, 'getResourceEditApproval').mockImplementation(async () => record);
  const save = vi.spyOn(runsApi, 'updateResourceEditDraft').mockImplementation(async (_run, _id, revision, draft) => {
    expect(revision).toBe(record.revision);
    record = { ...record, revision: (BigInt(revision) + 1n).toString(), draft: draft as Meta };
    return record;
  });
  const view = render(<Harness item={item} />);
  fireEvent.click(screen.getByRole('button', { name: '手动编辑' }));
  expect(decide).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: '继续' }));
  await screen.findByRole('button', { name: '编辑演示目标' });
  expect(screen.getByRole('button', { name: '继续' })).toBeDisabled();
  act(() => { useDeckStore.getState().setActiveDocument('constraints'); });
  expect(screen.getByRole('button', { name: '继续' })).toBeEnabled();
  fireEvent.click(screen.getByRole('button', { name: '继续' }));
  await screen.findByRole('button', { name: '编辑演示目标' });
  fireEvent.click(screen.getByRole('button', { name: '编辑演示目标' }));
  fireEvent.change(screen.getByLabelText('演示目标'), { target: { value: '用户修改草稿' } });
  const footer = () => within(document.querySelector('.management-footer') as HTMLElement);
  fireEvent.click(footer().getByRole('button', { name: '保存' }));
  await waitFor(() => expect(record.draft?.goal).toBe('用户修改草稿'));
  await waitFor(() => expect(useResourceApprovalStore.getState().editStates[resourceApprovalKey('r1', 'resa_1')]).toMatchObject({ phase: 'review', revision: '2', dirty: false }));
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
  await waitFor(() => expect(useResourceApprovalStore.getState().editStates[resourceApprovalKey('r1', 'resa_1')]).toMatchObject({ phase: 'review', revision: '3', dirty: false }));
  item = { ...item, approval: { ...item.approval!, revision: record.revision } };
  view.rerender(<Harness item={item} />);
  expect(save).toHaveBeenCalledTimes(2);
  expect(decide).not.toHaveBeenCalled();
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: '继续' })); });
  expect(decide).toHaveBeenCalledWith('r1', 'resa_1', 'c1', '3', 'approve', undefined, '1');
});
