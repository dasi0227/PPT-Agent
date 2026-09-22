import { polishApi } from '../api/polish';
import { threadsApi } from '../api/threads';
import type { PolishRequest, Thread } from '../api/types';
import type { CommandTimelineItem } from '../features/agent/eventReducer';
import { notifyModelFallback } from '../lib/modelExecution';
import { performCommand, commandActive } from './commandRuntime';
import { useComposerStore } from './composerStore';

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
      (signal, onProgress) => polishApi.polish(projectId, request, signal, onProgress),
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
export function generateNameCommand(
  projectId: string,
  threadId: string,
  previousTitle: string,
  apply: (thread: Thread) => void,
  id = `rename:${crypto.randomUUID()}`,
) {
  const initial: CommandTimelineItem = {
    id,
    type: 'command',
    kind: 'rename',
    title: previousTitle,
    status: 'loading',
    method: 'auto',
    timestamp: Date.now(),
  };
  return performCommand(
    threadId,
    initial,
    (signal, onProgress) => threadsApi.generateName(threadId, signal, onProgress),
    (thread) => {
      apply(thread);
      return {
        ...initial,
        status: 'completed',
        title: thread.title,
        content: `${previousTitle || '未命名会话'} → ${thread.title}`,
      };
    },
    () => {
      void generateNameCommand(projectId, threadId, previousTitle, apply, id);
    },
  );
}
