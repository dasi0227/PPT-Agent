import { compareRevision } from '../api/session';
import { threadsApi } from '../api/threads';
import type { CommandActivityRecord, PolishRequest, Thread } from '../api/types';
import { useBriefingStore } from './briefingStore';
import { performCommand } from './commandRuntime';
import { useContextWindowStore } from './contextWindowStore';
import { generateNameCommand, polishCommand } from './textCommandStore';
import { useThreadStore } from './threadStore';

export async function retryRecordedCommand(
  record: CommandActivityRecord,
  feedback?: string,
): Promise<boolean> {
  const { id, kind, project_id: projectId, thread_id: threadId, request, result } = record;
  if (kind === 'polish') {
    const next = { ...request, thread_id: threadId } as unknown as PolishRequest;
    // A failure retry reuses the persisted round; only explicit revision
    // feedback rebases it on the most recent successful result.
    if (feedback !== undefined) {
      if (typeof result?.content === 'string') next.polish_content = result.content;
      next.feedback = feedback;
    }
    return polishCommand(threadId, next, id);
  }
  if (kind === 'handoff') {
    const briefing = result?.briefing as { briefing_id?: string } | undefined;
    return useBriefingStore.getState().generate(
      projectId, threadId, kind,
      briefing?.briefing_id,
      feedback ?? (request.feedback as string | undefined), id,
    );
  }
  if (kind === 'compact') {
    await useContextWindowStore.getState().load(threadId, '');
    return useContextWindowStore.getState().compact(threadId, id);
  }
  const apply = (updated: Thread) => useThreadStore.setState((state) => ({
    threadsByProjectId: {
      ...state.threadsByProjectId,
      [projectId]: (state.threadsByProjectId[projectId] ?? []).map((thread) =>
        thread.id === updated.id && compareRevision(updated.naming_revision, thread.naming_revision) >= 0 ? updated : thread),
    },
  }));
  if (record.method === 'auto') return generateNameCommand(threadId, apply, id);
  return performCommand(
    threadId, id,
    (signal) => threadsApi.patch(threadId, { title: String(request.title ?? '') }, id, signal),
    apply,
    () => { void retryRecordedCommand(record); },
  );
}
