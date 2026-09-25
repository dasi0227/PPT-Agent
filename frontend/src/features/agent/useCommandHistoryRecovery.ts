import { useEffect } from 'react';
import { loadThreadHistory, subscribeThreadEvents, type ThreadEvent } from '../../api/threadJournal';
import { useRunStore } from '../../stores/runStore';
import type { TimelineItem } from './eventReducer';
import type { HistoryEntry } from './historyHydrator';

// Initial history and live updates use the same ordered projection. Disconnecting
// the view closes only its subscription; execution belongs to the server.
export function useCommandHistoryRecovery(threadId: string | null, _items: TimelineItem[]) {
  useEffect(() => {
    if (!threadId) return;
    let disposed = false; let loading = true; let generation = 0;
    const entries = new Map<number, ThreadEvent>();
    const project = () => {
      if (!disposed && !loading) useRunStore.getState().syncThreadHistory(threadId,
        [...entries.values()].sort((a, b) => a.seq - b.seq) as HistoryEntry[]);
    };
    const reload = async () => {
      const requestGeneration = ++generation;
      loading = true;
      try { const history = await loadThreadHistory(threadId); if (disposed || requestGeneration !== generation) return;
        for (const event of history.events) entries.set(event.seq, event);
      } catch { /* Reconnection will retry; keep the current projection visible. */ } finally { if (requestGeneration === generation) { loading = false; project(); } }
    };
    const stop = subscribeThreadEvents(threadId, {
      event: (event) => { entries.set(event.seq, event); project(); },
      reset: (history) => { generation++; entries.clear(); for (const event of history.events) entries.set(event.seq, event); loading = false; project(); },
    });
    void reload();
    return () => { disposed = true; stop(); };
  }, [threadId]);
}
