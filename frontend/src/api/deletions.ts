import { fetchClient } from './client';

export interface PendingDeletion {
  kind: 'project' | 'thread';
  id: string;
  project_id: string;
  token: string;
  title: string;
  undo_until: number;
}

export interface DeletionResponse {
  deletion: PendingDeletion;
  server_now: number;
}

export interface DeletionNotice extends PendingDeletion {
  expiresAt: number;
}

// Deduct the entire round trip so clock skew cannot extend the server deadline.
export function deletionNotice(deletion: PendingDeletion, serverNow: number, startedAt: number): DeletionNotice {
  return { ...deletion, expiresAt: startedAt + Math.max(0, deletion.undo_until - serverNow) };
}

export const deletionsApi = {
  list: () => fetchClient<{ deletions: PendingDeletion[]; server_now: number }>('/deletions', { reportError: false }),
  undo: (deletion: PendingDeletion) => fetchClient<void>(`/deletions/${deletion.kind}/${encodeURIComponent(deletion.id)}/undo`, {
    method: 'POST', body: JSON.stringify({ token: deletion.token }), reportError: false,
  }),
};
