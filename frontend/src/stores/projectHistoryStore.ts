import { create } from 'zustand';
import { projectHistoryApi, type HistoryPreview, type HistoryState } from '../api/projectHistory';
import { invalidateHistoryRequests } from '../api/client';
import { composerScene, loadProjectComposer, restoreDraftMentions, saveProjectComposer, useComposerStore, type ComposerReference, type ComposerState } from './composerStore';
import { projectWorkspaceRoute } from '../features/workspace/routes';
import { useThreadStore } from './threadStore';
import { useDeckStore } from './deckStore';
import { useRunStore } from './runStore';
import { useGitCommitStore } from './gitCommitStore';

interface HistoryUI {
  states: Record<string, HistoryState>;
  stateErrorByProjectId: Record<string, boolean>;
  dialog: { projectId: string; runId?: string; preview: HistoryPreview; operation: string } | null;
  busy: boolean;
  error: string | null;
  preview: (projectId: string, runId?: string) => Promise<void>;
  execute: () => Promise<void>;
}
export const useProjectHistoryStore = create<HistoryUI>((set, get) => ({
  states: {}, stateErrorByProjectId: {}, dialog: null, busy: false, error: null,
  preview: async (projectId, runId) => {
    if (get().busy) return;
    set({ busy: true, error: null });
    try { const preview = await projectHistoryApi.preview(projectId, runId); set({ dialog: { projectId, runId, preview, operation: crypto.randomUUID() } }); }
    catch (error) { set({ error: error instanceof Error ? error.message : '无法读取回退预览' }); }
    finally { set({ busy: false }); }
  },
  execute: async () => {
    const dialog = get().dialog;
    if (!dialog || get().busy) return;
    set({ busy: true, error: null });
    const { projectId, preview, runId, operation } = dialog;
    try {
      const state = await projectHistoryApi.switch(projectId, preview, operation, {
        composer: composerScene(), slide_id: useDeckStore.getState().currentSlideId,
        view: useDeckStore.getState().globalView, preview_mode: useDeckStore.getState().previewMode,
        active_thread_id: useThreadStore.getState().getActiveThreadId(projectId),
      }, runId);
      applyHistoryScene(projectId, state);
      reloadHistory(projectId, state);
    } catch (error) { set({ busy: false, error: error instanceof Error ? error.message : '项目历史切换失败，请重试' }); }
  },
}));
const appliedKey = (id: string) => `ppt-agent-history-scene-${id}`;
const threadKey = (id: string) => `ppt-agent-history-thread-${id}`;
export function applyHistoryScene(projectId: string, state: HistoryState) {
  loadProjectComposer(projectId);
  const scene = state.scene;
  if (!scene || !state.scene_revision || localStorage.getItem(appliedKey(projectId)) === String(state.scene_revision)) return;
  const input = scene.input;
  if (input && scene.thread_id) {
    const threadId = scene.thread_id;
    const images = input.command.attachments ?? [];
    const selections = input.dom_selections ?? [];
    const refs: ComposerReference[] = [];
    for (const ref of input.reference_order ?? []) {
      const image = images.find((value) => value.id === ref.ref_id);
      const selection = selections.find((value) => value.selection_id === ref.ref_id);
      if (ref.kind === 'image' && image) refs.push({ kind: 'image', attachment: { attachmentId: image.id, name: image.original_name, size: image.size_bytes, mediaType: image.media_type } });
      if (ref.kind === 'dom' && selection) refs.push({ kind: 'dom', selection });
    }
    // Images-only requests may omit reference_order.
    for (const image of images) if (!refs.some((r) => r.kind === 'image' && r.attachment.attachmentId === image.id)) refs.push({ kind: 'image', attachment: { attachmentId: image.id, name: image.original_name, size: image.size_bytes, mediaType: image.media_type } });
    for (const selection of selections) if (!refs.some((r) => r.kind === 'dom' && r.selection.selection_id === selection.selection_id)) refs.push({ kind: 'dom', selection });
    const scope = input.scope_input;
    useComposerStore.getState().resetForProject();
    useComposerStore.setState({
      threadDrafts: { [threadId]: input.command.instruction }, threadReferences: { [threadId]: refs },
      nextMarkerByThread: { [threadId]: Math.max(0, ...selections.map((s) => s.marker_no)) + 1 },
      mode: input.command.mode, modelProfileName: input.model, modelSelectionExplicit: !!input.model, selectedSkillIds: input.skill_ids ?? [],
      scopeObject: scope.object, scopeSelection: scope.selection.kind, lastNonGlobalSelection: scope.selection.kind,
      customSlideIds: scope.selection.slide_ids ?? [], customSectionIds: scope.selection.section_ids ?? [], userTouchedTarget: true,
      restoredInputs: { [threadId]: { scope: input.scope_input, options: input.command.options, component_names: input.component_names ?? [], mentioned_slide_ids: input.mentioned_slide_ids ?? [] } },
    });
    localStorage.setItem(threadKey(projectId), threadId);
  } else if (scene.composer) {
    useComposerStore.getState().resetForProject();
    useComposerStore.setState(scene.composer as Partial<ComposerState>);
    for (const threadId of Object.keys(useComposerStore.getState().threadResourceMentions)) restoreDraftMentions(threadId);
    if (scene.active_thread_id) localStorage.setItem(threadKey(projectId), scene.active_thread_id);
  }
  saveProjectComposer(projectId);
  localStorage.setItem(appliedKey(projectId), String(state.scene_revision));
}
export function selectHistoryThread(projectId: string) {
  const thread = localStorage.getItem(threadKey(projectId));
  const store = useThreadStore.getState();
  if (thread && store.displayThreads(projectId).some((t) => t.id === thread)) {
    store.setActiveThread(projectId, thread);
    localStorage.removeItem(threadKey(projectId));
  }
}
export function reloadHistory(projectId: string, state: HistoryState) {
  invalidateHistoryRequests();
  useRunStore.getState().dropSessions(Object.keys(useRunStore.getState().sessions));
  useGitCommitStore.getState().closeAll();
  // A fresh document destroys every iframe, timer, stream and in-flight callback.
  // The server-persisted scene is applied before the new document loads history.
  window.location.replace(historyWorkspaceRoute(projectId, state));
}
export function historyWorkspaceRoute(projectId: string, state: HistoryState) {
  return projectWorkspaceRoute(projectId, {
    slideId: state.scene?.input?.scope_input.selection.current_slide_id ?? state.scene?.slide_id,
    view: state.scene?.view,
    mode: state.scene?.preview_mode,
  });
}
