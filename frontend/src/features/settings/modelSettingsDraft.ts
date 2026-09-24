import type { ModelConfig, ModelProvider, ModelProtocol, ModelSettings, SettingsEdit } from '../../api/settings';

export type DraftModel = ModelConfig & { id: string; previous_name?: string; originalProvider: string; originalProtocol: ModelProtocol; originalBaseURL: string; key: string };
export type Draft = Omit<ModelSettings, 'llm'> & { llm: DraftModel[] };
export type Editable = Pick<DraftModel, 'name' | 'provider' | 'protocol' | 'base_url' | 'model' | 'key'>;

export const normalizeBaseURL = (url: string) => url.trim().replace(/\/+$/, '');
export function canKeepKey(row: DraftModel, edit: Editable = row): boolean {
  return row.has_key && edit.provider === row.originalProvider && edit.protocol === row.originalProtocol && normalizeBaseURL(edit.base_url) === row.originalBaseURL;
}

export function changeProvider(edit: Editable, provider: ModelProvider): Editable {
  return { ...edit, provider: provider.id, protocol: provider.default_protocol, base_url: provider.base_urls[provider.default_protocol] ?? '', key: '' };
}

export function changeProtocol(edit: Editable, protocol: ModelProtocol, provider?: ModelProvider): Editable {
  // Preserve a hand-entered gateway. Only replace an untouched official preset.
  const preset = provider?.base_urls[edit.protocol];
  const base_url = !edit.base_url.trim() || preset && normalizeBaseURL(edit.base_url) === normalizeBaseURL(preset)
    ? provider?.base_urls[protocol] ?? '' : edit.base_url;
  return { ...edit, protocol, base_url, key: '' };
}

export function makeDraft(value: ModelSettings, submitted: DraftModel[] = []): Draft {
  return { ...value, llm: value.llm.map((row) => ({
    ...row,
    id: submitted.find((candidate) => candidate.name === row.name)?.id ?? crypto.randomUUID(),
    previous_name: row.name,
    originalProvider: row.provider,
    originalProtocol: row.protocol,
    originalBaseURL: normalizeBaseURL(row.base_url),
    key: '',
  })) };
}

export function modelChanged(row: DraftModel, edit?: Editable): boolean {
  return !row.previous_name || !!edit && (['name', 'provider', 'protocol', 'base_url', 'model', 'key'] as const).some((field) => edit[field] !== row[field]);
}

export function validation(row: DraftModel, rows: DraftModel[]): string {
  if (!row.name.trim()) return '请填写配置名称。';
  if ([...row.name.trim()].length > 80) return '名称最多为 80 个字符。';
  if (rows.some((other) => other.id !== row.id && other.name.trim() === row.name.trim())) return '配置名称已被使用。';
  if (!row.provider || !row.model.trim()) return '请选择供应商并填写模型标识。';
  if (!['responses', 'anthropic'].includes(row.protocol)) return '请选择 Responses 或 Anthropic 协议。';
  try {
    const url = new URL(row.base_url.trim());
    if (!['https:', 'http:'].includes(url.protocol) || !url.hostname || url.username || url.password || /[?#\s\\]/u.test(row.base_url.trim())) throw new Error();
    if (/\/(responses|messages|chat\/completions)\/?$/.test(url.pathname)) return '请填写 API 基础地址，不要包含 /responses 或 /messages 等接口路径。';
  } catch { return '请填写有效的 HTTP(S) API 地址，不包含凭证或查询参数。'; }
  if (/\p{Cc}/u.test(row.name + row.model + row.key)) return '请使用单行文本，不要包含控制字符。';
  if (!row.key.trim() && !canKeepKey(row)) return '请填写 API key；更换供应商、协议或地址需要重新填写。';
  return '';
}

export function savedSnapshot(draft: Draft): Draft {
  return { ...draft, llm: draft.llm.filter((row) => row.previous_name) };
}

export function prepareModelSave(draft: Draft, row: DraftModel, edit: Editable): Draft {
  const updated = { ...row, ...edit, name: edit.name.trim(), base_url: normalizeBaseURL(edit.base_url), model: edit.model.trim(), key: edit.key.trim() };
  const replace = (name: string | null) => row.previous_name && name === row.previous_name ? updated.name : name;
  return {
    ...draft,
    llm: draft.llm.filter((candidate) => candidate.previous_name || candidate.id === row.id)
      .map((candidate) => candidate.id === row.id ? updated : candidate),
    main_road: { default: replace(draft.main_road.default)!, fallback: replace(draft.main_road.fallback) },
    side_road: { ...draft.side_road, ...Object.fromEntries(Object.entries(draft.side_road).map(([key, value]) => [key, replace(value)])) },
  };
}

export function settingsPayload(draft: Draft): SettingsEdit {
  return {
    revision: draft.revision,
    main_road: draft.main_road,
    side_road: draft.side_road,
    llm: draft.llm.map((row) => ({
      name: row.name, provider: row.provider, protocol: row.protocol, base_url: row.base_url, model: row.model,
      ...(row.previous_name ? { previous_name: row.previous_name } : {}),
      ...(row.key.trim() ? { key: row.key.trim() } : {}),
    })),
  };
}

export function applySavedSettings(result: ModelSettings, submitted: Draft, current: Draft): Draft {
  const next = makeDraft(result, submitted.llm);
  const submittedIds = new Set(submitted.llm.map((row) => row.id));
  // Preserve unfinished new cards; existing cards retain stable IDs so their
  // separate edit buffers survive saves elsewhere on the page.
  return { ...next, llm: [...next.llm, ...current.llm.filter((row) => !row.previous_name && !submittedIds.has(row.id))] };
}
