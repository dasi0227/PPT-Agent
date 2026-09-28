import { useLayoutEffect } from 'react';
import { create } from 'zustand';

export type WorkspaceActionId = 'play' | 'export' | 'setting' | 'repo' | 'home';

interface WorkspaceAction {
  run: () => void;
  disabledReason?: string;
}

// Entries exist only while the UI that owns each action is mounted.
export const useWorkspaceActions = create<Record<WorkspaceActionId, WorkspaceAction | null>>(() => ({
  play: null,
  export: null,
  setting: null,
  repo: null,
  home: null,
}));

export function isWorkspaceAction(id: string): id is WorkspaceActionId {
  return id === 'play' || id === 'export' || id === 'setting' || id === 'repo' || id === 'home';
}

export function useWorkspaceAction(id: WorkspaceActionId, run: () => void, disabledReason?: string) {
  useLayoutEffect(() => {
    const action = { run, disabledReason };
    useWorkspaceActions.setState({ [id]: action });
    return () => {
      if (useWorkspaceActions.getState()[id] === action) {
        useWorkspaceActions.setState({ [id]: null });
      }
    };
  }, [id, run, disabledReason]);
}
