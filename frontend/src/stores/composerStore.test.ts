import { beforeEach, describe, expect, it } from 'vitest';
import { useComposerStore } from './composerStore';

describe('composerStore', () => {
  beforeEach(() => {
    useComposerStore.setState({
      scopeObject: 'presentation',
      scopeSelection: 'current_page',
      lastNonGlobalSelection: 'current_page',
      customSlideIds: [],
      customSectionIds: [],
      selectedSkillIds: [],
      threadDrafts: {},
      threadReferences: {},
      nextMarkerByThread: {},
      editingSelectionIdByThread: {},
    });
  });

  it('selects at most three Skills and allows selected Skills to be removed', () => {
    const store = useComposerStore.getState();
    for (const id of ['one', 'two', 'three', 'four']) store.toggleSkill(id);
    expect(useComposerStore.getState().selectedSkillIds).toEqual(['one', 'two', 'three']);

    useComposerStore.getState().toggleSkill('two');
    expect(useComposerStore.getState().selectedSkillIds).toEqual(['one', 'three']);
  });

  it('removes selections that are no longer returned by the backend', () => {
    useComposerStore.setState({ selectedSkillIds: ['one', 'missing', 'two'] });
    useComposerStore.getState().reconcileSkills(['one', 'two']);
    expect(useComposerStore.getState().selectedSkillIds).toEqual(['one', 'two']);
  });

  it('keeps a briefing draft isolated to its new thread until it is sent', () => {
    const text = '# Handoff\n\n继续完成当前项目';
    useComposerStore.getState().setThreadDraft('thread-next', text);

    expect(useComposerStore.getState().threadDrafts).toEqual({ 'thread-next': text });

    useComposerStore.getState().setThreadDraft('thread-next', `${text}\n\n补充一项验收条件。`);
    expect(useComposerStore.getState().threadDrafts['thread-next']).toContain('验收条件');

    useComposerStore.getState().clearThreadDraft('thread-next');
    expect(useComposerStore.getState().threadDrafts).toEqual({});
  });

  it('restores the previous page selection after leaving global resources', () => {
    useComposerStore.getState().setScopeSelection('custom_sections');
    useComposerStore.getState().toggleCustomSection('sec_one');
    useComposerStore.getState().setScopeObject('global');
    expect(useComposerStore.getState().scopeSelection).toBe('all_pages');
    useComposerStore.getState().setScopeObject('global');

    useComposerStore.getState().setScopeObject('presentation');
    expect(useComposerStore.getState().scopeSelection).toBe('custom_sections');
    expect(useComposerStore.getState().customSectionIds).toEqual(['sec_one']);
  });

  it('removes custom page and section IDs that no longer exist', () => {
    useComposerStore.setState({ customSlideIds: ['sli_one', 'sli_gone'], customSectionIds: ['sec_one', 'sec_gone'] });
    useComposerStore.getState().reconcileScopeIds(['sli_one'], ['sec_one']);
    expect(useComposerStore.getState().customSlideIds).toEqual(['sli_one']);
    expect(useComposerStore.getState().customSectionIds).toEqual(['sec_one']);
  });

  it('keeps mixed references ordered and DOM marker numbers monotonic', () => {
    const store = useComposerStore.getState();
    store.addThreadAttachment('thread-one', { attachmentId: 'att_one', name: 'one.png', size: 10, mediaType: 'image/png' });
    const base = { kind: 'element' as const, comment: '', slide_id: 'sli_one', html_revision: 1, html_hash: 'sha256:a', canvas: { width: 1920 as const, height: 1080 as const }, rect: { x: 1, y: 1, width: 10, height: 10 }, status: 'active' as const, dom_targets: [], chrome_targets: [] };
    store.addThreadDOMSelection('thread-one', { ...base, selection_id: 'sel_one', marker_no: 1, dedupe_key: 'one' });
    store.addThreadDOMSelection('thread-one', { ...base, selection_id: 'sel_two', marker_no: 2, dedupe_key: 'two' });
    useComposerStore.getState().removeThreadDOMSelection('thread-one', 'sel_one');
    expect(useComposerStore.getState().threadReferences['thread-one'].map((item) => item.kind)).toEqual(['image', 'dom']);
    expect(useComposerStore.getState().nextMarkerByThread['thread-one']).toBe(3);
  });

  it('deduplicates DOM selections without consuming another marker', () => {
    const selection = { selection_id: 'sel_one', marker_no: 1, kind: 'element' as const, comment: '', slide_id: 'sli_one', html_revision: 1, html_hash: 'sha256:a', canvas: { width: 1920 as const, height: 1080 as const }, rect: { x: 1, y: 1, width: 10, height: 10 }, status: 'active' as const, dom_targets: [], chrome_targets: [], dedupe_key: 'same' };
    useComposerStore.getState().addThreadDOMSelection('thread-one', selection);
    useComposerStore.getState().addThreadDOMSelection('thread-one', { ...selection, selection_id: 'sel_duplicate', marker_no: 2 });
    expect(useComposerStore.getState().threadReferences['thread-one']).toHaveLength(1);
    expect(useComposerStore.getState().nextMarkerByThread['thread-one']).toBe(2);
  });
});
