import type React from 'react';
import type { Artifact, ScopeLevel } from '../../api/types';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '../../components/ui/dropdown-menu';

type ComposerTarget = { artifact: Artifact; level: ScopeLevel };

interface ScopeOption {
  level: ScopeLevel;
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
  { artifact: 'ppt', label: '幻灯片' },
];

function scopeLabel(level: ScopeLevel): string {
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
  locked?: boolean;
}

const EMPTY_PROJECT_HINT = '当前为空项目，请先确定整体的设计稿';
const LOCKED_TARGET_LABEL = '整份设计稿';

export const TargetSelector: React.FC<TargetSelectorProps> = ({
  artifact,
  level,
  onTargetChange,
  disabled,
  locked = false,
}) => {
  const selected = { artifact, level };
  const segmentClass = (active: boolean) => [
    'flex h-6 min-w-0 flex-1 items-center justify-center rounded-full px-2 text-[11px] font-medium transition-colors',
    active ? 'bg-surface text-text-900 shadow-sm' : 'text-text-400 hover:text-text-700',
  ].join(' ');

  if (locked) {
    const hintId = 'target-selector-locked-hint';
    return (
      <div className="group relative min-w-0">
        <button
          type="button"
          aria-label={`目标：${LOCKED_TARGET_LABEL}`}
          aria-describedby={hintId}
          className={[
            'inline-flex h-7 min-w-0 max-w-[112px] shrink cursor-default items-center rounded-md border border-border bg-transparent px-1 text-[11px] font-medium text-text-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent',
            disabled ? 'opacity-45' : '',
          ].join(' ')}
        >
          <span className="min-w-0 truncate">{LOCKED_TARGET_LABEL}</span>
        </button>
        <span
          role="tooltip"
          id={hintId}
          className="pointer-events-none absolute bottom-full right-0 mb-1.5 w-max max-w-[176px] rounded-md border border-border bg-surface px-2 py-1 text-[11px] leading-4 text-text-600 opacity-0 shadow-md transition-opacity group-hover:opacity-100 group-focus-within:opacity-100"
        >
          {EMPTY_PROJECT_HINT}
        </span>
      </div>
    );
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={`目标：${targetLabel(selected)}`}
          disabled={disabled}
          className="inline-flex h-7 min-w-0 max-w-[112px] shrink items-center rounded-md border border-border bg-transparent px-2 text-[11px] font-medium text-text-600 transition-colors hover:bg-panel-muted hover:text-text-900 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:cursor-not-allowed disabled:opacity-45"
        >
          <span className="min-w-0 truncate">{targetLabel(selected)}</span>
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
