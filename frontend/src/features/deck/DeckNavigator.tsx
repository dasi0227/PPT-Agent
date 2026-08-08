import React, { useEffect, useMemo, useState } from 'react';
import { useProjectStore } from '../../stores/projectStore';
import { useDeckStore } from '../../stores/deckStore';
import { useSpecStore } from '../../stores/specStore';
import { useActiveSession } from '../agent/useActiveSession';
import { useUIStore } from '../../stores/uiStore';
import type { Slide } from '../../api/types';
import { SlidePlacement, slidesApi } from '../../api/slides';
import { cn } from '../../lib/utils';
import { Layers, FileText, Plus, Trash2, Presentation, PanelLeftClose, ArrowUp, ArrowDown } from 'lucide-react';
import { ConfirmModal } from '../../components/ui/modal-confirm';
import { IconButton, InlineNotice } from '../../components/ui/primitives';
import { IsolatedSlidePreview } from '../viewer/IsolatedSlidePreview';
import { hasRenderedHTML, type ResourceState, useSlideRenderCache } from '../viewer/useSlideRenderCache';

function formatDirectoryNumber(value: string | undefined, level: 'section' | 'subsection'): string {
  const normalized = String(value ?? '')
    .split('.')
    .filter(Boolean)
    .map((part) => {
      const parsed = Number.parseInt(part, 10);
      return Number.isNaN(parsed) ? part : String(parsed);
    })
    .join('.');
  if (!normalized) return '';
  return level === 'section' && !normalized.includes('.') ? `${normalized}.` : normalized;
}

function SlideThumbnail({
  slide,
  index,
  state,
}: {
  slide: Slide;
  index: number;
  state: ResourceState<string>;
}) {
  const html = state.status === 'ready' ? state.data : undefined;
  return (
    <div className="relative flex h-9 w-14 shrink-0 items-center justify-center overflow-hidden rounded-md border border-border bg-surface text-[11px] font-medium text-text-400">
      {html ? (
        <IsolatedSlidePreview
          slides={[{ id: slide.id, html }]}
          index={0}
          title={`第 ${index + 1} 页缩略图`}
          className="border-0 bg-white motion-safe:animate-[timeline-enter_120ms_ease-out]"
          style={{
            width: '400%',
            height: '400%',
            transform: 'scale(0.25)',
            transformOrigin: 'top left',
            pointerEvents: 'none',
          }}
        />
      ) : '暂无'}
    </div>
  );
}

function SlideDirectoryContent({
  slide,
  index,
  title,
  view,
  state,
}: {
  slide: Slide;
  index: number;
  title: string;
  view: 'html' | 'outline';
  state: ResourceState<string>;
}) {
  return (
    <div className="relative flex h-9 min-w-0 items-center overflow-hidden">
      {view === 'html' ? (
        <div key="html" className="motion-safe:animate-[timeline-enter_120ms_ease-out]">
          <SlideThumbnail slide={slide} index={index} state={state} />
        </div>
      ) : (
        <span
          key="outline"
          className="min-w-0 truncate text-[13px] font-medium text-text-900 motion-safe:animate-[timeline-enter_120ms_ease-out]"
          title={title}
        >
          {title}
        </span>
      )}
    </div>
  );
}

type DirectoryEntry = {
  slide: Slide;
  placement: SlidePlacement;
};

type DirectorySubsection = {
  id: string;
  number: string;
  title: string;
  slides: DirectoryEntry[];
};

type DirectorySection = {
  id: string;
  number: string;
  title: string;
  directSlides: DirectoryEntry[];
  subsections: DirectorySubsection[];
};

function samePlacement(left: SlidePlacement, right: SlidePlacement): boolean {
  return left.section_id === right.section_id && (left.subsection_id ?? '') === (right.subsection_id ?? '');
}

export const DeckNavigator: React.FC = () => {
  const { activeProjectId, projects, slidesByProjectId, loadProjectSlides, projectError } = useProjectStore();
  const { currentPage, globalView, setCurrentPage } = useDeckStore();
  const { toggleLeftPanel } = useUIStore();
  const session = useActiveSession();
  const runActive = session.status === 'running' || session.status === 'waiting';
  const [dragSlideId, setDragSlideId] = useState<string | null>(null);
  const [operationError, setOperationError] = useState('');

  const [slideToDelete, setSlideToDelete] = useState<{id: string, title: string} | null>(null);

  const project = projects.find(p => p.id === activeProjectId);
  const slides = useMemo(
    () => activeProjectId ? slidesByProjectId[activeProjectId] ?? [] : [],
    [activeProjectId, slidesByProjectId],
  );
  const specView = useSpecStore((state) => activeProjectId ? state.byProjectId[activeProjectId] : undefined);
  const { getState: getRenderState, load: loadRender } = useSlideRenderCache(activeProjectId);
  const directorySections = useMemo<DirectorySection[]>(() => {
    if (!specView?.outline) return [];
    const used = new Set<string>();
    return specView.outline.sections.map((section, sectionIndex) => {
      const directSlides: DirectoryEntry[] = [];
      const subsectionSlides = new Map<string, DirectoryEntry[]>();
      for (const subsection of section.subsections) subsectionSlides.set(subsection.id, []);
      for (const slide of slides) {
        const spec = specView.slide_specs?.[slide.id];
        if (!spec || spec.section_id !== section.id) continue;
        const placement: SlidePlacement = {
          slide_id: slide.id,
          section_id: section.id,
          ...(spec.subsection_id ? { subsection_id: spec.subsection_id } : {}),
        };
        const entry = { slide, placement };
        if (spec.subsection_id && subsectionSlides.has(spec.subsection_id)) {
          subsectionSlides.get(spec.subsection_id)!.push(entry);
        } else {
          directSlides.push(entry);
        }
        used.add(slide.id);
      }
      return {
        id: section.id,
        number: String(sectionIndex + 1).padStart(2, '0'),
        title: section.title,
        directSlides,
        subsections: section.subsections.map((subsection, subsectionIndex) => ({
          id: subsection.id,
          number: `${sectionIndex + 1}.${subsectionIndex + 1}`,
          title: subsection.title,
          slides: subsectionSlides.get(subsection.id) ?? [],
        })),
      };
    });
  }, [slides, specView]);
  const directoryEntries = useMemo(
    () => directorySections.flatMap((section) => [
      ...section.directSlides,
      ...section.subsections.flatMap((subsection) => subsection.slides),
    ]),
    [directorySections],
  );

  useEffect(() => {
    for (const slide of slides) {
      if (hasRenderedHTML(slide)) void loadRender(slide, 'prefetch');
    }
  }, [loadRender, slides]);

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

  const handleDragStart = (e: React.DragEvent, slideId: string) => {
    setDragSlideId(slideId);
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', slideId);
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
  };

  const handleRowDrop = (e: React.DragEvent, target: DirectoryEntry) => {
    e.preventDefault();
    const sourceSlideId = e.dataTransfer.getData('text/plain');
    setDragSlideId(null);
    if (!sourceSlideId || sourceSlideId === target.slide.id || !activeProjectId || runActive) {
      return;
    }
    moveEntry(sourceSlideId, target.placement, target.slide.id);
  };

  const submitStructure = async (entries: DirectoryEntry[]) => {
    if (!activeProjectId || runActive || entries.length === 0) return;
    setOperationError('');
    try {
      await slidesApi.restructure(
        activeProjectId,
        entries.map((entry) => entry.slide.id),
        entries.map((entry) => entry.placement),
      );
      refresh();
    } catch (error) {
      setOperationError(error instanceof Error ? `${error.message}。请重试。` : '页面结构调整失败，请重试。');
    }
  };

  const moveEntry = (slideId: string, targetPlacement: Omit<SlidePlacement, 'slide_id'>, beforeSlideId?: string) => {
    const source = directoryEntries.find((entry) => entry.slide.id === slideId);
    if (!source) return;
    const next = directoryEntries
      .filter((entry) => entry.slide.id !== slideId)
      .map((entry) => ({ ...entry, placement: { ...entry.placement } }));
    const moved: DirectoryEntry = {
      slide: source.slide,
      placement: {
        slide_id: source.slide.id,
        section_id: targetPlacement.section_id,
        ...(targetPlacement.subsection_id ? { subsection_id: targetPlacement.subsection_id } : {}),
      },
    };
    let insertAt = beforeSlideId ? next.findIndex((entry) => entry.slide.id === beforeSlideId) : -1;
    if (insertAt < 0) {
      insertAt = next.reduce((last, entry, index) =>
        samePlacement(entry.placement, moved.placement) ? index + 1 : last, next.length);
    }
    next.splice(insertAt, 0, moved);
    void submitStructure(next);
  };

  const moveEntryByDirection = (entry: DirectoryEntry, direction: -1 | 1) => {
    const currentIndex = directoryEntries.findIndex((candidate) => candidate.slide.id === entry.slide.id);
    const target = directoryEntries[currentIndex + direction];
    if (!target) return;
    const crossesSection = entry.placement.section_id !== target.placement.section_id;
    const next = directoryEntries
      .filter((candidate) => candidate.slide.id !== entry.slide.id)
      .map((candidate) => ({ ...candidate, placement: { ...candidate.placement } }));
    const moved: DirectoryEntry = {
      slide: entry.slide,
      placement: {
        slide_id: entry.slide.id,
        section_id: target.placement.section_id,
        ...(!crossesSection && target.placement.subsection_id ? { subsection_id: target.placement.subsection_id } : {}),
      },
    };
    const targetIndex = next.findIndex((candidate) => candidate.slide.id === target.slide.id);
    const insertAt = crossesSection
      ? targetIndex + (direction < 0 ? 1 : 0)
      : targetIndex + (direction > 0 ? 1 : 0);
    next.splice(insertAt, 0, moved);
    void submitStructure(next);
    setCurrentPage(insertAt);
  };

  const handleGroupDrop = (event: React.DragEvent, targetPlacement: Omit<SlidePlacement, 'slide_id'>) => {
    event.preventDefault();
    const sourceSlideId = event.dataTransfer.getData('text/plain');
    setDragSlideId(null);
    if (!sourceSlideId || !activeProjectId || runActive) return;
    moveEntry(sourceSlideId, targetPlacement);
  };

  const renderSlideRow = (entry: DirectoryEntry, renderedIndex: number) => {
    const { slide } = entry;
    const spec = specView?.slide_specs?.[slide.id];
    const slideIndex = slides.findIndex((item) => item.id === slide.id);
    const directoryIndex = directoryEntries.findIndex((item) => item.slide.id === slide.id);
    return (
      <div
        key={slide.id}
        draggable={!runActive}
        onDragStart={(event) => handleDragStart(event, slide.id)}
        onDragOver={handleDragOver}
        onDrop={(event) => handleRowDrop(event, entry)}
        onDragEnd={() => setDragSlideId(null)}
        className={cn(
          "group relative grid min-h-[60px] w-full grid-cols-[28px_minmax(0,1fr)_78px] items-center gap-2 rounded-md border-l-[3px] border-transparent px-2 py-2 text-sm transition-colors cursor-pointer focus-within:bg-panel-muted",
          currentPage === slideIndex
            ? "bg-accent-soft text-accent font-medium border-l-[3px] border-accent"
            : "text-text-600 hover:bg-black/5",
          dragSlideId === slide.id && "opacity-50"
        )}
        onClick={() => slideIndex >= 0 && setCurrentPage(slideIndex)}
      >
        <span className="text-center text-xs font-semibold tabular-nums text-text-400 group-hover:text-text-600">{String(renderedIndex + 1).padStart(2, '0')}</span>
        <SlideDirectoryContent
          slide={slide}
          index={renderedIndex}
          title={slide.title || spec?.title || '未命名'}
          view={globalView}
          state={getRenderState(slide)}
        />
        {!runActive && (
          <div className="flex shrink-0 justify-end opacity-0 group-hover:opacity-100 group-focus-within:opacity-100">
            <IconButton label="上移本页" className="h-6 w-6" disabled={directoryIndex <= 0} onClick={(event) => { event.stopPropagation(); moveEntryByDirection(entry, -1); }}>
              <ArrowUp className="h-3.5 w-3.5" />
            </IconButton>
            <IconButton label="下移本页" className="h-6 w-6" disabled={directoryIndex < 0 || directoryIndex >= directoryEntries.length - 1} onClick={(event) => { event.stopPropagation(); moveEntryByDirection(entry, 1); }}>
              <ArrowDown className="h-3.5 w-3.5" />
            </IconButton>
            <IconButton label="删除本页" className="h-6 w-6 hover:bg-danger-soft hover:text-danger" onClick={(event) => { event.stopPropagation(); handleDelete(slide.id, slide.title); }}>
              <Trash2 className="h-3.5 w-3.5" />
            </IconButton>
          </div>
        )}
      </div>
    );
  };

  return (
    <div className="flex flex-col h-full bg-panel">
      <div className="h-12 border-b border-border flex items-center justify-between px-3 shrink-0 bg-panel">
        <div className="flex items-center">
          <Presentation className="mr-2 h-4 w-4 text-accent" strokeWidth={1.75} />
          <span className="text-sm font-semibold text-text-900">目录</span>
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
            ) : directorySections.length === 0 ? (
              <div className="text-center p-4 text-text-400 text-sm">目录加载中...</div>
            ) : (
              (() => {
                let renderedIndex = 0;
                return directorySections.map((section) => (
                  <React.Fragment key={section.id}>
                    <div
                      onDragOver={handleDragOver}
                      onDrop={(event) => handleGroupDrop(event, { section_id: section.id })}
                      className="px-3 pb-1 pt-3 text-[12px] font-normal text-text-600"
                    >
                      {formatDirectoryNumber(section.number, 'section')} {section.title}
                    </div>
                    {(() => {
                      const subsectionByID = new Map(section.subsections.map((subsection) => [subsection.id, subsection]));
                      const renderedSubsections = new Set<string>();
                      const sectionEntries = directoryEntries.filter((entry) => entry.placement.section_id === section.id);
                      const rows: React.ReactNode[] = [];
                      for (const entry of sectionEntries) {
                        const subsectionID = entry.placement.subsection_id;
                        const subsection = subsectionID ? subsectionByID.get(subsectionID) : undefined;
                        if (subsection && !renderedSubsections.has(subsection.id)) {
                          renderedSubsections.add(subsection.id);
                          rows.push(
                            <div
                              key={`${section.id}:${subsection.id}:heading`}
                              onDragOver={handleDragOver}
                              onDrop={(event) => handleGroupDrop(event, { section_id: section.id, subsection_id: subsection.id })}
                              className="px-3 py-1 text-[11px] font-normal text-text-400"
                            >
                              {formatDirectoryNumber(subsection.number, 'subsection')} {subsection.title}
                            </div>,
                          );
                        }
                        rows.push(renderSlideRow(entry, renderedIndex));
                        renderedIndex += 1;
                      }
                      for (const subsection of section.subsections) {
                        if (renderedSubsections.has(subsection.id)) continue;
                        rows.push(
                          <div
                            key={`${section.id}:${subsection.id}:empty-heading`}
                            onDragOver={handleDragOver}
                            onDrop={(event) => handleGroupDrop(event, { section_id: section.id, subsection_id: subsection.id })}
                            className="px-3 py-1 text-[11px] font-normal text-text-400"
                          >
                            {formatDirectoryNumber(subsection.number, 'subsection')} {subsection.title}
                          </div>,
                        );
                      }
                      return rows;
                    })()}
                  </React.Fragment>
                ));
              })()
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
