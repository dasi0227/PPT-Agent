import { describe, expect, it } from 'vitest';
import type { ComponentReference, Prompt } from '../../api/types';
import {
  findComponentTrigger,
  findPageTrigger,
  findPromptTrigger,
  matchComponents,
  matchPages,
  matchPrompts,
  pageDisplayName,
  type PageMentionCandidate,
} from './promptMatching';

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
    expect(matchPrompts(prompts, '摘要').map((prompt) => prompt.id)).toEqual(['name', 'value']);
    expect(matchPrompts(prompts, 'SUM').map((prompt) => prompt.id)).toEqual(['desc']);
    expect(matchPrompts(prompts, '交付').map((prompt) => prompt.id)).toEqual(['value', 'tag']);
  });

  it('shows all enabled prompts in stable order for an empty query', () => {
    expect(matchPrompts(prompts, '').map((prompt) => prompt.id)).toEqual(['value', 'tag', 'desc', 'name']);
  });

  it('excludes disabled prompts from search and empty-query candidates', () => {
    const disabled = prompts.map((prompt) => prompt.id === 'desc' ? { ...prompt, disabled: true } : prompt);
    expect(matchPrompts(disabled, 'summary').map((prompt) => prompt.id)).toEqual([]);
    expect(matchPrompts(disabled, '').map((prompt) => prompt.id)).toEqual(['value', 'tag', 'name']);
  });
});

describe('component matching', () => {
  const components: ComponentReference[] = [
    { id: 'feature-card', name: '能力卡片', description: '展示核心能力', tags: ['card'], disabled: false, open_url: '' },
    { id: 'trend-chart', name: '趋势图', description: '年度增长', tags: ['chart'], disabled: false, open_url: '' },
    { id: 'disabled-card', name: '禁用卡片', description: '不可用', tags: ['card'], disabled: true, open_url: '' },
  ];

  it('detects # only at the start, after an ASCII space, or after a newline', () => {
    expect(findComponentTrigger('#能力', 3)).toEqual({ start: 0, end: 3, query: '能力' });
    expect(findComponentTrigger('参考 #card', 8)).toEqual({ start: 3, end: 8, query: 'card' });
    expect(findComponentTrigger('正文\n#趋势', 6)).toEqual({ start: 3, end: 6, query: '趋势' });
    expect(findComponentTrigger('word#card', 9)).toBeNull();
    expect(findComponentTrigger('中文，#能力', 6)).toBeNull();
    expect(findComponentTrigger('$能力', 3)).toBeNull();
    expect(findPromptTrigger('#能力', 3)).toBeNull();
  });

  it('matches name, directory id, description and tags while excluding disabled components', () => {
    expect(matchComponents(components, '能力').map((component) => component.id)).toEqual(['feature-card']);
    expect(matchComponents(components, 'FEATURE').map((component) => component.id)).toEqual(['feature-card']);
    expect(matchComponents(components, '年度').map((component) => component.id)).toEqual(['trend-chart']);
    expect(matchComponents(components, 'chart').map((component) => component.id)).toEqual(['trend-chart']);
    expect(matchComponents(components, '').map((component) => component.id)).toEqual(['feature-card', 'trend-chart']);
    expect(matchComponents(components, '禁用')).toEqual([]);
  });
});

describe('page matching', () => {
  const pages: PageMentionCandidate[] = [
    { slideId: 'sli_a', ordinal: 1, title: '封面', keyMessage: '开场', specState: 'ready', htmlState: 'fresh' },
    { slideId: 'sli_b', ordinal: 2, title: '融资历程', keyMessage: '融资三轮', specState: 'ready', htmlState: 'spec_stale' },
    { slideId: 'sli_c', ordinal: 3, title: '', keyMessage: '', specState: 'pending', htmlState: 'not_materialized' },
  ];

  it('detects @ independently at valid boundaries', () => {
    expect(findPageTrigger('@融资', 3)).toEqual({ start: 0, end: 3, query: '融资' });
    expect(findPageTrigger('修改 @3', 5)).toEqual({ start: 3, end: 5, query: '3' });
    expect(findPageTrigger('正文\n@Page', 8)).toEqual({ start: 3, end: 8, query: 'Page' });
    expect(findPageTrigger('mail@example', 12)).toBeNull();
    expect(findPageTrigger('中文，@融资', 6)).toBeNull();
    expect(findPromptTrigger('@融资', 3)).toBeNull();
    expect(findComponentTrigger('@融资', 3)).toBeNull();
  });

  it('matches title and ordinal without searching key messages', () => {
    expect(matchPages(pages, '融资').map((page) => page.slideId)).toEqual(['sli_b']);
    expect(matchPages(pages, '2').map((page) => page.slideId)).toEqual(['sli_b']);
    expect(matchPages(pages, 'PAGE 3').map((page) => page.slideId)).toEqual(['sli_c']);
    expect(matchPages(pages, '三轮')).toEqual([]);
    expect(matchPages(pages, '').map((page) => page.slideId)).toEqual(['sli_a', 'sli_b', 'sli_c']);
  });

  it('formats titled and untitled pages without a placeholder', () => {
    expect(pageDisplayName(pages[1])).toBe('Page 2 · 融资历程');
    expect(pageDisplayName(pages[2])).toBe('Page 3');
  });
});
