import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  APIError,
  fetchClient,
  NetworkError,
  RequestCanceledError,
  RequestTimeoutError,
} from './client';

describe('fetchClient', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  it('returns undefined for successful empty JSON responses', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('', { status: 200 })));
    await expect(fetchClient<void>('/empty')).resolves.toBeUndefined();
  });

  it('returns undefined for no-content responses', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(null, { status: 204 })));
    await expect(fetchClient<void>('/empty')).resolves.toBeUndefined();
  });

  it('preserves structured error fields and request ID', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({
      error: { code: 'BAD_STATE', message: '当前状态不可执行', details: { stage: 'verify' } },
    }), {
      status: 409,
      headers: { 'Content-Type': 'application/json', 'X-Request-ID': 'req-123' },
    })));

    const error = await fetchClient('/broken').catch((caught: unknown) => caught) as APIError;
    expect(error).toBeInstanceOf(APIError);
    expect(error).toMatchObject({
      status: 409,
      code: 'BAD_STATE',
      message: '当前状态不可执行',
      details: { stage: 'verify' },
      requestId: 'req-123',
      retryable: false,
    });
  });

  it('uses the API retryable field as the HTTP retry authority', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify({
      error: { code: 'PROVIDER_UNAVAILABLE', message: '模型服务暂时不可用', retryable: true },
    }), {
      status: 503,
      headers: { 'Content-Type': 'application/json' },
    })));

    const error = await fetchClient('/broken').catch((caught: unknown) => caught) as APIError;
    expect(error).toMatchObject({ code: 'PROVIDER_UNAVAILABLE', retryable: true });
  });

  it('does not infer retryability from an HTTP status without an authoritative field', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('gateway unavailable', { status: 502 })));
    const error = await fetchClient('/broken').catch((caught: unknown) => caught) as APIError;
    expect(error).toBeInstanceOf(APIError);
    expect(error.message).toBe('gateway unavailable');
    expect(error.details).toBe('gateway unavailable');
    expect(error.retryable).toBe(false);
  });

  it('distinguishes a user abort', async () => {
    const controller = new AbortController();
    vi.stubGlobal('fetch', vi.fn((_input, init) => new Promise((_resolve, reject) => {
      init?.signal?.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')));
    })));
    const request = fetchClient('/slow', { signal: controller.signal });
    controller.abort();
    await expect(request).rejects.toBeInstanceOf(RequestCanceledError);
  });

  it('distinguishes a timeout', async () => {
    vi.useFakeTimers();
    vi.stubGlobal('fetch', vi.fn((_input, init) => new Promise((_resolve, reject) => {
      init?.signal?.addEventListener('abort', () => reject(new DOMException('aborted', 'AbortError')));
    })));
    const request = fetchClient('/slow', { timeoutMs: 50 });
    const assertion = expect(request).rejects.toBeInstanceOf(RequestTimeoutError);
    await vi.advanceTimersByTimeAsync(50);
    await assertion;
  });

  it('distinguishes a network failure', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('offline'); }));
    await expect(fetchClient('/offline')).rejects.toBeInstanceOf(NetworkError);
  });
});
