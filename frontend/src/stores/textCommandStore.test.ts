import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { polishApi } from '../api/polish';
import type { CommandActivityRecord, PolishRequest } from '../api/types';
import { useComposerStore } from './composerStore';
import { useRunStore } from './runStore';
import { polishCommand, retryPolish } from './textCommandStore';
import { retryRecordedCommand } from './commandRecovery';

beforeEach(() => {
  vi.useFakeTimers();
  useRunStore.setState({ sessions: {} });
  useComposerStore.setState({ polishing: false, threadDrafts: { t1: '原始草稿' } });
});
afterEach(() => {
  vi.restoreAllMocks();
  vi.useRealTimers();
});

it('preserves the original draft and uses only the latest result for revisions', async () => {
  const polish = vi.spyOn(polishApi, 'polish')
    .mockResolvedValueOnce({ title: '明确受众与交付要求', content: '第一版完整指令', changed: true, prompt_version: 'test' })
    .mockResolvedValueOnce({ title: '补充验收要求', content: '第二版完整指令', changed: true, prompt_version: 'test' })
    .mockResolvedValueOnce({ title: '精简表达', content: '第三版完整指令', changed: true, prompt_version: 'test' });
  const request: PolishRequest = {
    instruction: '原始草稿', thread_id: 't1',
  };
  const first = polishCommand('t1', request, 'polish:result-contract');
  await vi.runAllTimersAsync();
  await expect(first).resolves.toBe(true);
  expect(useRunStore.getState().sessions.t1.timelineItems).toEqual([
    expect.objectContaining({ title: '明确受众与交付要求', content: '第一版完整指令', status: 'completed' }),
  ]);
  const revision = retryPolish('polish:result-contract', '补充验收要求');
  await vi.runAllTimersAsync();
  await expect(revision).resolves.toBe(true);
  expect(polish.mock.calls[1][0]).toEqual({ instruction: '原始草稿', thread_id: 't1', polish_content: '第一版完整指令', feedback: '补充验收要求' });
  expect(useRunStore.getState().sessions.t1.timelineItems).toEqual([
    expect.objectContaining({ title: '补充验收要求', content: '第二版完整指令', status: 'completed' }),
  ]);
  const nextRevision = retryPolish('polish:result-contract', '精简表达');
  await vi.runAllTimersAsync();
  await expect(nextRevision).resolves.toBe(true);
  expect(polish.mock.calls[2][0]).toEqual({ instruction: '原始草稿', thread_id: 't1', polish_content: '第二版完整指令', feedback: '精简表达' });
  expect(useComposerStore.getState().threadDrafts.t1).toBe('原始草稿');
});

it.each([undefined, '本轮新反馈'])('restores the saved round and rebases only explicit revision feedback: %s', async (feedback) => {
  const polish = vi.spyOn(polishApi, 'polish').mockResolvedValue({
    title: '润色完成', content: '完整结果', changed: true, prompt_version: 'test',
  });
  const request: PolishRequest = {
    instruction: '原始草稿', thread_id: 't1', polish_content: '本轮修改基础', feedback: '本轮已有反馈',
  };
  const record: CommandActivityRecord = {
    id: 'polish:recovery-contract', thread_id: 't1', project_id: 'p1', kind: 'polish', method: 'manual',
    status: feedback === undefined ? 'failed' : 'completed', phase: 2, previous_title: '',
    request: { ...request }, result: { content: '最近成功结果' }, created_at: 1, updated_at: 2,
  };
  const pending = retryRecordedCommand(record, feedback);
  await vi.runAllTimersAsync();
  await expect(pending).resolves.toBe(true);
  expect(polish.mock.calls[0][0]).toEqual(feedback === undefined ? request : {
    ...request, polish_content: '最近成功结果', feedback,
  });
  expect(record.request).toEqual(request);
});
