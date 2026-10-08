import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { parsePublicEvent } from '../../api/sse';
import type { ReviewResult } from '../../api/types';
import { ToolActivityRow } from './ActivityRows';
import { hydrateRunFromHistory } from './historyHydrator';
import type { ToolActivityItem } from './eventReducer';

const base = { schema_version: 6, run_id: 'run_review', occurred_at: '2026-09-29T10:00:00Z' };
const completed = (review: ReviewResult) => ({
  ...base, call_id: 'review_1', tool: 'review_task', status: 'completed',
  display: { label: '已审查演示文稿' }, review,
});

describe('artifact review timeline', () => {
  it.each([
    ['approve', '审查通过'], ['revise', '需要核实／修订'], ['refuse', '拒绝交付'],
  ] as const)('restores and expands %s with plain-text reasons', (type, title) => {
    const review: ReviewResult = { decision: type, reasons: ['第 3 页的数据已核对。', '第 4 页需要保留风险说明。'] };
    const { items } = hydrateRunFromHistory([
      { seq: 1, ts: 1, run_id: base.run_id, turn: 'agent', type: 'tool.started', data: { ...base, call_id: 'review_1', tool: 'review_task', display: { label: '正在审查 PPT 成果' } } },
      { seq: 2, ts: 2, run_id: base.run_id, turn: 'agent', type: 'tool.completed', data: completed(review) },
    ]);
    const item = items.find((value): value is ToolActivityItem => value.type === 'tool');
    expect(item?.review).toEqual(review);
    expect(item?.status).toBe('completed');
    render(<ToolActivityRow item={item!} />);
    expect(screen.queryByText(title)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '已审查演示文稿' }));
    expect(screen.getByText(title)).toBeInTheDocument();
    expect(screen.getAllByRole('listitem').map(value => value.textContent)).toEqual(review.reasons);
  });

  it('does not allow expanding an in-progress review even with detail text', () => {
    render(<ToolActivityRow item={{
      id: 'review_1', runId: base.run_id, callId: 'review_1', type: 'tool', tool: 'review_task',
      label: '正在审查 PPT 成果', detail: '正在渲染', status: 'running', timestamp: 1,
    }} />);
    const row = screen.getByRole('button', { name: '正在审查 PPT 成果' });
    expect(row).toBeDisabled();
    expect(row).not.toHaveAttribute('aria-expanded');
    fireEvent.click(row);
    expect(screen.queryByText('正在渲染')).not.toBeInTheDocument();
  });

  it('distinguishes review execution failures from assessment results', () => {
    expect(parsePublicEvent('tool.completed', completed({ decision: 'approve', reasons: [] }))).toBeNull();
    expect(parsePublicEvent('tool.completed', { ...completed({ decision: 'revise', reasons: ['需核实来源。'] }), tool: 'read_resource' })).toBeNull();
    const failure = { ...base, call_id: 'review_1', tool: 'review_task', status: 'failed', display: { label: '未完成成果审查' }, error: { code: 'REVIEW_FAILED', message: '未完成成果审查。', retryable: false } };
    expect(parsePublicEvent('tool.completed', failure)).not.toBeNull();
    expect(parsePublicEvent('tool.completed', { ...failure, review: { decision: 'revise', reasons: ['服务异常。'] } })).toBeNull();
  });
});
