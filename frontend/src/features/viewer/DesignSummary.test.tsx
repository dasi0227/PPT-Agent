import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { DesignSummary } from './DesignSummary';

describe('DesignSummary', () => {
  it('shows visual direction, ordered layout preferences and decoration placements', () => {
    render(<DesignSummary design={{
      direction: 'Use engineering diagrams',
      layout_preferences: ['Keep a spacious grid', 'Use fewer cards'],
      decorations: { page_number: 'bottom-right', section_title: 'top-left', deck_title: 'none', key_message: 'none' },
    }} />);
    expect(screen.getByText('全局视觉规范')).toBeInTheDocument();
    expect(screen.queryByText('主题')).not.toBeInTheDocument();
    expect(screen.getByText('Use engineering diagrams')).toBeInTheDocument();
    expect(screen.getByText('Keep a spacious grid')).toBeInTheDocument();
    expect(screen.getByText('Use fewer cards')).toBeInTheDocument();
    expect(screen.getByText('页码')).toBeInTheDocument();
    expect(screen.getByText('右下')).toBeInTheDocument();
    expect(screen.getByText('左上')).toBeInTheDocument();
    expect(screen.getAllByText('暂不展示')).toHaveLength(2);
  });

  it('shows empty visual direction and layout preferences without stored placeholders', () => {
    render(<DesignSummary design={{
      direction: '', layout_preferences: [],
      decorations: { page_number: 'bottom-right', section_title: 'top-left', deck_title: 'none', key_message: 'none' },
    }} />);
    expect(screen.getByText('暂无视觉方向')).toBeInTheDocument();
    expect(screen.getByText('暂无排版偏好')).toBeInTheDocument();
  });
});
