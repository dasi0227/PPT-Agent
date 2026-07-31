import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { DesignSpecSummary } from './DesignSpecSummary';

describe('DesignSpecSummary', () => {
  it('renders global visual language without slide content', () => {
    render(<DesignSpecSummary spec={{
      schema_version: '2.0',
      revision: 3,
      canvas: { ratio: '16:9' },
      palette: ['#111827', '#2563eb'],
      typography: { heading: 'Inter' },
      spacing: { unit: 8 },
      radius: { card: 16 },
      shadows: { card: 'soft' },
      layout_system: { grid: '12-col', density: 'medium' },
      signature: 'minimal geometric accent',
      motion: { policy: 'restrained' },
    }} />);
    expect(screen.getByText('全局视觉规范')).toBeInTheDocument();
    expect(screen.getByText('rev 3')).toBeInTheDocument();
    expect(screen.getByText('minimal geometric accent')).toBeInTheDocument();
    expect(screen.getByText('12-col · medium')).toBeInTheDocument();
  });
});
