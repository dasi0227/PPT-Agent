import React from 'react';
import { ProjectTabs } from './ProjectTabs';
import { DeckNavigator } from '../deck/DeckNavigator';
import { ThemeSelector } from '../viewer/ThemeSelector';
import { PreviewWorkspace } from '../viewer/PreviewWorkspace';
import { AgentPanel } from '../agent/AgentPanel';
import { useUIStore } from '../../stores/uiStore';
import { PanelGroup, Panel, PanelResizeHandle } from 'react-resizable-panels';
import { WorkspaceEmptyState } from './WorkspaceEmptyState';
import { useProjectStore } from '../../stores/projectStore';
import { workspacePanelLayout } from './panelLayout';

export const AppShell: React.FC = () => {
  const preferences = useUIStore();
  const [deckActionsTarget, setDeckActionsTarget] = React.useState<HTMLDivElement | null>(null);
  const [themeTarget, setThemeTarget] = React.useState<HTMLDivElement | null>(null);
  const { activeProjectId } = useProjectStore();
  const shellRef = React.useRef<HTMLDivElement>(null);
  const [workspaceWidth, setWorkspaceWidth] = React.useState(() =>
    typeof window === 'undefined' ? 0 : window.innerWidth);
  const layout = workspacePanelLayout(workspaceWidth, preferences.leftPanelHidden, preferences.rightPanelHidden);
  const { leftHidden: leftPanelHidden, rightHidden: rightPanelHidden } = layout;
  const canExpandLeft = !workspacePanelLayout(workspaceWidth, false, true).leftHidden;
  const canExpandRight = !workspacePanelLayout(workspaceWidth, true, false).rightHidden;
  const expandLeft = () => {
    if (!canExpandLeft) return;
    const next = workspacePanelLayout(workspaceWidth, false, preferences.rightPanelHidden);
    useUIStore.setState({ leftPanelHidden: false, rightPanelHidden: next.leftHidden || preferences.rightPanelHidden });
  };
  const expandRight = () => {
    if (!canExpandRight) return;
    useUIStore.setState({ rightPanelHidden: false });
  };

  React.useLayoutEffect(() => {
    const shell = shellRef.current;
    if (!shell) return;
    const updateWidth = () => {
      const width = shell.getBoundingClientRect().width || window.innerWidth || 0;
      setWorkspaceWidth(width);
    };
    updateWidth();
    if (typeof ResizeObserver === 'undefined') {
      window.addEventListener('resize', updateWidth);
      return () => window.removeEventListener('resize', updateWidth);
    }
    const observer = new ResizeObserver(updateWidth);
    observer.observe(shell);
    return () => observer.disconnect();
  }, []);

  return (
    <div ref={shellRef} className="flex flex-col h-[100dvh] w-screen bg-workspace text-text-900 overflow-hidden font-sans relative">
      {activeProjectId !== null && <ProjectTabs />}
      {activeProjectId !== null && <ThemeSelector key={activeProjectId} projectId={activeProjectId} container={themeTarget} />}
      
      <div className="flex flex-1 overflow-hidden relative">
        {activeProjectId === null ? (
          <WorkspaceEmptyState />
        ) : (
          <PanelGroup 
            key={`group-${leftPanelHidden}-${rightPanelHidden}`} 
            direction="horizontal" 
            autoSaveId={`workspace-shell-v9-${leftPanelHidden ? 'noleft' : 'left'}-${rightPanelHidden ? 'noright' : 'right'}`}
          >
            {!leftPanelHidden && (
              <>
                <Panel id="left" order={1} defaultSize={layout.leftDefault} minSize={layout.leftMin} maxSize={26} collapsible={false} className="bg-panel border-r border-border">
                  <DeckNavigator actionsRef={setDeckActionsTarget} />
                </Panel>
                <PanelResizeHandle aria-label="调整左栏宽度" className="w-[3px] bg-border hover:bg-accent transition-colors" />
              </>
            )}
            
            <Panel id="center" order={2} defaultSize={layout.centerDefault} minSize={layout.centerMin} className="bg-canvas flex flex-col">
              <PreviewWorkspace themeRef={setThemeTarget} deckActionsTarget={deckActionsTarget} sidebarControls={{
                leftHidden: leftPanelHidden,
                rightHidden: rightPanelHidden,
                canExpandLeft,
                canExpandRight,
                onExpandLeft: expandLeft,
                onExpandRight: expandRight,
              }} />
            </Panel>
            
            {!rightPanelHidden && (
              <>
                <PanelResizeHandle aria-label="调整右栏宽度" className="w-[3px] bg-border hover:bg-accent transition-colors" />
                <Panel
                  id="right"
                  order={3}
                  defaultSize={layout.rightDefault}
                  minSize={layout.rightMin}
                  maxSize={40}
                  collapsible={false}
                  className="bg-panel border-l border-border"
                >
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
