import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { PreviewStatusBar } from './PreviewControls';

describe('PreviewStatusBar source switch', () => {
  it('renders conditionally and toggles the HTML source view', () => {
    const onContentModeChange = vi.fn();
    const props = {
      documentOpen: false,
      view: 'html' as const,
      onViewChange: vi.fn(),
      contentMode: 'preview' as const,
      onContentModeChange,
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

    const { rerender } = render(<PreviewStatusBar {...props} sourceToggleVisible={false} />);
    expect(screen.queryByRole('switch', { name: '显示源码' })).not.toBeInTheDocument();

    rerender(<PreviewStatusBar {...props} sourceToggleVisible />);
    const sourceSwitch = screen.getByRole('switch', { name: '显示源码' });
    expect(sourceSwitch).toHaveAttribute('aria-checked', 'false');
    fireEvent.click(sourceSwitch);
    expect(onContentModeChange).toHaveBeenCalledWith('source');

    rerender(<PreviewStatusBar {...props} contentMode="source" sourceToggleVisible />);
    expect(sourceSwitch).toHaveAttribute('aria-checked', 'true');
    fireEvent.click(sourceSwitch);
    expect(onContentModeChange).toHaveBeenLastCalledWith('preview');
  });
});
