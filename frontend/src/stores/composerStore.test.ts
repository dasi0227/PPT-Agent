import { beforeEach, describe, expect, it } from 'vitest';
import { useComposerStore } from './composerStore';

describe('composerStore Skills', () => {
  beforeEach(() => {
    useComposerStore.setState({ selectedSkillIds: [] });
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
});
