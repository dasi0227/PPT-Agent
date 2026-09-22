import type { CommandTimelineItem } from './eventReducer';
import { CommandActivity } from './CommandActivity';
import { cancelCommand, retryCommand } from '../../stores/commandRuntime';
import { retryPolish } from '../../stores/textCommandStore';
import { retryRecordedCommand } from '../../stores/commandRecovery';
import { useComposerStore } from '../../stores/composerStore';
import { useActiveThreadId } from './useActiveSession';
export function TextCommandActivity({ item }: { item: CommandTimelineItem }) {
  const threadId = useActiveThreadId();
  const polishing = useComposerStore((state) => state.polishing);
  return (
    <CommandActivity
      kind={item.kind}
      title={item.title}
      timestamp={item.timestamp}
      status={item.status}
      phase={item.phase}
      cancellable={item.cancellable}
      metadata={
        item.kind === 'rename' ? (
          <span>{item.method === 'manual' ? '手动' : '自动'}</span>
        ) : item.kind === 'compact' ? (
          <span>{item.method === 'auto' ? '自动' : '手动'}</span>
        ) : undefined
      }
      content={item.kind === 'rename' ? undefined : item.content}
      onCancel={() => cancelCommand(item.id)}
      onRetry={() => item.commandRecord ? void retryRecordedCommand(item.commandRecord) : retryCommand(item.id)}
      busy={item.kind === 'polish' && polishing}
      onRevise={item.kind === 'polish' ? (feedback) => item.commandRecord
        ? retryRecordedCommand(item.commandRecord, feedback) : retryPolish(item.id, feedback) : undefined}
      onPrimary={
        item.kind === 'polish'
          ? () => {
              if (threadId)
                useComposerStore.getState().setThreadDraft(threadId, item.content ?? '');
            }
          : undefined
      }
      primaryLabel="应用到输入框"
    >
      {item.kind === 'rename' && <p>{item.content}</p>}
    </CommandActivity>
  );
}
