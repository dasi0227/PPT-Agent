import { runCommand } from './commands';
import type { BriefingKind, BriefingRequest, BriefingResponse } from './types';
export const briefingsApi = {
  generate: (_projectId: string, kind: BriefingKind, payload: BriefingRequest, signal?: AbortSignal, onProgress?: (phase: number) => void, commandId?: string) =>
    runCommand<BriefingResponse>(payload.thread_id, kind, {}, { signal, onProgress, commandId, feedback: payload.feedback }),
};
