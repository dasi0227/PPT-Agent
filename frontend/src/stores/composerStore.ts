import { create } from 'zustand';
import type { Artifact, RunMode, ScopeLevel } from '../api/types';

const RECENT_MODEL_KEY = 'ppt-agent-recent-model-profile-v1';
export const MAX_SELECTED_SKILLS = 3;

function initialModelProfile(): string | null {
  if (typeof localStorage === 'undefined') return null;
  return localStorage.getItem(RECENT_MODEL_KEY);
}

interface ComposerState {
  artifact: Artifact;
  level: ScopeLevel;
  mode: RunMode;
  modelProfileName: string | null;
  polishing: boolean;
  selectedSkillIds: string[];
  threadDrafts: Record<string, string>;
  userTouchedTarget: boolean;
  setArtifact: (artifact: Artifact) => void;
  setLevel: (level: ScopeLevel) => void;
  setIntent: (mode: RunMode) => void;
  setModelProfileName: (name: string) => void;
  setPolishing: (value: boolean) => void;
  setThreadDraft: (threadId: string, text: string) => void;
  clearThreadDraft: (threadId: string) => void;
  toggleSkill: (id: string) => void;
  reconcileSkills: (validIds: string[]) => void;
  applyContextDefault: (hasSlides: boolean) => void;
  resetForProject: () => void;
}

export const useComposerStore = create<ComposerState>((set) => ({
  artifact: 'ppt',
  level: 'slide',
  mode: 'execute',
  modelProfileName: initialModelProfile(),
  polishing: false,
  selectedSkillIds: [],
  threadDrafts: {},
  userTouchedTarget: false,
  setArtifact: (artifact) => set({ artifact, userTouchedTarget: true }),
  setLevel: (level) => set({ level, userTouchedTarget: true }),
  setIntent: (mode) => set({ mode }),
  setModelProfileName: (name) => {
    if (typeof localStorage !== 'undefined') localStorage.setItem(RECENT_MODEL_KEY, name);
    set({ modelProfileName: name });
  },
  setPolishing: (polishing) => set({ polishing }),
  setThreadDraft: (threadId, text) => set((state) => {
    if (text) return { threadDrafts: { ...state.threadDrafts, [threadId]: text } };
    if (!(threadId in state.threadDrafts)) return state;
    const threadDrafts = { ...state.threadDrafts };
    delete threadDrafts[threadId];
    return { threadDrafts };
  }),
  clearThreadDraft: (threadId) => set((state) => {
    if (!(threadId in state.threadDrafts)) return state;
    const threadDrafts = { ...state.threadDrafts };
    delete threadDrafts[threadId];
    return { threadDrafts };
  }),
  toggleSkill: (id) => set((state) => {
    if (state.selectedSkillIds.includes(id)) {
      return { selectedSkillIds: state.selectedSkillIds.filter((selected) => selected !== id) };
    }
    if (state.selectedSkillIds.length >= MAX_SELECTED_SKILLS) return state;
    return { selectedSkillIds: [...state.selectedSkillIds, id] };
  }),
  reconcileSkills: (validIds) => set((state) => {
    const valid = new Set(validIds);
    const selectedSkillIds = state.selectedSkillIds.filter((id) => valid.has(id)).slice(0, MAX_SELECTED_SKILLS);
    return selectedSkillIds.length === state.selectedSkillIds.length &&
      selectedSkillIds.every((id, index) => id === state.selectedSkillIds[index])
      ? state
      : { selectedSkillIds };
  }),
  applyContextDefault: (hasSlides) => set((state) => {
    if (state.userTouchedTarget) return state;
    const artifact = hasSlides ? 'ppt' : 'spec';
    const level = hasSlides ? 'slide' : 'deck';
    return state.artifact === artifact && state.level === level ? state : { artifact, level };
  }),
  resetForProject: () => set({
    artifact: 'ppt',
    level: 'slide',
    mode: 'execute',
    polishing: false,
    selectedSkillIds: [],
    threadDrafts: {},
    userTouchedTarget: false,
  }),
}));
