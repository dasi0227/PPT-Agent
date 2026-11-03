import { useContextWindowStore } from '../../stores/contextWindowStore';

export async function runManualCompaction(threadId: string): Promise<void> {
  await useContextWindowStore.getState().compact(threadId);
}
