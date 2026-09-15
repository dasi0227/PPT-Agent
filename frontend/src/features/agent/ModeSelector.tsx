import React from 'react';
import { Check } from 'lucide-react';
import type { RunMode } from '../../api/types';
import { cn } from '../../lib/utils';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '../../components/ui/dropdown-menu';
import { MODE_META, MODE_ORDER } from './modeMeta';

interface ModeSelectorProps {
  mode: RunMode;
  disabled?: boolean;
  onChange: (mode: RunMode) => void;
}

export const ModeSelector: React.FC<ModeSelectorProps> = ({
  mode,
  disabled = false,
  onChange,
}) => {
  const CurrentIcon = MODE_META[mode].icon;
  const title = `${MODE_META[mode].label} · ${MODE_META[mode].description}`;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={`交互方式：${MODE_META[mode].label}`}
          title={title}
          disabled={disabled}
          className="composer-mode-button inline-flex h-7 shrink-0 items-center gap-0.5 rounded-md border border-border bg-transparent px-1.5 text-[11px] font-medium text-text-600 transition-colors hover:bg-panel-muted hover:text-text-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:cursor-not-allowed disabled:opacity-45"
        >
          <CurrentIcon className="h-3.5 w-3.5 shrink-0" strokeWidth={1.75} />
          <span className="composer-mode-label shrink-0 whitespace-nowrap">{MODE_META[mode].label}</span>
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent side="top" align="start" className="w-[224px] p-1">
        {MODE_ORDER.map((candidate) => {
          const meta = MODE_META[candidate];
          const Icon = meta.icon;
          const active = candidate === mode;
          return (
            <DropdownMenuItem
              key={candidate}
              aria-label={`交互方式：${meta.label}`}
              onSelect={() => onChange(candidate)}
              className={cn(
                'flex items-start gap-2.5 rounded-sm px-2 py-1.5',
                active ? 'bg-accent-soft/70' : 'hover:bg-panel-muted',
              )}
            >
              <Icon
                className={cn('mt-0.5 h-3.5 w-3.5 shrink-0', active ? 'text-accent' : 'text-text-600')}
                strokeWidth={1.75}
              />
              <span className="min-w-0 flex-1">
                <span className={cn('block text-xs font-semibold', active ? 'text-accent' : 'text-text-900')}>
                  {meta.label}
                </span>
                <span className="block text-[11px] leading-4 text-text-400">{meta.description}</span>
              </span>
              {active && <Check className="mt-1 h-3.5 w-3.5 shrink-0 text-accent" strokeWidth={2.5} />}
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
};
