import type { Prompt, PromptTag } from '../../api/types';

export const promptTagLabels: Record<PromptTag, string> = {
  identity: '身份',
  deliverable: '交付',
  constraint: '约束',
  git: 'Git',
  review: '审查',
  other: '其它',
};

export const promptTagOrder = Object.keys(promptTagLabels) as PromptTag[];

export interface PromptTrigger {
  start: number;
  end: number;
  query: string;
}

export function findPromptTrigger(text: string, caret: number): PromptTrigger | null {
  if (caret < 0 || caret > text.length) return null;
  const before = text.slice(0, caret);
  const match = before.match(/(?:^|[ \n])([$¥])([^ \n$¥]*)$/);
  if (!match) return null;
  return {
    start: caret - match[1].length - match[2].length,
    end: caret,
    query: match[2],
  };
}

function includes(value: string, query: string): boolean {
  return value.toLocaleLowerCase().includes(query.toLocaleLowerCase());
}

export function matchPrompts(prompts: Prompt[], query: string, recentIds: string[]): Prompt[] {
  const enabledPrompts = prompts.filter((prompt) => !prompt.disabled);
  if (!query) {
    const byId = new Map(enabledPrompts.map((prompt) => [prompt.id, prompt]));
    return recentIds.flatMap((id) => {
      const prompt = byId.get(id);
      return prompt ? [prompt] : [];
    }).slice(0, 5);
  }
  const recentOrder = new Map(recentIds.map((id, index) => [id, index]));
  return enabledPrompts
    .map((prompt) => {
      const rank = includes(prompt.key_zh, query)
        ? 0
        : includes(prompt.key_en, query)
          ? 1
          : prompt.tags.some((tag) => includes(tag, query) || includes(promptTagLabels[tag], query))
            ? 2
            : includes(prompt.value, query)
              ? 3
              : 4;
      return { prompt, rank };
    })
    .filter(({ rank }) => rank < 4)
    .sort((left, right) => (
      left.rank - right.rank
      || (recentOrder.get(left.prompt.id) ?? Number.MAX_SAFE_INTEGER)
        - (recentOrder.get(right.prompt.id) ?? Number.MAX_SAFE_INTEGER)
      || right.prompt.updated_at - left.prompt.updated_at
      || left.prompt.key_zh.localeCompare(right.prompt.key_zh, 'zh-CN')
    ))
    .slice(0, 8)
    .map(({ prompt }) => prompt);
}
