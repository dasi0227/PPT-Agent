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
    compactable_tokens: 12000,
    compact_threshold_tokens: 12000,
    status: 'idle',
    buckets: { system_prompt: 10, runtime: 20, chat_history: 55, read_file: 10, other: 5 },
    details: {
      system_prompt: [{ name: 'system prompts', tokens: 5 }, { name: 'tool definitions', tokens: 5 }],
      runtime: [{ name: 'runtime context', tokens: 7 }, { name: 'runtime messages', tokens: 13 }],
      chat_history: [{ name: 'user messages', tokens: 20 }, { name: 'assistant messages', tokens: 20 }, { name: 'tools execution', tokens: 15 }],
      read_file: [{ name: 'read_resource', tokens: 10 }, { name: 'read_image', tokens: 0 }],
      other: [{ name: 'other', tokens: 5 }],
    },
  },
  compaction: {
    id: 'cmp_1', thread_id: 't1', project_id: 'p1', trigger: 'manual',
    title: '收敛上下文协议与前端实现', content: '## 目标与意图\n继续',
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

  it('keeps a live snapshot when an earlier read resolves later', async () => {
    let resolveRead!: (value: typeof response.snapshot) => void;
    vi.spyOn(threadsApi, 'contextWindow').mockReturnValue(new Promise((resolve) => { resolveRead = resolve; }));
    const loading = useContextWindowStore.getState().load('t1', 'model');
    const live = { ...response.snapshot };
    useContextWindowStore.getState().update('t1', live);
    resolveRead({ ...response.snapshot });
    await loading;
    expect(useContextWindowStore.getState().sessions.t1.snapshot).toBe(live);
    expect(useContextWindowStore.getState().sessions.t1.loading).toBe(false);
  });

  it('inserts a manual compaction immediately and deduplicates by id', async () => {
    vi.spyOn(threadsApi, 'compact').mockResolvedValue(response);
    useContextWindowStore.setState({
      sessions: {
        t1: { snapshot: response.snapshot, loading: false, compacting: false },
      },
    });
    await expect(useContextWindowStore.getState().compact('t1', 'compact:request1')).resolves.toBe(true);
    await expect(useContextWindowStore.getState().compact('t1', 'compact:request1')).resolves.toBe(true);

    const session = useRunStore.getState().sessions.t1;
    expect(session).toBeDefined();
    expect(session.timelineItems).toHaveLength(1);
    expect(session.timelineItems[0]).toMatchObject({
      id: 'compact:request1',
      type: 'context_compaction',
      title: '收敛上下文协议与前端实现',
    });
  });
});
