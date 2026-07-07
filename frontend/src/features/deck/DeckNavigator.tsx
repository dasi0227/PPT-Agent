import React, { useState } from 'react';
import { useProjectStore } from '../../stores/projectStore';
import { useDeckStore } from '../../stores/deckStore';
import { useActiveSession } from '../agent/useActiveSession';
import { slidesApi } from '../../api/slides';
import { cn } from '../../lib/utils';
import { Layers, FileText, AlertTriangle, Plus, Trash2 } from 'lucide-react';

export const DeckNavigator: React.FC = () => {
  const { activeProjectId, projects, slidesByProjectId, loadProjectSlides } = useProjectStore();
  const { currentPage, setCurrentPage } = useDeckStore();
  const session = useActiveSession();
  const runActive = session.status === 'running' || session.status === 'needs_input';
  const [dragIndex, setDragIndex] = useState<number | null>(null);

  const project = projects.find(p => p.id === activeProjectId);
  const slides = activeProjectId ? slidesByProjectId[activeProjectId] || [] : [];

  if (!project) {
    return <div className="p-4 text-text-400 text-sm">No project selected</div>;
  }

  const refresh = () => {
    if (activeProjectId) void loadProjectSlides(activeProjectId);
  };

  const handleAdd = () => {
    if (!activeProjectId || runActive) return;
    const anchor = slides.length > 0 ? slides[slides.length - 1].id : undefined;
    slidesApi.add(activeProjectId, anchor ? { after_slide_id: anchor } : {})
      .then(refresh)
      .catch((err) => console.error(err));
  };

  const handleDelete = (slideId: string, title: string) => {
    if (runActive) return;
    if (!window.confirm(`确认删除「${title || '未命名'}」这一页吗？此操作不可撤销。`)) return;
    slidesApi.remove(slideId).then(refresh).catch((err) => console.error(err));
  };

  const handleDrop = (targetIndex: number) => {
    if (dragIndex === null || dragIndex === targetIndex || !activeProjectId || runActive) {
      setDragIndex(null);
      return;
    }
    const ids = slides.map((s) => s.id);
    const [moved] = ids.splice(dragIndex, 1);
    ids.splice(targetIndex, 0, moved);
    setDragIndex(null);
    slidesApi.reorder(activeProjectId, ids).then(refresh).catch((err) => console.error(err));
  };

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
            <div
              key={slide.id}
              draggable={!runActive}
              onDragStart={() => setDragIndex(index)}
              onDragOver={(e) => e.preventDefault()}
              onDrop={() => handleDrop(index)}
              className={cn(
                "w-full px-3 py-2 rounded-md text-sm transition-colors flex items-center group cursor-pointer",
                currentPage === index
                  ? "bg-mode-normal/10 text-mode-normal font-medium"
                  : "text-text-600 hover:bg-black/5",
                dragIndex === index && "opacity-50"
              )}
              onClick={() => setCurrentPage(index)}
            >
              <span className="w-6 text-xs text-text-400 group-hover:text-text-600">{index + 1}</span>
              <span className="truncate flex-1">{slide.title || '未命名'}</span>
              {slide.outline_dirty && (
                <span
                  className="ml-1 shrink-0 text-amber-600 inline-flex"
                  aria-label="待更新"
                  title="大纲已改，待更新"
                >
                  <AlertTriangle className="w-3.5 h-3.5" />
                </span>
              )}
              {!runActive && (
                <button
                  aria-label="删除本页"
                  title="删除本页"
                  onClick={(e) => { e.stopPropagation(); handleDelete(slide.id, slide.title); }}
                  className="ml-1 shrink-0 p-0.5 rounded text-text-400 opacity-0 group-hover:opacity-100 hover:text-red-500 transition-opacity"
                >
                  <Trash2 className="w-3.5 h-3.5" />
                </button>
              )}
            </div>
          ))
        )}
      </div>

      <div className="p-2 border-t border-border">
        <button
          onClick={handleAdd}
          disabled={runActive}
          title={runActive ? 'AI 运行中，暂不可编辑结构' : '在末尾加一页'}
          className="w-full flex items-center justify-center px-3 py-2 rounded-md text-sm text-text-600 hover:bg-black/5 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
        >
          <Plus className="w-4 h-4 mr-1" /> 加页
        </button>
      </div>
    </div>
  );
};
