import { runCommand } from './commands';
import type { PolishRequest, PolishResponse } from './types';
export const polishApi = {
  polish: (_projectId: string, payload: PolishRequest, signal?: AbortSignal, onProgress?: (phase: number) => void, commandId?: string) =>
    runCommand<PolishResponse>(payload.thread_id, 'polish', payload, { signal, onProgress, commandId, feedback: payload.feedback }),
};
