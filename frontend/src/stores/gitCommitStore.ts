import { create } from 'zustand';
import { commandsApi, cancelPersistedCommand } from '../api/commands';
import {
  gitCommitPhase,
  isGitCommitRunning,
  subscribeGitCommitExecution,
  type GitCommitExecution,
  type GitCommitOutput,
} from '../api/gitCommits';
import type {
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

function commitStatus(command: GitCommitExecution): ProjectCommitSession['status'] {
  if (isGitCommitRunning(command.status)) return 'running';
  if (command.status === 'completed') return command.result?.empty ? 'empty' : 'completed';
  return command.status === 'canceled' ? 'canceled' : 'failed';
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

  const settleOperation = (projectId: string, operation: GitCommitExecution) => {
    const current = get().sessions[projectId];
    current?.streamClose?.();
    if (operation.status === 'completed' && operation.result && !operation.result.empty) {
      appendTimelineItem(operation.thread_id, completedItem(operation.command_id, operation.result));
    } else if (['failed', 'canceled', 'interrupted'].includes(operation.status)) {
      appendTimelineItem(
        operation.thread_id,
        failedItem(
          operation.command_id,
          new Date().toISOString(),
          operation.error?.retryable === true,
          operation.status === 'canceled',
        ),
      );
    } else if (operation.result?.empty) {
      showGlobalWarning('当前项目没有可提交的变更');
    }
    writePersisted(null, projectId);
    patch(projectId, {
      operationId: operation.command_id,
      sourceThreadId: operation.thread_id,
      status: commitStatus(operation),
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
  ) => {
    get().sessions[projectId]?.streamClose?.();
    let close = () => {};
    let chain = Promise.resolve(),
      paintedAt = 0,
      lastPhase = -1,
      terminal = false;
    const onUpdate = (command: GitCommitExecution) => {
      const session = get().sessions[projectId];
      if (!session || session.operationId !== operationId || terminal ||
        !['creating', 'running'].includes(session.status)) return;
      if (isGitCommitRunning(command.status)) {
        patch(projectId, { status: 'running', phase: gitCommitPhase(command.phase) });
        writePersisted({ projectId, threadId, operationId }, projectId);
        const next = command.phase;
        if (next > lastPhase) {
          lastPhase = next;
          chain = chain.then(async () => {
            await new Promise((resolve) =>
              window.setTimeout(resolve, Math.max(0, 450 - (Date.now() - paintedAt))),
            );
            const live = get().sessions[projectId];
            if (live?.operationId !== operationId || !['creating', 'running'].includes(live.status)) return;
            patch(projectId, { displayPhase: next });
            paintedAt = Date.now();
          });
        }
        return;
      }
      terminal = true;
      close();
      writePersisted(null, projectId);
      const result = command.result;
      if (command.status === 'completed' && result && !result.empty) {
        patch(projectId, { settling: true, streamClose: null });
        void chain.then(async () => {
          await new Promise((resolve) =>
            window.setTimeout(resolve, Math.max(0, 450 - (Date.now() - paintedAt))),
          );
          if (get().sessions[projectId]?.operationId !== operationId ||
            get().sessions[projectId]?.status !== 'running') return;
          appendTimelineItem(threadId, completedItem(operationId, result));
          patch(projectId, {
            status: 'completed', phase: null, settling: false, canceling: false,
          });
        });
      } else {
        settleOperation(projectId, command);
      }
    };
    close = subscribeGitCommitExecution(operationId, {
      onUpdate,
      onError: () => {
        if (terminal) return;
        void commandsApi.get<GitCommitOutput>(operationId).then((operation) => {
          if (get().sessions[projectId]?.operationId === operationId && !isGitCommitRunning(operation.status))
            settleOperation(projectId, operation);
        }).catch(() => {});
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
        streamClose: null,
      });
      try {
        const operation = await commandsApi.create<GitCommitOutput>(threadId, 'commit', {}, {
          requestKey: newClientIdentity('req'), commandId,
        });
        if (!isGitCommitRunning(operation.status)) {
          settleOperation(projectId, operation);
          return operation.status === 'completed';
        }
        patch(projectId, {
          operationId: operation.command_id,
          sourceThreadId: threadId,
          status: 'running',
          phase: gitCommitPhase(operation.phase),
        });
        writePersisted({ projectId, threadId, operationId: operation.command_id }, projectId);
        subscribe(projectId, threadId, operation.command_id);
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
        await cancelPersistedCommand(session.operationId);
        const operation = await commandsApi.get<GitCommitOutput>(session.operationId);
        if (get().sessions[projectId]?.operationId === operation.command_id && !isGitCommitRunning(operation.status))
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
            const operation = await commandsApi.get<GitCommitOutput>(saved.operationId);
            patch(saved.projectId, {
              operationId: operation.command_id,
              sourceThreadId: operation.thread_id,
              status: commitStatus(operation),
              phase: gitCommitPhase(operation.phase),
            });
            if (isGitCommitRunning(operation.status)) {
              subscribe(saved.projectId, saved.threadId, saved.operationId);
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
