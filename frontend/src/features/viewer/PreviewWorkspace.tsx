import React, { useEffect, useRef } from 'react';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { LayoutGrid, MonitorPlay, ChevronLeft, ChevronRight } from 'lucide-react';
import { cn } from '../../lib/utils';

export const PreviewWorkspace: React.FC = () => {
  const { currentPage, previewMode, enterOverview, exitOverview, goNext, goPrev } = useDeckStore();
  const { activeProjectId, slidesByProjectId } = useProjectStore();
  const iframeRef = useRef<HTMLIFrameElement>(null);

  const slides = activeProjectId ? slidesByProjectId[activeProjectId] || [] : [];
  const hasSlides = slides.length > 0;

  useEffect(() => {
    if (iframeRef.current && iframeRef.current.contentWindow && previewMode === 'main') {
      iframeRef.current.contentWindow.postMessage({ type: 'goto', index: currentPage }, '*');
    }
  }, [currentPage, previewMode]);

  // Update content via postMessage instead of srcDoc when slide content changes
  useEffect(() => {
    if (iframeRef.current && iframeRef.current.contentWindow && previewMode === 'main') {
      iframeRef.current.contentWindow.postMessage({ 
        type: 'update', 
        content: slides[currentPage]?.html_path || ''
      }, '*');
    }
  }, [slides, currentPage, previewMode]);

  return (
    <div className="flex flex-col h-full bg-background relative">
      {/* Toolbar */}
      <div className="h-12 border-b border-border flex items-center justify-between px-4 shrink-0 bg-surface">
        <div className="flex items-center space-x-2">
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
        </div>
      </div>

      {/* Main Preview Area */}
      <div className="flex-1 overflow-hidden p-6 flex items-center justify-center relative">
        {previewMode === 'main' ? (
          <div className="w-full h-full max-w-5xl aspect-video bg-white shadow-sm ring-1 ring-border rounded-md overflow-hidden flex items-center justify-center">
            {hasSlides ? (
              <iframe
                ref={iframeRef}
                src="/slide-runtime/index.html"
                sandbox="allow-scripts allow-same-origin"
                className="w-full h-full border-none"
                title="Slide Preview"
              />
            ) : (
              <div className="text-text-400">No content to preview</div>
            )}
          </div>
        ) : (
          <div className="absolute inset-0 overflow-y-auto p-6 bg-background">
            <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-6 max-w-6xl mx-auto">
              {slides.map((slide, i) => (
                <div 
                  key={slide.id} 
                  className={cn(
                    "aspect-video bg-white ring-1 ring-border rounded shadow-sm cursor-pointer hover:ring-mode-overview transition-all overflow-hidden relative",
                    currentPage === i && "ring-2 ring-mode-overview"
                  )}
                  onClick={() => {
                    useDeckStore.getState().setCurrentPage(i);
                    exitOverview();
                  }}
                >
                  <iframe
                    src={slide.html_path || '/slide-runtime/index.html'}
                    sandbox="allow-scripts"
                    className="w-full h-full border-none pointer-events-none origin-top-left"
                    style={{ transform: 'scale(0.25)', width: '400%', height: '400%' }}
                    title={`Slide ${i + 1}`}
                  />
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
