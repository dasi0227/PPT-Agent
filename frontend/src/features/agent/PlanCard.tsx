import React, { useState } from 'react';
import { ChevronDown, ChevronRight, ListTodo, CheckCircle2, Circle, Loader2 } from 'lucide-react';
import { PlanItem } from './eventReducer';
import { cn } from '../../lib/utils';

export const PlanCard: React.FC<{ item: PlanItem }> = ({ item }) => {
  const [expanded, setExpanded] = useState(true);

  return (
    <div className="border border-border rounded-md bg-surface my-2 overflow-hidden shadow-sm">
      <button 
        onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center px-3 py-2 text-sm font-medium text-text-900 bg-background/50 hover:bg-black/5 transition-colors border-b border-border"
      >
        {expanded ? <ChevronDown className="w-4 h-4 mr-1 text-text-400" /> : <ChevronRight className="w-4 h-4 mr-1 text-text-400" />}
        <ListTodo className="w-4 h-4 mr-2 text-text-600" />
        <span className="flex-1 text-left">{item.title || "Execution Plan"}</span>
      </button>
      
      {expanded && (
        <div className="p-3 space-y-2">
          {item.steps.map((step, idx) => (
            <div key={step.id || idx} className="flex items-start text-sm">
              <div className="mr-2 mt-0.5 shrink-0">
                {step.status === 'completed' ? <CheckCircle2 className="w-4 h-4 text-mode-normal" /> :
                 step.status === 'in_progress' ? <Loader2 className="w-4 h-4 animate-spin text-mode-ask" /> :
                 <Circle className="w-4 h-4 text-text-400" />}
              </div>
              <div>
                <div className={cn("font-medium", step.status === 'completed' ? "text-text-600 line-through" : "text-text-900")}>
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
