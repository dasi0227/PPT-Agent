import React from 'react';
import type { Artifact, ClarificationPolicy, InteractionIntent, TargetLevel } from '../../api/types';
import { cn } from '../../lib/utils';

interface ModeSwitcherProps {
  artifact: Artifact;
  level: TargetLevel;
  intent: InteractionIntent;
  clarification: ClarificationPolicy;
  onArtifactChange: (value: Artifact) => void;
  onLevelChange: (value: TargetLevel) => void;
  onIntentChange: (value: InteractionIntent) => void;
  onClarificationChange: (value: ClarificationPolicy) => void;
  disabled?: boolean;
}

function Segment<T extends string>({ value, options, onChange, disabled }: {
  value: T;
  options: Array<{ value: T; label: string }>;
  onChange: (value: T) => void;
  disabled?: boolean;
}) {
  return (
    <div className="flex rounded-md border border-border bg-panel-muted p-0.5">
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          disabled={disabled}
          aria-pressed={option.value === value}
          onClick={() => onChange(option.value)}
          className={cn(
            'rounded px-2 py-1 text-xs font-medium transition-colors disabled:opacity-50',
            option.value === value ? 'bg-surface text-accent' : 'text-text-400 hover:text-text-600',
          )}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}

export const ModeSwitcher: React.FC<ModeSwitcherProps> = (props) => (
  <div className="flex flex-wrap items-center gap-1.5">
    <Segment
      value={props.artifact}
      options={[{ value: 'blueprint', label: '蓝图' }, { value: 'presentation', label: 'HTML' }]}
      onChange={props.onArtifactChange}
      disabled={props.disabled}
    />
    <Segment
      value={props.level}
      options={[{ value: 'slide', label: '当前页' }, { value: 'deck', label: '整份' }]}
      onChange={props.onLevelChange}
      disabled={props.disabled}
    />
    <button
      type="button"
      disabled={props.disabled}
      aria-pressed={props.intent === 'consult'}
      onClick={() => props.onIntentChange(props.intent === 'consult' ? 'apply' : 'consult')}
      className={cn(
        'rounded border px-2 py-1 text-xs font-medium',
        props.intent === 'consult' ? 'border-accent bg-accent text-white' : 'border-border text-text-600',
      )}
    >
      讨论
    </button>
    <button
      type="button"
      disabled={props.disabled || props.intent === 'consult'}
      aria-pressed={props.clarification === 'before_apply'}
      onClick={() => props.onClarificationChange(props.clarification === 'before_apply' ? 'when_blocked' : 'before_apply')}
      className={cn(
        'rounded border px-2 py-1 text-xs font-medium disabled:opacity-40',
        props.clarification === 'before_apply' ? 'border-warning bg-warning text-white' : 'border-border text-text-600',
      )}
    >
      执行前确认
    </button>
  </div>
);
