import { useEffect, useRef, useState } from 'react';
import { ListTree, NotebookPen, Palette, Plus, Trash2, ArrowUp, ArrowDown } from 'lucide-react';
import type { Design, Manifest, Outline, OutlineSection, OutlineSlideNode, OutlineSubsection, ResourceEditApproval } from '../../api/types';
import { runsApi } from '../../api/runs';
import { Button } from '../../components/ui/primitives';
import { useResourceApprovalStore, type ActiveResourceApproval } from '../../stores/resourceApprovalStore';
import { DocumentCanvas } from './DocumentCanvas';
import { ManifestFields, DesignFields } from './AuthoringFields';
import type { ManagementController, TextEdit } from './ManagementEditor';
import { partLabel } from './semanticLabels';

type EditableResource = Manifest | Design | Outline;
type InlineDraft<T> = TextEdit<T> & { base: T };

function errorMessage(error: unknown) { return error instanceof Error ? error.message : '操作失败'; }

function ApprovalActions({ busy, onClose, onApprove }: { busy: boolean; onClose: () => void; onApprove: () => void }) {
  return <div className="flex flex-wrap justify-end gap-2">
    <Button type="button" variant="ghost" disabled={busy} onClick={onClose}>返回</Button>
    <Button type="button" variant="ghost" className="ui-success-solid" disabled={busy} onClick={onApprove}>{busy ? '提交中…' : '保存并通过'}</Button>
  </div>;
}

function FieldApprovalEditor<T extends Manifest | Design>({ record, persist, approve, close }: {
  record: ResourceEditApproval; persist: (draft: EditableResource) => Promise<ResourceEditApproval>;
  approve: (record: ResourceEditApproval) => Promise<void>; close: () => void;
}) {
  const value = record.draft as T;
  const [inline, setInline] = useState<InlineDraft<T>>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const pending = useRef(false);
  const save = async (next: T) => {
    if (pending.current) return null;
    pending.current = true; setBusy(true); setError('');
    try { return await persist(next); }
    catch (cause) { setError(errorMessage(cause)); return null; }
    finally { pending.current = false; setBusy(false); }
  };
  const inlineValue = () => {
    if (!inline) return null;
    const text = inline.value.trim();
    if ([...text].length < (inline.minLength ?? 0) || [...text].length > inline.maxLength) {
      setError(`${inline.label}长度不符合要求`); return null;
    }
    return inline.update(inline.base, text);
  };
  const saveInline = async () => {
    const next = inlineValue();
    if (!next) return;
    const saved = await save(next);
    if (saved) setInline(undefined);
  };
  const controller: ManagementController<T> = {
    value: inline?.base ?? value, draft: inline, saving: busy, disabled: busy || Boolean(inline), error,
    start(edit) { if (!busy) { setInline({ ...edit, base: structuredClone(value) }); setError(''); } },
    changeText(text) { setInline(current => current ? { ...current, value: text } : undefined); setError(''); },
    cancel() { setInline(undefined); setError(''); },
    saveText: saveInline,
    async commit(update) { if (!inline && !busy) await save(update(structuredClone(value))); },
  };
  const submit = async () => {
    let current = record;
    if (inline) {
      const next = inlineValue();
      if (!next) return;
      const saved = await save(next);
      if (!saved) return;
      current = saved;
      setInline(undefined);
    }
    setBusy(true);
    try { await approve(current); }
    catch (cause) { setError(errorMessage(cause)); setBusy(false); }
  };
  const isManifest = record.resource === 'manifest';
  return <DocumentCanvas title={partLabel(record.resource)} icon={isManifest ? NotebookPen : Palette}
    footer={<ApprovalActions busy={busy} onClose={close} onApprove={() => void submit()} />}>
    {error && <p className="mb-3 text-xs text-danger" role="alert">{error}</p>}
    {isManifest ? <ManifestFields editor={controller as unknown as ManagementController<Manifest>} />
      : <DesignFields editor={controller as unknown as ManagementController<Design>} />}
  </DocumentCanvas>;
}

function newID(prefix: 'sec' | 'sub' | 'sli') { return `${prefix}_${crypto.randomUUID().replace(/-/g, '')}`; }
function move<T>(items: T[], index: number, delta: number): T[] {
  const next = [...items];
  const target = index + delta;
  if (target < 0 || target >= next.length) return next;
  [next[index], next[target]] = [next[target], next[index]];
  return next;
}

function OutlineApprovalEditor({ record, persist, approve, close }: {
  record: ResourceEditApproval; persist: (draft: EditableResource) => Promise<ResourceEditApproval>;
  approve: (record: ResourceEditApproval) => Promise<void>; close: () => void;
}) {
  const [outline, setOutline] = useState<Outline>(() => structuredClone(record.draft as Outline));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const dirty = JSON.stringify(outline) !== JSON.stringify(record.draft);
  const updateSection = (index: number, edit: (section: OutlineSection) => OutlineSection) => setOutline(current => ({
    sections: current.sections.map((section, i) => i === index ? edit(section) : section),
  }));
  const updateSubsection = (sectionIndex: number, subIndex: number, edit: (sub: OutlineSubsection) => OutlineSubsection) =>
    updateSection(sectionIndex, section => ({ ...section, subsections: section.subsections.map((sub, i) => i === subIndex ? edit(sub) : sub) }));
  const save = async () => {
    if (!dirty) return record;
    setBusy(true); setError('');
    try { return await persist(outline); }
    catch (cause) { setError(errorMessage(cause)); return null; }
    finally { setBusy(false); }
  };
  const submit = async () => {
    const current = await save();
    if (!current) return;
    setBusy(true);
    try { await approve(current); }
    catch (cause) { setError(errorMessage(cause)); setBusy(false); }
  };
  const inputClass = 'min-w-0 flex-1 rounded-md border border-border bg-surface px-2 py-1 text-sm text-text-900 ui-interactive';
  const actionClass = 'ui-interactive rounded-md p-1.5 text-text-600 disabled:opacity-40';
  const destinations = outline.sections.flatMap(section => section.subsections.length
    ? section.subsections.map(sub => ({ id: sub.id, label: `${section.title} / ${sub.title}` }))
    : [{ id: section.id, label: section.title }]);
  const relocateSlide = (slideId: string, destination: string) => setOutline(current => {
    const next = structuredClone(current);
    let moving: OutlineSlideNode | undefined;
    for (const section of next.sections) {
      const direct = section.slides.findIndex(slide => slide.id === slideId);
      if (direct >= 0) moving = section.slides.splice(direct, 1)[0];
      for (const sub of section.subsections) {
        const index = sub.slides.findIndex(slide => slide.id === slideId);
        if (index >= 0) moving = sub.slides.splice(index, 1)[0];
      }
    }
    if (!moving) return current;
    for (const section of next.sections) {
      if (section.id === destination && section.subsections.length === 0) section.slides.push(moving);
      for (const sub of section.subsections) if (sub.id === destination) sub.slides.push(moving);
    }
    return next;
  });
  const relocateSubsection = (subId: string, destination: string) => setOutline(current => {
    const next = structuredClone(current);
    let moving: OutlineSubsection | undefined;
    for (const section of next.sections) {
      const index = section.subsections.findIndex(sub => sub.id === subId);
      if (index >= 0) moving = section.subsections.splice(index, 1)[0];
    }
    if (!moving) return current;
    const target = next.sections.find(section => section.id === destination && section.slides.length === 0);
    if (!target) return current;
    target.subsections.push(moving);
    return next;
  });
  const buttons = (index: number, count: number, onMove: (delta: number) => void, onDelete: () => void, label: string) => <div className="flex shrink-0 items-center gap-0.5">
    <button type="button" className={actionClass} disabled={index === 0 || busy} aria-label={`上移${label}`} onClick={() => onMove(-1)}><ArrowUp className="h-4 w-4" /></button>
    <button type="button" className={actionClass} disabled={index === count - 1 || busy} aria-label={`下移${label}`} onClick={() => onMove(1)}><ArrowDown className="h-4 w-4" /></button>
    <button type="button" className={`${actionClass} ui-danger`} disabled={busy} aria-label={`删除${label}`} onClick={onDelete}><Trash2 className="h-4 w-4" /></button>
  </div>;
  const slides = (nodes: OutlineSlideNode[], change: (next: OutlineSlideNode[]) => void, label: string, parentId: string) => <div className="space-y-2">
    {nodes.map((slide, index) => <div key={slide.id} className="flex items-center gap-2">
      <input className={inputClass} aria-label={`${label}页面标题`} maxLength={160} value={slide.title} onChange={event => change(nodes.map((node, i) => i === index ? { ...node, title: event.target.value } : node))} />
      {destinations.length > 1 && <select className="max-w-32 rounded-md border border-border bg-surface px-1 py-1 text-xs ui-interactive" aria-label="移动页面至" value={parentId}
        onChange={event => relocateSlide(slide.id, event.target.value)}>{destinations.map(destination => <option key={destination.id} value={destination.id}>{destination.label}</option>)}</select>}
      {buttons(index, nodes.length, delta => change(move(nodes, index, delta)), () => change(nodes.filter((_, i) => i !== index)), '页面')}
    </div>)}
    <button type="button" className="management-add ui-interactive" disabled={busy || nodes.length >= 200} onClick={() => change([...nodes, { id: newID('sli'), title: '待明确' }])}><Plus aria-hidden="true" />新增页面</button>
  </div>;
  return <DocumentCanvas title="目录结构" icon={ListTree} footer={<div className="flex flex-wrap justify-end gap-2">
    <Button type="button" variant="ghost" disabled={busy} onClick={close}>返回</Button>
    <Button type="button" variant="secondary" disabled={!dirty || busy} onClick={() => void save()}>保存草稿</Button>
    <Button type="button" variant="ghost" className="ui-success-solid" disabled={busy} onClick={() => void submit()}>{busy ? '提交中…' : '保存并通过'}</Button>
  </div>}>
    {error && <p className="mb-3 text-xs text-danger" role="alert">{error}</p>}
    <div className="space-y-6">
      {outline.sections.map((section, sectionIndex) => <section key={section.id} className="management-section">
        <div className="flex items-center gap-2">
          <input className={inputClass} aria-label="章节标题" maxLength={160} value={section.title} onChange={event => updateSection(sectionIndex, value => ({ ...value, title: event.target.value }))} />
          {buttons(sectionIndex, outline.sections.length,
            delta => setOutline(current => ({ sections: move(current.sections, sectionIndex, delta) })),
            () => setOutline(current => ({ sections: current.sections.filter((_, i) => i !== sectionIndex) })), '章节')}
        </div>
        <input className={inputClass} aria-label="章节目的" maxLength={400} value={section.purpose} onChange={event => updateSection(sectionIndex, value => ({ ...value, purpose: event.target.value }))} />
        {section.subsections.length ? <div className="space-y-4 pl-4">
          {section.subsections.map((sub, subIndex) => <section key={sub.id} className="space-y-2 border-l border-border pl-3">
            <div className="flex items-center gap-2"><input className={inputClass} aria-label="小节标题" maxLength={160} value={sub.title}
              onChange={event => updateSubsection(sectionIndex, subIndex, value => ({ ...value, title: event.target.value }))} />
              {outline.sections.length > 1 && <select className="max-w-32 rounded-md border border-border bg-surface px-1 py-1 text-xs ui-interactive" aria-label="移动小节至" value={section.id}
                onChange={event => relocateSubsection(sub.id, event.target.value)}>{outline.sections.filter(value => value.slides.length === 0).map(value =>
                  <option key={value.id} value={value.id}>{value.title}</option>)}</select>}
              {buttons(subIndex, section.subsections.length,
                delta => updateSection(sectionIndex, value => ({ ...value, subsections: move(value.subsections, subIndex, delta) })),
                () => updateSection(sectionIndex, value => ({ ...value, subsections: value.subsections.filter((_, i) => i !== subIndex) })), '小节')}
            </div>
            <input className={inputClass} aria-label="小节目的" maxLength={400} value={sub.purpose}
              onChange={event => updateSubsection(sectionIndex, subIndex, value => ({ ...value, purpose: event.target.value }))} />
            {slides(sub.slides, next => updateSubsection(sectionIndex, subIndex, value => ({ ...value, slides: next })), '小节', sub.id)}
          </section>)}
        </div> : slides(section.slides, next => updateSection(sectionIndex, value => ({ ...value, slides: next })), '章节', section.id)}
        <button type="button" className="management-add ui-interactive" disabled={busy || section.slides.length > 0 || section.subsections.length >= 64}
          onClick={() => updateSection(sectionIndex, value => ({ ...value, subsections: [...value.subsections,
            { id: newID('sub'), title: '待明确', purpose: '待明确', slides: [] }] }))}><Plus aria-hidden="true" />新增小节</button>
      </section>)}
      <button type="button" className="management-add ui-interactive" disabled={busy || outline.sections.length >= 32}
        onClick={() => setOutline(current => ({ sections: [...current.sections,
          { id: newID('sec'), title: '待明确', purpose: '待明确', slides: [], subsections: [] }] }))}><Plus aria-hidden="true" />新增章节</button>
    </div>
  </DocumentCanvas>;
}

export function ApprovalDraftEditor({ active }: { active: ActiveResourceApproval }) {
  const [record, setRecord] = useState<ResourceEditApproval | null>(null);
  const [error, setError] = useState('');
  const close = useResourceApprovalStore(state => state.close);
  useEffect(() => {
    let live = true;
    setRecord(null); setError('');
    void runsApi.getResourceEditApproval(active.runId, active.interactionId)
      .then(next => { if (live) setRecord(next); })
      .catch(cause => { if (live) setError(errorMessage(cause)); });
    return () => { live = false; };
  }, [active.runId, active.interactionId]);
  const persist = async (draft: EditableResource) => {
    if (!record) throw new Error('草稿尚未加载');
    const next = await runsApi.updateResourceEditDraft(active.runId, active.interactionId, record.revision, draft);
    setRecord(next);
    return next;
  };
  const approve = async (current: ResourceEditApproval) => {
    await runsApi.decideResourceEditApproval(active.runId, active.interactionId, active.callId, current.revision, 'approve');
    close();
  };
  if (!record) return <DocumentCanvas title={partLabel(active.resource)}>{error ? <p role="alert" className="text-danger">{error}</p> : <p className="text-text-600">加载中…</p>}</DocumentCanvas>;
  if (record.state !== 'pending') return <DocumentCanvas title={partLabel(active.resource)}><Button onClick={close}>返回</Button></DocumentCanvas>;
  if (active.resource === 'outline') return <OutlineApprovalEditor key={record.interaction_id} record={record} persist={persist} approve={approve} close={close} />;
  return <FieldApprovalEditor key={record.interaction_id} record={record} persist={persist} approve={approve} close={close} />;
}
