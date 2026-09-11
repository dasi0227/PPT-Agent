import { create } from 'zustand';
import type { DOMSelection, RunMode, ScopeObject, ScopeSelectionKind } from '../api/types';

const RECENT_MODEL_KEY = 'ppt-agent-recent-model-profile-v1';
export const MAX_SELECTED_SKILLS = 3;
export const MAX_MESSAGE_ATTACHMENTS = 8;

export interface ComposerAttachment {
  attachmentId: string;
  name: string;
  size: number;
  mediaType: 'image/png' | 'image/jpeg' | 'image/webp';
}
export type ComposerReference =
  | { kind: 'image'; attachment: ComposerAttachment }
  | { kind: 'dom'; selection: DOMSelection };

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
  threadReferences: Record<string, ComposerReference[]>;
  nextMarkerByThread: Record<string, number>;
  editingSelectionIdByThread: Record<string, string | undefined>;
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
	addThreadAttachment: (threadId: string, attachment: ComposerAttachment) => void;
	removeThreadAttachment: (threadId: string, attachmentId: string) => void;
  addThreadDOMSelection: (threadId: string, selection: DOMSelection) => void;
  updateThreadDOMSelection: (threadId: string, selectionId: string, update: Partial<Pick<DOMSelection, 'comment' | 'status' | 'dom_targets'>>) => void;
  removeThreadDOMSelection: (threadId: string, selectionId: string) => void;
  setEditingDOMSelection: (threadId: string, selectionId?: string) => void;
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
  threadReferences: {},
  nextMarkerByThread: {},
  editingSelectionIdByThread: {},
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
	const threadDrafts = { ...state.threadDrafts };
	const threadReferences = { ...state.threadReferences };
	const nextMarkerByThread = { ...state.nextMarkerByThread };
	const editingSelectionIdByThread = { ...state.editingSelectionIdByThread };
	delete threadDrafts[threadId];
	delete threadReferences[threadId];
	delete nextMarkerByThread[threadId];
	delete editingSelectionIdByThread[threadId];
	return { threadDrafts, threadReferences, nextMarkerByThread, editingSelectionIdByThread };
  }),
	addThreadAttachment: (threadId, attachment) => set((state) => {
		const existing = state.threadReferences[threadId] ?? [];
		if (existing.some((candidate) => candidate.kind === 'image' && candidate.attachment.attachmentId === attachment.attachmentId) || existing.filter((candidate) => candidate.kind === 'image').length >= MAX_MESSAGE_ATTACHMENTS) return state;
		return { threadReferences: { ...state.threadReferences, [threadId]: [...existing, { kind: 'image', attachment }] } };
	}),
	removeThreadAttachment: (threadId, attachmentId) => set((state) => {
		const existing = state.threadReferences[threadId] ?? [];
		const next = existing.filter((candidate) => candidate.kind !== 'image' || candidate.attachment.attachmentId !== attachmentId);
		if (next.length === existing.length) return state;
		const threadReferences = { ...state.threadReferences };
		if (next.length === 0) delete threadReferences[threadId];
		else threadReferences[threadId] = next;
		return { threadReferences };
	}),
  addThreadDOMSelection: (threadId, selection) => set((state) => {
    const existing = state.threadReferences[threadId] ?? [];
    if (existing.some((item) => item.kind === 'dom' && (item.selection.selection_id === selection.selection_id || item.selection.dedupe_key === selection.dedupe_key)) || existing.filter((item) => item.kind === 'dom').length >= 8) return state;
    return {
      threadReferences: { ...state.threadReferences, [threadId]: [...existing, { kind: 'dom', selection }] },
      nextMarkerByThread: { ...state.nextMarkerByThread, [threadId]: Math.max(state.nextMarkerByThread[threadId] ?? 1, selection.marker_no + 1) },
    };
  }),
  updateThreadDOMSelection: (threadId, selectionId, update) => set((state) => ({
    threadReferences: { ...state.threadReferences, [threadId]: (state.threadReferences[threadId] ?? []).map((item) => item.kind === 'dom' && item.selection.selection_id === selectionId ? { ...item, selection: { ...item.selection, ...update } } : item) },
  })),
  removeThreadDOMSelection: (threadId, selectionId) => set((state) => {
    const next = (state.threadReferences[threadId] ?? []).filter((item) => item.kind !== 'dom' || item.selection.selection_id !== selectionId);
    const threadReferences = { ...state.threadReferences };
    if (next.length === 0) delete threadReferences[threadId]; else threadReferences[threadId] = next;
    const editingSelectionIdByThread = { ...state.editingSelectionIdByThread };
    if (editingSelectionIdByThread[threadId] === selectionId) delete editingSelectionIdByThread[threadId];
    return { threadReferences, editingSelectionIdByThread };
  }),
  setEditingDOMSelection: (threadId, selectionId) => set((state) => ({ editingSelectionIdByThread: { ...state.editingSelectionIdByThread, [threadId]: selectionId } })),
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
    mode: 'execute', polishing: false, selectedSkillIds: [], threadDrafts: {}, threadReferences: {}, nextMarkerByThread: {}, editingSelectionIdByThread: {}, userTouchedTarget: false,
  }),
}));
