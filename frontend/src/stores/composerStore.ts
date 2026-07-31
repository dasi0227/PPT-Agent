import { create } from 'zustand';
import type { Artifact, ClarificationPolicy, InteractionIntent, TargetLevel } from '../api/types';

interface ComposerState {
  artifact: Artifact;
  level: TargetLevel;
  intent: InteractionIntent;
  clarification: ClarificationPolicy;
  userTouchedTarget: boolean;
  focusNonce: number;
  setArtifact: (artifact: Artifact) => void;
  setLevel: (level: TargetLevel) => void;
  setIntent: (intent: InteractionIntent) => void;
  setClarification: (clarification: ClarificationPolicy) => void;
  applyContextDefault: (hasSlides: boolean) => void;
  resetForProject: () => void;
  requestBlueprintFocus: () => void;
}

export const useComposerStore = create<ComposerState>((set) => ({
  artifact: 'presentation',
  level: 'slide',
  intent: 'apply',
  clarification: 'when_blocked',
  userTouchedTarget: false,
  focusNonce: 0,
  setArtifact: (artifact) => set({ artifact, userTouchedTarget: true }),
  setLevel: (level) => set({ level, userTouchedTarget: true }),
  setIntent: (intent) => set({ intent }),
  setClarification: (clarification) => set({ clarification }),
  applyContextDefault: (hasSlides) => set((state) => state.userTouchedTarget ? {} : {
    artifact: hasSlides ? 'presentation' : 'blueprint',
    level: hasSlides ? 'slide' : 'deck',
  }),
  resetForProject: () => set({ level: 'slide', intent: 'apply', clarification: 'when_blocked', userTouchedTarget: false }),
  requestBlueprintFocus: () => set((state) => ({
    artifact: 'blueprint',
    level: 'deck',
    intent: 'apply',
    userTouchedTarget: true,
    focusNonce: state.focusNonce + 1,
  })),
}));
