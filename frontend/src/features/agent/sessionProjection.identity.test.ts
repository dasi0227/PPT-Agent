import { describe, expect, it } from 'vitest';
import type { PublicObject, TimelineRecord } from '../../api/session';
import type { SessionState } from '../../stores/sessionStore';
import { commandFixture, groupFixture, interactionFixture, itemFixture, runFixture, sessionFixture, threadFixture } from '../../test/sessionFixtures';
import { presentSession } from './sessionPresentation';
import { groupTimelineItems } from './timelineGrouping';
import type { TimelineItem } from './timelineTypes';

function sessionWith(records: TimelineRecord[], overrides: Partial<SessionState> = {}): SessionState {
  const group = groupFixture();
  return sessionFixture({
    thread: threadFixture({ active_run_id: 'r1' }), runsById: { r1: runFixture() },
    groupsById: { [group.id]: group }, orderedGroupIds: [group.id],
    itemsById: Object.fromEntries(records.map(record => [record.id, record])), isEmpty: false, ...overrides,
  });
}

const textRecord = () => itemFixture({ id: 'thinking:1', kind: 'thinking', summary: { text: '思考中' } });
const toolRecord = () => itemFixture({ id: 'tool:1', kind: 'tool', position: '2', tool_call_id: 'c1',
  summary: { tool: 'edit_meta', status: 'completed', display: { label: '编辑基本信息' } } });

describe('session timeline projection identity', () => {
  it('keeps the item array for run, connection and loading changes, including group progress updates', () => {
    const source = sessionWith([textRecord(), toolRecord()]);
    const initial = presentSession(source);
    const run = { ...source.runsById.r1, status: 'waiting' as const, active_duration_ms: 3000, updated_at: 2000 };
    const waiting = presentSession({ ...source, runsById: { r1: run } });
    expect(waiting.status).toBe('waiting');
    expect(waiting.timelineItems).toBe(initial.timelineItems);
    expect(waiting.groups).toBe(initial.groups);

    const loading = { ...source, connectionStatus: 'reconnecting' as const,
      detailStates: { 'tool:1': { status: 'loading' as const, error: null } } };
    expect(presentSession(loading).timelineItems).toBe(initial.timelineItems);
    const group = { ...source.groupsById['run:r1'], status: 'waiting', summary: { active_duration_ms: 4000 } };
    expect(presentSession({ ...loading, groupsById: { [group.id]: group }, orderedGroupIds: [group.id] }).timelineItems)
      .toBe(initial.timelineItems);
  });

  it('updates terminal group data and group membership without recreating unrelated items', () => {
    const text = textRecord();
    const final = itemFixture({ id: 'final:1', kind: 'final', position: '2',
      summary: { text: '已完成', affected_targets: [{ type: 'deck', part: 'meta' }] } });
    const notice = itemFixture({ id: 'notice:1', kind: 'terminal_notice', position: '3', summary: { reason: 'user_requested' } });
    const source = sessionWith([text, final, notice]);
    const initial = presentSession(source).timelineItems;
    const group = { ...source.groupsById['run:r1'], status: 'canceled',
      summary: { active_duration_ms: 6000, terminal: { affected_targets: [{ type: 'deck', part: 'meta' }] } } };
    const next = presentSession({ ...source, groupsById: { [group.id]: group } }).timelineItems;
    expect(next[0]).toBe(initial[0]);
    expect(next[1]).toMatchObject({ type: 'final', durationMs: 6000, affectedTargets: [{ type: 'deck', part: 'meta' }] });
    expect(next[2]).toMatchObject({ type: 'terminal_notice', status: 'canceled', message: '你已停止本次任务。' });
    const done = presentSession({ ...source, groupsById: { [group.id]: { ...group, status: 'done' } } }).timelineItems;
    expect(done.map(item => item.id)).toEqual(['thinking:1', 'final:1']);
    expect(done[0]).toBe(initial[0]);
    expect(done[1]).toBe(next[1]);
    const moved = presentSession({ ...source, groupsById: { [group.id]: { ...group, run_id: 'r2' } } }).timelineItems;
    expect(moved.every(item => item.runId === 'r2')).toBe(true);
  });

  it('refreshes newer command entities and resolved command/body details while preserving other items', () => {
    const command = commandFixture();
    const record = itemFixture({ id: 'command:cmd1', kind: 'command', position: '2', summary: command as unknown as PublicObject });
    const text = textRecord();
    const source = sessionWith([text, record], { commandsById: { cmd1: command } });
    const initial = presentSession(source).timelineItems;
    const current = { ...command, entity_revision: '2', status: 'completed' as const, content_ref: 'command-full' };
    const currentSource = { ...source, commandsById: { cmd1: current } };
    const deferred = presentSession(currentSource).timelineItems;
    expect(deferred[0]).toBe(initial[0]);
    expect(deferred[1]).toMatchObject({ type: 'deferred', detailRef: 'command-full' });
    const detail = { ...current, result: { content: '润色结果' } } as unknown as PublicObject;
    const resolved = presentSession({ ...currentSource, detailsByRef: { 'command-full': detail } }).timelineItems;
    expect(resolved[0]).toBe(initial[0]);
    expect(resolved[1]).toMatchObject({ type: 'command', content: '润色结果', status: 'completed' });

    const bodyRecord = { ...text, body_ref: 'body' };
    const bodySource = sessionWith([bodyRecord]);
    const excerpt = presentSession(bodySource).timelineItems;
    const body = presentSession({ ...bodySource, detailsByRef: { body: { text: '完整正文' } } }).timelineItems;
    expect(body[0]).toMatchObject({ text: '完整正文', detailRef: undefined });
    expect(body[0]).not.toBe(excerpt[0]);

    const pinnedSource = sessionWith([text], { commandsById: { cmd1: command } });
    const pinned = presentSession(pinnedSource).timelineItems;
    expect(presentSession({ ...pinnedSource, detailsByRef: { unrelated: {} } }).timelineItems).toBe(pinned);
    expect(presentSession({ ...pinnedSource, commandsById: { cmd1: { ...command, phase: 2, entity_revision: '2' } } }).timelineItems[1])
      .toMatchObject({ phase: 2 });
  });

  it('keeps deferred approvals separate, then joins their detail and newer answers onto the current tool', () => {
    const tool = toolRecord();
    const approval = interactionFixture({ id: 'a1', kind: 'resource', request: null, detail_ref: 'approval-full' });
    const record = itemFixture({ id: 'interaction:a1', kind: 'interaction', position: '3', interaction_id: 'a1',
      summary: approval as unknown as PublicObject });
    const source = sessionWith([textRecord(), tool, record], { interactionsById: { a1: approval } });
    const deferred = presentSession(source).timelineItems;
    expect(deferred.map(item => item.type)).toEqual(['thinking', 'tool', 'deferred']);
    const full = { ...approval, request: { resource: 'meta', target: { type: 'deck', part: 'meta' } } };
    const detailedSource = { ...source, detailsByRef: { 'approval-full': full as unknown as PublicObject } };
    const joined = presentSession(detailedSource).timelineItems;
    expect(joined).toHaveLength(2);
    expect(joined[0]).toBe(deferred[0]);
    expect(joined[1]).toMatchObject({ id: 'tool:1', interactionState: 'pending', approval: { interactionId: 'a1' } });
    expect(presentSession({ ...detailedSource, detailsByRef: { ...detailedSource.detailsByRef, unrelated: {} } }).timelineItems).toBe(joined);

    const answered = { ...full, entity_revision: '2', request_revision: '2', state: 'answered' as const,
      detail_ref: undefined, answer: { decision: 'reject', feedback: '调整后再试' } };
    const answeredSource = { ...source, interactionsById: { a1: answered } };
    const updated = presentSession(answeredSource).timelineItems;
    expect(updated[0]).toBe(deferred[0]);
    expect(updated[1]).toMatchObject({ interactionState: 'answered', requestRevision: '2',
      approval: { answer: { decision: 'reject', feedback: '调整后再试' } } });
    const changedTool = { ...tool, entity_revision: '2', summary: { ...tool.summary, status: 'failed' } };
    expect(presentSession({ ...answeredSource, itemsById: { ...source.itemsById, [tool.id]: changedTool } }).timelineItems[1])
      .toMatchObject({ status: 'failed', approval: { answer: { decision: 'reject' } } });
    const newScene = presentSession({ ...answeredSource, sceneRevision: '2' }).timelineItems;
    expect(newScene[0]).not.toBe(updated[0]);
    expect(newScene[1]).not.toBe(updated[1]);
    expect(newScene.every(item => item.sceneRevision === '2')).toBe(true);
  });

  it('updates pinned interaction timestamps, optimistic delivery and changed records independently', () => {
    const text = textRecord();
    const question = interactionFixture();
    const input = { clientMessageId: 'local1', text: '追加要求', createdAt: 2000, status: 'sending' as const };
    const source = sessionWith([text], { interactionsById: { q1: question }, optimisticInputs: { local1: input } });
    const initial = presentSession(source).timelineItems;
    const laterSource = { ...source, runsById: { r1: { ...source.runsById.r1, updated_at: 4000 } } };
    const later = presentSession(laterSource).timelineItems;
    expect(later[0]).toBe(initial[0]);
    expect(later[1]).toMatchObject({ type: 'question', timestamp: 4000 });
    expect(later[2]).toBe(initial[2]);
    const failed = presentSession({ ...laterSource, optimisticInputs: { local1: { ...input, status: 'error' } } }).timelineItems;
    expect(failed[0]).toBe(initial[0]);
    expect(failed[1]).toBe(later[1]);
    expect(failed[2]).toMatchObject({ deliveryStatus: 'rejected' });
    const changed = { ...text, summary: { text: '思考更新' } };
    const changedItems = presentSession({ ...laterSource, itemsById: { [changed.id]: changed } }).timelineItems;
    expect(changedItems[0]).toMatchObject({ text: '思考更新' });
    expect(changedItems[1]).toBe(later[1]);
    expect(changedItems[2]).toBe(initial[2]);
  });
});

it('groups indexed items in persisted group order and keeps unpaged items in their original order', () => {
  const completed = groupFixture({ id: 'run:old', run_id: 'old', status: 'done', summary: { active_duration_ms: 5000 } });
  const current = groupFixture();
  const items: TimelineItem[] = [
    { id: 'local', type: 'user_turn', text: '待发送', timestamp: 1 },
    { id: 'current', type: 'thinking', text: '运行中', messageId: 'current', groupId: current.id, timestamp: 2 },
    { id: 'old-input', type: 'user_turn', text: '旧请求', groupId: completed.id, timestamp: 3 },
    { id: 'read-1', type: 'tool', tool: 'read_meta', callId: 'read-1', label: '读取', status: 'completed', groupId: completed.id, timestamp: 4 },
    { id: 'read-2', type: 'tool', tool: 'read_meta', callId: 'read-2', label: '读取', status: 'completed', groupId: completed.id, timestamp: 5 },
    { id: 'old-final', type: 'final', text: '完成', messageId: 'final', affectedTargets: [], groupId: completed.id, timestamp: 6 },
    { id: 'unpaged', type: 'thinking', text: '未加载分组', messageId: 'unpaged', groupId: 'run:unpaged', timestamp: 7 },
  ];
  const entries = groupTimelineItems(items, [completed, current]);
  expect(entries).toMatchObject([
    { kind: 'item', item: { id: 'old-input' } },
    { kind: 'run_summary', id: completed.id, durationMs: 5000, terminalItem: { id: 'old-final' },
      processEntries: [{ kind: 'tool_group', items: [{ id: 'read-1' }, { id: 'read-2' }] }] },
    { kind: 'group_page', groupId: current.id },
    { kind: 'item', item: { id: 'current' } },
    { kind: 'item', item: { id: 'local' } },
    { kind: 'item', item: { id: 'unpaged' } },
  ]);
});
