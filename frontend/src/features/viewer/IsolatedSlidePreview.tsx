import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { RuntimeSlide, runtimeEventFromFrame } from './previewProtocol';
import { Button } from '../../components/ui/primitives';
import type { DOMSelection } from '../../api/types';
import { DraftSelectionControls } from './DraftSelectionControls';
import { PreviewLoading } from '../../components/ui/PreviewLoading';

interface IsolatedSlidePreviewProps {
  slides: RuntimeSlide[];
  index: number;
  title: string;
  className?: string;
  passive?: boolean;
  style?: React.CSSProperties;
  selectionMode?: 'element' | 'region' | 'none';
  selectionSlide?: { id: string; hash: string };
  draftSelections?: DOMSelection[];
  onSelection?: (selection: DOMSelection) => void;
  onSelectionMessage?: (message: string) => void;
  onSelectionCanceled?: () => void;
  onSelectionRemove?: (selectionId: string) => void;
  onSelectionPresence?: (statuses: import('./previewProtocol').SelectionPresence[]) => void;
  replayRequest?: { id: number; slideId: string };
}

export const IsolatedSlidePreview: React.FC<IsolatedSlidePreviewProps> = ({
  slides,
  index,
  title,
  className,
  passive = false,
  style,
  selectionMode = 'none',
  selectionSlide,
  draftSelections = [],
  onSelection,
  onSelectionMessage,
  onSelectionCanceled,
  onSelectionRemove,
  onSelectionPresence,
  replayRequest,
}) => {
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const indexRef = useRef(index);
  const [renderError, setRenderError] = useState('');
  const [themeError, setThemeError] = useState('');
  const [themeLoading, setThemeLoading] = useState(true);
  const [displayedDocument, setDisplayedDocument] = useState<{ id: string; html: string } | null>(null);
  const appearanceHash = slides[index]?.frame.appearance?.hash || '';
  const [runtimeVersion, setRuntimeVersion] = useState(0);
  const [runtimeReady, setRuntimeReady] = useState(false);
  const deliveredReplayRef = useRef(replayRequest?.id);
  const activeSlideId = slides[index]?.id;
  const activeHTML = slides[index]?.html;
  const hasDisplay = displayedDocument?.id === activeSlideId && displayedDocument?.html === activeHTML;
  const selectionHash = selectionSlide?.hash;
  // A response from an older displayed artifact must not reach the composer,
  // including presence probes that do not carry a hash themselves.
  const sessionID = useMemo(() => `selection_${typeof crypto !== 'undefined' && crypto.randomUUID ? crypto.randomUUID() : Math.random().toString(36).slice(2)}`, [activeSlideId, selectionHash]);
  indexRef.current = index;

  const sendDeck = useCallback(() => {
    iframeRef.current?.contentWindow?.postMessage({
      type: 'updateDeck',
      slides,
      index: indexRef.current,
    }, '*');
  }, [slides]);

  const sendSelectionState = useCallback(() => {
    if (!activeSlideId) return;
    iframeRef.current?.contentWindow?.postMessage({
      type: 'setSelectionMode', session_id: sessionID, slide_id: activeSlideId,
      mode: selectionSlide ? selectionMode : 'none', html_hash: selectionSlide?.hash ?? '',
    }, '*');
    if (!selectionSlide) return;
    iframeRef.current?.contentWindow?.postMessage({
      type: 'renderDraftSelections', session_id: sessionID, slide_id: selectionSlide.id,
      selections: draftSelections.filter((item) => item.slide_id === selectionSlide.id).map((item) => ({
        selection_id: item.selection_id, marker_no: item.marker_no, rect: item.rect, status: item.status,
      })),
    }, '*');
    const stale = draftSelections.filter((item) => item.slide_id === selectionSlide.id && (item.html_hash !== selectionSlide.hash));
    if (stale.length > 0) iframeRef.current?.contentWindow?.postMessage({ type: 'probeDraftSelections', session_id: sessionID, slide_id: selectionSlide.id, selections: stale }, '*');
  }, [activeSlideId, draftSelections, selectionMode, selectionSlide, sessionID]);

  useEffect(() => {
    const frame = iframeRef.current;
    if (!frame || typeof IntersectionObserver === 'undefined') return;
    const observer = new IntersectionObserver(entries => {
      frame.contentWindow?.postMessage({ type: 'setPreviewVisibility', active: entries.some(entry => entry.isIntersecting) }, '*');
    });
    observer.observe(frame);
    return () => observer.disconnect();
  }, [runtimeVersion, runtimeReady]);

  useEffect(() => {
    const onMessage = (event: MessageEvent) => {
      const message = runtimeEventFromFrame(event, iframeRef.current?.contentWindow ?? null);
      if (!message) return;
      if (message.type === 'runtimeReady') {
        setRenderError('');
        sendDeck();
        setRuntimeReady(true);
        sendSelectionState();
      } else if (message.type === 'themeApplying' || message.type === 'themeApplied' || message.type === 'themeApplyFailed') {
        if (message.slide_id !== activeSlideId || message.appearance_hash !== appearanceHash) return;
        setThemeLoading(message.type === 'themeApplying');
        setThemeError(message.type === 'themeApplyFailed' ? message.message : '');
        if (message.type === 'themeApplied' && activeSlideId && activeHTML !== undefined) {
          setDisplayedDocument({ id: activeSlideId, html: activeHTML });
        }
        if (message.type === 'themeApplied' && selectionSlide) {
          sendSelectionState();
          const selections = draftSelections.filter(item => item.slide_id === selectionSlide.id && item.html_hash === selectionSlide.hash);
          if (selections.length) iframeRef.current?.contentWindow?.postMessage({type:'probeDraftSelections',session_id:sessionID,slide_id:selectionSlide.id,selections,refresh:true},'*');
        }
      } else if (message.type === 'renderError') {
        setRenderError(message.message);
        setThemeLoading(false);
      } else if (message.type === 'selectionCreated') {
        if (selectionMode !== 'none' && selectionSlide && message.session_id === sessionID && message.slide_id === selectionSlide.id
          && message.selection.slide_id === selectionSlide.id && message.selection.html_hash === selectionSlide.hash) onSelection?.(message.selection);
      } else if (message.type === 'selectionCanceled' && message.session_id === sessionID) {
        onSelectionCanceled?.();
      } else if (message.type === 'selectionPresence' && selectionSlide && message.session_id === sessionID && message.slide_id === selectionSlide.id) {
        onSelectionPresence?.(message.statuses);
      } else if ((message.type === 'selectionEmpty' || message.type === 'selectionRejected') && message.session_id === sessionID) {
        onSelectionMessage?.(message.message);
      }
    };
    window.addEventListener('message', onMessage);
    return () => window.removeEventListener('message', onMessage);
  }, [activeHTML, activeSlideId, appearanceHash, draftSelections, onSelection, onSelectionCanceled, onSelectionMessage, onSelectionPresence, selectionMode, selectionSlide, sessionID, sendDeck, sendSelectionState]);

  useEffect(() => {
    setThemeError('');
    setRenderError('');
    setThemeLoading(Boolean(activeSlideId));
  }, [activeSlideId, activeHTML, appearanceHash]);

  useEffect(() => {
    sendDeck();
  }, [sendDeck]);

  useEffect(() => {
    if (runtimeReady) return;
    const timer = window.setTimeout(() => {
      setThemeLoading(false);
      setRenderError('预览初始化超时，请重试');
    }, 30000);
    return () => window.clearTimeout(timer);
  }, [runtimeReady, runtimeVersion]);

  useEffect(() => {
    iframeRef.current?.contentWindow?.postMessage({ type: 'gotoSlide', index }, '*');
  }, [index]);

  useEffect(() => { sendSelectionState(); }, [sendSelectionState]);

  useEffect(() => {
    if (!runtimeReady || !replayRequest || replayRequest.id === deliveredReplayRef.current) return;
    deliveredReplayRef.current = replayRequest.id;
    iframeRef.current?.contentWindow?.postMessage({
      type: 'replayCurrentSlide',
      slide_id: replayRequest.slideId,
    }, '*');
  }, [replayRequest, runtimeReady]);

  return (
    <div className="relative h-full w-full">
      <iframe
        key={runtimeVersion}
        ref={iframeRef}
        src="/slide-runtime/index.html"
        sandbox="allow-scripts"
        className={className}
        style={{ ...style, opacity: hasDisplay ? style?.opacity : 0, pointerEvents: hasDisplay ? style?.pointerEvents : 'none' }}
        title={title}
        aria-hidden={!hasDisplay || undefined}
        tabIndex={passive || !hasDisplay ? -1 : undefined}
        onLoad={sendDeck}
      />
      {selectionSlide && slides[index]?.id === selectionSlide.id && onSelectionRemove && !renderError && (
        <DraftSelectionControls slideId={selectionSlide.id} selections={draftSelections} onRemove={onSelectionRemove} />
      )}
      {themeLoading && !themeError && !renderError && <PreviewLoading label="正在加载主题外观" miniature={passive} />}
      {!passive && themeError && !renderError && (
        <div role="alert" className="absolute inset-x-3 bottom-3 flex items-center justify-between gap-3 rounded bg-surface/95 px-3 py-2 text-xs text-text-700 shadow-sm">
          <span>主题预览未应用：{themeError}</span>
          <Button variant="secondary" className="h-7 px-2 text-xs" onClick={() => {
            setThemeError(''); setThemeLoading(true);
            iframeRef.current?.contentWindow?.postMessage({type:'retryTheme',slide_id:activeSlideId},'*');
          }}>重试</Button>
        </div>
      )}
      {!passive && renderError && (
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
              setRuntimeReady(false);
              setDisplayedDocument(null);
              setThemeLoading(true);
              setRuntimeVersion((value) => value + 1);
            }}
          >重试</Button>
        </div>
      )}
    </div>
  );
};
