import { useMemo, useState } from 'react';
import { ChevronDown, ChevronRight, FilePlus2, FolderPlus, GripVertical, Plus, Trash2 } from 'lucide-react';
import { useProjectStore } from '../../stores/projectStore';
import { useDeckStore } from '../../stores/deckStore';
import { useActiveSession } from '../agent/useActiveSession';
import type { MutationPosition, OutlineSlideNode, PPTMutation } from '../../api/types';
import { cn } from '../../lib/utils';
import { flattenOutline, orderedSlides } from './selectors';

const clientRef = (kind: string) => `${kind}-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`;

export function DeckNavigator() {
  const activeProjectId = useProjectStore((state) => state.activeProjectId);
  const snapshot = useProjectStore((state) => activeProjectId ? state.contentByProjectId[activeProjectId] : undefined);
  const mutateProject = useProjectStore((state) => state.mutateProject);
  const pending = useProjectStore((state) => activeProjectId ? state.mutationPendingByProjectId[activeProjectId] : false);
  const currentSlideId = useDeckStore((state) => state.currentSlideId);
  const setCurrentSlideId = useDeckStore((state) => state.setCurrentSlideId);
  const { status } = useActiveSession();
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  const [draggedSlideId, setDraggedSlideId] = useState<string>();
  const [error, setError] = useState('');
  const slides = useMemo(() => orderedSlides(snapshot), [snapshot]);
  const locked = pending || ['creating', 'running', 'waiting', 'canceling'].includes(status);

  const mutate = async (request: PPTMutation) => {
    if (!activeProjectId || locked) return;
    setError('');
    try { await mutateProject(activeProjectId, request); } catch (reason) { setError(reason instanceof Error ? reason.message : '目录操作失败，请重试'); }
  };
  const outlineRevision = snapshot?.outline.revision;
  const positionForSibling = (parentId: string, siblings: string[], index: number): MutationPosition => index < siblings.length ? { parent_id: parentId, before_id: siblings[index] } : { parent_id: parentId };
  const rename = (nodeId: string, current: string, field: 'title' | 'label') => { const value = window.prompt('请输入新名称', current)?.trim(); if (value) void mutate({ op:'outline.update',expected_revision:outlineRevision,node_id:nodeId,changes:{[field]:value} }); };
  const remove = async (nodeId: string, slide = false) => {
    if (!window.confirm('确认删除？')) return;
    const index = slides.findIndex((item) => item.id === nodeId);
    await mutate({ op:'outline.remove',expected_revision:outlineRevision,node_id:nodeId });
    if (slide && currentSlideId === nodeId) { const next = slides[index + 1] ?? slides[index - 1]; setCurrentSlideId(next?.id ?? null); }
  };
  const moveSlide = (slideId: string, parentId: string, siblings: OutlineSlideNode[], targetIndex: number) => {
    const without = siblings.filter((item) => item.slide_id !== slideId).map((item) => item.slide_id);
    const clamped = Math.max(0, Math.min(targetIndex, without.length));
    void mutate({ op:'outline.move',expected_revision:outlineRevision,node_id:slideId,position:positionForSibling(parentId,without,clamped) });
  };

  if (!snapshot) return <aside className="flex h-full items-center justify-center border-r border-border bg-panel p-4 text-xs text-text-400">目录加载中</aside>;
  return <aside className="flex h-full min-h-0 flex-col border-r border-border bg-panel" aria-label="演示目录">
    <div className="flex items-center justify-between border-b border-border px-3 py-2">
      <div><h2 className="text-sm font-semibold text-text-900">目录</h2><p className="text-[10px] text-text-400">{slides.length} 页</p></div>
      <button disabled={locked} className="rounded p-1.5 hover:bg-panel-muted disabled:opacity-40" aria-label="新增章节" onClick={() => void mutate({op:'outline.insert',expected_revision:outlineRevision,node:{kind:'section',client_ref:clientRef('section'),title:'新章节',purpose:'待补充章节目的',slides:[],subsections:[]},position:{}})}><FolderPlus className="h-4 w-4" /></button>
    </div>
    {error && <div role="alert" className="border-b border-danger/20 bg-danger-soft px-3 py-2 text-xs text-danger">{error}</div>}
    {locked && <div className="border-b border-border bg-warning-soft px-3 py-2 text-xs text-warning">{pending ? '正在更新目录' : '任务运行中，目录暂不可编辑'}</div>}
    <div className="min-h-0 flex-1 overflow-y-auto p-2">
      {snapshot.outline.sections.length === 0 ? <div className="flex h-32 flex-col items-center justify-center gap-2 text-center text-xs text-text-400"><FilePlus2 className="h-5 w-5"/><span>目录为空，先新增章节</span></div> : snapshot.outline.sections.map((section) => {
        const sectionCollapsed=collapsed[section.id];
        return <section key={section.id} className="mb-2 rounded-lg border border-border bg-surface">
          <div className="group flex items-center gap-1 px-2 py-2">
            <button onClick={()=>setCollapsed((state)=>({...state,[section.id]:!sectionCollapsed}))}>{sectionCollapsed?<ChevronRight className="h-3.5 w-3.5"/>:<ChevronDown className="h-3.5 w-3.5"/>}</button>
            <button className="min-w-0 flex-1 truncate text-left text-xs font-semibold" onDoubleClick={()=>rename(section.id,section.title,'title')}>{section.title}</button>
            <button disabled={locked} aria-label={`在${section.title}新增页面`} onClick={()=>void mutate({op:'outline.insert',expected_revision:outlineRevision,node:{kind:'slide',client_ref:clientRef('slide'),label:'新页面',role:'content'},position:{parent_id:section.subsections[0]?.id??section.id}})}><Plus className="h-3.5 w-3.5"/></button>
            <button disabled={locked||section.slides.length>0||section.subsections.length>0} aria-label={`删除${section.title}`} onClick={()=>void remove(section.id)}><Trash2 className="h-3.5 w-3.5"/></button>
          </div>
          {!sectionCollapsed && <div className="pb-1">
            {section.slides.map((node,index)=><SlideRow key={node.slide_id} node={node} index={flattenOutline(snapshot.outline).find((item)=>item.node.slide_id===node.slide_id)?.ordinal??index+1} selected={currentSlideId===node.slide_id} locked={locked} pending={!snapshot.slides_by_id[node.slide_id]?.spec} onSelect={()=>setCurrentSlideId(node.slide_id)} onRename={()=>rename(node.slide_id,node.label,'label')} onRemove={()=>void remove(node.slide_id,true)} onMove={(delta)=>moveSlide(node.slide_id,section.id,section.slides,index+delta)} onDrag={()=>setDraggedSlideId(node.slide_id)} onDrop={()=>draggedSlideId&&moveSlide(draggedSlideId,section.id,section.slides,index)} />)}
            {section.subsections.map((subsection)=><div key={subsection.id} className="mx-1 mb-1 rounded border border-border/70">
              <div className="flex items-center gap-1 px-2 py-1.5"><span className="min-w-0 flex-1 truncate text-[11px] font-medium" onDoubleClick={()=>rename(subsection.id,subsection.title,'title')}>{subsection.title}</span><button disabled={locked} aria-label={`在${subsection.title}新增页面`} onClick={()=>void mutate({op:'outline.insert',expected_revision:outlineRevision,node:{kind:'slide',client_ref:clientRef('slide'),label:'新页面',role:'content'},position:{parent_id:subsection.id}})}><Plus className="h-3 w-3"/></button><button disabled={locked||subsection.slides.length>0} onClick={()=>void remove(subsection.id)}><Trash2 className="h-3 w-3"/></button></div>
              {subsection.slides.map((node,index)=><SlideRow key={node.slide_id} node={node} index={flattenOutline(snapshot.outline).find((item)=>item.node.slide_id===node.slide_id)?.ordinal??index+1} selected={currentSlideId===node.slide_id} locked={locked} pending={!snapshot.slides_by_id[node.slide_id]?.spec} onSelect={()=>setCurrentSlideId(node.slide_id)} onRename={()=>rename(node.slide_id,node.label,'label')} onRemove={()=>void remove(node.slide_id,true)} onMove={(delta)=>moveSlide(node.slide_id,subsection.id,subsection.slides,index+delta)} onDrag={()=>setDraggedSlideId(node.slide_id)} onDrop={()=>draggedSlideId&&moveSlide(draggedSlideId,subsection.id,subsection.slides,index)} />)}
            </div>)}
            {section.slides.length===0&&section.subsections.length===0&&<button disabled={locked} className="mx-2 mb-1 flex items-center gap-1 text-[11px] text-accent" onClick={()=>void mutate({op:'outline.insert',expected_revision:outlineRevision,node:{kind:'subsection',client_ref:clientRef('subsection'),title:'新子节'},position:{parent_id:section.id}})}><Plus className="h-3 w-3"/>新增子节</button>}
          </div>}
        </section>;
      })}
    </div>
  </aside>;
}

function SlideRow({node,index,selected,locked,pending,onSelect,onRename,onRemove,onMove,onDrag,onDrop}:{node:OutlineSlideNode;index:number;selected:boolean;locked:boolean;pending:boolean;onSelect:()=>void;onRename:()=>void;onRemove:()=>void;onMove:(delta:number)=>void;onDrag:()=>void;onDrop:()=>void}){
  return <div draggable={!locked} onDragStart={onDrag} onDragOver={(event)=>event.preventDefault()} onDrop={onDrop} className={cn('group mx-1 flex items-center gap-1 rounded px-1.5 py-1.5 text-xs',selected?'bg-accent-soft text-accent':'hover:bg-panel-muted')}>
    <GripVertical className="h-3 w-3 text-text-400"/><button className="w-5 text-[10px] tabular-nums text-text-400" onClick={onSelect}>{index}</button><button className="min-w-0 flex-1 truncate text-left" onClick={onSelect} onDoubleClick={onRename}>{node.label}</button>{pending&&<span className="h-1.5 w-1.5 animate-pulse rounded-full bg-warning" title="等待生成设计稿"/>}<button disabled={locked} className="opacity-0 group-hover:opacity-100" onClick={()=>onMove(-1)}>↑</button><button disabled={locked} className="opacity-0 group-hover:opacity-100" onClick={()=>onMove(1)}>↓</button><button disabled={locked} className="opacity-0 group-hover:opacity-100" onClick={onRemove}><Trash2 className="h-3 w-3"/></button>
  </div>;
}
