import React from 'react';
import { Bot, PanelRightClose } from 'lucide-react';
import { IconButton } from '../../components/ui/primitives';
import { useUIStore } from '../../stores/uiStore';
import { CommandComposer, type CommandComposerHandle } from './CommandComposer';
import type { StarterTemplate } from './starterTemplates';
import { ThreadTabs } from './ThreadTabs';
import { Timeline } from './Timeline';
import { useRunStore } from '../../stores/runStore';
import { useActiveThreadId, useActiveSession } from './useActiveSession';
import { ContextWindowPanel } from './ContextWindowPanel';
import { PlanIndicator } from './PlanIndicator';
import { SuggestPanel } from './SuggestPanel';
import { RenamePanel } from './RenamePanel';

export const AgentPanel: React.FC = () => {
  const composerRef = React.useRef<CommandComposerHandle>(null);
  const [starterAvailable, setStarterAvailable] = React.useState(false);
  const selectStarter = React.useCallback((template: StarterTemplate) => {
    void composerRef.current?.applyStarterTemplate(template);
  }, []);
  const [panelElement, setPanelElement] = React.useState<HTMLDivElement | null>(null);
  const headerRef = React.useRef<HTMLElement>(null);
  const [openPanel, setOpenPanel] = React.useState<'plan' | 'context' | null>(null);
  const changePlanOpen = React.useCallback((open: boolean) => {
    setOpenPanel((current) => open ? 'plan' : current === 'plan' ? null : current);
  }, []);
  const changeContextOpen = React.useCallback((open: boolean) => {
    setOpenPanel((current) => open ? 'context' : current === 'context' ? null : current);
  }, []);
  const toggleRightPanel = useUIStore((state) => state.toggleRightPanel);
  const threadId = useActiveThreadId();
  const { plan, status, activeRunId } = useActiveSession();
  const clearPlan = useRunStore(state => state.clearPlan);
  const busy = ['creating', 'running', 'waiting', 'recovering', 'canceling'].includes(status);
  const stopped = plan?.eventRunId !== activeRunId || !busy;

  return (
    <div ref={setPanelElement} className="agent-panel relative flex h-full flex-col bg-panel">
      <header ref={headerRef} className="workspace-panel-header flex shrink-0 items-center justify-between">
        <div className="flex items-center text-base font-semibold text-text-900">
          <Bot className="mr-2 h-[18px] w-[18px] text-accent" strokeWidth={1.75} />
          智能体
        </div>
        <div className="relative flex items-center gap-0.5">
          <PlanIndicator
            plan={plan} stopped={stopped} open={openPanel === 'plan'} onOpenChange={changePlanOpen}
            clearDisabled={busy} onClear={threadId ? () => clearPlan(threadId) : undefined}
          />
          <ContextWindowPanel anchorRef={headerRef} open={openPanel === 'context'} onOpenChange={changeContextOpen} />
          <IconButton label="隐藏右侧对话" onClick={toggleRightPanel}>
            <PanelRightClose className="h-[18px] w-[18px]" strokeWidth={1.75} />
          </IconButton>
        </div>
      </header>
      <ThreadTabs />
      <Timeline starterAvailable={starterAvailable} onStarterSelect={selectStarter} />
	  <RenamePanel />
	  <SuggestPanel />
	  <CommandComposer ref={composerRef} menuBoundary={panelElement} onStarterAvailabilityChange={setStarterAvailable} />
    </div>
  );
};
