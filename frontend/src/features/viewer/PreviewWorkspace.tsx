import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  ChevronLeft,
  ChevronRight,
  LayoutGrid,
  MonitorPlay,
  PanelLeftOpen,
  PanelRightOpen,
} from 'lucide-react';
import type { Slide } from '../../api/types';
import { Button, Disclosure, IconButton, InlineNotice, Skeleton } from '../../components/ui/primitives';
import { cn } from '../../lib/utils';
import { useBlueprintStore } from '../../stores/blueprintStore';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { useUIStore } from '../../stores/uiStore';
import { DesignSpecSummary } from './DesignSpecSummary';
import { EmptyState } from './EmptyState';
import { IsolatedSlidePreview } from './IsolatedSlidePreview';
import { SlideBlueprintCard } from './SlideBlueprintCard';
import { hasRenderedHTML, ResourceState, useSlideRenderCache } from './useSlideRenderCache';

function PreviewFrame({
  slide,
  state,
  retry,
  title,
}: {
  slide: Slide;
  state: ResourceState<string>;
  retry: () => void;
  title: string;
}) {
  const visibleHtml = state.status === 'ready'
    ? state.data
    : state.status === 'loading' || state.status === 'error'
      ? state.previous
      : undefined;

  return (
    <div className="relative h-full w-full overflow-hidden rounded bg-white shadow-canvas ring-1 ring-border">
      {visibleHtml !== undefined && (
        <IsolatedSlidePreview
          slides={[{ id: slide.id, html: visibleHtml }]}
          index={0}
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
  blueprint,
}: {
  slide: Slide;
  index: number;
  selected: boolean;
  state: ResourceState<string>;
  load: () => void;
  select: () => void;
  blueprint?: React.ReactNode;
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
  return (
    <button
      ref={ref}
      type="button"
      data-testid={`overview-slide-${slide.id}`}
      onClick={select}
      aria-label={`打开第 ${index + 1} 页：${slide.title || '未命名页面'}`}
      className={cn(
        'group relative aspect-video overflow-hidden rounded bg-surface text-left shadow-sm ring-1 ring-border hover:ring-accent',
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
      ) : state.status === 'error' ? (
        <div className="flex h-full flex-col items-center justify-center gap-2 p-4 text-center text-xs text-danger">
          <span>HTML 加载失败</span>
          <span className="text-text-600">打开页面后可重试</span>
        </div>
      ) : blueprint ?? (
        <div className="flex h-full flex-col items-center justify-center gap-1.5 p-5 text-center">
          <span className="text-[10px] font-medium uppercase tracking-wide text-text-400">
            {slide.layout || '页面'}
          </span>
          <span className="line-clamp-2 text-xs font-medium text-text-900">
            {slide.title || '未命名页面'}
          </span>
          <span className="text-[10px] text-text-400">
            {hasRenderedHTML(slide) ? '缩略图加载中' : '页面未物化'}
          </span>
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
      'input, textarea, select, button, a, [contenteditable="true"], [role="separator"], [role="menuitem"], [role="tab"]',
    );
}

export const PreviewWorkspace: React.FC = () => {
  const {
    currentPage,
    previewMode,
    enterOverview,
    exitOverview,
    goNext,
    goPrev,
    effectiveView,
    globalView,
    setGlobalView,
    setCurrentPage,
  } = useDeckStore();
  const { activeProjectId, slidesByProjectId } = useProjectStore();
  const { leftPanelHidden, rightPanelHidden, toggleLeftPanel, toggleRightPanel } = useUIStore();
  const blueprintView = useBlueprintStore((state) => activeProjectId ? state.byProjectId[activeProjectId] : undefined);
  const blueprintLoading = useBlueprintStore((state) => activeProjectId ? state.loading[activeProjectId] : false);
  const blueprintError = useBlueprintStore((state) => activeProjectId ? state.error[activeProjectId] : undefined);
  const loadBlueprint = useBlueprintStore((state) => state.loadProject);
  const projectId = activeProjectId;
  const slides = useMemo(
    () => projectId ? slidesByProjectId[projectId] || [] : [],
    [projectId, slidesByProjectId],
  );
  const { getState, load } = useSlideRenderCache(projectId);
  const [fullscreen, setFullscreen] = useState(false);
  const canvasRef = useRef<HTMLDivElement>(null);

  const hasSlides = slides.length > 0;
  const safePage = Math.min(currentPage, Math.max(0, slides.length - 1));
  const currentSlide = slides[safePage];
  const currentHasHTML = currentSlide ? hasRenderedHTML(currentSlide) : false;
  const currentView = currentSlide ? effectiveView(currentSlide.id, currentHasHTML) : 'html';
  const currentState = currentSlide ? getState(currentSlide) : { status: 'idle' as const };

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
          {previewMode === 'main' && currentSlide && (
            <div className="flex h-8 overflow-hidden rounded-md border border-border bg-surface text-xs">
              <button
                onClick={() => setGlobalView('outline')}
                className={cn('px-3', globalView === 'outline' ? 'bg-accent-soft font-medium text-accent' : 'text-text-600 hover:bg-panel-muted')}
              >蓝图</button>
              <button
                onClick={() => setGlobalView('html')}
                className={cn('border-l border-border px-3', globalView === 'html' ? 'bg-accent-soft font-medium text-accent' : 'text-text-600 hover:bg-panel-muted')}
              >页面</button>
            </div>
          )}

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
              <div className="flex h-full w-full items-center justify-center rounded bg-surface shadow-canvas ring-1 ring-border">
                <EmptyState />
              </div>
            ) : currentView === 'html' && currentHasHTML ? (
              <PreviewFrame
                slide={currentSlide}
                state={currentState}
                retry={() => void load(currentSlide, 'current')}
                title={`第 ${safePage + 1} 页 HTML 预览`}
              />
            ) : currentView === 'html' ? (
              <div className="flex h-full w-full items-center justify-center rounded bg-surface shadow-canvas ring-1 ring-border">
                <InlineNotice tone="warning" className="max-w-md">
                  当前页面尚未生成 HTML。可切换到“蓝图”查看内容，或在右侧让 Agent 生成当前页。
                </InlineNotice>
              </div>
            ) : blueprintView?.slides?.[currentSlide.id] ? (
              <SlideBlueprintCard
                blueprint={blueprintView.slides[currentSlide.id]}
                state={blueprintView.materialization?.[currentSlide.id]?.state ?? 'unknown'}
              />
            ) : blueprintLoading ? (
              <div className="flex h-full w-full flex-col gap-3 rounded bg-surface p-8 shadow-canvas ring-1 ring-border">
                <Skeleton className="h-8 w-2/3" />
                <Skeleton className="h-5 w-1/2" />
                <Skeleton className="mt-4 h-40 w-full" />
                <span className="sr-only">蓝图正在加载</span>
              </div>
            ) : blueprintError ? (
              <InlineNotice tone="danger" className="max-w-md">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <span>蓝图加载失败，请重试。</span>
                    <Disclosure label="错误详情">
                      <p className="break-all font-mono text-[11px]">{blueprintError}</p>
                    </Disclosure>
                  </div>
                  <Button variant="secondary" onClick={() => projectId && void loadBlueprint(projectId)}>重试</Button>
                </div>
              </InlineNotice>
            ) : (
              <InlineNotice tone="warning" className="max-w-md">
                <div className="flex items-center justify-between gap-3">
                  <span>当前页面还没有可用蓝图。</span>
                  <Button variant="secondary" onClick={() => projectId && void loadBlueprint(projectId)}>重新加载</Button>
                </div>
              </InlineNotice>
            )}
          </div>
        ) : (
          <div className="absolute inset-0 overflow-y-auto p-6">
            <div className="mx-auto mb-6 max-w-6xl">
              {blueprintView?.design_spec && <DesignSpecSummary spec={blueprintView.design_spec} />}
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
                    setCurrentPage(index);
                    exitOverview();
                  }}
                  blueprint={blueprintView?.slides?.[slide.id] ? (
                    <div className="pointer-events-none h-full w-full">
                      <SlideBlueprintCard
                        blueprint={blueprintView.slides[slide.id]}
                        state={blueprintView.materialization?.[slide.id]?.state ?? 'unknown'}
                        compact
                      />
                    </div>
                  ) : undefined}
                />
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
