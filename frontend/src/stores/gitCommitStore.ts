import { create } from 'zustand';
import { cancelPersistedCommand, runCommand } from '../api/commands';
import { gitCommitPhase, isGitCommitRunning, type GitCommitOutput } from '../api/gitCommits';
import { compareRevision } from '../api/session';
import type { GitCommitPhase } from '../api/types';
import { useSessionStore, type SessionState } from './sessionStore';
import { showGlobalError, showGlobalWarning } from './toastStore';
import { RequestCanceledError } from '../api/client';

export interface ProjectCommitSession {
  operationId: string | null;
  sourceThreadId: string | null;
  status: 'idle' | 'creating' | 'running' | 'empty' | 'completed' | 'failed' | 'canceled';
  phase: GitCommitPhase | null;
  displayPhase?: number;
  startedAt?: number;
  canceling?: boolean;
}
const usePendingCommits = create<{ pending: Record<string, { threadId: string; id: string }> }>(() => ({ pending: {} }));
const actions = {
  async start(projectId: string, threadId: string, commandId?: string): Promise<boolean> {
    const current = getState().sessions[projectId];
    if (current && ['creating', 'running'].includes(current.status)) return false;
    const id = commandId ?? `commit:${crypto.randomUUID()}`;
    usePendingCommits.setState(state => ({ pending: { ...state.pending, [projectId]: { threadId, id } } }));
    try {
      await useSessionStore.getState().ensure(threadId, projectId);
      const result = await runCommand<GitCommitOutput>(threadId, 'commit', {}, { commandId: id });
      if (result.empty) showGlobalWarning('当前项目没有可提交的变更');
      return true;
    } catch (error) {
      if (!(error instanceof RequestCanceledError)) showGlobalError(error instanceof Error ? error.message : '请求失败，请重试');
      return false;
    } finally {
      usePendingCommits.setState(state => {
        const pending = { ...state.pending }; delete pending[projectId]; return { pending };
      });
    }
  },
  async cancel(projectId: string): Promise<void> {
    const command = getState().sessions[projectId];
    if (!command?.operationId || command.canceling || command.phase === 'committing') return;
    try {
      await cancelPersistedCommand(command.operationId);
      if (command.sourceThreadId) await useSessionStore.getState().reconnect(command.sourceThreadId);
    } catch (error) { showGlobalError(error instanceof Error ? error.message : '停止命令失败'); }
  },
};
type GitCommitStore = typeof actions & { sessions: Record<string, ProjectCommitSession> };
const cache = new WeakMap<object, WeakMap<object, GitCommitStore>>();
function present(sources: Record<string, SessionState>, pending: ReturnType<typeof usePendingCommits.getState>['pending']): GitCommitStore {
  let inner = cache.get(sources);
  if (!inner) { inner = new WeakMap(); cache.set(sources, inner); }
  const saved = inner.get(pending); if (saved) return saved;
  const sessions: Record<string, ProjectCommitSession> = {};
  const commands = Object.values(sources).flatMap(source => Object.values(source.commandsById)).filter(command => command.kind === 'commit')
    .sort((a, b) => a.created_at - b.created_at || compareRevision(a.entity_revision, b.entity_revision));
  for (const command of commands) {
    const result = command.result as GitCommitOutput | undefined;
    sessions[command.project_id] = { operationId: command.command_id, sourceThreadId: command.thread_id,
      status: isGitCommitRunning(command.status) ? 'running' : command.status === 'completed' ? result?.empty ? 'empty' : 'completed'
        : command.status === 'canceled' ? 'canceled' : 'failed',
      phase: gitCommitPhase(command.phase), displayPhase: command.phase, startedAt: command.created_at, canceling: command.status === 'cancel_requested' };
  }
  for (const [projectId, value] of Object.entries(pending)) {
    if (sessions[projectId]?.operationId !== value.id) sessions[projectId] = {
      operationId: value.id, sourceThreadId: value.threadId, status: 'creating', phase: null,
    };
  }
  const value = { ...actions, sessions }; inner.set(pending, value); return value;
}
function getState(): GitCommitStore { return present(useSessionStore.getState().sessions, usePendingCommits.getState().pending); }
export const useGitCommitStore = Object.assign(
  function useCommitView<T = GitCommitStore>(selector: (value: GitCommitStore) => T = value => value as unknown as T): T {
    const sessions = useSessionStore(state => state.sessions);
    const pending = usePendingCommits(state => state.pending);
    return selector(present(sessions, pending));
  }, { getState },
);
