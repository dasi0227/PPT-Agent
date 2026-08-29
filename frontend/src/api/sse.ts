import { SSEEvent, SSEEventName } from './types';

export interface SSEOptions {
  onMessage?: (event: SSEEvent) => void;
  onError?: (err: Event) => void;
  onStatus?: (status: 'connecting' | 'open' | 'reconnecting' | 'closed') => void;
  onUnknown?: (eventName: string, data: unknown) => void;
  onClose?: () => void;
  lastEventId?: string;
}

export const SSE_EVENT_NAMES: readonly SSEEventName[] = [
  'run.started', 'run.progress', 'run.resumed', 'run.completed', 'run.failed', 'run.error', 'run.canceled',
  'plan.updated', 'plan.approval_requested', 'plan.approval_answered', 'run.mode_changed',
  'message.reasoning', 'message.milestone', 'message.final',
  'tool.started', 'tool.completed', 'question.asked', 'question.answered',
];

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function hasString(data: Record<string, unknown>, key: string): boolean {
  return typeof data[key] === 'string' && data[key] !== '';
}

export function parseSSEEvent(eventName: string, raw: string, id?: string): SSEEvent | null {
  let data: unknown;
  try {
    data = JSON.parse(raw) as unknown;
  } catch {
    return null;
  }
  return parsePublicEvent(eventName, data, id);
}

export function parsePublicEvent(eventName: string, data: unknown, id?: string): SSEEvent | null {
  if (!SSE_EVENT_NAMES.includes(eventName as SSEEventName)) return null;
  if (!isRecord(data)) return null;
  if (!validBase(data) || containsForbiddenField(data) || !validPayload(eventName as SSEEventName, data)) return null;
  return { id, event: eventName as SSEEventName, data } as unknown as SSEEvent;
}

const progressStages = new Set(['thinking', 'planning', 'reading', 'writing', 'rendering', 'finalizing']);
const businessTools = new Set(['read_ppt', 'mutate_ppt', 'search_refs', 'render_slide']);
const planStatuses = new Set(['pending', 'in_progress', 'completed', 'failed']);
const rawHTMLPattern = /<\s*\/?\s*[a-z][a-z0-9-]*(?:\s+[^>]*)?\/?\s*>/i;

function validBase(data: Record<string, unknown>): boolean {
  return data.schema_version === 3
    && hasString(data, 'run_id')
    && hasString(data, 'occurred_at')
    && String(data.occurred_at).endsWith('Z')
    && !Number.isNaN(Date.parse(String(data.occurred_at)));
}

function validPayload(eventName: SSEEventName, data: Record<string, unknown>): boolean {
  switch (eventName) {
    case 'run.started':
      return validRunScope(data.scope)
        && ['talk', 'ask', 'plan', 'execute'].includes(String(data.mode))
        && hasString(data, 'user_input');
    case 'run.progress':
      return progressStages.has(String(data.stage))
        && hasSafeString(data, 'text')
        && validOptionalPublicTarget(data.target)
        && validProgress(data.progress);
    case 'run.resumed':
      return true;
    case 'run.completed':
    case 'run.failed':
    case 'run.error':
    case 'run.canceled':
      return isNonNegativeInteger(data.duration_ms)
        && Array.isArray(data.affected_targets)
        && validTargets(data.affected_targets)
        && (data.error === null || validOptionalError(data.error))
        && hasString(data, 'trace_id')
        && (data.reason === undefined || ['user_requested', 'superseded'].includes(String(data.reason)))
        && (!['run.failed', 'run.error'].includes(eventName) || isRecord(data.error));
    case 'plan.updated':
      return validPlan(data.plan);
    case 'plan.approval_requested':
      return hasString(data, 'interaction_id') && validPlan(data.plan);
    case 'plan.approval_answered':
      return hasString(data, 'interaction_id') && hasString(data, 'plan_id') && isNonNegativeInteger(data.revision)
        && ['approve', 'revise', 'cancel'].includes(String(data.decision))
        && (data.decision !== 'revise' || hasSafeString(data, 'feedback'));
    case 'run.mode_changed':
      return ['talk', 'ask', 'plan', 'execute'].includes(String(data.previous_mode))
        && ['talk', 'ask', 'plan', 'execute'].includes(String(data.mode));
    case 'message.reasoning':
    case 'message.final':
      return hasString(data, 'message_id')
        && hasSafeString(data, 'text')
        && (eventName !== 'message.final' || validTargets(data.affected_targets));
    case 'message.milestone':
      return hasString(data, 'message_id')
        && hasSafeString(data, 'text')
        && validUniqueStringArray(data.completed_step_ids, false);
    case 'tool.started':
      return hasString(data, 'call_id')
        && businessTools.has(String(data.tool))
        && (data.plan_step_id === undefined || typeof data.plan_step_id === 'string')
        && validOptionalPublicTarget(data.target)
        && validDisplay(data.display);
    case 'tool.completed':
      return hasString(data, 'call_id')
        && businessTools.has(String(data.tool))
        && ['completed', 'failed'].includes(String(data.status))
        && validOptionalPublicTarget(data.target)
        && validDisplay(data.display)
        && validOptionalError(data.error)
        && (data.status !== 'failed' || isRecord(data.error))
        && validPreview(data.preview, String(data.run_id));
    case 'question.asked':
      return hasString(data, 'question_id')
        && hasSafeString(data, 'prompt')
        && (data.header === undefined || (typeof data.header === 'string' && !rawHTMLPattern.test(data.header)))
        && ['single', 'multiple'].includes(String(data.selection))
        && typeof data.allow_custom === 'boolean'
        && validQuestionOptions(data.options, data.allow_custom)
        && validOptionalQuestionFields(data.questions);
    case 'question.answered':
      return hasString(data, 'question_id')
        && validAnswer(data.answer)
        && hasSafeString(data, 'display_text');
  }
}

function validDisplay(value: unknown): boolean {
  return isRecord(value)
    && hasSafeString(value, 'label')
    && (value.detail === undefined || (typeof value.detail === 'string' && !rawHTMLPattern.test(value.detail)));
}

function hasSafeString(data: Record<string, unknown>, key: string): boolean {
  return hasString(data, key) && !rawHTMLPattern.test(String(data[key]));
}

function validRunScope(value: unknown): boolean {
  if (!isRecord(value)) return false;
  if (!['spec', 'ppt'].includes(String(value.artifact))) return false;
  if (value.level === 'slide') return hasString(value, 'slide_id') && value.slide_id !== 'current';
  return value.level === 'deck' && (value.slide_id === undefined || value.slide_id === '');
}

function validOptionalPublicTarget(value: unknown): boolean {
  return value === undefined || validPublicTarget(value);
}

function validPublicTarget(value: unknown): boolean {
  if (!isRecord(value)) return false;
  if (value.type === 'deck') {
    return (value.slide_id === undefined || value.slide_id === '')
      && (value.display_name === undefined || (typeof value.display_name === 'string' && !rawHTMLPattern.test(value.display_name)))
      && validOptionalNonNegativeInteger(value.insertions)
      && validOptionalNonNegativeInteger(value.deletions)
      && validOptionalSafeString(value.local_path)
      && validOptionalSafeString(value.open_url)
      && ['manifest', 'outline', 'design'].includes(String(value.part));
  }
  return value.type === 'slide'
    && hasString(value, 'slide_id')
    && (value.display_name === undefined || (typeof value.display_name === 'string' && !rawHTMLPattern.test(value.display_name)))
    && validOptionalNonNegativeInteger(value.insertions)
    && validOptionalNonNegativeInteger(value.deletions)
    && validOptionalSafeString(value.local_path)
    && validOptionalSafeString(value.open_url)
    && ['spec', 'html'].includes(String(value.part));
}

function validOptionalNonNegativeInteger(value: unknown): boolean {
  return value === undefined || (typeof value === 'number' && Number.isInteger(value) && value >= 0);
}

function validOptionalSafeString(value: unknown): boolean {
  return value === undefined || (typeof value === 'string' && !rawHTMLPattern.test(value));
}

function validTargets(value: unknown): boolean {
  return value === undefined || (Array.isArray(value) && value.every(validPublicTarget));
}

function validProgress(value: unknown): boolean {
  return value === undefined || (
    isRecord(value)
    && isNonNegativeInteger(value.current)
    && typeof value.total === 'number'
    && Number.isInteger(value.total)
    && value.total > 0
    && Number(value.current) <= Number(value.total)
    && hasString(value, 'unit')
  );
}

function isNonNegativeInteger(value: unknown): boolean {
  return typeof value === 'number' && Number.isInteger(value) && value >= 0;
}

function validOptionalError(value: unknown): boolean {
  return value === undefined || (
    isRecord(value)
    && hasString(value, 'code')
    && hasSafeString(value, 'message')
    && typeof value.retryable === 'boolean'
  );
}

function validPlan(value: unknown): boolean {
  if (!isRecord(value)
    || !hasString(value, 'plan_id')
    || typeof value.revision !== 'number'
    || !Number.isInteger(value.revision)
    || value.revision < 1
    || !hasSafeString(value, 'title') || !hasSafeString(value, 'content')
    || !['awaiting_approval', 'active', 'completed', 'canceled'].includes(String(value.status))
    || !Array.isArray(value.steps)
    || value.steps.length === 0) return false;
  const ids = new Set<string>();
  let active = 0;
  for (const rawStep of value.steps) {
    if (!isRecord(rawStep)
      || !hasString(rawStep, 'id')
      || !hasSafeString(rawStep, 'title')
      || !planStatuses.has(String(rawStep.status))
      || ids.has(String(rawStep.id))) return false;
    ids.add(String(rawStep.id));
    if (rawStep.status === 'in_progress') active += 1;
  }
  return active <= 1;
}

function validUniqueStringArray(value: unknown, allowEmpty: boolean): boolean {
  if (!Array.isArray(value) || (!allowEmpty && value.length === 0)) return false;
  const strings = value.filter((entry) => typeof entry === 'string' && entry.trim() !== '') as string[];
  return strings.length === value.length && new Set(strings).size === strings.length;
}

function validPreview(value: unknown, runId: string): boolean {
  if (value === undefined) return true;
  if (!isRecord(value)
    || !hasString(value, 'slide_id')
    || !hasString(value, 'image_url')
    || !Array.isArray(value.warnings)
    || !value.warnings.every((warning) => typeof warning === 'string' && !rawHTMLPattern.test(warning))) return false;
  return String(value.image_url).startsWith(`/api/v1/runs/${runId}/`);
}

function validQuestionOptions(value: unknown, allowCustom: unknown): boolean {
  if (!Array.isArray(value) || value.length > 3 || (value.length === 0 && allowCustom !== true)) return false;
  const ids = new Set<string>();
  return value.every((rawOption) => {
    if (!isRecord(rawOption)
      || !hasString(rawOption, 'id')
      || !hasSafeString(rawOption, 'label')
      || ids.has(String(rawOption.id))
      || (rawOption.description !== undefined
        && (typeof rawOption.description !== 'string' || rawHTMLPattern.test(rawOption.description)))) return false;
    ids.add(String(rawOption.id));
    return true;
  });
}

function validOptionalQuestionFields(value: unknown): boolean {
  if (value === undefined) return true;
  if (!Array.isArray(value) || value.length === 0) return false;
  const ids = new Set<string>();
  return value.every((rawQuestion) => {
    if (!isRecord(rawQuestion)
      || !hasString(rawQuestion, 'id')
      || !hasSafeString(rawQuestion, 'title')
      || ids.has(String(rawQuestion.id))
      || (rawQuestion.description !== undefined
        && (typeof rawQuestion.description !== 'string' || rawHTMLPattern.test(rawQuestion.description)))
      || typeof rawQuestion.allow_custom !== 'boolean'
      || !validQuestionOptions(rawQuestion.options, rawQuestion.allow_custom)) return false;
    ids.add(String(rawQuestion.id));
    return true;
  });
}

function validAnswer(value: unknown): boolean {
  return isRecord(value)
    && validUniqueStringArray(value.selected_option_ids, true)
    && typeof value.custom_text === 'string'
    && (value.answers === undefined || validQuestionAnswers(value.answers));
}

function validQuestionAnswers(value: unknown): boolean {
  if (!Array.isArray(value)) return false;
  const ids = new Set<string>();
  return value.every((rawAnswer) => {
    if (!isRecord(rawAnswer)
      || !hasString(rawAnswer, 'question_id')
      || ids.has(String(rawAnswer.question_id))
      || (rawAnswer.selected_option_id !== undefined && typeof rawAnswer.selected_option_id !== 'string')
      || (rawAnswer.custom_text !== undefined && typeof rawAnswer.custom_text !== 'string')) return false;
    ids.add(String(rawAnswer.question_id));
    return rawAnswer.selected_option_id !== undefined || rawAnswer.custom_text !== undefined;
  });
}

function containsForbiddenField(value: unknown): boolean {
  if (Array.isArray(value)) return value.some(containsForbiddenField);
  if (!isRecord(value)) return false;
  const forbidden = new Set([
    'args', 'arguments', 'html', 'observation', 'result', 'path',
    'screenshot_path', 'hash', 'reasoning_content', 'provider_reasoning',
  ]);
  return Object.entries(value).some(([key, child]) =>
    forbidden.has(key.toLowerCase()) || containsForbiddenField(child));
}

export function subscribeRunEvents(runId: string, options: SSEOptions): () => void {
  const url = new URL(`/api/v1/runs/${runId}/events`, window.location.origin);
  // Add Last-Event-ID as a query parameter if standard EventSource is used,
  // assuming the backend fallback logic supports reading it from query params.
  // The backend run_handler.go should parse `last_event_id` query param if header is absent.
  if (options.lastEventId) {
    url.searchParams.set('last_event_id', options.lastEventId);
  }
  
  options.onStatus?.('connecting');
  const source = new EventSource(url.toString());
  let closed = false;

  const handleMessage = (e: MessageEvent) => {
    const event = parseSSEEvent(e.type, String(e.data), e.lastEventId || undefined);
    if (event) {
      options.onMessage?.(event);
    } else {
      options.onUnknown?.(e.type, e.data);
    }
  };

  SSE_EVENT_NAMES.forEach(type => {
    source.addEventListener(type, handleMessage);
  });
  source.onopen = () => options.onStatus?.('open');
  source.onmessage = (event) => options.onUnknown?.(event.type, event.data);

  source.onerror = (err) => {
    if (closed) return;
    options.onStatus?.('reconnecting');
    options.onError?.(err);
  };

  return () => {
    if (closed) return;
    closed = true;
    source.close();
    options.onStatus?.('closed');
    if (options.onClose) options.onClose();
  };
}
