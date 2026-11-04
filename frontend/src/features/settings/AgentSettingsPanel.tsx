import { useEffect, useRef, useState } from 'react';
import type { AgentSettings } from '../../api/settings';
import { Skeleton } from '../../components/ui/primitives';
import { Switch } from '../../components/ui/switch';
import { useAgentSettingsStore } from '../../stores/agentSettingsStore';

const fields: ReadonlyArray<readonly [Exclude<keyof AgentSettings, 'revision'>, string, string?]> = [
  ['auto_rename_enabled', '自动命名', '开启后，根据会话内容自动生成和更新会话名称。'],
  ['auto_suggest_enabled', '下一步推荐', '开启后，根据当前会话和项目进展自动推荐下一步操作。'],
  ['show_tool_failures', '工具调用失败显示', '开启后，立即显示每次工具调用失败的原因。'],
  ['require_resource_approval', '全局参考编辑审批', '开启后，修改基本信息、全局约束和目录结构需手动审批。'],
  ['require_high_risk_approval', '高风险操作审批', '开启后，扩大修改范围需手动审批。'],
];

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
    <div className="settings-heading"><h1>智能体</h1></div>
    {error && <p role="alert" className="mb-5 text-sm text-danger">{error}</p>}
    <div className="settings-routing"><div className="settings-group">
      {fields.map(([key, label, desc]) => <div className="settings-row" key={key}>
        <div>
          <h3 id={`agent-${key}`}>{label}</h3>
          {desc && <p id={`agent-${key}-desc`}>{desc}</p>}
        </div>
        <div className="settings-value flex justify-end">
          {loading && !value ? <Skeleton className="h-5 w-9 rounded-full" /> : <Switch
            aria-labelledby={`agent-${key}`} aria-describedby={desc ? `agent-${key}-desc` : undefined}
            checked={value?.[key] ?? (key !== 'show_tool_failures')}
            disabled={loading || busy || !value} onCheckedChange={checked => void change(key, checked)} />}
        </div>
      </div>)}
    </div></div>
  </>;
}
