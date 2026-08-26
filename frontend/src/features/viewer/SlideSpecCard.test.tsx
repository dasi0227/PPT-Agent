import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { SlideSpec } from '../../api/types';
import { SlideSpecCard } from './SlideSpecCard';

const spec: SlideSpec = {
  version: '4.0', revision: 2, project_id: 'p1', slide_id: 'slide-stable',
  title: '预算正在增长', key_message: '投入正在转为正式预算',
  elements: [{ type: 'chart', intent: '用数字与趋势图展示连续增长' }],
  layout: 'data-story', created_at: 1, updated_at: 2,
};

describe('SlideSpecCard', () => {
  it('renders semantic spec fields and state', () => {
    render(<SlideSpecCard spec={spec} state="spec_stale" role="evidence" />);
    expect(screen.getByText('论据')).toBeInTheDocument();
    expect(screen.getByText('预算正在增长')).toBeInTheDocument();
    expect(screen.getByText('投入正在转为正式预算')).toBeInTheDocument();
    expect(screen.getByText('Layout: data-story')).toBeInTheDocument();
    expect(screen.getByText('chart')).toBeInTheDocument();
    expect(screen.getByText('设计稿有更新')).toBeInTheDocument();
  });
});
