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
  const centerDefaultSize =
    100 - (leftPanelHidden ? 0 : 22) - (rightPanelHidden ? 0 : 28);

  return (
    <div className="flex flex-col h-[100dvh] w-screen bg-workspace text-text-900 overflow-hidden font-sans relative">
      <ProjectTabs />
      
      <div className="flex flex-1 overflow-hidden relative">
        {activeProjectId === null ? (
          <WorkspaceEmptyState />
        ) : (
          <PanelGroup 
            key={`group-${leftPanelHidden}-${rightPanelHidden}`} 
            direction="horizontal" 
            autoSaveId={`workspace-shell-v8-${leftPanelHidden ? 'noleft' : 'left'}-${rightPanelHidden ? 'noright' : 'right'}`}
          >
            {!leftPanelHidden && (
              <>
                <Panel id="left" order={1} defaultSize={22} minSize={16} maxSize={32} collapsible={false} className="bg-panel border-r border-border">
                  <DeckNavigator />
                </Panel>
                <PanelResizeHandle aria-label="调整左栏宽度" className="w-[3px] bg-border hover:bg-accent transition-colors" />
              </>
            )}
            
            <Panel id="center" order={2} defaultSize={centerDefaultSize} minSize={30} className="bg-canvas flex flex-col">
              <PreviewWorkspace />
            </Panel>
            
            {!rightPanelHidden && (
              <>
                <PanelResizeHandle aria-label="调整右栏宽度" className="w-[3px] bg-border hover:bg-accent transition-colors" />
                <Panel id="right" order={3} defaultSize={28} minSize={20} maxSize={40} collapsible={false} className="bg-panel border-l border-border">
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
