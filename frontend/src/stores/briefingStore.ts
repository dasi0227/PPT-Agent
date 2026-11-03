import { create } from 'zustand';
import { briefingsApi } from '../api/briefings';
import type { BriefingKind } from '../api/types';
import type { BriefingTimelineItem } from '../features/agent/timelineTypes';
import { useRunStore } from './runStore';
import { performCommand } from './commandRuntime';
interface BriefingSession {
  status: 'idle' | 'generating';
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
    set((state) => ({
      sessions: {
        ...state.sessions,
        [projectId]: { status: 'generating', requestKey },
      },
    }));
    const retry = () => {
      void get().generate(projectId, threadId, kind, briefingId, feedback, id);
    };
    try {
      return await performCommand(
        threadId,
        id,
        (signal) =>
          briefingsApi.generate(
            kind,
            {
              thread_id: threadId,
              ...(feedback ? { feedback } : {}),
            },
            signal,
            undefined,
            id,
          ),
        () => {},
        retry,
      );
    } finally {
      if (get().sessions[projectId]?.requestKey === requestKey)
        set((state) => ({
          sessions: {
            ...state.sessions,
            [projectId]: { status: 'idle' },
          },
        }));
    }
  },
}));
