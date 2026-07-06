import React, { useState } from 'react';
import { ChevronDown, ChevronRight, ListTodo, CheckCircle2, Circle, Loader2, XCircle, MinusCircle } from 'lucide-react';
import { PlanState, PlanStepStatus } from '../../api/types';
import { cn } from '../../lib/utils';

function StepIcon({ status }: { status: PlanStepStatus }) {
  switch (status) {
    case 'completed': return <CheckCircle2 className="w-4 h-4 text-mode-final" />;
    case 'in_progress': return <Loader2 className="w-4 h-4 animate-spin text-mode-ask" />;
    case 'failed': return <XCircle className="w-4 h-4 text-mode-error" />;
    case 'skipped': return <MinusCircle className="w-4 h-4 text-text-400" />;
    case 'pending':
    default: return <Circle className="w-4 h-4 text-text-400" />;
  }
}

export const PlanCard: React.FC<{ plan: PlanState }> = ({ plan }) => {
  const [expanded, setExpanded] = useState(true);
  const done = plan.steps.filter((s) => s.status === 'completed').length;

  return (
    <div className="border border-border rounded-md bg-surface overflow-hidden shadow-sm">
      <button
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center px-3 py-2 text-sm font-medium text-text-900 bg-background/50 hover:bg-black/5 transition-colors border-b border-border"
      >
        {expanded ? <ChevronDown className="w-4 h-4 mr-1 text-text-400" /> : <ChevronRight className="w-4 h-4 mr-1 text-text-400" />}
        <ListTodo className="w-4 h-4 mr-2 text-text-600" />
        <span className="flex-1 text-left">{plan.title || '构建计划'}</span>
        <span className="text-xs text-text-400 tabular-nums">{done}/{plan.steps.length}</span>
      </button>

      {expanded && (
        <div className="p-3 space-y-2">
          {plan.steps.map((step) => (
            <div
              key={step.id}
              className={cn(
                'flex items-start text-sm rounded px-1.5 py-1 -mx-1.5',
                step.status === 'in_progress' && 'bg-mode-ask/10'
              )}
            >
              <div className="mr-2 mt-0.5 shrink-0">
                <StepIcon status={step.status} />
              </div>
              <div className="min-w-0">
                <div className={cn(
                  'font-medium',
                  step.status === 'completed' ? 'text-text-600 line-through' :
                  step.status === 'failed' ? 'text-mode-error' :
                  step.status === 'in_progress' ? 'text-text-900' : 'text-text-900'
                )}>
                  {step.title}
                </div>
                {step.detail && <div className="text-xs text-text-400 mt-0.5">{step.detail}</div>}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
