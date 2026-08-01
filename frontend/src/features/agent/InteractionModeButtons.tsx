import { CircleHelp, MessageCircle } from 'lucide-react';
import type React from 'react';
import type { ClarificationPolicy, InteractionIntent } from '../../api/types';
import { cn } from '../../lib/utils';

interface InteractionModeButtonsProps {
  intent: InteractionIntent;
  clarification: ClarificationPolicy;
  onIntentChange: (intent: InteractionIntent) => void;
  onClarificationChange: (clarification: ClarificationPolicy) => void;
  disabled?: boolean;
}

export const InteractionModeButtons: React.FC<InteractionModeButtonsProps> = ({
  intent,
  clarification,
  onIntentChange,
  onClarificationChange,
  disabled,
}) => {
  const isTalk = intent === 'consult';
  const isAsk = intent === 'apply' && clarification === 'before_apply';

  const toggleTalk = () => {
    onIntentChange(isTalk ? 'apply' : 'consult');
    onClarificationChange('when_blocked');
  };

  const toggleAsk = () => {
    onIntentChange('apply');
    onClarificationChange(isAsk ? 'when_blocked' : 'before_apply');
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
        title="先澄清需求并形成方案，确认后再执行修改"
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
