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
    if (typeof result?.content === 'string') next.instruction = result.content;
    if (feedback !== undefined) next.feedback = feedback;
    return polishCommand(projectId, threadId, next, id);
  }
  if (kind === 'kickoff' || kind === 'handoff') {
    const briefing = result?.briefing as { briefing_id?: string } | undefined;
    return useBriefingStore.getState().generate(
      projectId, threadId, kind,
      briefing?.briefing_id ?? (request.briefing_id as string | undefined),
      feedback ?? (request.feedback as string | undefined), id,
    );
  }
  if (kind === 'compact') {
    await useContextWindowStore.getState().load(threadId, '');
    return useContextWindowStore.getState().compact(threadId, id);
  }
  const store = useThreadStore.getState();
  const previous = store.threadsByProjectId[projectId]?.find((thread) => thread.id === threadId)?.title ?? record.previous_title;
  const apply = (updated: Thread) => useThreadStore.setState((state) => ({
    threadsByProjectId: {
      ...state.threadsByProjectId,
      [projectId]: (state.threadsByProjectId[projectId] ?? []).map((thread) =>
        thread.id === updated.id && updated.naming_revision >= thread.naming_revision ? updated : thread),
    },
  }));
  if (record.method === 'auto') return generateNameCommand(projectId, threadId, previous, apply, id);
  const initial = {
    id, type: 'command' as const, kind: 'rename' as const, title: previous || '新会话',
    status: 'loading' as const, method: 'manual' as const, timestamp: record.created_at,
  };
  return performCommand(
    threadId, initial,
    (signal) => threadsApi.patch(threadId, { title: String(request.title ?? '') }, id, signal),
    (thread) => {
      apply(thread);
      const title = thread.title.trim() || '新会话';
      return {
        ...initial, status: 'completed', title,
        content: previous.trim() === thread.title.trim()
          ? `保留当前名称：${title}` : `${previous || '新会话'} → ${title}`,
      };
    },
    () => { void retryRecordedCommand(record); },
  );
}
