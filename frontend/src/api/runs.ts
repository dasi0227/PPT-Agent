import { fetchClient } from './client';
import { Run, RunPayload, NeedsInputPayload } from './types';

export const runsApi = {
  create: (threadId: string, payload: RunPayload) => fetchClient<Run>(`/threads/${threadId}/runs`, {
    method: 'POST',
    body: JSON.stringify(payload)
  }),
  submitInput: (runId: string, payload: NeedsInputPayload) => fetchClient<void>(`/runs/${runId}/input`, {
    method: 'POST',
    body: JSON.stringify(payload)
  }),
  cancel: (runId: string) => fetchClient<void>(`/runs/${runId}`, {
    method: 'DELETE'
  }),
};
