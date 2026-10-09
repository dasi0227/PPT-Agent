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
  editStates: Record<string, { phase: 'editing' | 'review'; revision?: number; dirty: boolean; busy: boolean }>;
  open: (active: ActiveResourceApproval) => void;
  close: () => void;
  editorState: (key: string, dirty: boolean, busy: boolean) => void;
  saved: (key: string, revision: number) => void;
  finish: (key: string) => void;
}>((set) => ({
  active: null,
  editStates: {},
  open: (active) => set(state => ({ active, editStates: { ...state.editStates,
    [resourceApprovalKey(active.runId, active.interactionId)]: { phase: 'editing', dirty: false, busy: false } } })),
  close: () => set({ active: null }),
  editorState: (key, dirty, busy) => set(state => state.editStates[key] ? ({ editStates: { ...state.editStates,
    [key]: { ...state.editStates[key], phase: dirty || busy ? 'editing' : state.editStates[key].phase, dirty, busy } } }) : state),
  saved: (key, revision) => set(state => state.editStates[key] ? ({ editStates: { ...state.editStates,
    [key]: { phase: 'review', revision, dirty: false, busy: false } } }) : state),
  finish: (key) => set(state => {
    const editStates = { ...state.editStates }; delete editStates[key];
    return { editStates, active: state.active && resourceApprovalKey(state.active.runId, state.active.interactionId) === key ? null : state.active };
  }),
}));

export function resourceApprovalKey(runId: string, interactionId: string) { return `${runId}:${interactionId}`; }
