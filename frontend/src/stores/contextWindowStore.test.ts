import { beforeEach, describe, expect, it, vi } from 'vitest';
import { threadsApi } from '../api/threads';
import type { CompactContextResponse } from '../api/types';
import { useContextWindowStore } from './contextWindowStore';
import { useRunStore } from './runStore';

const response: CompactContextResponse = {
  snapshot: {
    total: 100,
    max: 1000,
    ratio: 0.1,
    status: 'idle',
    buckets: { system_prompt: 10, runtime: 10, chat_history: 60, read_file: 10, run_command: 5, other: 5 },
    details: {
      system_prompt: [{ name: 'system prompts', tokens: 5 }, { name: 'tool definitions', tokens: 5 }],
      runtime: [{ name: 'runtime state', tokens: 4 }, { name: 'runtime resources', tokens: 3 }, { name: 'runtime messages', tokens: 3 }],
      chat_history: [{ name: 'user messages', tokens: 20 }, { name: 'assistant messages', tokens: 20 }, { name: 'other tools', tokens: 10 }, { name: 'context summary', tokens: 10 }],
      read_file: [{ name: 'read_ppt', tokens: 5 }, { name: 'read_image', tokens: 0 }, { name: 'read_project', tokens: 5 }],
      run_command: [{ name: 'ls', tokens: 5 }],
      other: [{ name: 'other', tokens: 5 }],
    },
  },
  compaction: {
    id: 'cmp_1', thread_id: 't1', project_id: 'p1', trigger: 'manual',
    title: '收敛上下文协议与前端实现', summary: '## 目标与意图\n继续',
    before_tokens: 500, after_tokens: 100, max_tokens: 1000,
    reclaimed_tokens: 400, duration_ms: 20, created_at: 1,
  },
};

describe('context window store', () => {
  beforeEach(() => {
    useContextWindowStore.setState({ sessions: {} });
    useRunStore.setState({ sessions: {} });
    vi.restoreAllMocks();
  });

  it('inserts a manual compaction immediately and deduplicates by id', async () => {
    vi.spyOn(threadsApi, 'compact').mockResolvedValue(response);
    await expect(useContextWindowStore.getState().compact('t1')).resolves.toBe(true);
    await expect(useContextWindowStore.getState().compact('t1')).resolves.toBe(true);

    const session = useRunStore.getState().sessions.t1;
    expect(session).toBeDefined();
    expect(session.timelineItems).toHaveLength(1);
    expect(session.timelineItems[0]).toMatchObject({
      id: 'context-compaction:cmp_1',
      type: 'context_compaction',
      title: '收敛上下文协议与前端实现',
    });
  });
});
