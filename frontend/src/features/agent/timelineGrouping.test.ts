import { describe, expect, it } from 'vitest';
import type { TimelineItem } from './eventReducer';
import { groupTimelineItems, visibleTimelineItems } from './timelineGrouping';

describe('tool failure visibility', () => {
  it('hides failed attempts before grouping and restores them without losing terminal causes or user decisions', () => {
    const items: TimelineItem[] = [
      { id: 'failed', type: 'tool', runId: 'run', callId: 'failed', tool: 'render_slide', label: '渲染失败', status: 'failed', timestamp: 1,
        error: { code: 'RENDER_FAILED', message: '页面无法渲染。', retryable: false } },
      { id: 'blocked', type: 'tool', runId: 'run', callId: 'blocked', tool: 'run_command', label: '参数错误', status: 'blocked', timestamp: 2 },
      { id: 'success', type: 'tool', runId: 'run', callId: 'success', tool: 'render_slide', label: '已渲染', status: 'completed', timestamp: 3 },
      { id: 'rejected', type: 'tool', runId: 'run', callId: 'rejected', tool: 'edit_manifest', label: '未应用', status: 'failed', timestamp: 4,
        error: { code: 'RESOURCE_EDIT_REJECTED', message: '用户拒绝了本次资源编辑。', retryable: false } },
      { id: 'terminal', type: 'terminal_notice', runId: 'run', status: 'failed', message: '任务已停止。原因：页面无法渲染。', affectedTargets: [], timestamp: 5 },
    ];
    const hidden = visibleTimelineItems(items, false);
    expect(hidden.map(item => item.id)).toEqual(['success', 'rejected', 'terminal']);
    expect(groupTimelineItems(hidden)[0]).toMatchObject({ kind: 'run_summary', terminalItem: { id: 'terminal' } });
    expect(visibleTimelineItems(items, true)).toBe(items);
    expect(items).toHaveLength(5);
  });
});

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
        callId: 'read',
        tool: 'read_resource',
        label: '已读取整份结构',
        status: 'completed',
        timestamp: 2,
      },
      {
        id: 'old:terminal',
        type: 'terminal_notice',
        runId: 'old',
        status: 'canceled',
        reason: 'superseded',
        message: '此前任务因服务中断而结束。',
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

  it('groups two or more consecutive successful commands but keeps a single one separate', () => {
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

    expect(groupTimelineItems([command('1')]).map((entry) => entry.kind))
      .toEqual(['item']);
    expect(groupTimelineItems([command('1'), command('2')]))
      .toEqual([expect.objectContaining({ kind: 'tool_group', items: expect.arrayContaining([
        expect.objectContaining({ callId: '1' }),
        expect.objectContaining({ callId: '2' }),
      ]) })]);
    expect(groupTimelineItems([command('1'), command('2'), command('3')]))
      .toEqual([expect.objectContaining({ kind: 'tool_group', items: expect.arrayContaining([
        expect.objectContaining({ callId: '1' }),
        expect.objectContaining({ callId: '2' }),
        expect.objectContaining({ callId: '3' }),
      ]) })]);
  });
});
