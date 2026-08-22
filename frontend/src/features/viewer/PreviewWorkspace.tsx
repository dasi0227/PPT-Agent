import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  ChevronLeft,
  ChevronRight,
  LayoutGrid,
  MonitorPlay,
  PanelLeftOpen,
  PanelRightOpen,
} from 'lucide-react';
import type { Slide, SlideSpec } from '../../api/types';
import { Button, Disclosure, IconButton, InlineNotice, Skeleton } from '../../components/ui/primitives';
import { cn } from '../../lib/utils';
import { useDeckStore, type PageView } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { useUIStore } from '../../stores/uiStore';
import { DesignSummary } from './DesignSummary';
import { EmptyState } from './EmptyState';
import { IsolatedSlidePreview } from './IsolatedSlidePreview';
import type { RuntimeSlide } from './previewProtocol';
import { SlideSpecCard } from './SlideSpecCard';
import { slideRoleLabel } from './semanticLabels';
import { hasRenderedHTML, ResourceState, useSlideRenderCache } from './useSlideRenderCache';

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
  runtimeSlides,
  runtimeIndex,
}: {
  slide: Slide;
  state: ResourceState<string>;
  retry: () => void;
  title: string;
  runtimeSlides?: RuntimeSlide[];
  runtimeIndex?: number;
}) {
  const visibleHtml = visibleHTML(state);
  const deck = runtimeSlides && runtimeIndex !== undefined && runtimeIndex >= 0
    ? runtimeSlides
    : visibleHtml !== undefined
      ? [{ id: slide.id, html: visibleHtml }]
      : [];
  const deckIndex = runtimeSlides && runtimeIndex !== undefined && runtimeIndex >= 0
    ? runtimeIndex
    : 0;

  return (
    <div className="relative h-full w-full overflow-hidden rounded bg-white shadow-canvas ring-1 ring-border">
      {deck.length > 0 && (
        <IsolatedSlidePreview
          slides={deck}
          index={deckIndex}
          className="h-full w-full border-0"
          title={title}
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
  spec,
  view,
}: {
  slide: Slide;
  index: number;
  selected: boolean;
  state: ResourceState<string>;
  load: () => void;
  select: () => void;
  spec?: SlideSpec;
  view: PageView;
}) {
  const ref = useRef<HTMLButtonElement>(null);
  const loadRef = useRef(load);
  loadRef.current = load;
  useEffect(() => {
    const node = ref.current;
    if (!node || view !== 'html' || !hasRenderedHTML(slide)) return;
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
  }, [slide, view]);

  const html = view === 'html' && state.status === 'ready' ? state.data : undefined;
  const title = spec?.title || slide.title || '未命名页面';
  return (
    <button
      ref={ref}
      type="button"
      data-testid={`overview-slide-${slide.id}`}
      onClick={select}
      aria-label={`打开第 ${index + 1} 页：${title}`}
      className={cn(
        'group relative aspect-video overflow-hidden text-left ring-1 ring-border hover:ring-accent',
        view === 'outline'
          ? 'rounded-lg bg-surface transition-[background-color,box-shadow] hover:bg-panel'
          : 'rounded bg-surface shadow-sm',
        selected && 'ring-2 ring-accent',
      )}
    >
      {html !== undefined ? (
        <IsolatedSlidePreview
          slides={[{ id: slide.id, html }]}
          index={0}
          className="h-[400%] w-[400%] origin-top-left scale-25 border-0 bg-white pointer-events-none"
          title={`第 ${index + 1} 页预览`}
        />
      ) : view === 'html' && state.status === 'error' ? (
        <div className="flex h-full flex-col items-center justify-center gap-2 p-4 text-center text-xs text-danger">
          <span>HTML 加载失败</span>
          <span className="text-text-600">打开页面后可重试</span>
        </div>
      ) : view === 'outline' && spec ? (
        <div className="h-full p-3.5">
          <div className="flex items-center gap-1 text-[9px] leading-none">
            <span className="inline-flex h-[18px] items-center rounded-[5px] bg-panel-muted px-1.5 font-semibold tracking-[0.12em] tabular-nums text-text-600">
              {String(index + 1).padStart(2, '0')}
            </span>
            <span className="inline-flex h-[18px] items-center rounded-[5px] bg-accent-soft px-1.5 font-semibold text-accent">
              {slideRoleLabel(spec.role)}
            </span>
          </div>
          <div className="mt-2 h-[44px]">
            <h2 className="line-clamp-2 text-[17px] font-semibold leading-[22px] text-text-900">
              {title}
            </h2>
          </div>
        </div>
      ) : (
        <div className="flex h-full flex-col items-center justify-center gap-1.5 p-5 text-center">
          <span className="text-[10px] font-medium uppercase tracking-wide text-text-400">
            {slide.layout || '页面'}
          </span>
          <span className="line-clamp-2 text-xs font-medium text-text-900">
            {slide.title || '未命名页面'}
          </span>
          <span className="text-[10px] text-text-400">
            {view === 'html' && hasRenderedHTML(slide) ? '缩略图加载中' : '页面未物化'}
          </span>
        </div>
      )}
      {view !== 'outline' && (
        <span className="absolute bottom-2 right-2 rounded bg-ink/75 px-1.5 py-0.5 text-xs text-white">{index + 1}</span>
      )}
      {view === 'html' && !hasRenderedHTML(slide) && (
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
      'input, textarea, select, button, a, [contenteditable="true"], [role="separator"], [role="menuitem"], [role="tab"]',
    );
}

export const PreviewWorkspace: React.FC = () => {
  const {
    currentSlideId,
    previewMode,
    enterOverview,
    exitOverview,
    effectiveView,
    globalView,
    setGlobalView,
    setCurrentSlideId,
  } = useDeckStore();
  const {
    activeProjectId,
    slidesByProjectId,
    specByProjectId,
    contentLoadingByProjectId,
    contentErrorByProjectId,
    loadProjectContent,
  } = useProjectStore();
  const { leftPanelHidden, rightPanelHidden, toggleLeftPanel, toggleRightPanel } = useUIStore();
  const specView = activeProjectId ? specByProjectId[activeProjectId] : undefined;
  const specLoading = activeProjectId ? contentLoadingByProjectId[activeProjectId] : false;
  const specError = activeProjectId ? contentErrorByProjectId[activeProjectId] : undefined;
  const projectId = activeProjectId;
  const slides = useMemo(
    () => projectId ? slidesByProjectId[projectId] || [] : [],
    [projectId, slidesByProjectId],
  );
  const { getState, load } = useSlideRenderCache(projectId);
  const [fullscreen, setFullscreen] = useState(false);
  const canvasRef = useRef<HTMLDivElement>(null);

  const hasSlides = slides.length > 0;
  const selectedIndex = currentSlideId ? slides.findIndex((slide) => slide.id === currentSlideId) : -1;
  const safePage = selectedIndex >= 0 ? selectedIndex : 0;
  const currentSlide = slides[safePage];
  const goPrev = useCallback(() => {
    if (safePage > 0) setCurrentSlideId(slides[safePage - 1].id);
  }, [safePage, setCurrentSlideId, slides]);
  const goNext = useCallback(() => {
    if (safePage < slides.length - 1) setCurrentSlideId(slides[safePage + 1].id);
  }, [safePage, setCurrentSlideId, slides]);
  const currentHasHTML = currentSlide ? hasRenderedHTML(currentSlide) : false;
  const currentView = currentSlide ? effectiveView(currentSlide.id, currentHasHTML) : 'html';
  const currentState = currentSlide ? getState(currentSlide) : { status: 'idle' as const };
  const runtimeSlides = useMemo<RuntimeSlide[]>(() => {
    return slides.flatMap((slide) => {
      if (!hasRenderedHTML(slide)) return [];
      const html = visibleHTML(getState(slide));
      return html === undefined ? [] : [{ id: slide.id, html }];
    });
  }, [getState, slides]);
  const runtimeIndex = currentSlide
    ? runtimeSlides.findIndex((slide) => slide.id === currentSlide.id)
    : -1;

  useEffect(() => {
    if (!currentSlide || !currentHasHTML || currentView !== 'html' || previewMode !== 'main') return;
    void load(currentSlide, 'current').then(() => {
      const previous = slides[safePage - 1];
      const next = slides[safePage + 1];
      if (previous && hasRenderedHTML(previous)) void load(previous, 'prefetch');
      if (next && hasRenderedHTML(next)) void load(next, 'prefetch');
    });
  }, [currentHasHTML, currentSlide, currentView, load, previewMode, safePage, slides]);

  useEffect(() => {
    const onFullscreenChange = () => setFullscreen(document.fullscreenElement === canvasRef.current);
    document.addEventListener('fullscreenchange', onFullscreenChange);
    return () => document.removeEventListener('fullscreenchange', onFullscreenChange);
  }, []);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (isEditableTarget(event.target)) return;
      if (event.key === 'ArrowLeft' && safePage > 0) {
        event.preventDefault();
        goPrev();
      } else if (event.key === 'ArrowRight' && safePage < slides.length - 1) {
        event.preventDefault();
        goNext();
      } else if (event.key.toLowerCase() === 'o') {
        event.preventDefault();
        if (previewMode === 'overview') exitOverview();
        else enterOverview();
      } else if (event.key === 'Escape' && document.fullscreenElement) {
        void document.exitFullscreen();
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [enterOverview, exitOverview, goNext, goPrev, previewMode, safePage, slides.length]);

  const present = useCallback(() => {
    if (!canvasRef.current) return;
    void canvasRef.current.requestFullscreen();
  }, []);

  return (
    <div className="relative flex h-full flex-col bg-canvas">
      <div className="flex h-12 shrink-0 items-center justify-between border-b border-border bg-panel px-3">
        <div className="flex items-center gap-1">
          {leftPanelHidden && (
            <IconButton label="展开左侧目录" onClick={toggleLeftPanel}>
              <PanelLeftOpen className="h-4 w-4" strokeWidth={1.75} />
            </IconButton>
          )}
          <IconButton
            label={previewMode === 'overview' ? '切换到单页视图' : '切换到概览视图'}
            onClick={previewMode === 'overview' ? exitOverview : enterOverview}
            className={previewMode === 'overview' ? 'bg-accent-soft text-accent' : undefined}
          >
            <LayoutGrid className="h-4 w-4" strokeWidth={1.75} />
          </IconButton>
          <IconButton label="全屏放映" onClick={present} disabled={!currentSlide || !currentHasHTML || currentView !== 'html'}>
            <MonitorPlay className="h-4 w-4" strokeWidth={1.75} />
          </IconButton>
        </div>

        <div className="flex items-center gap-3">
          <div className="flex items-center rounded-full bg-panel-muted p-0.5 text-xs">
            <button
              type="button"
              onClick={() => setGlobalView('outline')}
              aria-pressed={globalView === 'outline'}
              className={cn(
                'h-7 rounded-full px-3 font-medium transition-colors',
                globalView === 'outline' ? 'bg-surface text-text-900 shadow-sm' : 'text-text-400 hover:text-text-700',
              )}
            >设计稿</button>
            <button
              type="button"
              onClick={() => setGlobalView('html')}
              aria-pressed={globalView === 'html'}
              className={cn(
                'h-7 rounded-full px-3 font-medium transition-colors',
                globalView === 'html' ? 'bg-surface text-text-900 shadow-sm' : 'text-text-400 hover:text-text-700',
              )}
            >幻灯片</button>
          </div>

          <div className="flex items-center gap-1">
            <IconButton label="上一页" onClick={goPrev} disabled={!hasSlides || safePage === 0}>
              <ChevronLeft className="h-4 w-4" strokeWidth={1.75} />
            </IconButton>
            <span className="min-w-14 text-center text-sm tabular-nums text-text-600">
              {hasSlides ? `${safePage + 1} / ${slides.length}` : '0 / 0'}
            </span>
            <IconButton label="下一页" onClick={goNext} disabled={!hasSlides || safePage >= slides.length - 1}>
              <ChevronRight className="h-4 w-4" strokeWidth={1.75} />
            </IconButton>
            {rightPanelHidden && (
              <IconButton label="展开右侧对话" onClick={toggleRightPanel}>
                <PanelRightOpen className="h-4 w-4" strokeWidth={1.75} />
              </IconButton>
            )}
          </div>
        </div>
      </div>

      <div
        ref={canvasRef}
        data-fullscreen={fullscreen || undefined}
        className="relative flex flex-1 items-center justify-center overflow-hidden bg-canvas p-6 data-[fullscreen=true]:p-0"
      >
        {previewMode === 'main' ? (
          <div className="flex aspect-video h-auto max-h-full w-full max-w-5xl items-center justify-center">
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
                runtimeSlides={runtimeSlides}
                runtimeIndex={runtimeIndex}
              />
            ) : currentView === 'html' ? (
              <div className="flex h-full w-full items-center justify-center rounded bg-surface shadow-canvas ring-1 ring-border">
                <p className="text-sm text-text-400">暂时没有幻灯片内容</p>
              </div>
            ) : specView?.slide_specs?.[currentSlide.id] ? (
              <SlideSpecCard
                spec={specView.slide_specs[currentSlide.id]}
                state={specView.materialization?.[currentSlide.id]?.state ?? 'unknown'}
              />
            ) : specLoading ? (
              <div className="flex h-full w-full flex-col gap-3 rounded bg-surface p-8 shadow-canvas ring-1 ring-border">
                <Skeleton className="h-8 w-2/3" />
                <Skeleton className="h-5 w-1/2" />
                <Skeleton className="mt-4 h-40 w-full" />
                <span className="sr-only">设计稿正在加载</span>
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
              <div className="flex h-full w-full items-center justify-center rounded bg-surface shadow-canvas ring-1 ring-border">
                <EmptyState />
              </div>
            )}
          </div>
        ) : !hasSlides ? (
          <div className="flex flex-1 items-center justify-center">
            <p className="text-sm text-text-400">暂无页面</p>
          </div>
        ) : (
          <div className="absolute inset-0 overflow-y-auto p-6">
            <div className="mx-auto mb-6 max-w-6xl">
              {specView?.design && <DesignSummary design={specView.design} />}
            </div>
            <div className="mx-auto grid max-w-6xl grid-cols-2 gap-5 md:grid-cols-3 lg:grid-cols-4">
              {slides.map((slide, index) => (
                <OverviewSlide
                  key={slide.id}
                  slide={slide}
                  index={index}
                  selected={safePage === index}
                  state={getState(slide)}
                  view={globalView}
                  load={() => load(slide, 'prefetch')}
                  select={() => {
                    setCurrentSlideId(slide.id);
                    exitOverview();
                  }}
                  spec={specView?.slide_specs?.[slide.id]}
                />
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
