import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { ContextCompactionTimelineItem } from './eventReducer';
import { ContextCompactionActivity } from './ContextCompactionActivity';

const item: ContextCompactionTimelineItem = {
  id: 'context-compaction:cmp_1',
  type: 'context_compaction',
  compactionId: 'cmp_1',
  trigger: 'manual',
  title: '收敛上下文协议与前端实现',
  summary: '## 目标与意图\n\n继续完成正式实现。',
  beforeTokens: 0,
  afterTokens: 0,
  maxTokens: 65536,
  reclaimedTokens: 24,
  durationMs: 3600,
  timestamp: new Date('2026-09-17T06:30:00.000Z').getTime(),
};

describe('context compaction timeline presentation', () => {
  it('renders flat metadata and reveals the complete summary', () => {
    const { container } = render(<ContextCompactionActivity item={item} />);
    const button = screen.getByRole('button', { name: /compact: 收敛上下文协议与前端实现/ });
    expect(button).toHaveAttribute('aria-expanded', 'false');
    expect(screen.getByText('手动触发')).toBeInTheDocument();
    expect(screen.getByText('0.0k')).toHaveClass('text-danger');
    expect(screen.queryByText('继续完成正式实现。')).not.toBeInTheDocument();
    expect(container.querySelector('.context-compaction-card')).toBeNull();
    expect(container.querySelector('.bg-success-soft')).toBeInTheDocument();

    fireEvent.click(button);
    expect(button).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getByText('继续完成正式实现。')).toBeInTheDocument();
    expect(screen.getByText('目标与意图').closest('div')?.parentElement).toHaveClass('border-t');
  });
});
