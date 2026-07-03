import { SSEEvent } from './types';

export interface SSEOptions {
  onMessage?: (event: SSEEvent) => void;
  onError?: (err: Event) => void;
  onClose?: () => void;
  lastEventId?: string;
}

export function subscribeRunEvents(runId: string, options: SSEOptions): () => void {
  const url = new URL(`/api/v1/runs/${runId}/events`, window.location.origin);
  // Add Last-Event-ID as a query parameter if standard EventSource is used,
  // assuming the backend fallback logic supports reading it from query params.
  // The backend run_handler.go should parse `last_event_id` query param if header is absent.
  if (options.lastEventId) {
    url.searchParams.set('last_event_id', options.lastEventId);
  }
  
  const source = new EventSource(url.toString());

  const handleMessage = (e: MessageEvent) => {
    try {
      const data = JSON.parse(e.data);
      if (options.onMessage) {
        options.onMessage({ id: e.lastEventId, event: e.type as any, data });
      }
    } catch (err) {
      console.error('Failed to parse SSE data:', err);
    }
  };

  const eventTypes = ['run.started', 'thought', 'tool_call', 'tool_result', 'progress', 'token', 'artifact', 'needs_input', 'info', 'done', 'error'];
  
  eventTypes.forEach(type => {
    source.addEventListener(type, handleMessage);
  });

  source.onerror = (err) => {
    if (options.onError) options.onError(err);
    // If done or error, we might want to close
  };

  return () => {
    source.close();
    if (options.onClose) options.onClose();
  };
}
