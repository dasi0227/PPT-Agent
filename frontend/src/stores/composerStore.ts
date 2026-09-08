import { create } from 'zustand';
import type { RunMode, ScopeObject, ScopeSelectionKind } from '../api/types';

const RECENT_MODEL_KEY = 'ppt-agent-recent-model-profile-v1';
export const MAX_SELECTED_SKILLS = 3;

function initialModelProfile(): string | null {
  if (typeof localStorage === 'undefined') return null;
  return localStorage.getItem(RECENT_MODEL_KEY);
}

interface ComposerState {
  scopeObject: ScopeObject;
  scopeSelection: ScopeSelectionKind;
  lastNonGlobalSelection: ScopeSelectionKind;
  customSlideIds: string[];
  customSectionIds: string[];
  mode: RunMode;
  modelProfileName: string | null;
  polishing: boolean;
  selectedSkillIds: string[];
  threadDrafts: Record<string, string>;
  userTouchedTarget: boolean;
  setScopeObject: (object: ScopeObject) => void;
  setScopeSelection: (selection: ScopeSelectionKind) => void;
  toggleCustomSlide: (slideId: string) => void;
  toggleCustomSection: (sectionId: string) => void;
  setIntent: (mode: RunMode) => void;
  setModelProfileName: (name: string) => void;
  setPolishing: (value: boolean) => void;
  setThreadDraft: (threadId: string, text: string) => void;
  clearThreadDraft: (threadId: string) => void;
  toggleSkill: (id: string) => void;
  reconcileSkills: (validIds: string[]) => void;
  reconcileScopeIds: (slideIds: string[], sectionIds: string[]) => void;
  applyContextDefault: (hasSlides: boolean) => void;
  resetForProject: () => void;
}

const toggleId = (ids: string[], id: string) => (
  ids.includes(id) ? ids.filter((candidate) => candidate !== id) : [...ids, id]
);

export const useComposerStore = create<ComposerState>((set) => ({
  scopeObject: 'presentation',
  scopeSelection: 'current_page',
  lastNonGlobalSelection: 'current_page',
  customSlideIds: [],
  customSectionIds: [],
  mode: 'execute',
  modelProfileName: initialModelProfile(),
  polishing: false,
  selectedSkillIds: [],
  threadDrafts: {},
  userTouchedTarget: false,
  setScopeObject: (scopeObject) => set((state) => {
    if (scopeObject === 'global') {
      return state.scopeObject === 'global'
        ? { userTouchedTarget: true }
        : { scopeObject, scopeSelection: 'all_pages', lastNonGlobalSelection: state.scopeSelection, userTouchedTarget: true };
    }
    return {
      scopeObject,
      ...(state.scopeObject === 'global' ? { scopeSelection: state.lastNonGlobalSelection } : {}),
      userTouchedTarget: true,
    };
  }),
  setScopeSelection: (scopeSelection) => set((state) => (
    state.scopeObject === 'global' ? state : { scopeSelection, lastNonGlobalSelection: scopeSelection, userTouchedTarget: true }
  )),
  toggleCustomSlide: (slideId) => set((state) => ({ customSlideIds: toggleId(state.customSlideIds, slideId), userTouchedTarget: true })),
  toggleCustomSection: (sectionId) => set((state) => ({ customSectionIds: toggleId(state.customSectionIds, sectionId), userTouchedTarget: true })),
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
    if (state.selectedSkillIds.includes(id)) return { selectedSkillIds: state.selectedSkillIds.filter((selected) => selected !== id) };
    if (state.selectedSkillIds.length >= MAX_SELECTED_SKILLS) return state;
    return { selectedSkillIds: [...state.selectedSkillIds, id] };
  }),
  reconcileSkills: (validIds) => set((state) => {
    const valid = new Set(validIds);
    const selectedSkillIds = state.selectedSkillIds.filter((id) => valid.has(id)).slice(0, MAX_SELECTED_SKILLS);
    return selectedSkillIds.length === state.selectedSkillIds.length && selectedSkillIds.every((id, index) => id === state.selectedSkillIds[index])
      ? state : { selectedSkillIds };
  }),
  reconcileScopeIds: (slideIds, sectionIds) => set((state) => {
    const slides = new Set(slideIds);
    const sections = new Set(sectionIds);
    const customSlideIds = state.customSlideIds.filter((id) => slides.has(id));
    const customSectionIds = state.customSectionIds.filter((id) => sections.has(id));
    return customSlideIds.length === state.customSlideIds.length && customSectionIds.length === state.customSectionIds.length
      ? state
      : { customSlideIds, customSectionIds };
  }),
  applyContextDefault: (hasSlides) => set((state) => {
    if (state.userTouchedTarget) return state;
    const scopeObject: ScopeObject = hasSlides ? 'presentation' : 'spec';
    const scopeSelection: ScopeSelectionKind = hasSlides ? 'current_page' : 'all_pages';
    return state.scopeObject === scopeObject && state.scopeSelection === scopeSelection ? state : { scopeObject, scopeSelection, lastNonGlobalSelection: scopeSelection };
  }),
  resetForProject: () => set({
    scopeObject: 'presentation', scopeSelection: 'current_page', lastNonGlobalSelection: 'current_page', customSlideIds: [], customSectionIds: [],
    mode: 'execute', polishing: false, selectedSkillIds: [], threadDrafts: {}, userTouchedTarget: false,
  }),
}));
