import { useEffect } from 'react';
import { threadsApi } from '../../api/threads';
import { currentHistoryEpoch } from '../../api/client';
import { commandActive, upsertCommand } from '../../stores/commandRuntime';
import { hydrateRunFromHistory, type HistoryEntry } from './historyHydrator';
import type { TimelineItem } from './eventReducer';

// A page refresh disconnects synchronous commands. Read their durable outcome
// until cancellation or completion settles; never leave a restored spinner stuck.
export function useCommandHistoryRecovery(threadId: string | null, items: TimelineItem[]) {
  const pending = items
    .filter((item) =>
      (item.commandRecord?.status === 'loading' || (item.type === 'git_commit' && item.status === 'loading')) &&
      !commandActive(item.id))
    .map((item) => item.id).sort().join('|');
  useEffect(() => {
    if (!threadId || !pending) return;
    let disposed = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const epoch = currentHistoryEpoch();
    const ids = new Set(pending.split('|'));
    const poll = async () => {
      try {
        const history = await threadsApi.history(threadId);
        if (disposed || epoch !== currentHistoryEpoch()) return;
        const restored = hydrateRunFromHistory(history as unknown as HistoryEntry[]);
        for (const item of restored.items) {
          if (ids.has(item.id) && !commandActive(item.id)) upsertCommand(threadId, item);
        }
      } catch { /* A later read can settle a temporarily unavailable backend. */ }
      if (!disposed && epoch === currentHistoryEpoch()) timer = setTimeout(poll, 1500);
    };
    void poll();
    return () => { disposed = true; clearTimeout(timer); };
  }, [threadId, pending]);
}
