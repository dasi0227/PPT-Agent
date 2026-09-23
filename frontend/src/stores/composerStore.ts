import { create } from 'zustand';
import type { CreateRunRequest, DOMSelection, RunMode, ScopeSelectionKind } from '../api/types';

const RECENT_MODEL_KEY = 'ppt-agent-recent-model-profile-v2';
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

export interface ComposerState {
  restoredInputs: Record<string, Partial<CreateRunRequest>>;
  threadResourceMentions: Record<string, Pick<CreateRunRequest, 'component_names' | 'mentioned_slide_ids'>>;
  scopeSelection: ScopeSelectionKind;
  customSlideIds: string[];
  customSectionIds: string[];
  mode: RunMode;
  appliedApprovalByThread: Record<string, string>;
  applyApprovedExecution: (threadId: string, runId: string) => void;
  modelProfileName: string | null;
  modelSelectionExplicit: boolean;
  reconcileModels: (names: string[], defaultName: string) => void;
  polishing: boolean;
  selectedSkillIds: string[];
  threadDrafts: Record<string, string>;
  threadReferences: Record<string, ComposerReference[]>;
  nextMarkerByThread: Record<string, number>;
  editingSelectionIdByThread: Record<string, string | undefined>;
  userTouchedTarget: boolean;
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
  updateThreadDOMSelection: (threadId: string, selectionId: string, update: Partial<Pick<DOMSelection, 'comment' | 'status' | 'rect' | 'dom_targets' | 'chrome_targets'>>) => void;
  removeThreadDOMSelection: (threadId: string, selectionId: string) => void;
  setEditingDOMSelection: (threadId: string, selectionId?: string) => void;
  toggleSkill: (id: string) => void;
  reconcileSkills: (validIds: string[]) => void;
  reconcileScopeIds: (slideIds: string[], sectionIds: string[]) => void;
  applyContextDefault: (hasSlides: boolean) => void;
  resetForProject: () => void;
}

function clearRestoredScopes(state: ComposerState) {
  return Object.fromEntries(Object.entries(state.restoredInputs).map(([id, input]) => [id, { ...input, scope: undefined }]));
}
const toggleId = (ids: string[], id: string) => (
  ids.includes(id) ? ids.filter((candidate) => candidate !== id) : [...ids, id]
);

export const useComposerStore = create<ComposerState>((set) => ({
  restoredInputs: {},
  threadResourceMentions: {},
  scopeSelection: 'all_pages',
  customSlideIds: [],
  customSectionIds: [],
  mode: 'execute',
  appliedApprovalByThread: {},
  applyApprovedExecution: (threadId, runId) => set((state) => {
    if (state.appliedApprovalByThread[threadId] === runId) return state;
    return {
      mode: 'execute',
      appliedApprovalByThread: { ...state.appliedApprovalByThread, [threadId]: runId },
    };
  }),
  modelProfileName: initialModelProfile(),
  modelSelectionExplicit: !!initialModelProfile(),
  polishing: false,
  selectedSkillIds: [],
  threadDrafts: {},
  threadReferences: {},
  nextMarkerByThread: {},
  editingSelectionIdByThread: {},
  userTouchedTarget: false,
  setScopeSelection: (scopeSelection) => set((state) => ({ restoredInputs: clearRestoredScopes(state), scopeSelection, userTouchedTarget: true })),
  toggleCustomSlide: (slideId) => set((state) => ({ restoredInputs: clearRestoredScopes(state), customSlideIds: toggleId(state.customSlideIds, slideId), userTouchedTarget: true })),
  toggleCustomSection: (sectionId) => set((state) => ({ restoredInputs: clearRestoredScopes(state), customSectionIds: toggleId(state.customSectionIds, sectionId), userTouchedTarget: true })),
  setIntent: (mode) => set({ mode }),
  setModelProfileName: (name) => {
    if (typeof localStorage !== 'undefined') localStorage.setItem(RECENT_MODEL_KEY, name);
    set({ modelProfileName: name, modelSelectionExplicit: true });
  },
  reconcileModels: (names, defaultName) => set((state) => {
    if (state.modelSelectionExplicit && state.modelProfileName && names.includes(state.modelProfileName)) return state;
    if (typeof localStorage !== 'undefined') localStorage.removeItem(RECENT_MODEL_KEY);
    return { modelProfileName: defaultName, modelSelectionExplicit: false };
  }),
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
 const threadResourceMentions = { ...state.threadResourceMentions }; delete threadResourceMentions[threadId];
 const restoredInputs = { ...state.restoredInputs }; delete restoredInputs[threadId];
	const threadReferences = { ...state.threadReferences };
	const nextMarkerByThread = { ...state.nextMarkerByThread };
	const editingSelectionIdByThread = { ...state.editingSelectionIdByThread };
	delete threadDrafts[threadId];
	delete threadReferences[threadId];
	delete nextMarkerByThread[threadId];
	delete editingSelectionIdByThread[threadId];
	return { restoredInputs, threadResourceMentions, threadDrafts, threadReferences, nextMarkerByThread, editingSelectionIdByThread };
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
    if (Object.keys(state.restoredInputs).length > 0) return state;
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
    if (state.userTouchedTarget && hasSlides) return state;
    const scopeSelection: ScopeSelectionKind = hasSlides ? 'current_page' : 'all_pages';
    const userTouchedTarget = hasSlides ? state.userTouchedTarget : false;
    return state.scopeSelection === scopeSelection && state.userTouchedTarget === userTouchedTarget
      ? state
      : { scopeSelection, userTouchedTarget };
  }),
  resetForProject: () => set({
    restoredInputs: {},
    threadResourceMentions: {},
    scopeSelection: 'all_pages', customSlideIds: [], customSectionIds: [],
    mode: 'execute', appliedApprovalByThread: {}, polishing: false, selectedSkillIds: [], threadDrafts: {}, threadReferences: {}, nextMarkerByThread: {}, editingSelectionIdByThread: {}, userTouchedTarget: false,
  }),
}));

// Pure composer edits persist per project without entering model history or
// invalidating the server's restore point.
let draftProject: string | null = null;
const draftKey = (id: string) => `ppt-agent-composer-pages-v2-${id}`;
export function composerScene(): Partial<ComposerState> {
  const s = useComposerStore.getState();
  return {
    scopeSelection: s.scopeSelection,
    customSlideIds:s.customSlideIds, customSectionIds:s.customSectionIds, mode:s.mode,
    appliedApprovalByThread:s.appliedApprovalByThread,
    modelProfileName:s.modelProfileName, modelSelectionExplicit:s.modelSelectionExplicit, selectedSkillIds:s.selectedSkillIds,
    threadDrafts:s.threadDrafts, threadReferences:s.threadReferences, nextMarkerByThread:s.nextMarkerByThread,
    editingSelectionIdByThread:s.editingSelectionIdByThread, userTouchedTarget:s.userTouchedTarget, restoredInputs:s.restoredInputs,
    threadResourceMentions:s.threadResourceMentions,
  };
}
export function loadProjectComposer(projectId: string | null) {
  if (draftProject === projectId) return;
  const saved = projectId ? localStorage.getItem(draftKey(projectId)) : null;
  draftProject = null;
  useComposerStore.getState().resetForProject();
  if (saved) { try { useComposerStore.setState(JSON.parse(saved) as Partial<ComposerState>); } catch { /* An invalid local draft is ignored. */ } }
  draftProject = projectId;
  for (const threadId of Object.keys(useComposerStore.getState().threadResourceMentions)) restoreDraftMentions(threadId);
}
// Inline editor chips become explicit removable references after rehydration;
// the persisted draft contains data, never executable editor HTML.
export function restoreDraftMentions(threadId: string) {
  const state = useComposerStore.getState();
  const refs = state.threadResourceMentions[threadId];
  if (!refs?.component_names?.length && !refs?.mentioned_slide_ids?.length) return;
  const previous = state.restoredInputs[threadId];
  useComposerStore.setState({
    restoredInputs: { ...state.restoredInputs, [threadId]: { ...previous,
      component_names: [...new Set([...(previous?.component_names ?? []), ...(refs.component_names ?? [])])],
      mentioned_slide_ids: [...new Set([...(previous?.mentioned_slide_ids ?? []), ...(refs.mentioned_slide_ids ?? [])])],
    } },
    threadResourceMentions: { ...state.threadResourceMentions, [threadId]: {} },
  });
}
export function saveProjectComposer(projectId: string) {
  localStorage.setItem(draftKey(projectId), JSON.stringify(composerScene()));
}
useComposerStore.subscribe(() => { if (draftProject) saveProjectComposer(draftProject); });
