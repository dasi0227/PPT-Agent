import React from 'react';
import { ProjectTabs } from './ProjectTabs';
import { DeckNavigator } from '../deck/DeckNavigator';
import { PreviewWorkspace } from '../viewer/PreviewWorkspace';
import { AgentPanel } from '../agent/AgentPanel';
import { useUIStore } from '../../stores/uiStore';
import { cn } from '../../lib/utils';

export const AppShell: React.FC = () => {
  const { leftPanelCollapsed, rightPanelOpen } = useUIStore();

  return (
    <div className="flex flex-col h-screen w-screen bg-background text-text-900 overflow-hidden font-sans">
      <ProjectTabs />
      
      <div className="flex flex-1 overflow-hidden">
        {/* Left Panel */}
        <aside 
          className={cn(
            "flex-shrink-0 border-r border-border bg-surface transition-all duration-300 flex flex-col",
            leftPanelCollapsed ? "w-0 opacity-0" : "w-[280px] opacity-100"
          )}
        >
          <DeckNavigator />
        </aside>

        {/* Center Panel */}
        <main className="flex-1 min-w-0 bg-background flex flex-col">
          <PreviewWorkspace />
        </main>

        {/* Right Panel */}
        <aside 
          className={cn(
            "flex-shrink-0 border-l border-border bg-surface transition-all duration-300 flex flex-col",
            rightPanelOpen ? "w-[380px] opacity-100" : "w-0 opacity-0"
          )}
        >
          <AgentPanel />
        </aside>
      </div>
    </div>
  );
};
