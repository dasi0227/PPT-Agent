import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { flushSync } from 'react-dom';
import {
  BarChart3,
  Code,
  Layers,
  List,
  Quote,
  Table,
  TrendingUp,
  Type,
  Workflow,
  type LucideIcon,
} from 'lucide-react';
import type { Slide } from '../../api/types';
import { Button, Disclosure, InlineNotice, Skeleton } from '../../components/ui/primitives';
import { cn } from '../../lib/utils';
import { useAppShortcuts } from '../../lib/useAppShortcuts';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { useUIStore } from '../../stores/uiStore';
import { ProjectDocumentView } from './ProjectDocumentView';
import { DesignSummary } from './DesignSummary';
import { IsolatedSlidePreview } from './IsolatedSlidePreview';
import type { RuntimeSlide } from './previewProtocol';
import { buildRuntimeFrame } from './runtimeFrame';
import { elementTypeLabel, slideRoleLabel } from './semanticLabels';
import { SlideSpecCard } from './SlideSpecCard';
import { hasRenderedHTML, ResourceState, useSlideRenderCache } from './useSlideRenderCache';
import { orderedSlides } from '../deck/selectors';
import { useComposerStore } from '../../stores/composerStore';
import { useThreadStore } from '../../stores/threadStore';
import { useActiveSession, useActiveThreadId } from '../agent/useActiveSession';
import { showGlobalWarning } from '../../stores/toastStore';
import type { DOMSelection } from '../../api/types';
import { ExportProgressDialog } from '../export/ExportProgressDialog';
import { useExportStore } from '../../stores/exportStore';
import { useGitCommitStore } from '../../stores/gitCommitStore';
import { useRunStore } from '../../stores/runStore';
import { useCanvasPan } from './useCanvasPan';
import { useAuthoringBlock } from './useAuthoringBlock';
import { PreviewStatusBar, PreviewToolbar, type PreviewSidebarControls } from './PreviewControls';

const ZOOM_MIN = 0.5;
const HTMLSourceView = React.lazy(() => import('./HTMLSourceView').then((module) => ({ default: module.HTMLSourceView })));
const ZOOM_MAX = 3;
const ZOOM_STEP = 1.15;

const ELEMENT_TYPE_ICONS: Record<string, LucideIcon> = {
  text: Type,
  list: List,
  metric: TrendingUp,
  quote: Quote,
  table: Table,
  chart: BarChart3,
  diagram: Workflow,
  code: Code,
  asset: Layers,
};

function visibleHTML(state: ResourceState<string>): string | undefined {
  if (state.status === 'ready') return state.data;
  if (state.status === 'loading' || state.status === 'error') return state.previous;
  return undefined;
}

function PreviewFrame({
  slide,
  state,
  retry,
  title,
  fullscreen = false,
  runtimeSlides,
  runtimeIndex,
  fallbackFrame,
  selectionMode,
  selectionSlide,
  draftSelections,
  onSelection,
  onSelectionMessage,
  onSelectionCanceled,
  onSelectionRemove,
  onSelectionPresence,
  replayRequest,
}: {
  slide: Slide;
  state: ResourceState<string>;
  retry: () => void;
  title: string;
  fullscreen?: boolean;
  runtimeSlides?: RuntimeSlide[];
  runtimeIndex?: number;
  fallbackFrame?: RuntimeSlide['frame'];
  selectionMode?: 'element' | 'region' | 'none';
  selectionSlide?: { id: string; hash: string };
  draftSelections?: DOMSelection[];
  onSelection?: (selection: DOMSelection) => void;
  onSelectionMessage?: (message: string) => void;
  onSelectionCanceled?: () => void;
  onSelectionRemove?: (selectionId: string) => void;
  onSelectionPresence?: (statuses: import('./previewProtocol').SelectionPresence[]) => void;
  replayRequest?: { id: number; slideId: string };
}) {
  const visibleHtml = visibleHTML(state);
  const deck = runtimeSlides && runtimeIndex !== undefined && runtimeIndex >= 0
    ? runtimeSlides
    : visibleHtml !== undefined
      ? fallbackFrame ? [{ id: slide.id, html: visibleHtml, frame: fallbackFrame }] : []
      : [];
  const deckIndex = runtimeSlides && runtimeIndex !== undefined && runtimeIndex >= 0
    ? runtimeIndex
    : 0;

  return (
    <div className={cn(
      'relative h-full w-full overflow-hidden bg-white',
      fullscreen ? 'rounded-none' : 'rounded shadow-canvas ring-1 ring-border',
    )}>
      {deck.length > 0 && (
        <IsolatedSlidePreview
          keyboardShortcuts
          slides={deck}
          index={deckIndex}
          className="h-full w-full border-0"
          title={title}
          selectionMode={selectionMode}
          selectionSlide={selectionSlide}
          draftSelections={draftSelections}
          onSelection={onSelection}
          onSelectionMessage={onSelectionMessage}
          onSelectionCanceled={onSelectionCanceled}
          onSelectionRemove={onSelectionRemove}
          onSelectionPresence={onSelectionPresence}
          replayRequest={replayRequest}
        />
      )}
      {(state.status === 'idle' || (state.status === 'loading' && !state.previous)) && (
        <div className="absolute inset-0 flex flex-col gap-3 p-8">
          <Skeleton className="h-10 w-2/3" />
          <Skeleton className="h-5 w-1/2" />
          <Skeleton className="mt-auto h-1/2 w-full" />
          <span className="sr-only">HTML 正在加载</span>
        </div>
      )}
      {state.status === 'loading' && state.previous && (
        <div className="absolute right-3 top-3 rounded-md border border-accent/20 bg-panel/95 px-2 py-1 text-xs text-accent shadow-overlay">
          页面正在更新
        </div>
      )}
      {state.status === 'error' && (
        <InlineNotice tone="danger" className="absolute inset-x-3 bottom-3 flex items-center justify-between gap-3 bg-danger-soft/95">
          <div className="min-w-0">
            <span>HTML 加载失败，请检查网络后重试。</span>
            <Disclosure label="错误详情">
              <p className="break-all font-mono text-[11px]">{state.error.message}</p>
            </Disclosure>
          </div>
          <Button variant="secondary" onClick={retry}>重试</Button>
        </InlineNotice>
      )}
    </div>
  );
}

function OverviewSlide({
  slide,
  index,
  selected,
  state,
  load,
  select,
  frame,
}: {
  slide: Slide;
  index: number;
  selected: boolean;
  state: ResourceState<string>;
  load: () => void;
  select: () => void;
  frame?: RuntimeSlide['frame'];
}) {
  const ref = useRef<HTMLButtonElement>(null);
  const loadRef = useRef(load);
  loadRef.current = load;
  useEffect(() => {
    const node = ref.current;
    if (!node || !hasRenderedHTML(slide)) return;
    if (typeof IntersectionObserver === 'undefined') {
      void loadRef.current();
      return;
    }
    const observer = new IntersectionObserver((entries) => {
      if (entries.some((entry) => entry.isIntersecting)) {
        void loadRef.current();
        observer.disconnect();
      }
    }, { rootMargin: '160px' });
    observer.observe(node);
    return () => observer.disconnect();
  }, [slide]);

  const html = state.status === 'ready' ? state.data : undefined;
  const title = slide.title || '未命名页面';
  const keyMessage = slide.spec?.key_message.trim() || '';
  const elements = slide.spec?.elements ?? [];
  return (
    <button
      ref={ref}
      type="button"
      data-testid={`overview-slide-${slide.id}`}
      onClick={select}
      aria-label={`打开第 ${index + 1} 页：${title}`}
      className={cn(
        'overview-slide-card group relative aspect-video overflow-hidden text-left ring-1 ring-border hover:ring-accent',
        'rounded bg-surface shadow-sm',
        selected && 'ring-2 ring-accent',
      )}
    >
      {html !== undefined && frame ? (
        <IsolatedSlidePreview
          slides={[{ id: slide.id, html, frame }]}
          index={0}
          className="h-[400%] w-[400%] origin-top-left scale-[0.25] border-0 bg-white pointer-events-none"
          title={`第 ${index + 1} 页预览`}
        />
      ) : state.status === 'error' ? (
        <div className="flex h-full flex-col items-center justify-center gap-2 p-4 text-center text-xs text-danger">
          <span>HTML 加载失败</span>
          <span className="text-text-600">打开页面后可重试</span>
        </div>
      ) : hasRenderedHTML(slide) ? (
        <div className="flex h-full flex-col items-center justify-center gap-1.5 p-5 text-center">
          <span className="line-clamp-2 text-xs font-medium text-text-900">{title}</span>
          <span className="text-[10px] text-text-400">缩略图加载中</span>
        </div>
      ) : (
        <div className="flex h-full flex-col p-3.5 pb-7">
          <span className="inline-flex w-fit items-center rounded-full bg-accent-soft px-2 py-0.5 text-[10px] font-medium text-accent">
            {slideRoleLabel(slide.role ?? 'content')}
          </span>
          <div className="flex flex-1 flex-col justify-center py-1.5">
            <span className="line-clamp-2 text-xs font-semibold text-text-900">{title}</span>
            {keyMessage ? (
              <span className="mt-1 line-clamp-2 text-[11px] leading-snug text-text-600">{keyMessage}</span>
            ) : null}
          </div>
          {elements.length > 0 ? (
            <div className="flex flex-wrap gap-1.5">
              {elements.map((element, elementIndex) => {
                const Icon = ELEMENT_TYPE_ICONS[element.type];
                return (
                  <span
                    key={elementIndex}
                    className="inline-flex items-center gap-1 rounded-md border border-border bg-surface px-1.5 py-0.5 text-[10px] text-text-600"
                  >
                    {Icon ? <Icon className="h-3 w-3 text-text-400" strokeWidth={1.75} /> : null}
                    {elementTypeLabel(element.type)}
                  </span>
                );
              })}
            </div>
          ) : null}
        </div>
      )}
      <span className="absolute bottom-2 right-2 rounded bg-ink/75 px-1.5 py-0.5 text-xs text-white">{index + 1}</span>
      {!hasRenderedHTML(slide) && (
        <span className="absolute bottom-2 left-2 rounded bg-warning-soft px-1.5 py-0.5 text-[10px] font-medium text-warning">
          未生成 HTML
        </span>
      )}
    </button>
  );
}

function isEditableTarget(target: EventTarget | null): boolean {
  return target instanceof Element
    && !!target.closest(
      'input, textarea, select, button, a, [contenteditable="true"], [role="separator"], [role="menuitem"], [role="menuitemradio"], [role="tab"]',
    );
}

interface PreviewWorkspaceProps {
  sidebarControls?: PreviewSidebarControls;
}

export const PreviewWorkspace: React.FC<PreviewWorkspaceProps> = ({ sidebarControls }) => {
  const {
    currentSlideId,
    activeDocument,
    previewMode,
    enterOverview,
    exitOverview,
    effectiveView,
    globalView,
    contentMode,
    setContentMode,
    setGlobalView,
    setCurrentSlideId,
  } = useDeckStore();
  const {
    activeProjectId,
    contentByProjectId,
    contentLoadingByProjectId,
    contentErrorByProjectId,
    loadProjectContent,
  } = useProjectStore();
  const projectId = activeProjectId;
  const ui = useUIStore();
  const leftPanelHidden = sidebarControls?.leftHidden ?? ui.leftPanelHidden;
  const rightPanelHidden = sidebarControls?.rightHidden ?? ui.rightPanelHidden;
  const { showRightPanel } = ui;
  const activeThreadId = useActiveThreadId();
  const runSession = useActiveSession();
  const allRunSessions = useRunStore((state) => state.sessions);
  const projectThreads = useThreadStore((state) => projectId ? state.threadsByProjectId[projectId] ?? [] : []);
  const commitSession = useGitCommitStore((state) => projectId ? state.sessions[projectId] : undefined);
  const exportSession = useExportStore((state) => state.session);
  const startExport = useExportStore((state) => state.start);
  const composer = useComposerStore();
  const ensureActiveThread = useThreadStore((state) => state.ensureActiveThread);
  const snapshot = activeProjectId ? contentByProjectId[activeProjectId] : undefined;
  const specLoading = activeProjectId ? contentLoadingByProjectId[activeProjectId] : false;
  const specError = activeProjectId ? contentErrorByProjectId[activeProjectId] : undefined;
  const slides = useMemo(
    () => orderedSlides(snapshot),
    [snapshot],
  );
  const { getState, load } = useSlideRenderCache(projectId);
  const [fullscreen, setFullscreen] = useState(false);
  const [replayRequest, setReplayRequest] = useState<{ id: number; slideId: string }>();
  const replayRequestIDRef = useRef(0);
  const [selectionMode, setSelectionMode] = useState<'element' | 'region' | 'none'>('none');
  const [zoom, setZoom] = useState(1);
  const canvasRef = useRef<HTMLDivElement>(null);

  const hasSlides = slides.length > 0;
  const runBlocking = projectThreads.some((thread) => ['creating', 'running', 'waiting', 'paused', 'recovering', 'canceling'].includes(allRunSessions[thread.id]?.status ?? 'idle'));
  const commitBlocking = commitSession?.status === 'creating' || commitSession?.status === 'running';
  const exportBlocking = exportSession?.projectId === projectId && ['accepted', 'running', 'ready', 'delivering'].includes(exportSession.operation.status);
  const managementBlocked = useAuthoringBlock(projectId);
  const exportDisabled = !projectId || !hasSlides || runBlocking || commitBlocking || exportBlocking;
  const exportDisabledReason = !projectId ? '请先打开项目' : !hasSlides ? '暂无幻灯片可导出' : runBlocking ? 'Agent 任务运行中，暂时不能导出' : commitBlocking ? '项目正在提交，暂时不能导出' : exportBlocking ? '当前项目已有导出任务' : undefined;
  const selectedIndex = currentSlideId ? slides.findIndex((slide) => slide.id === currentSlideId) : -1;
  const safePage = selectedIndex >= 0 ? selectedIndex : 0;
  const currentSlide = slides[safePage];
  const goPrev = useCallback(() => {
    if (!activeDocument && safePage > 0) setCurrentSlideId(slides[safePage - 1].id);
  }, [activeDocument, safePage, setCurrentSlideId, slides]);
  const goNext = useCallback(() => {
    if (!activeDocument && safePage < slides.length - 1) setCurrentSlideId(slides[safePage + 1].id);
  }, [activeDocument, safePage, setCurrentSlideId, slides]);
  const zoomIn = useCallback(() => setZoom((value) => Math.min(ZOOM_MAX, Number((value * ZOOM_STEP).toFixed(3)))), []);
  const zoomOut = useCallback(() => setZoom((value) => Math.max(ZOOM_MIN, Number((value / ZOOM_STEP).toFixed(3)))), []);
  const currentHasHTML = currentSlide ? hasRenderedHTML(currentSlide) : false;
  const presentationSlide = currentHasHTML ? currentSlide : slides.find(hasRenderedHTML);
  const currentView = currentSlide ? effectiveView(currentSlide.id, currentHasHTML) : 'html';
  const currentState = currentSlide ? getState(currentSlide) : { status: 'idle' as const };
  const runtimeSlides = useMemo<RuntimeSlide[]>(() => {
    return slides.flatMap((slide) => {
      if (!hasRenderedHTML(slide)) return [];
      const html = visibleHTML(getState(slide));
      const frame = snapshot ? buildRuntimeFrame(snapshot, slide.id) : undefined;
      return html === undefined || !frame ? [] : [{ id: slide.id, html, frame }];
    });
  }, [getState, slides, snapshot]);
  const runtimeIndex = currentSlide
    ? runtimeSlides.findIndex((slide) => slide.id === currentSlide.id)
    : -1;
  const draftSelections = useMemo(() => activeThreadId
    ? (composer.threadReferences[activeThreadId] ?? []).flatMap((item) => item.kind === 'dom' ? [item.selection] : [])
    : [], [activeThreadId, composer.threadReferences]);
  const selectionSlide = currentState.status === 'ready' && currentSlide?.html_hash ? {
    id: currentSlide.id, hash: currentSlide.html_hash,
  } : undefined;
  const selectionEnabled = !activeDocument && contentMode === 'preview' && previewMode === 'main' && currentView === 'html' && Boolean(selectionSlide) && !fullscreen
    && !['creating', 'waiting', 'paused', 'recovering', 'canceling'].includes(runSession.status);
  const zoomEnabled = !activeDocument && contentMode === 'preview' && hasSlides && previewMode === 'main' && currentView === 'html' && currentHasHTML;
  const canvasPan = useCanvasPan({
    viewportRef: canvasRef,
    enabled: zoomEnabled && !fullscreen,
    selectionActive: selectionMode !== 'none',
    zoom,
    pageKey: `${projectId}:${currentSlide?.id}`,
  });

  const acceptSelection = useCallback(async (raw: DOMSelection) => {
    if (!projectId) return;
    let threadId = activeThreadId;
    if (!threadId) {
      try { threadId = await ensureActiveThread(projectId); } catch {
        showGlobalWarning('无法创建会话，请重试。');
        const retry = selectionMode; setSelectionMode('none'); window.setTimeout(() => setSelectionMode(retry), 0);
        return;
      }
    }
    const state = useComposerStore.getState();
    const references = state.threadReferences[threadId] ?? [];
    const selections = references.flatMap((item) => item.kind === 'dom' ? [item.selection] : []);
    const duplicate = selections.find((item) => item.dedupe_key && item.dedupe_key === raw.dedupe_key);
    if (duplicate) {
      state.setEditingDOMSelection(threadId, duplicate.selection_id);
      showRightPanel();
      setSelectionMode('none');
      return;
    }
    if (selections.length >= 8) {
      showGlobalWarning('每条消息最多添加 8 个 DOM 标记。');
      const retry = selectionMode; setSelectionMode('none'); window.setTimeout(() => setSelectionMode(retry), 0);
      return;
    }
    const selectionID = `sel_${typeof crypto !== 'undefined' && crypto.randomUUID ? crypto.randomUUID() : Math.random().toString(36).slice(2)}`;
    const next: DOMSelection = { ...raw, selection_id: selectionID, marker_no: state.nextMarkerByThread[threadId] ?? 1 };
    const encoder = new TextEncoder();
    const totalBytes = [...selections, next].reduce((sum, item) => sum + encoder.encode(JSON.stringify({ dom_targets: item.dom_targets ?? [], decoration_targets: item.decoration_targets ?? [] })).byteLength, 0);
    if (totalBytes > 256 * 1024) {
      showGlobalWarning('选择内容过大，请缩小范围。');
      const retry = selectionMode; setSelectionMode('none'); window.setTimeout(() => setSelectionMode(retry), 0);
      return;
    }
    state.addThreadDOMSelection(threadId, next);
    state.setEditingDOMSelection(threadId, next.selection_id);
    showRightPanel();
    setSelectionMode('none');
  }, [activeThreadId, ensureActiveThread, projectId, selectionMode, showRightPanel]);

  useEffect(() => {
    if (activeDocument || !currentSlide || !currentHasHTML || currentView !== 'html' || previewMode !== 'main') return;
    void load(currentSlide, 'current').then(() => {
      const previous = slides[safePage - 1];
      const next = slides[safePage + 1];
      if (previous && hasRenderedHTML(previous)) void load(previous, 'prefetch');
      if (next && hasRenderedHTML(next)) void load(next, 'prefetch');
    });
  }, [activeDocument, currentHasHTML, currentSlide, currentView, load, previewMode, safePage, slides]);

  useEffect(() => {
    const onFullscreenChange = () => setFullscreen(Boolean(canvasRef.current && document.fullscreenElement === canvasRef.current));
    document.addEventListener('fullscreenchange', onFullscreenChange);
    return () => document.removeEventListener('fullscreenchange', onFullscreenChange);
  }, []);

  useEffect(() => {
    if (!selectionEnabled) setSelectionMode('none');
  }, [selectionEnabled]);

  useEffect(() => {
	setSelectionMode('none');
  }, [activeDocument, activeThreadId, currentSlideId, currentView, previewMode]);

  useEffect(() => {
    setZoom(1);
  }, [previewMode, currentView]);

  useEffect(() => {
    if (!activeThreadId) return;
    const known = new Set(slides.map((slide) => slide.id));
    const state = useComposerStore.getState();
    (state.threadReferences[activeThreadId] ?? []).forEach((item) => {
      if (item.kind !== 'dom') return;
      const status = known.has(item.selection.slide_id)
        ? (item.selection.status === 'page_deleted' ? 'active' : item.selection.status)
        : 'page_deleted';
      if (status !== item.selection.status) state.updateThreadDOMSelection(activeThreadId, item.selection.selection_id, { status });
    });
  }, [activeThreadId, slides]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && selectionMode !== 'none') {
        event.preventDefault();
        setSelectionMode('none');
        return;
      }
      if (isEditableTarget(event.target)) return;
      if (event.key === 'Escape' && document.fullscreenElement) {
        void document.exitFullscreen();
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [selectionMode]);

  const toggleOverview = useCallback(() => {
    if (!hasSlides) return;
    setContentMode('preview');
    if (previewMode === 'overview') exitOverview();
    else enterOverview();
  }, [hasSlides, setContentMode, previewMode, exitOverview, enterOverview]);

  useAppShortcuts(!projectId || (!hasSlides && !activeDocument) ? {} : activeDocument ? {
    'deck.overview': toggleOverview,
  } : {
    'deck.overview': toggleOverview,
    'deck.previous': goPrev,
    'deck.next': goNext,
    'deck.zoom_in': () => { if (zoomEnabled) zoomIn(); },
    'deck.zoom_out': () => { if (zoomEnabled) zoomOut(); },
    'deck.view': () => setGlobalView(globalView === 'html' ? 'outline' : 'html'),
    'deck.form': () => { if (globalView === 'html' && previewMode === 'main') setContentMode(contentMode === 'preview' ? 'source' : 'preview'); },
  });

  const present = useCallback(async () => {
    const expectedSlideID = presentationSlide?.id;
    if (!expectedSlideID) return;
    // Mount the slide canvas while the click still carries fullscreen permission.
    flushSync(() => {
      setCurrentSlideId(expectedSlideID);
      setGlobalView('html');
      setContentMode('preview');
      exitOverview();
    });
    const canvas = canvasRef.current;
    if (!canvas) return;
    try {
      await canvas.requestFullscreen();
    } catch {
      return;
    }
    replayRequestIDRef.current += 1;
    setReplayRequest({ id: replayRequestIDRef.current, slideId: expectedSlideID });
  }, [presentationSlide?.id, setCurrentSlideId, setContentMode, setGlobalView, exitOverview]);

  return (
    <div className="preview-workspace relative flex h-full min-h-0 min-w-0 flex-col bg-canvas">
      <PreviewToolbar
        projectId={projectId}
        pageControlsDisabled={Boolean(activeDocument) || !hasSlides}
        hasPages={hasSlides}
        contentMode={contentMode}
        overview={!activeDocument && previewMode === 'overview'}
        onToggleOverview={toggleOverview}
        canPresent={Boolean(presentationSlide)}
        onPresent={present}
        exportDisabled={exportDisabled}
        exportDisabledReason={exportDisabledReason}
        onExport={(format) => { if (projectId) void startExport(projectId, format); }}
        selectionMode={selectionMode}
        selectionEnabled={selectionEnabled}
        onSelectionModeChange={setSelectionMode}
        sidebarControls={sidebarControls ?? {
          leftHidden: leftPanelHidden,
          rightHidden: rightPanelHidden,
          canExpandLeft: true,
          canExpandRight: true,
          onExpandLeft: ui.toggleLeftPanel,
          onExpandRight: ui.toggleRightPanel,
        }}
      />

      {contentMode === 'source' && !activeDocument && globalView === 'html' && previewMode === 'main' && projectId ? (
        <React.Suspense fallback={<div className="flex flex-1 items-center justify-center text-sm text-text-400">正在加载源码…</div>}>
          <HTMLSourceView projectId={projectId} slideId={currentSlide?.id} title={currentSlide?.title} ordinal={safePage + 1} hash={currentSlide?.html_hash} sceneRevision={snapshot?.scene_revision} available={currentHasHTML} />
        </React.Suspense>
      ) : activeDocument ? (
        <ProjectDocumentView
          key={`${projectId}:${activeDocument}`}
          document={activeDocument}
          snapshot={snapshot}
          error={specError}
          blocked={managementBlocked}
          onRetry={() => { if (projectId) void loadProjectContent(projectId); }}
        />
      ) : <div
        ref={canvasRef}
        data-fullscreen={fullscreen || undefined}
        className={cn(
          'relative flex min-h-0 flex-1 items-center justify-center overflow-hidden bg-canvas p-6 data-[fullscreen=true]:p-0',
          canvasPan.canPan && 'touch-none select-none [&_iframe]:pointer-events-none',
          canvasPan.canPan && (canvasPan.dragging ? 'cursor-grabbing' : 'cursor-grab'),
        )}
        {...canvasPan.pointerHandlers}
      >
        {previewMode === 'main' ? (
          <div
            ref={canvasPan.stageRef}
            data-testid="slide-preview-stage"
            className={cn(
              'flex w-full items-center justify-center',
              !canvasPan.dragging && 'transition-transform duration-150 ease-out motion-reduce:transition-none',
              fullscreen ? 'h-full max-w-none' : 'aspect-video h-auto max-h-full max-w-5xl',
            )}
            style={!fullscreen && currentView === 'html' && currentHasHTML ? {
              transform: `translate(${canvasPan.position.x}px, ${canvasPan.position.y}px) scale(${zoom})`,
            } : undefined}
          >
            {!projectId ? (
              <InlineNotice tone="info">请先从顶部项目 Tab 打开或新建项目。</InlineNotice>
            ) : !hasSlides ? (
              <p className="text-sm text-text-400">暂无页面</p>
            ) : currentView === 'html' && currentHasHTML ? (
              <PreviewFrame
                slide={currentSlide}
                state={currentState}
                retry={() => void load(currentSlide, 'current')}
                title={`第 ${safePage + 1} 页 HTML 预览`}
                fullscreen={fullscreen}
                runtimeSlides={runtimeSlides}
                runtimeIndex={runtimeIndex}
                fallbackFrame={snapshot ? buildRuntimeFrame(snapshot, currentSlide.id) : undefined}
                selectionMode={selectionEnabled ? selectionMode : 'none'}
                selectionSlide={selectionSlide}
                draftSelections={draftSelections}
                onSelection={(selection) => { void acceptSelection(selection); }}
                onSelectionMessage={showGlobalWarning}
                onSelectionCanceled={() => setSelectionMode('none')}
                onSelectionRemove={fullscreen ? undefined : (selectionId) => {
                  if (!activeThreadId) return;
                  const state = useComposerStore.getState();
                  const belongsToPage = (state.threadReferences[activeThreadId] ?? []).some((reference) => (
                    reference.kind === 'dom' && reference.selection.selection_id === selectionId
                    && reference.selection.slide_id === currentSlide.id
                  ));
                  if (belongsToPage) state.removeThreadDOMSelection(activeThreadId, selectionId);
                }}
                replayRequest={replayRequest}
                onSelectionPresence={(statuses) => {
                  if (!activeThreadId) return;
                  const state = useComposerStore.getState();
                  const references = state.threadReferences[activeThreadId] ?? [];
                  statuses.forEach((item) => {
                    const current = references.find((reference) => reference.kind === 'dom' && reference.selection.selection_id === item.selection_id);
                    if (current?.kind !== 'dom') return;
                    if (item.snapshot) {
                      state.updateThreadDOMSelection(activeThreadId, item.selection_id, { rect:item.snapshot.rect, dom_targets:item.snapshot.dom_targets, decoration_targets:item.snapshot.decoration_targets, status:item.status });
                      return;
                    }
                    const targetStatuses = new Map((item.targets ?? []).map((target) => [target.target_id, target.status]));
                    const domTargets = current.selection.dom_targets?.map((target) => ({ ...target, status: targetStatuses.get(target.target_id) ?? target.status }));
                    const targetsChanged = domTargets?.some((target, index) => target.status !== current.selection.dom_targets?.[index]?.status) ?? false;
                    if (current.selection.status !== item.status || targetsChanged) state.updateThreadDOMSelection(activeThreadId, item.selection_id, { status: item.status, dom_targets: domTargets });
                  });
                }}
              />
            ) : currentView === 'html' ? (
              <div className="flex h-full w-full items-center justify-center rounded bg-surface shadow-canvas ring-1 ring-border">
                <p className="text-sm text-text-400">幻灯片尚未生成</p>
              </div>
            ) : specLoading && !snapshot ? (
              <div className="flex h-full w-full flex-col gap-3 rounded bg-surface p-8 shadow-canvas ring-1 ring-border">
                <Skeleton className="h-8 w-2/3" />
                <Skeleton className="h-5 w-1/2" />
                <Skeleton className="mt-4 h-40 w-full" />
                <span className="text-sm text-text-400">等待生成设计稿</span>
              </div>
            ) : specError ? (
              <InlineNotice tone="danger" className="max-w-md">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <span>设计稿加载失败，请重试。</span>
                    <Disclosure label="错误详情">
                      <p className="break-all font-mono text-[11px]">{specError}</p>
                    </Disclosure>
                  </div>
                  <Button variant="secondary" onClick={() => projectId && void loadProjectContent(projectId)}>重试</Button>
                </div>
              </InlineNotice>
            ) : (
              <SlideSpecCard key={`${projectId}:${currentSlide.id}`} title={currentSlide.title} spec={currentSlide.spec} role={currentSlide.role ?? 'content'}
                projectId={projectId} slideId={currentSlide.id} hash={snapshot?.hashes[`spec:${currentSlide.id}`]} sceneRevision={snapshot?.scene_revision} blocked={managementBlocked} />
            )}
          </div>
        ) : !hasSlides ? (
          <div className="flex flex-1 items-center justify-center">
            <p className="text-sm text-text-400">暂无页面</p>
          </div>
        ) : (
          <div className="scrollbar-none absolute inset-0 overflow-y-auto p-6">
            <div className="mx-auto mb-6 max-w-6xl">
              {snapshot?.design && <DesignSummary design={snapshot.design} />}
            </div>
            <div className="mx-auto grid max-w-6xl grid-cols-2 gap-5 md:grid-cols-3 lg:grid-cols-4">
              {slides.map((slide, index) => (
                <OverviewSlide
                  key={slide.id}
                  slide={slide}
                  index={index}
                  selected={safePage === index}
                  state={getState(slide)}
                  load={() => load(slide, 'prefetch')}
                  select={() => {
                    setCurrentSlideId(slide.id);
                    exitOverview();
                  }}
                  frame={snapshot ? buildRuntimeFrame(snapshot, slide.id) : undefined}
                />
              ))}
            </div>
          </div>
        )}
      </div>}
      <PreviewStatusBar
        documentOpen={Boolean(activeDocument)}
        sourceToggleVisible={!activeDocument && globalView === 'html' && previewMode === 'main'}
        view={globalView}
        onViewChange={setGlobalView}
        contentMode={contentMode}
        onContentModeChange={setContentMode}
        pageControlsDisabled={Boolean(activeDocument) || !hasSlides}
        pageIndex={safePage}
        pageCount={slides.length}
        onPrevious={goPrev}
        onNext={goNext}
        zoom={zoom}
        zoomMin={ZOOM_MIN}
        zoomMax={ZOOM_MAX}
        zoomEnabled={zoomEnabled}
        onZoomOut={zoomOut}
        onZoomIn={zoomIn}
      />
      <ExportProgressDialog />
    </div>
  );
};
