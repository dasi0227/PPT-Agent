import React from 'react';
import { ChevronDown } from 'lucide-react';
import type { LLMProfile } from '../../api/types';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '../../components/ui/dropdown-menu';

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
  const triggerLabel = loading
    ? '加载模型…'
    : selected
      ? selected.name
      : profiles.length === 0
        ? '暂无可用模型'
        : '选择模型';
  const title = mismatch
    ? '当前任务需要页面图片观察，请选择标记为支持页面观察的模型'
    : selected
      ? `${selected.name} · ${selected.model}${selected.capabilities.vision ? ' · 支持页面观察' : ''}`
      : '选择本次任务使用的模型';

  return (
    <div className="min-w-0" title={title}>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            aria-label="模型"
            disabled={disabled || loading || profiles.length === 0}
            className="inline-flex h-7 min-w-0 max-w-[152px] shrink items-center gap-0.5 rounded-md border border-border bg-transparent px-1 text-[11px] font-medium text-text-600 transition-colors hover:bg-panel-muted hover:text-text-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:cursor-not-allowed disabled:opacity-45"
          >
            <span className="min-w-0 truncate">{triggerLabel}</span>
            <ChevronDown className="h-3.5 w-3.5 shrink-0" strokeWidth={1.75} />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent side="top" align="end" className="max-h-64 w-[196px] overflow-y-auto p-1">
          {profiles.map((profile) => {
            const optionDisabled = requiresVision && !profile.capabilities.vision;
            const active = profile.name === value;
            return (
              <DropdownMenuItem
                key={profile.name}
                aria-label={`模型：${profile.name}`}
                disabled={optionDisabled}
                onSelect={() => onChange(profile.name)}
                className={[
                  'flex items-center gap-2 rounded-sm px-2 py-1.5 text-xs',
                  active ? 'bg-panel-muted text-text-900' : 'text-text-600',
                  optionDisabled ? 'cursor-not-allowed opacity-45' : '',
                ].join(' ')}
              >
                <span className="min-w-0 flex-1 truncate">{profile.name}</span>
              </DropdownMenuItem>
            );
          })}
        </DropdownMenuContent>
      </DropdownMenu>
      {mismatch && (
        <p className="mt-0.5 max-w-[190px] text-[10px] leading-4 text-danger">
          此模型不支持页面观察
        </p>
      )}
    </div>
  );
};
