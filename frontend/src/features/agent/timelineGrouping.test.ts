import { describe, expect, it } from 'vitest';
import { groupFixture } from '../../test/sessionFixtures';
import type { TimelineItem, ToolActivityItem } from './timelineTypes';
import { groupTimelineItems, visibleTimelineItems } from './timelineGrouping';

const tool = (id: string, extra: Partial<ToolActivityItem> = {}): ToolActivityItem => ({
  id, type: 'tool', groupId: 'run:r1', runId: 'r1', callId: id, tool: 'read_resource',
  label: '读取资源', timestamp: 1000, status: 'completed', ...extra,
});
describe('persisted timeline groups', () => {
  it('keeps terminal status even when failed tools are hidden', () => {
    const items: TimelineItem[] = [tool('failed', { status: 'failed' })];
    const entries = groupTimelineItems(visibleTimelineItems(items, false), [groupFixture({ status: 'failed' })]);
    expect(entries).toMatchObject([{ kind: 'run_summary', status: 'failed', processEntries: [] }]);
    expect(visibleTimelineItems(items, true)).toBe(items);
  });
  it('does not infer task completion from a final message', () => {
    const final: TimelineItem = { id: 'final', type: 'final', groupId: 'run:r1', runId: 'r1',
      messageId: 'm1', text: '阶段结果', timestamp: 1000, affectedTargets: [] };
    expect(groupTimelineItems([final], [groupFixture()]).some(entry => entry.kind === 'run_summary')).toBe(false);
    expect(groupTimelineItems([final], [groupFixture({ status: 'done' })])).toMatchObject([
      { kind: 'run_summary', status: 'completed', terminalItem: { id: 'final' } },
    ]);
  });
  it('groups completed tools while keeping deferred details and pending approvals accessible', () => {
    const group = groupFixture({ status: 'done' });
    const entries = groupTimelineItems([tool('a'), tool('b'), tool('c', { detailRef: 'detail' }),
      tool('d', { interactionState: 'pending' })], [group]);
    expect(entries).toMatchObject([{ kind: 'run_summary', processEntries: [
      { kind: 'tool_group', items: [{ id: 'a' }, { id: 'b' }] }, { kind: 'item', item: { id: 'c' } }, { kind: 'item', item: { id: 'd' } },
    ] }]);
  });
});
