import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { DesignSummary } from './DesignSummary';

describe('DesignSummary', () => {
  it('renders global visual language without slide content', () => {
    render(<DesignSummary design={{
      schema_version: '3.0',
      revision: 3,
      project_id: 'p1',
      canvas: { ratio: '16:9' },
      palette: ['#111827', '#2563eb'],
      typography: { heading: 'Inter' },
      spacing: { unit: 8 },
      radius: { card: 16 },
      shadows: { card: 'soft' },
      layout_system: { grid: '12-col', density: 'medium' },
      signature: 'minimal geometric accent',
      motion: { policy: 'restrained' },
      created_at: 1,
      updated_at: 2,
    }} />);
    expect(screen.getByText('全局视觉规范')).toBeInTheDocument();
    expect(screen.getByText('rev 3')).toBeInTheDocument();
    expect(screen.getByText('minimal geometric accent')).toBeInTheDocument();
    expect(screen.getByText('12-col · medium')).toBeInTheDocument();
  });
});
