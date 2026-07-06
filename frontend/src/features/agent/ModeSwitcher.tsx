import React from 'react';
import { cn } from '../../lib/utils';
import { InteractionMode, SubMode } from './modeMapping';

const MODES: Array<{ id: InteractionMode; label: string; dot: string }> = [
  { id: 'outline', label: '大纲', dot: 'bg-mode-normal' },
  { id: 'page', label: '单页', dot: 'bg-mode-normal' },
  { id: 'overview', label: '全局', dot: 'bg-mode-overview' },
  { id: 'repo', label: '仓库', dot: 'bg-mode-repo' },
];

interface ModeSwitcherProps {
  interactionMode: InteractionMode;
  subMode: SubMode;
  onModeChange: (mode: InteractionMode) => void;
  onSubModeChange: (subMode: SubMode) => void;
  disabled?: boolean;
}

export const ModeSwitcher: React.FC<ModeSwitcherProps> = ({
  interactionMode,
  subMode,
  onModeChange,
  onSubModeChange,
  disabled,
}) => {
  const toggleSub = (target: 'talk' | 'ask') => {
    onSubModeChange(subMode === target ? 'normal' : target);
  };

  return (
    <div className="flex items-center justify-between gap-2 mb-2">
      {/* 主切换器：四档互斥单选 */}
      <div className="flex items-center bg-background rounded-md p-0.5 border border-border">
        {MODES.map((m) => {
          const active = interactionMode === m.id;
          return (
            <button
              key={m.id}
              type="button"
              disabled={disabled}
              onClick={() => onModeChange(m.id)}
              className={cn(
                'flex items-center gap-1 px-2 py-1 rounded text-xs font-medium transition-colors disabled:opacity-50',
                active ? 'bg-surface text-text-900 shadow-sm' : 'text-text-400 hover:text-text-600'
              )}
              aria-pressed={active}
            >
              <span className={cn('w-1.5 h-1.5 rounded-full', active ? m.dot : 'bg-text-400/40')} />
              {m.label}
            </button>
          );
        })}
      </div>

      {/* 副模式：Talk / Ask 互斥可取消 */}
      <div className="flex items-center gap-1">
        <button
          type="button"
          disabled={disabled}
          onClick={() => toggleSub('talk')}
          className={cn(
            'px-2 py-1 rounded text-xs font-medium border transition-colors disabled:opacity-50',
            subMode === 'talk'
              ? 'bg-mode-talk text-white border-mode-talk'
              : 'text-text-600 border-border hover:bg-black/5'
          )}
          aria-pressed={subMode === 'talk'}
        >
          Talk
        </button>
        <button
          type="button"
          disabled={disabled}
          onClick={() => toggleSub('ask')}
          className={cn(
            'px-2 py-1 rounded text-xs font-medium border transition-colors disabled:opacity-50',
            subMode === 'ask'
              ? 'bg-mode-ask text-white border-mode-ask'
              : 'text-text-600 border-border hover:bg-black/5'
          )}
          aria-pressed={subMode === 'ask'}
        >
          Ask
        </button>
      </div>
    </div>
  );
};
