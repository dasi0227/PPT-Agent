---
id: ARTIFACT-TARGET-BLUEPRINT-REFACTOR
title: Artifact Target、Deck Blueprint 与前后端协议重构
status: approved-for-planning
owner: shared
date: 2026-08-01
depends_on:
  - docs/v2/10-interaction-modes.md
  - docs/v2/30-agent-pipeline-v2.md
  - docs/v2/40-api-and-data-contracts.md
---

# Artifact Target、Deck Blueprint 与前后端协议重构

## 1. 结论

废弃把 `outline / generate / edit / command` 混放在同一个 `kind` 维度的设计。新的 Run 协议以三个正交维度表达任务：

1. `target.artifact`：用户正在处理哪一种领域制品，取值 `blueprint | presentation`。
2. `target.level`：作用范围，取值 `slide | deck`。
3. `interaction`：本次请求是执行变更还是只讨论，以及 Agent 何时允许向用户澄清。

核心矩阵固定为：

| artifact | level | 用户语义 | 后端职责 |
|---|---|---|---|
| `blueprint` | `deck` | 创建或调整整份 PPT 的目录、叙事结构、页面顺序 | 创建/修改 `deck.json` 与各页 `slide.json` |
| `blueprint` | `slide` | 调整指定页面的标题、核心句、内容和视觉意图 | 修改目标 `slide.json` |
| `presentation` | `deck` | 将蓝图物化为整套 HTML，或整体调整已生成 PPT | 设计总监、整套物化、全局修改、校验和交付 |
| `presentation` | `slide` | 生成或修改指定页面 HTML | 根据页面状态自动选择 materialize/revise/rebuild |

`generate`、`edit`、`render` 不再是用户协议里的顶层任务类型：

- `materialize`：内部操作，把 Blueprint 物化为 HTML。
- `revise`：内部操作，在现有 HTML 上执行可验证的小范围修改。
- `rebuild`：内部操作，根据最新 Blueprint 重建页面。
- `render`：HTTP 查询能力，只负责把已生成的 HTML 交给预览 Runtime。

## 2. 设计原则

### 2.1 名词描述领域对象，动词留给内部执行

前端和 API 暴露 `blueprint`、`presentation`、`slide`、`deck` 等稳定领域名词。物化、补丁、重建属于 Runner 的内部决策，不要求用户理解。

### 2.2 “当前页”是前端状态，不是后端作用域

`current` 仅表示前端当前选中的页面。提交 Run 前，前端必须解析成稳定的 `slide_id`。后端不再接受 `current`，也不以易漂移的 `page_index` 作为写入目标。

### 2.3 Repo 从 PPT Run 的作用域移除

资产仓库是独立 bounded context。MVP 中：

- 用户通过资产 UI 管理主题、布局、组件和图片。
- Blueprint/Presentation Agent 只能搜索、读取和挂载现有资产。
- 不通过 PPT Run 创建、修改或删除 Repo 资产。
- 未来若增加“资产管理 Agent”，使用独立的 Asset Run/API，不重新塞入 PPT 的 target level。

### 2.4 Talk 和 Ask 是交互策略

- `talk` 对应 `interaction.intent=consult`，Harness 只暴露读工具与控制工具，不产生制品变更。
- `ask` 不再是 kind，也不需要一个固定的 AskRunner。它对应 `interaction.clarification`：
  - `when_blocked`：默认，仅在关键不确定性阻塞正确执行时提问。
  - `before_apply`：规划完成后、首次写入前请求确认。
  - `never`：不提问，采用安全且最小的合理假设继续。

`needs_input` 继续作为 Runtime 事件存在，它是运行过程状态，不是任务种类。

## 3. 新领域模型

### 3.1 Blueprint 是逻辑聚合，不是一个巨大 JSON

文件布局：

```text
<project_work_dir>/
├── deck.json
├── design/
│   └── design-spec.json
├── slides/
│   └── <stable-slide-id>/
│       ├── slide.json
│       └── index.html
├── common/
│   ├── tokens.css
│   └── base.css
└── versions/
    ├── deck/
    ├── blueprint-slide-<slide-id>/
    ├── presentation-slide-<slide-id>/
    └── design/
```

权威边界：

- `deck.json`：PPT 目标、目录层级、页面顺序。
- `slide.json`：单页语义蓝图。
- `design-spec.json`：全局视觉规范。
- `index.html`：单页 Presentation 产物。

`deck.json` 不复制完整页面内容，避免与各页 `slide.json` 形成双重真相。

### 3.2 deck.json

```json
{
  "schema_version": "2.0",
  "revision": 3,
  "project_id": "project-id",
  "title": "AI 产品商业化方案",
  "goal": "推动管理层批准下一阶段商业化投入",
  "audience": "业务管理层",
  "language": "zh-CN",
  "core_thesis": "AI 产品已经进入规模化商业落地阶段",
  "narrative_arc": "机会出现 → 现状问题 → 产品方案 → 商业价值 → 决策请求",
  "sections": [
    {
      "id": "section-market",
      "number": "01",
      "title": "市场机会",
      "subsections": [
        {
          "id": "subsection-demand",
          "number": "1.1",
          "title": "需求正在爆发"
        }
      ]
    }
  ],
  "outline_order": [
    "slide-cover",
    "slide-demand",
    "slide-solution",
    "slide-closing"
  ],
  "created_at": 1785513600,
  "updated_at": 1785513600
}
```

约束：

- `schema_version` 必填，当前固定 `2.0`。
- `revision` 为单调递增整数，每次结构性修改递增。
- `sections[].id`、`subsections[].id` 在 deck 内唯一。
- `number` 是展示标签，不作为主键。
- `outline_order` 只包含稳定 slide id，且不得重复。
- 每个 slide id 必须存在对应 `slides/<id>/slide.json`。
- 删除 section/subsection 前必须确认没有页面引用，或在同一事务中迁移引用。

### 3.3 slide.json

MVP 使用精简语义模型，不建立页面元素 AST：

```json
{
  "schema_version": "2.0",
  "revision": 5,
  "id": "slide-demand",
  "section": "section-market",
  "subsection": "subsection-demand",
  "role": "evidence",
  "title": "企业 AI 预算正在快速增长",
  "key_message": "未来两年，AI 投入将从试验预算转向正式业务预算",
  "content": {
    "summary": "通过市场数据证明企业投入进入规模化阶段",
    "points": [
      "AI 预算连续两年增长",
      "采购主体从技术部门扩展到业务部门",
      "项目目标从技术验证转向经营结果"
    ]
  },
  "visual_intent": {
    "archetype": "data-story",
    "description": "左侧突出增长数字，右侧用简洁趋势图展示预算变化",
    "asset_queries": [
      "enterprise AI investment growth"
    ]
  },
  "speaker_notes": "",
  "created_at": 1785513600,
  "updated_at": 1785513600
}
```

字段约束：

- `id` 为稳定标识，创建后不可改变。
- `section` 必填；`subsection` 可空。
- `role` 允许：
  - `cover`
  - `agenda`
  - `section-divider`
  - `context`
  - `problem`
  - `insight`
  - `evidence`
  - `comparison`
  - `solution`
  - `process`
  - `case-study`
  - `summary`
  - `cta`
  - `closing`
- `title`、`key_message` 必填。核心句必须表达这一页唯一希望受众记住的结论。
- `content.summary` 必填。
- `content.points` 为 0–6 项；封面、章节页允许为空。
- `visual_intent.archetype` 必填，它表达信息构图类型，而不是固定模板。
- `visual_intent.description` 必填，允许 Agent 自由完成具体 HTML 设计。
- `asset_queries` 可空，用于搜索 Repo 中的现有资产。
- 不增加 `content.parts[]`、文本框坐标、元素树、低层布局 DSL。

### 3.4 design-spec.json

沿用现有 `design/design-spec.json`，但将内容目标从 `subject` 中剥离。目标、受众和核心命题以 `deck.json` 为权威；design spec 只描述视觉系统：

```json
{
  "schema_version": "2.0",
  "revision": 2,
  "palette": [],
  "typography": {},
  "layout_system": {
    "grid": "12-col",
    "rhythm": "宽留白与左对齐信息块",
    "density": "medium"
  },
  "signature": "贯穿全篇的链路脉冲 SVG",
  "motion": {
    "policy": "仅封面与关键数据使用强调动效"
  }
}
```

### 3.5 Presentation 与 Revision 追溯

每个 HTML 产物必须能够追溯生成输入：

```json
{
  "presentation_revision": 7,
  "source_deck_revision": 3,
  "source_blueprint_revision": 5,
  "source_design_revision": 2,
  "generated_by_run_id": "run-id",
  "generated_at": 1785513600
}
```

实现可选择：

- 写入 HTML `<meta>`；以及
- 在 SQLite `slides` 表保存当前 revision 字段。

推荐二者都保留：SQLite 用于高效状态查询，HTML meta 用于制品自描述。

页面同步状态由 revision 计算：

```text
not_materialized:
  index.html 不存在

fresh:
  HTML source_blueprint_revision == slide.json revision
  且 source_design_revision == design-spec revision

blueprint_stale:
  source_blueprint_revision < slide.json revision

design_stale:
  source_design_revision < design-spec revision

unknown:
  legacy HTML 没有 revision 元数据
```

废弃把 `outline_dirty` 作为唯一判断依据；迁移期可保留为兼容投影字段。

## 4. 新 Run 协议

### 4.1 CreateRun 请求

```json
{
  "target": {
    "artifact": "presentation",
    "level": "slide",
    "slide_id": "slide-demand"
  },
  "interaction": {
    "intent": "apply",
    "clarification": "when_blocked"
  },
  "instruction": "把这一页的数据结论做得更有冲击力",
  "options": {
    "language": "zh-CN",
    "theme_id": "",
    "desired_slide_count": 10
  }
}
```

类型：

```ts
type ArtifactTarget = "blueprint" | "presentation";
type TargetLevel = "slide" | "deck";
type InteractionIntent = "apply" | "consult";
type ClarificationPolicy = "when_blocked" | "before_apply" | "never";

interface RunTarget {
  artifact: ArtifactTarget;
  level: TargetLevel;
  slide_id?: string;
}

interface RunInteraction {
  intent: InteractionIntent;
  clarification: ClarificationPolicy;
}

interface CreateRunRequest {
  target: RunTarget;
  interaction: RunInteraction;
  instruction: string;
  options?: {
    language?: string;
    theme_id?: string;
    desired_slide_count?: number;
  };
}
```

验证规则：

- `target.level=slide` 时 `slide_id` 必填且必须属于当前 thread 的 project。
- `target.level=deck` 时禁止携带 `slide_id`。
- `interaction.intent=consult` 时 Harness 不得获得写工具。
- `clarification` 缺省为 `when_blocked`。
- `instruction` 必填且 trim 后非空。
- 不再接收 `page_index` 作为新协议写入目标。

### 4.2 Run 响应与事件

`run.started` 改为：

```json
{
  "run_id": "run-id",
  "target": {
    "artifact": "presentation",
    "level": "slide",
    "slide_id": "slide-demand"
  },
  "interaction": {
    "intent": "apply",
    "clarification": "when_blocked"
  },
  "user_input": "..."
}
```

现有 `thought` 事件在后续 Runtime 路线中改名为 `status.summary`。迁移期本次重构可先保留事件名，禁止扩散新的依赖。

`done.result` 最少包含：

```json
{
  "artifact": "presentation",
  "level": "slide",
  "affected_slide_ids": ["slide-demand"],
  "operation": "revise",
  "revisions": {
    "deck": 3,
    "design": 2,
    "slides": {
      "slide-demand": {
        "blueprint": 5,
        "presentation": 7
      }
    }
  },
  "warnings": []
}
```

### 4.3 斜杠命令

MVP 不再把斜杠命令当作另一套领域协议。保留的快捷输入只负责在前端选择 target：

| Legacy | 新语义 |
|---|---|
| `/current ...` | 当前选中 slide → `presentation/slide` |
| `/page N ...` | 前端把 N 解析为稳定 slide id → `presentation/slide` |
| `/overview ...` | `presentation/deck` |
| `/talk ...` | 保留当前 target，`interaction.intent=consult` |
| `/ask ...` | 保留当前 target，`clarification=before_apply` |
| `/repo ...` | 移除，提示使用资产库 UI |
| `/prompt ...` | 移出核心 Run 协议，作为前端“优化指令”辅助能力 |
| `/recap ...` | 独立只读查询能力，不作为 Artifact Run |

## 5. Runner 与 Harness 适配

### 5.1 RunnerResolver

用明确的组合注册替代 `RunService` 中按 kind/scope/command 的交叉分支：

```go
type TargetKey struct {
    Artifact model.Artifact
    Level    model.TargetLevel
}

type RunnerResolver interface {
    Resolve(spec model.WorkSpec) (run.Runner, error)
}
```

注册四类 Runner：

```text
blueprint/deck         -> BlueprintDeckRunner
blueprint/slide        -> BlueprintSlideRunner
presentation/deck      -> PresentationDeckRunner
presentation/slide     -> PresentationSlideRunner
```

`interaction.intent=consult` 不注册另一组 Runner；在 Harness Policy 层将工具裁剪为只读和 control。

### 5.2 BlueprintDeckRunner

职责：

- 首次创建时生成 `deck.json + slide.json[]`。
- 已存在时读取当前 Blueprint，执行目录、页面顺序与多页语义调整。
- 结构删除仍使用 `needs_input` 或 `before_apply` 策略。
- 修改 section/subsection 名称不逐页复制写入。
- 修改 slide 顺序只改 `deck.json.outline_order` 与数据库 position。
- 完成后发 `artifact{blueprint_deck}` 和受影响页面 id。

工具：

- `read_deck_blueprint`
- `submit_deck_blueprint`
- `patch_deck_metadata`
- `upsert_section`
- `delete_section`
- `add_slide_blueprint`
- `delete_slide_blueprint`
- `reorder_slides`
- `patch_slide_blueprint`
- `validate_blueprint`
- `finish`

### 5.3 BlueprintSlideRunner

职责：

- 锁定稳定 `slide_id`。
- 读取 `deck.json` 以校验 section/subsection 引用。
- 修改单页 `slide.json`。
- 不生成或修改 HTML。
- revision 递增后，页面状态自然变为 `blueprint_stale`。

工具：

- `read_slide_blueprint`
- `patch_slide_blueprint`
- `validate_slide_blueprint`
- `finish`

### 5.4 PresentationDeckRunner

统一替代当前整套 `generate + overview edit`：

1. 读取完整 Blueprint 与现有 Presentation 状态。
2. 若 design spec 不存在或用户要求重设计，运行 Design Director。
3. 生成计划。
4. 对 `not_materialized / blueprint_stale / design_stale` 页面决定重建。
5. 对纯全局 token 调整改 `design-spec/tokens`。
6. 对不能由 token 表达的全局结构调整执行 fan-out revise。
7. 浏览器级和静态级校验。
8. 结构化交付 revision 与 warnings。

内部操作由状态与计划决定：

- 首次整套：`materialize`
- Blueprint 大幅变化：`rebuild`
- 全局 token 变化：`revise-design`
- 页面结构性全局修改：`fanout-revise`
- 混合任务：允许一个 Run 中组合多种内部操作

### 5.5 PresentationSlideRunner

统一替代当前单页 `generate/edit`：

- HTML 不存在 → `materialize`。
- Blueprint revision 更新 → 默认 `rebuild`。
- 用户只要求局部视觉或文案调整 → 优先 `revise`。
- 修改与 Blueprint 冲突时：
  - 若用户明确只改成稿，允许 Presentation 偏离 Blueprint，并记录 divergence warning。
  - 若语义内容变化，先更新 Blueprint，再重建 Presentation。
- 写入后执行 validate，并记录 source revision。

### 5.6 工具门控

现有 `Tool.Scopes()` 迁移为能力声明：

```go
type Capability string

const (
    CapabilityReadBlueprint       Capability = "blueprint.read"
    CapabilityWriteBlueprint      Capability = "blueprint.write"
    CapabilityReadPresentation    Capability = "presentation.read"
    CapabilityWritePresentation   Capability = "presentation.write"
    CapabilityReadAssets          Capability = "assets.read"
    CapabilityMountAssets         Capability = "assets.mount"
    CapabilityControl             Capability = "control"
)
```

工具声明 `Capabilities()`，Policy 根据 target 与 interaction 计算允许集合。迁移期可保留 `Scopes()` 适配器，但新 Runner 不继续扩展旧 scope。

## 6. 后端完整适配

### 6.1 model

新增：

- `Artifact`
- `TargetLevel`
- `RunTarget`
- `InteractionIntent`
- `ClarificationPolicy`
- `RunInteraction`
- `WorkSpec`
- `DeckBlueprint`
- `Section`
- `Subsection`
- `SlideBlueprint`
- `VisualIntent`
- `ArtifactRevision`
- `MaterializationState`

`Run` 新增：

- `TargetArtifact`
- `TargetLevel`
- `TargetSlideID`
- `InteractionIntent`
- `ClarificationPolicy`
- `RequestPayload` 或规范化 `work_spec_json`

旧字段 `kind/scope/page_index/mode/command` 在迁移期只读兼容，最终删除。

### 6.2 migrations/store

新增迁移文件，不修改 `0001_init.sql`。

建议 `runs` 新增列：

```sql
target_artifact       TEXT
target_level          TEXT
target_slide_id       TEXT
interaction_intent    TEXT
clarification_policy  TEXT
work_spec_json        TEXT NOT NULL DEFAULT '{}'
```

`slides` 新增：

```sql
position                       INTEGER NOT NULL DEFAULT 0
blueprint_revision             INTEGER NOT NULL DEFAULT 0
presentation_revision          INTEGER NOT NULL DEFAULT 0
source_deck_revision           INTEGER NOT NULL DEFAULT 0
source_blueprint_revision      INTEGER NOT NULL DEFAULT 0
source_design_revision         INTEGER NOT NULL DEFAULT 0
```

`projects` 新增：

```sql
deck_path           TEXT NOT NULL DEFAULT 'deck.json'
deck_revision       INTEGER NOT NULL DEFAULT 0
design_revision     INTEGER NOT NULL DEFAULT 0
```

数据库约束与索引：

- `(project_id, position)` 唯一。
- `target_slide_id` 外键在 SQLite 迁移复杂时可由 Service 层校验。
- 新 Run 只写新字段；兼容请求同时写旧字段投影，直到迁移结束。

### 6.3 blueprint package

新建 `internal/blueprint`：

- Go 类型与 JSON 编解码。
- `deck.schema.json`、`slide.schema.json`。
- schema 编译缓存与同步守卫测试。
- 引用完整性验证。
- revision 递增规则。
- legacy slide-json → v2 slide blueprint 转换器。

不要继续把 v2 Blueprint 塞入现有 `agent/slidejson`，避免旧字段语义污染新模型。

### 6.4 service

新增：

- `BlueprintService`
- `PresentationStateService`
- `RunnerResolver`

Service 必须提供原子用例：

- 创建/替换整套 Blueprint。
- 修改单页 Blueprint。
- section/subsection CRUD。
- 页面添加、删除、重排。
- 计算 MaterializationState。
- 记录 Presentation source revision。

文件写与数据库写需要统一补偿策略。任何复合写失败时不得留下数据库与文件 revision 不一致。

### 6.5 HTTP API

保留 `POST /threads/:id/runs`，请求体切换为新协议。

增加 Blueprint 查询与手动编辑 API：

```text
GET   /projects/:id/blueprint
PATCH /projects/:id/blueprint
GET   /slides/:id/blueprint
PATCH /slides/:id/blueprint
```

也可继续复用 `/slides/:id`，但响应必须明确区分：

```json
{
  "meta": {},
  "blueprint": {},
  "materialization": {}
}
```

推荐使用显式 `/blueprint` 子资源，减少现有 Slide 元数据接口的兼容负担。

错误码：

- `INVALID_TARGET`
- `SLIDE_NOT_FOUND`
- `SLIDE_NOT_IN_PROJECT`
- `BLUEPRINT_INVALID`
- `BLUEPRINT_REFERENCE_BROKEN`
- `BLUEPRINT_REVISION_CONFLICT`
- `PRESENTATION_NOT_MATERIALIZED`
- `RUN_TARGET_UNSUPPORTED`

PATCH 请求应携带 `expected_revision`，冲突返回 409，避免手动编辑与 Agent 写入互相覆盖。

## 7. 前端完整适配

### 7.1 API types

删除新代码对 `RunKind/RunScope/RunMode/RunCommand` 的依赖，新增与后端一致的 target/interaction 类型。

`SlideContent` 替换为 `SlideBlueprint`；`Slide` 响应新增：

- `blueprint`
- `materialization.state`
- `materialization.revisions`

### 7.2 Composer

主模式由四个互斥按钮改为两个制品维度：

```text
蓝图 Blueprint | 成稿 Presentation
```

目标范围来自工作区上下文：

- 中央选中单页 → 默认 `slide`
- 用户在目录/蓝图总览视图 → 默认 `deck`
- “整套生成/整体调整”显式切为 `deck`

前端可展示 `当前页 / 整套`，但协议发送 `slide / deck`。

副交互：

- “讨论”开关 → `consult`
- “执行前询问”开关 → `before_apply`
- 默认 → `apply + when_blocked`

### 7.3 Blueprint UI

废弃把 Outline JSON 伪装成白色 PPT 页的 `OutlineCard`。

新增：

- `BlueprintWorkspace`
- `DeckBlueprintHeader`
- `SectionNavigator`
- `SlideBlueprintCard`
- `KeyMessageEditor`
- `VisualIntentEditor`
- `MaterializationBadge`

左侧导航按 section/subsection 分组：

```text
01 市场机会
  1.1 需求正在爆发
      02 企业 AI 预算正在增长
      03 采购主体正在变化
  1.2 竞争窗口
      04 竞争格局
```

单页 Blueprint 卡显示：

- 目录眉标，例如 `01 市场机会 · 1.1 需求正在爆发`
- role
- title
- key message
- content summary/points
- visual archetype/description
- materialization state

### 7.4 Preview 与状态刷新

现有安全 Preview Runtime 保持不变。

Run 完成后：

- blueprint Run 刷新 `deck.json + slide blueprints`；
- presentation Run 刷新 HTML 与 materialization state；
- 不再依赖 `outline_dirty` 推断；
- stale 页面在左侧导航、Blueprint 卡和 HTML 预览工具栏统一显示状态。

### 7.5 Store

新增：

- `blueprintStore`
- `targetStore` 或在 `composerStore` 中保存 `artifact/level`

`deckStore.currentPage` 继续用于 UI，但提交时解析为 `slide.id`。

跨 project 切换时：

- target artifact 可保留用户偏好；
- target slide 必须重新解析；
- 不允许把上一个 project 的 slide id 带到新 Run。

## 8. Legacy 兼容与迁移

### 8.1 请求适配

迁移窗口内后端接受旧请求，并转换为 WorkSpec：

| legacy | 新 WorkSpec |
|---|---|
| `outline + overview` | `blueprint/deck + apply` |
| `outline + current/page` | `blueprint/slide + apply`，当前页或 page index 解析为 slide id |
| `generate + current/page + page_index` | `presentation/slide + apply`，page index 解析为 slide id |
| `generate + overview` | `presentation/deck + apply` |
| `edit + current/page` | `presentation/slide + apply` |
| `edit + overview` | `presentation/deck + apply` |
| `mode=talk` 或 `command/talk` | 保留推断出的 target，`consult` |
| `mode=ask` 或 `command/ask` | 保留推断出的 target，`apply + before_apply` |
| `edit + repo` | 返回迁移提示，UI 不再发出 |

转换逻辑集中在 `LegacyRunAdapter`，禁止散落在 handler、service、runner。

### 8.2 数据迁移

对每个 legacy project：

1. 从 project 和 slides 元数据创建 `deck.json`。
2. 若无法推断章节，创建默认 section：
   - `id=section-main`
   - `number=01`
   - `title=正文`
3. 将旧 slide.json 转换为新 SlideBlueprint：
   - `title` 原样迁移。
   - `subtitle` 合并进 content summary 或 points。
   - `bullets` → `content.points`。
   - `content_intent` → `content.summary`。
   - `layout` → `visual_intent.archetype` 的映射。
   - `chart_intent` → visual description。
   - `notes` → `speaker_notes`。
4. 旧 HTML 标记为 `unknown` materialization state，首次 presentation 操作补 revision 元数据。
5. 迁移幂等：已有 schema_version=2.0 时跳过。

### 8.3 迁移阶段

#### Phase 1：领域模型和双读

- 新 schema、BlueprintService、Legacy converter。
- 现有 UI 仍工作。
- 新 API 可读 Blueprint。

#### Phase 2：新 Run 协议和 Resolver

- 新 target/interaction 请求。
- LegacyRunAdapter。
- 四类 Runner 接入。
- 新旧 SSE 客户端均可消费。

#### Phase 3：前端切换

- Blueprint UI。
- Composer target/interaction。
- stable slide id。
- materialization state。

#### Phase 4：删除旧路径

- 前端不再发送旧字段。
- 观察一个迁移窗口后删除 legacy runner routing。
- DB 旧字段最后删除，避免同一个 PR 同时做不可逆 schema 清理。

## 9. 测试与验收

### 9.1 后端

- JSON schema 正反例。
- section/subsection 引用完整性。
- revision 单调递增与 409 冲突。
- legacy 转换幂等。
- 四种 target 组合精确选择 Runner。
- consult 模式无写工具、无 artifact 写入。
- before_apply 只在首次写入前产生 needs_input。
- presentation/slide 自动选择 materialize/revise/rebuild。
- 页面重排只改变 position/outline_order，不改变 stable id。
- 文件与 DB 复合写失败补偿。
- SSE run.started/done 使用新契约。
- Legacy 请求映射兼容。

### 9.2 前端

- Composer 四种目标组合。
- 当前页解析为 stable slide id。
- project 切换不会复用旧 slide id。
- SectionNavigator 分组与重排。
- SlideBlueprintCard 字段编辑。
- revision conflict 提示与重新加载。
- materialization 状态展示。
- Blueprint Run 与 Presentation Run 完成后的定向刷新。
- 旧项目迁移后可正常打开。

### 9.3 端到端

1. 新项目 → `blueprint/deck` → 生成目录与单页蓝图。
2. 编辑 `blueprint/slide` → revision 增加，页面变 stale。
3. `presentation/deck` → 全套物化，所有页面 fresh。
4. 再改一页 Blueprint → 只有目标页 stale。
5. `presentation/slide` → 只重建目标页。
6. `presentation/deck + consult` → 只输出分析，无文件和 revision 变化。
7. 调整 section 标题 → 所有引用页面的眉标在下一次重建后统一变化。
8. 重排页面 → stable id、版本历史和 thread 引用不漂移。

## 10. 非目标

- 不设计通用页面元素 AST。
- 不做多人实时协同。
- 不在本阶段实现资产管理 Agent。
- 不引入通用 DAG 编排平台。
- 不在同一个提交中删除全部 legacy 字段。
- 不改变已完成的安全 Preview Runtime 边界。

## 11. Definition of Done

- 前后端不再以 `generate/edit` 决定用户语义。
- 新请求只使用 target/interaction。
- 四种核心组合全部有真实 Runner 与端到端测试。
- `current/page_index/overview/repo` 不再出现在新协议。
- 每个项目存在合法 `deck.json` 和 `design-spec.json`。
- 每页存在合法 v2 `slide.json`。
- Blueprint 与 Presentation revision 可追溯。
- Blueprint UI 不再伪装成白色幻灯片。
- Legacy 项目和请求在迁移窗口内继续可用。
- `go test ./...`、`go vet ./...`、前端 test/tsc/build 全绿。
