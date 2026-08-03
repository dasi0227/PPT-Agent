import { create } from 'zustand';
import type { Artifact, InteractionIntent, TargetLevel } from '../api/types';

const RECENT_MODEL_KEY = 'ppt-agent-recent-model-profile-v1';

function initialModelProfile(): string | null {
  if (typeof localStorage === 'undefined') return null;
  return localStorage.getItem(RECENT_MODEL_KEY);
}

interface ComposerState {
  artifact: Artifact;
  level: TargetLevel;
  intent: InteractionIntent;
  modelProfileName: string | null;
  userTouchedTarget: boolean;
  setArtifact: (artifact: Artifact) => void;
  setLevel: (level: TargetLevel) => void;
  setIntent: (intent: InteractionIntent) => void;
  setModelProfileName: (name: string) => void;
  applyContextDefault: (hasSlides: boolean) => void;
  resetForProject: () => void;
}

export const useComposerStore = create<ComposerState>((set) => ({
  artifact: 'presentation',
  level: 'slide',
  intent: 'execute',
  modelProfileName: initialModelProfile(),
  userTouchedTarget: false,
  setArtifact: (artifact) => set({ artifact, userTouchedTarget: true }),
  setLevel: (level) => set({ level, userTouchedTarget: true }),
  setIntent: (intent) => set({ intent }),
  setModelProfileName: (name) => {
    if (typeof localStorage !== 'undefined') localStorage.setItem(RECENT_MODEL_KEY, name);
    set({ modelProfileName: name });
  },
  applyContextDefault: (hasSlides) => set((state) => {
    if (state.userTouchedTarget) return state;
    const artifact = hasSlides ? 'presentation' : 'spec';
    const level = hasSlides ? 'slide' : 'deck';
    return state.artifact === artifact && state.level === level ? state : { artifact, level };
  }),
  resetForProject: () => set({
    artifact: 'presentation',
    level: 'slide',
    intent: 'execute',
    userTouchedTarget: false,
  }),
}));
