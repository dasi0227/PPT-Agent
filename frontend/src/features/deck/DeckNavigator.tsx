import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useProjectStore } from '../../stores/projectStore';
import { useDeckStore } from '../../stores/deckStore';
import { useActiveSession } from '../agent/useActiveSession';
import { useUIStore } from '../../stores/uiStore';
import type { ProjectContentSnapshot, Slide } from '../../api/types';
import { SlidePlacement, slidesApi } from '../../api/slides';
import { cn } from '../../lib/utils';
import { Layers, FileText, Plus, Presentation, PanelLeftClose, ChevronRight, MoreHorizontal } from 'lucide-react';
import { ConfirmModal } from '../../components/ui/modal-confirm';
import { FormModal } from '../../components/ui/modal-form';
import { IconButton, InlineNotice } from '../../components/ui/primitives';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '../../components/ui/dropdown-menu';
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
  return level === 'section' && normalized.includes('.') ? normalized.split('.')[0] : normalized;
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

type DirectoryGroup = {
  id: string;
  placement: Omit<SlidePlacement, 'slide_id'>;
  entries: DirectoryEntry[];
};

type DirectoryRenameTarget =
  | { kind: 'section'; sectionId: string; title: string }
  | { kind: 'subsection'; sectionId: string; subsectionId: string; title: string }
  | { kind: 'slide'; slideId: string; title: string };

function samePlacement(left: SlidePlacement, right: SlidePlacement): boolean {
  return left.section_id === right.section_id && (left.subsection_id ?? '') === (right.subsection_id ?? '');
}

export const DeckNavigator: React.FC = () => {
  const {
    activeProjectId,
    projects,
    slidesByProjectId,
    specByProjectId,
    loadProjectContent,
    applyProjectContentSnapshot,
    projectError,
  } = useProjectStore();
  const { currentSlideId, globalView, setCurrentSlideId } = useDeckStore();
  const { toggleLeftPanel } = useUIStore();
  const session = useActiveSession();
  const runActive = session.status === 'running' || session.status === 'waiting';
  const [dragSlideId, setDragSlideId] = useState<string | null>(null);
  const [operationError, setOperationError] = useState('');
  const [expandedSectionIds, setExpandedSectionIds] = useState<Set<string>>(new Set());
  const [structureUpdating, setStructureUpdating] = useState(false);
  const [keyboardFocusTarget, setKeyboardFocusTarget] = useState<string | null>(null);
  const lastInteractionWasKeyboard = useRef(false);

  const [slideToDelete, setSlideToDelete] = useState<{id: string, title: string} | null>(null);
  const [structureToDelete, setStructureToDelete] = useState<
    | { kind: 'section'; sectionId: string; title: string }
    | { kind: 'subsection'; sectionId: string; subsectionId: string; title: string; fallsBackToDirect: boolean }
    | null
  >(null);
  const [renameTarget, setRenameTarget] = useState<DirectoryRenameTarget | null>(null);

  const project = projects.find(p => p.id === activeProjectId);
  const slides = useMemo(
    () => activeProjectId ? slidesByProjectId[activeProjectId] ?? [] : [],
    [activeProjectId, slidesByProjectId],
  );
  const specView = activeProjectId ? specByProjectId[activeProjectId] : undefined;
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
  const directoryGroups = useMemo<DirectoryGroup[]>(
    () => directorySections.flatMap((section) => {
      const directGroup: DirectoryGroup = {
        id: `section:${section.id}`,
        placement: { section_id: section.id },
        entries: section.directSlides,
      };
      const subsectionGroups: DirectoryGroup[] = section.subsections.map((subsection) => ({
        id: `subsection:${subsection.id}`,
        placement: { section_id: section.id, subsection_id: subsection.id },
        entries: subsection.slides,
      }));
      return [
        ...(section.directSlides.length > 0 || section.subsections.length === 0 ? [directGroup] : []),
        ...subsectionGroups,
      ];
    }),
    [directorySections],
  );
  const directoryEntries = useMemo(
    () => directoryGroups.flatMap((group) => group.entries),
    [directoryGroups],
  );
  const currentSectionId = useMemo(() => {
    const currentSlide = slides.find((slide) => slide.id === currentSlideId);
    return currentSlide ? specView?.slide_specs?.[currentSlide.id]?.section_id : undefined;
  }, [currentSlideId, slides, specView]);

  useEffect(() => {
    setExpandedSectionIds(new Set());
  }, [activeProjectId]);

  useEffect(() => {
    if (!currentSectionId) return;
    setExpandedSectionIds((current) => {
      if (current.has(currentSectionId)) return current;
      const next = new Set(current);
      next.add(currentSectionId);
      return next;
    });
  }, [activeProjectId, currentSectionId]);

  useEffect(() => {
    for (const slide of slides) {
      if (hasRenderedHTML(slide)) void loadRender(slide, 'prefetch');
    }
  }, [loadRender, slides]);

  const refresh = () => {
    if (activeProjectId) void loadProjectContent(activeProjectId);
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
    if (runActive || structureUpdating) return;
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

  const submitStructure = async (entries: DirectoryEntry[]): Promise<boolean> => {
    const projectId = activeProjectId;
    if (!projectId || runActive || structureUpdating || entries.length === 0) return false;
    setOperationError('');
    setStructureUpdating(true);
    try {
      const snapshot = await slidesApi.restructure(
        projectId,
        entries.map((entry) => entry.slide.id),
        entries.map((entry) => entry.placement),
      );
      applyProjectContentSnapshot(projectId, snapshot);
      return true;
    } catch (error) {
      setOperationError(error instanceof Error ? `${error.message}。请重试。` : '页面结构调整失败，请重试。');
      return false;
    } finally {
      setStructureUpdating(false);
    }
  };

  // runStructureMutation drives the manual add/remove-section/subsection calls:
  // they all return the authoritative {slides, spec} snapshot, applied atomically.
  const runStructureMutation = async (
    mutate: (projectId: string) => Promise<ProjectContentSnapshot>,
    failureText: string,
  ): Promise<boolean> => {
    const projectId = activeProjectId;
    if (!projectId || runActive || structureUpdating) return false;
    setOperationError('');
    setStructureUpdating(true);
    try {
      const snapshot = await mutate(projectId);
      applyProjectContentSnapshot(projectId, snapshot);
      return true;
    } catch (error) {
      setOperationError(error instanceof Error ? `${error.message}。请重试。` : failureText);
      return false;
    } finally {
      setStructureUpdating(false);
    }
  };

  const handleAddSection = () =>
    void runStructureMutation((projectId) => slidesApi.addSection(projectId), '新增章节失败，请重试。');

  const handleAddSubsection = (sectionId: string) =>
    void runStructureMutation((projectId) => slidesApi.addSubsection(projectId, sectionId), '新增子节失败，请重试。');

  const confirmRename = async (title: string) => {
    const target = renameTarget;
    if (!target) return;
    const ok = await runStructureMutation(
      (projectId) => {
        if (target.kind === 'section') {
          return slidesApi.renameSection(projectId, target.sectionId, title);
        }
        if (target.kind === 'subsection') {
          return slidesApi.renameSubsection(projectId, target.sectionId, target.subsectionId, title);
        }
        return slidesApi.renameSlide(projectId, target.slideId, title);
      },
      '重命名失败，请重试。',
    );
    if (!ok) throw new Error('重命名失败');
  };

  const confirmStructureDelete = async () => {
    if (!structureToDelete) return;
    const target = structureToDelete;
    const ok = await runStructureMutation(
      (projectId) => target.kind === 'section'
        ? slidesApi.removeSection(projectId, target.sectionId)
        : slidesApi.removeSubsection(projectId, target.sectionId, target.subsectionId),
      target.kind === 'section' ? '删除章节失败，请重试。' : '删除子节失败，请重试。',
    );
    if (!ok) throw new Error(operationError || '删除失败');
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
    const projectId = activeProjectId;
    if (!projectId || structureUpdating) return;
    const groups = directoryGroups.map((group) => ({
      ...group,
      entries: group.entries.map((candidate) => ({
        ...candidate,
        placement: { ...candidate.placement },
      })),
    }));
    const groupIndex = groups.findIndex((group) =>
      group.entries.some((candidate) => candidate.slide.id === entry.slide.id));
    if (groupIndex < 0) return;
    const currentGroup = groups[groupIndex];
    const entryIndex = currentGroup.entries.findIndex((candidate) => candidate.slide.id === entry.slide.id);
    const adjacentIndex = entryIndex + direction;

    if (adjacentIndex >= 0 && adjacentIndex < currentGroup.entries.length) {
      [currentGroup.entries[entryIndex], currentGroup.entries[adjacentIndex]] =
        [currentGroup.entries[adjacentIndex], currentGroup.entries[entryIndex]];
    } else {
      const targetGroup = groups[groupIndex + direction];
      if (!targetGroup) return;
      const [source] = currentGroup.entries.splice(entryIndex, 1);
      const moved: DirectoryEntry = {
        ...source,
        placement: {
          slide_id: source.slide.id,
          ...targetGroup.placement,
        },
      };
      if (direction > 0) targetGroup.entries.unshift(moved);
      else targetGroup.entries.push(moved);
    }

    void submitStructure(groups.flatMap((group) => group.entries));
  };

  const handleGroupDrop = (event: React.DragEvent, targetPlacement: Omit<SlidePlacement, 'slide_id'>) => {
    event.preventDefault();
    const sourceSlideId = event.dataTransfer.getData('text/plain');
    setDragSlideId(null);
    if (!sourceSlideId || !activeProjectId || runActive || structureUpdating) return;
    moveEntry(sourceSlideId, targetPlacement);
  };

  const toggleSection = (sectionId: string) => {
    setExpandedSectionIds((current) => {
      const next = new Set(current);
      if (next.has(sectionId)) next.delete(sectionId);
      else next.add(sectionId);
      return next;
    });
  };

  const handleKeyboardNavigation = () => {
    lastInteractionWasKeyboard.current = true;
  };

  const handlePointerInteraction = () => {
    lastInteractionWasKeyboard.current = false;
    setKeyboardFocusTarget(null);
  };

  const handleDirectoryFocus = (target: string) => {
    if (lastInteractionWasKeyboard.current) setKeyboardFocusTarget(target);
  };

  const handleDirectoryBlur = (event: React.FocusEvent<HTMLElement>, target: string) => {
    if (!event.relatedTarget || !event.currentTarget.contains(event.relatedTarget as Node)) {
      setKeyboardFocusTarget((current) => current === target ? null : current);
    }
  };

  const renderSlideRow = (entry: DirectoryEntry, renderedIndex: number) => {
    const { slide } = entry;
    const spec = specView?.slide_specs?.[slide.id];
    const slideTitle = slide.title || spec?.title || '未命名';
    const groupIndex = directoryGroups.findIndex((group) =>
      group.entries.some((candidate) => candidate.slide.id === slide.id));
    const groupEntryIndex = groupIndex >= 0
      ? directoryGroups[groupIndex].entries.findIndex((candidate) => candidate.slide.id === slide.id)
      : -1;
    const canMoveUp = groupIndex >= 0 && (groupEntryIndex > 0 || groupIndex > 0);
    const canMoveDown = groupIndex >= 0 && (
      groupEntryIndex < directoryGroups[groupIndex].entries.length - 1
      || groupIndex < directoryGroups.length - 1
    );
    const keyboardFocused = keyboardFocusTarget === `slide:${slide.id}`;
    return (
      <div
        key={slide.id}
        draggable={!runActive && !structureUpdating}
        onDragStart={(event) => handleDragStart(event, slide.id)}
        onDragOver={handleDragOver}
        onDrop={(event) => handleRowDrop(event, entry)}
        onDragEnd={() => setDragSlideId(null)}
        onFocusCapture={() => handleDirectoryFocus(`slide:${slide.id}`)}
        onBlurCapture={(event) => handleDirectoryBlur(event, `slide:${slide.id}`)}
        className={cn(
          "group relative grid min-h-[60px] w-full grid-cols-[32px_minmax(0,1fr)] items-center gap-2 rounded-md px-2 py-2 text-sm transition-colors cursor-pointer focus-within:bg-panel-muted",
          currentSlideId === slide.id
            ? "bg-accent/5"
            : "text-text-600 hover:bg-black/5",
          dragSlideId === slide.id && "opacity-50"
        )}
        onClick={() => setCurrentSlideId(slide.id)}
      >
        {currentSlideId === slide.id && (
          <span aria-hidden="true" className="absolute inset-y-1.5 left-0 w-[3px] rounded-full bg-accent" />
        )}
        <span className="text-left text-[16px] font-bold tracking-[0.02em] tabular-nums text-text-900">{String(renderedIndex + 1).padStart(2, '0')}</span>
        <SlideDirectoryContent
          slide={slide}
          index={renderedIndex}
          title={slideTitle}
          view={globalView}
          state={getRenderState(slide)}
        />
        {!runActive && (
          <div className={cn(
            "pointer-events-none absolute right-1 top-1/2 z-10 -translate-y-1/2 bg-gradient-to-r from-transparent via-panel to-panel pl-5 opacity-0 transition-opacity group-hover:pointer-events-auto group-hover:opacity-100",
            keyboardFocused && "pointer-events-auto opacity-100",
          )}>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <IconButton
                  label="页面操作"
                  className="h-7 w-7"
                  disabled={structureUpdating}
                >
                  <MoreHorizontal className="h-4 w-4" />
                </IconButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="min-w-[132px]">
                <DropdownMenuItem disabled={structureUpdating} onSelect={() => setRenameTarget({ kind: 'slide', slideId: slide.id, title: slideTitle })}>
                  重命名
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem disabled={structureUpdating || !canMoveUp} onSelect={() => moveEntryByDirection(entry, -1)}>
                  上移本页
                </DropdownMenuItem>
                <DropdownMenuItem disabled={structureUpdating || !canMoveDown} onSelect={() => moveEntryByDirection(entry, 1)}>
                  下移本页
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem destructive disabled={structureUpdating} onSelect={() => handleDelete(slide.id, slideTitle)}>
                  删除本页
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        )}
      </div>
    );
  };

  // renderSubsectionHeading draws a subsection label plus a hover action menu.
  // ownsPage + subsectionCount decide the confirm copy: a lone subsection that
  // owns pages drops them to direct; deleting one among several requires it empty.
  const renderSubsectionHeading = (
    section: DirectorySection,
    subsection: DirectorySubsection,
    keySuffix: string,
  ) => {
    const ownsPage = subsection.slides.length > 0;
    const fallsBackToDirect = ownsPage && section.subsections.length === 1;
    const keyboardFocused = keyboardFocusTarget === `subsection:${subsection.id}`;
    return (
      <div
        key={`${section.id}:${subsection.id}:${keySuffix}`}
        onDragOver={handleDragOver}
        onDrop={(event) => handleGroupDrop(event, { section_id: section.id, subsection_id: subsection.id })}
        onFocusCapture={() => handleDirectoryFocus(`subsection:${subsection.id}`)}
        onBlurCapture={(event) => handleDirectoryBlur(event, `subsection:${subsection.id}`)}
        className="group/sub relative grid min-h-7 grid-cols-[32px_minmax(0,1fr)_20px] items-center gap-2 px-2 pt-2 pb-0.5 text-[11px]"
      >
        <span className="text-left tabular-nums text-text-400">{formatDirectoryNumber(subsection.number, 'subsection')}</span>
        <span className="min-w-0 truncate font-medium text-text-400">{subsection.title}</span>
        {!runActive && (
          <div className={cn(
            "pointer-events-none flex h-5 w-5 items-center justify-center justify-self-end opacity-0 transition-opacity group-hover/sub:pointer-events-auto group-hover/sub:opacity-100",
            keyboardFocused && "pointer-events-auto opacity-100",
          )}>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <IconButton
                  label="子节操作"
                  className="h-5 w-5"
                  disabled={structureUpdating}
                >
                  <MoreHorizontal className="h-3.5 w-3.5" />
                </IconButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="min-w-[132px]">
                <DropdownMenuItem
                  disabled={structureUpdating}
                  onSelect={() => setRenameTarget({
                    kind: 'subsection',
                    sectionId: section.id,
                    subsectionId: subsection.id,
                    title: subsection.title,
                  })}
                >
                  重命名
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  destructive
                  disabled={structureUpdating}
                  onSelect={() => setStructureToDelete({
                    kind: 'subsection',
                    sectionId: section.id,
                    subsectionId: subsection.id,
                    title: subsection.title,
                    fallsBackToDirect,
                  })}
                >
                  删除子节
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        )}
      </div>
    );
  };

  return (
    <div
      className="flex flex-col h-full bg-panel"
      onKeyDownCapture={handleKeyboardNavigation}
      onPointerDownCapture={handlePointerInteraction}
    >
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
          <div className="flex-1 overflow-y-auto p-2">
            {slides.length === 0 ? (
              <div className="text-center p-4 text-text-400 text-sm">暂无页面</div>
            ) : directorySections.length === 0 ? (
              <div className="text-center p-4 text-text-400 text-sm">目录加载中...</div>
            ) : (
              (() => {
                let renderedIndex = 0;
                return directorySections.map((section) => {
                  const expanded = expandedSectionIds.has(section.id);
                  const keyboardFocused = keyboardFocusTarget === `section:${section.id}`;
                  return (
                  <section
                    key={section.id}
                    className="mb-1.5"
                    onFocusCapture={() => handleDirectoryFocus(`section:${section.id}`)}
                    onBlurCapture={(event) => handleDirectoryBlur(event, `section:${section.id}`)}
                  >
                    <div className="group/section relative">
                      <button
                        type="button"
                        aria-expanded={expanded}
                        aria-label={`${expanded ? '收起' : '展开'}第 ${formatDirectoryNumber(section.number, 'section')} 章 ${section.title}`}
                        onClick={() => toggleSection(section.id)}
                        onDragOver={handleDragOver}
                        onDrop={(event) => handleGroupDrop(event, { section_id: section.id })}
                        className="grid min-h-10 w-full grid-cols-[32px_minmax(0,1fr)_28px] items-center gap-2 rounded-lg px-2 py-1.5 text-left text-text-900 hover:bg-black/[0.04]"
                      >
                        <span className="relative flex h-4 items-center text-left text-[13px] font-semibold tabular-nums text-text-600">
                          <span className={cn(
                            "transition-opacity group-hover/section:opacity-0",
                            keyboardFocused && "opacity-0",
                          )}>
                            {formatDirectoryNumber(section.number, 'section')}
                          </span>
                          <ChevronRight
                            aria-hidden="true"
                            className={cn(
                              "absolute h-4 w-4 text-text-600 opacity-0 transition-[opacity,transform] duration-150 motion-reduce:transition-none group-hover/section:opacity-100",
                              keyboardFocused && "opacity-100",
                              expanded && "rotate-90",
                            )}
                          />
                        </span>
                        <span className="min-w-0 truncate text-[13px] font-semibold" title={section.title}>
                          {section.title}
                        </span>
                        <span aria-hidden="true" />
                      </button>
                      {!runActive && (
                        <div className={cn(
                          "pointer-events-none absolute right-2 top-1/2 z-10 -translate-y-1/2 opacity-0 transition-opacity group-hover/section:pointer-events-auto group-hover/section:opacity-100",
                          keyboardFocused && "pointer-events-auto opacity-100",
                        )}>
                          <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                              <IconButton
                                label="章节操作"
                                className="h-7 w-7 bg-panel/95"
                                disabled={structureUpdating}
                              >
                                <MoreHorizontal className="h-4 w-4" />
                              </IconButton>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end" className="min-w-[132px]">
                              <DropdownMenuItem
                                disabled={structureUpdating}
                                onSelect={() => setRenameTarget({ kind: 'section', sectionId: section.id, title: section.title })}
                              >
                                重命名
                              </DropdownMenuItem>
                              <DropdownMenuItem disabled={structureUpdating} onSelect={() => handleAddSubsection(section.id)}>
                                新增子节
                              </DropdownMenuItem>
                              <DropdownMenuSeparator />
                              <DropdownMenuItem
                                destructive
                                disabled={structureUpdating}
                                onSelect={() => setStructureToDelete({ kind: 'section', sectionId: section.id, title: section.title })}
                              >
                                删除章节
                              </DropdownMenuItem>
                            </DropdownMenuContent>
                          </DropdownMenu>
                        </div>
                      )}
                    </div>
                    <div
                      aria-hidden={!expanded}
                      ref={(node) => node?.toggleAttribute('inert', !expanded)}
                      className={cn(
                        "grid transition-[grid-template-rows,opacity] duration-200 motion-reduce:transition-none",
                        expanded ? "grid-rows-[1fr] opacity-100" : "grid-rows-[0fr] opacity-0",
                      )}
                    >
                      <div className="relative min-h-0 overflow-hidden pb-1">
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
                          rows.push(renderSubsectionHeading(section, subsection, 'heading'));
                        }
                        rows.push(renderSlideRow(entry, renderedIndex));
                        renderedIndex += 1;
                      }
                      for (const subsection of section.subsections) {
                        if (renderedSubsections.has(subsection.id)) continue;
                        rows.push(renderSubsectionHeading(section, subsection, 'empty-heading'));
                      }
                      return rows;
                    })()}
                      </div>
                    </div>
                  </section>
                  );
                });
              })()
            )}
          </div>

          <div className="grid grid-cols-2 gap-1 border-t border-border p-2">
            <button
              onClick={() => void handleAdd()}
              disabled={runActive}
              title={runActive ? 'AI 运行中，暂不可编辑结构' : '在末尾加一页'}
              className="flex items-center justify-center px-3 py-2 rounded-md text-sm text-text-600 hover:bg-black/5 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              <Plus className="w-4 h-4 mr-1" /> 新增页面
            </button>
            <button
              onClick={handleAddSection}
              disabled={runActive || structureUpdating}
              title={runActive ? 'AI 运行中，暂不可编辑结构' : '在末尾新增章节'}
              className="flex items-center justify-center px-3 py-2 rounded-md text-sm text-text-600 hover:bg-black/5 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              <Plus className="w-4 h-4 mr-1" /> 新增章节
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

          <ConfirmModal
            open={!!structureToDelete}
            onOpenChange={(open) => !open && setStructureToDelete(null)}
            title={structureToDelete?.kind === 'section' ? '删除章节' : '删除子节'}
            description={
              structureToDelete?.kind === 'section'
                ? `确认删除章节「${structureToDelete.title || '未命名'}」吗？此操作不可撤销。`
                : structureToDelete?.fallsBackToDirect
                  ? `确认删除子节「${structureToDelete?.title || '未命名'}」吗？其页面将回落为本章直属页。`
                  : `确认删除子节「${structureToDelete?.title || '未命名'}」吗？此操作不可撤销。`
            }
            variant="danger"
            confirmLabel="删除"
            onConfirm={confirmStructureDelete}
          />

          <FormModal<string>
            open={!!renameTarget}
            onOpenChange={(open) => !open && setRenameTarget(null)}
            title={renameTarget?.kind === 'section'
              ? '重命名章节'
              : renameTarget?.kind === 'subsection'
                ? '重命名子节'
                : '重命名页面'}
            initialValue={renameTarget?.title ?? ''}
            validate={(value) => {
              const normalized = value.trim();
              if (!normalized) return '名称不能为空';
              if ([...normalized].length > 60) return '名称不能超过 60 个字符';
              return null;
            }}
            confirmLabel="保存"
            onSubmit={(value) => confirmRename(value.trim())}
            renderField={(value, setValue, error) => (
              <div>
                <input
                  autoFocus
                  aria-label="新名称"
                  type="text"
                  value={value}
                  onChange={(event) => setValue(event.target.value)}
                  className="w-full rounded-md border border-border bg-panel px-3 py-2 text-sm transition-colors focus:border-accent"
                  placeholder="请输入新名称"
                />
                {error && <p className="mt-2 text-xs text-danger">{error}</p>}
              </div>
            )}
          />
        </>
      )}
    </div>
  );
};
