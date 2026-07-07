---
id: V2-CONTRACTS
title: v2 API 与数据契约变更
status: draft
owner: shared
depends_on: [V2-INDEX, API-SSE, API-REST, DATA-MODEL]
verifies: [V2-G1, V2-G4]
---

# v2 API 与数据契约变更

本文件汇总 v2 相对 v1 的**契约增量**，是前后端对齐的唯一差异清单。**原则：只增不改**——不破坏 v1 已有字段/事件语义，
只新增可选字段与新事件类型，保证向后兼容与断线续传。

## 1. REST 端点：无新增，仅前端使用方式变化

v2 **不新增 REST 端点**。所有能力已在 v1 路由中（见 `httpapi/router.go`）。变化仅在前端如何调用：

| 端点 | v1 前端 | v2 前端 |
|---|---|---|
| `POST /threads/{id}/runs` | 只发 `kind=generate` | 按交互模式发 `kind=outline/generate/edit/command`（见 [10-interaction-modes](10-interaction-modes.md#4-意图--run-字段映射矩阵权威)） |
| `POST /projects/{id}/threads` | 仅在无 thread 时隐式建一个 | 显式多 thread 管理（见 [20-multi-thread-windows](20-multi-thread-windows.md)） |
| `GET /threads/{id}/history` | 未使用 | 打开历史 thread 时恢复 timeline |
| `DELETE /threads/{id}` | 未使用 | 关闭标签的可选删除（二次确认） |

### 1.1 CreateRun 请求体（前端需补齐）

后端 `createRunBody`（`run_handler.go`）已接受全部字段；v2 前端**必须能发出**以下组合（v1 前端类型缺失）：

```jsonc
// 大纲生成（新建项目第一步）
{ "kind": "outline", "instruction": "给投资人讲 AI 产品", "brief": "", "slide_count": 8, "language": "zh" }

// 整套/单页生成
{ "kind": "generate", "scope": "current", "page_index": 2, "theme": "" }

// 单页编辑
{ "kind": "edit", "scope": "current", "page_index": 2, "instruction": "标题改大" }

// 全局编辑
{ "kind": "edit", "scope": "overview", "instruction": "主色改品牌蓝" }

// 仓库编辑
{ "kind": "edit", "scope": "repo", "instruction": "卡片圆角调大" }

// 只说不做 / 提问澄清
{ "kind": "command", "command": "talk", "mode": "talk", "instruction": "分析整体节奏" }
{ "kind": "command", "command": "ask",  "mode": "ask",  "instruction": "帮我改第3页" }
```

## 2. SSE 事件：新增 plan / plan.update

### 2.1 事件类型表增量

在 v1 事件表（`run.started/thought/tool_call/tool_result/progress/token/artifact/needs_input/info/done/error`）基础上新增：

| event | data 字段 | 终态 | 说明 |
|---|---|---|---|
| `plan` | `{ id, title, steps:[{id,title,status,detail?}] }` | 否 | PLAN 阶段一次性发出完整步骤清单 |
| `plan.update` | `{ id, step_id, status, detail? }` | 否 | 单个 step 状态增量更新 |

- `status` 枚举：`pending | in_progress | completed | failed | skipped`。
- 后端 `model/event.go` 新增 `EventPlan="plan"`、`EventPlanUpdate="plan.update"`；二者 `Terminal()=false`。
- 持久化于 `run_events`，参与 `Last-Event-ID` 续传（与其它事件一致）。

### 2.2 progress.stage 语义扩展

v1 `stage` 允许 `turn`/`page`。v2 新增：

| stage | 语义 | current / total |
|---|---|---|
| `design` | 设计总监阶段 | 通常 1/1（阶段性存在即可） |
| `validate` | 全局校验阶段 | `current`=已校验页，`total`=总页数 |

> 客户端 MUST 按 `stage` 分派展示，禁止假设固定枚举（沿用 v1 API-SSE 约定）。

### 2.3 artifact.artifact_type 扩展

v1 `artifact_type ∈ {slide_html, design, asset, version}`。v2 新增 **`design_spec`**：

```text
event: artifact
data: { "artifact_type": "design_spec", "ref": "design/design-spec.json" }
```

### 2.4 done.result 结构化

v1 `done.result` 为 `{summary}`。v2 整套生成的 result 升级为结构化对象（见 [30-agent-pipeline-v2](30-agent-pipeline-v2.md#82-stage-5-结构化交付)）。
**兼容性**：其它 kind（edit/outline/command）的 `done.result` 仍可为 `{summary}`；前端 FinalResultCard MUST 兼容两种形态（有结构字段则结构化展示，否则回退 summary 文本）。

### 2.5 序列约束增量

| ID | 约束 |
|---|---|
| `V2-SSE-001` | `plan` 在一次 Run 内至多一次，且 MUST 在其 `plan.update` 之前 |
| `V2-SSE-002` | `plan.update.step_id` MUST 命中同 Run 已发 `plan` 的某 step |
| `V2-SSE-003` | `/talk` 模式 MUST NOT 发 `plan`/`plan.update`/`artifact`（沿用 API-SSE-004 精神） |
| `V2-SSE-004` | 新增事件不改变"终态恰好一个 done/error"约束（API-SSE-002 不变） |

### 2.6 示例流（整套生成 v2）

```text
id:1  event:run.started   data:{"run_id":"r1","kind":"generate","scope":"overview","mode":"normal"}
id:2  event:progress      data:{"stage":"design","current":1,"total":1}
id:3  event:tool_call     data:{"tool":"submit_design_spec","args":{...},"call_id":"c1"}
id:4  event:tool_result   data:{"call_id":"c1","ok":true,"observation":"design_spec 已落盘"}
id:5  event:artifact      data:{"artifact_type":"design_spec","ref":"design/design-spec.json"}
id:6  event:plan          data:{"id":"plan_r1","title":"构建 8 页","steps":[{"id":"s0",...},...]}
id:7  event:plan.update   data:{"id":"plan_r1","step_id":"s0","status":"in_progress"}
id:8  event:progress      data:{"stage":"page","current":1,"total":8}
id:9  event:artifact      data:{"artifact_type":"slide_html","ref":"slides/000/index.html","page_index":0}
id:10 event:plan.update   data:{"id":"plan_r1","step_id":"s0","status":"completed"}
...
id:N  event:done          data:{"result":{"project_id":"p1","slide_count":8,"design_spec_ref":"...","signature":"链路脉冲","warnings":[]}}
```

## 3. 数据模型 / 文件布局增量

### 3.1 新文件产物

| 路径（project work_dir 内） | 内容 | 版本化 |
|---|---|---|
| `design/design-spec.json` | 设计语言（palette/type/layout/signature/motion） | 产 `design` 版本（复用 v1 versioning） |

> 公共层 `common/tokens.css` / `common/base.css` 布局不变；v2 只是让 tokens.css 可由 design_spec 生成。

### 3.2 SQLite

- **无 schema 变更**：`runs.scope` 的 CHECK 约束（`current/page/overview/repo`）不变；plan 事件走 `run_events` 无需新表。
- design_spec 作为文件产物 + `run_events`/version 记录，不新增业务表。

## 4. 前端类型对齐（TS）

`frontend/src/api/types.ts` 需要的增量（详见 [50-frontend-architecture-v2](50-frontend-architecture-v2.md)）：

```ts
// kind 补齐（v1 缺 outline/command）
export type RunKind = 'outline' | 'generate' | 'edit' | 'command';

export interface RunPayload {
  kind: RunKind;
  instruction: string;
  scope?: RunScope;                 // outline 首次可省
  mode?: 'normal' | 'talk' | 'ask';
  page_index?: number;
  command?: 'talk' | 'ask' | 'prompt' | 'recap';
  brief?: string; slide_count?: number; language?: string;  // outline
  theme?: string;                   // generate
}

// SSE 事件补齐
export type SSEEventName =
  | 'run.started' | 'thought' | 'tool_call' | 'tool_result' | 'progress'
  | 'token' | 'artifact' | 'needs_input' | 'info' | 'done' | 'error'
  | 'plan' | 'plan.update';

export type PlanStepStatus = 'pending' | 'in_progress' | 'completed' | 'failed' | 'skipped';
export interface PlanStep { id: string; title: string; status: PlanStepStatus; detail?: string }
```

## 5. 兼容性与迁移

| 关注点 | 结论 |
|---|---|
| v1 前端连 v2 后端 | 兼容：v1 前端忽略未知 `plan` 事件即可（但仍撞硬编码 generate 问题——需前端修复） |
| v2 前端连 v1 后端 | 兼容降级：无 `plan`/`design_spec` 事件时 PlanCard 保持空态（m7 已预留） |
| Last-Event-ID 续传 | 新事件同样持久化，续传无差异 |
| 已有 project 数据 | 无迁移：design-spec.json 缺失时流水线首次生成补齐 |

## 6. 验收标准（Given-When-Then）

- **AC-V2-CTR-001**（`V2-G1`）
  - GIVEN v2 前端
  - WHEN 新建项目并生成大纲
  - THEN 发出 `POST /threads/{id}/runs` body `kind=outline`，返回 201 + `events_url`

- **AC-V2-CTR-002**（`V2-G4`）
  - GIVEN 后端新增 plan 事件
  - WHEN 读取整套生成事件流
  - THEN 出现恰好一个 `plan` 且其后 `plan.update` 的 step_id 均命中该 plan

- **AC-V2-CTR-003**
  - GIVEN 断线后带 `Last-Event-ID` 重连
  - WHEN 续订
  - THEN `plan`/`plan.update`/`artifact{design_spec}` 均可被续发，无重复

## 7. 校验方式

```bash
go test ./internal/run -run 'TestSSESequence|TestSSEResume|TestPlanEventPersisted'
go test ./internal/httpapi -run 'TestE2EOutlineRun|TestE2EPipelineEvents'
cd frontend && pnpm tsc --noEmit   # 类型对齐
```

## 8. 依赖

- [API-SSE](../v1/40-api/sse-events.md)、[API-REST](../v1/40-api/rest-endpoints.md)、[DATA-MODEL](../v1/30-data-model/data-model.md)、[30-agent-pipeline-v2](30-agent-pipeline-v2.md)
