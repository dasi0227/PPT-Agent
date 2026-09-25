import { fetchClient } from './client';

export interface ThreadEvent {
  id: string; seq: number; ts: number; type: string; run_id?: string;
  command_id?: string; attempt_id?: string; turn: string; data: Record<string, unknown>;
}
export interface ThreadHistory {
  events: ThreadEvent[]; cursor: string; scene_revision: number; thread_id: string; seq: number;
}
const cursors = new Map<string, string>();
export async function loadThreadHistory(id: string): Promise<ThreadHistory> {
  const history = await fetchClient<ThreadHistory>(`/threads/${id}/history`, { reportError: false });
  cursors.set(id, history.cursor);
  return history;
}
interface Listener { event: (event: ThreadEvent) => void; reset?: (history: ThreadHistory) => void; status?: (status: 'connecting' | 'open' | 'reconnecting') => void }
interface Connection { source: EventSource; listeners: Set<Listener>; reconnect?: ReturnType<typeof setTimeout>; refreshing: boolean }
const connections = new Map<string, Connection>();
export function subscribeThreadEvents(threadId: string, listener: Listener): () => void {
  let connection = connections.get(threadId);
  if (!connection) {
    const listeners = new Set<Listener>();
    const open = () => {
      const url = new URL(`/api/v1/threads/${threadId}/events`, window.location.origin);
      const cursor = cursors.get(threadId); if (cursor) url.searchParams.set('cursor', cursor);
      const source = new EventSource(url.toString());
      source.addEventListener('thread.event', (message: MessageEvent) => {
        try {
          const event = JSON.parse(String(message.data)) as ThreadEvent;
          if (!Number.isSafeInteger(event.seq) || event.seq < 1 || typeof event.type !== 'string') throw new Error('invalid event');
          cursors.set(threadId, message.lastEventId);
          listeners.forEach((current) => current.event(event));
        } catch { source.close(); void recover(); }
      });
      source.addEventListener('history.reset', () => { source.close(); void recover(); });
      source.onopen = () => listeners.forEach((current) => current.status?.('open'));
      source.onerror = () => { source.close(); void recover(); };
      return source;
    };
    const recover = async () => {
      const current = connections.get(threadId);
      if (!current || current.refreshing) return;
      current.refreshing = true;
      listeners.forEach((value) => value.status?.('reconnecting'));
      try {
        const history = await loadThreadHistory(threadId);
        if (connections.get(threadId) !== current) return;
        listeners.forEach((value) => { value.reset?.(history); history.events.forEach(value.event); });
      } catch { /* Keep the durable cursor and retry when connectivity returns. */ }
      finally {
        if (connections.get(threadId) === current) current.reconnect = setTimeout(() => {
          current.refreshing = false; current.source = open();
        }, 1000);
      }
    };
    connection = { source: open(), listeners, refreshing: false };
    connections.set(threadId, connection);
  }
  connection.listeners.add(listener); listener.status?.('connecting');
  return () => {
    connection.listeners.delete(listener);
    if (connection.listeners.size === 0) {
      connection.source.close(); clearTimeout(connection.reconnect); connections.delete(threadId);
    }
  };
}
