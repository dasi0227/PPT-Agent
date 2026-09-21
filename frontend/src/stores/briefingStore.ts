import { create } from 'zustand';
import { briefingsApi } from '../api/briefings';
import type { BriefingKind } from '../api/types';
import type { BriefingTimelineItem } from '../features/agent/eventReducer';
import { notifyModelFallback } from '../lib/modelExecution';
import { useRunStore } from './runStore';
import { cancelCommand, performCommand } from './commandRuntime';
interface BriefingSession {
  status: 'idle' | 'generating';
  kind: BriefingKind | null;
  threadId: string | null;
  itemId: string | null;
  requestKey?: string;
}
interface BriefingStore {
  sessions: Record<string, BriefingSession>;
  generate: (
    projectId: string,
    threadId: string,
    kind: BriefingKind,
    briefingId?: string,
    feedback?: string,
    retryItemId?: string,
  ) => Promise<boolean>;
  cancel: (projectId: string) => void;
}
export const useBriefingStore = create<BriefingStore>((set, get) => ({
  sessions: {},
  generate: async (projectId, threadId, kind, briefingId, feedback, retryItemId) => {
    if (get().sessions[projectId]?.status === 'generating') return false;
    const existing = useRunStore
      .getState()
      .sessions[
        threadId
      ]?.timelineItems.find((item): item is BriefingTimelineItem => item.type === 'briefing' && (item.id === retryItemId || (!!briefingId && item.briefingId === briefingId)));
    const requestKey = crypto.randomUUID();
    const id = retryItemId ?? existing?.id ?? `briefing:${crypto.randomUUID()}`;
    const initial: BriefingTimelineItem = {
      ...existing,
      id,
      type: 'briefing',
      kind,
      briefingId: briefingId ?? '',
      status: 'loading',
      versions: existing?.versions.slice(-1) ?? [],
      timestamp: Date.now(),
    };
    set((state) => ({
      sessions: {
        ...state.sessions,
        [projectId]: { status: 'generating', kind, threadId, itemId: id, requestKey },
      },
    }));
    const retry = () => {
      void get().generate(projectId, threadId, kind, briefingId, feedback, id);
    };
    try {
      return await performCommand(
        threadId,
        initial,
        (signal, onProgress) =>
          briefingsApi.generate(
            projectId,
            kind,
            {
              thread_id: threadId,
              ...(briefingId ? { briefing_id: briefingId } : {}),
              ...(feedback ? { feedback } : {}),
            },
            signal,
            onProgress,
          ),
        (result) => {
          notifyModelFallback(result.model_execution, kind === 'kickoff' ? '启动说明' : '交接内容');
          return {
            ...initial,
            id: `briefing:${result.briefing.briefing_id}`,
            status: 'completed',
            phase: 2,
            briefingId: result.briefing.briefing_id,
            versions: result.briefing.versions.slice(-1),
          };
        },
        retry,
      );
    } finally {
      if (get().sessions[projectId]?.requestKey === requestKey)
        set((state) => ({
          sessions: {
            ...state.sessions,
            [projectId]: { status: 'idle', kind: null, threadId: null, itemId: null },
          },
        }));
    }
  },
  cancel: (projectId) => {
    const id = get().sessions[projectId]?.itemId;
    if (id) cancelCommand(id);
  },
}));
export function isProjectBriefingActive(projectId: string | null) {
  return Boolean(
    projectId && useBriefingStore.getState().sessions[projectId]?.status === 'generating',
  );
}
