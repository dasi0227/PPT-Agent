import { CircleHelp, MessageCircle } from 'lucide-react';
import type React from 'react';
import type { InteractionIntent } from '../../api/types';
import { cn } from '../../lib/utils';

interface InteractionModeButtonsProps {
  intent: InteractionIntent;
  onIntentChange: (intent: InteractionIntent) => void;
  disabled?: boolean;
}

export const InteractionModeButtons: React.FC<InteractionModeButtonsProps> = ({
  intent,
  onIntentChange,
  disabled,
}) => {
  const isTalk = intent === 'talk';
  const isAsk = intent === 'ask';

  const toggleTalk = () => {
    onIntentChange(isTalk ? 'execute' : 'talk');
  };

  const toggleAsk = () => {
    onIntentChange(isAsk ? 'execute' : 'ask');
  };

  const buttonClass = (selected: boolean) => cn(
    'inline-flex h-7 shrink-0 items-center gap-0.5 rounded-md border px-1 text-[11px] font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent disabled:cursor-not-allowed disabled:opacity-45',
    selected
      ? 'border-accent/30 bg-accent-soft text-accent'
      : 'border-border bg-transparent text-text-600 hover:bg-panel-muted hover:text-text-900',
  );

  return (
    <div className="flex shrink-0 items-center gap-0.5" aria-label="交互方式">
      <button
        type="button"
        aria-label="讨论"
        aria-pressed={isTalk}
        title="只分析和交流，不修改项目内容"
        disabled={disabled}
        onClick={toggleTalk}
        className={buttonClass(isTalk)}
      >
        <MessageCircle className="h-3.5 w-3.5" strokeWidth={1.75} />
        讨论
      </button>
      <button
        type="button"
        aria-label="询问"
        aria-pressed={isAsk}
        title="只读探索，并允许 Agent 在需要时向你提问"
        disabled={disabled}
        onClick={toggleAsk}
        className={buttonClass(isAsk)}
      >
        <CircleHelp className="h-3.5 w-3.5" strokeWidth={1.75} />
        询问
      </button>
    </div>
  );
};
