import { cancelPersistedCommand } from '../../api/commands';
import { showGlobalError } from '../../stores/toastStore';
import type { GitCommitPhase } from '../../api/types';
import { useGitCommitStore } from '../../stores/gitCommitStore';
import { useProjectStore } from '../../stores/projectStore';
import { useActiveThreadId } from './useActiveSession';
import type { GitCommitTimelineItem } from './eventReducer';
import { CommandActivity } from './CommandActivity';
export function GitCommitProgress({ phase }: { phase: GitCommitPhase | null }) {
  const projectId = useProjectStore((state) => state.activeProjectId);
  const session = useGitCommitStore((state) => (projectId ? state.sessions[projectId] : undefined));
  return (
    <CommandActivity
      kind="commit"
      title="提交项目版本"
      status="loading"
      timestamp={session?.startedAt ?? Date.now()}
      phase={session?.displayPhase ?? -1}
      cancellable={
        !!session?.operationId && !session.canceling && !session.settling && phase !== 'committing'
      }
      onCancel={() => {
        if (projectId) void useGitCommitStore.getState().cancel(projectId);
      }}
    />
  );
}
export function GitCommitEvent({ item }: { item: GitCommitTimelineItem }) {
  const projectId = useProjectStore((state) => state.activeProjectId),
    threadId = useActiveThreadId();
  const busy = useGitCommitStore((state) =>
    projectId
      ? ['creating', 'running'].includes(state.sessions[projectId]?.status ?? 'idle')
      : false,
  );
  return (
    <CommandActivity
      kind="commit"
      title={item.title ?? '提交项目版本'}
      timestamp={item.timestamp}
      status={item.status}
      busy={busy}
      phase={item.phase}
      cancellable={item.cancellable}
      onCancel={() => { void cancelPersistedCommand(item.operationId).catch((error) => showGlobalError(error instanceof Error ? error.message : '停止命令失败')); }}
      onRetry={item.retryable === false ? undefined : () => {
        if (projectId && threadId) void useGitCommitStore.getState().start(projectId, threadId, item.operationId);
      }}
      metadata={
        item.status === 'completed' && item.hash ? (
          <>
            <span>
              {item.branch} {item.hash}
            </span>
            <span aria-hidden="true">·</span>
            <span>{item.filesChanged} files</span>
            <span aria-hidden="true">·</span>
            <span className="text-success">+{item.insertions}</span>
            <span className="text-danger">−{item.deletions}</span>
          </>
        ) : undefined
      }
    >
      <ul className="space-y-1">
        {item.items?.map((text, index) => (
          <li className="flex gap-2" key={index}>
            <span className="text-success">•</span>
            <span>{text}</span>
          </li>
        ))}
      </ul>
    </CommandActivity>
  );
}
