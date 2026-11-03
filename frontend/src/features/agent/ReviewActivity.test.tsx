import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { ReviewResult } from '../../api/types';
import { ToolActivityRow } from './ActivityRows';
import { presentSession } from './sessionPresentation';
import { groupFixture, itemFixture, sessionFixture } from '../../test/sessionFixtures';
import type { ToolActivityItem } from './timelineTypes';

const base = { run_id: 'run_review' };
const completed = (review: ReviewResult) => ({
  ...base, call_id: 'review_1', tool: 'review_ppt', status: 'completed',
  display: { label: '已审查演示文稿' }, review,
});

describe('artifact review timeline', () => {
  it('restores and expands the complete Markdown assessment', () => {
    const review: ReviewResult = { result: '## 审查结论\n\n- 第 3 页的数据已核对。\n- 第 4 页需要保留风险说明。' };
    const group = groupFixture({ run_id: base.run_id });
    const record = itemFixture({ id: 'review', kind: 'tool', tool_call_id: 'review_1', summary: completed(review) });
    const view = presentSession(sessionFixture({ groupsById: { [group.id]: group }, orderedGroupIds: [group.id], itemsById: { [record.id]: record } }));
    const item = view.timelineItems.find((value): value is ToolActivityItem => value.type === 'tool');
    expect(item?.review).toEqual(review);
    expect(item?.status).toBe('completed');
    render(<ToolActivityRow item={item!} />);
    expect(screen.queryByRole('heading', { name: '审查结论' })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: '已审查演示文稿' }));
    expect(screen.getByRole('heading', { name: '审查结论' })).toBeInTheDocument();
    expect(screen.getAllByRole('listitem').map(value => value.textContent)).toEqual(['第 3 页的数据已核对。', '第 4 页需要保留风险说明。']);
    expect(screen.queryByText('审查通过')).not.toBeInTheDocument();
    expect(screen.queryByText('拒绝交付')).not.toBeInTheDocument();
  });

  it('does not allow expanding an in-progress review even with detail text', () => {
    render(<ToolActivityRow item={{
      id: 'review_1', runId: base.run_id, callId: 'review_1', type: 'tool', tool: 'review_ppt',
      label: '正在审查 PPT 成果', detail: '正在渲染', status: 'running', timestamp: 1,
    }} />);
    const row = screen.getByRole('button', { name: '正在审查 PPT 成果' });
    expect(row).toBeDisabled();
    expect(row).not.toHaveAttribute('aria-expanded');
    fireEvent.click(row);
    expect(screen.queryByText('正在渲染')).not.toBeInTheDocument();
  });

  it('keeps execution failures separate from assessment content', () => {
    render(<ToolActivityRow item={{ id: 'failed', runId: base.run_id, callId: 'failed', type: 'tool',
      tool: 'review_ppt', label: '未完成成果审查', status: 'failed', timestamp: 1000,
      error: { code: 'REVIEW_FAILED', message: '未完成成果审查。', retryable: false },
    }} />);
    expect(screen.queryByRole('heading', { name: '审查结论' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: '未完成成果审查' })).toBeInTheDocument();
  });
});
