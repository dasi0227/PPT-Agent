import type { ResourceScope, TagDefinition } from '../api/types';
import { useTagStore } from '../stores/tagStore';

// API response fixtures only; application labels come from /tags.
const fixtures: Record<ResourceScope, [string, string][]> = {
  theme: [["minimal", "极简"], ["business", "商务"], ["technology", "科技"], ["cool", "清冷"], ["warm", "温暖"], ["other", "其它"]],
  component: [["card", "卡片"], ["chart", "统计图"], ["table", "表格"], ["list", "列表"], ["process", "流程"], ["metric", "指标"], ["other", "其它"]],
  skill: [["workflow", "工作流"], ["methodology", "方法论"], ["manual", "操作手册"], ["experience", "开发经验"], ["other", "其它"]],
  snippet: [["identity", "身份"], ["deliverable", "交付"], ["constraint", "约束"], ["git", "Git"], ["review", "审查"], ["other", "其它"]],
};

export function installTagDictionaryFixture() {
  const tags: Partial<Record<ResourceScope, TagDefinition[]>> = {};
  for (const scope of Object.keys(fixtures) as ResourceScope[]) {
    tags[scope] = fixtures[scope].map(([key, name], index) => ({
      id: `tag_${scope}_${key}`, scope, key, name, is_system: true,
      sort_order: (index + 1) * 10, created_at: 0, updated_at: 0,
    }));
  }
  useTagStore.setState({ tags, load: async () => undefined });
}
