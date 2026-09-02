import { describe, expect, it } from 'vitest';
import type { Prompt } from '../../api/types';
import { findPromptTrigger, matchPrompts } from './promptMatching';

const prompts: Prompt[] = [
  { id: 'value', name: '写作 / Draft', desc: '撰写页面', value: '生成高管摘要', tags: ['deliverable'], disabled: false, created_at: 1, updated_at: 4 },
  { id: 'tag', name: '图表 / Chart', desc: '选择可视化', value: '选择图表', tags: ['deliverable'], disabled: false, created_at: 1, updated_at: 3 },
  { id: 'desc', name: '分析 / Analysis', desc: 'Executive summary helper', value: '分析内容', tags: ['review'], disabled: false, created_at: 1, updated_at: 2 },
  { id: 'name', name: '摘要模板 / Brief', desc: '提炼内容', value: '生成内容', tags: ['other'], disabled: false, created_at: 1, updated_at: 1 },
];

describe('prompt matching', () => {
  it('detects triggers only at the start, after ASCII space, or after a newline', () => {
    expect(findPromptTrigger('$sum', 4)).toEqual({ start: 0, end: 4, query: 'sum' });
    expect(findPromptTrigger('正文 ¥摘要', 6)).toEqual({ start: 3, end: 6, query: '摘要' });
    expect(findPromptTrigger('正文\n$draft', 9)).toEqual({ start: 3, end: 9, query: 'draft' });
    expect(findPromptTrigger('price$20', 8)).toBeNull();
    expect(findPromptTrigger('中文，$摘要', 6)).toBeNull();
  });

  it('matches non-prefix content and preserves field priority', () => {
    expect(matchPrompts(prompts, '摘要', []).map((prompt) => prompt.id)).toEqual(['name', 'value']);
    expect(matchPrompts(prompts, 'SUM', []).map((prompt) => prompt.id)).toEqual(['desc']);
    expect(matchPrompts(prompts, '交付', []).map((prompt) => prompt.id)).toEqual(['value', 'tag']);
  });

  it('uses at most five valid recent prompts for an empty query', () => {
    expect(matchPrompts(prompts, '', ['desc', 'missing', 'name']).map((prompt) => prompt.id)).toEqual(['desc', 'name']);
  });

  it('excludes disabled prompts from search and recent candidates', () => {
    const disabled = prompts.map((prompt) => prompt.id === 'desc' ? { ...prompt, disabled: true } : prompt);
    expect(matchPrompts(disabled, 'summary', []).map((prompt) => prompt.id)).toEqual([]);
    expect(matchPrompts(disabled, '', ['desc', 'name']).map((prompt) => prompt.id)).toEqual(['name']);
  });
});
