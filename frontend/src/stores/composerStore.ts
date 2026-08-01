import { create } from 'zustand';
import type { Artifact, ClarificationPolicy, InteractionIntent, TargetLevel } from '../api/types';

interface ComposerState {
  artifact: Artifact;
  level: TargetLevel;
  intent: InteractionIntent;
  clarification: ClarificationPolicy;
  userTouchedTarget: boolean;
  setArtifact: (artifact: Artifact) => void;
  setLevel: (level: TargetLevel) => void;
  setIntent: (intent: InteractionIntent) => void;
  setClarification: (clarification: ClarificationPolicy) => void;
  applyContextDefault: (hasSlides: boolean) => void;
  resetForProject: () => void;
}

export const useComposerStore = create<ComposerState>((set) => ({
  artifact: 'presentation',
  level: 'slide',
  intent: 'apply',
  clarification: 'when_blocked',
  userTouchedTarget: false,
  setArtifact: (artifact) => set({ artifact, userTouchedTarget: true }),
  setLevel: (level) => set({ level, userTouchedTarget: true }),
  setIntent: (intent) => set({ intent }),
  setClarification: (clarification) => set({ clarification }),
  applyContextDefault: (hasSlides) => set((state) => {
    if (state.userTouchedTarget) return state;
    const artifact = hasSlides ? 'presentation' : 'blueprint';
    const level = hasSlides ? 'slide' : 'deck';
    return state.artifact === artifact && state.level === level ? state : { artifact, level };
  }),
  resetForProject: () => set({
    artifact: 'presentation',
    level: 'slide',
    intent: 'apply',
    clarification: 'when_blocked',
    userTouchedTarget: false,
  }),
}));
