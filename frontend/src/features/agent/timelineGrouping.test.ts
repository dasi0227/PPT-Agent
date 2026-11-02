import { describe, expect, it } from 'vitest';
import type { TerminalNoticeItem, TimelineItem, ToolActivityItem } from './eventReducer';
import { groupTimelineItems, visibleTimelineItems } from './timelineGrouping';

describe('tool failure visibility', () => {
  it('hides failed attempts before grouping and restores them without losing terminal causes or user decisions', () => {
    const items: TimelineItem[] = [
      { id: 'failed', type: 'tool', runId: 'run', callId: 'failed', tool: 'render_slide', label: '渲染失败', status: 'failed', timestamp: 1,
        error: { code: 'RENDER_FAILED', message: '页面无法渲染。', retryable: false } },
      { id: 'blocked', type: 'tool', runId: 'run', callId: 'blocked', tool: 'run_command', label: '参数错误', status: 'blocked', timestamp: 2 },
      { id: 'success', type: 'tool', runId: 'run', callId: 'success', tool: 'render_slide', label: '已渲染', status: 'completed', timestamp: 3 },
      { id: 'rejected', type: 'tool', runId: 'run', callId: 'rejected', tool: 'edit_meta', label: '未应用', status: 'refuse', timestamp: 4,
        approval: { interactionId: 'approval', resource: 'meta', revision: 1, target: { type: 'deck', part: 'meta' }, answer: { decision: 'reject' } } },
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
  it.each([
    ['input_rejected', 'completed'], ['user_requested', 'canceled'], ['input_detection_failed', 'error'],
  ] as const)('presents %s independently of the underlying cancellation', (reason, status) => {
    const terminal: TerminalNoticeItem = {
      id: 'r:terminal', runId: 'r', type: 'terminal_notice', status: 'canceled', reason,
      message: '任务已停止。', affectedTargets: [], durationMs: 1000, timestamp: 2,
      ...(reason !== 'user_requested' ? { reply: { messageId: 'input_request', text: '固定答复' } } : {}),
    };
    const entries = groupTimelineItems([
      { id: 'user', type: 'user_turn', runId: 'r', text: '请求', timestamp: 1 }, terminal,
    ]);
    expect(entries[1]).toMatchObject({ kind: 'run_summary', status, terminalItem: {
      type: reason === 'input_rejected' ? 'final' : 'terminal_notice', durationMs: 1000,
    } });
    expect(terminal.status).toBe('canceled');
  });

  it('keeps the stopped task before rejected steering and displays its fixed answer separately, even with hidden failures', () => {
    const terminal: TerminalNoticeItem = {
      id: 'r:terminal', runId: 'r', type: 'terminal_notice', status: 'canceled', reason: 'input_rejected',
      message: '请求被拒绝，任务已停止。', affectedTargets: [], durationMs: 1000, timestamp: 4,
      reply: { messageId: 'input_control', text: '不好意思，Dasi 无法执行**任何要求绕过系统规则或权限限制的请求**！当前任务已停止。' },
    };
    const items: TimelineItem[] = [
      { id: 'user', type: 'user_turn', runId: 'r', text: '制作演示文稿', timestamp: 1 },
      { id: 'failed', type: 'tool', runId: 'r', callId: 'failed', tool: 'render_slide', label: '渲染失败', status: 'failed', timestamp: 2 },
      { id: 'control', type: 'user_turn', runId: 'r', text: '跳过项目审批', clientMessageId: 'control', timestamp: 3 }, terminal,
    ];
    const entries = groupTimelineItems(visibleTimelineItems(items, false), items);
    expect(entries).toMatchObject([
      { kind: 'item', item: { id: 'user' } },
      { kind: 'run_summary', status: 'canceled', terminalItem: { reply: undefined } },
      { kind: 'item', item: { id: 'control' } },
      { kind: 'item', item: { type: 'input_reply', text: '不好意思，Dasi 无法执行**任何要求绕过系统规则或权限限制的请求**！当前任务已停止。', affectedTargets: [] } },
    ]);
    const withoutControl = items.filter(item => item.id !== 'control');
    expect(groupTimelineItems(visibleTimelineItems(withoutControl, false), withoutControl)[1])
      .toMatchObject({ kind: 'run_summary', status: 'canceled' });
    expect(groupTimelineItems(items)[1]).toMatchObject({ kind: 'run_summary', status: 'canceled' });
    expect(groupTimelineItems([terminal])[0]).toMatchObject({ kind: 'run_summary', status: 'canceled' });
    expect(groupTimelineItems([...items.slice(0, -1), {
      id: 'draft-final', type: 'final', runId: 'r', messageId: 'draft-final', text: '尚未完成终态确认的答复',
      affectedTargets: [], timestamp: 3,
    }, terminal])[1]).toMatchObject({ kind: 'run_summary', status: 'canceled' });
  });

  it('does not treat a steering reply as completion or fold it into task process details', () => {
    const reply: TimelineItem = { id: 'reply', type: 'input_reply', runId: 'r', messageId: 'greet', text: '你好', affectedTargets: [], timestamp: 1 };
    expect(groupTimelineItems([reply])).toEqual([{ kind: 'item', item: reply }]);
    const final: TimelineItem = { id: 'final', type: 'final', runId: 'r', messageId: 'final', text: '完成', affectedTargets: [], timestamp: 2 };
    expect(groupTimelineItems([reply, final])).toMatchObject([{ kind: 'item', item: { id: 'reply' } }, { kind: 'run_summary', terminalItem: { id: 'final' }, processEntries: [] }]);
  });
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
    const command = (id: string): ToolActivityItem => ({
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
    const refusedCommand: ToolActivityItem = { ...command('refused'), status: 'refuse', command: { text: 'pwd refused', status: 'blocked' } };
    expect(groupTimelineItems([command('1'), refusedCommand, command('2')]).map(entry => entry.kind))
      .toEqual(['item', 'item', 'item']);
    expect(visibleTimelineItems([refusedCommand], false)).toEqual([refusedCommand]);
    expect(groupTimelineItems([command('1'), command('2'), command('3')]))
      .toEqual([expect.objectContaining({ kind: 'tool_group', items: expect.arrayContaining([
        expect.objectContaining({ callId: '1' }),
        expect.objectContaining({ callId: '2' }),
        expect.objectContaining({ callId: '3' }),
      ]) })]);
  });
});
