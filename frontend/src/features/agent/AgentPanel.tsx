import React from 'react';
import { Bot, PanelRightClose } from 'lucide-react';
import { IconButton } from '../../components/ui/primitives';
import { useUIStore } from '../../stores/uiStore';
import { CommandComposer } from './CommandComposer';
import { ThreadTabs } from './ThreadTabs';
import { Timeline } from './Timeline';
import { useActiveSession } from './useActiveSession';
import { ContextWindowPanel } from './ContextWindowPanel';
import { PlanIndicator } from './PlanIndicator';
import { RenamePanel } from './RenamePanel';

export const AgentPanel: React.FC = () => {
  const headerRef = React.useRef<HTMLElement>(null);
  const [openPanel, setOpenPanel] = React.useState<'plan' | 'context' | null>(null);
  const changePlanOpen = React.useCallback((open: boolean) => {
    setOpenPanel((current) => open ? 'plan' : current === 'plan' ? null : current);
  }, []);
  const changeContextOpen = React.useCallback((open: boolean) => {
    setOpenPanel((current) => open ? 'context' : current === 'context' ? null : current);
  }, []);
  const toggleRightPanel = useUIStore((state) => state.toggleRightPanel);
  const { plan } = useActiveSession();

  return (
    <div className="agent-panel relative flex h-full flex-col bg-panel">
      <header ref={headerRef} className="flex h-12 shrink-0 items-center justify-between border-b border-border bg-panel px-3">
        <div className="flex items-center text-sm font-semibold text-text-900">
          <Bot className="mr-2 h-4 w-4 text-accent" strokeWidth={1.75} />
          智能体
        </div>
        <div className="relative flex items-center gap-0.5">
          <PlanIndicator plan={plan} open={openPanel === 'plan'} onOpenChange={changePlanOpen} />
          <ContextWindowPanel anchorRef={headerRef} open={openPanel === 'context'} onOpenChange={changeContextOpen} />
          <IconButton label="隐藏右侧对话" onClick={toggleRightPanel}>
            <PanelRightClose className="h-4 w-4" strokeWidth={1.75} />
          </IconButton>
        </div>
      </header>
      <ThreadTabs />
      <Timeline />
	  <RenamePanel />
	  <CommandComposer />
    </div>
  );
};
