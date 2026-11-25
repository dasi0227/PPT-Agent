import type { ModelConfig, ModelSettings, Provider } from '../../api/settings';

export type Editable = Omit<ModelConfig, 'id' | 'provider'> & { provider: Provider | ''; key: string };
export type DraftModel = Editable & { id: string; saved: boolean };
export type Draft = { models: DraftModel[]; routes: ModelSettings['routes'] };

export function savedModel(model: ModelConfig): DraftModel {
  return { ...model, saved: true, key: '' };
}
export function modelChanged(row: DraftModel, edit?: Editable): boolean {
  return !row.saved || !!edit && (['name', 'protocol', 'provider', 'base_url', 'model', 'key'] as const).some(field => edit[field] !== row[field]);
}
export function validation(row: DraftModel, rows: DraftModel[]): string {
  if (!row.name.trim()) return '请填写配置名称。';
  if (rows.some(other => other.saved && other.id !== row.id && other.name === row.name)) return '配置名称已被使用。';
  if (!row.provider) return '请选择厂商。';
  if (!row.model.trim()) return '请填写模型标识。';
  if (!row.base_url.trim()) return '请填写 API 基础地址。';
  if (!row.saved && !row.key.trim()) return '请填写 API key。';
  return '';
}
