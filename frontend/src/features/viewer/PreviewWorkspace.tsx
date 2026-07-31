import React, { useEffect, useMemo, useState } from 'react';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { useBlueprintStore } from '../../stores/blueprintStore';
import { useUIStore } from '../../stores/uiStore';
import { slidesApi } from '../../api/slides';
import { LayoutGrid, MonitorPlay, ChevronLeft, ChevronRight, PanelLeftOpen, PanelRightOpen } from 'lucide-react';
import { cn } from '../../lib/utils';
import { EmptyState } from './EmptyState';
import { SlideBlueprintCard } from './SlideBlueprintCard';
import { DesignSpecSummary } from './DesignSpecSummary';
import { NewProjectHint } from '../workspace/NewProjectHint';
import { IsolatedSlidePreview } from './IsolatedSlidePreview';
import { RuntimeSlide } from './previewProtocol';

export const PreviewWorkspace: React.FC = () => {
  const { currentPage, previewMode, enterOverview, exitOverview, goNext, goPrev, effectiveView, globalView, setGlobalView } = useDeckStore();
  const { activeProjectId, slidesByProjectId } = useProjectStore();
  const { leftPanelHidden, rightPanelHidden, toggleLeftPanel, toggleRightPanel } = useUIStore();
  const [htmlBySlideId, setHtmlBySlideId] = useState<Record<string, string>>({});
  const blueprintView = useBlueprintStore((state) => activeProjectId ? state.byProjectId[activeProjectId] : undefined);

  const slides = useMemo(() => activeProjectId && activeProjectId !== 'new-pending' ? slidesByProjectId[activeProjectId] || [] : [], [activeProjectId, slidesByProjectId]);
  const hasSlides = slides.length > 0;
  const runtimeSlides = useMemo<RuntimeSlide[]>(
    () => slides.flatMap((slide) => {
      const html = htmlBySlideId[slide.id];
      return slide.html_path && html !== undefined ? [{ id: slide.id, html }] : [];
    }),
    [htmlBySlideId, slides],
  );

  const currentSlide = hasSlides ? slides[Math.min(currentPage, slides.length - 1)] : undefined;
  const currentHasHtml = !!currentSlide?.html_path;
  const currentRuntimeIndex = currentSlide
    ? runtimeSlides.findIndex((slide) => slide.id === currentSlide.id)
    : -1;
  const currentView = currentSlide ? effectiveView(currentSlide.id, currentHasHtml) : 'html';
  const showIframe = previewMode === 'main'
    && currentView === 'html'
    && currentHasHtml
    && currentRuntimeIndex >= 0;

  useEffect(() => {
    const controller = new AbortController();
    const readySlides = slides.filter((slide) => slide.html_path);
    setHtmlBySlideId((current) => Object.keys(current).length > 0 ? {} : current);
    if (readySlides.length === 0) {
      return () => controller.abort();
    }
    void Promise.all(readySlides.map(async (slide) => {
      try {
        return [slide.id, await slidesApi.render(slide.id, controller.signal)] as const;
      } catch (error) {
        if (!controller.signal.aborted) console.error(error);
        return null;
      }
    })).then((entries) => {
      if (controller.signal.aborted) return;
      setHtmlBySlideId(Object.fromEntries(entries.filter((entry): entry is readonly [string, string] => entry !== null)));
    });
    return () => controller.abort();
  }, [activeProjectId, slides]);

  if (activeProjectId === 'new-pending') {
    return <NewProjectHint />;
  }

  return (
    <div className="flex flex-col h-full bg-background relative">
      {/* Toolbar */}
      <div className="h-12 border-b border-border flex items-center justify-between px-4 shrink-0 bg-surface">
        <div className="flex items-center space-x-2">
          {leftPanelHidden && (
            <button 
              onClick={toggleLeftPanel}
              className="p-1.5 rounded-md hover:bg-black/5 text-text-400 hover:text-text-600 transition-colors mr-2"
              title="展开左侧目录"
            >
              <PanelLeftOpen className="w-4 h-4" />
            </button>
          )}
          <button 
            onClick={previewMode === 'overview' ? exitOverview : enterOverview}
            className={cn("p-1.5 rounded-md hover:bg-black/5 text-text-600 transition-colors", previewMode === 'overview' && "bg-mode-overview/10 text-mode-overview")}
            title="Overview"
          >
            <LayoutGrid className="w-4 h-4" />
          </button>
          <button className="p-1.5 rounded-md hover:bg-black/5 text-text-600 transition-colors" title="Present">
            <MonitorPlay className="w-4 h-4" />
          </button>
        </div>

        <div className="flex items-center space-x-4">
          {/* 大纲 / HTML 全局段控：切 globalView，effectiveView 内部按 hasHtml 兜底降级 */}
          {previewMode === 'main' && currentSlide && (
            <div className="flex items-center rounded-md border border-border overflow-hidden text-xs">
              <button
                onClick={() => setGlobalView('outline')}
                className={cn(
                  "px-2.5 py-1 transition-colors",
                  globalView === 'outline' ? "bg-mode-normal/10 text-mode-normal font-medium" : "text-text-600 hover:bg-black/5"
                )}
              >
                蓝图
              </button>
              <button
                onClick={() => setGlobalView('html')}
                className={cn(
                  "px-2.5 py-1 transition-colors border-l border-border",
                  globalView === 'html' ? "bg-mode-normal/10 text-mode-normal font-medium" : "text-text-600 hover:bg-black/5"
                )}
              >
                HTML
              </button>
            </div>
          )}

          <div className="flex items-center space-x-2">
            <button onClick={goPrev} disabled={currentPage === 0} className="p-1 text-text-600 hover:bg-black/5 disabled:opacity-50 rounded">
              <ChevronLeft className="w-5 h-5" />
            </button>
            <span className="text-sm text-text-600 min-w-[3rem] text-center">
              {hasSlides ? `${currentPage + 1} / ${slides.length}` : '0 / 0'}
            </span>
            <button onClick={goNext} disabled={currentPage >= slides.length - 1} className="p-1 text-text-600 hover:bg-black/5 disabled:opacity-50 rounded">
              <ChevronRight className="w-5 h-5" />
            </button>
            {rightPanelHidden && (
              <button 
                onClick={toggleRightPanel}
                className="p-1.5 ml-2 rounded-md hover:bg-black/5 text-text-400 hover:text-text-600 transition-colors"
                title="展开右侧对话"
              >
                <PanelRightOpen className="w-4 h-4" />
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Main Preview Area */}
      <div className="flex-1 overflow-hidden p-6 flex items-center justify-center relative">
        {previewMode === 'main' ? (
          <div className="w-full h-full max-w-5xl aspect-video flex items-center justify-center">
            {!hasSlides ? (
              <div className="w-full h-full bg-white shadow-sm ring-1 ring-border rounded-md overflow-hidden flex items-center justify-center">
                <EmptyState />
              </div>
            ) : showIframe ? (
              <div className="w-full h-full bg-white shadow-sm ring-1 ring-border rounded-md overflow-hidden flex items-center justify-center">
                <IsolatedSlidePreview
                  slides={runtimeSlides}
                  index={currentRuntimeIndex}
                  className="w-full h-full border-none"
                  title="Slide Preview"
                />
              </div>
            ) : (
              currentSlide && blueprintView?.slides?.[currentSlide.id] && (
                <SlideBlueprintCard
                  blueprint={blueprintView.slides[currentSlide.id]}
                  state={blueprintView.materialization?.[currentSlide.id]?.state ?? 'unknown'}
                />
              )
            )}
          </div>
        ) : (
          <div className="absolute inset-0 overflow-y-auto p-6 bg-background">
            <div className="mx-auto mb-6 max-w-6xl">
              {blueprintView?.design_spec && <DesignSpecSummary spec={blueprintView.design_spec} />}
            </div>
            <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-6 max-w-6xl mx-auto">
              {slides.map((slide, i) => (
                <div 
                  key={slide.id} 
                  className={cn(
                    "aspect-video rounded shadow-sm cursor-pointer hover:ring-mode-overview transition-all overflow-hidden relative ring-1 ring-border",
                    currentPage === i && "ring-2 ring-mode-overview"
                  )}
                  onClick={() => {
                    useDeckStore.getState().setCurrentPage(i);
                    exitOverview();
                  }}
                >
                  {slide.html_path && htmlBySlideId[slide.id] !== undefined ? (
                    <IsolatedSlidePreview
                      slides={[{ id: slide.id, html: htmlBySlideId[slide.id] }]}
                      index={0}
                      className="w-full h-full border-none pointer-events-none origin-top-left bg-white"
                      style={{ transform: 'scale(0.25)', width: '400%', height: '400%' }}
                      title={`Slide ${i + 1}`}
                    />
                  ) : (
                    blueprintView?.slides?.[slide.id] ? (
                      <div className="h-full w-full pointer-events-none">
                        <SlideBlueprintCard
                          blueprint={blueprintView.slides[slide.id]}
                          state={blueprintView.materialization?.[slide.id]?.state ?? 'unknown'}
                          compact
                        />
                      </div>
                    ) : null
                  )}
                  {/* 网格空态徽标：左下角 amber，用于一眼分辨该页尚无 HTML 产物（#6 补充） */}
                  {!slide.html_path && (
                    <div className="absolute bottom-2 left-2 bg-amber-500/80 text-white text-[10px] px-1.5 py-0.5 rounded backdrop-blur-sm">
                      暂无 HTML
                    </div>
                  )}
                  <div className="absolute bottom-2 right-2 bg-black/50 text-white text-xs px-1.5 py-0.5 rounded backdrop-blur-sm">
                    {i + 1}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
