import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { MilestoneItem, ToolActivityItem } from './eventReducer';
import { MilestoneRow, ToolGroupRow } from './ActivityRows';
import { TimelineDisclosure } from './TimelineDisclosure';

describe('timeline disclosure motion', () => {
  it('keeps expansion state on the shared disclosure container', () => {
    const { container, rerender } = render(
      <TimelineDisclosure open={false}>
        <span>历史活动</span>
      </TimelineDisclosure>,
    );

    expect(container.firstChild).toHaveAttribute('data-state', 'closed');
    expect(container.firstChild).toHaveAttribute('aria-hidden', 'true');

    rerender(
      <TimelineDisclosure open>
        <span>历史活动</span>
      </TimelineDisclosure>,
    );

    expect(container.firstChild).toHaveAttribute('data-state', 'open');
    expect(container.firstChild).toHaveAttribute('aria-hidden', 'false');
  });

  it('does not attach a dedicated entry animation to milestone rows', () => {
    const item: MilestoneItem = {
      id: 'run-1:milestone:1',
      type: 'milestone',
      runId: 'run-1',
      messageId: 'message-1',
      text: '完成页面编排计划',
      completedStepIds: ['step-1'],
      timestamp: 1,
    };
    const { container } = render(<MilestoneRow item={item} />);

    expect(container.firstChild).not.toHaveClass('motion-safe:animate-[timeline-enter_120ms_ease-out]');
  });

  it('opens grouped tool rows through the shared disclosure', () => {
    const item: ToolActivityItem = {
      id: 'run-1:tool:1',
      type: 'tool',
      runId: 'run-1',
      callId: 'call-1',
      tool: 'read_ppt',
      label: '已读取演示内容',
      status: 'completed',
      timestamp: 1,
    };
    const { container } = render(<ToolGroupRow items={[item, { ...item, id: 'run-1:tool:2', callId: 'call-2' }]} />);
    const disclosure = container.querySelector('.timeline-disclosure');

    expect(disclosure).toHaveAttribute('data-state', 'closed');
    fireEvent.click(screen.getByRole('button', { name: /已读取/ }));
    expect(disclosure).toHaveAttribute('data-state', 'open');
    expect(container.querySelector('.timeline-disclosure-rows')).toBeInTheDocument();
  });
});
