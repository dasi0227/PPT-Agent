import { describe, expect, it } from 'vitest';
import type { Prompt } from '../../api/types';
import { findPromptTrigger, matchPrompts } from './promptMatching';

const prompts: Prompt[] = [
  { id: 'value', key_zh: '写作', key_en: 'draft', value: '生成高管摘要', tags: ['deliverable'], disabled: false, created_at: 1, updated_at: 4 },
  { id: 'tag', key_zh: '图表', key_en: 'chart', value: '选择图表', tags: ['deliverable'], disabled: false, created_at: 1, updated_at: 3 },
  { id: 'en', key_zh: '分析', key_en: 'executive-summary', value: '分析内容', tags: ['review'], disabled: false, created_at: 1, updated_at: 2 },
  { id: 'zh', key_zh: '摘要模板', key_en: 'brief', value: '提炼内容', tags: ['other'], disabled: false, created_at: 1, updated_at: 1 },
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
    expect(matchPrompts(prompts, '摘要', []).map((prompt) => prompt.id)).toEqual(['zh', 'value']);
    expect(matchPrompts(prompts, 'SUM', []).map((prompt) => prompt.id)).toEqual(['en']);
    expect(matchPrompts(prompts, '交付', []).map((prompt) => prompt.id)).toEqual(['value', 'tag']);
  });

  it('uses at most five valid recent prompts for an empty query', () => {
    expect(matchPrompts(prompts, '', ['en', 'missing', 'zh']).map((prompt) => prompt.id)).toEqual(['en', 'zh']);
  });

  it('excludes disabled prompts from search and recent candidates', () => {
    const disabled = prompts.map((prompt) => prompt.id === 'en' ? { ...prompt, disabled: true } : prompt);
    expect(matchPrompts(disabled, 'summary', []).map((prompt) => prompt.id)).toEqual([]);
    expect(matchPrompts(disabled, '', ['en', 'zh']).map((prompt) => prompt.id)).toEqual(['zh']);
  });
});
