import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { Braces, Check, type LucideIcon } from 'lucide-react';
import type { PPTMutation, ProjectContentSnapshot, RestrictedPatch } from '../../api/types';
import { Button } from '../../components/ui/primitives';
import { TextField } from '../../components/ui/text-field';
import { useProjectStore } from '../../stores/projectStore';
import { DocumentCanvas } from './DocumentCanvas';
import { JSONPreview } from './JSONPreview';
import './management.css';

type Update<T> = (current: T) => T;
export interface TextEdit<T> {
  id: string; label: string; value: string; minLength?: number; maxLength: number; multiline?: boolean;
  update: (current: T, text: string) => T;
}
interface Draft<T> extends TextEdit<T> { base: T; hash?: string; scene?: number }
export interface ManagementController<T> {
  value: T; draft?: Draft<T>; saving: boolean; disabled: boolean; unavailable?: string;
  start: (edit: TextEdit<T>) => void;
  changeText: (text: string) => void;
  cancel: () => void;
  saveText: () => Promise<void>;
  commit: (update: Update<T>, undoable?: boolean) => Promise<void>;
  error?: string;
}

/** Only changed business fields are submitted. Optional fields are removed, never stored as null. */
export function changedFields<T extends object>(previous: T, next: T): RestrictedPatch[] {
  const before = previous as Record<string, unknown>;
  const after = next as Record<string, unknown>;
  return [...new Set([...Object.keys(before), ...Object.keys(after)])].flatMap((key): RestrictedPatch[] => {
    if (JSON.stringify(before[key]) === JSON.stringify(after[key])) return [];
    const path = `/${key.replace(/~/g, '~0').replace(/\//g, '~1')}`;
    if (after[key] === undefined) return [{ op: 'remove', path }];
    return [{ op: before[key] === undefined ? 'add' : 'replace', path, value: after[key] }];
  });
}

export function ManagementEditor<T>({ projectId, resourceKey, title, icon, value, hash, sceneRevision, blocked, mutation, children, notice, jsonValue, jsonAvailable = true }: {
  projectId?: string; resourceKey: string; title: string; icon: LucideIcon; value: T;
  hash?: string; sceneRevision?: number; blocked?: string;
  mutation: (next: T, previous: T) => PPTMutation;
  children: (editor: ManagementController<T>) => ReactNode;
  notice?: ReactNode; jsonValue?: unknown; jsonAvailable?: boolean;
}) {
  const [draft, setDraft] = useState<Draft<T>>();
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string>();
  const [message, setMessage] = useState<string>();
  const [showJSON, setShowJSON] = useState(false);
  const [undo, setUndo] = useState<{ value: T; hash?: string; scene?: number }>();
  const pending = useRef(false);
  const mounted = useRef(true);
  const returnTarget = useRef<HTMLElement | null>(null);
  const latest = useRef({ value, hash, sceneRevision, blocked });
  latest.current = { value, hash, sceneRevision, blocked };
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  const stale = draft && (draft.hash !== hash || draft.scene !== sceneRevision);
  const unavailable = saving ? undefined : blocked ?? (stale ? '内容已更新，请取消后重新编辑。' : undefined);
  const disabled = !projectId || Boolean(blocked) || saving || Boolean(draft);
  const undoStale = undo && (undo.hash !== hash || undo.scene !== sceneRevision);

  function focusTrigger() {
    // The button is mounted again after the inline editor closes.
    const id = returnTarget.current?.dataset.editId ?? returnTarget.current?.dataset.focusFallback;
    requestAnimationFrame(() => {
      if (!mounted.current) return;
      if (returnTarget.current?.isConnected) returnTarget.current.focus();
      else if (id) document.getElementById(id)?.focus();
    });
  }

  async function save(next: T, base: T, capturedHash: string | undefined, capturedScene: number | undefined, undoable = false): Promise<boolean> {
    if (!projectId || pending.current) return false;
    const current = latest.current;
    const snapshot = useProjectStore.getState().contentByProjectId[projectId];
    if (current.blocked) { setError(current.blocked); return false; }
    // Check both props and the store; a queued render must not allow an old array to overwrite a new one.
    if (current.hash !== capturedHash || current.sceneRevision !== capturedScene
      || (snapshot && (snapshot.hashes[resourceKey] !== capturedHash || snapshot.scene_revision !== capturedScene))) {
      setError('内容已更新，请取消后重新编辑。'); return false;
    }
    if (JSON.stringify(next) === JSON.stringify(base)) { setDraft(undefined); setError(undefined); focusTrigger(); return true; }
    pending.current = true; setSaving(true); setError(undefined); setMessage(undefined);
    try {
      const result: ProjectContentSnapshot = await useProjectStore.getState().mutateProject(projectId, {
        ...mutation(next, base), expected_hash: capturedHash, expected_scene_revision: capturedScene,
      });
      if (!mounted.current) return true;
      setDraft(undefined);
      setMessage(undoable ? '已删除条目' : '已保存');
      setUndo(undoable ? { value: base, hash: result.hashes[resourceKey], scene: result.scene_revision } : undefined);
      focusTrigger();
      return true;
    } catch (cause) {
      if (mounted.current) setError(cause instanceof Error ? cause.message : '保存失败，请重试。');
      void useProjectStore.getState().loadProjectContent(projectId);
      return false;
    } finally {
      pending.current = false;
      if (mounted.current) setSaving(false);
    }
  }

  const controller: ManagementController<T> = {
    value: draft?.base ?? value, draft, saving, disabled, unavailable, error,
    start(edit) {
      if (disabled || pending.current) return;
      returnTarget.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
      setError(undefined); setMessage(undefined); setUndo(undefined);
      setDraft({ ...edit, base: structuredClone(value), hash, scene: sceneRevision });
    },
    changeText(text) { setDraft(current => current ? { ...current, value: text } : current); setError(undefined); },
    cancel() { if (pending.current) return; setDraft(undefined); setError(undefined); focusTrigger(); },
    async saveText() {
      if (!draft || unavailable || pending.current) return;
      const text = draft.value.trim();
      if ([...text].length < (draft.minLength ?? 0)) { setError(`请填写${draft.label}，至少 ${draft.minLength} 个字符。`); return; }
      if ([...text].length > draft.maxLength) { setError(`${draft.label}不能超过 ${draft.maxLength} 个字符。`); return; }
      await save(draft.update(draft.base, text), draft.base, draft.hash, draft.scene);
    },
    async commit(update, undoable = false) {
      if (disabled || pending.current) return;
      returnTarget.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
      const base = structuredClone(value);
      await save(update(base), base, hash, sceneRevision, undoable);
    },
  };

  const feedbackError = !saving && Boolean(error || unavailable);
  const feedback = saving ? '正在保存…' : unavailable ?? (!draft ? error : undefined) ?? message;
  return <DocumentCanvas title={title} icon={icon} actions={
    <button type="button" className="management-json-toggle ui-interactive" aria-label="JSON 切换" aria-pressed={showJSON}
      title={showJSON ? '返回内容视图' : '查看 JSON'} disabled={Boolean(draft) || saving || !jsonAvailable}
      onClick={() => setShowJSON(current => !current)}><Braces aria-hidden="true" />JSON</button>
  } footer={feedback ? <div className="management-feedback" role={feedbackError ? 'alert' : 'status'}>
    {!feedbackError && !saving && <Check aria-hidden="true" />}<span>{feedback}</span>
    {undo && !undoStale && <button type="button" className="ui-interactive" disabled={disabled} onClick={async () => {
      if (disabled) return;
      await save(undo.value, value, undo.hash, undo.scene);
    }}>撤销</button>}
  </div> : undefined}>
    {notice}
    {showJSON && jsonAvailable ? <JSONPreview value={jsonValue ?? value} /> : children(controller)}
  </DocumentCanvas>;
}

export function InlineTextEditor<T>({ editor, hideLabel = false }: { editor: ManagementController<T>; hideLabel?: boolean }) {
  const errorId = useId();
  const draft = editor.draft;
  if (!draft) return null;
  return <form className="management-editor" noValidate onSubmit={event => { event.preventDefault(); void editor.saveText(); }}
    onKeyDown={event => {
      if (event.nativeEvent.isComposing) return;
      if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); editor.cancel(); }
      if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) { event.preventDefault(); void editor.saveText(); }
    }}>
    <fieldset disabled={editor.saving}>
      <TextField label={draft.label} hideLabel={hideLabel} value={draft.value} onChange={editor.changeText} minLength={draft.minLength}
        maxLength={draft.maxLength} multiline={draft.multiline} autoFocus describedBy={errorId} invalid={Boolean(editor.error)} />
    </fieldset>
    <p id={errorId} className="management-error" role={editor.error ? 'alert' : undefined}>{editor.error}</p>
    <div className="management-editor-actions">
      <Button type="button" variant="ghost" className="management-cancel" disabled={editor.saving} onClick={editor.cancel}>取消</Button>
      <Button type="submit" variant="primary" disabled={editor.saving || Boolean(editor.unavailable)}>{editor.saving ? '保存中…' : '保存'}</Button>
    </div>
  </form>;
}
