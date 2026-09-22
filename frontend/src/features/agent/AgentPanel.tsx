import React, { useState } from 'react';
import { Bot, GitCommitHorizontal, Loader2, PanelRightClose } from 'lucide-react';
import { IconButton } from '../../components/ui/primitives';
import { useUIStore } from '../../stores/uiStore';
import { CommandComposer } from './CommandComposer';
import { ThreadTabs } from './ThreadTabs';
import { Timeline } from './Timeline';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { useComposerStore } from '../../stores/composerStore';
import { useGitCommitStore } from '../../stores/gitCommitStore';
import { useBriefingStore } from '../../stores/briefingStore';
import { useActiveSession } from './useActiveSession';
import { ContextWindowPanel } from './ContextWindowPanel';
import { PlanIndicator } from './PlanIndicator';
import { RenamePanel } from './RenamePanel';

export const AgentPanel: React.FC = () => {
  const [polishToolbarContainer, setPolishToolbarContainer] = useState<HTMLDivElement | null>(null);
  const toggleRightPanel = useUIStore((state) => state.toggleRightPanel);
  const activeProjectId = useProjectStore((state) => state.activeProjectId);
  const ensureActiveThread = useThreadStore((state) => state.ensureActiveThread);
  const polishing = useComposerStore((state) => state.polishing);
  const startCommit = useGitCommitStore((state) => state.start);
  const commitSession = useGitCommitStore((state) => (
    activeProjectId ? state.sessions[activeProjectId] : undefined
  ));
  const { status: runStatus, plan } = useActiveSession();
  const commitActive = commitSession?.status === 'creating' || commitSession?.status === 'running';
  const briefingActive = useBriefingStore((state) => (
    activeProjectId ? state.sessions[activeProjectId]?.status === 'generating' : false
  ));
  const runActive = ['creating', 'running', 'waiting', 'paused', 'recovering', 'canceling'].includes(runStatus);
  const commitDisabled = !activeProjectId || commitActive || briefingActive || polishing || runActive;
  const commit = async () => {
    if (!activeProjectId || commitDisabled) return;
    const threadId = await ensureActiveThread(activeProjectId);
    await startCommit(activeProjectId, threadId);
  };

  return (
    <div className="agent-panel relative flex h-full flex-col bg-panel">
      <header className="flex h-12 shrink-0 items-center justify-between border-b border-border bg-panel px-3">
        <div className="flex items-center text-sm font-semibold text-text-900">
          <Bot className="mr-2 h-4 w-4 text-accent" strokeWidth={1.75} />
          智能体
        </div>
        <div className="relative flex items-center gap-0.5">
          <PlanIndicator plan={plan} running={runActive} />
          <ContextWindowPanel />
          <div ref={setPolishToolbarContainer} className="contents" />
          <IconButton
            label={commitActive ? '正在提交项目版本' : '提交项目版本'}
            expandableLabel="提交"
            onClick={() => void commit()}
            disabled={commitDisabled}
          >
            {commitActive
              ? <Loader2 className="h-4 w-4 animate-spin motion-reduce:animate-none" strokeWidth={1.75} />
              : <GitCommitHorizontal className="h-4 w-4" strokeWidth={1.75} />}
          </IconButton>
          <IconButton label="隐藏右侧对话" onClick={toggleRightPanel}>
            <PanelRightClose className="h-4 w-4" strokeWidth={1.75} />
          </IconButton>
        </div>
      </header>
      <ThreadTabs />
      <Timeline />
	  <RenamePanel />
      <CommandComposer polishToolbarContainer={polishToolbarContainer} />
    </div>
  );
};
