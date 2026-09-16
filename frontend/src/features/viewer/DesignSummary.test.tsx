import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { DesignSummary } from './DesignSummary';

describe('DesignSummary', () => {
  it('renders global visual language without slide content', () => {
    render(<DesignSummary design={{
      version: '4.0',
      revision: 3,
      project_id: 'pro_aaaaaa',
      theme: 'swiss-modern',
      direction: 'minimal geometric accent',
      chrome: [{ type: 'page_number', placement: 'bottom-right', style: 'tiny muted mono counter' }],
      created_at: 1,
      updated_at: 2,
    }} />);
    expect(screen.getByText('全局视觉规范')).toBeInTheDocument();
    expect(screen.getByText('主题')).toBeInTheDocument();
    expect(screen.getByText('swiss-modern')).toBeInTheDocument();
    expect(screen.getByText('视觉方向')).toBeInTheDocument();
    expect(screen.getByText('minimal geometric accent')).toBeInTheDocument();
    expect(screen.getByText('页面装饰')).toBeInTheDocument();
    expect(screen.getByText('页码（右下）')).toBeInTheDocument();
  });

  it('labels an undecided visual direction instead of implying one', () => {
    render(<DesignSummary design={{
      version: '4.0', revision: 1, project_id: 'pro_aaaaaa', theme: 'swiss-modern', direction: '待确定',
      chrome: [], created_at: 1, updated_at: 1,
    }} />);

    expect(screen.getByText('视觉方向待确定')).toBeInTheDocument();
  });
});
