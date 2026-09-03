import { fetchClient } from './client';
import type { BriefingKind, BriefingRequest, BriefingResponse } from './types';

export const briefingsApi = {
  generate: (
    projectId: string,
    kind: BriefingKind,
    payload: BriefingRequest,
    signal?: AbortSignal,
  ) => fetchClient<BriefingResponse>(`/projects/${projectId}/${kind}`, {
    method: 'POST',
    body: JSON.stringify(payload),
    signal,
    timeoutMs: 50_000,
  }),
};
