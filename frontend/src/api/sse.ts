import { RESOURCE_EDIT_TOOLS } from './resourceTools';
import { subscribeThreadEvents } from './threadJournal';
import { RUN_ACTIVITIES, SSEEvent, SSEEventName } from './types';

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
  'command.permission_requested', 'command.permission_answered',
  'scope.expansion_requested', 'scope.expansion_answered', 'scope.updated',
  'message.reasoning', 'message.milestone', 'message.final',
  'tool.started', 'tool.completed', 'tool.content_prechecked', 'question.asked', 'question.answered',
  'resource.edit_approval_requested', 'resource.edit_approval_updated', 'resource.edit_approval_answered',
  'context.window.updated',
  'context.compacted',
];

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function hasString(data: Record<string, unknown>, key: string): boolean {
  return typeof data[key] === 'string' && data[key] !== '';
}

function isPositiveInteger(value: unknown): boolean {
  return typeof value === 'number' && Number.isInteger(value) && value > 0;
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

export class PublicEventValidationError extends Error {
  constructor(readonly eventName: string, readonly sequence: string | undefined, readonly field: string) {
    super(`Invalid public event ${eventName} at ${sequence ?? 'unknown'}: ${field}`);
    this.name = 'PublicEventValidationError';
  }
}

// Unknown journal records are internal events; malformed public records require recovery.
export function readPublicEvent(eventName: string, data: unknown, id?: string, expectedRunId?: string): SSEEvent | null {
  if (!SSE_EVENT_NAMES.includes(eventName as SSEEventName)) return null;
  const event = parsePublicEvent(eventName, data, id);
  if (event && (!expectedRunId || event.data.run_id === expectedRunId)) return event;
  let field = '$payload';
  if (!isRecord(data)) field = '$';
  else if (data.schema_version !== 6) field = 'schema_version';
  else if (!hasString(data, 'run_id') || (expectedRunId && data.run_id !== expectedRunId)) field = 'run_id';
  else if (!validBase(data)) field = 'occurred_at';
  else field = forbiddenFieldPath(data) ?? publicPayloadIssue(eventName as SSEEventName, data) ?? field;
  const error = new PublicEventValidationError(eventName, id, field);
  console.error('Public event validation failed', { event: eventName, sequence: id, field });
  throw error;
}

const runActivities = new Set<string>(RUN_ACTIVITIES);
const businessTools = new Set(['review_task', 'read_resource', 'read_image', ...RESOURCE_EDIT_TOOLS, 'render_slide', 'run_command', 'git_commit', 'load_component', 'load_skill']);
const planStatuses = new Set(['pending', 'processing', 'completed', 'failed']);

function validBase(data: Record<string, unknown>): boolean {
  return data.schema_version === 6
    && hasString(data, 'run_id')
    && hasString(data, 'occurred_at')
    && String(data.occurred_at).endsWith('Z')
    && !Number.isNaN(Date.parse(String(data.occurred_at)));
}

function validPayload(eventName: SSEEventName, data: Record<string, unknown>): boolean {
  return publicPayloadIssue(eventName, data) === null;
}

function firstFieldIssue(checks: [string, boolean][]): string | null {
  return checks.find(([, valid]) => !valid)?.[0] ?? null;
}

function publicPayloadIssue(eventName: SSEEventName, data: Record<string, unknown>): string | null {
  switch (eventName) {
    case 'run.started':
      return firstFieldIssue([
        ['scope', validRunScope(data.scope)],
        ['mode', ['chat', 'grill', 'plan', 'execute'].includes(String(data.mode))],
        ['user_input', hasString(data, 'user_input')],
        ['skills', validSkills(data.skills)],
      ]);
    case 'run.progress':
      return firstFieldIssue([
        ['activity', runActivities.has(String(data.activity))],
        ...['stage', 'text', 'target', 'progress'].map((field): [string, boolean] => [field, !(field in data)]),
      ]);
    case 'run.resumed':
      return null;
    case 'run.completed':
    case 'run.failed':
    case 'run.error':
    case 'run.canceled':
      return firstFieldIssue([
        ['duration_ms', isNonNegativeInteger(data.duration_ms)],
        ['affected_targets', Array.isArray(data.affected_targets) && validTargets(data.affected_targets)],
        ['error', (data.error === null || validOptionalError(data.error))
          && (!['run.failed', 'run.error'].includes(eventName) || isRecord(data.error))],
        ['trace_id', hasString(data, 'trace_id')],
        ['reason', data.reason === undefined || ['user_requested', 'superseded'].includes(String(data.reason))],
      ]);
    case 'plan.updated':
      return validPlan(data.plan) ? null : 'plan';
    case 'plan.approval_requested':
      return firstFieldIssue([['interaction_id', hasString(data, 'interaction_id')], ['plan', validPlan(data.plan)]]);
    case 'plan.approval_answered':
      return firstFieldIssue([
        ['interaction_id', hasString(data, 'interaction_id')], ['plan_id', hasString(data, 'plan_id')],
        ['decision', ['approve', 'revise', 'refuse'].includes(String(data.decision))],
        ['feedback', data.feedback === undefined || typeof data.feedback === 'string'],
      ]);
    case 'command.permission_requested':
      return firstFieldIssue(['interaction_id', 'call_id', 'command', 'command_hash', 'reason_code', 'reason']
        .map((field): [string, boolean] => [field, hasString(data, field)]));
    case 'command.permission_answered':
      return firstFieldIssue([
        ['interaction_id', hasString(data, 'interaction_id')], ['call_id', hasString(data, 'call_id')],
        ['decision', ['allow_once', 'deny'].includes(String(data.decision))],
        ['command_hash', hasString(data, 'command_hash')],
      ]);
    case 'scope.expansion_requested':
      return firstFieldIssue([
        ['interaction_id', hasString(data, 'interaction_id')], ['call_id', hasString(data, 'call_id')],
        ['base_revision', isPositiveInteger(data.base_revision)], ['current_scope', validRunScope(data.current_scope)],
        ['requested_addition', validScopeAddition(data.requested_addition)], ['proposed_scope', validRunScope(data.proposed_scope)],
        ['affected_page_count', isNonNegativeInteger(data.affected_page_count)], ['reason', hasString(data, 'reason')],
      ]);
    case 'scope.expansion_answered':
      return firstFieldIssue([
        ['interaction_id', hasString(data, 'interaction_id')], ['call_id', hasString(data, 'call_id')],
        ['base_revision', isPositiveInteger(data.base_revision)],
        ['decision', ['approve', 'refuse', 'revise'].includes(String(data.decision))],
        ['applied_scope', data.applied_scope === undefined || validRunScope(data.applied_scope)],
      ]);
    case 'scope.updated':
      return firstFieldIssue([
        ['previous_scope', validRunScope(data.previous_scope)], ['scope', validRunScope(data.scope)], ['cause', hasString(data, 'cause')],
      ]);
    case 'run.mode_changed':
      return firstFieldIssue(['previous_mode', 'mode'].map((field): [string, boolean] =>
        [field, ['chat', 'grill', 'plan', 'execute'].includes(String(data[field]))]));
    case 'message.reasoning':
      return firstFieldIssue([['message_id', hasString(data, 'message_id')], ['text', hasString(data, 'text')]]);
    case 'message.final':
      return firstFieldIssue([
        ['message_id', hasString(data, 'message_id')], ['text', hasString(data, 'text')],
        ['affected_targets', Array.isArray(data.affected_targets) && validTargets(data.affected_targets)],
        ['suggested_next_inputs', validSuggestedNextInputs(data.suggested_next_inputs)],
      ]);
    case 'message.milestone':
      return firstFieldIssue([
        ['message_id', hasString(data, 'message_id')], ['text', hasString(data, 'text')],
        ['completed_step_ids', validUniqueStringArray(data.completed_step_ids, false)],
      ]);
    case 'tool.started':
    case 'tool.completed':
      return toolPayloadIssue(eventName, data);
    case 'tool.content_prechecked':
      return firstFieldIssue([['call_id', hasString(data, 'call_id')], ['content_precheck', validContentPrechecks(data.content_precheck)]]);
    case 'resource.edit_approval_requested':
    case 'resource.edit_approval_updated':
      return firstFieldIssue([
        ['interaction_id', hasString(data, 'interaction_id')], ['call_id', hasString(data, 'call_id')],
        ['resource', ['manifest', 'design', 'outline'].includes(String(data.resource))],
        ['revision', isPositiveInteger(data.revision)], ['target', validOptionalPublicTarget(data.target) && data.target !== undefined],
      ]);
    case 'resource.edit_approval_answered':
      return firstFieldIssue([
        ['interaction_id', hasString(data, 'interaction_id')], ['call_id', hasString(data, 'call_id')],
        ['resource', ['manifest', 'design', 'outline'].includes(String(data.resource))],
        ['revision', isPositiveInteger(data.revision)], ['decision', ['approve', 'reject'].includes(String(data.decision))],
      ]);
    case 'question.asked':
      return firstFieldIssue([
        ['question_id', hasString(data, 'question_id')],
        ...['prompt', 'selection', 'options', 'allow_custom'].map((field): [string, boolean] => [field, !(field in data)]),
        ['header', data.header === undefined || typeof data.header === 'string'],
        ['questions', validQuestionFields(data.questions)],
      ]);
    case 'question.answered':
      return firstFieldIssue([
        ['question_id', hasString(data, 'question_id')], ['answer', validAnswer(data.answer)], ['display_text', hasString(data, 'display_text')],
      ]);
    case 'context.window.updated':
      return firstFieldIssue([
        ['total', isNonNegativeInteger(data.total)], ['max', isPositiveInteger(data.max)],
        ['ratio', typeof data.ratio === 'number' && data.ratio >= 0], ['compactable_tokens', isNonNegativeInteger(data.compactable_tokens)],
        ['compact_threshold_tokens', isPositiveInteger(data.compact_threshold_tokens)], ['status', ['idle', 'compacting'].includes(String(data.status))],
        ['compaction', data.compaction === undefined || (isRecord(data.compaction) && hasString(data.compaction, 'id')
          && isNonNegativeInteger(data.compaction.phase) && Number(data.compaction.phase) <= 2)],
        ['buckets', validContextBuckets(data.buckets)], ['details', validContextDetails(data.details)],
        ['total', validContextWindowTotals(data)],
      ]);
    case 'context.compacted':
      return validContextCompaction(data.compaction) ? null : 'compaction';
  }
}

function toolPayloadIssue(eventName: string, data: Record<string, unknown>): string | null {
  if (eventName !== 'tool.started' && eventName !== 'tool.completed') return null;
  const completed = eventName === 'tool.completed';
  const checks: [string, boolean][] = [
    ['call_id', hasString(data, 'call_id')],
    ['tool', businessTools.has(String(data.tool))],
    ['target', validOptionalPublicTarget(data.target)],
    ['display.label', isRecord(data.display) && hasString(data.display, 'label')],
    ['display.detail', isRecord(data.display) && validOptionalString(data.display.detail)],
    ['command', validCommandProjection(data.command, completed, data.tool === 'run_command')],
  ];
  if (completed) checks.push(
    ['status', ['completed', 'blocked', 'failed'].includes(String(data.status))],
    ['changes', validOperationChanges(data)],
    ['error', validOptionalError(data.error) && (data.status !== 'failed' || isRecord(data.error))],
    ['preview', validPreview(data.preview)],
    ['image', validReadImage(data.image, data.tool, data.status)],
    ['review', validReview(data.review, data.tool, data.status)],
    ['resources', validLoadedResources(data.resources)],
    ['content_precheck', data.content_precheck === undefined || validContentPrechecks(data.content_precheck)],
  );
  else checks.push(['plan_step_id', data.plan_step_id === undefined || typeof data.plan_step_id === 'string']);
  return firstFieldIssue(checks);
}

function validSuggestedNextInputs(value: unknown): boolean {
  return Array.isArray(value)
    && value.length <= 3
    && value.every((item) => typeof item === 'string'
      && item.length > 0
      && Array.from(item).length <= 80
      && !/[\r\n\t]/u.test(item))
    && new Set(value.map((item) => String(item).toLocaleLowerCase())).size === value.length;
}

function validContextBuckets(value: unknown): boolean {
  if (!isRecord(value)) return false;
  const keys = ['system_prompt', 'runtime', 'chat_history', 'read_file', 'other'];
  return Object.keys(value).length === keys.length
    && keys
    .every((key) => isNonNegativeInteger(value[key]));
}

function validContextDetails(value: unknown): boolean {
  if (!isRecord(value)) return false;
  const groups: Record<string, string[]> = {
    system_prompt: ['system prompts', 'tool definitions'],
    runtime: ['runtime context', 'runtime messages'],
    chat_history: ['user messages', 'assistant messages', 'tools execution'],
    read_file: ['read_resource', 'read_image'],
    other: ['other'],
  };
  const keys = Object.keys(groups);
  return Object.keys(value).length === keys.length
    && keys.every((key) => Object.prototype.hasOwnProperty.call(value, key))
    && Object.entries(groups).every(([key, names]) => {
      const details = value[key];
      return Array.isArray(details)
        && details.length === names.length
        && details.every((detail, index) => isRecord(detail)
          && Object.keys(detail).length === 2
          && detail.name === names[index]
          && isNonNegativeInteger(detail.tokens));
    });
}

function validContextWindowTotals(data: Record<string, unknown>): boolean {
  if (!isRecord(data.buckets) || !isRecord(data.details)) return false;
  let bucketTotal = 0;
  for (const [key, tokens] of Object.entries(data.buckets)) {
    if (!isNonNegativeInteger(tokens)) return false;
    const bucketTokens = Number(tokens);
    const details = data.details[key];
    if (!Array.isArray(details)) return false;
    const detailTotal = details.reduce((sum, detail) => (
      isRecord(detail) && isNonNegativeInteger(detail.tokens) ? sum + detail.tokens : sum
    ), 0);
    if (detailTotal !== bucketTokens) return false;
    bucketTotal += bucketTokens;
  }
  return bucketTotal === data.total;
}

function validContextCompaction(value: unknown): boolean {
  return isRecord(value)
    && hasString(value, 'id')
    && ['auto', 'manual'].includes(String(value.trigger))
    && validCompactionTitle(value.title)
    && hasString(value, 'content')
    && isNonNegativeInteger(value.before_tokens)
    && isNonNegativeInteger(value.after_tokens)
    && typeof value.max_tokens === 'number' && Number.isInteger(value.max_tokens) && value.max_tokens > 0
    && isNonNegativeInteger(value.reclaimed_tokens)
    && isNonNegativeInteger(value.duration_ms)
    && isNonNegativeInteger(value.created_at);
}

function validCompactionTitle(value: unknown): boolean {
  return typeof value === 'string'
    && value.trim() !== ''
    && Array.from(value).length <= 48
    && !/[\r\n\t\p{Cc}]/u.test(value)
    && !/<\/?[A-Za-z][^>]*>/u.test(value);
}

function validLoadedResources(value: unknown): boolean {
  if (value === undefined) return true;
  if (!Array.isArray(value)) return false;
  return value.every((entry) => isRecord(entry)
    && ['component', 'skill'].includes(String(entry.kind))
    && hasString(entry, 'id')
    && hasString(entry, 'name')
    && validOptionalString(entry.open_url)
    && !('local_path' in entry)
    && !('html' in entry)
    && !('content' in entry)
    && !('preview' in entry));
}

function validSkills(value: unknown): boolean {
  if (value === undefined) return true;
  if (!Array.isArray(value) || value.length > 3) return false;
  const ids = new Set<string>();
  return value.every((entry) => {
    if (!isRecord(entry) || !hasString(entry, 'id') || !hasString(entry, 'name') ||
      !hasString(entry, 'description') || ids.has(String(entry.id))) return false;
    ids.add(String(entry.id));
    return validOptionalString(entry.local_path) && validOptionalString(entry.open_url);
  });
}

function validRunScope(value: unknown): boolean {
  if (!isRecord(value)) return false;
  if ('object' in value) return false;
  if (!Array.isArray(value.slide_ids) || !value.slide_ids.every((id) => typeof id === 'string' && id.startsWith('sli_'))) return false;
  if (!isRecord(value.source) || !['current_page', 'all_pages', 'custom_pages', 'custom_sections'].includes(String(value.source.kind))) return false;
  return typeof value.include_run_created_slides === 'boolean' && isPositiveInteger(value.revision);
}

function validScopeAddition(value: unknown): boolean {
  if (!isRecord(value)) return false;
  const slidesValid = value.slide_ids === undefined || (Array.isArray(value.slide_ids) && value.slide_ids.every((id) => typeof id === 'string' && id.startsWith('sli_')));
  return slidesValid && !('object' in value);
}

function validOptionalPublicTarget(value: unknown): boolean {
  return value === undefined || validPublicTarget(value);
}

function validPublicTarget(value: unknown): boolean {
  if (!isRecord(value) || !validArtifactDiff(value.diff)) return false;
  if (value.type === 'file') {
    return (value.slide_id === undefined || value.slide_id === '')
      && value.part === 'content'
      && hasString(value, 'display_name')
      && validOptionalNonNegativeInteger(value.insertions)
      && validOptionalNonNegativeInteger(value.deletions)
      && validOptionalString(value.local_path)
      && validOptionalString(value.open_url);
  }
  if (value.type === 'deck') {
    return (value.slide_id === undefined || value.slide_id === '')
      && (value.display_name === undefined || typeof value.display_name === 'string')
      && validOptionalNonNegativeInteger(value.insertions)
      && validOptionalNonNegativeInteger(value.deletions)
      && validOptionalString(value.local_path)
      && validOptionalString(value.open_url)
      && ['manifest', 'outline', 'design'].includes(String(value.part));
  }
  return value.type === 'slide'
    && hasString(value, 'slide_id')
    && (value.display_name === undefined || typeof value.display_name === 'string')
    && validOptionalNonNegativeInteger(value.insertions)
    && validOptionalNonNegativeInteger(value.deletions)
    && validOptionalString(value.local_path)
    && validOptionalString(value.open_url)
    && ['spec', 'html'].includes(String(value.part));
}

function validDiffValue(value: unknown): boolean {
  if (typeof value !== 'string') return false;
  try { JSON.parse(value); return true; } catch { return false; }
}

function validOperationChanges(data: Record<string, unknown>): boolean {
  const editing = RESOURCE_EDIT_TOOLS.some(tool => tool === data.tool);
  if (data.changes === undefined) return !(editing && data.status === 'completed');
  return data.status === 'completed' && (editing || data.tool === 'run_command')
    && Array.isArray(data.changes) && data.changes.every(target =>
      validPublicTarget(target) && isRecord(target) && target.diff !== undefined);
}

function validArtifactDiff(value: unknown): boolean {
  if (value === undefined) return true; // Read and running targets have no frozen diff.
  if (!isRecord(value) || !['added', 'modified', 'deleted'].includes(String(value.status)) || typeof value.filename !== 'string') return false;
  switch (value.kind) {
    case 'outline': return value.filename.length > 0 && Array.isArray(value.groups) && value.groups.every(group =>
      isRecord(group) && Array.isArray(group.rows) && group.rows.every(row => {
        if (!isRecord(row) || !['context', 'added', 'removed'].includes(String(row.kind)) || typeof row.title !== 'string'
          || (row.order !== undefined && typeof row.order !== 'string')) return false;
        if (row.node === 'chapter') return row.depth === 0;
        if (row.node === 'subchapter') return row.depth === 1;
        return ['page', 'purpose'].includes(String(row.node)) && (row.depth === 1 || row.depth === 2);
      }));
    case 'binary': return value.filename.length > 0;
    case 'unavailable': return typeof value.error === 'string' && value.error.length > 0;
    case 'fields': return value.filename.length > 0 && Array.isArray(value.fields) && value.fields.every(field =>
      isRecord(field) && typeof field.field === 'string' && (field.label === undefined || typeof field.label === 'string')
      && Array.isArray(field.rows) && field.rows.every(row => isRecord(row) && ['added', 'removed'].includes(String(row.kind)) && validDiffValue(row.value)));
    case 'text': return value.filename.length > 0 && Array.isArray(value.hunks) && value.hunks.every(hunk =>
      isRecord(hunk) && ['old_start', 'old_count', 'new_start', 'new_count'].every(key => isNonNegativeInteger(hunk[key]))
      && (hunk.context_before === undefined || (Array.isArray(hunk.context_before) && hunk.context_before.every(row =>
        isRecord(row) && row.kind === 'context' && typeof row.text === 'string'
        && isNonNegativeInteger(row.old_line) && Number(row.old_line) > 0
        && isNonNegativeInteger(row.new_line) && Number(row.new_line) > 0)))
      && Array.isArray(hunk.rows) && hunk.rows.every(row => {
        if (!isRecord(row) || typeof row.text !== 'string' || !validOptionalNonNegativeInteger(row.old_line) || !validOptionalNonNegativeInteger(row.new_line)) return false;
        if (row.kind === 'context') return Number(row.old_line) > 0 && Number(row.new_line) > 0;
        if (row.kind === 'removed') return Number(row.old_line) > 0 && row.new_line === undefined;
        return row.kind === 'added' && Number(row.new_line) > 0 && row.old_line === undefined;
      }));
    default: return false;
  }
}

function validOptionalNonNegativeInteger(value: unknown): boolean {
  return value === undefined || (typeof value === 'number' && Number.isInteger(value) && value >= 0);
}

function validOptionalString(value: unknown): boolean {
  return value === undefined || typeof value === 'string';
}

function validTargets(value: unknown): boolean {
  return value === undefined || (Array.isArray(value) && value.every(validPublicTarget));
}

function isNonNegativeInteger(value: unknown): boolean {
  return typeof value === 'number' && Number.isInteger(value) && value >= 0;
}

function validOptionalError(value: unknown): boolean {
  return value === undefined || (
    isRecord(value)
    && hasString(value, 'code')
    && hasString(value, 'message')
    && typeof value.retryable === 'boolean'
  );
}

function validPlan(value: unknown): boolean {
  if (!isRecord(value)
    || !hasString(value, 'plan_id')
    || !hasString(value, 'title') || !hasString(value, 'content')
    || !['awaiting_approval', 'active', 'completed', 'canceled'].includes(String(value.status))
    || !Array.isArray(value.steps)
    || value.steps.length === 0) return false;
  const ids = new Set<string>();
  let active = 0;
  for (const rawStep of value.steps) {
    if (!isRecord(rawStep)
      || !hasString(rawStep, 'id')
      || !hasString(rawStep, 'title')
      || !planStatuses.has(String(rawStep.status))
      || ids.has(String(rawStep.id))) return false;
    ids.add(String(rawStep.id));
    if (rawStep.status === 'processing') active += 1;
  }
  return active <= 1;
}

function validUniqueStringArray(value: unknown, allowEmpty: boolean): boolean {
  if (!Array.isArray(value) || (!allowEmpty && value.length === 0)) return false;
  const strings = value.filter((entry) => typeof entry === 'string' && entry.trim() !== '') as string[];
  return strings.length === value.length && new Set(strings).size === strings.length;
}

function validPreview(value: unknown): boolean {
  if (value === undefined) return true;
  if (!isRecord(value)
    || !hasString(value, 'slide_id')
    || !hasString(value, 'image_url')
    || !Array.isArray(value.warnings)
    || !value.warnings.every((warning) => typeof warning === 'string')) return false;
  return /^\/api\/v1\/runs\/[A-Za-z0-9_-]+\/screenshots\/shot_[A-Za-z0-9_-]+$/.test(String(value.image_url));
}

function validReadImage(value: unknown, tool: unknown, status: unknown): boolean {
  if (value === undefined) return true;
  if (tool !== 'read_image' || status !== 'completed' || !isRecord(value)
    || !hasString(value, 'image_url')) return false;
  const url = String(value.image_url);
  if (value.source === 'render') {
    return typeof value.slide_id === 'string' && value.slide_id !== ''
      && /^\/api\/v1\/runs\/[A-Za-z0-9_-]+\/screenshots\/shot_[A-Za-z0-9_-]+$/.test(url);
  }
  if (value.source === 'attachment') {
    return value.slide_id === undefined
      && /^\/api\/v1\/projects\/[A-Za-z0-9_-]+\/attachments\/att_[A-Za-z0-9_-]+\/content\?variant=(thumbnail|original)$/.test(url);
  }
  return false;
}

function validCommandProjection(value: unknown, terminal: boolean, required: boolean): boolean {
  if (value === undefined) return !required;
  // Command source and captured output are rendered as text, including HTML.
  if (!isRecord(value) || !hasString(value, 'text')) return false;
  if (!terminal) {
    return !['status', 'exit_code', 'duration_ms', 'stdout_preview', 'stderr_preview']
      .some((field) => field in value);
  }
  return ['completed', 'blocked', 'failed'].includes(String(value.status))
    && (value.exit_code === undefined
      || (typeof value.exit_code === 'number' && Number.isInteger(value.exit_code) && value.exit_code >= -1))
    && validOptionalNonNegativeInteger(value.duration_ms)
    && (value.output_truncated === undefined || typeof value.output_truncated === 'boolean')
    && validOptionalString(value.stdout_preview)
    && validOptionalString(value.stderr_preview);
}

function validQuestionOptions(value: unknown, allowCustom: unknown): boolean {
  if (!Array.isArray(value) || value.length > 3 || (value.length === 0 && allowCustom !== true)) return false;
  const ids = new Set<string>();
  return value.every((rawOption) => {
    if (!isRecord(rawOption)
      || !hasString(rawOption, 'id')
      || !hasString(rawOption, 'label')
      || ids.has(String(rawOption.id))
      || !hasString(rawOption, 'description')) return false;
    ids.add(String(rawOption.id));
    return true;
  });
}

function validQuestionFields(value: unknown): boolean {
  if (!Array.isArray(value) || value.length === 0) return false;
  const ids = new Set<string>();
  return value.every((rawQuestion) => {
    if (!isRecord(rawQuestion)
      || !hasString(rawQuestion, 'id')
      || !hasString(rawQuestion, 'question')
      || ids.has(String(rawQuestion.id))
      || !hasString(rawQuestion, 'reason')
      || typeof rawQuestion.allow_custom !== 'boolean'
      || !validQuestionOptions(rawQuestion.options, rawQuestion.allow_custom)) return false;
    ids.add(String(rawQuestion.id));
    return true;
  });
}

function validAnswer(value: unknown): boolean {
  return isRecord(value)
    && !('selected_option_ids' in value)
    && !('custom_text' in value)
    && validQuestionAnswers(value.answers);
}

function validQuestionAnswers(value: unknown): boolean {
  if (!Array.isArray(value) || value.length === 0) return false;
  const ids = new Set<string>();
  return value.every((rawAnswer) => {
    if (!isRecord(rawAnswer)
      || !hasString(rawAnswer, 'question_id')
      || ids.has(String(rawAnswer.question_id))
      || (rawAnswer.selected_option_id !== undefined && typeof rawAnswer.selected_option_id !== 'string')
      || (rawAnswer.custom_text !== undefined && typeof rawAnswer.custom_text !== 'string')) return false;
    ids.add(String(rawAnswer.question_id));
    const selected = typeof rawAnswer.selected_option_id === 'string' && rawAnswer.selected_option_id.trim() !== '';
    const custom = typeof rawAnswer.custom_text === 'string' && rawAnswer.custom_text.trim() !== '';
    return rawAnswer.skipped === true ? !selected && !custom : (rawAnswer.skipped === undefined || rawAnswer.skipped === false) && selected !== custom;
  });
}

const forbiddenPublicFields = new Set([
  'args', 'arguments', 'html', 'observation', 'result', 'path',
  'screenshot_path', 'hash', 'reasoning_content', 'provider_reasoning',
]);

function forbiddenFieldPath(value: unknown, prefix = ''): string | null {
  if (Array.isArray(value)) {
    for (let index = 0; index < value.length; index++) {
      const field = forbiddenFieldPath(value[index], `${prefix}[${index}]`);
      if (field) return field;
    }
  } else if (isRecord(value)) {
    for (const [key, child] of Object.entries(value)) {
      const field = prefix ? `${prefix}.${key}` : key;
      if (forbiddenPublicFields.has(key.toLowerCase())) return field;
      const nested = forbiddenFieldPath(child, field);
      if (nested) return nested;
    }
  }
  return null;
}

function containsForbiddenField(value: unknown): boolean {
  return forbiddenFieldPath(value) !== null;
}

export function subscribeRunEvents(runId: string, options: SSEOptions & { threadId: string }): () => void {
  return subscribeThreadEvents(options.threadId, {
    status: options.onStatus,
    event: (entry, source) => {
      if (entry.run_id !== runId || entry.type === 'user_turn') return;
      if (source === 'replay' && entry.type === 'context.window.updated') return;
      try {
        const event = readPublicEvent(entry.type, entry.data, String(entry.seq), runId);
        if (event) options.onMessage?.(event);
        else options.onUnknown?.(entry.type, entry.data);
      } catch (error) {
        if (!(error instanceof PublicEventValidationError) || source !== 'replay') throw error;
        // A durable malformed record cannot be repaired by reconnecting again.
        // Continue replay so later authoritative lifecycle events still apply.
      }
    },
    error: () => options.onError?.(new Event('stream.error')),
    reset: () => options.onError?.(new Event('history.reset')),
  });
}

function validReview(value: unknown, tool: unknown, status: unknown): boolean {
  const required = tool === 'review_task' && status === 'completed';
  if (value === undefined) return !required;
  return required && isRecord(value) && Object.keys(value).length === 2
    && ['approve', 'revise', 'refuse'].includes(String(value.decision))
    && Array.isArray(value.reasons) && value.reasons.length > 0
    && value.reasons.every(reason => typeof reason === 'string' && reason.trim().length > 0);
}

function validContentPrechecks(value: unknown): boolean {
  return Array.isArray(value) && value.length > 0 && value.every(item => {
    if (!isRecord(item) || !hasString(item, 'assessment_id') || !hasString(item, 'slide_id') || !hasString(item, 'rubric') ||
      !['completed', 'unavailable', 'skipped', 'stale'].includes(String(item.status))) return false;
    if (item.status === 'completed' && (!isRecord(item.scores) || Object.keys(item.scores).length !== 3)) return false;
    return item.scores === undefined || (isRecord(item.scores) && Object.values(item.scores).every(score =>
      isRecord(score) && typeof score.score === 'number' && Number.isFinite(score.score) &&
      typeof score.max_score === 'number' && score.max_score >= 1 && score.max_score <= 9 && score.score >= 0 && score.score <= score.max_score &&
      typeof score.confidence === 'number' && score.confidence >= 0 && score.confidence <= 1 &&
      isRecord(score.legend) && isRecord(score.probabilities)));
  });
}
