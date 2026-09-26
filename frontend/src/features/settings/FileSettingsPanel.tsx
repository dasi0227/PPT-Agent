import { useEffect, useRef, useState } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { filesApi, fileOpenOptions, type FileOpenWith, type FileSettings } from '../../api/files';
import { Select } from '../../components/ui/select';
import { useFileSettingsStore } from '../../stores/fileSettingsStore';

export function FileSettingsPanel({ refreshKey, onSavingChange }: {
  refreshKey: number; onSavingChange: (saving: boolean) => void;
}) {
  const value = useFileSettingsStore(state => state.value);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const inFlight = useRef(false);
  useEffect(() => {
    let active = true;
    setLoading(true); setError('');
    void useFileSettingsStore.getState().load()
      .catch(cause => { if (active) setError(cause instanceof Error ? cause.message : '文件设置读取失败'); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [refreshKey]);
  const update = async (prepare: (base: FileSettings) => Promise<FileSettings | null>) => {
    if (!value || inFlight.current) return;
    inFlight.current = true; setBusy(true); onSavingChange(true); setError('');
    try {
      const edit = await prepare({ open_with: value.open_with, custom_app_path: value.custom_app_path,
        custom_apps: value.custom_apps, revision: value.revision });
      if (!edit) return;
      await useFileSettingsStore.getState().save(edit);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '文件设置保存失败，原设置仍然有效');
    } finally { inFlight.current = false; setBusy(false); onSavingChange(false); }
  };
  const add = () => update(async base => {
    const { application } = await filesApi.pickApplication();
    if (!application) return null;
    if (base.custom_apps.some(app => app.path === application.path)) return null;
    return { ...base, custom_apps: [...base.custom_apps, application] };
  });
  const remove = (path: string) => update(async base => ({ ...base,
    custom_apps: base.custom_apps.filter(app => app.path !== path),
    ...(base.open_with === 'custom' && base.custom_app_path === path ? { open_with: 'system' as const, custom_app_path: '' } : {}),
  }));
  const change = (choice: string) => update(async base => ({ ...base,
    open_with: choice.startsWith('app:') ? 'custom' : choice as FileOpenWith,
    custom_app_path: choice.startsWith('app:') ? choice.slice(4) : '',
  }));
  const options = [...fileOpenOptions, ...(value?.custom_apps ?? []).map(app => ({ value: `app:${app.path}`, label: app.name }))];
  return <>
    <div className="settings-heading"><h1>文件</h1></div>
    {error && <p role="alert" className="mb-5 text-sm text-danger">{error}，可点击顶部刷新后重试。</p>}
    {loading ? <p className="py-16 text-sm text-text-600">正在读取文件设置…</p> : value && <div className="settings-routing">
      <div className="mb-3 flex items-center justify-between gap-4">
        <h2 className="!mb-0">打开文件</h2>
        <button type="button" className="settings-primary ui-primary inline-flex shrink-0 items-center gap-1.5" disabled={busy || !value.supported} onClick={() => void add()}>
          <Plus size={15} aria-hidden="true" />添加应用
        </button>
      </div>
      <div className="settings-group">
        <div className="settings-row"><div><h3>默认打开方式</h3><p>点击文件旁的打开图标时使用，适用于所有项目。选择后自动保存。</p></div>
          <Select aria-label="默认打开方式" className="settings-value" value={value.open_with === 'custom' ? `app:${value.custom_app_path}` : value.open_with} options={options}
            disabled={busy || !value.supported} onValueChange={choice => void change(choice)} />
        </div>
        {value.custom_apps.map(app => <div key={app.path} className="settings-row"><div className="min-w-0"><h3>{app.name}</h3><p className="break-all">{app.path}</p></div>
          <button type="button" aria-label={`删除应用 ${app.name}`} className="inline-flex min-h-9 shrink-0 items-center gap-1.5 rounded-md px-3 text-[13px] text-danger ui-danger disabled:cursor-not-allowed disabled:opacity-50"
            disabled={busy || !value.supported} onClick={() => void remove(app.path)}><Trash2 size={15} aria-hidden="true" />删除应用</button>
        </div>)}
      </div>
      {!value.supported && <p className="mt-4 text-sm text-text-600">当前仅支持在 macOS 本机打开文件。</p>}
    </div>}
  </>;
}
