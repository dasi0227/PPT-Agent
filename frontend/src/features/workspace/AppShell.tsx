import React from 'react';
import { ProjectTabs } from './ProjectTabs';
import { DeckNavigator } from '../deck/DeckNavigator';
import { PreviewWorkspace } from '../viewer/PreviewWorkspace';
import { AgentPanel } from '../agent/AgentPanel';
import { useUIStore } from '../../stores/uiStore';
import { PanelGroup, Panel, PanelResizeHandle } from 'react-resizable-panels';
import { WorkspaceEmptyState } from './WorkspaceEmptyState';
import { useProjectStore } from '../../stores/projectStore';
import { ChevronRight, ChevronLeft } from 'lucide-react';

export const AppShell: React.FC = () => {
  const { leftPanelHidden, rightPanelHidden, toggleLeftPanel, toggleRightPanel } = useUIStore();
  const { activeProjectId } = useProjectStore();

  return (
    <div className="flex flex-col h-screen w-screen bg-background text-text-900 overflow-hidden font-sans relative">
      <ProjectTabs />
      
      <div className="flex flex-1 overflow-hidden relative">
        {activeProjectId === null ? (
          <WorkspaceEmptyState />
        ) : (
          <PanelGroup direction="horizontal" autoSaveId="workspace-shell-v6">
            {!leftPanelHidden && (
              <>
                <Panel id="left" defaultSize={22} minSize={16} maxSize={32} collapsible={false} className="bg-surface border-r border-border">
                  <DeckNavigator />
                </Panel>
                <PanelResizeHandle className="w-[3px] bg-border hover:bg-mode-normal transition-colors" />
              </>
            )}
            
            <Panel id="center" minSize={30} className="bg-background flex flex-col">
              <PreviewWorkspace />
            </Panel>
            
            {!rightPanelHidden && (
              <>
                <PanelResizeHandle className="w-[3px] bg-border hover:bg-mode-normal transition-colors" />
                <Panel id="right" defaultSize={28} minSize={20} maxSize={40} collapsible={false} className="bg-surface border-l border-border">
                  <AgentPanel />
                </Panel>
              </>
            )}
          </PanelGroup>
        )}

        {/* Floating Restore Buttons */}
        {activeProjectId !== null && activeProjectId !== 'new-pending' && leftPanelHidden && (
          <button 
            onClick={toggleLeftPanel}
            className="absolute left-0 top-1/2 -translate-y-1/2 w-4 h-12 bg-surface border border-l-0 border-border rounded-r-md flex items-center justify-center hover:bg-black/5 text-text-400 hover:text-text-600 z-10 shadow-sm"
          >
            <ChevronRight className="w-3 h-3" />
          </button>
        )}
        {activeProjectId !== null && activeProjectId !== 'new-pending' && rightPanelHidden && (
          <button 
            onClick={toggleRightPanel}
            className="absolute right-0 top-1/2 -translate-y-1/2 w-4 h-12 bg-surface border border-r-0 border-border rounded-l-md flex items-center justify-center hover:bg-black/5 text-text-400 hover:text-text-600 z-10 shadow-sm"
          >
            <ChevronLeft className="w-3 h-3" />
          </button>
        )}
      </div>
    </div>
  );
};
