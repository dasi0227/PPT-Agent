const API_BASE = '/api/v1';
const DEFAULT_TIMEOUT_MS = 15_000;

export class APIError extends Error {
  readonly retryable: boolean;

  constructor(
    public readonly status: number,
    public readonly code: string | undefined,
    message: string,
    public readonly details?: unknown,
    public readonly requestId?: string,
    retryable?: boolean,
  ) {
    super(message);
    this.name = 'APIError';
    this.retryable = retryable === true;
  }
}

export class RequestTimeoutError extends Error {
  readonly retryable = true;
  constructor(public readonly timeoutMs: number) {
    super(`请求超过 ${Math.round(timeoutMs / 1000)} 秒未完成，请重试`);
    this.name = 'RequestTimeoutError';
  }
}

export class RequestCanceledError extends Error {
  readonly retryable = false;
  constructor() {
    super('请求已取消');
    this.name = 'RequestCanceledError';
  }
}

export class NetworkError extends Error {
  readonly retryable = true;
  constructor(public readonly cause: unknown) {
    super('网络连接失败，请检查网络后重试');
    this.name = 'NetworkError';
  }
}

export interface FetchClientOptions extends RequestInit {
  timeoutMs?: number;
  responseType?: 'json' | 'text';
}

function combineSignals(signals: Array<AbortSignal | undefined>): { signal: AbortSignal; cleanup: () => void } {
  const controller = new AbortController();
  const listeners: Array<() => void> = [];
  for (const signal of signals) {
    if (!signal) continue;
    if (signal.aborted) {
      controller.abort(signal.reason);
      break;
    }
    const abort = () => controller.abort(signal.reason);
    signal.addEventListener('abort', abort, { once: true });
    listeners.push(() => signal.removeEventListener('abort', abort));
  }
  return { signal: controller.signal, cleanup: () => listeners.forEach((remove) => remove()) };
}

async function readErrorBody(response: Response): Promise<unknown> {
  const text = await response.text();
  if (!text) return undefined;
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return text;
  }
}

function errorFields(body: unknown): { code?: string; message?: string; details?: unknown; retryable?: boolean } {
  if (!body || typeof body !== 'object') {
    return typeof body === 'string' ? { message: body } : {};
  }
  const error = (body as { error?: unknown }).error;
  if (!error || typeof error !== 'object') return {};
  const raw = error as Record<string, unknown>;
  return {
    code: typeof raw.code === 'string' ? raw.code : undefined,
    message: typeof raw.message === 'string' ? raw.message : undefined,
    details: raw.details,
    retryable: typeof raw.retryable === 'boolean' ? raw.retryable : undefined,
  };
}

export async function fetchClient<T>(path: string, options: FetchClientOptions = {}): Promise<T> {
  const url = `${API_BASE}${path}`;
  const {
    timeoutMs = DEFAULT_TIMEOUT_MS,
    responseType = 'json',
    signal: externalSignal,
    ...requestOptions
  } = options;
  const headers = {
    'Content-Type': 'application/json',
    ...options.headers,
  };
  const timeoutController = new AbortController();
  const timeout = window.setTimeout(() => timeoutController.abort('timeout'), timeoutMs);
  const combined = combineSignals([externalSignal ?? undefined, timeoutController.signal]);

  try {
    const response = await fetch(url, { ...requestOptions, headers, signal: combined.signal });

    if (!response.ok) {
      const body = await readErrorBody(response);
      const parsed = errorFields(body);
      throw new APIError(
        response.status,
        parsed.code,
        parsed.message || `请求失败（HTTP ${response.status}）`,
        parsed.details ?? body,
        response.headers.get('X-Request-ID') ?? undefined,
        parsed.retryable,
      );
    }

    if (response.status === 204) return undefined as T;
    if (responseType === 'text') return response.text() as Promise<T>;

    const text = await response.text();
    if (text.trim() === '') return undefined as T;
    return JSON.parse(text) as T;
  } catch (error) {
    if (error instanceof APIError) throw error;
    if (timeoutController.signal.aborted) throw new RequestTimeoutError(timeoutMs);
    if (externalSignal?.aborted) throw new RequestCanceledError();
    if (error instanceof DOMException && error.name === 'AbortError') throw new RequestCanceledError();
    throw new NetworkError(error);
  } finally {
    window.clearTimeout(timeout);
    combined.cleanup();
  }
}
