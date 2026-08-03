import React from 'react';
import { Eye, Waypoints } from 'lucide-react';
import type { LLMProfile } from '../../api/types';

interface ModelSelectorProps {
  profiles: LLMProfile[];
  value: string | null;
  requiresVision: boolean;
  loading: boolean;
  disabled?: boolean;
  onChange: (name: string) => void;
}

export const ModelSelector: React.FC<ModelSelectorProps> = ({
  profiles,
  value,
  requiresVision,
  loading,
  disabled = false,
  onChange,
}) => {
  const selected = profiles.find((profile) => profile.name === value);
  const mismatch = Boolean(selected && requiresVision && !selected.capabilities.vision);
  const title = mismatch
    ? '当前任务需要页面图片观察，请选择标记为支持页面观察的模型'
    : selected
      ? `${selected.name} · ${selected.model}${selected.capabilities.vision ? ' · 支持页面观察' : ''}`
      : '选择本次任务使用的模型';

  return (
    <div className="min-w-0" title={title}>
      <div className="flex min-w-0 items-center gap-1">
        <Waypoints className="h-3.5 w-3.5 shrink-0 text-text-500" strokeWidth={1.75} />
        <select
          aria-label="模型"
          value={value ?? ''}
          disabled={disabled || loading || profiles.length === 0}
          onChange={(event) => onChange(event.target.value)}
          className="h-7 max-w-[152px] min-w-0 rounded-md border border-transparent bg-transparent px-1 text-xs text-text-700 outline-none hover:bg-surface focus-visible:border-accent disabled:cursor-not-allowed disabled:opacity-50"
        >
          {loading && <option value="">加载模型…</option>}
          {!loading && profiles.length === 0 && <option value="">暂无可用模型</option>}
          {profiles.map((profile) => (
            <option
              key={profile.name}
              value={profile.name}
              disabled={requiresVision && !profile.capabilities.vision}
            >
              {profile.name} · {profile.model}
              {profile.capabilities.vision ? ' · 支持页面观察' : requiresVision ? ' · 不支持页面观察' : ''}
            </option>
          ))}
        </select>
        {selected?.capabilities.vision && (
          <Eye aria-label="支持页面观察" className="h-3.5 w-3.5 shrink-0 text-accent" strokeWidth={1.75} />
        )}
      </div>
      {mismatch && (
        <p className="mt-0.5 max-w-[190px] text-[10px] leading-4 text-danger">
          此模型不支持页面观察
        </p>
      )}
    </div>
  );
};
