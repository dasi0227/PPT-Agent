import React from 'react';
import { Blocks, BookOpenText } from 'lucide-react';
import type { Skill } from '../../api/types';
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
            disabled={disabled || loading || skills.length === 0}
            className={[
              'composer-skill-button inline-flex h-7 min-w-0 max-w-[88px] shrink-0 items-center gap-1 rounded-md border px-2 text-[11px] font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:cursor-not-allowed disabled:opacity-45',
              count > 0
                ? 'border-accent/30 bg-accent-soft text-accent'
                : 'border-border bg-transparent text-text-600 hover:bg-panel-muted hover:text-text-900',
            ].join(' ')}
          >
            <Blocks className="h-3.5 w-3.5 shrink-0" strokeWidth={1.75} />
            <span className="composer-skill-label min-w-0 truncate">{label}</span>
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent
          side="top"
          align="end"
          className="max-h-72 w-[min(370px,calc(100vw-24px))] overflow-y-auto p-1"
        >
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
                className={[
                  'flex min-h-12 items-start gap-2 rounded-sm px-2 py-2 text-xs',
                  active ? 'bg-accent-soft text-text-900' : 'text-text-600',
                  optionDisabled ? 'cursor-not-allowed opacity-45' : '',
                ].join(' ')}
              >
                <BookOpenText
                  className={`mt-0.5 h-4 w-4 shrink-0 ${active ? 'text-accent' : 'text-text-400'}`}
                  strokeWidth={1.75}
                />
                <span className="min-w-0 leading-[1.45]">
                  <span className="mr-1.5 font-semibold text-text-900">{skill.name}</span>
                  <span>{skill.description}</span>
                </span>
              </DropdownMenuItem>
            );
          })}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
};
