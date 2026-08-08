import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { DesignSummary } from './DesignSummary';

describe('DesignSummary', () => {
  it('renders global visual language without slide content', () => {
    render(<DesignSummary design={{
      version: '3.0',
      revision: 3,
      project: 'pro_aaaaaa',
      theme: 'swiss-modern',
      direction: 'minimal geometric accent',
      density: 'medium',
      chrome: [{ type: 'page_number', placement: 'bottom-right', style: 'tiny muted mono counter' }],
      created_at: 1,
      updated_at: 2,
    }} />);
    expect(screen.getByText('全局视觉规范')).toBeInTheDocument();
    expect(screen.getByText('rev 3')).toBeInTheDocument();
    expect(screen.getByText('minimal geometric accent')).toBeInTheDocument();
    expect(screen.getByText('swiss-modern · medium')).toBeInTheDocument();
    expect(screen.getByText('chrome: page_number@bottom-right')).toBeInTheDocument();
  });
});
