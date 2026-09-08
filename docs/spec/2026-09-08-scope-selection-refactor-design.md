# 多页任务范围选择与运行时扩权设计

**日期：** 2026-09-08

**状态：** 核心需求、Q1–Q15 与范围选择视觉方案已确认，可进入实现

**范围：** Composer 范围选择、稳定页面集合、Runtime 写权限、运行中扩权、任务状态、持久化、SSE、恢复、测试与验收

**视觉原型：** [`docs/demo/2026-09-08-scope-selector-demo.html`](../demo/2026-09-08-scope-selector-demo.html)

**关联设计：** [`docs/spec/2026-09-02-page-mention-design.md`](./2026-09-02-page-mention-design.md)、[`TODO.md`](../../TODO.md)

---

## 一、背景与问题

当前 `RunScope` 只能表达：

```text
slide / deck × spec / ppt
```

这套模型存在四个直接问题：

1. 只能选单页或整份，无法准确表达多页和章节范围。
2. `deck` 为 Agent 开放了整份演示文稿的写权限，用户点名几页时仍缺少真正的写入边界。
3. `ppt` 的名称和实际能力不清晰，无法直接表达“只改 HTML”或“同时改 Spec + HTML”。
4. Run 启动后缺少由 Agent 发起、由用户批准的范围扩展机制。

此前的 `@page` 设计只把页面作为只读上下文指针，不承担授权职责。本次设计引入正式的多页授权范围后，两者必须继续分工：

- `@page`：告诉 Agent “请参考这些页面”，不自动授予写权限。
- `RunScope`：告诉 Runtime “本次 Run 最多允许写哪些页面和对象”。

## 二、目标与非目标

### 2.1 目标

- 支持当前页、全部页、自选页、自选章四种页面来源。
- 支持设计稿、幻灯片、演示文稿、全局资源四种对象权限。
- 所有页面范围在 Run 创建时解析为稳定的 `slide_id[]`。
- Runtime 允许读取范围外内容，但拒绝任何范围外写入。
- Run 中 Agent 可调用 `request_privilege` 申请增加权限，用户可批准、拒绝或调整。
- 批量工作状态、恢复与完成判断不再把“授权范围”误当成“必须修改的清单”。
- 范围选择 UI 按最终原型收敛，保持轻量，不显示解释性灰字和多余操作按钮。

### 2.2 非目标

- 不修改 `@page` 的页面引用协议；它仍是只读上下文指针。
- 不支持“第 3 页只改 Spec、第 5 页只改 HTML”这类逐页差异化授权。
- 不支持同一 Run 内主动缩小已有权限。
- 第一版不提供独立的“运行中手动编辑范围”入口；运行中范围变化由扩权审批卡触发。
- 不在本次设计中改变空项目创建流程和默认 Run 范围；它们保持现有产品行为，另行设计。
- 不为旧 `RunScope`、旧数据库结构或历史开发数据提供兼容、双读、双写和迁移逻辑。

## 三、核心产品语义

### 3.1 范围是写权限，不是任务清单

`RunScope` 只回答一个问题：

> 这个 Run 最多可以修改哪些内容？

选中 12 页不代表 12 页都必须被修改。Agent 可以读取范围外内容作为参考，也可以只修改范围内的一部分页面。完成判断不得因为某个范围内页面没有被写入而阻塞。

需要批量进度和断点续做时，使用独立的 `WorkLedger`；只有 Agent 在计划中明确承诺处理的页面才进入工作清单。两套状态不能互相替代：

```text
RunScope      = 能不能写
WorkLedger    = 是否计划处理、处理到哪一步
@mentioned    = 希望 Agent 参考什么
```

### 3.2 页面维度

| UI 名称 | `selection.kind` | 解析规则 |
| --- | --- | --- |
| 当前页 | `current_page` | 提交时解析为当前稳定 `slide_id` |
| 全部页 | `all_pages` | 提交时快照当前 outline 内的全部 `slide_id` |
| 自选页 | `custom_pages` | 使用用户勾选的稳定 `slide_id[]` |
| 自选章 | `custom_sections` | 使用 `section_id[]` 展开并快照为稳定 `slide_id[]` |

“自选章”只对应 outline 的 section，不再提供 subsection / “自选节”入口。

页码和标题只用于展示。授权、恢复、审计和工具校验一律使用稳定 ID；重排或改名不会改变已授权对象。

### 3.3 对象维度

| UI 名称 | `object` | 可写内容 |
| --- | --- | --- |
| 设计稿 | `spec` | 范围内页面的 Spec |
| 幻灯片 | `html` | 范围内页面的 HTML |
| 演示文稿 | `presentation` | 范围内页面的 Spec + HTML |
| 全局资源 | `global` | manifest、outline、design、全部页面 Spec、全部页面 HTML，以及结构性操作 |

选择“全局资源”时：

- 页面维度强制切为“全部页”。
- 其他页面选项置为不可用。
- `slide_ids` 仍保存选择当下的全量页面快照，用于展示和审计。
- 授权判断以 `global` 为最高权限，不受快照集合限制。

### 3.4 笛卡尔积规则

页面集合与对象权限直接组成笛卡尔积：

```text
有效写权限 = slide_ids × object capabilities
```

例如：页面 `{3, 5, 8}` 与“演示文稿”表示这三页的 Spec 和 HTML 都可写。

第一版不引入 `grants[]`，也不表达逐页不同对象权限。这样做会产生可预期的扩权副作用：若当前为 `{3,5} × HTML`，Agent 同时申请“第 8 页”和“Spec”，批准后的完整范围是 `{3,5,8} × (Spec + HTML)`，而不是只为第 8 页增加 Spec。审批 UI 必须把这个完整结果展示出来。

## 四、数据模型

### 4.1 创建请求与持久化模型分离

客户端提交的是选择意图，服务端解析并生成规范化 `RunScope`。服务端不得信任客户端自行计算的章节展开结果。

```go
type ScopeObject string

const (
    ScopeObjectSpec         ScopeObject = "spec"
    ScopeObjectHTML         ScopeObject = "html"
    ScopeObjectPresentation ScopeObject = "presentation"
    ScopeObjectGlobal       ScopeObject = "global"
)

type ScopeSelectionKind string

const (
    ScopeCurrentPage    ScopeSelectionKind = "current_page"
    ScopeAllPages       ScopeSelectionKind = "all_pages"
    ScopeCustomPages    ScopeSelectionKind = "custom_pages"
    ScopeCustomSections ScopeSelectionKind = "custom_sections"
)

type ScopeSelectionInput struct {
    Kind           ScopeSelectionKind `json:"kind"`
    CurrentSlideID string             `json:"current_slide_id,omitempty"`
    SlideIDs       []string           `json:"slide_ids,omitempty"`
    SectionIDs     []string           `json:"section_ids,omitempty"`
}

type CreateRunScopeInput struct {
    Object    ScopeObject         `json:"object"`
    Selection ScopeSelectionInput `json:"selection"`
}
```

Runtime 持有的规范化范围：

```go
type ScopeSource struct {
    Kind       ScopeSelectionKind `json:"kind"`
    SectionIDs []string           `json:"section_ids,omitempty"`
}

type RunScope struct {
    Object                   ScopeObject  `json:"object"`
    SlideIDs                 []string     `json:"slide_ids"`
    Source                   ScopeSource  `json:"source"`
    IncludeRunCreatedSlides  bool         `json:"include_run_created_slides"`
    Revision                 int64        `json:"revision"`
}
```

说明：

- `slide_ids` 是授权页面集合的唯一事实来源，去重后按 outline 顺序保存。
- `source` 只用于展示和审计，不参与授权判断。
- `revision` 从 1 开始，每次有效范围变化加 1。
- `include_run_created_slides` 仅在 `all_pages` / `global` 语义下为 `true`。
- 标题、页码和页数不进入 `RunScope`，由当前 outline 生成展示投影。

### 4.2 校验不变量

服务端创建 Run 前必须保证：

1. `object` 是四个枚举值之一。
2. 非空项目的规范化 `slide_ids` 至少包含一项。
3. 所有 `slide_id`、`section_id` 必须存在于当前项目 outline。
4. `current_page` 只能携带一个有效 `current_slide_id`。
5. `custom_pages` 至少选择一页。
6. `custom_sections` 至少选择一章，并由服务端展开为该章直接和子层级包含的全部页面。
7. `all_pages` 忽略客户端页面列表，始终由服务端重新快照。
8. `global` 必须规范化为 `all_pages`，并令 `include_run_created_slides=true`。
9. `slide_ids` 不允许使用 `"current"`、页码或标题替代稳定 ID。

### 4.3 页面新增、删除与重排

- 当前 Run 通过已授权结构操作创建的新页，自动追加到活动 `slide_ids`，并产生一次 `scope.updated`。
- 其他来源新增的页面不自动进入非 `global` Run 的活动范围。
- 被删除页面从活动 `slide_ids` 移除；历史事件仍保留删除前的 ID 与展示快照。
- 页面重排、改名不改变活动 `slide_ids`；UI 使用最新页码和标题显示。
- 由于结构操作属于 `global`，现阶段“Run 内新建页”通常已经处于全局权限；上述规则仍作为 Runtime 不变量保留。

## 五、权限判断

### 5.1 能力集合

```text
spec         = { slide.spec }
html         = { slide.html }
presentation = { slide.spec, slide.html }
global       = { manifest, outline, design, all slide.spec, all slide.html, structure }
```

`spec` 与 `html` 互不包含，二者并集为 `presentation`；`global` 是最高权限。

### 5.2 Runtime 写入硬边界

所有写操作都必须在实际执行前和执行结果提交前各校验一次：

1. 根据 mutation / tool request 解析真实写目标。
2. 校验目标对象是否包含在 `scope.object` 能力集合中。
3. 非 `global` 时校验目标 `slide_id` 是否包含在 `scope.slide_ids`。
4. 失败时返回 `TARGET_OUT_OF_SCOPE`，不得产生临时提交、版本、文件或持久化副作用。
5. Completion Gate 再对 ChangeSet 做兜底校验，确保没有绕过前置检查的越界写入。

读取不受 `RunScope` 限制。ContextEngine 可以注入范围外摘要，Agent 也可以通过 `read_ppt` 读取范围外 Spec、HTML、outline 和 design；只读不等于授权写入。

### 5.3 工具可见性

工具是否披露和工具是否允许执行继续由 Runtime 决定：

- `spec`：只披露或只允许范围内 Spec 写能力。
- `html`：只披露或只允许范围内 HTML 写能力。
- `presentation`：允许范围内 Spec 与 HTML 写能力。
- `global`：允许 deck 级和结构性写能力。

即便模型伪造参数或重放旧调用，也必须由 Runtime 使用当前 `scope.revision` 和实际目标重新校验。

## 六、运行中扩权

### 6.1 控制工具

新增 Runtime 控制工具 `request_privilege`。名称固定使用正确拼写，不使用 `request_priviledge`。

它不是普通写工具，也不是 Shell `command.permission`：

- Agent 只能请求，不能自行批准。
- 该调用必须是模型一次响应中的唯一控制调用。
- Runtime 在等待用户时持久化 checkpoint，并暂停 Agent 循环。
- 用户回答后 Runtime 更新活动范围，再把结果作为工具返回值交还 Agent。

建议输入：

```json
{
  "add_slide_ids": ["sli_08"],
  "add_object": "spec",
  "reason": "需要同步第 8 页的内容结构，并读取后更新其设计稿"
}
```

约束：

- `add_slide_ids` 与 `add_object` 至少提供一项。
- 只允许提交增量，不接受完整新范围。
- `reason` 必填，面向用户说明必要性。
- `base_revision`、`call_id` 和 `interaction_id` 由 Runtime 注入，不由 Agent 自报。
- 已经完全包含在当前范围中的请求直接返回成功 no-op，不弹审批。
- 当前已是 `global` 时不存在更高权限，直接返回成功 no-op。

### 6.2 合并规则

页面维度做集合并集：

```text
result.slide_ids = current.slide_ids ∪ requested.slide_ids
```

对象维度按以下格合并：

| 当前 | 申请增加 | 结果 |
| --- | --- | --- |
| spec | html | presentation |
| html | spec | presentation |
| spec/html | presentation | presentation |
| 任意 | global | global + all_pages |
| presentation | spec/html | presentation |

当请求同时增加页面和对象时，先分别合并两个维度，再计算完整笛卡尔积。审批卡展示的是完整结果，而不是只展示 Agent 输入的增量。

### 6.3 审批交互

扩权审批卡采用与 `ask_clarification` 相近的 Run 内控制交互，但使用独立类型。卡片必须显示：

- 当前范围。
- Agent 请求增加的页面和对象。
- 合并后的完整范围。
- 完整范围覆盖的页面数量。
- Agent 给出的原因。
- 笛卡尔积导致的额外影响。

用户可执行：

- **批准**：接受系统计算的完整结果。
- **拒绝**：活动范围不变，Agent 收到 denied 结果后自行调整方案。
- **调整后提交**：复用范围选择器编辑最终结果并提交。

调整规则：

- 最终结果必须是当前活动范围的超集，不能撤销当前 Run 已有权限。
- 用户可以少给 Agent 一部分申请，也可以给出超出 Agent 申请的更大范围。
- 若用户想缩小已有权限，必须停止当前 Run，并用较小范围继续或新建 Run。
- 调整面板中的局部选择可即时变化，但审批本身仍需明确的“提交调整”动作，避免把浏览选择误当成授权。

### 6.4 活动范围重写

批准后直接重写当前 Run 的 `RunCommand.Scope`：

```text
active scope = approved result scope
revision     = previous revision + 1
```

不在活动状态中保留 `initial_scope + grants[]`，也不创建新 Run。初始范围、申请、批准人与前后差异通过持久化事件审计。

范围更新必须和 checkpoint、RunCommand、事件在同一事务边界内提交，避免重启后出现“工具以新范围执行，但 Run 记录仍是旧范围”。

## 七、公开事件与 REST 接口

### 7.1 创建 Run

`POST /api/v1/threads/:id/runs` 的 `scope` 改为选择输入：

```json
{
  "scope": {
    "object": "presentation",
    "selection": {
      "kind": "custom_pages",
      "slide_ids": ["sli_03", "sli_05", "sli_08"]
    }
  }
}
```

服务端响应和 `run.started` 返回规范化范围：

```json
{
  "scope": {
    "object": "presentation",
    "slide_ids": ["sli_03", "sli_05", "sli_08"],
    "source": { "kind": "custom_pages" },
    "include_run_created_slides": false,
    "revision": 1
  }
}
```

旧字段 `artifact`、`level`、`slide_id` 直接删除，不保留兼容解析。

### 7.2 扩权事件

新增三类公共事件：

```text
scope.expansion_requested
scope.expansion_answered
scope.updated
```

`scope.expansion_requested`：

```json
{
  "schema_version": 3,
  "run_id": "run_01",
  "occurred_at": "2026-09-08T10:00:00Z",
  "interaction_id": "scope_int_01",
  "call_id": "call_09",
  "base_revision": 1,
  "current_scope": {},
  "requested_addition": {
    "slide_ids": ["sli_08"],
    "object": "spec"
  },
  "proposed_scope": {},
  "affected_slide_count": 3,
  "reason": "需要同步页面结构"
}
```

`scope.expansion_answered` 记录用户决定、实际提交范围和展示摘要；`scope.updated` 记录 `before_scope`、`after_scope`、`cause`、`interaction_id`，并作为历史回放恢复活动范围的权威事件。

`cause` 第一版支持：

```text
agent_request_approved
user_adjustment
run_created_slide
slide_deleted
```

### 7.3 回答扩权

新增：

```text
POST /api/v1/runs/:id/scope-expansion
```

请求体：

```json
{
  "interaction_id": "scope_int_01",
  "base_revision": 1,
  "decision": "adjust",
  "adjusted_scope": {
    "object": "presentation",
    "selection": {
      "kind": "custom_pages",
      "slide_ids": ["sli_03", "sli_05", "sli_08", "sli_10"]
    }
  }
}
```

`decision` 为 `approve | reject | adjust`。服务端必须校验 `interaction_id`、待处理请求和 `base_revision` 一致；重复提交同一答案幂等成功，冲突答案返回 `409 SCOPE_EXPANSION_STALE`。

## 八、持久化与恢复

### 8.1 数据库直接切换

在开发期直接更新 `backend/migrations/0001_init.sql`：

- 删除 `scope_artifact`、`scope_level`、`scope_slide_id`。
- 增加 `scope_object`、`scope_slide_ids_json`、`scope_source_json`、`scope_include_run_created_slides`、`scope_revision`。
- `run_command_json` 继续保存完整活动 `RunCommand`；上述列是同事务更新的索引投影。
- 不新增历史数据迁移脚本，不支持旧库双读。

推荐字段约束：

```sql
scope_object TEXT NOT NULL
  CHECK (scope_object IN ('spec','html','presentation','global')),
scope_slide_ids_json TEXT NOT NULL,
scope_source_json TEXT NOT NULL,
scope_include_run_created_slides INTEGER NOT NULL DEFAULT 0,
scope_revision INTEGER NOT NULL CHECK (scope_revision >= 1)
```

### 8.2 Checkpoint

`RuntimeCheckpoint` 必须保存：

- 当前完整 `RunCommand`，包含最新 `RunScope`。
- 待回答的 `PendingScopeExpansion`。
- 当前 `WorkLedger`。
- 原有 plan、requirements、changes、phase 和 loop 信息。

恢复顺序：

1. 读取 Run 的活动 `RunCommand`。
2. 读取最新 checkpoint。
3. 校验二者 `scope.revision` 一致。
4. 若存在待回答扩权，恢复 waiting 卡片，不重复创建 interaction。
5. 若扩权已提交但 Agent 尚未收到工具结果，按 `call_id` 重放相同结果。
6. 已完成 work item 不重新生成，继续 pending / failed 项。

## 九、批量工作状态

### 9.1 独立 WorkLedger

为满足批量进度、失败恢复和“继续剩余页面”，新增独立状态：

```go
type SlideWorkStatus string

const (
    SlideWorkPending SlideWorkStatus = "pending"
    SlideWorkRunning SlideWorkStatus = "running"
    SlideWorkDone    SlideWorkStatus = "done"
    SlideWorkFailed  SlideWorkStatus = "failed"
)

type SlideWorkItem struct {
    SlideID    string          `json:"slide_id"`
    Status     SlideWorkStatus `json:"status"`
    PlanStepID string          `json:"plan_step_id,omitempty"`
    LastError  string          `json:"last_error,omitempty"`
}
```

### 9.2 工作项来源

- 范围中的页面不会自动变成 `pending`。
- 执行计划可在 step 上声明稳定 `target_slide_ids`，Runtime 据此创建 work item。
- 无计划模式下，页面第一次进入明确的批量写调用时创建对应 work item。
- work item 必须处于当前 `RunScope` 内；扩权批准前不能预先创建越界工作项。
- 单页有多个写操作时，状态聚合到同一个 slide item；全部操作成功才为 `done`。

### 9.3 完成与恢复

- Completion Gate 检查显式 WorkLedger 中的 `pending/running/failed`，但不检查 `RunScope` 中未进入 WorkLedger 的页面。
- 用户明确取消某个计划步骤时，应同步移除对应 work item，而不是缩小权限。
- 恢复时保留 `done`，只继续 `pending`，并向 Agent展示 `failed` 的错误证据。
- `RequirementLedger` 不再把 scope 作为普通需求项，也不能因为任意一次成功写入就将所有用户需求标为完成。

## 十、前端交互与视觉规范

### 10.1 Composer 范围按钮

按钮文案只显示页面模式和对象：

```text
自选页 · 演示文稿
自选章 · 幻灯片
全部页 · 全局资源
```

不显示“自选页 12 页”等数量。数量只出现在展开的选择窗口中。

### 10.2 任务范围窗口

窗口保持两行 pill：

1. 对象：设计稿 / 幻灯片 / 演示文稿 / 全局资源。
2. 页面：当前页 / 全部页 / 自选页 / 自选章。

删除以下内容：

- “决定本次任务可以修改哪些内容”等说明文字。
- “硬边界：Agent 可读取……”等灰色解释。
- “允许修改 1 页的 Spec + HTML”等影响摘要。
- 重置、完成、确认按钮。

普通 Composer 选择立即写入本地 draft；点击外部或关闭窗口不会回滚，发送 Run 时使用最后一次有效选择。

### 10.3 自选窗口

点击“自选页”或“自选章”时，在任务范围窗口上方叠加一个独立紧凑窗口，不切换完整页面，也不把大列表塞进 pill 内部。

自选页与自选章使用同构列表：

- 自选页：checkbox + 页码 + 页面 title。
- 自选章：checkbox + 章节 title + 页数。
- 顶部显示窗口名称和已选数量。
- 选择即时生效，不提供清空、完成或确认按钮。
- 列表可滚动；关闭上层窗口不关闭下层范围窗口。
- 自选项为空时 Composer 发送按钮禁用，并提示至少选择一页或一章。

### 10.4 全局资源联动

- 切到“全局资源”时页面行自动选中“全部页”，其余页面 pill 禁用。
- 切回非全局对象时恢复用户上一次非全局页面选择，避免意外丢失勾选。
- 全局资源状态下不显示自选窗口。

### 10.5 Run 状态展示

- `run.started` 后，Composer draft 与活动 Run 分离，当前 Run 范围由服务端响应/SSE 驱动。
- `scope.updated` 到达时更新任务状态中的范围摘要和 revision。
- 扩权 waiting 卡是时间线中的交互项，历史回放必须能恢复“待回答”和“已回答”状态。
- 页面标题和页码从最新 outline 投影；已删除页面在历史项中使用事件内展示快照。

### 10.6 可访问性与键盘

- 两组 pill 使用 `role="radiogroup"` / `role="radio"` 和 `aria-checked`。
- 自选列表使用多选语义与 `aria-selected`。
- `Esc` 先关闭自选窗口，再关闭范围窗口。
- 关闭自选窗口后焦点回到对应“自选页/自选章”pill。
- 禁用项既要有视觉态，也要设置真实 `disabled` / `aria-disabled`，不能只降透明度。

## 十一、后端与前端改造范围

### 11.1 后端

- `backend/internal/model/run_command.go`：替换 `Artifact/ScopeLevel/RunScope`，增加选择输入、规范化与 revision。
- `backend/internal/service/run.go`：创建 Run 时按 outline 解析范围；支持活动范围事务更新。
- `backend/internal/workflow/tools.go`：披露并处理 `request_privilege` 控制调用。
- `backend/internal/workflow/runtime.go`：增加 pending scope interaction、checkpoint、恢复与工具结果重放。
- `backend/internal/workflow/ppt_tools.go`、`render_tool.go`、`completion.go`：按页面集合和对象能力校验。
- `backend/internal/model/event.go`、`public_event.go`、`workflow/public_events.go`：增加三类 scope 事件及验证。
- `backend/internal/httpapi/router.go`、`run_handler.go`：增加扩权回答端点。
- `backend/internal/store/sqlite/*`、`backend/migrations/0001_init.sql`：切换新持久化结构与原子更新。
- `backend/internal/contextengine/*`、prompt modules：向 Agent 清晰注入活动范围、revision、可读/可写差异和 WorkLedger。

### 11.2 前端

- `frontend/src/api/types.ts`、`runs.ts`、`sse.ts`：切换新 Scope DTO、回答接口和事件联合类型。
- `frontend/src/features/agent/TargetSelector.tsx`：按原型重构两行范围窗口和叠加自选窗口。
- `frontend/src/features/agent/CommandComposer*`：管理选择 draft、全局联动、发送校验与简短按钮标签。
- `frontend/src/stores/runStore.ts`：维护活动 scope revision、扩权 waiting/answered 状态和 WorkLedger。
- `frontend/src/features/agent/eventReducer.ts`、`historyHydrator.ts`：支持新事件实时和历史回放。
- 新增 `ScopeExpansionPanel`：显示当前/申请/结果范围并处理批准、拒绝、调整提交。

## 十二、实现顺序

1. 替换共享数据模型、校验器和数据库 schema，更新现有 model/store 测试。
2. 实现创建 Run 的选择解析和规范化，打通 REST/SSE 新 scope。
3. 改造 Runtime 写权限、工具披露和 Completion Gate。
4. 重构 Composer `TargetSelector` 与自选窗口，接入真实 outline。
5. 增加 `request_privilege`、pending interaction、审批 API 和事件。
6. 增加活动范围原子更新、checkpoint 恢复与幂等重放。
7. 引入独立 WorkLedger，补批量进度与续做。
8. 完成前后端集成测试、视觉回归和旧协议清理。

## 十三、测试计划

### 13.1 模型与规范化

- 当前页解析为稳定 ID，拒绝 `"current"`。
- 全部页按提交时 outline 快照，顺序稳定且去重。
- 自选页空集合、失效 ID、重复 ID被正确处理。
- 自选章正确包含 section 的直接页面和 subsection 页面。
- 全局资源强制全部页并启用 run-created-page 跟随。
- 旧 `artifact/level/slide_id` 请求被拒绝。

### 13.2 授权

- 四种 object 对 Spec、HTML、manifest、outline、design、结构操作形成正确允许矩阵。
- 范围外读取成功，范围外写入在前置检查和 Completion Gate 均失败。
- 模型伪造 page ordinal/title 或重放旧 revision 无法越权。
- 多页 ChangeSet 中任何一项越权都不会产生部分提交。

### 13.3 扩权

- 单独增加页面、单独增加对象、同时增加两维的合并结果正确。
- `spec + html = presentation`，`global` 强制全部页。
- 已包含请求不弹卡并返回 no-op。
- 批准、拒绝、调整均可恢复 Agent；调整不能缩小当前范围。
- 用户可把调整结果扩展到 Agent 请求之外。
- 过期 revision、错误 interaction、冲突重答返回 409；同答案重放幂等。
- 进程在 requested、answered、updated 各边界重启后状态一致。

### 13.4 WorkLedger

- 仅显式计划目标进入 pending，范围内其他页不进入工作清单。
- 单页批量操作正确经历 pending → running → done/failed。
- 恢复不重新处理 done 页面。
- Completion Gate 只阻塞显式未完成 work item，不因未写范围页失败。

### 13.5 前端

- 两行 pill、按钮标签、叠加窗口与原型一致。
- 自选页和自选章列表均显示标题并支持多选。
- 不出现灰色解释、影响摘要、重置、完成、普通确认按钮。
- 目标按钮不显示页数。
- 全局资源联动、空选择禁发、Esc 层级关闭和焦点恢复正确。
- 实时 SSE 与历史 hydration 得到相同范围和审批卡状态。

## 十四、验收标准

1. 用户可以在 Composer 内选择当前页、全部页、自选页或自选章，并与四种对象自由组合；全局资源除外，它固定全部页。
2. Run 创建后，服务端和事件中只出现稳定 `slide_id[]`，页码变化不影响权限。
3. Agent 能读取范围外内容，但任何范围外写操作都会被 Runtime 拒绝且不产生副作用。
4. Agent 可请求扩权，用户可批准、拒绝或调整；未经用户接受，活动范围不变。
5. 扩权卡明确显示合并后的完整笛卡尔积和影响页数。
6. 批准后当前 Run 原地更新范围并继续，不创建新 Run。
7. 自选页/自选章采用范围窗口上方的紧凑列表，Composer 按钮只显示名称，不显示数量。
8. 范围窗口中没有解释性灰字、影响摘要、重置、完成或普通确认按钮。
9. 选中但未修改的页面不会导致 Run 无法完成。
10. 中断恢复后，活动范围、待审批状态和批量页面进度均可正确恢复，已完成页面不会被无故重做。

## 十五、QA 决策记录

本节只记录“重新 grilling”之后的 15 个有效问题。重开前的早期 Q1–Q3 草案不属于当前决策依据。后续视觉评审覆盖旧结论时，以“最终落点”为准。

| Q | 问题 | 用户选择 / 回答 | 最终落点 |
| --- | --- | --- | --- |
| Q1 | 范围是提示还是权限？ | 硬边界 | 范围外可读，范围外写由 Runtime 拒绝。 |
| Q2 | 页面对象如何区分？ | 选择“页面对象与全局资源分开”的方案 | 最终结合 Q5 收敛为 Spec、HTML、Spec+HTML、全局资源四类。 |
| Q3 | 当前页/全部页/自选页/章节如何持久化？ | 接受提交时解析为稳定 ID | 授权只认 `slide_id[]`，选择来源仅用于展示和审计。 |
| Q4 | 权限用多个 grant 还是直接组合？ | 直接笛卡尔积 | 页面集合与对象能力只有一个活动组合，不支持逐页差异权限。 |
| Q5 | 最终对象选项是什么？ | 设计稿、幻灯片、演示文稿、全局资源 | 分别对应 Spec、HTML、Spec+HTML、全部项目资源。 |
| Q6 | Agent 如何申请更多权限？ | 增加 `request_priviledge` 概念 | 正式命名为 `request_privilege`；Agent 请求、用户授权、Runtime 执行。 |
| Q7 | 批准扩权后如何保存？ | 重写 scope | 原地更新当前 Run 的活动范围，不保存 `initial_scope + grants[]`，不新建 Run。 |
| Q8 | “全局资源”具体覆盖什么？ | 全部 | manifest、outline、design、所有页面 Spec/HTML；页面强制全部页。 |
| Q9 | Agent 申请增量还是完整范围？ | 只提交需要新增的内容 | Runtime 合并两个维度，展示完整笛卡尔积，批准后重写活动范围。 |
| Q10 | 章节粒度怎么表达？ | 当时要求章、节分开 | 后续视觉评审覆盖：移除“所选节/自选节”，第一版只保留“自选章”。 |
| Q11 | 扩权审批如何交互？ | 类似 `ask_clarification` | 用户可批准、拒绝、调整后提交；Run 等待回答再继续。 |
| Q12 | “全部页”是否包含 Run 新建页？ | 只自动纳入当前 Run 自己创建的页 | 初始仍是稳定 ID 快照；外部新增不跟随，Run 授权创建的页面自动追加。 |
| Q13 | 选中的范围是否都必须完成？ | 否，范围只是权限边界 | Scope 不作为工作清单或完成门槛；批量进度由独立 WorkLedger 表达。 |
| Q14 | 用户调整能否超过 Agent 申请？ | 可以，但不能撤销当前 Run 已有权限 | 最终范围可任意扩大；缩小需停止当前 Run 后以更小范围继续。 |
| Q15 | 自选页/章节选择器放在哪里？ | 保留范围 popup 的 pill，并在其中进入二级多选 | 后续视觉评审进一步收敛为范围窗口上方的独立紧凑叠加窗口。 |

## 十六、视觉评审追加记录

| 顺序 | 用户反馈 | 最终处理 |
| --- | --- | --- |
| V1 | 所有灰色解释文本都不需要 | 删除说明、副标题、硬边界提示和影响摘要。 |
| V2 | 重置和完成造成拥挤 | 普通范围选择器删除重置、完成和确认，选择即时生效。 |
| V3 | 去掉“所选节”，文字改为“自选页/自选章” | 页面选项只保留当前页、全部页、自选页、自选章。 |
| V4 | 具体选择组件太大，希望看更轻的呈现 | 放弃大面积二级选择视图。 |
| V5 | 最好在范围窗口上方累加一个窗口 | 最终采用上下叠放的双窗口结构。 |
| V6 | 自选页也应像自选章一样使用 title 列表 | 两种列表同构；自选页显示页码与 title。 |
| V7 | 范围按钮不显示“自选页 12 页” | 按钮只显示“自选页”，数量仅在展开窗口出现。 |
| V8 | 临时 Demo 只保留左栏目，聚焦修改处 | 原型收敛为单 Agent 栏，不模拟完整编辑器页面。 |

---

以上 Q1–Q15 与 V1–V8 共同构成本需求的有效产品依据；发生冲突时，后续视觉评审和本文前述规范优先。
