import { fetchClient } from './client';
import { Run, CreateRunRequest, NeedsInputPayload } from './types';

export const runsApi = {
  get: (runId: string) => fetchClient<Run>(`/runs/${runId}`),
  create: (threadId: string, payload: CreateRunRequest) => fetchClient<Run>(`/threads/${threadId}/runs`, {
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
