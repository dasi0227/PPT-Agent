import React, { useCallback, useEffect, useRef, useState } from 'react';
import { RuntimeSlide, runtimeEventFromFrame } from './previewProtocol';

interface IsolatedSlidePreviewProps {
  slides: RuntimeSlide[];
  index: number;
  title: string;
  className?: string;
  style?: React.CSSProperties;
}

export const IsolatedSlidePreview: React.FC<IsolatedSlidePreviewProps> = ({
  slides,
  index,
  title,
  className,
  style,
}) => {
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const indexRef = useRef(index);
  const [renderError, setRenderError] = useState('');
  indexRef.current = index;

  const sendDeck = useCallback(() => {
    iframeRef.current?.contentWindow?.postMessage({
      type: 'updateDeck',
      slides,
      index: indexRef.current,
    }, '*');
  }, [slides]);

  useEffect(() => {
    const onMessage = (event: MessageEvent) => {
      const message = runtimeEventFromFrame(event, iframeRef.current?.contentWindow ?? null);
      if (!message) return;
      if (message.type === 'runtimeReady') {
        setRenderError('');
        sendDeck();
      } else {
        setRenderError(message.message);
      }
    };
    window.addEventListener('message', onMessage);
    return () => window.removeEventListener('message', onMessage);
  }, [sendDeck]);

  useEffect(() => {
    iframeRef.current?.contentWindow?.postMessage({ type: 'gotoSlide', index }, '*');
  }, [index]);

  useEffect(() => {
    sendDeck();
  }, [sendDeck]);

  return (
    <div className="relative h-full w-full">
      <iframe
        ref={iframeRef}
        src="/slide-runtime/index.html"
        sandbox="allow-scripts"
        className={className}
        style={style}
        title={title}
        onLoad={sendDeck}
      />
      {renderError && (
        <div
          role="alert"
          className="absolute inset-x-3 bottom-3 rounded bg-mode-error/90 px-3 py-2 text-xs text-white"
        >
          预览加载失败：{renderError}
        </div>
      )}
    </div>
  );
};
