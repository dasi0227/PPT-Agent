import React from 'react';
import { Blocks, BookOpenText, Check } from 'lucide-react';
import type { Skill } from '../../api/types';
import { cn } from '../../lib/utils';
import { MAX_SELECTED_SKILLS } from '../../stores/composerStore';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '../../components/ui/dropdown-menu';

interface SkillSelectorProps {
  skills: Skill[];
  selectedIds: string[];
  loading: boolean;
  disabled?: boolean;
  onToggle: (id: string) => void;
}

export const SkillSelector: React.FC<SkillSelectorProps> = ({
  skills,
  selectedIds,
  loading,
  disabled = false,
  onToggle,
}) => {
  const selected = new Set(selectedIds);
  const count = selectedIds.length;
  const label = loading ? '加载技能…' : count > 0 ? `技能 ${count}` : '技能';
  const title = loading
    ? '正在加载技能'
    : skills.length === 0
      ? '暂无可用技能'
      : `选择技能，最多 ${MAX_SELECTED_SKILLS} 个`;

  return (
    <div className="min-w-0 shrink-0" title={title}>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            aria-label="技能"
            disabled={disabled || loading}
            className={[
              'composer-skill-button inline-flex h-7 min-w-0 max-w-[88px] shrink-0 items-center gap-1 rounded-md border border-transparent px-2 text-[11px] font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:cursor-not-allowed disabled:opacity-45',
              count > 0
                ? 'bg-accent-soft text-accent'
                : 'bg-transparent text-text-600 hover:bg-panel-muted hover:text-text-900 focus-visible:bg-panel-muted focus-visible:text-text-900 data-[state=open]:bg-panel-muted data-[state=open]:text-text-900',
            ].join(' ')}
          >
            <Blocks className="h-3.5 w-3.5 shrink-0" strokeWidth={1.75} />
            <span className="composer-skill-label min-w-0 truncate">{label}</span>
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent
          side="top"
          align="start"
          className={[
            'max-h-72 max-w-[calc(100vw-24px)] overflow-y-auto p-1',
            skills.length === 0 ? 'w-[220px]' : 'w-[280px]',
          ].join(' ')}
        >
          {skills.length === 0 && (
            <div className="flex min-h-16 items-center justify-center px-4 py-3 text-xs text-text-400">
              暂无技能
            </div>
          )}
          {skills.map((skill) => {
            const active = selected.has(skill.id);
            const optionDisabled = !active && count >= MAX_SELECTED_SKILLS;
            return (
              <DropdownMenuItem
                key={skill.id}
                aria-label={`技能：${skill.name}`}
                aria-checked={active}
                role="menuitemcheckbox"
                disabled={optionDisabled}
                onSelect={(event) => {
                  event.preventDefault();
                  if (!optionDisabled) onToggle(skill.id);
                }}
                className={cn(
                  'flex items-start gap-2.5 rounded-sm px-2 py-1.5 text-xs',
                  active ? 'bg-accent-soft/70' : 'hover:bg-panel-muted',
                  optionDisabled ? 'cursor-not-allowed opacity-45' : '',
                )}
              >
                <BookOpenText
                  className={cn('mt-0.5 h-3.5 w-3.5 shrink-0', active ? 'text-accent' : 'text-text-600')}
                  strokeWidth={1.75}
                />
                <span className="min-w-0 flex-1">
                  <span className={cn('block truncate font-semibold', active ? 'text-accent' : 'text-text-900')}>
                    {skill.name}
                  </span>
                  <span className="block truncate text-[11px] leading-4 text-text-400" title={skill.description}>
                    {skill.description}
                  </span>
                </span>
                {active && <Check className="mt-1 h-3.5 w-3.5 shrink-0 text-accent" strokeWidth={2.5} />}
              </DropdownMenuItem>
            );
          })}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
};
