import { SSEEvent } from './types';

export interface SSEOptions {
  onMessage?: (event: SSEEvent) => void;
  onError?: (err: Event) => void;
  onClose?: () => void;
  lastEventId?: string;
}

export function subscribeRunEvents(runId: string, options: SSEOptions): () => void {
  const url = new URL(`/api/v1/runs/${runId}/events`, window.location.origin);
  // Note: Standard EventSource doesn't support setting Last-Event-ID header directly in constructor.
  // It handles it automatically for reconnects.
  // For initial custom Last-Event-ID, we might need fetch-stream, but let's stick to standard EventSource for MVP 
  // unless we use fetch-stream polyfill. We can pass it as a query param if backend supports, but spec says Header.
  // For now, EventSource is standard.
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
