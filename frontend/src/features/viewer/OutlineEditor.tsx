import { useEffect, useLayoutEffect, useRef, useState, type DragEvent, type ReactNode } from 'react';
import { ArrowDown, ArrowUp, Check, ChevronDown, ChevronRight, ChevronsDown, ChevronsUp, Ellipsis, FilePlus2, FileText, Folder, FolderInput, FolderOpen, FolderPlus, GripVertical, List, Pencil, Plus, Trash2, X } from 'lucide-react';
import type { Outline, OutlineSection, OutlineSlideNode, OutlineSubsection } from '../../api/types';
import { Button, IconButton } from '../../components/ui/primitives';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator, DropdownMenuSub, DropdownMenuSubContent, DropdownMenuSubTrigger, DropdownMenuTrigger } from '../../components/ui/dropdown-menu';
import { ConfirmModal } from '../../components/ui/modal-confirm';
import { useDeckStore } from '../../stores/deckStore';
import { DocumentCanvas } from './DocumentCanvas';
import { JSONPreview } from './JSONPreview';
import { applyOutlineCommand, findOutlineNode, newOutlineNode, outlineDestinations, type OutlineCommand, type OutlineKind, type OutlineNode } from './outlineEditing';
import './outline.css';

type Field = 'title' | 'purpose';
type Edit = { id: string; kind: OutlineKind; title: string; purpose: string; field: Field; version: string; base: Outline; addition?: Extract<OutlineCommand, { type: 'insert' }> };
const labels = { section: '章节', subsection: '小节', slide: '页面' };

export function OutlineEditor({ value, version, blocked, notice, commit, saveDraft, restoreDraft, canRestore = false, draftMode = false, onEditingChange, onDraftSaved, openSlide }: {
  value: Outline; version: string; blocked?: string; notice?: ReactNode;
  commit: (command: OutlineCommand, version: string) => Promise<void>;
  saveDraft?: (next: Outline, version: string) => Promise<number>;
  restoreDraft?: () => Promise<void>; canRestore?: boolean; draftMode?: boolean;
  onEditingChange?: (editing: boolean, busy: boolean) => void;
  onDraftSaved?: (revision: number) => void;
  openSlide?: (id: string) => void;
}) {
  const [edit, setEdit] = useState<Edit>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [reviewRevision, setReviewRevision] = useState<number>();
  const [collapsed, setCollapsed] = useState<Set<string>>(() => new Set());
  const [deleting, setDeleting] = useState<{ id: string; version: string }>();
  const [dropTarget, setDropTarget] = useState<string>();
  const pending = useRef(false);
  const drag = useRef<{ id: string; version: string }>();
  const root = useRef<HTMLDivElement>(null);
  const input = useRef<HTMLInputElement>(null);
  const showJSON = useDeckStore(state => state.contentMode === 'source');
  const isEditing = Boolean(edit);
  const editingId = edit?.id;
  const editingField = edit?.field;
  const creating = Boolean(edit?.addition);
  const stale = edit && edit.version !== version;
  const unavailable = blocked ?? (stale ? '目录已更新，请取消后重新编辑。' : undefined);
  const locked = Boolean(blocked) || busy || Boolean(edit);
  const displayed = edit ? edit.addition ? applyOutlineCommand(edit.base, edit.addition) : edit.base : value;
  const pages = displayed.sections.flatMap(section => [...section.slides, ...section.subsections.flatMap(sub => sub.slides)]);

  useEffect(() => {
    useDeckStore.getState().setSourceBlocked(isEditing || busy);
    return () => useDeckStore.getState().setSourceBlocked(false);
  }, [isEditing, busy]);
  useEffect(() => {
    onEditingChange?.(isEditing, busy);
    if (!isEditing && !busy && reviewRevision !== undefined) {
      onDraftSaved?.(reviewRevision);
      setReviewRevision(undefined);
    }
  }, [isEditing, busy, onEditingChange, onDraftSaved, reviewRevision]);
  useLayoutEffect(() => {
    if (!editingId) return;
    input.current?.focus({ preventScroll: true });
    input.current?.select();
    if (creating) input.current?.closest('.outline-row')?.scrollIntoView({ block: 'nearest' });
  }, [editingId, editingField, creating]);

  function focusRow(id?: string) {
    requestAnimationFrame(() => {
      const row = id ? root.current?.querySelector<HTMLElement>(`[data-node-id="${id}"] .outline-title`) : undefined;
      (row ?? root.current?.querySelector<HTMLElement>('.outline-add-section'))?.focus({ preventScroll: true });
    });
  }
  function cancel() {
    if (pending.current) return;
    const id = edit?.addition ? edit.addition.position.parent_id : edit?.id;
    setEdit(undefined); setError(''); setMessage(''); focusRow(id);
  }
  function start(id: string, field: Field = 'title') {
    if (locked || pending.current) return;
    const item = findOutlineNode(value, id);
    if (!item) return;
    setError(''); setMessage('');
    setEdit({ id, kind: item.kind, title: item.node.title, purpose: 'purpose' in item.node ? item.node.purpose : '', field, version, base: structuredClone(value) });
  }
  function add(kind: OutlineKind, parentId?: string) {
    if (locked || pending.current) return;
    const node = newOutlineNode(kind);
    const addition: Extract<OutlineCommand, { type: 'insert' }> = { type: 'insert', kind, node, position: parentId ? { parent_id: parentId } : {} };
    try { applyOutlineCommand(value, addition); } catch (cause) { setError(cause instanceof Error ? cause.message : '新增失败'); return; }
    if (parentId) setCollapsed(current => { const next = new Set(current); next.delete(parentId); return next; });
    setError(''); setMessage('');
    setEdit({ id: node.id, kind, title: node.title, purpose: 'purpose' in node ? node.purpose : '', field: 'title', version, base: structuredClone(value), addition });
  }
  function editCommand(): OutlineCommand | undefined {
    if (!edit || unavailable) return;
    const title = edit.title.trim(), purpose = edit.purpose.trim();
    const field: Field = !title || [...title].length > 160 ? 'title' : 'purpose';
    if (!title || [...title].length > 160 || (edit.kind !== 'slide' && (!purpose || [...purpose].length > 400))) {
      setEdit({ ...edit, field });
      setError(field === 'title' ? '请填写名称，最多 160 个字符。' : '请填写目的，最多 400 个字符。');
      input.current?.focus({ preventScroll: true });
      return;
    }
    if (edit.addition) return { ...edit.addition, node: { ...edit.addition.node, title, ...(edit.kind !== 'slide' ? { purpose } : {}) } };
    return { type: 'update', id: edit.id, title, ...(edit.kind !== 'slide' ? { purpose } : {}) };
  }
  async function perform(task: () => Promise<void>): Promise<boolean> {
    if (pending.current) return false;
    pending.current = true; setBusy(true); setError(''); setMessage('');
    try { await task(); if (!draftMode) setMessage('已保存'); return true; }
    catch (cause) { setError(cause instanceof Error ? cause.message : '保存失败，请重试。'); return false; }
    finally { pending.current = false; setBusy(false); }
  }
  async function confirmEdit() {
    const command = editCommand();
    if (!command || !edit) return;
    if (JSON.stringify(applyOutlineCommand(edit.base, command)) === JSON.stringify(edit.base)) { cancel(); return; }
    if (await perform(() => commit(command, edit.version))) { setEdit(undefined); focusRow(edit.addition ? command.type === 'insert' ? command.position.parent_id : undefined : edit.id); }
  }
  async function save() {
    if (!draftMode || !saveDraft || unavailable) return;
    const command = edit ? editCommand() : undefined;
    if (edit && !command) return;
    const next = command && edit ? applyOutlineCommand(edit.base, command) : value;
    let revision: number | undefined;
    if (await perform(async () => { revision = await saveDraft(next, edit?.version ?? version); })) {
      setEdit(undefined); setReviewRevision(revision);
    }
  }
  async function restore() {
    if (!draftMode || !restoreDraft || blocked) return;
    if (await perform(restoreDraft)) { setEdit(undefined); setError(''); }
  }
  async function change(command: OutlineCommand, capturedVersion = version) {
    if (locked || pending.current) return false;
    return perform(() => commit(command, capturedVersion));
  }
  function fold(id: string) {
    if (edit) return;
    setCollapsed(current => { const next = new Set(current); if (next.has(id)) next.delete(id); else next.add(id); return next; });
  }
  function siblingMove(id: string, delta: number) {
    const item = findOutlineNode(value, id);
    if (!item) return;
    const index = item.siblings.findIndex(node => node.id === id), target = item.siblings[index + delta];
    if (!target) return;
    void change({ type: 'move', id, position: { ...(item.parent ? { parent_id: item.parent.id } : {}), ...(delta < 0 ? { before_id: target.id } : { after_id: target.id }) } });
  }
  function dropCommand(event: DragEvent, targetId: string): OutlineCommand | undefined {
    const moving = drag.current && findOutlineNode(value, drag.current.id), target = findOutlineNode(value, targetId);
    if (!moving || !target || moving.node.id === targetId) return;
    if (moving.kind === target.kind) {
      const parentId = target.parent?.id;
      if (moving.parent?.id !== parentId && !outlineDestinations(value, moving.kind, moving.parent?.id).some(item => item.id === parentId)) return;
      const after = event.clientY > event.currentTarget.getBoundingClientRect().top + event.currentTarget.getBoundingClientRect().height / 2;
      return { type: 'move', id: moving.node.id, position: { ...(parentId ? { parent_id: parentId } : {}), ...(after ? { after_id: targetId } : { before_id: targetId }) } };
    }
    if (outlineDestinations(value, moving.kind, moving.parent?.id).some(item => item.id === targetId)) {
      return { type: 'move', id: moving.node.id, position: { parent_id: targetId } };
    }
  }
  function field(node: OutlineNode, kind: OutlineKind, name: Field) {
    const active = edit?.id === node.id;
    const text = active ? edit[name] : name === 'title' ? node.title : 'purpose' in node ? node.purpose : '';
    const className = name === 'title' ? 'outline-title' : 'outline-purpose';
    if (active && edit.field === name) return <span className={`outline-edit-field ${className}`}>
      <span className="outline-field-mirror" aria-hidden="true">{text || '\u00a0'}</span>
      <input ref={input} value={text} aria-label={`${labels[kind]}${name === 'title' ? '名称' : '目的'}`} aria-invalid={Boolean(error)} disabled={busy}
        onChange={event => { setEdit({ ...edit, [name]: event.target.value }); setError(''); }}
        onKeyDown={event => {
          if (event.nativeEvent.isComposing || event.keyCode === 229) return;
          if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); cancel(); }
          if (event.key === 'Enter') { event.preventDefault(); void confirmEdit(); }
          if (event.key === 'Tab' && kind !== 'slide' && ((name === 'title' && !event.shiftKey) || (name === 'purpose' && event.shiftKey))) {
            event.preventDefault(); setEdit({ ...edit, field: name === 'title' ? 'purpose' : 'title' });
          }
        }} />
    </span>;
    return <button type="button" className={`${className} ui-interactive`} title={text} disabled={active ? busy : locked}
      onClick={() => active ? setEdit({ ...edit, field: name }) : start(node.id, name)}>{text || (name === 'purpose' ? '待明确' : '未命名')}</button>;
  }
  function menu(node: OutlineNode, kind: OutlineKind) {
    const item = findOutlineNode(displayed, node.id)!;
    const index = item.siblings.findIndex(value => value.id === node.id);
    const destinations = outlineDestinations(displayed, kind, item.parent?.id);
    const section = node as OutlineSection;
    const promote = !draftMode && kind === 'subsection' && (node as OutlineSubsection).slides.length > 0;
    const removeDisabled = !draftMode && (kind === 'section' ? Boolean(section.slides.length || section.subsections.length)
      : promote && item.parent && 'subsections' in item.parent ? item.parent.subsections.length > 1 : false);
    return <DropdownMenu><DropdownMenuTrigger asChild><IconButton label={`${node.title}操作`} disabled={locked}><Ellipsis /></IconButton></DropdownMenuTrigger>
      <DropdownMenuContent align="end" sideOffset={6}>
        <DropdownMenuItem onSelect={() => start(node.id)}><Pencil className="mr-2 h-3.5 w-3.5" />编辑{labels[kind]}</DropdownMenuItem>
        {kind !== 'slide' && !(kind === 'section' && section.subsections.length) && <DropdownMenuItem disabled={(node as OutlineSubsection).slides.length >= 200} onSelect={() => add('slide', node.id)}><FilePlus2 className="mr-2 h-3.5 w-3.5" />新增页面</DropdownMenuItem>}
        {kind === 'section' && <DropdownMenuItem disabled={section.subsections.length >= 64} onSelect={() => add('subsection', node.id)}><FolderPlus className="mr-2 h-3.5 w-3.5" />新增小节</DropdownMenuItem>}
        <DropdownMenuSeparator />
        <DropdownMenuItem disabled={index === 0} onSelect={() => siblingMove(node.id, -1)}><ArrowUp className="mr-2 h-3.5 w-3.5" />上移</DropdownMenuItem>
        <DropdownMenuItem disabled={index === item.siblings.length - 1} onSelect={() => siblingMove(node.id, 1)}><ArrowDown className="mr-2 h-3.5 w-3.5" />下移</DropdownMenuItem>
        {kind !== 'section' && <DropdownMenuSub><DropdownMenuSubTrigger disabled={!destinations.length}><FolderInput className="mr-2 h-3.5 w-3.5" />移动到…<ChevronRight className="ml-auto h-3.5 w-3.5" /></DropdownMenuSubTrigger>
          <DropdownMenuSubContent>{destinations.map(target => <DropdownMenuItem key={target.id} onSelect={() => {
            void change({ type: 'move', id: node.id, position: { parent_id: target.id } }).then(done => { if (done) setCollapsed(current => { const next = new Set(current); next.delete(target.id); return next; }); });
          }}>{target.title}</DropdownMenuItem>)}</DropdownMenuSubContent>
        </DropdownMenuSub>}
        <DropdownMenuSeparator />
        <DropdownMenuItem destructive disabled={removeDisabled} onSelect={() => setDeleting({ id: node.id, version })}><Trash2 className="mr-2 h-3.5 w-3.5" />{promote ? '取消小节' : `删除${labels[kind]}`}</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>;
  }
  function row(node: OutlineNode, kind: OutlineKind, ordinal: number | string) {
    const editing = edit?.id === node.id, container = kind !== 'slide', folded = collapsed.has(node.id);
    const Icon = container ? folded ? Folder : FolderOpen : FileText;
    return <div data-node-id={node.id} className={`outline-row outline-${kind}-row${editing ? ' outline-row-editing' : ''}${dropTarget === node.id ? ' outline-drop-target' : ''}`}
      onDragOver={event => { if (!locked && dropCommand(event, node.id)) { event.preventDefault(); event.dataTransfer.dropEffect = 'move'; setDropTarget(node.id); } }}
      onDragLeave={event => { if (!event.currentTarget.contains(event.relatedTarget as Node)) setDropTarget(undefined); }}
      onDrop={event => {
        event.preventDefault(); const command = dropCommand(event, node.id), captured = drag.current;
        drag.current = undefined; setDropTarget(undefined);
        if (command && captured) void change(command, captured.version);
      }}>
      <button type="button" className="outline-grip ui-interactive" draggable={!locked} disabled={locked} aria-label={`拖动${node.title}`}
        onDragStart={event => { drag.current = { id: node.id, version }; event.dataTransfer.effectAllowed = 'move'; event.dataTransfer.setData('text/plain', node.id); }}
        onDragEnd={() => { drag.current = undefined; setDropTarget(undefined); }}><GripVertical /></button>
      {container && <IconButton className="outline-fold" label={`${folded ? '展开' : '收起'}${node.title}`} disabled={Boolean(edit)} onClick={() => fold(node.id)}>{folded ? <ChevronRight /> : <ChevronDown />}</IconButton>}
      {kind === 'slide' && <span className="outline-ordinal">{String(ordinal).padStart(2, '0')}</span>}
      {kind === 'slide' && openSlide ? <button type="button" className="outline-node-link ui-interactive" aria-label={`查看${node.title}设计稿`} title="查看设计稿" disabled={isEditing || busy} onClick={() => openSlide(node.id)}><Icon className="outline-node-icon" aria-hidden="true" /></button>
        : <Icon className="outline-node-icon" aria-hidden="true" />}
      {container && <span className="outline-ordinal">第 <strong>{ordinal}</strong> {kind === 'section' ? '章' : '节'}</span>}
      <div className="outline-copy">{field(node, kind, 'title')}{container && field(node, kind, 'purpose')}</div>
      <div className="outline-row-actions">
        {editing ? <><IconButton label="确认编辑" className="outline-confirm" disabled={busy || Boolean(unavailable)} onClick={() => void confirmEdit()}><Check /></IconButton><IconButton label="取消编辑" disabled={busy} onClick={cancel}><X /></IconButton></>
          : menu(node, kind)}
      </div>
    </div>;
  }
  function slideRows(slides: OutlineSlideNode[], parentId: string) {
    return slides.length ? slides.map(slide => <div key={slide.id}>{row(slide, 'slide', pages.findIndex(page => page.id === slide.id) + 1)}</div>)
      : <Button variant="ghost" className="outline-add-page" disabled={locked} onClick={() => add('slide', parentId)}><Plus />新增页面</Button>;
  }
  const allFolded = displayed.sections.length > 0 && displayed.sections.every(section => collapsed.has(section.id));
  const deletingItem = deleting && findOutlineNode(value, deleting.id);
  const promote = !draftMode && deletingItem?.kind === 'subsection' && (deletingItem.node as OutlineSubsection).slides.length > 0;
  const feedback = busy ? draftMode ? undefined : '正在保存…' : unavailable ?? (error || message);
  const feedbackError = !busy && Boolean(error || unavailable);
  const status = feedback ? <span className="management-feedback" role={feedbackError ? 'alert' : 'status'}>
    {!feedbackError && !busy && <Check aria-hidden="true" />}{feedback}
  </span> : undefined;
  if (showJSON && !edit && !busy) return <JSONPreview title="目录结构" value={value} notice={notice} />;
  return <>
    <DocumentCanvas title="目录结构" icon={List} actions={<div className="outline-heading-actions"><span>{displayed.sections.length} 章 · {pages.length} 页</span>
      <IconButton className="outline-collapse-all" label={allFolded ? '展开全部' : '收起全部'} disabled={Boolean(edit) || !displayed.sections.length} onClick={() => setCollapsed(allFolded ? new Set() : new Set(displayed.sections.map(section => section.id)))}>{allFolded ? <ChevronsDown /> : <ChevronsUp />}</IconButton></div>}
      footer={draftMode ? <div className="outline-footer">{status}<div className="ml-auto flex gap-2">
        <Button variant="ghost" disabled={busy || Boolean(blocked) || (!edit && !canRestore)} onClick={() => void restore()}>恢复</Button>
        <Button variant="ghost" disabled={busy || Boolean(unavailable)} onClick={() => void save()}>{busy ? '保存中…' : '保存'}</Button>
      </div></div> : status}>
      {notice}
      <div ref={root} className="outline-tree">
        {displayed.sections.map((section, sectionIndex) => <section className="outline-section" key={section.id}>
          {row(section, 'section', sectionIndex + 1)}
          {!collapsed.has(section.id) && <div className="outline-section-children">
            {section.subsections.length ? section.subsections.map((sub, subIndex) => <section className="outline-subsection" key={sub.id}>
              {row(sub, 'subsection', `${sectionIndex + 1}.${subIndex + 1}`)}
              {!collapsed.has(sub.id) && <div className="outline-subsection-children">{slideRows(sub.slides, sub.id)}</div>}
            </section>) : slideRows(section.slides, section.id)}
          </div>}
        </section>)}
        <Button variant="ghost" className="outline-add-section" disabled={locked || displayed.sections.length >= 32} onClick={() => add('section')}><Plus />新增章节</Button>
      </div>
    </DocumentCanvas>
    <ConfirmModal open={Boolean(deletingItem)} onOpenChange={open => { if (!open) setDeleting(undefined); }} title={promote ? '取消小节' : `删除${deletingItem ? labels[deletingItem.kind] : ''}`}
      description={promote ? `取消「${deletingItem?.node.title}」的分组，保留页面。` : `确定删除「${deletingItem?.node.title}」${deletingItem?.kind !== 'slide' ? '及其包含的页面' : ''}？`}
      confirmLabel={promote ? '确认' : '删除'} variant="danger" onConfirm={async () => {
        if (!deleting || locked) throw new Error('项目正忙，请稍后再操作。');
        if (!await change({ type: 'remove', id: deleting.id, ...(promote ? { promote: true } : {}) }, deleting.version)) throw new Error('删除失败');
      }} />
  </>;
}
