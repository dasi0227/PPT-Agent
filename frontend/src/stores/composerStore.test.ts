import { beforeEach, describe, expect, it } from 'vitest';
import { useComposerStore } from './composerStore';

describe('composerStore Skills', () => {
  beforeEach(() => {
    useComposerStore.setState({ selectedSkillIds: [], threadDrafts: {} });
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
});
