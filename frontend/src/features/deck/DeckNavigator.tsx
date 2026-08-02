import React, { useState } from 'react';
import { useProjectStore } from '../../stores/projectStore';
import { useDeckStore } from '../../stores/deckStore';
import { useBlueprintStore } from '../../stores/blueprintStore';
import { useActiveSession } from '../agent/useActiveSession';
import { useUIStore } from '../../stores/uiStore';
import { slidesApi } from '../../api/slides';
import { cn } from '../../lib/utils';
import { Layers, FileText, Plus, Trash2, Presentation, PanelLeftClose, ArrowUp, ArrowDown } from 'lucide-react';
import { ConfirmModal } from '../../components/ui/modal-confirm';
import { MaterializationBadge } from '../viewer/MaterializationBadge';
import { IconButton, InlineNotice } from '../../components/ui/primitives';
import { hasRenderedHTML } from '../viewer/useSlideRenderCache';

export const DeckNavigator: React.FC = () => {
  const { activeProjectId, projects, slidesByProjectId, loadProjectSlides, projectError } = useProjectStore();
  const { currentPage, setCurrentPage } = useDeckStore();
  const { toggleLeftPanel } = useUIStore();
  const session = useActiveSession();
  const runActive = session.status === 'running' || session.status === 'waiting';
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [operationError, setOperationError] = useState('');

  const [slideToDelete, setSlideToDelete] = useState<{id: string, title: string} | null>(null);

  const project = projects.find(p => p.id === activeProjectId);
  const slides = activeProjectId ? slidesByProjectId[activeProjectId] || [] : [];
  const blueprintView = useBlueprintStore((state) => activeProjectId ? state.byProjectId[activeProjectId] : undefined);

  const refresh = () => {
    if (activeProjectId) void loadProjectSlides(activeProjectId);
  };

  const handleAdd = async () => {
    if (!activeProjectId || runActive) return;
    setOperationError('');
    const anchor = slides.length > 0 ? slides[slides.length - 1].id : undefined;
    try {
      await slidesApi.add(activeProjectId, anchor ? { after_slide_id: anchor } : {});
      refresh();
    } catch (error) {
      setOperationError(error instanceof Error ? `${error.message}。请重试。` : '新增页面失败，请重试。');
    }
  };

  const handleDelete = (slideId: string, title: string) => {
    if (runActive) return;
    setSlideToDelete({ id: slideId, title: title });
  };

  const confirmDelete = async () => {
    if (!slideToDelete) return;
    setOperationError('');
    try {
      await slidesApi.remove(slideToDelete.id);
      refresh();
    } catch (error) {
      setOperationError(error instanceof Error ? `${error.message}。请重试。` : '删除页面失败，请重试。');
      throw error;
    }
  };

  const handleDragStart = (e: React.DragEvent, index: number) => {
    setDragIndex(index);
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', index.toString());
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
  };

  const handleDrop = (e: React.DragEvent, targetIndex: number) => {
    e.preventDefault();
    const sourceIndexStr = e.dataTransfer.getData('text/plain');
    if (!sourceIndexStr) return;

    const sourceIndex = parseInt(sourceIndexStr, 10);
    if (sourceIndex === targetIndex || !activeProjectId || runActive) {
      setDragIndex(null);
      return;
    }

    const ids = slides.map((s) => s.id);
    const [moved] = ids.splice(sourceIndex, 1);
    ids.splice(targetIndex, 0, moved);
    setDragIndex(null);
    void reorder(ids);
  };

  const reorder = async (ids: string[]) => {
    if (!activeProjectId || runActive) return;
    setOperationError('');
    try {
      await slidesApi.reorder(activeProjectId, ids);
      refresh();
    } catch (error) {
      setOperationError(error instanceof Error ? `${error.message}。请重试。` : '页面排序失败，请重试。');
    }
  };

  const moveSlide = (index: number, direction: -1 | 1) => {
    const target = index + direction;
    if (target < 0 || target >= slides.length || runActive) return;
    const ids = slides.map((slide) => slide.id);
    [ids[index], ids[target]] = [ids[target], ids[index]];
    void reorder(ids);
    setCurrentPage(target);
  };

  return (
    <div className="flex flex-col h-full bg-panel">
      <div className="h-12 border-b border-border flex items-center justify-between px-3 shrink-0 bg-panel">
        <div className="flex items-center">
          <Presentation className="w-4 h-4 text-text-600 mr-2" />
          <span className="font-medium text-text-900 text-sm">幻灯片</span>
        </div>
        <IconButton label="隐藏左侧目录" onClick={toggleLeftPanel}>
          <PanelLeftClose className="w-4 h-4" strokeWidth={1.75} />
        </IconButton>
      </div>

      {!project ? (
        <div className="flex-1 flex items-center justify-center p-6 text-center">
          <div className="flex flex-col items-center">
            <div className="w-12 h-12 rounded-full bg-black/5 flex items-center justify-center mb-4">
              <FileText className="w-6 h-6 text-text-400" />
            </div>
            <p className="text-sm font-medium text-text-900 mb-1">等待输入需求...</p>
            <p className="text-xs text-text-400">请在右侧发送指令以生成大纲</p>
          </div>
        </div>
      ) : (
        <>
          <div className="p-4 border-b border-border">
            <h2 className="font-semibold text-text-900 truncate" title={project.title}>{project.title || '未命名项目'}</h2>
            <div className="flex items-center text-xs text-text-600 mt-1 space-x-3">
              <span className="flex items-center"><Layers className="w-3 h-3 mr-1"/> {project.theme}</span>
              <span className="flex items-center"><FileText className="w-3 h-3 mr-1"/> {slides.length} 页</span>
            </div>
          </div>

          {(operationError || projectError) && <InlineNotice tone="danger" className="m-2 text-xs">{operationError || `${projectError}。请重试。`}</InlineNotice>}
          <div className="flex-1 overflow-y-auto p-2 space-y-1">
            {slides.length === 0 ? (
              <div className="text-center p-4 text-text-400 text-sm">暂无页面</div>
            ) : (
              slides.map((slide, index) => {
                const bp = blueprintView?.slides?.[slide.id];
                const section = blueprintView?.deck?.sections?.find((item) => item.id === bp?.section_id);
                const subsection = section?.subsections.find((item) => item.id === bp?.subsection_id);
                const prev = index > 0 ? blueprintView?.slides?.[slides[index - 1].id] : undefined;
                return (
                <React.Fragment key={slide.id}>
                  {section && prev?.section_id !== bp?.section_id && (
                    <div className="px-3 pb-1 pt-3 text-[11px] font-semibold uppercase tracking-wide text-text-400">
                      {section.number} {section.title}
                    </div>
                  )}
                  {subsection && prev?.subsection_id !== bp?.subsection_id && (
                    <div className="px-3 py-1 text-[10px] font-medium text-text-400">{subsection.number} {subsection.title}</div>
                  )}
                <div
                  draggable={!runActive}
                  onDragStart={(e) => handleDragStart(e, index)}
                  onDragOver={(e) => handleDragOver(e)}
                  onDrop={(e) => handleDrop(e, index)}
                  className={cn(
                    "relative w-full border-l-[3px] border-transparent px-2 py-2 rounded-md text-sm transition-colors flex items-center group cursor-pointer focus-within:bg-panel-muted",
                    currentPage === index
                      ? "bg-accent-soft text-accent font-medium border-l-[3px] border-accent"
                      : "text-text-600 hover:bg-black/5",
                    dragIndex === index && "opacity-50"
                  )}
                  onClick={() => setCurrentPage(index)}
                >
                  <div className="mr-2 flex h-9 w-14 shrink-0 items-center justify-center rounded border border-border bg-surface text-[10px] text-text-400">
                    {slide.layout || '版式'}
                  </div>
                  <span className="w-6 text-xs tabular-nums text-text-400 group-hover:text-text-600">{index + 1}</span>
                  <span className="truncate flex-1" title={slide.title || '未命名'}>{slide.title || '未命名'}</span>
                  <MaterializationBadge state={
                    blueprintView?.materialization?.[slide.id]?.state
                      ?? (hasRenderedHTML(slide) ? 'unknown' : 'not_materialized')
                  } />
                  {!runActive && (
                    <div className="ml-1 flex shrink-0 opacity-0 group-hover:opacity-100 group-focus-within:opacity-100">
                      <IconButton label="上移本页" className="h-6 w-6" disabled={index === 0} onClick={(event) => { event.stopPropagation(); moveSlide(index, -1); }}>
                        <ArrowUp className="h-3.5 w-3.5" />
                      </IconButton>
                      <IconButton label="下移本页" className="h-6 w-6" disabled={index === slides.length - 1} onClick={(event) => { event.stopPropagation(); moveSlide(index, 1); }}>
                        <ArrowDown className="h-3.5 w-3.5" />
                      </IconButton>
                      <IconButton label="删除本页" className="h-6 w-6 hover:bg-danger-soft hover:text-danger" onClick={(event) => { event.stopPropagation(); handleDelete(slide.id, slide.title); }}>
                        <Trash2 className="h-3.5 w-3.5" />
                      </IconButton>
                    </div>
                  )}
                </div>
                </React.Fragment>
              )})
            )}
          </div>

          <div className="p-2 border-t border-border">
            <button
              onClick={() => void handleAdd()}
              disabled={runActive}
              title={runActive ? 'AI 运行中，暂不可编辑结构' : '在末尾加一页'}
              className="w-full flex items-center justify-center px-3 py-2 rounded-md text-sm text-text-600 hover:bg-black/5 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              <Plus className="w-4 h-4 mr-1" /> 加页
            </button>
          </div>

          <ConfirmModal
            open={!!slideToDelete}
            onOpenChange={(open) => !open && setSlideToDelete(null)}
            title="删除页面"
            description={`确认删除「${slideToDelete?.title || '未命名'}」这一页吗？此操作不可撤销。`}
            variant="danger"
            confirmLabel="删除"
            onConfirm={confirmDelete}
          />
        </>
      )}
    </div>
  );
};
