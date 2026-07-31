import React, { useEffect, useMemo, useRef } from 'react';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { useUIStore } from '../../stores/uiStore';
import { useActiveSession } from '../agent/useActiveSession';
import { slidesApi, SlidePatch } from '../../api/slides';
import { LayoutGrid, MonitorPlay, ChevronLeft, ChevronRight, PanelLeftOpen, PanelRightOpen } from 'lucide-react';
import { cn } from '../../lib/utils';
import { EmptyState } from './EmptyState';
import { OutlineCard } from './OutlineCard';
import { NewProjectHint } from '../workspace/NewProjectHint';

export const PreviewWorkspace: React.FC = () => {
  const { currentPage, previewMode, enterOverview, exitOverview, goNext, goPrev, effectiveView, globalView, setGlobalView } = useDeckStore();
  const { activeProjectId, slidesByProjectId, loadProjectSlides } = useProjectStore();
  const { leftPanelHidden, rightPanelHidden, toggleLeftPanel, toggleRightPanel } = useUIStore();
  const session = useActiveSession();
  const runActive = session.status === 'running' || session.status === 'needs_input';
  const iframeRef = useRef<HTMLIFrameElement>(null);

  const slides = useMemo(() => activeProjectId && activeProjectId !== 'new-pending' ? slidesByProjectId[activeProjectId] || [] : [], [activeProjectId, slidesByProjectId]);
  const hasSlides = slides.length > 0;
  const slidePaths = useMemo(() => slides.map((slide) => slide.html_path ? `/api/v1/slides/${encodeURIComponent(slide.id)}/render` : ''), [slides]);
  const currentPageRef = useRef(currentPage);

  const currentSlide = hasSlides ? slides[Math.min(currentPage, slides.length - 1)] : undefined;
  const currentHasHtml = !!currentSlide?.html_path;
  const currentView = currentSlide ? effectiveView(currentSlide.id, currentHasHtml) : 'html';
  const showIframe = previewMode === 'main' && currentView === 'html' && currentHasHtml;

  const patchSlide = (slideId: string, patch: SlidePatch) => {
    if (!activeProjectId || activeProjectId === 'new-pending') return;
    slidesApi.patch(slideId, patch)
      .then(() => loadProjectSlides(activeProjectId))
      .catch((err) => console.error(err));
  };

  useEffect(() => {
    currentPageRef.current = currentPage;
  }, [currentPage]);

  useEffect(() => {
    if (iframeRef.current && iframeRef.current.contentWindow && showIframe) {
      iframeRef.current.contentWindow.postMessage({ type: 'goto', index: currentPage }, '*');
    }
  }, [currentPage, showIframe]);

  // Load the whole deck into the runtime once, then let `goto` switch active pages.
  useEffect(() => {
    if (iframeRef.current && iframeRef.current.contentWindow && showIframe && slidePaths.length > 0) {
      iframeRef.current.contentWindow.postMessage({
        type: 'update',
        slides: slidePaths,
        index: currentPageRef.current
      }, '*');
    }
  }, [activeProjectId, showIframe, slidePaths]);

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
                大纲
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
                <iframe
                  ref={iframeRef}
                  src="/slide-runtime/index.html"
                  sandbox="allow-scripts allow-same-origin"
                  className="w-full h-full border-none"
                  title="Slide Preview"
                />
              </div>
            ) : (
              currentSlide && (
                <OutlineCard
                  slide={currentSlide}
                  editable={!runActive}
                  dirty={currentSlide.outline_dirty}
                  onPatch={(patch) => patchSlide(currentSlide.id, patch)}
                />
              )
            )}
          </div>
        ) : (
          <div className="absolute inset-0 overflow-y-auto p-6 bg-background">
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
                  {slide.html_path ? (
                    <iframe
                      src={`/api/v1/slides/${encodeURIComponent(slide.id)}/render`}
                      sandbox="allow-scripts"
                      className="w-full h-full border-none pointer-events-none origin-top-left bg-white"
                      style={{ transform: 'scale(0.25)', width: '400%', height: '400%' }}
                      title={`Slide ${i + 1}`}
                    />
                  ) : (
                    <div className="w-full h-full pointer-events-none">
                      <OutlineCard slide={slide} editable={false} dirty={slide.outline_dirty} onPatch={() => {}} compact />
                    </div>
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
