import {
  forwardRef,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ComponentPropsWithoutRef,
  type ReactNode,
} from 'react';
import {
  ArrowDown,
  ArrowUp,
  ChevronDown,
  ChevronRight,
  Ellipsis,
  FilePlus2,
  FolderPlus,
  GripVertical,
  List,
  NotebookPen,
  Paintbrush,
  PanelLeftClose,
  Pencil,
  Plus,
  Trash2,
} from 'lucide-react';
import type {
  MutationPosition,
  OutlineSection,
  OutlineSlideNode,
  PPTMutation,
  SlideRole,
  ProjectContentSnapshot,
  Slide,
} from '../../api/types';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from '../../components/ui/dropdown-menu';
import { ConfirmModal } from '../../components/ui/modal-confirm';
import { FormModal } from '../../components/ui/modal-form';
import { IconButton } from '../../components/ui/primitives';
import { cn } from '../../lib/utils';
import { useDeckStore } from '../../stores/deckStore';
import { useProjectStore } from '../../stores/projectStore';
import { useUIStore } from '../../stores/uiStore';
import { useActiveSession } from '../agent/useActiveSession';
import { IsolatedSlidePreview } from '../viewer/IsolatedSlidePreview';
import { buildRuntimeFrame } from '../viewer/runtimeFrame';
import { Select } from '../../components/ui/select';
import { useAuthoringBlock } from '../viewer/useAuthoringBlock';
import { TextField } from '../viewer/ManagementEditor';
import { partLabel, slideRoleLabel } from '../viewer/semanticLabels';
import {
  hasRenderedHTML,
  type ResourceState,
  useSlideRenderCache,
} from '../viewer/useSlideRenderCache';
import { flattenOutline, orderedSlides } from './selectors';

const clientRef = (kind: string) => `${kind}-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;

type EditTarget =
  | { kind: 'section'; id: string; value: string; purpose: string; hash?: string; scene?: number }
  | { kind: 'subsection'; id: string; value: string; purpose: string; hash?: string; scene?: number }
  | { kind: 'page'; id: string; value: string; role: SlideRole; hash?: string; scene?: number };

type OutlineRevision = { hash?: string; scene?: number };

type DeleteTarget = OutlineRevision & (
  | { kind: 'section'; id: string; title: string }
  | { kind: 'subsection'; id: string; title: string; promote: boolean }
  | { kind: 'page'; id: string; title: string });

type NewSubsectionTarget = OutlineRevision & { section: OutlineSection };
type NewSubsectionForm = { title: string; purpose: string };

const newSubsectionInitialValue: NewSubsectionForm = {
  title: '新子节',
  purpose: '',
};

const OverflowTrigger = forwardRef<
  HTMLButtonElement,
  ComponentPropsWithoutRef<'button'> & { label: string }
>(({ label, className, onClick, ...props }, ref) => (
  <button
    ref={ref}
    type="button"
    aria-label={label}
    title="更多操作"
    className={cn(
      'grid h-7 w-7 place-items-center rounded-md text-text-400 opacity-0 transition-[opacity,background-color,color] hover:bg-surface hover:text-text-900 focus-visible:opacity-100 disabled:pointer-events-none disabled:opacity-30 group-hover/section:opacity-100 group-hover/subsection:opacity-100 group-hover/page:opacity-100 data-[state=open]:bg-surface data-[state=open]:text-text-900 data-[state=open]:opacity-100',
      className,
    )}
    onClick={(event) => {
      event.stopPropagation();
      onClick?.(event);
    }}
    {...props}
  >
    <Ellipsis className="h-4 w-4" strokeWidth={1.8} />
  </button>
));
OverflowTrigger.displayName = 'OverflowTrigger';

function MenuIcon({ children }: { children: ReactNode }) {
  return <span className="mr-2 inline-flex h-4 w-4 items-center justify-center text-text-400">{children}</span>;
}

function DeckNavigatorChrome({
  pageCount,
  sectionCount,
  onInsertSection,
  insertDisabled,
}: {
  pageCount: number;
  sectionCount: number;
  onInsertSection: () => void;
  insertDisabled: boolean;
}) {
  const toggleLeftPanel = useUIStore((state) => state.toggleLeftPanel);
  const activeDocument = useDeckStore((state) => state.activeDocument);
  const setActiveDocument = useDeckStore((state) => state.setActiveDocument);

  return (
    <>
      <header
        data-testid="deck-navigator-title-row"
        className="flex h-12 shrink-0 items-center justify-between border-b border-border bg-panel px-3"
      >
        <div className="flex min-w-0 items-center text-sm font-semibold text-text-900">
          <List className="mr-2 h-4 w-4 shrink-0 text-accent" strokeWidth={1.75} aria-hidden="true" />
          <h2 className="truncate">目录</h2>
        </div>
        <div className="ml-2 flex shrink-0 items-center gap-1">
          <IconButton label="隐藏左侧目录" onClick={toggleLeftPanel}>
            <PanelLeftClose className="h-4 w-4" strokeWidth={1.75} />
          </IconButton>
        </div>
      </header>

      <div
        data-testid="deck-navigator-summary-row"
        className="flex h-9 shrink-0 items-center border-b border-border bg-surface px-3"
      >
        <span className="shrink-0 whitespace-nowrap text-xs tabular-nums text-text-400">
          {sectionCount} 章 · {pageCount} 页
        </span>
        <button
          type="button"
          onClick={onInsertSection}
          disabled={insertDisabled}
          aria-label="新增章节"
          title="新增章节"
          className="ml-1.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-text-600 transition-colors hover:bg-black/5 hover:text-text-900 disabled:cursor-not-allowed disabled:opacity-40"
        >
          <FolderPlus className="h-4 w-4" strokeWidth={1.75} />
        </button>
      </div>
      <nav aria-label="项目文档" className="shrink-0 space-y-0.5 border-b border-border px-2 py-2">
        {([{ kind: 'manifest', icon: NotebookPen }, { kind: 'design', icon: Paintbrush }] as const).map(({ kind, icon: Icon }) => (
          <button
            key={kind}
            type="button"
            aria-current={activeDocument === kind ? 'page' : undefined}
            onClick={() => setActiveDocument(kind)}
            className={cn(
              'flex h-9 w-full items-center gap-2.5 rounded-md px-3 text-left text-sm transition-colors focus-visible:bg-accent-soft focus-visible:text-accent',
              activeDocument === kind ? 'bg-accent-soft font-medium text-accent' : 'text-text-600 hover:bg-panel-muted hover:text-text-900',
            )}
          >
            <Icon className="h-4 w-4 shrink-0" strokeWidth={1.75} aria-hidden="true" />
            {partLabel(kind)}
          </button>
        ))}
      </nav>
    </>
  );
}

function SlideThumbnail({
  slide,
  snapshot,
  state,
  load,
}: {
  slide: Slide;
  snapshot: ProjectContentSnapshot;
  state: ResourceState<string>;
  load: () => void;
}) {
  const ref = useRef<HTMLDivElement>(null);
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
    }, { rootMargin: '120px' });
    observer.observe(node);
    return () => observer.disconnect();
  }, [slide]);

  const html = state.status === 'ready'
    ? state.data
    : state.status === 'loading' || state.status === 'error'
      ? state.previous
      : undefined;
  const frame = buildRuntimeFrame(snapshot, slide.id);

  return (
    <div
      ref={ref}
      className="relative aspect-video min-w-0 max-w-36 overflow-hidden rounded border border-border-strong bg-surface shadow-sm"
    >
      {html && frame ? (
        <IsolatedSlidePreview
          slides={[{ id: slide.id, html, frame }]}
          index={0}
          className="pointer-events-none h-[400%] w-[400%] origin-top-left scale-[0.25] border-0 bg-white"
          title={`${slide.title || '页面'}缩略图`}
        />
      ) : (
        <div className="flex h-full items-center justify-center bg-panel-muted text-[10px] text-text-400">
          {hasRenderedHTML(slide) ? '加载中' : '未生成'}
        </div>
      )}
      {state.status === 'loading' && html && (
        <span className="absolute right-1 top-1 h-1.5 w-1.5 animate-pulse rounded-full bg-accent" />
      )}
    </div>
  );
}

interface SlideRowProps {
  node: OutlineSlideNode;
  slide: Slide;
  snapshot: ProjectContentSnapshot;
  ordinal: number;
  siblingIndex: number;
  siblingCount: number;
  selected: boolean;
  locked: boolean;
  pending: boolean;
  view: 'outline' | 'html';
  state: ResourceState<string>;
  load: () => void;
  onSelect: () => void;
  onRename: () => void;
  onRemove: () => void;
  onMove: (delta: number) => void;
  onDrag: () => void;
  onDrop: () => void;
}

function SlideRow({
  node,
  slide,
  snapshot,
  ordinal,
  siblingIndex,
  siblingCount,
  selected,
  locked,
  pending,
  view,
  state,
  load,
  onSelect,
  onRename,
  onRemove,
  onMove,
  onDrag,
  onDrop,
}: SlideRowProps) {
  return (
    <div
      draggable={!locked}
      onDragStart={onDrag}
      onDragOver={(event) => event.preventDefault()}
      onDrop={onDrop}
      onClick={onSelect}
      role="button"
      aria-current={selected ? 'page' : undefined}
      className={cn(
        'group/page relative mx-1 my-px grid cursor-pointer grid-cols-[20px_42px_minmax(0,1fr)_28px] items-center gap-0.5 rounded-md border border-transparent px-1 transition-colors',
        view === 'outline' ? 'min-h-[58px] py-1' : 'min-h-[76px] py-1.5',
        selected ? 'border-accent/15 bg-accent-soft' : 'hover:bg-panel-muted',
      )}
    >
      <span className={cn(
        'absolute bottom-2 left-0 top-2 w-0.5 rounded-r',
        selected ? 'bg-accent' : 'bg-transparent',
      )} />
      <GripVertical
        className="h-3.5 w-3.5 text-text-400 opacity-0 transition-opacity group-hover/page:opacity-100"
        strokeWidth={1.7}
      />
      <span className={cn(
        'text-center font-mono text-xl font-[760] leading-none tabular-nums',
        selected ? 'text-accent' : 'text-text-700',
      )}>
        {String(ordinal).padStart(2, '0')}
      </span>
      {view === 'outline' ? (
        <span className="flex min-w-0 items-center gap-2 pl-0.5">
          <span className="truncate text-sm font-semibold leading-none text-text-900">{node.title || '未命名页面'}</span>
          {pending && <span className="h-1.5 w-1.5 shrink-0 animate-pulse rounded-full bg-warning" title="等待生成设计稿" />}
        </span>
      ) : (
        <SlideThumbnail slide={slide} snapshot={snapshot} state={state} load={load} />
      )}
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <OverflowTrigger label={`${node.title || '页面'}操作`} disabled={locked} />
        </DropdownMenuTrigger>
        <DropdownMenuContent side="right" align="start" sideOffset={6}>
          <DropdownMenuItem onSelect={onRename}>
            <MenuIcon><Pencil className="h-3.5 w-3.5" /></MenuIcon>编辑属性
          </DropdownMenuItem>
          <DropdownMenuItem disabled={siblingIndex === 0} onSelect={() => onMove(-1)}>
            <MenuIcon><ArrowUp className="h-3.5 w-3.5" /></MenuIcon>上移本页
          </DropdownMenuItem>
          <DropdownMenuItem disabled={siblingIndex === siblingCount - 1} onSelect={() => onMove(1)}>
            <MenuIcon><ArrowDown className="h-3.5 w-3.5" /></MenuIcon>下移本页
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem destructive onSelect={onRemove}>
            <MenuIcon><Trash2 className="h-3.5 w-3.5" /></MenuIcon>删除本页
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}

export function DeckNavigator() {
  const activeProjectId = useProjectStore((state) => state.activeProjectId);
  const snapshot = useProjectStore((state) => activeProjectId ? state.contentByProjectId[activeProjectId] : undefined);
  const mutateProject = useProjectStore((state) => state.mutateProject);
  const pendingMutation = useProjectStore((state) => activeProjectId ? state.mutationPendingByProjectId[activeProjectId] : false);
  const currentSlideId = useDeckStore((state) => state.currentSlideId);
  const activeDocument = useDeckStore((state) => state.activeDocument);
  const setCurrentSlideId = useDeckStore((state) => state.setCurrentSlideId);
  const globalView = useDeckStore((state) => state.globalView);
  const { status } = useActiveSession();
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  const [draggedSlideId, setDraggedSlideId] = useState<string>();
  const [editTarget, setEditTarget] = useState<EditTarget | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null);
  const [newSubsectionTarget, setNewSubsectionTarget] = useState<NewSubsectionTarget | null>(null);
  const editInitialValue = useMemo(() => ({ title: editTarget?.value ?? '', purpose: editTarget && editTarget.kind !== 'page' ? editTarget.purpose : '', role: editTarget?.kind === 'page' ? editTarget.role : 'content' as SlideRole }), [editTarget]);
  const slides = useMemo(() => orderedSlides(snapshot), [snapshot]);
  const flat = useMemo(() => flattenOutline(snapshot?.outline), [snapshot?.outline]);
  const ordinalById = useMemo(() => Object.fromEntries(flat.map((item) => [item.node.slide_id, item.ordinal])), [flat]);
  const slideById = useMemo(() => Object.fromEntries(slides.map((slide) => [slide.id, slide])), [slides]);
  const { getState, load } = useSlideRenderCache(activeProjectId);
  const runLocked = ['creating', 'running', 'waiting', 'paused', 'recovering', 'canceling'].includes(status);
  const managementBlocked = useAuthoringBlock(activeProjectId);
  const locked = pendingMutation || runLocked || Boolean(managementBlocked);
  const outlineHash = snapshot?.hashes.outline;

  const commitMutation = async (request: PPTMutation) => {
    if (!activeProjectId || locked) throw new Error(managementBlocked ?? '项目正忙，请稍后重试。');
    await mutateProject(activeProjectId, { expected_scene_revision: snapshot?.scene_revision, ...request });
  };

  const mutate = async (request: PPTMutation) => {
    try {
      await commitMutation(request);
    } catch {
      // The API client reports non-Agent backend errors through the global toast layer.
    }
  };

  const positionForSibling = (parentId: string, siblings: string[], index: number): MutationPosition =>
    index < siblings.length ? { parent_id: parentId, before_id: siblings[index] } : { parent_id: parentId };

  const insertPage = (parentId: string) => mutate({
    op: 'outline.insert',
    expected_hash: outlineHash,
    node: { kind: 'slide', client_ref: clientRef('slide'), title: '新页面', role: 'content' },
    position: { parent_id: parentId },
  });

  const insertSection = () => mutate({
    op: 'outline.insert',
    expected_hash: outlineHash,
    node: {
      kind: 'section',
      client_ref: clientRef('section'),
      title: '新章节',
      purpose: '待补充章节目的',
      slides: [],
      subsections: [],
    },
    position: {},
  });

  const moveSlide = (slideId: string, parentId: string, siblings: OutlineSlideNode[], targetIndex: number) => {
    const without = siblings.filter((item) => item.slide_id !== slideId).map((item) => item.slide_id);
    const clamped = Math.max(0, Math.min(targetIndex, without.length));
    void mutate({
      op: 'outline.move',
      expected_hash: outlineHash,
      node_id: slideId,
      position: positionForSibling(parentId, without, clamped),
    });
  };

  const moveSibling = (id: string, parentId: string, siblings: string[], direction: number) => {
    const index = siblings.indexOf(id);
    const target = siblings[index + direction];
    if (!target) return;
    void mutate({ op: 'outline.move', expected_hash: outlineHash, node_id: id,
      position: { parent_id: parentId, ...(direction < 0 ? { before_id: target } : { after_id: target }) } });
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    if (deleteTarget.hash !== outlineHash || deleteTarget.scene !== snapshot?.scene_revision) throw new Error('目录已更新，请关闭后重新操作。');
    const request: PPTMutation = {
      op: 'outline.remove',
      expected_hash: deleteTarget.hash,
      expected_scene_revision: deleteTarget.scene,
      node_id: deleteTarget.id,
      ...(deleteTarget.kind === 'subsection' && deleteTarget.promote ? { child_policy: 'promote_to_section' as const } : {}),
    };
    const index = deleteTarget.kind === 'page' ? slides.findIndex((item) => item.id === deleteTarget.id) : -1;
    await commitMutation(request);
    if (deleteTarget.kind === 'page' && currentSlideId === deleteTarget.id) {
      const next = slides[index + 1] ?? slides[index - 1];
      useDeckStore.setState({ currentSlideId: next?.id ?? null });
    }
  };

  if (!snapshot) {
    return (
      <aside className="flex h-full min-h-0 flex-col border-r border-border-strong bg-panel" aria-label="演示目录">
        <DeckNavigatorChrome
          pageCount={0}
          sectionCount={0}
          onInsertSection={() => {}}
          insertDisabled
        />
        <div className="flex min-h-0 flex-1 items-center justify-center p-4 text-xs text-text-400">
          目录加载中
        </div>
      </aside>
    );
  }

  return (
    <>
      <aside className="flex h-full min-h-0 flex-col border-r border-border-strong bg-panel" aria-label="演示目录">
        <DeckNavigatorChrome
          pageCount={slides.length}
          sectionCount={snapshot.outline.sections.length}
          onInsertSection={() => void insertSection()}
          insertDisabled={locked}
        />

        <div
          data-testid="deck-navigator-scroll"
          className="deck-navigator-scroll scrollbar-none flex min-h-0 flex-1 flex-col overflow-y-auto px-2 pb-8 pt-2"
        >
          {snapshot.outline.sections.length === 0 ? (
            <div
              data-testid="deck-navigator-empty"
              className="flex min-h-32 flex-1 items-center justify-center text-center text-xs text-text-400"
            >
              内容为空
            </div>
          ) : (
            <>
              {snapshot.outline.sections.map((section, sectionIndex) => {
                const sectionCollapsed = collapsed[section.id];
                const isEmpty = section.slides.length === 0 && section.subsections.length === 0;
                return (
              <section
                key={section.id}
                className={cn('pb-2', sectionIndex > 0 && 'border-t border-border/80 pt-2')}
              >
                <div className="group/section mx-1 grid min-h-11 grid-cols-[20px_42px_minmax(0,1fr)_28px] items-center gap-0.5 rounded-md px-1 hover:bg-panel-muted">
                  <button
                    type="button"
                    className="grid h-6 w-5 place-items-center text-text-500"
                    aria-label={`${sectionCollapsed ? '展开' : '折叠'}${section.title}`}
                    onClick={() => setCollapsed((state) => ({ ...state, [section.id]: !sectionCollapsed }))}
                  >
                    {sectionCollapsed
                      ? <ChevronRight className="h-3.5 w-3.5" strokeWidth={1.8} />
                      : <ChevronDown className="h-3.5 w-3.5" strokeWidth={1.8} />}
                  </button>
                  <span className="whitespace-nowrap text-center text-[11px] font-semibold tabular-nums text-text-400">
                    第 <strong className="text-[13px] font-bold text-text-600">{sectionIndex + 1}</strong> 章
                  </span>
                  <button
                    type="button"
                    className="min-w-0 truncate pl-2 text-left text-sm font-[720] text-text-900"
                    onClick={() => setCollapsed((state) => ({ ...state, [section.id]: !sectionCollapsed }))}
                  >
                    {section.title}
                  </button>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <OverflowTrigger label={`${section.title}操作`} disabled={locked} />
                    </DropdownMenuTrigger>
                    <DropdownMenuContent side="right" align="start" sideOffset={6}>
                      <DropdownMenuItem disabled={sectionIndex === 0} onSelect={() => moveSibling(section.id, '', snapshot.outline.sections.map(item => item.id), -1)}><MenuIcon><ArrowUp className="h-3.5 w-3.5" /></MenuIcon>上移章节</DropdownMenuItem>
                      <DropdownMenuItem disabled={sectionIndex === snapshot.outline.sections.length - 1} onSelect={() => moveSibling(section.id, '', snapshot.outline.sections.map(item => item.id), 1)}><MenuIcon><ArrowDown className="h-3.5 w-3.5" /></MenuIcon>下移章节</DropdownMenuItem>
                      <DropdownMenuItem onSelect={() => setEditTarget({ kind: 'section', id: section.id, value: section.title, purpose: section.purpose, hash: outlineHash, scene: snapshot?.scene_revision })}>
                        <MenuIcon><Pencil className="h-3.5 w-3.5" /></MenuIcon>编辑属性
                      </DropdownMenuItem>
                      {section.subsections.length === 0 ? (
                        <DropdownMenuItem onSelect={() => void insertPage(section.id)}>
                          <MenuIcon><FilePlus2 className="h-3.5 w-3.5" /></MenuIcon>新增页面
                        </DropdownMenuItem>
                      ) : (
                        <DropdownMenuSub>
                          <DropdownMenuSubTrigger>
                            <MenuIcon><FilePlus2 className="h-3.5 w-3.5" /></MenuIcon>
                            <span className="flex-1">新增页面</span>
                            <ChevronRight className="ml-2 h-3.5 w-3.5 text-text-400" />
                          </DropdownMenuSubTrigger>
                          <DropdownMenuSubContent sideOffset={6}>
                            {section.subsections.map((subsection) => (
                              <DropdownMenuItem key={subsection.id} onSelect={() => void insertPage(subsection.id)}>
                                {subsection.title}
                              </DropdownMenuItem>
                            ))}
                          </DropdownMenuSubContent>
                        </DropdownMenuSub>
                      )}
                      <DropdownMenuItem onSelect={() => setNewSubsectionTarget({ section, hash: outlineHash, scene: snapshot?.scene_revision })}>
                        <MenuIcon><Plus className="h-3.5 w-3.5" /></MenuIcon>新增子节
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem
                        destructive
                        disabled={!isEmpty}
                        onSelect={() => setDeleteTarget({ hash: outlineHash, scene: snapshot?.scene_revision, kind: 'section', id: section.id, title: section.title })}
                      >
                        <MenuIcon><Trash2 className="h-3.5 w-3.5" /></MenuIcon>删除章节
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>

                {!sectionCollapsed && (
                  <div className="overflow-hidden">
                    {section.slides.map((node, index) => {
                      const slide = slideById[node.slide_id];
                      if (!slide) return null;
                      return (
                        <SlideRow
                          key={node.slide_id}
                          node={node}
                          slide={slide}
                          snapshot={snapshot}
                          ordinal={ordinalById[node.slide_id] ?? index + 1}
                          siblingIndex={index}
                          siblingCount={section.slides.length}
                          selected={!activeDocument && currentSlideId === node.slide_id}
                          locked={locked}
                          pending={!snapshot.slides_by_id[node.slide_id]?.spec}
                          view={globalView}
                          state={getState(slide)}
                          load={() => load(slide, 'prefetch')}
                          onSelect={() => setCurrentSlideId(node.slide_id)}
                          onRename={() => setEditTarget({ kind: 'page', id: node.slide_id, value: node.title, role: node.role, hash: outlineHash, scene: snapshot?.scene_revision })}
                          onRemove={() => setDeleteTarget({ hash: outlineHash, scene: snapshot?.scene_revision, kind: 'page', id: node.slide_id, title: node.title })}
                          onMove={(delta) => moveSlide(node.slide_id, section.id, section.slides, index + delta)}
                          onDrag={() => setDraggedSlideId(node.slide_id)}
                          onDrop={() => draggedSlideId && moveSlide(draggedSlideId, section.id, section.slides, index)}
                        />
                      );
                    })}

                    {section.subsections.map((subsection, subsectionIndex) => (
                      <div key={subsection.id}>
                        <div className="group/subsection mx-1 grid min-h-[34px] grid-cols-[20px_42px_minmax(0,1fr)_28px] items-center gap-0.5 px-1 text-text-400">
                          <span aria-hidden="true" />
                          <span className="whitespace-nowrap text-center text-[9px] font-semibold tabular-nums">
                            第 <strong className="text-[11px] font-[720] text-text-500">{sectionIndex + 1}.{subsectionIndex + 1}</strong> 节
                          </span>
                          <span className="truncate pl-2 text-[10px] font-semibold">{subsection.title}</span>
                          <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                              <OverflowTrigger label={`${subsection.title}操作`} disabled={locked} />
                            </DropdownMenuTrigger>
                            <DropdownMenuContent side="right" align="start" sideOffset={6}>
                              <DropdownMenuItem disabled={subsectionIndex === 0} onSelect={() => moveSibling(subsection.id, section.id, section.subsections.map(item => item.id), -1)}><MenuIcon><ArrowUp className="h-3.5 w-3.5" /></MenuIcon>上移子节</DropdownMenuItem>
                              <DropdownMenuItem disabled={subsectionIndex === section.subsections.length - 1} onSelect={() => moveSibling(subsection.id, section.id, section.subsections.map(item => item.id), 1)}><MenuIcon><ArrowDown className="h-3.5 w-3.5" /></MenuIcon>下移子节</DropdownMenuItem>
                              <DropdownMenuItem onSelect={() => setEditTarget({ kind: 'subsection', id: subsection.id, value: subsection.title, purpose: subsection.purpose, hash: outlineHash, scene: snapshot?.scene_revision })}>
                                <MenuIcon><Pencil className="h-3.5 w-3.5" /></MenuIcon>编辑属性
                              </DropdownMenuItem>
                              <DropdownMenuSeparator />
                              <DropdownMenuItem
                                destructive
                                disabled={subsection.slides.length > 0 && section.subsections.length > 1}
                                onSelect={() => setDeleteTarget({
                                  hash: outlineHash,
                                  scene: snapshot?.scene_revision,
                                  kind: 'subsection',
                                  id: subsection.id,
                                  title: subsection.title,
                                  promote: subsection.slides.length > 0,
                                })}
                              >
                                <MenuIcon><Trash2 className="h-3.5 w-3.5" /></MenuIcon>
                                {subsection.slides.length > 0 ? '取消子节' : '删除子节'}
                              </DropdownMenuItem>
                            </DropdownMenuContent>
                          </DropdownMenu>
                        </div>
                        {subsection.slides.map((node, index) => {
                          const slide = slideById[node.slide_id];
                          if (!slide) return null;
                          return (
                            <SlideRow
                              key={node.slide_id}
                              node={node}
                              slide={slide}
                              snapshot={snapshot}
                              ordinal={ordinalById[node.slide_id] ?? index + 1}
                              siblingIndex={index}
                              siblingCount={subsection.slides.length}
                              selected={!activeDocument && currentSlideId === node.slide_id}
                              locked={locked}
                              pending={!snapshot.slides_by_id[node.slide_id]?.spec}
                              view={globalView}
                              state={getState(slide)}
                              load={() => load(slide, 'prefetch')}
                              onSelect={() => setCurrentSlideId(node.slide_id)}
                              onRename={() => setEditTarget({ kind: 'page', id: node.slide_id, value: node.title, role: node.role, hash: outlineHash, scene: snapshot?.scene_revision })}
                              onRemove={() => setDeleteTarget({ hash: outlineHash, scene: snapshot?.scene_revision, kind: 'page', id: node.slide_id, title: node.title })}
                              onMove={(delta) => moveSlide(node.slide_id, subsection.id, subsection.slides, index + delta)}
                              onDrag={() => setDraggedSlideId(node.slide_id)}
                              onDrop={() => draggedSlideId && moveSlide(draggedSlideId, subsection.id, subsection.slides, index)}
                            />
                          );
                        })}
                      </div>
                    ))}
                  </div>
                )}
              </section>
                );
              })}
            </>
          )}
        </div>
      </aside>

      <FormModal<{ title: string; purpose: string; role: SlideRole }>
        open={editTarget !== null}
        onOpenChange={(open) => !open && setEditTarget(null)}
        title={`编辑${editTarget?.kind === 'section' ? '章节' : editTarget?.kind === 'subsection' ? '子节' : '页面'}`}
        initialValue={editInitialValue}
        validate={value => {
          if (!value.title.trim()) return '名称不能为空';
          if ([...value.title].length > 160) return '名称不能超过 160 个字符';
          if (editTarget?.kind !== 'page' && (!value.purpose.trim() || [...value.purpose].length > 400)) return '请填写目的，最多 400 个字符';
          return null;
        }}
        onSubmit={async value => {
          if (!editTarget) return;
          if (locked) throw new Error('项目正忙，请稍后再保存。');
          if (editTarget.hash !== outlineHash || editTarget.scene !== snapshot?.scene_revision) throw new Error('目录已更新，请关闭后重新编辑。');
          await commitMutation({ op: 'outline.update', expected_hash: editTarget.hash, expected_scene_revision: editTarget.scene, node_id: editTarget.id,
            changes: editTarget.kind === 'page' ? { title: value.title.trim(), role: value.role } : { title: value.title.trim(), purpose: value.purpose.trim() } });
        }}
        renderField={(value, setValue, error) => <div className="space-y-4">
          <TextField label="名称" value={value.title} minLength={1} maxLength={160} onChange={title => setValue({ ...value, title })} />
          {editTarget?.kind === 'page' ? <Select aria-label="页面角色" value={value.role} onValueChange={role => setValue({ ...value, role: role as SlideRole })}
            options={['cover', 'agenda', 'context', 'content', 'definition', 'evidence', 'comparison', 'example', 'how-to', 'transition', 'summary', 'conclusion'].map(role => ({ value: role, label: slideRoleLabel(role) }))} />
            : <TextField label="目的" value={value.purpose} minLength={1} maxLength={400} multiline onChange={purpose => setValue({ ...value, purpose })} />}
          {error && <p role="alert" className="text-xs text-danger">{error}</p>}
        </div>}
      />

      <FormModal<NewSubsectionForm>
        open={newSubsectionTarget !== null}
        onOpenChange={(open) => !open && setNewSubsectionTarget(null)}
        title="新增子节"
        initialValue={newSubsectionInitialValue}
        validate={(value) => {
          if (!value.title.trim()) return '名称不能为空';
          if ([...value.title.trim()].length > 160) return '名称不能超过 160 个字符';
          if (!value.purpose.trim()) return '目的不能为空';
          if ([...value.purpose.trim()].length > 400) return '目的不能超过 400 个字符';
          return null;
        }}
        onSubmit={async (value) => {
          if (!newSubsectionTarget) return;
          if (newSubsectionTarget.hash !== outlineHash || newSubsectionTarget.scene !== snapshot?.scene_revision) throw new Error('目录已更新，请关闭后重新操作。');
          const section = newSubsectionTarget.section;
          await commitMutation({
            op: 'outline.insert',
            expected_hash: newSubsectionTarget.hash,
            expected_scene_revision: newSubsectionTarget.scene,
            node: {
              kind: 'subsection',
              client_ref: clientRef('subsection'),
              title: value.title.trim(),
              purpose: value.purpose.trim(),
            },
            position: { parent_id: section.id },
            ...(section.slides.length > 0 ? { direct_slides_policy: 'move_into_new_subsection' as const } : {}),
          });
        }}
        renderField={(value, setValue, error) => (
          <div className="space-y-3">
            <label className="block">
              <span className="mb-1 block text-xs font-medium text-text-600">名称</span>
              <input
                autoFocus
                type="text"
                value={value.title}
                onChange={(event) => setValue({ ...value, title: event.target.value })}
                className="w-full rounded-md border border-border bg-panel px-3 py-2 text-sm text-text-900 transition-colors focus:outline-none"
              />
            </label>
            <label className="block">
              <span className="mb-1 block text-xs font-medium text-text-600">目的</span>
              <textarea
                rows={3}
                value={value.purpose}
                onChange={(event) => setValue({ ...value, purpose: event.target.value })}
                className="w-full resize-none rounded-md border border-border bg-panel px-3 py-2 text-sm text-text-900 transition-colors focus:outline-none"
              />
            </label>
            {newSubsectionTarget && newSubsectionTarget.section.slides.length > 0 && (
              <p className="mt-2 text-xs text-text-400">
                现有 {newSubsectionTarget.section.slides.length} 页将移入这个子节。
              </p>
            )}
            {error && <p className="mt-2 text-xs text-danger">{error}</p>}
          </div>
        )}
      />

      <ConfirmModal
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title={deleteTarget?.kind === 'subsection' && deleteTarget.promote ? `取消子节「${deleteTarget.title}」？` : `删除「${deleteTarget?.title ?? ''}」？`}
        description={deleteTarget?.kind === 'subsection' && deleteTarget.promote
          ? '子节中的页面会提升为章节直属页面，不会删除页面。'
          : '此操作不可撤销。'}
        confirmLabel={deleteTarget?.kind === 'subsection' && deleteTarget.promote ? '取消子节' : '删除'}
        variant="danger"
        onConfirm={confirmDelete}
      />
    </>
  );
}
