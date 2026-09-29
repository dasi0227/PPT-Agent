import { polishApi } from '../api/polish';
import { threadsApi } from '../api/threads';
import type { PolishRequest, Thread } from '../api/types';
import type { CommandTimelineItem } from '../features/agent/eventReducer';
import { notifyModelFallback } from '../lib/modelExecution';
import { performCommand, commandActive } from './commandRuntime';
import { useComposerStore } from './composerStore';
import { showGlobalError } from './toastStore';
import { currentHistoryEpoch } from '../api/client';

let activePolishRequest: symbol | undefined;

export async function polishCommand(
  projectId: string,
  threadId: string,
  request: PolishRequest,
  id = `polish:${crypto.randomUUID()}`,
): Promise<boolean> {
  if (commandActive(id) || useComposerStore.getState().polishing) return false;
  const requestToken = Symbol(id);
  activePolishRequest = requestToken;
  useComposerStore.getState().setPolishing(true);
  const initial: CommandTimelineItem = {
    id,
    type: 'command',
    kind: 'polish',
    title: '润色输入内容',
    status: 'loading',
    timestamp: Date.now(),
  };
  polishRequests.set(id, { projectId, threadId, request });
  try {
    return await performCommand(
      threadId,
      initial,
      (signal, onProgress) => polishApi.polish(projectId, request, signal, onProgress, id),
      (result) => {
        notifyModelFallback(result.model_execution, '输入润色');
        polishRequests.set(id, {
          projectId,
          threadId,
          request: { ...request, instruction: result.content },
        });
        return { ...initial, status: 'completed', title: result.title, content: result.content };
      },
      () => {
        void polishCommand(projectId, threadId, request, id);
      },
    );
  } finally {
    if (activePolishRequest === requestToken) {
      activePolishRequest = undefined;
      useComposerStore.getState().setPolishing(false);
    }
  }
}
const polishRequests = new Map<
  string,
  { projectId: string; threadId: string; request: PolishRequest }
>();
export function retryPolish(id: string, feedback: string) {
  const saved = polishRequests.get(id);
  if (saved)
    return polishCommand(saved.projectId, saved.threadId, { ...saved.request, feedback }, id);
  return Promise.resolve(false);
}
// Naming updates the title and its entry point; it never creates authoring activity.
export async function generateNameCommand(
  _projectId: string,
  threadId: string,
  _previousTitle: string,
  apply: (thread: Thread) => void,
  id = `rename:${crypto.randomUUID()}`,
): Promise<boolean> {
  const epoch = currentHistoryEpoch();
  try {
    const thread = await threadsApi.generateName(threadId, undefined, undefined, id);
    if (epoch !== currentHistoryEpoch()) return false;
    apply(thread);
    return true;
  } catch (error) {
    showGlobalError(error instanceof Error ? error.message : '生成会话名称失败');
    return false;
  }
}
