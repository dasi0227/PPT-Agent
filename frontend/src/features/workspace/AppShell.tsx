import React from 'react';
import { ProjectTabs } from './ProjectTabs';
import { DeckNavigator } from '../deck/DeckNavigator';
import { PreviewWorkspace } from '../viewer/PreviewWorkspace';
import { AgentPanel } from '../agent/AgentPanel';
import { useUIStore } from '../../stores/uiStore';
import { PanelGroup, Panel, PanelResizeHandle } from 'react-resizable-panels';
import { WorkspaceEmptyState } from './WorkspaceEmptyState';
import { useProjectStore } from '../../stores/projectStore';

export const AppShell: React.FC = () => {
  const { leftPanelHidden, rightPanelHidden } = useUIStore();
  const { activeProjectId } = useProjectStore();

  return (
    <div className="flex flex-col h-screen w-screen bg-background text-text-900 overflow-hidden font-sans">
      <ProjectTabs />
      
      <div className="flex flex-1 overflow-hidden">
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
      </div>
    </div>
  );
};
