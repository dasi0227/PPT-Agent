import { fetchClient } from './client';
import type { CommitReceipt, RunView } from './session';

export interface InteractionWrite { scene_revision: string; request_revision: string }
export interface RunAction { scene_revision: string; entity_revision: string; request_key: string }
export interface RunMutation { run: RunView; receipt: CommitReceipt }
import {
  CommandPermissionRequest,
  CreateRunRequest,
  PlanApprovalRequest,
  Run,
  RunCancelReason,
  RunInputPayload,
  ScopeExpansionRequest,
  ResourceEditApproval,
  Meta,
  Constraints,
  Outline,
  SteerRunRequest,
  SteerRunResponse,
} from './types';

export const runsApi = {
  create: (threadId: string, payload: CreateRunRequest & { scene_revision: string }) => fetchClient<Run>(`/threads/${threadId}/runs`, {
    method: 'POST',
    body: JSON.stringify(payload),
    reportError: false,
  }),
  submitInput: (runId: string, payload: RunInputPayload & InteractionWrite) => fetchClient<CommitReceipt>(`/runs/${runId}/input`, {
    method: 'POST',
    body: JSON.stringify(payload),
    reportError: false,
  }),
  submitPlanApproval: (runId: string, payload: PlanApprovalRequest & InteractionWrite) => fetchClient<CommitReceipt>(`/runs/${runId}/plan-approval`, {
    method: 'POST', body: JSON.stringify(payload), reportError: false,
  }),
  submitCommandPermission: (runId: string, payload: CommandPermissionRequest & InteractionWrite) =>
    fetchClient<CommitReceipt>(`/runs/${runId}/command-permission`, {
      method: 'POST',
      body: JSON.stringify(payload),
      reportError: false,
    }),
  submitScopeExpansion: (runId: string, payload: ScopeExpansionRequest & InteractionWrite) =>
    fetchClient<CommitReceipt>(`/runs/${runId}/scope-expansion`, {
      method: 'POST', body: JSON.stringify(payload), reportError: false,
    }),
  getResourceEditApproval: (runId: string, interactionId: string, sceneRevision: string) =>
    fetchClient<ResourceEditApproval>(`/runs/${runId}/resource-edit-approvals/${interactionId}?scene_revision=${encodeURIComponent(sceneRevision)}`, { reportError: false }),
  updateResourceEditDraft: (runId: string, interactionId: string, revision: string, draft: Meta | Constraints | Outline, sceneRevision: string) =>
    fetchClient<ResourceEditApproval>(`/runs/${runId}/resource-edit-approvals/${interactionId}/draft`, {
      method: 'PUT', body: JSON.stringify({ revision, draft, scene_revision: sceneRevision, request_revision: revision }), reportError: false,
    }),
  decideResourceEditApproval: (runId: string, interactionId: string, callId: string, revision: string, decision: 'approve' | 'reject', feedback: string | undefined, sceneRevision: string) =>
    fetchClient<CommitReceipt>(`/runs/${runId}/resource-edit-approvals/${interactionId}/decision`, {
      method: 'POST', body: JSON.stringify({ interaction_id: interactionId, call_id: callId, revision, decision, feedback, scene_revision: sceneRevision, request_revision: revision }), reportError: false,
    }),
  steer: (runId: string, payload: SteerRunRequest & { scene_revision: string }) => fetchClient<SteerRunResponse>(`/runs/${runId}/steer`, {
    method: 'POST',
    body: JSON.stringify(payload),
    reportError: false,
  }),
  cancel: (runId: string, reason: RunCancelReason, version: RunAction) =>
    fetchClient<RunMutation>(`/runs/${runId}?reason=${encodeURIComponent(reason)}`, {
    method: 'DELETE', body: JSON.stringify(version),
    reportError: false,
  }),
};
