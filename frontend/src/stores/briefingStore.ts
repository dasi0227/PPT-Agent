import { create } from 'zustand';
import { briefingsApi } from '../api/briefings';
import type { BriefingKind } from '../api/types';
import type { BriefingTimelineItem } from '../features/agent/eventReducer';
import { IDLE_SESSION, useRunStore } from './runStore';

interface BriefingSession {
  status: 'idle' | 'generating';
  kind: BriefingKind | null;
  threadId: string | null;
  itemId: string | null;
  controller: AbortController | null;
}

interface BriefingStore {
  sessions: Record<string, BriefingSession>;
  generate: (
    projectId: string,
    threadId: string,
    model: string,
    kind: BriefingKind,
    briefingId?: string,
    feedback?: string,
  ) => Promise<boolean>;
  cancel: (projectId: string) => void;
}

const idleSession = (): BriefingSession => ({
  status: 'idle',
  kind: null,
  threadId: null,
  itemId: null,
  controller: null,
});

function updateTimeline(
  threadId: string,
  update: (items: BriefingTimelineItem[]) => BriefingTimelineItem[],
): void {
  useRunStore.setState((state) => {
    const previous = state.sessions[threadId] ?? { ...IDLE_SESSION, processedEventIds: [] };
    const briefingItems = previous.timelineItems.filter(
      (item): item is BriefingTimelineItem => item.type === 'briefing',
    );
    const nextBriefings = update(briefingItems);
    const nextById = new Map(nextBriefings.map((item) => [item.id, item]));
    const timelineItems = previous.timelineItems
      .filter((item) => item.type !== 'briefing' || nextById.has(item.id))
      .map((item) => item.type === 'briefing' ? nextById.get(item.id) ?? item : item);
    const existingIds = new Set(timelineItems.map((item) => item.id));
    timelineItems.push(...nextBriefings.filter((item) => !existingIds.has(item.id)));
    return {
      sessions: {
        ...state.sessions,
        [threadId]: { ...previous, timelineItems },
      },
    };
  });
}

export const useBriefingStore = create<BriefingStore>((set, get) => {
  const patch = (projectId: string, value: Partial<BriefingSession>) => {
    set((state) => ({
      sessions: {
        ...state.sessions,
        [projectId]: { ...(state.sessions[projectId] ?? idleSession()), ...value },
      },
    }));
  };

  return {
    sessions: {},
    generate: async (projectId, threadId, model, kind, briefingId, feedback) => {
      if (get().sessions[projectId]?.status === 'generating') return false;
      const itemId = briefingId ? `briefing:${briefingId}` : `briefing:pending:${kind}:${Date.now()}`;
      const controller = new AbortController();
      const startedAt = Date.now();
      patch(projectId, {
        status: 'generating', kind, threadId, itemId, controller,
      });
      updateTimeline(threadId, (items) => {
        const existing = items.find((item) => item.id === itemId);
        if (existing) {
          return items.map((item) => item.id === itemId
            ? { ...item, status: 'loading', loadingStartedAt: startedAt }
            : item);
        }
        return [...items, {
          id: itemId,
          type: 'briefing',
          briefingId: briefingId ?? '',
          kind,
          status: 'loading',
          versions: [],
          loadingStartedAt: startedAt,
          timestamp: startedAt,
        }];
      });
      try {
        const response = await briefingsApi.generate(projectId, kind, {
          thread_id: threadId,
          model_profile_name: model,
          ...(briefingId ? { briefing_id: briefingId } : {}),
          ...(feedback ? { feedback } : {}),
        }, controller.signal);
        const completed: BriefingTimelineItem = {
          id: `briefing:${response.briefing.briefing_id}`,
          type: 'briefing',
          briefingId: response.briefing.briefing_id,
          kind: response.briefing.kind,
          status: 'completed',
          versions: response.briefing.versions,
          timestamp: response.briefing.updated_at * 1000,
        };
        updateTimeline(threadId, (items) => [
          ...items.filter((item) => item.id !== itemId && item.id !== completed.id),
          completed,
        ]);
        patch(projectId, { ...idleSession() });
        return true;
      } catch {
        updateTimeline(threadId, (items) => briefingId
          ? items.map((item) => item.id === itemId
            ? { ...item, status: 'completed', loadingStartedAt: undefined }
            : item)
          : items.filter((item) => item.id !== itemId));
        patch(projectId, { ...idleSession() });
        return false;
      }
    },
    cancel: (projectId) => {
      const session = get().sessions[projectId];
      if (!session || session.status !== 'generating') return;
      session.controller?.abort();
    },
  };
});

export function isProjectBriefingActive(projectId: string | null): boolean {
  return Boolean(projectId && useBriefingStore.getState().sessions[projectId]?.status === 'generating');
}
