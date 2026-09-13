import { beforeEach, describe, expect, it } from 'vitest';
import type { HistoryState } from '../api/projectHistory';
import { applyHistoryScene } from './projectHistoryStore';
import { composerScene, loadProjectComposer, useComposerStore } from './composerStore';

beforeEach(() => { loadProjectComposer(null); localStorage.clear(); useComposerStore.getState().resetForProject(); });
describe('project checkpoint composer scene', () => {
  it('restores full input and keeps subsequent text edits across refresh', () => {
    const selection = {
      selection_id: 'sel_one', marker_no: 3, kind: 'element' as const, comment: '缩小标题', slide_id: 's1',
      html_revision: 2, html_hash: 'hash', canvas: { width: 1920 as const, height: 1080 as const },
      rect: { x: 10, y: 10, width: 100, height: 100 }, status: 'active' as const,
      dom_targets: [], chrome_targets: [],
    };
    const state: HistoryState = { revision: 9, scene_revision: 9, checkpoints: [], latest: 'latest', scene: { thread_id: 't', input: {
      command: { instruction: 'input', mode: 'grill', options: { language: 'zh-CN' }, attachments: [{ id: 'img', original_name: 'reference.png', media_type: 'image/png', size_bytes: 42 }] },
      scope_input: { object: 'presentation', selection: { kind: 'custom_pages', slide_ids: ['s1', 's2'] } },
      model: 'model', skill_ids: ['skill'], component_names: ['chart'], mentioned_slide_ids: ['s2'], dom_selections: [selection], reference_order: [{ kind: 'dom', ref_id: 'sel_one' }, { kind: 'image', ref_id: 'img' }],
    } } };
    applyHistoryScene('p', state);
    const composer = useComposerStore.getState();
    expect(composer.threadDrafts.t).toBe('input');
    expect(composer.threadReferences.t).toEqual([{ kind: 'dom', selection }, { kind: 'image', attachment: { attachmentId: 'img', name: 'reference.png', size: 42, mediaType: 'image/png' } }]);
    expect(composer.nextMarkerByThread.t).toBe(4);
    expect(composer.customSlideIds).toEqual(['s1', 's2']);
    expect(composer.mode).toBe('grill'); expect(composer.modelProfileName).toBe('model');
    expect(composer.restoredInputs.t).toMatchObject({ component_names: ['chart'], mentioned_slide_ids: ['s2'] });
    composer.setThreadDraft('t', 'edited draft');
    loadProjectComposer(null); loadProjectComposer('p'); applyHistoryScene('p', state);
    expect(useComposerStore.getState().threadDrafts.t).toBe('edited draft');
    expect(useComposerStore.getState().restoredInputs.t.scope).toEqual(state.scene!.input!.scope_input);
    useComposerStore.getState().setScopeSelection('all_pages');
    expect(useComposerStore.getState().restoredInputs.t.scope).toBeUndefined();
  });
  it('restores the original drafts for all threads from the durable server scene', () => {
    loadProjectComposer('p');
    useComposerStore.setState({ threadDrafts: { t1: 'one', t2: 'two' }, mode: 'plan', customSectionIds: ['section'] });
    const original = JSON.parse(JSON.stringify(composerScene())) as Record<string, unknown>;
    useComposerStore.getState().resetForProject();
    localStorage.clear(); // Simulates returning without a local in-memory recovery backup.
    applyHistoryScene('p', { revision: 12, scene_revision: 12, checkpoints: [], scene: { composer: original, active_thread_id: 't2' } });
    expect(useComposerStore.getState().threadDrafts).toEqual({ t1: 'one', t2: 'two' });
    expect(useComposerStore.getState().mode).toBe('plan');
    expect(useComposerStore.getState().customSectionIds).toEqual(['section']);
  });
});
