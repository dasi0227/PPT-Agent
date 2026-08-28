import { fetchClient } from './client';
import { CancelRunResponse, Run, CreateRunRequest, RunInputPayload, PlanApprovalRequest, SteerRunRequest, SteerRunResponse } from './types';

export const runsApi = {
  get: (runId: string) => fetchClient<Run>(`/runs/${runId}`),
  activeForThread: (threadId: string) => fetchClient<Run>(`/threads/${threadId}/active-run`),
  create: (threadId: string, payload: CreateRunRequest) => fetchClient<Run>(`/threads/${threadId}/runs`, {
    method: 'POST',
    body: JSON.stringify(payload)
  }),
  submitInput: (runId: string, payload: RunInputPayload) => fetchClient<void>(`/runs/${runId}/input`, {
    method: 'POST',
    body: JSON.stringify(payload)
  }),
  submitPlanApproval: (runId: string, payload: PlanApprovalRequest) => fetchClient<void>(`/runs/${runId}/plan-approval`, {
    method: 'POST', body: JSON.stringify(payload),
  }),
  steer: (runId: string, payload: SteerRunRequest) => fetchClient<SteerRunResponse>(`/runs/${runId}/steer`, {
    method: 'POST',
    body: JSON.stringify(payload)
  }),
  resume: (runId: string) => fetchClient<Run>(`/runs/${runId}/resume`, {
    method: 'POST'
  }),
  cancel: (runId: string) => fetchClient<CancelRunResponse>(`/runs/${runId}`, {
    method: 'DELETE'
  }),
};
