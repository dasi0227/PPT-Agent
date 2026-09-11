import React, { useCallback, useEffect, useRef, useState } from 'react';
import { RuntimeSlide, runtimeEventFromFrame } from './previewProtocol';
import { Button } from '../../components/ui/primitives';
import type { DOMSelection } from '../../api/types';

interface IsolatedSlidePreviewProps {
  slides: RuntimeSlide[];
  index: number;
  title: string;
  className?: string;
  style?: React.CSSProperties;
  selectionMode?: 'element' | 'region' | 'none';
  selectionSlide?: { id: string; revision: number; hash: string };
  draftSelections?: DOMSelection[];
  onSelection?: (selection: DOMSelection) => void;
  onSelectionMessage?: (message: string) => void;
  onSelectionCanceled?: () => void;
  onSelectionPresence?: (statuses: Array<{ selection_id: string; status: 'active' | 'content_deleted'; targets?: Array<{ target_id: string; status: 'active' | 'content_deleted' }> }>) => void;
}

export const IsolatedSlidePreview: React.FC<IsolatedSlidePreviewProps> = ({
  slides,
  index,
  title,
  className,
  style,
  selectionMode = 'none',
  selectionSlide,
  draftSelections = [],
  onSelection,
  onSelectionMessage,
  onSelectionCanceled,
  onSelectionPresence,
}) => {
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const indexRef = useRef(index);
  const [renderError, setRenderError] = useState('');
  const [runtimeVersion, setRuntimeVersion] = useState(0);
  const sessionRef = useRef(`selection_${typeof crypto !== 'undefined' && crypto.randomUUID ? crypto.randomUUID() : Math.random().toString(36).slice(2)}`);
  indexRef.current = index;

  const sendDeck = useCallback(() => {
    iframeRef.current?.contentWindow?.postMessage({
      type: 'updateDeck',
      slides,
      index: indexRef.current,
    }, '*');
  }, [slides]);

  const sendSelectionState = useCallback(() => {
    if (!selectionSlide) return;
    iframeRef.current?.contentWindow?.postMessage({
      type: 'setSelectionMode', session_id: sessionRef.current, slide_id: selectionSlide.id,
      mode: selectionMode, html_revision: selectionSlide.revision, html_hash: selectionSlide.hash,
    }, '*');
    iframeRef.current?.contentWindow?.postMessage({
      type: 'renderDraftSelections', session_id: sessionRef.current, slide_id: selectionSlide.id,
      selections: draftSelections.filter((item) => item.slide_id === selectionSlide.id).map((item) => ({
        selection_id: item.selection_id, marker_no: item.marker_no, rect: item.rect, status: item.status,
      })),
    }, '*');
    const stale = draftSelections.filter((item) => item.slide_id === selectionSlide.id && (item.html_revision !== selectionSlide.revision || item.html_hash !== selectionSlide.hash));
    if (stale.length > 0) iframeRef.current?.contentWindow?.postMessage({ type: 'probeDraftSelections', session_id: sessionRef.current, slide_id: selectionSlide.id, selections: stale }, '*');
  }, [draftSelections, selectionMode, selectionSlide]);

  useEffect(() => {
    const onMessage = (event: MessageEvent) => {
      const message = runtimeEventFromFrame(event, iframeRef.current?.contentWindow ?? null);
      if (!message) return;
      if (message.type === 'runtimeReady') {
        setRenderError('');
        sendDeck();
        window.setTimeout(sendSelectionState, 0);
      } else if (message.type === 'renderError') {
        setRenderError(message.message);
      } else if (message.type === 'selectionCreated') {
        if (message.session_id === sessionRef.current && message.slide_id === selectionSlide?.id) onSelection?.(message.selection);
      } else if (message.type === 'selectionCanceled' && message.session_id === sessionRef.current) {
        onSelectionCanceled?.();
      } else if (message.type === 'selectionPresence' && message.session_id === sessionRef.current) {
        onSelectionPresence?.(message.statuses);
      } else if ((message.type === 'selectionEmpty' || message.type === 'selectionRejected') && message.session_id === sessionRef.current) {
        onSelectionMessage?.(message.message);
      }
    };
    window.addEventListener('message', onMessage);
    return () => window.removeEventListener('message', onMessage);
  }, [onSelection, onSelectionCanceled, onSelectionMessage, onSelectionPresence, selectionSlide?.id, sendDeck, sendSelectionState]);

  useEffect(() => {
    iframeRef.current?.contentWindow?.postMessage({ type: 'gotoSlide', index }, '*');
  }, [index]);

  useEffect(() => {
    sendDeck();
  }, [sendDeck]);

  useEffect(() => { sendSelectionState(); }, [sendSelectionState]);

  return (
    <div className="relative h-full w-full">
      <iframe
        key={runtimeVersion}
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
          className="absolute inset-x-3 bottom-3 flex items-center justify-between gap-3 rounded bg-danger/95 px-3 py-2 text-xs text-white"
        >
          <span>iframe 运行异常：{renderError}</span>
          <Button
            variant="secondary"
            className="h-7 bg-white px-2 text-xs text-danger"
            onClick={() => {
              setRenderError('');
              setRuntimeVersion((value) => value + 1);
            }}
          >重试</Button>
        </div>
      )}
    </div>
  );
};
