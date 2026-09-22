import type { ModelConfig, ModelSettings, SettingsEdit } from '../../api/settings';

export type DraftModel = ModelConfig & { id: string; previous_name?: string; originalProvider: string; key: string };
export type Draft = Omit<ModelSettings, 'llm'> & { llm: DraftModel[] };
export type Editable = Pick<DraftModel, 'name' | 'provider' | 'model' | 'key'>;

export function makeDraft(value: ModelSettings, submitted: DraftModel[] = []): Draft {
  return { ...value, llm: value.llm.map((row) => ({
    ...row,
    id: submitted.find((candidate) => candidate.name === row.name)?.id ?? crypto.randomUUID(),
    previous_name: row.name,
    originalProvider: row.provider,
    key: '',
  })) };
}

export function modelChanged(row: DraftModel, edit?: Editable): boolean {
  return !row.previous_name || !!edit && (['name', 'provider', 'model', 'key'] as const).some((field) => edit[field] !== row[field]);
}

export function validation(row: DraftModel, rows: DraftModel[]): string {
  if (!row.name.trim()) return '请填写配置名称。';
  if ([...row.name.trim()].length > 80) return '名称最多为 80 个字符。';
  if (rows.some((other) => other.id !== row.id && other.name.trim() === row.name.trim())) return '配置名称已被使用。';
  if (!row.provider || !row.model.trim()) return '请选择供应商并填写模型标识。';
  if (/\p{Cc}/u.test(row.name + row.model + row.key)) return '请使用单行文本，不要包含控制字符。';
  if (!row.key.trim() && (!row.has_key || row.provider !== row.originalProvider)) return '请填写 API key；更换供应商需要重新填写。';
  return '';
}

export function savedSnapshot(draft: Draft): Draft {
  return { ...draft, llm: draft.llm.filter((row) => row.previous_name) };
}

export function prepareModelSave(draft: Draft, row: DraftModel, edit: Editable): Draft {
  const updated = { ...row, ...edit, name: edit.name.trim(), model: edit.model.trim(), key: edit.key.trim() };
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
      name: row.name, provider: row.provider, model: row.model,
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
