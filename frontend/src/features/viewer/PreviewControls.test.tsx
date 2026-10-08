import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { PreviewStatusBar } from './PreviewControls';

describe('PreviewStatusBar', () => {
  it('keeps the source entry out of the status bar', () => {
    const props = {
      documentOpen: false,
      view: 'html' as const,
      onViewChange: vi.fn(),
      pageControlsDisabled: false,
      pageIndex: 0,
      pageCount: 1,
      onPrevious: vi.fn(),
      onNext: vi.fn(),
      zoom: 1,
      zoomMin: 0.5,
      zoomMax: 3,
      zoomEnabled: true,
      onZoomOut: vi.fn(),
      onZoomIn: vi.fn(),
    };

    render(<PreviewStatusBar {...props} />);
    expect(screen.queryByRole('switch', { name: '显示源码' })).not.toBeInTheDocument();
  });
});
