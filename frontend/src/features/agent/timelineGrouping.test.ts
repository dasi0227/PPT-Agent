import { describe, expect, it } from 'vitest';
import type { TimelineItem } from './eventReducer';
import { groupTimelineItems } from './timelineGrouping';

describe('timeline grouping', () => {
  it('folds an interrupted paused run before the next user turn', () => {
    const items: TimelineItem[] = [
      {
        id: 'old:user',
        type: 'user_turn',
        runId: 'old',
        text: '生成演示文稿',
        timestamp: 1,
      },
      {
        id: 'old:tool',
        type: 'tool',
        runId: 'old',
        callId: 'search',
        tool: 'search_refs',
        label: '已完成参考检索',
        status: 'completed',
        timestamp: 2,
      },
      {
        id: 'old:terminal',
        type: 'terminal_notice',
        runId: 'old',
        status: 'canceled',
        reason: 'superseded',
        message: '此前任务因服务中断而暂停，已停止执行。',
        affectedTargets: [],
        timestamp: 3,
      },
      {
        id: 'new:user',
        type: 'user_turn',
        runId: 'new',
        text: '换一个方向',
        timestamp: 4,
      },
    ];

    const grouped = groupTimelineItems(items);
    expect(grouped).toHaveLength(3);
    expect(grouped[1]).toMatchObject({
      kind: 'run_summary',
      runId: 'old',
      status: 'canceled',
      processEntries: [expect.objectContaining({ kind: 'item' })],
    });
    expect(grouped[2]).toMatchObject({
      kind: 'item',
      item: { type: 'user_turn', text: '换一个方向' },
    });
  });

  it('groups three consecutive successful commands but keeps two separate', () => {
    const command = (id: string): TimelineItem => ({
      id,
      type: 'tool',
      runId: 'r1',
      callId: id,
      tool: 'run_command',
      label: '已执行命令',
      status: 'completed',
      command: { text: `pwd ${id}`, status: 'completed' },
      timestamp: 1,
    });

    expect(groupTimelineItems([command('1'), command('2')]).map((entry) => entry.kind))
      .toEqual(['item', 'item']);
    expect(groupTimelineItems([command('1'), command('2'), command('3')]))
      .toEqual([expect.objectContaining({ kind: 'tool_group', items: expect.arrayContaining([
        expect.objectContaining({ callId: '1' }),
        expect.objectContaining({ callId: '2' }),
        expect.objectContaining({ callId: '3' }),
      ]) })]);
  });
});
