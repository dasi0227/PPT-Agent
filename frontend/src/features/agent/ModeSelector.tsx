import React from 'react';
import { Check, ChevronDown } from 'lucide-react';
import type { RunMode } from '../../api/types';
import { cn } from '../../lib/utils';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
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
      <DropdownMenuTrigger asChild disabled={disabled}>
        <button
          type="button"
          aria-label={`交互方式：${MODE_META[mode].label}`}
          title={title}
          disabled={disabled}
          className="composer-context-trigger ui-interactive composer-mode-button"
        >
          <CurrentIcon className="h-4 w-4 shrink-0 text-accent" strokeWidth={1.75} />
          <span className="font-medium">{MODE_META[mode].label}</span>
          <ChevronDown className="composer-context-chevron" strokeWidth={1.75} aria-hidden="true" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent side="top" align="start" sideOffset={8} avoidCollisions collisionPadding={12} className="w-[304px] rounded-xl p-1.5">
        <DropdownMenuLabel className="px-2.5 pb-2 text-[11px] font-normal text-text-600">模式选择</DropdownMenuLabel>
        {MODE_ORDER.map((candidate) => {
          const meta = MODE_META[candidate];
          const Icon = meta.icon;
          const active = candidate === mode;
          return (
            <DropdownMenuItem
              key={candidate}
              role="menuitemradio"
              aria-checked={active}
              aria-label={`交互方式：${meta.label}`}
              onSelect={() => onChange(candidate)}
              className={cn(
                'my-0.5 flex items-start gap-2.5 rounded-lg p-2.5',
                active && 'ui-selected',
              )}
            >
              <Icon
                className={cn('mt-0.5 h-4 w-4 shrink-0', active ? 'text-selected-foreground' : 'text-text-600')}
                strokeWidth={1.75}
              />
              <span className="min-w-0 flex-1">
                <span className={cn('block text-[13px] font-medium leading-5', active ? 'text-selected-foreground' : 'text-text-900')}>
                  {meta.label}
                </span>
                <span className="mt-0.5 block text-[11px] leading-4 text-text-600">{meta.description}</span>
              </span>
              {active && <Check className="mt-1 h-3.5 w-3.5 shrink-0 text-accent" strokeWidth={2.5} />}
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
};
