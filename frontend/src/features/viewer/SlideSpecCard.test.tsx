import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { SlideSpec } from '../../api/types';
import { SlideSpecCard } from './SlideSpecCard';

const spec: SlideSpec = {
  schema_version: '3.0', revision: 2, project_id: 'p1', slide_id: 'slide-stable',
  source_outline_revision: 1, section_id: 'section-main',
  role: 'evidence', title: '预算正在增长', key_message: '投入正在转为正式预算',
  content: { summary: '市场证据', points: ['连续增长', '业务部门加入'] },
  visual_intent: { archetype: 'data-story', description: '数字与趋势图', asset_queries: [] },
  speaker_notes: '', created_at: 1, updated_at: 2,
};

describe('SlideSpecCard', () => {
  it('renders semantic spec fields and state', () => {
    render(<SlideSpecCard spec={spec} state="spec_stale" />);
    expect(screen.getByText('预算正在增长')).toBeInTheDocument();
    expect(screen.getByText('投入正在转为正式预算')).toBeInTheDocument();
    expect(screen.getByText('data-story')).toBeInTheDocument();
    expect(screen.getByText('设计稿有更新')).toBeInTheDocument();
  });
});
