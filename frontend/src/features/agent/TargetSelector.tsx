import { ChevronDown } from 'lucide-react';
import type React from 'react';
import type { Artifact, TargetLevel } from '../../api/types';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '../../components/ui/dropdown-menu';

type ComposerTarget = { artifact: Artifact; level: TargetLevel };

interface ScopeOption {
  level: TargetLevel;
  label: string;
}

interface ObjectOption {
  artifact: Artifact;
  label: string;
}

const scopeOptions: ScopeOption[] = [
  { level: 'slide', label: '单页' },
  { level: 'deck', label: '整份' },
];

const objectOptions: ObjectOption[] = [
  { artifact: 'spec', label: '设计稿' },
  { artifact: 'presentation', label: '幻灯片' },
];

function scopeLabel(level: TargetLevel): string {
  return scopeOptions.find((option) => option.level === level)?.label ?? '整份';
}

function objectLabel(artifact: Artifact): string {
  return objectOptions.find((option) => option.artifact === artifact)?.label ?? '设计稿';
}

function targetLabel(target: ComposerTarget): string {
  return `${scopeLabel(target.level)}${objectLabel(target.artifact)}`;
}

interface TargetSelectorProps extends ComposerTarget {
  onTargetChange: (target: ComposerTarget) => void;
  disabled?: boolean;
}

export const TargetSelector: React.FC<TargetSelectorProps> = ({
  artifact,
  level,
  onTargetChange,
  disabled,
}) => {
  const selected = { artifact, level };
  const segmentClass = (active: boolean) => [
    'flex h-6 min-w-0 flex-1 items-center justify-center rounded-full px-2 text-[11px] font-medium transition-colors',
    active ? 'bg-surface text-text-900 shadow-sm' : 'text-text-400 hover:text-text-700',
  ].join(' ');

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={`目标：${targetLabel(selected)}`}
          disabled={disabled}
          className="inline-flex h-7 min-w-0 max-w-[112px] shrink items-center gap-0.5 rounded-md border border-border bg-transparent px-1 text-[11px] font-medium text-text-600 transition-colors hover:bg-panel-muted hover:text-text-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:cursor-not-allowed disabled:opacity-45"
        >
          <span className="min-w-0 truncate">{targetLabel(selected)}</span>
          <ChevronDown className="h-3.5 w-3.5 shrink-0" strokeWidth={1.75} />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent side="top" align="end" className="w-[224px] space-y-1 p-1.5">
        <div role="none" className="flex h-9 items-center gap-2 px-1">
          <span className="shrink-0 text-sm font-semibold text-text-900">范围</span>
          <div role="group" aria-label="范围" className="ml-auto flex w-[132px] rounded-full bg-panel-muted p-0.5">
            {scopeOptions.map((option) => (
              <DropdownMenuItem
                key={option.level}
                aria-label={`范围：${option.label}`}
                onSelect={(event) => {
                  event.preventDefault();
                  onTargetChange({ artifact, level: option.level });
                }}
                className={segmentClass(option.level === level)}
              >
                {option.label}
              </DropdownMenuItem>
            ))}
          </div>
        </div>
        <div role="none" className="flex h-9 items-center gap-2 px-1">
          <span className="shrink-0 text-sm font-semibold text-text-900">对象</span>
          <div role="group" aria-label="对象" className="ml-auto flex w-[132px] rounded-full bg-panel-muted p-0.5">
            {objectOptions.map((option) => (
              <DropdownMenuItem
                key={option.artifact}
                aria-label={`对象：${option.label}`}
                onSelect={(event) => {
                  event.preventDefault();
                  onTargetChange({ artifact: option.artifact, level });
                }}
                className={segmentClass(option.artifact === artifact)}
              >
                {option.label}
              </DropdownMenuItem>
            ))}
          </div>
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  );
};
