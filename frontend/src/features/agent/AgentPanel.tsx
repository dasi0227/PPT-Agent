import React from 'react';
import { Timeline } from './Timeline';
import { CommandComposer } from './CommandComposer';
import { ThreadTabs } from './ThreadTabs';
import { useActiveSession } from './useActiveSession';
import { useUIStore } from '../../stores/uiStore';
import { Bot, Loader2, AlertCircle, PanelRightClose } from 'lucide-react';
import { cn } from '../../lib/utils';

export const AgentPanel: React.FC = () => {
  const { status, target, interaction, progress, strategy } = useActiveSession();
  const { toggleRightPanel } = useUIStore();

  const getModeColor = () => {
    switch (interaction.intent) {
      case 'consult': return 'bg-mode-talk text-white';
      case 'apply': default: return target.artifact === 'blueprint' ? 'bg-mode-outline text-white' : 'bg-mode-normal text-white';
    }
  };

  return (
    <div className="flex flex-col h-full bg-surface">
      {/* Header */}
      <div className="h-12 border-b border-border flex items-center justify-between px-4 shrink-0 bg-background/50">
        <div className="flex items-center text-sm font-medium text-text-900">
          <Bot className="w-4 h-4 mr-2 text-text-600" />
          Agent
          {status !== 'idle' && (
            <span className={cn("ml-2 px-1.5 py-0.5 rounded text-[10px] uppercase font-bold", getModeColor())}>
              {target.artifact}/{target.level}
            </span>
          )}
          {strategy && (
            <span className="ml-1.5 text-[10px] uppercase font-medium text-text-400">
              {strategy}
            </span>
          )}
        </div>
        
        {/* Run Summary / Status */}
        <div className="flex items-center text-xs ml-auto mr-4">
          {status === 'running' && (
            <div className="flex items-center text-mode-normal">
              <Loader2 className="w-3 h-3 mr-1 animate-spin" />
              {progress ? progress.stage : 'Thinking...'}
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
          {status === 'canceled' && <div className="text-text-400">Canceled</div>}
        </div>

        <button onClick={toggleRightPanel} className="p-1 hover:bg-black/5 rounded text-text-400 hover:text-text-600 transition-colors">
          <PanelRightClose className="w-4 h-4" />
        </button>
      </div>

      <ThreadTabs />

      <Timeline />
      
      <CommandComposer />
    </div>
  );
};
