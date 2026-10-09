import { fetchClient } from './client';
import {
  CancelRunResponse,
  CommandPermissionRequest,
  CreateRunRequest,
  PlanApprovalRequest,
  Run,
  RunCancelReason,
  RunInputPayload,
  ScopeExpansionRequest,
  ResourceEditApproval,
  Manifest,
  Design,
  Outline,
  SteerRunRequest,
  SteerRunResponse,
} from './types';

export const runsApi = {
  get: (runId: string) => fetchClient<Run>(`/runs/${runId}`, { reportError: false }),
  activeForThread: (threadId: string) => fetchClient<Run>(`/threads/${threadId}/active-run`, { reportError: false }),
  create: (threadId: string, payload: CreateRunRequest) => fetchClient<Run>(`/threads/${threadId}/runs`, {
    method: 'POST',
    body: JSON.stringify(payload),
    reportError: false,
  }),
  submitInput: (runId: string, payload: RunInputPayload) => fetchClient<void>(`/runs/${runId}/input`, {
    method: 'POST',
    body: JSON.stringify(payload),
    reportError: false,
  }),
  submitPlanApproval: (runId: string, payload: PlanApprovalRequest) => fetchClient<void>(`/runs/${runId}/plan-approval`, {
    method: 'POST', body: JSON.stringify(payload), reportError: false,
  }),
  submitCommandPermission: (runId: string, payload: CommandPermissionRequest) =>
    fetchClient<void>(`/runs/${runId}/command-permission`, {
      method: 'POST',
      body: JSON.stringify(payload),
      reportError: false,
    }),
  submitScopeExpansion: (runId: string, payload: ScopeExpansionRequest) =>
    fetchClient<void>(`/runs/${runId}/scope-expansion`, {
      method: 'POST', body: JSON.stringify(payload), reportError: false,
    }),
  getResourceEditApproval: (runId: string, interactionId: string) =>
    fetchClient<ResourceEditApproval>(`/runs/${runId}/resource-edit-approvals/${interactionId}`, { reportError: false }),
  updateResourceEditDraft: (runId: string, interactionId: string, revision: number, draft: Manifest | Design | Outline) =>
    fetchClient<ResourceEditApproval>(`/runs/${runId}/resource-edit-approvals/${interactionId}/draft`, {
      method: 'PUT', body: JSON.stringify({ revision, draft }), reportError: false,
    }),
  decideResourceEditApproval: (runId: string, interactionId: string, callId: string, revision: number, decision: 'approve' | 'reject', feedback?: string) =>
    fetchClient<void>(`/runs/${runId}/resource-edit-approvals/${interactionId}/decision`, {
      method: 'POST', body: JSON.stringify({ interaction_id: interactionId, call_id: callId, revision, decision, feedback }), reportError: false,
    }),
  steer: (runId: string, payload: SteerRunRequest) => fetchClient<SteerRunResponse>(`/runs/${runId}/steer`, {
    method: 'POST',
    body: JSON.stringify(payload),
    reportError: false,
  }),
  resume: (runId: string) => fetchClient<Run>(`/runs/${runId}/resume`, {
    method: 'POST',
    reportError: false,
  }),
  cancel: (runId: string, reason: RunCancelReason = 'user_requested') =>
    fetchClient<CancelRunResponse>(`/runs/${runId}?reason=${encodeURIComponent(reason)}`, {
    method: 'DELETE',
    reportError: false,
  }),
};
