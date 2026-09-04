import { MessageCircleQuestion, MessagesSquare } from 'lucide-react';
import type React from 'react';
import type { RunMode } from '../../api/types';
import { cn } from '../../lib/utils';

interface InteractionModeButtonsProps {
  mode: RunMode;
  onIntentChange: (mode: RunMode) => void;
  disabled?: boolean;
}

export const InteractionModeButtons: React.FC<InteractionModeButtonsProps> = ({
  mode,
  onIntentChange,
  disabled,
}) => {
  const isChat = mode === 'chat';
  const isGrill = mode === 'grill';

  const toggleChat = () => {
    onIntentChange(isChat ? 'execute' : 'chat');
  };

  const toggleGrill = () => {
    onIntentChange(isGrill ? 'execute' : 'grill');
  };

  const buttonClass = (selected: boolean) => cn(
    'composer-mode-button inline-flex h-7 shrink-0 items-center gap-0.5 rounded-md border px-1 text-[11px] font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:cursor-not-allowed disabled:opacity-45',
    selected
      ? 'border-accent/30 bg-accent-soft text-accent'
      : 'border-border bg-transparent text-text-600 hover:bg-panel-muted hover:text-text-900',
  );

  return (
    <div className="flex shrink-0 items-center gap-0.5" aria-label="交互方式">
      <button
        type="button"
        aria-label="讨论"
        aria-pressed={isChat}
        title="只分析和交流，不修改项目内容"
        disabled={disabled}
        onClick={toggleChat}
        className={buttonClass(isChat)}
      >
        <MessagesSquare className="h-3.5 w-3.5" strokeWidth={1.75} />
        <span className="composer-mode-label">讨论</span>
      </button>
      <button
        type="button"
        aria-label="盘问"
        aria-pressed={isGrill}
        title="只读探索，并允许 Agent 在需要时向你提问"
        disabled={disabled}
        onClick={toggleGrill}
        className={buttonClass(isGrill)}
      >
        <MessageCircleQuestion className="h-3.5 w-3.5" strokeWidth={1.75} />
        <span className="composer-mode-label">盘问</span>
      </button>
    </div>
  );
};
