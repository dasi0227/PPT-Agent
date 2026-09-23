import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { polishApi } from '../api/polish';
import type { PolishRequest } from '../api/types';
import { useComposerStore } from './composerStore';
import { useRunStore } from './runStore';
import { polishCommand, retryPolish } from './textCommandStore';

beforeEach(() => {
  vi.useFakeTimers();
  useRunStore.setState({ sessions: {} });
  useComposerStore.setState({ polishing: false, threadDrafts: { t1: '原始草稿' } });
});
afterEach(() => {
  vi.restoreAllMocks();
  vi.useRealTimers();
});

it('uses generated titles in the timeline and only the latest content for revisions', async () => {
  const polish = vi.spyOn(polishApi, 'polish')
    .mockResolvedValueOnce({ title: '明确受众与交付要求', content: '第一版完整指令', changed: true, prompt_version: 'test' })
    .mockResolvedValueOnce({ title: '补充验收要求', content: '第二版完整指令', changed: true, prompt_version: 'test' });
  const request: PolishRequest = {
    instruction: '原始草稿', thread_id: 't1', mode: 'chat',
    scope: { selection: { kind: 'all_pages' } },
  };
  const first = polishCommand('p1', 't1', request, 'polish:result-contract');
  await vi.runAllTimersAsync();
  await expect(first).resolves.toBe(true);
  expect(useRunStore.getState().sessions.t1.timelineItems).toEqual([
    expect.objectContaining({ title: '明确受众与交付要求', content: '第一版完整指令', status: 'completed' }),
  ]);
  const revision = retryPolish('polish:result-contract', '补充验收要求');
  await vi.runAllTimersAsync();
  await expect(revision).resolves.toBe(true);
  expect(polish.mock.calls[1][1]).toMatchObject({ instruction: '第一版完整指令', feedback: '补充验收要求' });
  expect(useRunStore.getState().sessions.t1.timelineItems).toEqual([
    expect.objectContaining({ title: '补充验收要求', content: '第二版完整指令', status: 'completed' }),
  ]);
  expect(useComposerStore.getState().threadDrafts.t1).toBe('原始草稿');
});
