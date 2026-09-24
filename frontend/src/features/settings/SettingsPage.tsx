import { ShortcutSettingsPanel } from './ShortcutSettingsPanel';
import { HomeLogo } from '../../components/ui/HomeLogo';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useBlocker, useLocation, useNavigate } from 'react-router-dom';
import { Cpu, Eye, EyeOff, Keyboard, Loader2, Plus, RefreshCw, Route, Trash2 } from 'lucide-react';
import { settingsApi, SIDE_PURPOSES, type SidePurpose } from '../../api/settings';
import { ConfirmModal } from '../../components/ui/modal-confirm';
import { IconButton } from '../../components/ui/primitives';
import { Select } from '../../components/ui/select';
import { useComposerStore } from '../../stores/composerStore';
import { useProjectStore } from '../../stores/projectStore';
import { showGlobalError, showGlobalSuccess, showGlobalWarning } from '../../stores/toastStore';
import { homeRoute, projectRoute } from '../workspace/routes';
import { cn } from '../../lib/utils';
import { applySavedSettings, makeDraft, modelChanged, prepareModelSave, savedSnapshot, settingsPayload, validation, type Draft, type DraftModel, type Editable } from './modelSettingsDraft';
import './settings.css';

const providerNames: Record<string, string> = { openai: 'OpenAI', deepseek: 'DeepSeek', kimi: 'Kimi' };
const purposeLabels: Record<SidePurpose, [string, string]> = {
  rename: ['会话命名', '自动命名与立即命名'], compact: ['上下文压缩', '自动压缩与手动压缩'],
  commit: ['提交说明', '生成 Git 提交标题与摘要'], polish: ['输入润色', '整理并润色当前输入'],
  handoff: ['交接内容', '生成交接说明及后续修订'], kickoff: ['启动说明', '生成开发启动说明及后续修订'],
};
function ModelCard({ row, edit, open, providers, busy, saving, changed, error, onOpen, onEdit, onSave, onCancel, onDelete }: {
  row: DraftModel; edit: Editable; open: boolean; providers: string[]; busy: boolean; saving: boolean; changed: boolean; error?: string;
  onOpen: () => void; onEdit: (edit: Editable) => void; onSave: () => void; onCancel: () => void; onDelete: () => void;
}) {
  const [showKey, setShowKey] = useState(false);
  const front = useRef<HTMLButtonElement>(null);
  const back = useRef<HTMLFormElement>(null);
  useEffect(() => {
    const form = back.current;
    if (!open) {
      if (form?.contains(document.activeElement)) front.current?.focus();
      if (form) form.inert = true;
      setShowKey(false);
      return;
    }
    if (form) { form.inert = false; form.focus(); }
    const timer = window.setTimeout(() => {
      if (document.activeElement === form) form?.querySelector<HTMLButtonElement>('[data-provider-select]')?.focus();
    }, 350);
    return () => window.clearTimeout(timer);
  }, [open]);
  const update = (field: keyof Editable, value: string) => onEdit({ ...edit, [field]: value });
  return (
    <article className="settings-card" data-model-card={row.id} data-open={open}>
      <div className="settings-card-turn">
        <button ref={front} type="button" className="settings-card-face settings-card-front" disabled={busy}
          aria-expanded={open} aria-controls={`model-${row.id}`} tabIndex={open ? -1 : 0} aria-hidden={open}
          aria-label={`编辑 ${row.name || '新模型'}`} onClick={onOpen}>
          <img src={`/model-logos/${row.provider || 'openai'}.svg`} alt="" width={60} height={60} />
          <span>{row.name || '新模型'}</span>
          {changed && <small className="text-xs text-text-500">未保存</small>}
        </button>
        <form ref={back} id={`model-${row.id}`} className="settings-card-face settings-card-back" tabIndex={-1} aria-hidden={!open}
          aria-label={`编辑 ${row.name || '新模型'}`} aria-describedby={error ? `error-${row.id}` : undefined}
          onSubmit={(event) => { event.preventDefault(); onSave(); }} autoComplete="off" noValidate>
          <fieldset disabled={busy}>
            <label><span>provider</span><Select
              data-provider-select
              aria-label="供应商"
              ownerId={row.id}
              value={edit.provider}
              onValueChange={(value) => update('provider', value)}
              options={providers.map((provider) => ({ value: provider, label: providerNames[provider] ?? provider }))}
              disabled={busy}
              className="text-xs"
            /></label>
            <label><span>name</span><input value={edit.name} onChange={(event) => update('name', event.target.value)} maxLength={80} placeholder="配置名称" /></label>
            <label><span>model</span><input value={edit.model} onChange={(event) => update('model', event.target.value)} spellCheck={false} autoCapitalize="off" placeholder="模型标识" /></label>
            <label><span>key</span><span className="settings-key">
              <input type={showKey ? 'text' : 'password'} value={edit.key} onChange={(event) => update('key', event.target.value)}
                spellCheck={false} autoCapitalize="off" placeholder={row.has_key && edit.provider === row.originalProvider ? '已配置，留空保留' : '输入 API key'} autoComplete="new-password" />
              <button type="button" onClick={() => setShowKey(!showKey)} aria-label={showKey ? '隐藏密钥' : '显示密钥'} aria-pressed={showKey}>
                {showKey ? <EyeOff size={15} /> : <Eye size={15} />}
              </button>
            </span></label>
          </fieldset>
          <div className="settings-card-actions">
            <p id={`error-${row.id}`} className="settings-card-error" role="alert">{error}</p>
            <div className="flex items-center gap-2">
              <button type="submit" className="settings-primary flex flex-1 items-center justify-center gap-1.5" disabled={busy}>{saving && <Loader2 size={14} className="animate-spin" />}{saving ? '保存中…' : '保存'}</button>
              <button type="button" onClick={onCancel} disabled={busy} className="h-8 rounded-md px-2 text-xs text-text-600 hover:bg-panel-muted">取消</button>
              <IconButton label="删除模型" type="button" title="删除模型" aria-label={`删除 ${row.name || '新模型'}`} onClick={onDelete} disabled={busy} className="text-danger hover:bg-danger-soft"><Trash2 size={16} /></IconButton>
            </div>
          </div>
        </form>
      </div>
    </article>
  );
}

export function SettingsPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const activeProjectId = useProjectStore((state) => state.activeProjectId);
  const returnTo = typeof location.state?.returnTo === 'string' && !location.state.returnTo.startsWith('/settings')
    ? location.state.returnTo : activeProjectId ? projectRoute(activeProjectId) : homeRoute;
  const [section, setSection] = useState<'models' | 'routing' | 'shortcuts'>('models');
  const [shortcutDirty, setShortcutDirty] = useState(false);
  const [shortcutSaving, setShortcutSaving] = useState(false);
  const [shortcutRefresh, setShortcutRefresh] = useState(0);
  const [shortcutsVisited, setShortcutsVisited] = useState(false);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [edits, setEdits] = useState<Record<string, Editable>>({});
  const [openCard, setOpenCard] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [savingModelId, setSavingModelId] = useState<string | null>(null);
  const saveInFlight = useRef(false);
  const [error, setError] = useState('');
  const [cardErrors, setCardErrors] = useState<Record<string, string>>({});
  const [deleting, setDeleting] = useState<DraftModel | null>(null);
  const [reloadPrompt, setReloadPrompt] = useState(false);
  const grid = useRef<HTMLDivElement>(null);
  const leaving = useRef(false);
  const dirty = shortcutDirty || !!draft?.llm.some((row) => modelChanged(row, edits[row.id]));
  const blocker = useBlocker(({ currentLocation, nextLocation }) => (dirty || saving || shortcutSaving) && currentLocation.pathname !== nextLocation.pathname);
  useEffect(() => {
    if (blocker.state === 'blocked' && !dirty && !saving && !shortcutSaving) blocker.proceed();
  }, [blocker, dirty, saving, shortcutSaving]);
  const load = useCallback(async () => {
    setLoading(true); setError('');
    try {
      const value = makeDraft(await settingsApi.reload());
      setDraft(value); setEdits({}); setCardErrors({}); setOpenCard(null);
      useComposerStore.getState().reconcileModels(value.llm.map((row) => row.name), value.main_road.default);
      window.dispatchEvent(new Event('model-settings-saved'));
    } catch (cause) { setError(cause instanceof Error ? cause.message : '模型设置加载失败。'); }
    finally { setLoading(false); }
  }, []);
  useEffect(() => { void load(); }, [load]);
  useEffect(() => {
    const unload = (event: BeforeUnloadEvent) => { if (dirty || saving || shortcutSaving) { event.preventDefault(); event.returnValue = ''; } };
    window.addEventListener('beforeunload', unload);
    return () => window.removeEventListener('beforeunload', unload);
  }, [dirty, saving, shortcutSaving]);
  useEffect(() => {
    const close = (event: PointerEvent) => {
      if (saveInFlight.current) return;
      const target = event.target as Element;
      if (openCard && target.closest('[data-select-owner]')?.getAttribute('data-select-owner') === openCard) return;
      if (openCard && target.closest('[data-model-card]')?.getAttribute('data-model-card') !== openCard) setOpenCard(null);
    };
    const escape = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && !event.defaultPrevented && !saveInFlight.current && openCard) {
        const button = grid.current?.querySelector<HTMLButtonElement>(`[data-model-card="${openCard}"] .settings-card-front`);
        setOpenCard(null); button?.focus();
      }
    };
    document.addEventListener('pointerdown', close); document.addEventListener('keydown', escape);
    return () => { document.removeEventListener('pointerdown', close); document.removeEventListener('keydown', escape); };
  }, [openCard]);
  async function persist(next: Draft, message: string, completedId?: string): Promise<boolean> {
    if (saveInFlight.current) return false;
    saveInFlight.current = true;
    setSaving(true); setSavingModelId(completedId ?? null); setError('');
    try {
      const result = await settingsApi.save(settingsPayload(next));
      const selected = useComposerStore.getState();
      const renamed = next.llm.find((row) => row.previous_name === selected.modelProfileName);
      if (selected.modelSelectionExplicit && renamed && renamed.name !== renamed.previous_name) selected.setModelProfileName(renamed.name);
      useComposerStore.getState().reconcileModels(result.llm.map((row) => row.name), result.main_road.default);
      setDraft((current) => applySavedSettings(result, next, current ?? next));
      if (completedId) {
        setEdits((current) => { const rest = { ...current }; delete rest[completedId]; return rest; });
        setCardErrors((current) => { const rest = { ...current }; delete rest[completedId]; return rest; });
        setOpenCard((current) => current === completedId ? null : current);
      }
      window.dispatchEvent(new Event('model-settings-saved'));
      showGlobalSuccess(message);
      return true;
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : '保存失败，请稍后重试。';
      setError(message);
      showGlobalError(message);
      if (completedId) setCardErrors((current) => ({ ...current, [completedId]: message }));
      return false;
    } finally {
      saveInFlight.current = false;
      setSaving(false); setSavingModelId(null);
    }
  }

  async function saveCard(row: DraftModel) {
    if (!draft || saveInFlight.current || loading) return;
    const edit = edits[row.id] ?? row;
    // Validate before trimming so pasted line breaks cannot be silently accepted.
    const message = validation({ ...row, ...edit }, savedSnapshot(draft).llm);
    if (message) { setCardErrors((current) => ({ ...current, [row.id]: message })); return; }
    const next = prepareModelSave(draft, row, edit);
    await persist(next, '模型已保存，新任务将使用新配置。', row.id);
  }

  async function saveRouting(road: 'main_road' | 'side_road', key: 'default' | 'fallback' | SidePurpose, value: string) {
    if (!draft || saveInFlight.current || loading) return;
    if (road === 'main_road' && key !== 'default' && key !== 'fallback') return;
    const previous = draft;
    const next = savedSnapshot(draft);
    const current = key === 'default' || key === 'fallback' ? draft[road][key] : draft.side_road[key];
    if ((current ?? '') === value) return;
    if (key === 'default' && !value) return;
    if (value && !next.llm.some((row) => row.name === value)) return;
    if (road === 'main_road' && (key === 'default' || key === 'fallback')) {
      next.main_road = { ...next.main_road, [key]: value };
    } else {
      next.side_road = { ...next.side_road, [key]: value };
    }
    setDraft({ ...draft, [road]: next[road] });
    if (!await persist(next, '模型分配已保存。')) {
      setDraft((current) => current ? { ...current, main_road: previous.main_road, side_road: previous.side_road } : current);
    }
  }

  function cancelCard(row: DraftModel) {
    if (saveInFlight.current) return;
    if (!row.previous_name) setDraft((current) => current ? { ...current, llm: current.llm.filter((item) => item.id !== row.id) } : current);
    setEdits((current) => { const rest = { ...current }; delete rest[row.id]; return rest; });
    setCardErrors((current) => { const rest = { ...current }; delete rest[row.id]; return rest; });
    setOpenCard(null);
  }

  function addModel() {
    if (!draft || saveInFlight.current || loading) return;
    const row: DraftModel = { id: crypto.randomUUID(), name: '', provider: draft.providers[0], model: '', key: '', has_key: false, originalProvider: '' };
    setDraft({ ...draft, llm: [...draft.llm, row] }); setOpenCard(row.id);
  }

  function removeModel(row: DraftModel) {
    if (!draft || saveInFlight.current) return;
    if (!row.previous_name) { cancelCard(row); return; }
    if ([...Object.values(draft.main_road), ...Object.values(draft.side_road)].includes(row.previous_name)) {
      showGlobalWarning('此模型正在被使用，请先到“模型分配”解除引用。'); return;
    }
    if (savedSnapshot(draft).llm.length === 1) { showGlobalWarning('至少保留一个模型。'); return; }
    setDeleting(row);
  }

  async function confirmDelete() {
    if (!draft || !deleting || saveInFlight.current) return;
    const next = savedSnapshot(draft);
    next.llm = next.llm.filter((row) => row.id !== deleting.id);
    if (!await persist(next, '模型已删除。', deleting.id)) {
      // Keep the confirmation open for retry; the original model remains intact.
      throw new Error('模型删除失败');
    }
  }

  const refreshSettings = () => section === 'shortcuts' ? setShortcutRefresh(value => value + 1) : void load();

  const modelSelect = (value: string | null, change: (value: string) => void, label: string, empty?: string) => (
    <Select
      className="settings-value"
      aria-label={label}
      value={value ?? ''}
      onValueChange={change}
      placeholder="请选择模型"
      disabled={saving}
      options={[
        ...(empty ? [{ value: '', label: empty }] : []),
        ...(draft?.llm.filter((row) => row.previous_name).map((row) => ({ value: row.name, label: row.name })) ?? []),
      ]}
    />
  );
  return (
    <main className="model-settings flex h-[100dvh] min-h-0 flex-col overflow-hidden bg-workspace text-text-900">
      <header className="flex h-12 shrink-0 items-center border-b border-border-strong bg-surface px-2">
        <button type="button" onClick={() => navigate(returnTo)} className="flex items-center gap-2 rounded-md px-2 font-bold hover:bg-panel-muted" aria-label="返回项目">
          <img src="/logo.jpg" alt="" className="h-10 w-10 rounded-sm object-cover" /><span className="hidden sm:inline">Dasi PPT Agent</span>
        </button>
        <span className="mx-2 h-5 w-px bg-border" aria-hidden="true" /><span className="flex-1 font-bold">设置</span>
        <IconButton label="刷新设置" expandableLabel="刷新" disabled={(section !== 'shortcuts' && loading) || saving || shortcutSaving} onClick={() => dirty ? setReloadPrompt(true) : refreshSettings()}><RefreshCw size={16} className={loading ? 'animate-spin motion-reduce:animate-none' : undefined} /></IconButton>
        <IconButton label="返回主页" expandableLabel="主页" onClick={() => navigate(returnTo)} disabled={saving || shortcutSaving} className="ml-1"><HomeLogo /></IconButton>
      </header>
      <div className="flex min-h-0 flex-1 flex-col md:flex-row">
        <nav className="flex shrink-0 gap-1 border-b border-border bg-panel p-3 md:w-52 md:flex-col md:border-b-0 md:border-r" aria-label="设置栏目">
          {([['models', '模型配置', Cpu], ['routing', '模型分配', Route], ['shortcuts', '快捷键', Keyboard]] as const).map(([id, label, Icon]) => (
            <button key={id} type="button" disabled={saving || shortcutSaving} aria-current={section === id ? 'page' : undefined} onClick={() => { setSection(id); if (id === 'shortcuts') setShortcutsVisited(true); setOpenCard(null); }}
              className={cn('flex h-10 flex-1 items-center gap-2.5 rounded-lg px-3 text-sm md:flex-none', section === id ? 'bg-accent-soft font-semibold text-accent' : 'text-text-600 hover:bg-panel-muted')}><Icon size={17} />{label}</button>
          ))}
        </nav>
        <section className="min-h-0 min-w-0 flex-1 overflow-y-auto">
          <div className="settings-content">
            {section !== 'shortcuts' && <div className="settings-heading">
              <h1>{section === 'models' ? '模型配置' : '模型分配'}</h1>
              <span role="status" aria-live="polite" className="flex items-center gap-2 text-xs text-text-600">{(saving || shortcutSaving) && <><Loader2 size={14} className="animate-spin" />保存中…</>}</span>
            </div>}
            <div hidden={section !== 'shortcuts'}>{shortcutsVisited && <ShortcutSettingsPanel refreshKey={shortcutRefresh} onDirtyChange={setShortcutDirty} onSavingChange={setShortcutSaving} />}</div>
            <div hidden={section === 'shortcuts'}>
            {error && <div className="mb-5 rounded-lg bg-danger-soft px-4 py-3 text-sm text-danger" role="alert">{error}</div>}
            {loading ? <div className="flex items-center gap-2 py-16 text-sm text-text-600"><Loader2 size={17} className="animate-spin" />正在读取设置…</div> : draft && (
              section === 'models' ? <>
                <div className="mb-4 flex justify-end"><button type="button" className="flex items-center gap-1.5 rounded-md px-2 py-1.5 text-sm text-accent hover:bg-accent-soft" disabled={saving} onClick={addModel}><Plus size={16} />添加模型</button></div>
                <div ref={grid} className="settings-grid">
                  {draft.llm.map((row) => <ModelCard key={row.id} row={row} edit={edits[row.id] ?? row} open={openCard === row.id} providers={draft.providers} busy={saving} saving={savingModelId === row.id} changed={modelChanged(row, edits[row.id])} error={cardErrors[row.id]}
                    onOpen={() => setOpenCard(row.id)} onEdit={(edit) => { setEdits((current) => ({ ...current, [row.id]: edit })); setCardErrors((current) => ({ ...current, [row.id]: '' })); }}
                    onSave={() => void saveCard(row)} onCancel={() => cancelCard(row)} onDelete={() => removeModel(row)} />)}
                </div>
              </> : <div className="settings-routing">
                <h2>主路</h2>
                <div className="settings-group">
                  <div className="settings-row"><div><h3>默认模型</h3><p>聊天区没有手动选择时使用。</p></div>{modelSelect(draft.main_road.default, (value) => { void saveRouting('main_road', 'default', value); }, '主路默认模型')}</div>
                  <div className="settings-row"><div><h3>备用模型</h3><p>原模型重试失败后接替，本轮持续使用。</p></div>{modelSelect(draft.main_road.fallback, (value) => { void saveRouting('main_road', 'fallback', value); }, '主路备用模型', '不启用备用模型')}</div>
                </div>
                <h2>旁路</h2>
                <div className="settings-group">
                  <div className="settings-row"><div><h3>默认模型</h3><p>未单独指定模型的旁路使用此配置。</p></div>{modelSelect(draft.side_road.default, (value) => { void saveRouting('side_road', 'default', value); }, '旁路默认模型')}</div>
                  <div className="settings-row"><div><h3>备用模型</h3><p>具体用途模型或默认模型重试失败后接替。</p></div>{modelSelect(draft.side_road.fallback, (value) => { void saveRouting('side_road', 'fallback', value); }, '旁路备用模型', '不启用备用模型')}</div>
                  {SIDE_PURPOSES.map((purpose) => <div key={purpose} className="settings-row"><div><h3>{purposeLabels[purpose][0]}</h3><p>{purposeLabels[purpose][1]}</p></div>{modelSelect(draft.side_road[purpose], (value) => { void saveRouting('side_road', purpose, value); }, `${purposeLabels[purpose][0]}模型`, '使用旁路默认模型')}</div>)}
                </div>
              </div>
            )}
            </div>
          </div>
        </section>
      </div>
      <ConfirmModal open={blocker.state === 'blocked' && !saving && !shortcutSaving} onOpenChange={(open) => { if (!open && !leaving.current && blocker.state === 'blocked') blocker.reset(); }} title="离开设置？" description="还有未保存的修改，离开后将丢弃这些修改。" confirmLabel="丢弃并离开" cancelLabel="继续编辑" onConfirm={() => { if (blocker.state === 'blocked') { leaving.current = true; blocker.proceed(); } }} />
      <ConfirmModal open={reloadPrompt} onOpenChange={setReloadPrompt} title="重新读取设置？" description="重新读取会丢弃当前页面的未保存修改。" confirmLabel="重新读取" cancelLabel="继续编辑" onConfirm={refreshSettings} />
      <ConfirmModal open={!!deleting} onOpenChange={(open) => { if (!open && !saveInFlight.current) setDeleting(null); }} title="删除模型？" description={`删除「${deleting?.name || '新模型'}」后立即生效。`} confirmLabel="删除" onConfirm={confirmDelete} />
    </main>
  );
}
