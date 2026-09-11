import type {
  PlanState,
  BriefingKind,
  BriefingVersion,
  PublicLoadedResource,
  RunMode,
  RunScope,
  Skill,
	PublicDOMSelection,
	ReferenceOrderItem,
} from '../../api/types';
import { parsePublicEvent } from '../../api/sse';
import { reducePlan, reduceSSEEvent, type TimelineItem } from './eventReducer';

export interface HistoryEntry {
  seq: number;
  ts: number;
  run_id: string;
  turn: 'user' | 'agent';
  type: string;
  data: Record<string, unknown>;
}

function readDOMSelections(data: Record<string, unknown>): PublicDOMSelection[] {
	if (!Array.isArray(data.dom_selections)) return [];
	return data.dom_selections.flatMap((value) => {
		if (!isRecord(value) || typeof value.selection_id !== 'string' || typeof value.marker_no !== 'number' || typeof value.comment !== 'string' || !['active','content_deleted','page_deleted'].includes(String(value.status))) return [];
		return [{ selection_id:value.selection_id, marker_no:value.marker_no, comment:value.comment, status:value.status as PublicDOMSelection['status'] }];
	});
}

function readReferenceOrder(data: Record<string, unknown>): ReferenceOrderItem[] {
	if (!Array.isArray(data.reference_order)) return [];
	return data.reference_order.flatMap((value) => isRecord(value) && (value.kind === 'image' || value.kind === 'dom') && typeof value.ref_id === 'string' ? [{ kind:value.kind, ref_id:value.ref_id }] : []);
}

export interface HydratedRunView {
  items: TimelineItem[];
  plan: PlanState | null;
  session: HistorySessionState;
  lastEventId?: string;
}

export interface HistorySessionState {
  activeRunId: string | null;
  status: 'idle' | 'running' | 'waiting' | 'done' | 'error' | 'canceled';
  scope?: RunScope;
  mode?: RunMode;
  pendingQuestion: { id: string; prompt: string } | null;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function readHistoryScope(data: Record<string, unknown>): RunScope | undefined {
  const raw = data.scope;
  if (!isRecord(raw)) return undefined;
  const object = raw.object;
  const source = raw.source;
  if (!['spec', 'html', 'presentation', 'global'].includes(String(object)) ||
    !isRecord(source) || !['current_page', 'all_pages', 'custom_pages', 'custom_sections'].includes(String(source.kind)) ||
    !Array.isArray(raw.slide_ids) || typeof raw.revision !== 'number') return undefined;
  return {
    object: object as RunScope['object'],
    slide_ids: raw.slide_ids.filter((id): id is string => typeof id === 'string'),
    source: {
      kind: source.kind as RunScope['source']['kind'],
      ...(Array.isArray(source.section_ids) ? { section_ids: source.section_ids.filter((id): id is string => typeof id === 'string') } : {}),
    },
    include_run_created_slides: raw.include_run_created_slides === true,
    revision: raw.revision,
  };
}

function readHistoryIntent(data: Record<string, unknown>): RunMode | undefined {
  const mode = data.mode;
  return mode === 'chat' || mode === 'grill' || mode === 'plan' || mode === 'execute'
    ? mode
    : undefined;
}

function readHistorySkills(data: Record<string, unknown>): Skill[] {
  if (!Array.isArray(data.skills)) return [];
  return data.skills.slice(0, 3).flatMap((value) => {
    if (!isRecord(value) || typeof value.id !== 'string' || typeof value.name !== 'string' ||
      typeof value.description !== 'string') return [];
    return [{
      id: value.id,
      name: value.name,
      description: value.description,
      ...(typeof value.local_path === 'string' ? { local_path: value.local_path } : {}),
      ...(typeof value.open_url === 'string' ? { open_url: value.open_url } : {}),
    }];
  });
}

function readHistoryComponents(data: Record<string, unknown>): PublicLoadedResource[] {
  if (!Array.isArray(data.resources)) return [];
  return data.resources.slice(0, 8).flatMap((value) => {
    if (!isRecord(value) || value.kind !== 'component' ||
      typeof value.id !== 'string' || typeof value.name !== 'string') return [];
    return [{
      kind: 'component' as const,
      id: value.id,
      name: value.name,
      ...(typeof value.open_url === 'string' ? { open_url: value.open_url } : {}),
    }];
  });
}

function readBriefingVersions(data: Record<string, unknown>): BriefingVersion[] {
  if (!Array.isArray(data.versions)) return [];
  return data.versions.flatMap((value) => {
    if (!isRecord(value) ||
      typeof value.briefing_id !== 'string' ||
      typeof value.thread_id !== 'string' ||
      typeof value.project_id !== 'string' ||
      (value.kind !== 'kickoff' && value.kind !== 'handoff') ||
      typeof value.version_no !== 'number' ||
      typeof value.content !== 'string' ||
      typeof value.feedback !== 'string' ||
      typeof value.created_at !== 'number') return [];
    return [{
      briefing_id: value.briefing_id,
      thread_id: value.thread_id,
      project_id: value.project_id,
      kind: value.kind as BriefingKind,
      version_no: value.version_no,
      content: value.content,
      feedback: value.feedback,
      created_at: value.created_at,
    }];
  });
}

export function hydrateRunFromHistory(entries: HistoryEntry[] | unknown): HydratedRunView {
  const emptySession: HistorySessionState = {
    activeRunId: null,
    status: 'idle',
    pendingQuestion: null,
  };
  if (!Array.isArray(entries)) return { items: [], plan: null, session: emptySession };
  // Thread History is append-ordered. Public seq is only monotonic within one
  // Run, so sorting a multi-Run thread by seq would interleave separate turns.
  const ordered = entries.slice();
  let items: TimelineItem[] = [];
  let plan: PlanState | null = null;
  let session = emptySession;

  for (const entry of ordered) {
    if (entry.type === 'user_turn') {
      const scope = readHistoryScope(entry.data);
      const mode = readHistoryIntent(entry.data);
      if (!scope || !mode) continue;
      plan = null;
      session = {
        activeRunId: entry.run_id,
        status: 'running',
        scope,
        mode,
        pendingQuestion: null,
      };
      items.push({
        id: `${entry.run_id}:${entry.seq}`,
        type: 'user_turn',
        runId: entry.run_id,
        text: String(entry.data.text ?? ''),
        timestamp: (entry.ts || 0) * 1000,
        scope,
        mode,
        skills: readHistorySkills(entry.data),
        components: readHistoryComponents(entry.data),
		domSelections: readDOMSelections(entry.data),
		referenceOrder: readReferenceOrder(entry.data),
      });
      continue;
    }
    if (entry.type === 'steering') {
      const clientMessageId = String(entry.data.client_message_id ?? '');
      items.push({
        id: `steering_${clientMessageId}`,
        type: 'user_turn',
        runId: entry.run_id,
        text: String(entry.data.text ?? ''),
        clientMessageId,
        deliveryStatus: entry.data.status === 'rejected' ? 'rejected' : 'accepted',
        rejectionCode: String(entry.data.rejection_code ?? ''),
        timestamp: (entry.ts || 0) * 1000,
		domSelections: readDOMSelections(entry.data),
		referenceOrder: readReferenceOrder(entry.data),
      });
      continue;
    }
    if (entry.type === 'git.commit.completed') {
      const commit = isRecord(entry.data.commit) ? entry.data.commit : null;
      if (!commit || typeof commit.title !== 'string' || typeof commit.committed_at !== 'string') continue;
      items.push({
        id: `git-commit:${String(entry.data.operation_id ?? entry.run_id)}`,
        type: 'git_commit',
        operationId: String(entry.data.operation_id ?? entry.run_id),
        status: 'completed',
        title: commit.title,
        items: Array.isArray(commit.items) ? commit.items.filter((item: unknown): item is string => typeof item === 'string') : [],
        branch: String(commit.branch ?? ''),
        hash: String(commit.hash ?? ''),
        filesChanged: Number(commit.files_changed ?? 0),
        insertions: Number(commit.insertions ?? 0),
        deletions: Number(commit.deletions ?? 0),
        timestamp: Date.parse(commit.committed_at) || (entry.ts || 0) * 1000,
      });
      continue;
    }
    if (entry.type === 'git.commit.failed') {
      const error = isRecord(entry.data.error) ? entry.data.error : null;
      items.push({
        id: `git-commit:${String(entry.data.operation_id ?? entry.run_id)}`,
        type: 'git_commit',
        operationId: String(entry.data.operation_id ?? entry.run_id),
        status: 'failed',
        retryable: error?.retryable === true,
        timestamp: Date.parse(String(entry.data.occurred_at ?? '')) || (entry.ts || 0) * 1000,
      });
      continue;
    }
    if (entry.type === 'briefing') {
      const kind = entry.data.kind;
      const briefingId = entry.data.briefing_id;
      const versions = readBriefingVersions(entry.data);
      if ((kind !== 'kickoff' && kind !== 'handoff') ||
        typeof briefingId !== 'string' || versions.length === 0) continue;
      items.push({
        id: `briefing:${briefingId}`,
        type: 'briefing',
        briefingId,
        kind,
        status: 'completed',
        versions,
        timestamp: Number(entry.data.updated_at ?? entry.ts ?? 0) * 1000,
      });
      continue;
    }
    if (entry.type === 'context_compaction') {
      const trigger = entry.data.trigger;
      if ((trigger !== 'auto' && trigger !== 'manual') ||
        typeof entry.data.id !== 'string' ||
        typeof entry.data.summary !== 'string') continue;
      items.push({
        id: `context-compaction:${entry.data.id}`,
        type: 'context_compaction',
        compactionId: entry.data.id,
        trigger,
        summary: entry.data.summary,
        beforeTokens: Number(entry.data.before_tokens ?? 0),
        afterTokens: Number(entry.data.after_tokens ?? 0),
        maxTokens: Number(entry.data.max_tokens ?? 0),
        reclaimedTokens: Number(entry.data.reclaimed_tokens ?? 0),
        durationMs: Number(entry.data.duration_ms ?? 0),
        timestamp: Number(entry.data.created_at ?? entry.ts ?? 0) * 1000,
      });
      continue;
    }
    const event = parsePublicEvent(entry.type, entry.data, String(entry.seq));
    if (!event) continue;
    items = reduceSSEEvent(items, event);
    plan = reducePlan(plan, event);
    if (entry.run_id !== session.activeRunId) {
      session = { activeRunId: entry.run_id, status: 'running', pendingQuestion: null };
    }
    if (event.event === 'question.asked') {
      session = {
        ...session,
        status: 'waiting',
        pendingQuestion: { id: event.data.question_id, prompt: event.data.questions[0].title },
      };
    } else if (event.event === 'question.answered') {
      session = { ...session, status: 'running', pendingQuestion: null };
    } else if (event.event === 'plan.approval_requested') {
      session = { ...session, status: 'waiting', pendingQuestion: null };
    } else if (event.event === 'plan.approval_answered') {
      session = { ...session, status: 'running', pendingQuestion: null };
    } else if (event.event === 'command.permission_requested') {
      session = { ...session, status: 'waiting', pendingQuestion: null };
    } else if (event.event === 'command.permission_answered') {
      session = { ...session, status: 'running', pendingQuestion: null };
    } else if (event.event === 'scope.expansion_requested') {
      session = { ...session, status: 'waiting', pendingQuestion: null };
    } else if (event.event === 'scope.expansion_answered') {
      session = { ...session, status: 'running', pendingQuestion: null };
    } else if (event.event === 'scope.updated') {
      session = { ...session, scope: event.data.scope };
    } else if (event.event === 'run.mode_changed') {
      session = { ...session, status: 'running', mode: event.data.mode, pendingQuestion: null };
    } else if (event.event === 'run.resumed') {
      session = { ...session, status: 'running', pendingQuestion: null };
    } else if (event.event === 'run.completed') {
      session = {
        ...session,
        status: 'done',
        pendingQuestion: null,
      };
    } else if (event.event === 'run.canceled') {
      session = {
        ...session,
        status: 'canceled',
        pendingQuestion: null,
      };
    } else if (event.event === 'run.failed' || event.event === 'run.error') {
      session = {
        ...session,
        status: 'error',
        pendingQuestion: null,
      };
    }
  }
  return {
    items,
    plan,
    session,
    lastEventId: [...ordered].reverse().find((entry) =>
      entry.run_id === session.activeRunId && entry.type !== 'steering')?.seq.toString(),
  };
}

export function hydrateFromHistory(entries: HistoryEntry[] | unknown): TimelineItem[] {
  return hydrateRunFromHistory(entries).items;
}
