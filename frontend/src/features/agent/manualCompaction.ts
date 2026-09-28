import { threadsApi } from '../../api/threads';
import { useContextWindowStore } from '../../stores/contextWindowStore';
import { useRunStore } from '../../stores/runStore';
import { hydrateRunFromHistory, type HistoryEntry } from './historyHydrator';

export async function runManualCompaction(threadId: string): Promise<void> {
  const succeeded = await useContextWindowStore.getState().compact(threadId);
  if (!succeeded) return;
  const history = await threadsApi.history(threadId);
  const hydrated = hydrateRunFromHistory(history as unknown as HistoryEntry[]);
  useRunStore.getState().hydrateTimeline(
    threadId, hydrated.items, hydrated.plan, hydrated.session, hydrated.lastEventId,
  );
}
