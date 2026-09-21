import type { ModelExecution } from '../api/types';
import { showGlobalWarning } from '../stores/toastStore';

export function notifyModelFallback(value: ModelExecution | undefined, purpose: string) {
  if (value?.fallback_used) showGlobalWarning(`${purpose}已切换至备用模型 ${value.profile}`);
}
