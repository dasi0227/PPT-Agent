import React from 'react';
import { useProjectStore } from '../../stores/projectStore';
import { useDeckStore } from '../../stores/deckStore';
import { cn } from '../../lib/utils';
import { Layers, FileText } from 'lucide-react';

export const DeckNavigator: React.FC = () => {
  const { activeProjectId, projects, slidesByProjectId } = useProjectStore();
  const { currentPage, setCurrentPage } = useDeckStore();

  const project = projects.find(p => p.id === activeProjectId);
  const slides = activeProjectId ? slidesByProjectId[activeProjectId] || [] : [];

  if (!project) {
    return <div className="p-4 text-text-400 text-sm">No project selected</div>;
  }

  return (
    <div className="flex flex-col h-full">
      <div className="p-4 border-b border-border">
          <h2 className="font-semibold text-text-900 truncate" title={project.title}>{project.title || 'Untitled Project'}</h2>
        <div className="flex items-center text-xs text-text-600 mt-1 space-x-3">
          <span className="flex items-center"><Layers className="w-3 h-3 mr-1"/> {project.theme}</span>
          <span className="flex items-center"><FileText className="w-3 h-3 mr-1"/> {slides.length} pages</span>
        </div>
      </div>
      
      <div className="flex-1 overflow-y-auto p-2 space-y-1">
        {slides.length === 0 ? (
          <div className="text-center p-4 text-text-400 text-sm">No slides yet</div>
        ) : (
          slides.map((slide, index) => (
            <button
              key={slide.id}
              onClick={() => setCurrentPage(index)}
              className={cn(
                "w-full text-left px-3 py-2 rounded-md text-sm transition-colors flex items-center group",
                currentPage === index 
                  ? "bg-mode-normal/10 text-mode-normal font-medium" 
                  : "text-text-600 hover:bg-black/5"
              )}
            >
              <span className="w-6 text-xs text-text-400 group-hover:text-text-600">{index + 1}</span>
              <span className="truncate">Slide {index + 1}</span>
            </button>
          ))
        )}
      </div>
    </div>
  );
};
