import { useEffect, useRef, useState } from 'react';
import { Loader2 } from 'lucide-react';
import type { AgentSettings } from '../../api/settings';
import { Skeleton } from '../../components/ui/primitives';
import { Switch } from '../../components/ui/switch';
import { useAgentSettingsStore } from '../../stores/agentSettingsStore';

const fields = [
  ['show_tool_failures', '显示工具调用失败', '开启后，立即显示每次工具调用失败的原因。'],
  ['require_resource_approval', '资源编辑审批', '开启后，修改内容要求、视觉要求和目录结构需手动审批。'],
] as const;

export function AgentSettingsPanel({ refreshKey, onSavingChange }: {
  refreshKey: number; onSavingChange: (saving: boolean) => void;
}) {
  const value = useAgentSettingsStore(state => state.value);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const inFlight = useRef(false);
  useEffect(() => {
    let active = true;
    setLoading(true); setError('');
    void useAgentSettingsStore.getState().load(refreshKey > 0)
      .catch(cause => { if (active) setError(cause instanceof Error ? cause.message : '智能体设置读取失败'); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [refreshKey]);
  const change = async (key: Exclude<keyof AgentSettings, 'revision'>, checked: boolean) => {
    if (!value || inFlight.current) return;
    inFlight.current = true; setBusy(true); onSavingChange(true); setError('');
    try {
      await useAgentSettingsStore.getState().save({ ...value, [key]: checked });
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '智能体设置保存失败');
      await useAgentSettingsStore.getState().load().catch(() => {});
    } finally { inFlight.current = false; setBusy(false); onSavingChange(false); }
  };
  return <>
    <div className="settings-heading"><h1>智能体</h1>
      <span role="status" aria-live="polite" className="flex items-center gap-2 text-xs text-text-600">
        {busy && <><Loader2 size={14} className="animate-spin motion-reduce:animate-none" />保存中…</>}
      </span>
    </div>
    {error && <p role="alert" className="mb-5 text-sm text-danger">{error}</p>}
    <div className="settings-routing"><div className="settings-group">
      {fields.map(([key, label, desc]) => <div className="settings-row" key={key}>
        <div>
          <h3 id={`agent-${key}`}>{label}</h3>
          <p id={`agent-${key}-desc`}>{desc}</p>
        </div>
        <div className="settings-value flex justify-end">
          {loading && !value ? <Skeleton className="h-5 w-9 rounded-full" /> : <Switch
            aria-labelledby={`agent-${key}`} aria-describedby={`agent-${key}-desc`}
            checked={value?.[key] ?? (key === 'require_resource_approval')}
            disabled={loading || busy || !value} onCheckedChange={checked => void change(key, checked)} />}
        </div>
      </div>)}
    </div></div>
  </>;
}
