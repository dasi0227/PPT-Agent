import { describe, expect, it } from 'vitest';
import type { ModelSettings } from '../../api/settings';
import { applySavedSettings, changeProtocol, makeDraft, modelChanged, prepareModelSave, savedSnapshot, settingsPayload, validation, type DraftModel } from './modelSettingsDraft';

const settings: ModelSettings = {
  revision: 'revision-1', providers: [{ id: 'openai', name: 'OpenAI', default_protocol: 'responses', base_urls: { responses: 'https://api.openai.com/v1' } }, { id: 'kimi', name: 'Kimi', default_protocol: 'anthropic', base_urls: { anthropic: 'https://api.moonshot.cn/anthropic/v1' } }], protocols: ['responses', 'anthropic'],
  llm: [
    { name: '主模型', provider: 'openai', protocol: 'responses', base_url: 'https://api.openai.com/v1', model: 'main-model', has_key: true },
    { name: '备用模型', provider: 'kimi', protocol: 'anthropic', base_url: 'https://api.moonshot.cn/anthropic/v1', model: 'backup-model', has_key: true },
  ],
  main_road: { default: '主模型', fallback: '备用模型' },
  side_road: { default: '主模型', fallback: '备用模型', rename: '主模型', compact: null, commit: null, polish: null, handoff: null, kickoff: null },
};
const newCard = (id: string): DraftModel => ({ id, name: '', provider: 'openai', protocol: 'responses', base_url: 'https://api.openai.com/v1', model: '', has_key: false, originalProvider: '', originalProtocol: 'responses', originalBaseURL: '', key: '' });

describe('settings save boundaries', () => {
  it('persists one model and all its renamed references without including unfinished new cards', () => {
    const draft = makeDraft(settings);
    draft.llm.push(newCard('unfinished'));
    const row = draft.llm[0];
    const next = prepareModelSave(draft, row, { ...row, name: ' 新主模型 ', model: 'next-model', key: '' });
    const payload = settingsPayload(next);

    expect(payload.llm).toEqual([
      { previous_name: '主模型', name: '新主模型', provider: 'openai', protocol: 'responses', base_url: 'https://api.openai.com/v1', model: 'next-model' },
      { previous_name: '备用模型', name: '备用模型', provider: 'kimi', protocol: 'anthropic', base_url: 'https://api.moonshot.cn/anthropic/v1', model: 'backup-model' },
    ]);
    expect(payload.main_road).toEqual({ default: '新主模型', fallback: '备用模型' });
    expect(payload.side_road).toMatchObject({ default: '新主模型', rename: '新主模型', compact: null });
    expect(draft.main_road.default).toBe('主模型');
  });

  it('adds only the completed new model while routing saves use only persisted models', () => {
    const draft = makeDraft(settings);
    const added = newCard('added');
    draft.llm.push(added, newCard('unfinished'));
    const next = prepareModelSave(draft, added, { name: '新模型', provider: 'openai', protocol: 'responses', base_url: 'https://api.openai.com/v1', model: 'new-model', key: 'test-key' });

    expect(settingsPayload(next).llm).toHaveLength(3);
    expect(settingsPayload(next).llm[2]).toEqual({ name: '新模型', provider: 'openai', protocol: 'responses', base_url: 'https://api.openai.com/v1', model: 'new-model', key: 'test-key' });
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
      llm: submitted.llm.map(({ name, model, provider, protocol, base_url }) => ({ name, model, provider, protocol, base_url, has_key: true })),
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
    expect(validation({ ...row, provider: 'kimi', protocol: 'anthropic', base_url: 'https://api.moonshot.cn/anthropic/v1', key: 'replacement-key' }, draft.llm)).toBe('');
    expect(modelChanged(row, { ...row })).toBe(false);
    expect(modelChanged(row, { ...row, model: 'changed' })).toBe(true);
    expect(modelChanged(newCard('new'))).toBe(true);
  });

  it('requires a replacement credential for endpoint or protocol changes', () => {
    const draft = makeDraft(settings);
    const row = draft.llm[0];
    expect(validation({ ...row, base_url: 'https://gateway.example/v1' }, draft.llm)).toContain('重新填写');
    expect(validation({ ...row, protocol: 'anthropic' }, draft.llm)).toContain('重新填写');
    expect(validation({ ...row, base_url: row.base_url + '/' }, draft.llm)).toBe('');
    const edit = { ...row, base_url: 'https://gateway.example/prefix/v1/', key: 'replacement' };
    const payload = settingsPayload(prepareModelSave(draft, row, edit));
    expect(payload.llm[0]).toMatchObject({ provider: 'openai', protocol: 'responses', base_url: 'https://gateway.example/prefix/v1', key: 'replacement' });
    expect(validation({ ...row, base_url: 'https://gateway.example/v1/responses' }, draft.llm)).toContain('基础地址');
  });

  it('keeps a custom gateway when switching protocol but updates official presets', () => {
    const row = makeDraft(settings).llm[0];
    const provider = { id: 'deepseek', name: 'DeepSeek', default_protocol: 'anthropic' as const,
      base_urls: { responses: 'https://api.deepseek.com/v1', anthropic: 'https://api.deepseek.com/anthropic/v1' } };
    const official = changeProtocol({ ...row, provider: 'deepseek', base_url: provider.base_urls.responses }, 'anthropic', provider);
    expect(official.base_url).toBe(provider.base_urls.anthropic);
    const custom = changeProtocol({ ...row, base_url: 'https://gateway.example/v1', key: 'draft-secret' }, 'anthropic', provider);
    expect(custom).toMatchObject({ provider: 'openai', protocol: 'anthropic', base_url: 'https://gateway.example/v1', key: '' });
  });
});
