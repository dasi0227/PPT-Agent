// 草稿态（软创建）工具：草稿 project/thread 用 `draft_` 前缀的临时 id，
// 不落库、不建目录；产生首个真实副作用时才 flush 拿真实 id（方案 A）。
// 严禁把 draft_* 拼进任何 REST 路径或 SSE 订阅。
export const DRAFT_PREFIX = 'draft_';

export function newDraftId(): string {
  return DRAFT_PREFIX + crypto.randomUUID();
}

export function isDraftId(id: string | null | undefined): boolean {
  return typeof id === 'string' && id.startsWith(DRAFT_PREFIX);
}
