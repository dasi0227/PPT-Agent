import { SSEEvent, SSEEventName } from './types';

export interface SSEOptions {
  onMessage?: (event: SSEEvent) => void;
  onError?: (err: Event) => void;
  onStatus?: (status: 'connecting' | 'open' | 'reconnecting' | 'closed') => void;
  onUnknown?: (eventName: string, data: unknown) => void;
  onClose?: () => void;
  lastEventId?: string;
}

export const SSE_EVENT_NAMES: readonly SSEEventName[] = [
  'run.started', 'context.assembled', 'strategy.selected', 'plan.created',
  'stage.started', 'stage.completed', 'step.started', 'step.completed', 'step.failed',
  'tool.called', 'tool.completed', 'verification.completed',
  'repair.started', 'repair.completed', 'artifact.staged', 'artifact.committed',
  'status.summary', 'needs_input', 'run.completed', 'run.failed', 'run.canceled',
];

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function hasString(data: Record<string, unknown>, key: string): boolean {
  return typeof data[key] === 'string' && data[key] !== '';
}

export function parseSSEEvent(eventName: string, raw: string, id?: string): SSEEvent | null {
  if (!SSE_EVENT_NAMES.includes(eventName as SSEEventName)) return null;
  let data: unknown;
  try {
    data = JSON.parse(raw) as unknown;
  } catch {
    return null;
  }
  if (!isRecord(data)) return null;
  const valid =
    (eventName === 'strategy.selected' && hasString(data, 'strategy')) ||
    (eventName === 'plan.created' && isRecord(data.plan)) ||
    (eventName === 'tool.called' && hasString(data, 'call_id') && hasString(data, 'tool')) ||
    (eventName === 'tool.completed' && hasString(data, 'call_id')) ||
    ((eventName === 'artifact.staged' || eventName === 'artifact.committed') && isRecord(data.artifact)) ||
    (eventName === 'needs_input' && hasString(data, 'id') && hasString(data, 'prompt')) ||
    !['strategy.selected', 'plan.created', 'tool.called', 'tool.completed', 'artifact.staged', 'artifact.committed', 'needs_input'].includes(eventName);
  if (!valid) return null;
  return { id, event: eventName as SSEEventName, data } as SSEEvent;
}

export function subscribeRunEvents(runId: string, options: SSEOptions): () => void {
  const url = new URL(`/api/v1/runs/${runId}/events`, window.location.origin);
  // Add Last-Event-ID as a query parameter if standard EventSource is used,
  // assuming the backend fallback logic supports reading it from query params.
  // The backend run_handler.go should parse `last_event_id` query param if header is absent.
  if (options.lastEventId) {
    url.searchParams.set('last_event_id', options.lastEventId);
  }
  
  options.onStatus?.('connecting');
  const source = new EventSource(url.toString());
  let closed = false;

  const handleMessage = (e: MessageEvent) => {
    const event = parseSSEEvent(e.type, String(e.data), e.lastEventId || undefined);
    if (event) {
      options.onMessage?.(event);
    } else {
      options.onUnknown?.(e.type, e.data);
    }
  };

  SSE_EVENT_NAMES.forEach(type => {
    source.addEventListener(type, handleMessage);
  });
  source.onopen = () => options.onStatus?.('open');
  source.onmessage = (event) => options.onUnknown?.(event.type, event.data);

  source.onerror = (err) => {
    if (closed) return;
    options.onStatus?.('reconnecting');
    options.onError?.(err);
  };

  return () => {
    if (closed) return;
    closed = true;
    source.close();
    options.onStatus?.('closed');
    if (options.onClose) options.onClose();
  };
}
