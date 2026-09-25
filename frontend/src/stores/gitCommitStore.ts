import { create } from 'zustand';
import {
  gitCommitPhase,
  gitCommitsApi,
  isGitCommitRunning,
  subscribeGitCommitEvents,
} from '../api/gitCommits';
import type {
  GitCommitEvent,
  GitCommitOperation,
  GitCommitPhase,
  GitCommitResult,
} from '../api/types';
import type { GitCommitTimelineItem } from '../features/agent/eventReducer';
import { newClientIdentity } from '../lib/clientIdentity';
import { IDLE_SESSION, useRunStore } from './runStore';
import { showGlobalWarning } from './toastStore';

const STORAGE_KEY = 'ppt-agent-active-git-commits-v1';

export interface ProjectCommitSession {
  operationId: string | null;
  sourceThreadId: string | null;
  status: 'idle' | 'creating' | 'running' | 'empty' | 'completed' | 'failed' | 'canceled';
  phase: GitCommitPhase | null;
  displayPhase?: number;
  startedAt?: number;
  settling?: boolean;
  canceling?: boolean;
  lastEventId?: string;
  streamClose: (() => void) | null;
}

const idleSession = (): ProjectCommitSession => ({
  operationId: null,
  sourceThreadId: null,
  status: 'idle',
  phase: null,
  streamClose: null,
});

interface PersistedCommit {
  projectId: string;
  threadId: string;
  operationId: string;
  lastEventId?: string;
}

interface GitCommitStore {
  sessions: Record<string, ProjectCommitSession>;
  getSession: (projectId: string) => ProjectCommitSession;
  start: (projectId: string, threadId: string, commandId?: string) => Promise<boolean>;
  cancel: (projectId: string) => Promise<void>;
  recover: () => Promise<void>;
  closeAll: () => void;
}

function readPersisted(): PersistedCommit[] {
  if (typeof sessionStorage === 'undefined') return [];
  try {
    const value = JSON.parse(sessionStorage.getItem(STORAGE_KEY) ?? '[]') as unknown;
    return Array.isArray(value) ? (value as PersistedCommit[]) : [];
  } catch {
    return [];
  }
}

function writePersisted(value: PersistedCommit | null, projectId: string): void {
  if (typeof sessionStorage === 'undefined') return;
  const next = readPersisted().filter((item) => item.projectId !== projectId);
  if (value) next.push(value);
  if (next.length === 0) sessionStorage.removeItem(STORAGE_KEY);
  else sessionStorage.setItem(STORAGE_KEY, JSON.stringify(next));
}

function appendTimelineItem(threadId: string, item: GitCommitTimelineItem): void {
  useRunStore.setState((state) => {
    const previous = state.sessions[threadId] ?? { ...IDLE_SESSION, processedEventIds: [] };
    const index = previous.timelineItems.findIndex((candidate) => candidate.id === item.id);
    const timelineItems = previous.timelineItems.slice();
    if (index >= 0) timelineItems[index] = item;
    else timelineItems.push(item);
    return {
      sessions: {
        ...state.sessions,
        [threadId]: { ...previous, timelineItems },
      },
    };
  });
}

function completedItem(operationId: string, result: GitCommitResult): GitCommitTimelineItem {
  return {
    id: `git-commit:${operationId}`,
    type: 'git_commit',
    operationId,
    status: 'completed',
    title: result.title,
    items: result.items,
    branch: result.branch,
    hash: result.hash,
    filesChanged: result.files_changed,
    insertions: result.insertions,
    deletions: result.deletions,
    timestamp: Date.parse(result.committed_at) || Date.now(),
  };
}

function failedItem(
  operationId: string,
  occurredAt: string,
  retryable: boolean,
  canceled = false,
): GitCommitTimelineItem {
  return {
    id: `git-commit:${operationId}`,
    type: 'git_commit',
    operationId,
    status: canceled ? 'canceled' : 'failed',
    retryable,
    timestamp: Date.parse(occurredAt) || Date.now(),
  };
}

export const useGitCommitStore = create<GitCommitStore>((set, get) => {
  const patch = (projectId: string, value: Partial<ProjectCommitSession>) => {
    set((state) => ({
      sessions: {
        ...state.sessions,
        [projectId]: { ...(state.sessions[projectId] ?? idleSession()), ...value },
      },
    }));
  };

  const settleOperation = (projectId: string, operation: GitCommitOperation) => {
    const current = get().sessions[projectId];
    current?.streamClose?.();
    if (operation.status === 'completed' && operation.result) {
      appendTimelineItem(operation.thread_id, completedItem(operation.id, operation.result));
    } else if (operation.status === 'failed') {
      appendTimelineItem(
        operation.thread_id,
        failedItem(
          operation.id,
          new Date().toISOString(),
          operation.error?.retryable === true,
          operation.error?.code === 'COMMIT_CANCELED',
        ),
      );
    } else if (operation.status === 'empty') {
      showGlobalWarning('当前项目没有可提交的变更');
    }
    writePersisted(null, projectId);
    patch(projectId, {
      operationId: operation.id,
      sourceThreadId: operation.thread_id,
      status:
        operation.error?.code === 'COMMIT_CANCELED'
          ? 'canceled'
          : operation.status === 'accepted' || operation.status === 'running'
            ? 'running'
            : operation.status,
      phase: null,
      settling: false,
      canceling: false,
      streamClose: null,
    });
  };

  const subscribe = (
    projectId: string,
    threadId: string,
    operationId: string,
    lastEventId?: string,
  ) => {
    get().sessions[projectId]?.streamClose?.();
    let close = () => {};
    let chain = Promise.resolve(),
      paintedAt = 0,
      lastPhase = -1,
      terminal = false;
    const onMessage = (event: GitCommitEvent) => {
      const session = get().sessions[projectId];
      if (
        !session ||
        session.operationId !== operationId ||
        terminal ||
        !['creating', 'running'].includes(session.status)
      )
        return;
      if (event.event === 'git.commit.progress') {
        if (event.data.model_switch)
          showGlobalWarning(`提交说明已切换至备用模型 ${event.data.model_switch.to}`);
        patch(projectId, {
          status: 'running',
          phase: event.data.phase,
          lastEventId: event.id ?? session.lastEventId,
        });
        writePersisted(
          {
            projectId,
            threadId,
            operationId,
            lastEventId: event.id ?? session.lastEventId,
          },
          projectId,
        );
        const next = ['staging', 'analyzing', 'committing'].indexOf(event.data.phase);
        if (next > lastPhase) {
          lastPhase = next;
          chain = chain.then(async () => {
            await new Promise((resolve) =>
              window.setTimeout(resolve, Math.max(0, 450 - (Date.now() - paintedAt))),
            );
            const live = get().sessions[projectId];
            if (live?.operationId !== operationId || !['creating', 'running'].includes(live.status))
              return;
            patch(projectId, { displayPhase: next });
            paintedAt = Date.now();
          });
        }

        return;
      }
      terminal = true;
      close();
      writePersisted(null, projectId);
      if (event.event === 'git.commit.completed') {
        patch(projectId, { settling: true, streamClose: null });
        void chain.then(async () => {
          await new Promise((resolve) =>
            window.setTimeout(resolve, Math.max(0, 450 - (Date.now() - paintedAt))),
          );
          if (
            get().sessions[projectId]?.operationId !== operationId ||
            get().sessions[projectId]?.status !== 'running'
          )
            return;
          appendTimelineItem(threadId, completedItem(operationId, event.data.commit));
          patch(projectId, {
            status: 'completed',
            phase: null,
            settling: false,
            canceling: false,
            lastEventId: event.id,
          });
        });
      } else if (event.event === 'git.commit.failed') {
        appendTimelineItem(
          threadId,
          failedItem(
            operationId,
            event.data.occurred_at,
            event.data.error.retryable,
            event.data.error.code === 'COMMIT_CANCELED',
          ),
        );
        patch(projectId, {
          status: event.data.error.code === 'COMMIT_CANCELED' ? 'canceled' : 'failed',
          phase: null,
          streamClose: null,
          canceling: false,
          lastEventId: event.id,
        });
      } else {
        showGlobalWarning('当前项目没有可提交的变更');
        patch(projectId, {
          status: 'empty',
          phase: null,
          streamClose: null,
          lastEventId: event.id,
        });
      }
    };
    close = subscribeGitCommitEvents(operationId, {
      lastEventId,
      onMessage,
      onError: () => {
        if (terminal) return;
        void gitCommitsApi
          .get(operationId)
          .then((operation) => {
            if (
              get().sessions[projectId]?.operationId === operationId &&
              !isGitCommitRunning(operation.status)
            )
              settleOperation(projectId, operation);
          })
          .catch(() => {});
      },
    });
    patch(projectId, { streamClose: close });
  };

  return {
    sessions: {},
    getSession: (projectId) => get().sessions[projectId] ?? idleSession(),
    start: async (projectId, threadId, commandId) => {
      const current = get().sessions[projectId];
      if (current && (current.status === 'creating' || current.status === 'running')) return false;
      patch(projectId, {
        operationId: null,
        sourceThreadId: threadId,
        status: 'creating',
        phase: null,
        displayPhase: -1,
        startedAt: Date.now(),
        settling: false,
        canceling: false,
        lastEventId: undefined,
        streamClose: null,
      });
      try {
        const operation = await gitCommitsApi.create(projectId, {
          thread_id: threadId,
          client_request_id: newClientIdentity('req'),
          command_id: commandId,
        });
        if (!isGitCommitRunning(operation.status)) {
          settleOperation(projectId, operation);
          return operation.status !== 'failed';
        }
        patch(projectId, {
          operationId: operation.id,
          sourceThreadId: threadId,
          status: 'running',
          phase: gitCommitPhase(operation.phase),
        });
        writePersisted({ projectId, threadId, operationId: operation.id }, projectId);
        subscribe(projectId, threadId, operation.id);
        return true;
      } catch {
        patch(projectId, { status: 'idle', phase: null, streamClose: null });
        return false;
      }
    },
    cancel: async (projectId) => {
      const session = get().sessions[projectId];
      if (
        !session?.operationId ||
        session.canceling ||
        session.settling ||
        session.phase === 'committing'
      )
        return;
      patch(projectId, { canceling: true });
      try {
        const operation = await gitCommitsApi.cancel(session.operationId);
        if (get().sessions[projectId]?.operationId === operation.id && !isGitCommitRunning(operation.status))
          settleOperation(projectId, operation);
      } catch {
        /* The API reports the error; keep observing the actual operation. */
      } finally {
        if (get().sessions[projectId]?.operationId === session.operationId)
          patch(projectId, { canceling: false });
      }
    },
    recover: async () => {
      await Promise.all(
        readPersisted().map(async (saved) => {
          try {
            const operation = await gitCommitsApi.get(saved.operationId);
            patch(saved.projectId, {
              operationId: operation.id,
              sourceThreadId: operation.thread_id,
              status:
                operation.error?.code === 'COMMIT_CANCELED'
                  ? 'canceled'
                  : operation.status === 'accepted' || operation.status === 'running'
                    ? 'running'
                    : operation.status,
              phase: gitCommitPhase(operation.phase),
              lastEventId: saved.lastEventId,
            });
            if (isGitCommitRunning(operation.status)) {
              subscribe(saved.projectId, saved.threadId, saved.operationId, saved.lastEventId);
            } else {
              settleOperation(saved.projectId, operation);
            }
          } catch {
            writePersisted(null, saved.projectId);
            patch(saved.projectId, { status: 'idle', phase: null, streamClose: null });
          }
        }),
      );
    },
    closeAll: () => {
      Object.values(get().sessions).forEach((session) => session.streamClose?.());
    },
  };
});

export function isProjectCommitActive(projectId: string | null): boolean {
  if (!projectId) return false;
  const status = useGitCommitStore.getState().getSession(projectId).status;
  return status === 'creating' || status === 'running';
}
