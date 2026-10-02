import { afterEach, beforeEach, expect, it, vi } from 'vitest';

class FormatWorker {
  static instances: FormatWorker[] = [];
  onmessage: ((event: MessageEvent<{ id: number; content: string }>) => void) | null = null;
  onerror: (() => void) | null = null;
  onmessageerror: (() => void) | null = null;
  postMessage = vi.fn<(request: { id: number; source: string }) => void>();
  terminate = vi.fn();
  constructor() { FormatWorker.instances.push(this); }
  reply(index: number, content: string) {
    const request = this.postMessage.mock.calls[index][0];
    this.onmessage?.({ data: { id: request.id, content } } as MessageEvent<{ id: number; content: string }>);
  }
}

beforeEach(() => {
  vi.resetModules();
  FormatWorker.instances = [];
  vi.stubGlobal('Worker', FormatWorker);
});

afterEach(() => {
  FormatWorker.instances.forEach(worker => worker.onerror?.());
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

it('merges concurrent requests by version and caches independently completed versions', async () => {
  const { formatHTMLForDisplay } = await import('./htmlSourceFormatClient');
  const a = formatHTMLForDisplay('<main>A</main>', 'a');
  expect(formatHTMLForDisplay('<main>A</main>', 'a')).toBe(a);
  const b = formatHTMLForDisplay('<main>B</main>', 'b');
  const worker = FormatWorker.instances[0];
  expect(worker.postMessage).toHaveBeenCalledTimes(2);
  worker.reply(1, 'formatted B');
  worker.reply(0, 'formatted A');
  expect(await a).toBe('formatted A');
  expect(await b).toBe('formatted B');
  expect(await formatHTMLForDisplay('<main>A</main>', 'a')).toBe('formatted A');
  expect(worker.postMessage).toHaveBeenCalledTimes(2);
});

it('falls back on worker failure and ignores late events from the failed worker', async () => {
  const { formatHTMLForDisplay } = await import('./htmlSourceFormatClient');
  const original = '<main>A</main>';
  const request = formatHTMLForDisplay(original, 'a');
  const failed = FormatWorker.instances[0];
  failed.onerror?.();
  expect(await request).toBe(original);
  const next = formatHTMLForDisplay('<main>B</main>', 'b');
  failed.onerror?.();
  failed.reply(0, 'stale A');
  const current = FormatWorker.instances[1];
  expect(current.terminate).not.toHaveBeenCalled();
  current.reply(0, 'formatted B');
  expect(await next).toBe('formatted B');
});

it('falls back to raw source when the worker is unavailable or times out', async () => {
  vi.useFakeTimers();
  const { formatHTMLForDisplay } = await import('./htmlSourceFormatClient');
  const source = '<main>Raw</main>';
  const request = formatHTMLForDisplay(source, 'timeout');
  await vi.advanceTimersByTimeAsync(10000);
  expect(await request).toBe(source);
  vi.stubGlobal('Worker', undefined);
  expect(await formatHTMLForDisplay(source, 'no-worker')).toBe(source);
});
