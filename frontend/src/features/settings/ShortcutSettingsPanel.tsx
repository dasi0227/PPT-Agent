import { useEffect, useState } from 'react';
import { Loader2, RotateCcw } from 'lucide-react';
import { Button } from '../../components/ui/primitives';
import { ConfirmModal } from '../../components/ui/modal-confirm';
import { clearShortcutResetUndo, useShortcutStore } from '../../stores/shortcutStore';
import { bindingSignature, recordShortcut, shortcutCatalog, shortcutLabel, validateBindings, type ShortcutBinding, type ShortcutSettings } from '../../lib/shortcuts';
import { isMac } from '../../lib/platform';
import { showGlobalError } from '../../stores/toastStore';

export function ShortcutSettingsPanel({ refreshKey, onDirtyChange, onSavingChange }: {
  refreshKey: number;
  onDirtyChange: (dirty: boolean) => void;
  onSavingChange: (saving: boolean) => void;
}) {
  const revision = useShortcutStore(state => state.revision);
  const bindings = useShortcutStore(state => state.bindings);
  const saving = useShortcutStore(state => state.saving);
  const [draft, setDraft] = useState<ShortcutSettings | null>(null);
  const [baseline, setBaseline] = useState('');
  const [loading, setLoading] = useState(true);
  const [resetOpen, setResetOpen] = useState(false);
  const [error, setError] = useState('');
  const [recording, setRecording] = useState<string | null>(null);
  const fingerprint = (value: ShortcutSettings) => JSON.stringify(shortcutCatalog.map(def => bindingSignature(value.bindings[def.id])));
  const dirty = !!draft && fingerprint(draft) !== baseline;
  const invalid = draft ? validateBindings(draft.bindings) : null;
  const stale = !!draft && dirty && revision !== draft.revision;

  useEffect(() => { onDirtyChange(dirty); }, [dirty, onDirtyChange]);
  useEffect(() => { onSavingChange(saving); }, [saving, onSavingChange]);
  useEffect(() => () => { onDirtyChange(false); onSavingChange(false); }, [onDirtyChange, onSavingChange]);
  useEffect(() => {
    let active = true;
    setLoading(true); setError('');
    void useShortcutStore.getState().load().then(() => {
      if (!active) return;
      const { revision, bindings } = useShortcutStore.getState();
      const next = { revision, bindings };
      setDraft(next); setBaseline(fingerprint(next));
    }).catch(cause => { if (active) setError(cause instanceof Error ? cause.message : '快捷键设置加载失败'); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [refreshKey]);

  const change = (id: string, binding: ShortcutBinding) => {
    clearShortcutResetUndo();
    setError('');
    setDraft(current => current ? { ...current, bindings: { ...current.bindings, [id]: binding } } : current);
  };
  useEffect(() => {
    if (!draft || !dirty || invalid || stale || saving || loading || error) return;
    // Wait for the current edit to settle; invalid drafts never replace the saved bindings.
    const timer = window.setTimeout(() => {
      void useShortcutStore.getState().save(draft).then(value => {
        setDraft(value); setBaseline(fingerprint(value));
      }).catch(cause => {
        const message = cause instanceof Error ? cause.message : '快捷键保存失败';
        setError(message); showGlobalError(message);
      });
    }, 250);
    return () => window.clearTimeout(timer);
  }, [draft, dirty, invalid, stale, saving, loading, error]);
  useEffect(() => {
    if (!draft || dirty || saving || loading || draft.revision === revision) return;
    const next = { revision, bindings };
    setDraft(next); setBaseline(fingerprint(next)); setRecording(null);
  }, [bindings, dirty, draft, loading, revision, saving]);
  const heading = <><div className="settings-heading">
    <h1>快捷键</h1>
    <Button type="button" variant="secondary" icon={RotateCcw} className="shrink-0" onClick={() => { setRecording(null); setResetOpen(true); }}>全部重置</Button>
  </div>
    <ConfirmModal open={resetOpen} onOpenChange={setResetOpen} title="全部重置？"
      description={<>将把<strong className="font-semibold text-danger underline decoration-danger/70 decoration-2 underline-offset-4">所有自定义快捷键</strong>恢复为默认设置。</>}
      confirmLabel="全部重置" variant="danger" onConfirm={async () => {
        const store = useShortcutStore.getState();
        if (draft && dirty && !invalid && draft.revision === store.revision && !store.saving) await store.save(draft);
        const value = await store.reset();
        setDraft(value); setBaseline(fingerprint(value)); setError('');
      }} />
  </>;
  if (loading) return <>{heading}<div className="flex items-center gap-2 py-16 text-sm text-text-600"><Loader2 size={17} className="animate-spin" />正在读取快捷键…</div></>;
  if (!draft) return <>{heading}<p role="alert" className="text-sm text-danger">{error}</p></>;
  return (
    <>
    {heading}
    <div className="settings-routing">
      {(invalid || error || stale) && <p role="alert" className="mb-5 text-sm text-danger">{invalid || error || '配置已在其它窗口更新，请点击顶部刷新后重新编辑。'}</p>}
      {['输入触发符', '演示文稿', '输入框'].map(group => (
        <section key={group} className="mb-8">
          <h2>{group}</h2>
          <div className="settings-group">
            {shortcutCatalog.filter(def => def.group === group).map(def => {
              const binding = draft.bindings[def.id];
              return <div key={def.id} className="settings-row">
                <div><h3>{def.label}</h3></div>
                <input
                    aria-label={def.label}
                    value={recording === def.id && def.kind !== 'trigger' ? '按下组合键…' : shortcutLabel(binding)}
                    readOnly={def.kind !== 'trigger'}
                    disabled={saving}
                    maxLength={def.kind === 'trigger' ? 1 : undefined}
                    onChange={event => { if (def.kind === 'trigger') change(def.id, { trigger: event.target.value }); }}
                    onFocus={() => setRecording(def.id)}
                    onBlur={() => setRecording(null)}
                    onKeyDown={event => {
                      if (def.kind === 'trigger') return;
                      event.stopPropagation();
                      if (event.key === 'Tab') return;
                      event.preventDefault();
                      if (event.key === 'Escape') { event.currentTarget.blur(); return; }
                      if (event.nativeEvent.isComposing || ['Meta', 'Control', 'Alt', 'Shift'].includes(event.key)) return;
                      if (isMac() ? event.ctrlKey : event.metaKey) { setError('请使用 Command/Ctrl 或 Option/Alt 作为修饰键'); return; }
                      change(def.id, recordShortcut(event.nativeEvent));
                      event.currentTarget.blur();
                    }}
                    className="settings-value h-9 rounded-lg border border-border bg-panel px-3 text-center text-sm text-text-900 focus:bg-surface"
                  />
              </div>;
            })}
          </div>
        </section>
      ))}
    </div>
    </>
  );
}
