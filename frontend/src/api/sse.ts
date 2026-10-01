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

const runActivities = new Set<string>(RUN_ACTIVITIES);
const businessTools = new Set(['review_task', 'read_resource', 'read_image', ...RESOURCE_EDIT_TOOLS, 'render_slide', 'run_command', 'git_commit', 'load_component', 'load_skill']);
const planStatuses = new Set(['pending', 'processing', 'completed', 'failed']);
const rawHTMLPattern = /<\s*\/?\s*[a-z][a-z0-9-]*(?:\s+[^>]*)?\/?\s*>/i;

function validBase(data: Record<string, unknown>): boolean {
  return data.schema_version === 6
    && hasString(data, 'run_id')
    && hasString(data, 'occurred_at')
    && String(data.occurred_at).endsWith('Z')
    && !Number.isNaN(Date.parse(String(data.occurred_at)));
}

function validPayload(eventName: SSEEventName, data: Record<string, unknown>): boolean {
  switch (eventName) {
    case 'run.started':
      return validRunScope(data.scope)
        && ['chat', 'grill', 'plan', 'execute'].includes(String(data.mode))
        && hasString(data, 'user_input')
        && validSkills(data.skills);
    case 'run.progress':
      return runActivities.has(String(data.activity))
        && !['stage', 'text', 'target', 'progress'].some((field) => field in data);
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
      return hasString(data, 'interaction_id') && hasString(data, 'plan_id')
        && ['approve', 'revise', 'refuse'].includes(String(data.decision))
        && (data.feedback === undefined || (typeof data.feedback === 'string' && !rawHTMLPattern.test(data.feedback)));
    case 'command.permission_requested':
      return hasString(data, 'interaction_id')
        && hasString(data, 'call_id')
        && hasSafeString(data, 'command')
        && hasString(data, 'command_hash')
        && hasString(data, 'reason_code')
        && hasSafeString(data, 'reason');
    case 'command.permission_answered':
      return hasString(data, 'interaction_id')
        && hasString(data, 'call_id')
        && hasString(data, 'command_hash')
        && ['allow_once', 'deny'].includes(String(data.decision));
    case 'scope.expansion_requested':
      return hasString(data, 'interaction_id') && hasString(data, 'call_id')
        && isPositiveInteger(data.base_revision) && validRunScope(data.current_scope)
        && validScopeAddition(data.requested_addition) && validRunScope(data.proposed_scope)
        && isNonNegativeInteger(data.affected_page_count) && hasSafeString(data, 'reason');
    case 'scope.expansion_answered':
      return hasString(data, 'interaction_id') && hasString(data, 'call_id')
        && isPositiveInteger(data.base_revision) && ['approve', 'refuse', 'revise'].includes(String(data.decision))
        && (data.applied_scope === undefined || validRunScope(data.applied_scope));
    case 'scope.updated':
      return validRunScope(data.previous_scope) && validRunScope(data.scope) && hasString(data, 'cause');
    case 'run.mode_changed':
      return ['chat', 'grill', 'plan', 'execute'].includes(String(data.previous_mode))
        && ['chat', 'grill', 'plan', 'execute'].includes(String(data.mode));
    case 'message.reasoning':
      return hasString(data, 'message_id')
        && hasSafeString(data, 'text');
    case 'message.final':
      return hasString(data, 'message_id')
        && hasSafeString(data, 'text')
        && Array.isArray(data.affected_targets)
        && validTargets(data.affected_targets)
        && validSuggestedNextInputs(data.suggested_next_inputs);
    case 'message.milestone':
      return hasString(data, 'message_id')
        && hasSafeString(data, 'text')
        && validUniqueStringArray(data.completed_step_ids, false);
    case 'tool.started':
      return hasString(data, 'call_id')
        && businessTools.has(String(data.tool))
        && (data.plan_step_id === undefined || typeof data.plan_step_id === 'string')
        && validOptionalPublicTarget(data.target)
        && validDisplay(data.display)
        && validCommandProjection(data.command, false, data.tool === 'run_command');
    case 'tool.content_prechecked':
      return hasString(data, 'call_id') && validContentPrechecks(data.content_precheck);
    case 'tool.completed':
      return hasString(data, 'call_id')
        && businessTools.has(String(data.tool))
        && ['completed', 'blocked', 'failed'].includes(String(data.status))
        && validOptionalPublicTarget(data.target)
        && validOperationChanges(data)
        && validDisplay(data.display)
        && validOptionalError(data.error)
        && (data.status !== 'failed' || isRecord(data.error))
        && validPreview(data.preview, String(data.run_id))
        && validReadImage(data.image, data.tool, data.status)
        && validReview(data.review, data.tool, data.status)
        && validLoadedResources(data.resources)
        && (data.content_precheck === undefined || validContentPrechecks(data.content_precheck))
        && validCommandProjection(data.command, true, data.tool === 'run_command');
    case 'question.asked':
      return hasString(data, 'question_id')
        && !['prompt', 'selection', 'options', 'allow_custom'].some((field) => field in data)
        && (data.header === undefined || (typeof data.header === 'string' && !rawHTMLPattern.test(data.header)))
        && validQuestionFields(data.questions);
    case 'question.answered':
      return hasString(data, 'question_id')
        && validAnswer(data.answer)
        && hasSafeString(data, 'display_text');
    case 'context.window.updated':
      return isNonNegativeInteger(data.total)
        && typeof data.max === 'number' && Number.isInteger(data.max) && data.max > 0
        && typeof data.ratio === 'number' && data.ratio >= 0
        && isNonNegativeInteger(data.compactable_tokens)
        && typeof data.compact_threshold_tokens === 'number'
        && Number.isInteger(data.compact_threshold_tokens)
        && data.compact_threshold_tokens > 0
        && ['idle', 'compacting'].includes(String(data.status))
        && (data.compaction === undefined || (isRecord(data.compaction) && hasString(data.compaction, 'id')
          && isNonNegativeInteger(data.compaction.phase) && Number(data.compaction.phase) <= 2))
        && validContextBuckets(data.buckets)
        && validContextDetails(data.details)
        && validContextWindowTotals(data);
    case 'context.compacted':
      return validContextCompaction(data.compaction);
  }
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
  const keys = ['system_prompt', 'runtime', 'chat_history', 'read_file', 'run_command', 'other'];
  return Object.keys(value).length === keys.length
    && keys
    .every((key) => isNonNegativeInteger(value[key]));
}

function validContextDetails(value: unknown): boolean {
  if (!isRecord(value)) return false;
  const groups: Record<string, string[]> = {
    system_prompt: ['system prompts', 'tool definitions'],
    runtime: ['runtime state', 'runtime resources', 'runtime messages'],
    chat_history: ['user messages', 'assistant messages', 'other tools', 'context summary'],
    read_file: ['read_resource', 'read_image', 'read_project'],
    other: ['other'],
  };
  const keys = [...Object.keys(groups), 'run_command'];
  return Object.keys(value).length === keys.length
    && keys.every((key) => Object.prototype.hasOwnProperty.call(value, key))
    && validRunCommandDetails(value.run_command)
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

function validRunCommandDetails(value: unknown): boolean {
  if (!Array.isArray(value) || value.length < 1 || value.length > 4) return false;
  let previousTokens = Number.MAX_SAFE_INTEGER;
  let previousName = '';
  const names = new Set<string>();
  return value.every((detail, index) => {
    if (!isRecord(detail) || Object.keys(detail).length !== 2
      || typeof detail.name !== 'string' || !isNonNegativeInteger(detail.tokens)
      || names.has(detail.name)) return false;
    const tokens = Number(detail.tokens);
    names.add(detail.name);
    if (detail.name === 'run_command') return value.length === 1 && tokens === 0;
    if (detail.name === 'other command') {
      return index === value.length - 1 && tokens > 0;
    }
    if (index >= 3 || !/^[a-z0-9][a-z0-9._+-]{0,63}$/u.test(detail.name)
      || tokens <= 0 || tokens > previousTokens
      || (tokens === previousTokens && previousName !== '' && detail.name < previousName)) return false;
    previousTokens = tokens;
    previousName = detail.name;
    return true;
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
    && !rawHTMLPattern.test(value);
}

function validLoadedResources(value: unknown): boolean {
  if (value === undefined) return true;
  if (!Array.isArray(value)) return false;
  return value.every((entry) => isRecord(entry)
    && ['component', 'skill'].includes(String(entry.kind))
    && hasString(entry, 'id')
    && hasSafeString(entry, 'name')
    && validOptionalSafeString(entry.open_url)
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
    if (!isRecord(entry) || !hasString(entry, 'id') || !hasSafeString(entry, 'name') ||
      !hasSafeString(entry, 'description') || ids.has(String(entry.id))) return false;
    ids.add(String(entry.id));
    return validOptionalSafeString(entry.local_path) && validOptionalSafeString(entry.open_url);
  });
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
      && hasSafeString(value, 'display_name')
      && validOptionalNonNegativeInteger(value.insertions)
      && validOptionalNonNegativeInteger(value.deletions)
      && validOptionalSafeString(value.local_path)
      && validOptionalSafeString(value.open_url);
  }
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

function validOptionalSafeString(value: unknown): boolean {
  return value === undefined || (typeof value === 'string' && !rawHTMLPattern.test(value));
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
    && hasSafeString(value, 'message')
    && typeof value.retryable === 'boolean'
  );
}

function validPlan(value: unknown): boolean {
  if (!isRecord(value)
    || !hasString(value, 'plan_id')
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
    if (rawStep.status === 'processing') active += 1;
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
  if (!isRecord(value) || !hasSafeString(value, 'text')) return false;
  if (!terminal) {
    return !['status', 'exit_code', 'duration_ms', 'stdout_preview', 'stderr_preview']
      .some((field) => field in value);
  }
  return ['completed', 'blocked', 'failed'].includes(String(value.status))
    && (value.exit_code === undefined
      || (typeof value.exit_code === 'number' && Number.isInteger(value.exit_code) && value.exit_code >= -1))
    && validOptionalNonNegativeInteger(value.duration_ms)
    && (value.output_truncated === undefined || typeof value.output_truncated === 'boolean')
    && validOptionalSafeString(value.stdout_preview)
    && validOptionalSafeString(value.stderr_preview);
}

function validQuestionOptions(value: unknown, allowCustom: unknown): boolean {
  if (!Array.isArray(value) || value.length > 3 || (value.length === 0 && allowCustom !== true)) return false;
  const ids = new Set<string>();
  return value.every((rawOption) => {
    if (!isRecord(rawOption)
      || !hasString(rawOption, 'id')
      || !hasSafeString(rawOption, 'label')
      || ids.has(String(rawOption.id))
      || !hasSafeString(rawOption, 'description')) return false;
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
      || !hasSafeString(rawQuestion, 'question')
      || ids.has(String(rawQuestion.id))
      || !hasSafeString(rawQuestion, 'reason')
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

export function subscribeRunEvents(runId: string, options: SSEOptions & { threadId: string }): () => void {
  return subscribeThreadEvents(options.threadId, {
    status: options.onStatus,
    event: (entry) => {
      if (entry.run_id !== runId) return;
      if (entry.type === 'user_turn') return;
      const event = parseSSEEvent(entry.type, JSON.stringify(entry.data), String(entry.seq));
      if (event) options.onMessage?.(event);
    },
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
