import { create } from 'zustand';
import type { Artifact, InteractionIntent, TargetLevel } from '../api/types';

interface ComposerState {
  artifact: Artifact;
  level: TargetLevel;
  intent: InteractionIntent;
  userTouchedTarget: boolean;
  setArtifact: (artifact: Artifact) => void;
  setLevel: (level: TargetLevel) => void;
  setIntent: (intent: InteractionIntent) => void;
  applyContextDefault: (hasSlides: boolean) => void;
  resetForProject: () => void;
}

export const useComposerStore = create<ComposerState>((set) => ({
  artifact: 'presentation',
  level: 'slide',
  intent: 'execute',
  userTouchedTarget: false,
  setArtifact: (artifact) => set({ artifact, userTouchedTarget: true }),
  setLevel: (level) => set({ level, userTouchedTarget: true }),
  setIntent: (intent) => set({ intent }),
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
