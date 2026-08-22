import React from 'react';
import { ProjectTabs } from './ProjectTabs';
import { DeckNavigator } from '../deck/DeckNavigator';
import { PreviewWorkspace } from '../viewer/PreviewWorkspace';
import { AgentPanel } from '../agent/AgentPanel';
import { useUIStore } from '../../stores/uiStore';
import { PanelGroup, Panel, PanelResizeHandle } from 'react-resizable-panels';
import { WorkspaceEmptyState } from './WorkspaceEmptyState';
import { useProjectStore } from '../../stores/projectStore';

const RIGHT_PANEL_MIN_CONTROLS_PX = 400;
const RIGHT_PANEL_MIN_SIZE = 20;
const RIGHT_PANEL_MAX_SIZE = 40;

function rightPanelMinSize(workspaceWidth: number): number {
  if (!Number.isFinite(workspaceWidth) || workspaceWidth <= 0) return RIGHT_PANEL_MIN_SIZE;
  const controlsPercent = (RIGHT_PANEL_MIN_CONTROLS_PX / workspaceWidth) * 100;
  return Math.min(
    RIGHT_PANEL_MAX_SIZE,
    Math.max(RIGHT_PANEL_MIN_SIZE, Math.ceil(controlsPercent)),
  );
}

export const AppShell: React.FC = () => {
  const { leftPanelHidden, rightPanelHidden } = useUIStore();
  const { activeProjectId } = useProjectStore();
  const shellRef = React.useRef<HTMLDivElement>(null);
  const [workspaceWidth, setWorkspaceWidth] = React.useState(() =>
    typeof window === 'undefined' ? 0 : window.innerWidth);
  const dynamicRightMinSize = rightPanelMinSize(workspaceWidth);
  const rightPanelDefaultSize = rightPanelHidden ? 0 : Math.max(28, dynamicRightMinSize);
  const centerDefaultSize =
    100 - (leftPanelHidden ? 0 : 22) - rightPanelDefaultSize;

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
                <Panel
                  id="right"
                  order={3}
                  defaultSize={rightPanelDefaultSize}
                  minSize={dynamicRightMinSize}
                  maxSize={RIGHT_PANEL_MAX_SIZE}
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
