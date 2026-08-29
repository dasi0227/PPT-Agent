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
import { showGlobalNotice } from './toastStore';

const STORAGE_KEY = 'ppt-agent-active-git-commits-v1';

export interface ProjectCommitSession {
  operationId: string | null;
  sourceThreadId: string | null;
  status: 'idle' | 'creating' | 'running' | 'empty' | 'completed' | 'failed';
  phase: GitCommitPhase | null;
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
  start: (projectId: string, threadId: string, model: string) => Promise<boolean>;
  recover: () => Promise<void>;
  closeAll: () => void;
}

function readPersisted(): PersistedCommit[] {
  if (typeof sessionStorage === 'undefined') return [];
  try {
    const value = JSON.parse(sessionStorage.getItem(STORAGE_KEY) ?? '[]') as unknown;
    return Array.isArray(value) ? value as PersistedCommit[] : [];
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

function failedItem(operationId: string, occurredAt: string, retryable: boolean): GitCommitTimelineItem {
  return {
    id: `git-commit:${operationId}`,
    type: 'git_commit',
    operationId,
    status: 'failed',
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
      appendTimelineItem(operation.thread_id, failedItem(
        operation.id,
        new Date().toISOString(),
        operation.error?.retryable === true,
      ));
    } else if (operation.status === 'empty') {
      showGlobalNotice('当前项目没有可提交的变更');
    }
    writePersisted(null, projectId);
    patch(projectId, {
      operationId: operation.id,
      sourceThreadId: operation.thread_id,
      status: operation.status === 'accepted' || operation.status === 'running' ? 'running' : operation.status,
      phase: null,
      streamClose: null,
    });
  };

  const subscribe = (projectId: string, threadId: string, operationId: string, lastEventId?: string) => {
    get().sessions[projectId]?.streamClose?.();
    let close = () => {};
    const onMessage = (event: GitCommitEvent) => {
      const session = get().sessions[projectId];
      if (!session || session.operationId !== operationId) return;
      if (event.event === 'git.commit.progress') {
        patch(projectId, {
          status: 'running',
          phase: event.data.phase,
          lastEventId: event.id ?? session.lastEventId,
        });
        writePersisted({
          projectId, threadId, operationId, lastEventId: event.id ?? session.lastEventId,
        }, projectId);
        return;
      }
      close();
      writePersisted(null, projectId);
      if (event.event === 'git.commit.completed') {
        appendTimelineItem(threadId, completedItem(operationId, event.data.commit));
        patch(projectId, { status: 'completed', phase: null, streamClose: null, lastEventId: event.id });
      } else if (event.event === 'git.commit.failed') {
        appendTimelineItem(threadId, failedItem(operationId, event.data.occurred_at, event.data.error.retryable));
        patch(projectId, { status: 'failed', phase: null, streamClose: null, lastEventId: event.id });
      } else {
        showGlobalNotice('当前项目没有可提交的变更');
        patch(projectId, { status: 'empty', phase: null, streamClose: null, lastEventId: event.id });
      }
    };
    close = subscribeGitCommitEvents(operationId, {
      lastEventId,
      onMessage,
      onError: () => {
        void gitCommitsApi.get(operationId).then((operation) => {
          if (!isGitCommitRunning(operation.status)) settleOperation(projectId, operation);
        }).catch(() => {});
      },
    });
    patch(projectId, { streamClose: close });
  };

  return {
    sessions: {},
    getSession: (projectId) => get().sessions[projectId] ?? idleSession(),
    start: async (projectId, threadId, model) => {
      const current = get().sessions[projectId];
      if (current && (current.status === 'creating' || current.status === 'running')) return false;
      patch(projectId, {
        operationId: null, sourceThreadId: threadId, status: 'creating',
        phase: null, lastEventId: undefined, streamClose: null,
      });
      try {
        const operation = await gitCommitsApi.create(projectId, {
          thread_id: threadId,
          model,
          client_request_id: newClientIdentity('req'),
        });
        if (!isGitCommitRunning(operation.status)) {
          settleOperation(projectId, operation);
          return operation.status !== 'failed';
        }
        patch(projectId, {
          operationId: operation.id, sourceThreadId: threadId,
          status: 'running', phase: gitCommitPhase(operation.phase),
        });
        writePersisted({ projectId, threadId, operationId: operation.id }, projectId);
        subscribe(projectId, threadId, operation.id);
        return true;
      } catch {
        patch(projectId, { status: 'idle', phase: null, streamClose: null });
        return false;
      }
    },
    recover: async () => {
      await Promise.all(readPersisted().map(async (saved) => {
        try {
          const operation = await gitCommitsApi.get(saved.operationId);
          patch(saved.projectId, {
            operationId: operation.id, sourceThreadId: operation.thread_id,
            status: operation.status === 'accepted' || operation.status === 'running' ? 'running' : operation.status,
            phase: gitCommitPhase(operation.phase), lastEventId: saved.lastEventId,
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
      }));
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
