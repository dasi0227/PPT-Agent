import { act, fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { IsolatedSlidePreview } from './IsolatedSlidePreview';

describe('IsolatedSlidePreview runtime errors', () => {
  it('shows a retry action for errors emitted by its isolated frame', () => {
    const frameWindow = { postMessage: vi.fn() } as unknown as Window;
    Object.defineProperty(window.HTMLIFrameElement.prototype, 'contentWindow', {
      configurable: true,
      get: () => frameWindow,
    });
    const { container } = render(
      <IsolatedSlidePreview
        slides={[{ id: 's1', html: '<h1>Slide</h1>' }]}
        index={0}
        title="安全预览"
      />,
    );
    const initialFrame = container.querySelector('iframe');

    act(() => {
      window.dispatchEvent(new MessageEvent('message', {
        source: frameWindow,
        data: { type: 'renderError', message: '脚本执行失败' },
      }));
    });

    expect(screen.getByRole('alert')).toHaveTextContent('iframe 运行异常：脚本执行失败');
    fireEvent.click(screen.getByRole('button', { name: '重试' }));
    expect(screen.queryByRole('alert')).toBeNull();
    expect(container.querySelector('iframe')).not.toBe(initialFrame);
  });
});
