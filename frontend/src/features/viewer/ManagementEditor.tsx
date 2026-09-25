import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { ArrowDown, ArrowUp, Trash2 } from 'lucide-react';
import type { PPTMutation } from '../../api/types';
import { Button, IconButton, InlineNotice } from '../../components/ui/primitives';
import { useProjectStore } from '../../stores/projectStore';
import { showGlobalSuccess } from '../../stores/toastStore';
import { moveItem } from '../../lib/utils';

export function ManagementEditor<T>({ projectId, value, hash, sceneRevision, blocked, create = false, mutation, children, fields }: {
  projectId: string; value: T; hash?: string; sceneRevision?: number; blocked?: string; create?: boolean;
  mutation: (value: T) => PPTMutation; children: ReactNode;
  fields: (value: T, change: (value: T) => void) => ReactNode;
}) {
  const [edit, setEdit] = useState<{ value: T; hash?: string; scene?: number }>();
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string>();
  const stale = edit && (edit.hash !== hash || edit.scene !== sceneRevision);
  const unavailable = blocked ?? (stale ? '内容已更新，请取消后重新编辑。' : undefined);
  if (!edit) return <>
    <div className="mb-5 flex flex-wrap items-center justify-end gap-3">
      {blocked && <span className="text-xs text-text-600">{blocked}</span>}
      <Button variant="secondary" disabled={Boolean(blocked)} onClick={() => { setError(undefined); setEdit({ value: structuredClone(value), hash, scene: sceneRevision }); }}>{create ? '创建设计稿' : '编辑'}</Button>
    </div>
    {children}
  </>;
  return <form className="space-y-6" onSubmit={async event => {
    event.preventDefault();
    if (saving || unavailable) return;
    setSaving(true); setError(undefined);
    try {
      await useProjectStore.getState().mutateProject(projectId, { ...mutation(edit.value), expected_hash: edit.hash, expected_scene_revision: edit.scene });
      setEdit(undefined); showGlobalSuccess('已保存');
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '保存失败，请重试。');
      void useProjectStore.getState().loadProjectContent(projectId);
    } finally { setSaving(false); }
  }}>
    <fieldset disabled={saving || Boolean(blocked)} className="min-w-0 space-y-6">
      {fields(edit.value, value => setEdit({ ...edit, value }))}
    </fieldset>
    {(error || unavailable) && <InlineNotice tone="danger">{unavailable ?? error}</InlineNotice>}
    <div className="flex justify-end gap-2 border-t border-border pt-4">
      <Button variant="secondary" type="button" disabled={saving} onClick={() => { setEdit(undefined); setError(undefined); }}>取消</Button>
      <Button variant="primary" type="submit" disabled={saving || Boolean(unavailable)}>{saving ? '保存中…' : '保存'}</Button>
    </div>
  </form>;
}

export function TextField({ label, value, onChange, maxLength, minLength = 0, multiline = false }: {
  label: string; value: string; onChange: (value: string) => void; maxLength: number; minLength?: number; multiline?: boolean;
}) {
  const id = useId();
  const input = useRef<HTMLInputElement | HTMLTextAreaElement | null>(null);
  useEffect(() => {
    input.current?.setCustomValidity([...value.trim()].length < minLength ? `请填写${label}，至少 ${minLength} 个字符。` : '');
  }, [label, minLength, value]);
  const common = { id, value, ref: (node: HTMLInputElement | HTMLTextAreaElement | null) => { input.current = node; },
    required: minLength > 0, minLength, maxLength,
    onChange: (event: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => onChange(event.target.value),
    className: 'w-full min-w-0 rounded-md border border-border bg-panel px-3 py-2 text-sm text-text-900 outline-none disabled:opacity-50' };
  return <div className="space-y-2"><label htmlFor={id} className="block text-sm font-medium text-text-700">{label}</label>
    {multiline ? <textarea {...common} rows={3} className={`${common.className} resize-y`} /> : <input {...common} type="text" />}
  </div>;
}

export function ItemActions({ label, index, count, onMove, onRemove }: {
  label: string; index: number; count: number; onMove: (to: number) => void; onRemove: () => void;
}) {
  return <div className="flex items-center gap-1">
    <IconButton label={`上移${label}`} disabled={index === 0} onClick={() => onMove(index - 1)}><ArrowUp className="h-4 w-4" /></IconButton>
    <IconButton label={`下移${label}`} disabled={index === count - 1} onClick={() => onMove(index + 1)}><ArrowDown className="h-4 w-4" /></IconButton>
    <IconButton label={`删除${label}`} className="text-danger" onClick={onRemove}><Trash2 className="h-4 w-4" /></IconButton>
  </div>;
}

export function TextListField({ label, values, onChange, maxLength }: { label: string; values: string[]; onChange: (values: string[]) => void; maxLength: number }) {
  return <section className="space-y-3" aria-label={label}>
    <h3 className="text-sm font-semibold text-text-900">{label}</h3>
    {values.length === 0 && <p className="text-sm text-text-600">暂无条目</p>}
    {values.map((value, index) => <div key={index} className="space-y-1 border-b border-border pb-3">
      <TextField label={`${label} ${index + 1}`} value={value} minLength={1} maxLength={maxLength} multiline onChange={next => onChange(values.map((item, i) => i === index ? next : item))} />
      <div className="flex justify-end"><ItemActions label={`${label} ${index + 1}`} index={index} count={values.length} onMove={to => onChange(moveItem(values, index, to))} onRemove={() => onChange(values.filter((_, i) => i !== index))} /></div>
    </div>)}
    <Button type="button" variant="secondary" disabled={values.length >= 32} onClick={() => onChange([...values, ''])}>添加{label}</Button>
  </section>;
}
