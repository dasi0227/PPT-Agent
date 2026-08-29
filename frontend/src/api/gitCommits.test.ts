import { describe, expect, test } from 'vitest';
import { parseGitCommitEvent } from './gitCommits';

const base = {
  schema_version: 1,
  operation_id: 'gco_1',
  project_id: 'p1',
  thread_id: 't1',
  occurred_at: '2026-08-30T06:29:00.000Z',
};

describe('Git commit SSE parser', () => {
  test('accepts progress and completed events', () => {
    expect(parseGitCommitEvent('git.commit.progress', JSON.stringify({
      ...base,
      phase: 'analyzing',
    }), '2')).toMatchObject({
      id: '2',
      event: 'git.commit.progress',
      data: { phase: 'analyzing' },
    });

    expect(parseGitCommitEvent('git.commit.completed', JSON.stringify({
      ...base,
      commit: {
        title: 'fix: align chart labels',
        items: ['Align labels with data points'],
        branch: 'main',
        hash: '8af42d9',
        files_changed: 3,
        insertions: 46,
        deletions: 18,
        committed_at: '2026-08-30T06:29:08.000Z',
      },
    }))).not.toBeNull();
  });

  test('rejects malformed and unknown events', () => {
    expect(parseGitCommitEvent('git.commit.progress', JSON.stringify({
      ...base,
      phase: 'thinking',
    }))).toBeNull();
    expect(parseGitCommitEvent('run.progress', JSON.stringify(base))).toBeNull();
    expect(parseGitCommitEvent('git.commit.failed', '{')).toBeNull();
  });
});
