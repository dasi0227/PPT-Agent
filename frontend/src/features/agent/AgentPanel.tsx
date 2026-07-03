import React from 'react';
import { Timeline } from './Timeline';
import { CommandComposer } from './CommandComposer';
import { useRunStore } from '../../stores/runStore';
import { Bot, Loader2, AlertCircle } from 'lucide-react';
import { cn } from '../../lib/utils';

export const AgentPanel: React.FC = () => {
  const { status, mode, progress } = useRunStore();

  const getModeColor = () => {
    switch (mode) {
      case 'talk': return 'bg-mode-talk text-white';
      case 'ask': return 'bg-mode-ask text-white';
      case 'normal': default: return 'bg-mode-normal text-white';
    }
  };

  return (
    <div className="flex flex-col h-full bg-surface">
      {/* Header */}
      <div className="h-12 border-b border-border flex items-center justify-between px-4 shrink-0 bg-background">
        <div className="flex items-center text-sm font-medium text-text-900">
          <Bot className="w-4 h-4 mr-2 text-text-600" />
          Agent
          {status !== 'idle' && (
            <span className={cn("ml-2 px-1.5 py-0.5 rounded text-[10px] uppercase font-bold", getModeColor())}>
              {mode}
            </span>
          )}
        </div>
        
        {/* Run Summary / Status */}
        <div className="flex items-center text-xs">
          {status === 'running' && (
            <div className="flex items-center text-mode-normal">
              <Loader2 className="w-3 h-3 mr-1 animate-spin" />
              {progress ? `${progress.stage} (${progress.current}/${progress.total})` : 'Thinking...'}
            </div>
          )}
          {status === 'needs_input' && (
            <div className="flex items-center text-mode-ask font-medium">
              <AlertCircle className="w-3 h-3 mr-1" />
              Needs Input
            </div>
          )}
          {status === 'error' && (
            <div className="flex items-center text-mode-error">
              <AlertCircle className="w-3 h-3 mr-1" />
              Failed
            </div>
          )}
        </div>
      </div>

      <Timeline />
      
      <CommandComposer />
    </div>
  );
};
