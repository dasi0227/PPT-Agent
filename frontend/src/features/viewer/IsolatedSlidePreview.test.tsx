import { act, fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { IsolatedSlidePreview } from './IsolatedSlidePreview';
import type { RuntimeSlide } from './previewProtocol';

describe('IsolatedSlidePreview runtime errors', () => {
  it('disables selection while showing fallback HTML and rejects replies from an older artifact', () => {
    const postMessage = vi.fn();
    const frameWindow = { postMessage } as unknown as Window;
    Object.defineProperty(window.HTMLIFrameElement.prototype, 'contentWindow', { configurable: true, get: () => frameWindow });
    const slide: RuntimeSlide = { id: 's1', html: '<h1>old</h1>', frame: { slide_id: 's1', canvas: { width: 1920, height: 1080, aspect_ratio: '16:9' }, theme_id: 'swiss-modern', deck_title: 'Deck', ordinal: 1, total: 1, role: 'content', section: { id: 'sec_1', title: '正文', index: 1 }, numbering: { visible: true, format: 'number' }, chrome: [] } };
    const onSelection = vi.fn();
    const onPresence = vi.fn();
    const props = { slides: [slide], index: 0, title: '预览', selectionMode: 'element' as const, onSelection, onSelectionPresence: onPresence };
    const { rerender } = render(<IsolatedSlidePreview {...props} selectionSlide={{ id: 's1', hash: 'sha256:old' }} />);
    const latestMode = () => {
      const messages = postMessage.mock.calls.filter(([value]) => value.type === 'setSelectionMode');
      return messages[messages.length - 1][0];
    };
    const oldSession = latestMode().session_id;
    const rect = { x: 0, y: 0, width: 10, height: 10 };
    const selection = { kind: 'element', slide_id: 's1', html_hash: 'sha256:old', canvas: { width: 1920, height: 1080 }, status: 'active', rect, dom_targets: [], chrome_targets: [{ type: 'page_number', placement: 'bottom-right', style: '', text: '1', rect }] };
    const sendSelection = (session: string, hash: string) => {
      act(() => {
        window.dispatchEvent(new MessageEvent('message', { source: frameWindow,
          data: { type: 'selectionCreated', session_id: session, slide_id: 's1', selection: { ...selection, html_hash: hash } },
        }));
      });
    };

    // The old HTML remains visible while its replacement is loading.
    rerender(<IsolatedSlidePreview {...props} />);
    expect(latestMode()).toMatchObject({ mode: 'none', html_hash: '' });
    sendSelection(oldSession, 'sha256:old');
    expect(onSelection).not.toHaveBeenCalled();

    rerender(<IsolatedSlidePreview {...props} slides={[{ ...slide, html: '<h1>new</h1>' }]} selectionSlide={{ id: 's1', hash: 'sha256:new' }} />);
    const newSession = latestMode().session_id;
    expect(newSession).not.toBe(oldSession);
    sendSelection(oldSession, 'sha256:old');
    sendSelection(newSession, 'sha256:old');
    act(() => {
      window.dispatchEvent(new MessageEvent('message', { source: frameWindow,
        data: { type: 'selectionPresence', session_id: oldSession, slide_id: 's1', statuses: [{ selection_id: 'sel_1', status: 'content_deleted' }] },
      }));
    });
    expect(onSelection).not.toHaveBeenCalled();
    expect(onPresence).not.toHaveBeenCalled();
    sendSelection(newSession, 'sha256:new');
    expect(onSelection).toHaveBeenCalledTimes(1);
  });

  it('shows a retry action for errors emitted by its isolated frame', () => {
    const frameWindow = { postMessage: vi.fn() } as unknown as Window;
    Object.defineProperty(window.HTMLIFrameElement.prototype, 'contentWindow', {
      configurable: true,
      get: () => frameWindow,
    });
    const { container } = render(
      <IsolatedSlidePreview
        slides={[{ id: 's1', html: '<h1>Slide</h1>', frame: { slide_id: 's1', canvas: { width: 1920, height: 1080, aspect_ratio: '16:9' }, theme_id: 'swiss-modern', deck_title: 'Deck', ordinal: 1, total: 1, role: 'content', section: { id: 'sec_1', title: '正文', index: 1 }, numbering: { visible: true, format: 'number' }, chrome: [] } }]}
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

  it('forwards each new replay request once after the runtime is ready', () => {
    const frameWindow = { postMessage: vi.fn() } as unknown as Window;
    Object.defineProperty(window.HTMLIFrameElement.prototype, 'contentWindow', {
      configurable: true,
      get: () => frameWindow,
    });
    const slide: RuntimeSlide = { id: 's1', html: '<h1>Slide</h1>', frame: { slide_id: 's1', canvas: { width: 1920, height: 1080, aspect_ratio: '16:9' }, theme_id: 'swiss-modern', deck_title: 'Deck', ordinal: 1, total: 1, role: 'content', section: { id: 'sec_1', title: '正文', index: 1 }, numbering: { visible: true, format: 'number' }, chrome: [] } };
    const { rerender } = render(
      <IsolatedSlidePreview slides={[slide]} index={0} title="安全预览" replayRequest={{ id: 1, slideId: 's1' }} />,
    );

    act(() => {
      window.dispatchEvent(new MessageEvent('message', {
        source: frameWindow,
        data: { type: 'runtimeReady' },
      }));
    });
    expect(vi.mocked(frameWindow.postMessage)).not.toHaveBeenCalledWith(
      { type: 'replayCurrentSlide', slide_id: 's1' }, '*',
    );

    rerender(<IsolatedSlidePreview slides={[slide]} index={0} title="安全预览" replayRequest={{ id: 2, slideId: 's1' }} />);
    expect(vi.mocked(frameWindow.postMessage)).toHaveBeenCalledWith(
      { type: 'replayCurrentSlide', slide_id: 's1' }, '*',
    );
    const replayCalls = vi.mocked(frameWindow.postMessage).mock.calls.filter(([message]) => (
      (message as { type?: string }).type === 'replayCurrentSlide'
    ));
    expect(replayCalls).toHaveLength(1);
  });
});
