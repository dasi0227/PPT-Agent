interface FormatResponse { id: number; content: string }
interface PendingFormat { source: string; cacheKey: string; resolve: (content: string) => void }

const cache = new Map<string, string>();
const pending = new Map<number, PendingFormat>();
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
  pending.forEach(({ source, resolve }) => resolve(source));
  pending.clear();
}

function formatterWorker(): Worker {
  if (worker) return worker;
  worker = new Worker(new URL('./htmlSourceFormatter.worker.ts', import.meta.url), { type: 'module' });
  worker.onmessage = (event: MessageEvent<FormatResponse>) => {
    const request = pending.get(event.data.id);
    if (!request) return;
    pending.delete(event.data.id);
    remember(request.cacheKey, event.data.content);
    request.resolve(event.data.content);
  };
  worker.onerror = failPending;
  worker.onmessageerror = failPending;
  return worker;
}

export function formatHTMLForDisplay(source: string, sourceHash: string): Promise<string> {
  const cacheKey = sourceHash || source;
  const cached = cache.get(cacheKey);
  if (cached !== undefined) return Promise.resolve(cached);
  if (typeof Worker === 'undefined') return Promise.resolve(source);

  try {
    const formatter = formatterWorker();
    return new Promise((resolve) => {
      const id = ++nextId;
      pending.set(id, { source, cacheKey, resolve });
      try {
        formatter.postMessage({ id, source });
      } catch {
        failPending();
      }
    });
  } catch {
    return Promise.resolve(source);
  }
}
