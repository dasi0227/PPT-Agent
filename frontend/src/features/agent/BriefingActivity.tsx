import { useRunStore } from '../../stores/runStore';
import { useGitCommitStore } from '../../stores/gitCommitStore';
import { useBriefingStore } from '../../stores/briefingStore';
import { useComposerStore } from '../../stores/composerStore';
import { useProjectStore } from '../../stores/projectStore';
import { useThreadStore } from '../../stores/threadStore';
import { cancelCommand, retryCommand } from '../../stores/commandRuntime';
import type { BriefingTimelineItem } from './eventReducer';
import { CommandActivity } from './CommandActivity';
export function BriefingActivity({ item }: { item: BriefingTimelineItem }) {
  const projectId = useProjectStore((state) => state.activeProjectId);
  const generate = useBriefingStore((state) => state.generate);
  const busy = useBriefingStore((state) =>
    projectId ? state.sessions[projectId]?.status === 'generating' : false,
  );
  const runBusy = useRunStore((state) =>
    Object.values(state.sessions).some(
      (session) =>
        session.projectId === projectId &&
        ['creating', 'running', 'waiting', 'recovering', 'canceling'].includes(session.status),
    ),
  );
  const commitBusy = useGitCommitStore((state) =>
    projectId ? ['creating', 'running'].includes(state.sessions[projectId]?.status ?? '') : false,
  );
  const polishing = useComposerStore((state) => state.polishing);
  const version = item.versions[item.versions.length - 1];
  const content = version?.content ?? '';
  const title =
    content
      .split('\n')
      .find((line) => line.trim())
      ?.replace(/^#+\s*/, '') || (item.kind === 'kickoff' ? '启动简报' : '交接简报');
  return (
    <CommandActivity
      kind={item.kind}
      title={title}
      timestamp={item.timestamp}
      status={item.status}
      defaultOpen={item.phase === 2}
      phase={item.phase}
      cancellable={item.cancellable}
      onCancel={() => cancelCommand(item.id)}
      onRetry={() => retryCommand(item.id)}
      content={content}
      copyText={content}
      busy={busy || runBusy || commitBusy || polishing}
      onRevise={(feedback) =>
        projectId && version
          ? generate(projectId, version.thread_id, item.kind, item.briefingId, feedback)
          : Promise.resolve(false)
      }
      onPrimary={async () => {
        if (!projectId) return;
        const threadId = await useThreadStore.getState().createThread(projectId);
        useComposerStore.getState().setThreadDraft(threadId, content);
      }}
      primaryLabel="新建会话"
    />
  );
}
