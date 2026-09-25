import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { SlideSpec } from '../../api/types';
import { SlideSpecCard } from './SlideSpecCard';

const spec: SlideSpec = {
  key_message: '投入正在转为正式预算',
  elements: [{ type: 'chart', intent: '用数字与趋势图展示连续增长' }],
  layout: 'data-story',
};

describe('SlideSpecCard', () => {
  it('renders semantic spec fields without synchronization status', () => {
    render(<SlideSpecCard title="预算正在增长" spec={spec} role="evidence" />);
    expect(screen.getByText('论据')).toBeInTheDocument();
    expect(screen.getByText('预算正在增长')).toBeInTheDocument();
    expect(screen.getByText('投入正在转为正式预算')).toBeInTheDocument();
    expect(screen.getByText('布局建议：data-story')).toBeInTheDocument();
    expect(screen.getByText('图表')).toBeInTheDocument();
    expect(screen.queryByText('设计稿有更新')).not.toBeInTheDocument();
  });
});
