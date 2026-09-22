import { describe, expect, it } from 'vitest';
import type { ModelSettings } from '../../api/settings';
import { applySavedSettings, makeDraft, modelChanged, prepareModelSave, savedSnapshot, settingsPayload, validation, type DraftModel } from './modelSettingsDraft';

const settings: ModelSettings = {
  revision: 'revision-1', providers: ['openai', 'kimi'],
  llm: [
    { name: '主模型', provider: 'openai', model: 'main-model', has_key: true },
    { name: '备用模型', provider: 'kimi', model: 'backup-model', has_key: true },
  ],
  main_road: { default: '主模型', fallback: '备用模型' },
  side_road: { default: '主模型', fallback: '备用模型', rename: '主模型', compact: null, commit: null, polish: null, handoff: null, kickoff: null },
};
const newCard = (id: string): DraftModel => ({ id, name: '', provider: 'openai', model: '', has_key: false, originalProvider: '', key: '' });

describe('settings save boundaries', () => {
  it('persists one model and all its renamed references without including unfinished new cards', () => {
    const draft = makeDraft(settings);
    draft.llm.push(newCard('unfinished'));
    const row = draft.llm[0];
    const next = prepareModelSave(draft, row, { ...row, name: ' 新主模型 ', model: 'next-model', key: '' });
    const payload = settingsPayload(next);

    expect(payload.llm).toEqual([
      { previous_name: '主模型', name: '新主模型', provider: 'openai', model: 'next-model' },
      { previous_name: '备用模型', name: '备用模型', provider: 'kimi', model: 'backup-model' },
    ]);
    expect(payload.main_road).toEqual({ default: '新主模型', fallback: '备用模型' });
    expect(payload.side_road).toMatchObject({ default: '新主模型', rename: '新主模型', compact: null });
    expect(draft.main_road.default).toBe('主模型');
  });

  it('adds only the completed new model while routing saves use only persisted models', () => {
    const draft = makeDraft(settings);
    const added = newCard('added');
    draft.llm.push(added, newCard('unfinished'));
    const next = prepareModelSave(draft, added, { name: '新模型', provider: 'openai', model: 'new-model', key: 'test-key' });

    expect(settingsPayload(next).llm).toHaveLength(3);
    expect(settingsPayload(next).llm[2]).toEqual({ name: '新模型', provider: 'openai', model: 'new-model', key: 'test-key' });
    expect(settingsPayload(savedSnapshot(draft)).llm.map((row) => row.name)).toEqual(['主模型', '备用模型']);
  });

  it('preserves other edit identities and unfinished cards when accepting the saved response', () => {
    const draft = makeDraft(settings);
    const row = draft.llm[0];
    const other = draft.llm[1];
    const unfinished = newCard('unfinished');
    draft.llm.push(unfinished);
    const pendingEdit = { ...other, model: 'unsaved-model', key: 'unsaved-key' };
    const edits = { [other.id]: pendingEdit };
    const submitted = prepareModelSave(draft, row, { ...row, name: '新主模型', key: 'new-key' });
    const response: ModelSettings = {
      ...settings, revision: 'revision-2',
      llm: submitted.llm.map(({ name, model, provider }) => ({ name, model, provider, has_key: true })),
      main_road: submitted.main_road, side_road: submitted.side_road,
    };
    const accepted = applySavedSettings(response, submitted, draft);

    expect(accepted.revision).toBe('revision-2');
    expect(accepted.llm[0]).toMatchObject({ id: row.id, name: '新主模型', previous_name: '新主模型', key: '' });
    expect(accepted.llm[1].id).toBe(other.id);
    expect(edits[accepted.llm[1].id]).toEqual(pendingEdit);
    expect(accepted.llm[2]).toEqual(unfinished);
    expect(settingsPayload(savedSnapshot(accepted)).llm[0].previous_name).toBe('新主模型');
  });

  it('requires a new key when changing provider and recognizes reverted edits as clean', () => {
    const draft = makeDraft(settings);
    const row = draft.llm[0];
    expect(validation({ ...row, provider: 'kimi' }, draft.llm)).toContain('更换供应商');
    expect(validation({ ...row, provider: 'kimi', key: 'replacement-key' }, draft.llm)).toBe('');
    expect(modelChanged(row, { ...row })).toBe(false);
    expect(modelChanged(row, { ...row, model: 'changed' })).toBe(true);
    expect(modelChanged(newCard('new'))).toBe(true);
  });
});
