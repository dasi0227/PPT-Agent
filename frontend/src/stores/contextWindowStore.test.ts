import { beforeEach, describe, expect, it, vi } from 'vitest';
import { threadsApi } from '../api/threads';
import type { CompactContextResponse } from '../api/types';
import { selectContextWindow, useContextWindowStore } from './contextWindowStore';
import { useSessionStore } from './sessionStore';
import { sessionFixture } from '../test/sessionFixtures';

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
    useContextWindowStore.getState().drop('t1');
    useSessionStore.setState({ sessions: { t1: sessionFixture({ contextWindow: response.snapshot }) } });
    vi.restoreAllMocks();
  });

  it('discards an estimate from before a scene switch', async () => {
    let resolveRead!: (value: typeof response.snapshot) => void;
    vi.spyOn(threadsApi, 'contextWindow').mockReturnValue(new Promise(resolve => { resolveRead = resolve; }));
    const loading = useContextWindowStore.getState().load('t1', 'model');
    useSessionStore.setState({ sessions: { t1: sessionFixture({ sceneRevision: '2', generation: 1 }) } });
    resolveRead(response.snapshot);
    await loading;
    const detail = useContextWindowStore.getState().sessions.t1;
    expect(selectContextWindow(useSessionStore.getState().sessions.t1, detail, 'model')).toBeNull();
    expect(detail.loading).toBe(false);
  });

  it('uses shared capacity and leaves timeline insertion to committed batches', async () => {
    vi.spyOn(threadsApi, 'compact').mockResolvedValue(response);
    await expect(useContextWindowStore.getState().compact('t1', 'compact:request1')).resolves.toBe(true);
    expect(useSessionStore.getState().sessions.t1.itemsById).toEqual({});
  });
});
