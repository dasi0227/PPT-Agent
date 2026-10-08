import { create } from 'zustand';
import type { ApprovalResource } from '../api/types';

export interface ActiveResourceApproval {
  projectId: string;
  runId: string;
  interactionId: string;
  callId: string;
  resource: ApprovalResource;
}

export const useResourceApprovalStore = create<{
  active: ActiveResourceApproval | null;
  open: (active: ActiveResourceApproval) => void;
  close: () => void;
}>((set) => ({
  active: null,
  open: (active) => set({ active }),
  close: () => set({ active: null }),
}));
