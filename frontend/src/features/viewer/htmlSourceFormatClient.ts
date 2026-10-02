interface FormatResponse { id: number; content: string }
interface PendingFormat { source: string; cacheKey: string; resolve: (content: string) => void; timer: ReturnType<typeof setTimeout> }

const cache = new Map<string, string>();
const pending = new Map<number, PendingFormat>();
const inFlight = new Map<string, Promise<string>>();
let worker: Worker | null = null;
let nextId = 0;

function remember(cacheKey: string, content: string) {
  cache.delete(cacheKey);
  cache.set(cacheKey, content);
  if (cache.size > 8) {
    const oldestKey = cache.keys().next().value;
    if (oldestKey !== undefined) cache.delete(oldestKey);
  }
}

function failPending() {
  worker?.terminate();
  worker = null;
  pending.forEach(({ source, resolve, timer }) => {
    clearTimeout(timer);
    resolve(source);
  });
  pending.clear();
  inFlight.clear();
}

function formatterWorker(): Worker {
  if (worker) return worker;
  const formatter = new Worker(new URL('./htmlSourceFormatter.worker.ts', import.meta.url), { type: 'module' });
  worker = formatter;
  formatter.onmessage = (event: MessageEvent<FormatResponse>) => {
    if (worker !== formatter) return;
    const request = pending.get(event.data.id);
    if (!request) return;
    pending.delete(event.data.id);
    inFlight.delete(request.cacheKey);
    clearTimeout(request.timer);
    remember(request.cacheKey, event.data.content);
    request.resolve(event.data.content);
  };
  formatter.onerror = () => { if (worker === formatter) failPending(); };
  formatter.onmessageerror = () => { if (worker === formatter) failPending(); };
  return formatter;
}

export function formatHTMLForDisplay(source: string, sourceHash: string): Promise<string> {
  const cacheKey = sourceHash ? `hash:${sourceHash}` : `source:${source}`;
  const cached = cache.get(cacheKey);
  if (cached !== undefined) return Promise.resolve(cached);
  const existing = inFlight.get(cacheKey);
  if (existing) return existing;
  if (typeof Worker === 'undefined') return Promise.resolve(source);

  try {
    const formatter = formatterWorker();
    let resolve!: (content: string) => void;
    const promise = new Promise<string>(done => { resolve = done; });
    const id = ++nextId;
    inFlight.set(cacheKey, promise);
    pending.set(id, { source, cacheKey, resolve, timer: setTimeout(failPending, 10000) });
    try {
      formatter.postMessage({ id, source });
    } catch {
      failPending();
    }
    return promise;
  } catch {
    return Promise.resolve(source);
  }
}
