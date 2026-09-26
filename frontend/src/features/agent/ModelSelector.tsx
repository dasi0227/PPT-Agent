import React from 'react';
import { Check } from 'lucide-react';
import { ModelProviderIcon } from '../../components/ui/ModelProviderIcon';
import type { LLMProfile } from '../../api/types';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
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
  const triggerLabel = loading
    ? '加载模型…'
    : selected
      ? selected.name
      : profiles.length === 0
        ? '暂无可用模型'
        : '选择模型';
  const title = selected
      ? `${selected.name} · ${selected.model}${selected.capabilities.vision ? ' · 支持页面观察' : ''}`
      : '选择本次任务使用的模型';

  return (
    <div className="min-w-0 shrink-0" title={title}>
      <DropdownMenu>
        <DropdownMenuTrigger asChild disabled={disabled || loading || profiles.length === 0}>
          <button
            type="button"
            aria-label="模型"
            disabled={disabled || loading || profiles.length === 0}
            className="composer-model-button inline-flex h-7 min-w-0 max-w-[176px] shrink-0 items-center gap-1 rounded-md border border-transparent bg-transparent px-2 text-[11px] font-medium text-text-600 transition-colors ui-interactive focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-45"
          >
            <ModelProviderIcon provider={selected?.provider} className="h-3.5 w-3.5 shrink-0 object-contain" />
            <span className="min-w-0 truncate">{triggerLabel}</span>
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent side="top" align="end" className="max-h-64 w-[196px] overflow-y-auto p-1">
          <DropdownMenuLabel className="px-2 pb-2 text-[11px] font-normal text-text-600">模型选择</DropdownMenuLabel>
          {profiles.map((profile) => {
            const optionDisabled = requiresVision && !profile.capabilities.vision;
            const active = profile.name === value;
            return (
              <DropdownMenuItem
                key={profile.name}
                role="menuitemradio"
                aria-checked={active}
                aria-label={`模型：${profile.name}`}
                disabled={optionDisabled}
                onSelect={() => onChange(profile.name)}
                className={[
                  'flex items-center gap-2 rounded-sm px-2 py-1.5 text-xs',
                  active ? 'ui-selected' : 'text-text-600',
                  optionDisabled ? 'cursor-not-allowed opacity-45' : '',
                ].join(' ')}
              >
                <ModelProviderIcon provider={profile.provider} className="h-4 w-4 shrink-0 object-contain" />
                <span className="min-w-0 flex-1 truncate">{profile.name}</span>
                {active && <Check className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />}
              </DropdownMenuItem>
            );
          })}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
};
