# 仓库统一标签与启停设计

**日期：** 2026-09-02  
**状态：** 已确认  
**覆盖范围：** Theme / Component / Skill / Prompt 仓库的标签、筛选与启停

## 1. 决策摘要

- Theme、Component、Skill、Prompt 各自使用独立的标签枚举，不共享标签命名空间。
- 系统预设标签不可重命名、不可删除，但对象可以添加或移除这些标签。
- 后续允许创建、重命名和删除用户自定义标签；该能力必须使用 SQLite 管理标签定义与对象关联。
- `resource_id` 表示被打标签对象的稳定 ID，不表示用户 ID，也不依赖登录态。
- Component、Skill、Prompt 支持启停；Theme 不支持启停。
- Skill 左侧筛选从启停状态改为标签筛选，启停开关与禁用视觉状态继续保留。
- Component 使用与 Skill 相同的启停交互和 Agent 可见性约束。
- Prompt 禁用后仍可在仓库查看和编辑，但不进入 Composer 快捷候选。

## 2. 系统预设标签

### 2.1 Theme

| Key | 名称 | 预设主题 |
|---|---|---|
| `minimal` | 极简 | Swiss Modern |
| `business` | 商务 | Corporate Clean |
| `technology` | 科技 | Blueprint |
| `cool` | 清冷 | Tokyo Night |
| `warm` | 温暖 | Xiaohongshu White |
| `other` | 其它 | 未归入以上风格的主题 |

现有其他主题不删除，默认归入“其它”。五个具名主题可以继续调整色彩、字体、边界和装饰细节，使视觉方向更贴合对应标签，但不得破坏 Theme token 契约。

### 2.2 Component

| Key | 名称 |
|---|---|
| `card` | 卡片 |
| `chart` | 统计图 |
| `table` | 表格 |
| `list` | 列表 |
| `process` | 流程 |
| `metric` | 指标 |
| `other` | 其它 |

### 2.3 Skill

| Key | 名称 |
|---|---|
| `workflow` | 工作流 |
| `methodology` | 方法论 |
| `manual` | 操作手册 |
| `experience` | 开发经验 |
| `other` | 其它 |

### 2.4 Prompt

| Key | 名称 |
|---|---|
| `identity` | 身份 |
| `deliverable` | 交付 |
| `constraint` | 约束 |
| `git` | Git |
| `review` | 审查 |
| `other` | 其它 |

## 3. 预置资源标签

### Theme

- `swiss-modern`：极简
- `corporate-clean`：商务
- `blueprint`：科技
- `tokyo-night`：清冷
- `xiaohongshu-white`：温暖
- `bold-signal`、`editorial-serif`、`warm-pastel`：其它

### Component

- `feature-card`：卡片
- `svg-bar`：统计图
- `kv-list`：列表
- `stat-badge`：指标
- `quote-block`：其它

### Skill

- `story-architect`：方法论
- `executive-summary`：工作流
- `visual-hierarchy-review`：操作手册

### Prompt

- 叙事大纲、单页撰写、高管摘要、精简改写、数据洞察、图表建议、演讲备注：交付
- 深度分析、视觉审查、内容审查：审查

## 4. 筛选与启停

- 四个模块左侧顶部统一使用标签筛选，支持“全部 + 当前模块标签”。
- 标签筛选和详情标签保持单行横向滚动，并隐藏滚动条视觉。
- 标签筛选不隐式排除禁用对象；禁用状态通过目录项和详情页状态展示。
- Component、Skill、Prompt 详情页提供启停开关。
- 禁用 Component 不进入 Agent 组件目录，`load_component` 拒绝加载。
- 禁用 Skill 不进入固定选择列表，`load_skill` 拒绝加载。
- 禁用 Prompt 不进入 Composer 快捷候选和最近使用候选。
- Theme 仅使用标签筛选，不提供启停状态。
- 详情标题右侧操作统一按“编辑、删除、外部查看”排列；无文件入口的 Prompt 仅显示编辑和删除。
- 编辑使用 Dialog：Theme、Component、Skill 支持修改名称、描述和标签，Prompt 支持修改名称、描述、正文和标签。

## 5. SQLite 存储

标签定义、资源关联与启停状态统一存储在 SQLite：

```text
tags
- id
- scope              theme | component | skill | prompt
- key
- name
- normalized_name
- is_system
- sort_order
- created_at
- updated_at

resource_tags
- resource_type
- resource_id
- tag_id

resource_states
- resource_type
- resource_id
- disabled
- updated_at
```

- `resource_id` 是资源自身 ID，例如 `swiss-modern`、`stat-badge`、`story-architect` 或 Prompt ID。
- 系统标签使用 `is_system = 1`，更新名称和删除接口必须拒绝操作。
- 用户标签使用 `is_system = 0`，允许新增、重命名和删除。
- 删除用户标签时，在同一事务内级联删除 `resource_tags` 关系。
- 标签重命名只修改标签记录，对象关联通过稳定 `tag_id` 保持不变。
- 不增加 `user_id`、`owner_id` 或账户作用域；当前数据库就是本地仓库唯一作用域。
- `registry.json`、Prompt `tags_json` 与 Prompt 表内 `disabled` 字段全部删除，不保留双重事实来源。
- Theme、Component、Skill 的名称和描述只存放在资源文件 Frontmatter，SQLite 不保存覆盖值。
- Theme 使用 `theme.css` 顶部的 CSS 注释 Frontmatter，不再使用 `manifest.json`。
- Component 使用 `index.html` 顶部的 HTML 注释 Frontmatter，不再使用 JSON `<script id="meta">`。
- Skill 继续使用 `SKILL.md` 顶部的 Markdown Frontmatter。
- 编辑名称或描述时原子改写对应资源文件，正文内容保持不变；标签仍写入 `resource_tags`。
- Prompt 的 `name`、`desc` 和 `value` 继续直接存储在 `prompts` 表；`tags` 由统一关系表存储。

## 6. 当前阶段落地

- 本轮切换系统预设枚举、标签筛选和 Component / Prompt 启停。
- Theme、Component、Skill、Prompt 的标签关联统一写入 `resource_tags`。
- Component、Skill、Prompt 的启停状态统一写入 `resource_states`。
- Theme、Component、Skill 的名称和描述统一写入各自资源文件 Frontmatter。
- 系统标签由 migration 初始化并标记为 `is_system = 1`，不提供重命名或删除入口。
- 后续自定义标签 CRUD 直接复用 `tags` 表，不再调整资源存储协议。
- 不为旧标签值保留兼容分支。
