# PPT Agent 多模态 Observation、Steering、取消、幂等与错误设计

> 日期：2026-08-03
>
> 状态：设计确认稿
>
> 范围：提升 HTML PPT Agent 真实可用性的五项 Runtime 与前端能力
>
> 文档性质：后续实现与验收的权威增量规格

## 1. 结论

在 PPT Resource、字符串工具、Spec 命名、Completion Gate 和 Materialization Proof 已正确落地的
前提下，下一阶段只实施五项能力：

1. 多模态截图 Observation；
2. Active Run Steering；
3. 权威取消；
4. 端到端幂等；
5. 统一错误模型。

五项能力共同形成以下产品闭环：

```text
生成 PPT
→ Agent 看见真实页面
→ 用户在运行中追加要求
→ Agent 在同一个 ReAct Loop 中调整
→ 用户可安全取消
→ 网络重试和重复操作不破坏项目
→ 错误对 Runtime、Agent 和用户分别可执行
```

本设计不增加业务工具、Runtime 控制动作或公共事件种类，不改变公开任务模型，不引入多 Agent、
Workflow DAG、独立视觉 Verifier、进程级恢复或 Eval 平台。

## 2. 现状与需要补齐的边界

当前代码已经具备：

- `render_slide` 的 Chromium 截图、诊断、安全截图 URL 和 Render Evidence；
- `InputQueue`、`SteeringSource.DrainInputs()` 和 ReAct turn 间输入注入；
- Run `DELETE` 取消入口和 `context.Context` 传播；
- Run、Tool Call、Commit 等稳定 ID 的部分基础；
- Tool Result、HTTP API、Public Event 的分散错误结构；
- SSE 历史补发和 Last-Event-ID。

当前不足：

- LLM Message 正文仍是纯字符串，截图只供前端预览，Agent 看不到像素；
- Steering 输入只存在内存队列，没有独立 API 语义、持久化、确认状态和消息幂等；
- 取消接口只触发 `cancel()`，没有 `cancel_requested` 生命周期、工具配对收尾和 Worker 请求级取消；
- 创建 Run、Steering、Tool Call 和 Commit 没有统一幂等记录与请求哈希冲突规则；
- 错误码、重试策略、模型修正信息和用户安全信息没有共同事实来源。

目标是扩展现有架构，而不是替换 Single ReAct Runtime。

## 3. 保持不变的产品约束

```text
target.artifact = spec | presentation
target.level    = deck | slide
interaction     = talk | ask | execute
strategy        = chat | simple | complex
```

继续保持：

- 所有 Strategy 运行在一个连续 ReAct Loop；
- Complex 只额外拥有动态 Plan；
- 所有正常成功退出显式调用 `finish` 并通过 Completion Gate；
- 业务工具固定为 `read_ppt/mutate_ppt/mutate_ppt/search_refs/render_slide`；
- 控制动作固定为 `update_plan/ask_user/finish`；
- 公共事件固定为现有 11 种；
- staging、Evidence、Context、Strategy、Phase 和 Trace 默认不暴露给普通用户；
- 三栏、顶部 Project Tabs、右侧 Thread Tabs 和底部 Composer 保持不变。

## 4. 总体架构

```text
Frontend Composer / Cancel
        │
        ├── Create Run
        ├── Steer Active Run
        └── Request Cancel
        │
        ▼
Run Command API
        │
        ├── Idempotency Store
        ├── Canonical Error Mapper
        └── Active Run Registry
                │
                ▼
        Single ReAct Runtime
                │
        ┌───────┴────────┐
        │                │
  Steering Inbox   Cancellation Token
        │                │
        └───────┬────────┘
                ▼
       Tool Batch / Render Worker
                │
     text diagnostics + screenshot
                ▼
    Multimodal Observation Builder
                │
                ▼
     Provider Capability Adapter
```

五个独立模块：

| 模块 | 单一职责 |
| --- | --- |
| `ObservationBuilder` | 把 Tool Result 和受控截图转为模型可见多模态 Observation |
| `SteeringInbox` | 持久化、确认、排序并在安全边界交付 Active Run 输入 |
| `CancellationCoordinator` | 将取消请求传播到 Provider、工具、渲染和 staging 收尾 |
| `IdempotencyStore` | 对外部命令和有副作用内部动作执行去重与结果重放 |
| `AgentError` | 作为内部权威错误，生成 Model/Public/API/Trace 四种投影 |

## 5. 多模态截图 Observation

### 5.1 产品目标

`render_slide` 成功或发现布局问题后，Agent 必须同时获得：

- 对应 `slide_id` 和 `tool_call_id`；
- 结构化 diagnostics；
- 实际页面截图；
- 受控修正提示。

Agent 应能识别没有触发确定性检查的视觉问题，例如层级弱、留白失衡、对比不足、图文关系差和
跨页风格不一致，并在同一个 ReAct Loop 中继续修改和渲染。

这不是独立视觉评分器。Runtime 不替模型给页面打审美分，也不新增 Reviewer Agent。

### 5.2 内部消息模型

禁止继续用单个 `Content string` 表达所有模型输入。内部协议使用内容联合：

```text
Message
├── role
├── tool_call_id
├── tool_calls
└── content[]
    ├── text
    └── image
```

概念类型：

```go
type ContentPart struct {
    Type     string // text | image
    Text     string
    ImageRef string
    MIMEType string
    Detail   string // low | high
}
```

`ImageRef` 是 Runtime 内部受控引用，不是模型可填写的 URL。Provider Adapter 负责将其解析为供应商
支持的 data URL、上传文件 ID 或原生 image input。

不得将本地绝对路径传给 Provider。

### 5.3 Render Observation

一次成功 Render 的模型 Observation 逻辑结构：

```json
{
  "tool_call_id": "call-render-03",
  "resource": {
    "type": "slide",
    "slide_id": "slide-03",
    "part": "html"
  },
  "summary": "页面已渲染",
  "diagnostics": {
    "overflow": {
      "horizontal": false,
      "vertical": false
    },
    "clipping_count": 0,
    "console_errors": [],
    "failed_resources": [],
    "font_status": "loaded"
  },
  "source_hash": "runtime-internal"
}
```

随后附带一项 Image Content Part。`source_hash` 只进入内部 Observation 元数据和 Trace，公共事件
继续隐藏。

Render 失败但成功生成截图时，也可以把截图作为失败 Observation 发送给模型；Worker 未生成合法
截图时只返回文本错误。

### 5.4 Provider 能力

Provider Adapter 必须声明能力：

```go
type ProviderCapabilities struct {
    Vision            bool
    MultipleToolCalls bool
    ImageInputMIMEs   []string
    MaxImageBytes     int
}
```

规则：

- Presentation Execute Run 在预计需要 `render_slide` 时必须使用 `Vision=true` 的配置；
- Provider 不支持视觉时，Run 创建前返回 `VISION_CAPABILITY_REQUIRED`；
- 不允许静默丢弃 Image Part 后声称已完成视觉观察；
- Tool Schema 和业务工具名称不随 Provider 改变；
- Provider 特殊 wire format 只存在于 Adapter。

### 5.5 图片安全与预算

- 截图必须属于当前 Project 和 Run；
- 只允许 Runtime 生成的 screenshot ID；
- MIME 固定为受支持的 `image/png` 或压缩后的 `image/webp/jpeg`；
- 原图上限 1600×900，超过 Provider 字节上限时由 Runtime 等比压缩；
- 不读取模型传入的任意路径或 URL；
- 每次 Observation 默认只携带本次最新截图；
- Context Compaction 保留文字结论，移除已被后续 Render 取代的旧截图；
- 多页批量 Render 的截图按原 Tool Call 顺序与 `call_id` 配对。

### 5.6 前端投影

公共事件仍通过 `tool.completed.preview.image_url` 展示截图。禁止把 base64、Provider File ID、
内部 ImageRef 或绝对路径写入 SSE。

## 6. Active Run Steering

### 6.1 产品语义

Steering 是“把新增用户要求交给当前仍在执行的 Run”，不是创建新 Run，也不是 `ask_user` 的回答。

前端负责：

- Active Run 期间保持 Composer 可输入；
- 乐观显示用户追加消息；
- 展示“发送中、已接收、未能加入当前任务”；
- Run 已不可 Steering 时，将原输入保留并允许创建下一 Run。

后端负责：

- 判断 `expected_run_id` 是否仍是当前可 Steering 的 Run；
- 持久化 Steering；
- 幂等确认；
- 按接收顺序交付同一个 ReAct Loop；
- 在安全边界注入模型历史；
- 处理 Run 完成、取消和等待问题的竞态。

因此 Steering 不是纯前端功能。

### 6.2 API

新增明确入口：

```http
POST /api/v1/runs/{run_id}/steer
```

请求：

```json
{
  "expected_run_id": "run-123",
  "client_message_id": "msg-456",
  "content": "后面的页面统一改成深色风格"
}
```

约束：

- `expected_run_id` 必须等于路径 Run；
- `client_message_id` 在当前 Thread 内稳定且唯一；
- `content` 去除首尾空白后 1～8000 字符；
- 不允许携带 `reply_to`；问题回答继续走结构化 Answer 入口；
- 相同 `client_message_id + request_hash` 重放时返回第一次结果；
- 相同 ID、不同内容返回 `IDEMPOTENCY_KEY_REUSED`。

成功响应：

```json
{
  "status": "accepted",
  "run_id": "run-123",
  "client_message_id": "msg-456"
}
```

使用 `202 Accepted`，表示已进入权威 Inbox，不表示模型已经处理完成。

### 6.3 Steering 状态

内部状态：

```text
accepted → injected
accepted → rejected
```

最小持久字段：

```text
run_id
thread_id
client_message_id
request_hash
content
status
accepted_at
injected_at
rejection_code
```

不要求本轮实现进程重启后的 Run 恢复，但不得让页面刷新或 SSE 重连造成已接收 Steering 消失或重复。

### 6.4 可 Steering 状态

| Run 状态 | 行为 |
| --- | --- |
| `pending` | 接受并在 Runtime 首轮前注入 |
| `running` | 接受并在下一安全边界注入 |
| 等待 `ask_user` 回答 | Steering 接口拒绝 `RUN_WAITING_FOR_ANSWER`，避免与回答混淆 |
| `cancel_requested` | 拒绝 `RUN_CANCELING` |
| 正在 Completion/Commit | 拒绝 `RUN_NOT_STEERABLE` |
| terminal | 拒绝 `RUN_NOT_STEERABLE` |

前端收到 `RUN_NOT_STEERABLE` 后不得丢弃内容，应允许用户用相同文本创建下一 Run。

### 6.5 注入安全边界

Steering 只在以下位置 Drain：

```text
Run 初始化后、第一次 Agent.Next 前
每个 Tool Batch 全部完成并记录 Observation 后
ask_user 回答恢复后
Completion Gate 拒绝返回 Loop 后
```

禁止：

- 在一个 Tool Call 执行中途改变其参数；
- 在批量写调用中途插入新要求；
- 在原子 Commit 已开始后注入；
- 为 Steering 创建第二个 ReAct Loop。

多个未注入 Steering 按接收顺序组成独立 User Message Item，不拼成不可追踪的大字符串。

### 6.6 Plan 与 Strategy

- Simple Run 收到明显扩大范围的 Steering 时，可以沿用现有升级逻辑切换为 Complex；
- Complex Run 由 Agent 在同一个 Loop 中调用 `update_plan` 修订计划；
- Steering 不直接修改 Plan；
- Steering 不绕过当前 WorkSpec 写权限。若新增要求超出公开 target scope，Runtime 返回 Scope
  Observation，Agent 应请求用户开启新的合适 Run。

## 7. 权威取消

### 7.1 状态语义

取消是请求—确认过程：

```text
running
→ cancel_requested（内部）
→ canceled（公共终态）
```

`cancel_requested` 可以是数据库字段或内部时间戳，不增加公共 Run Status 枚举。普通 UI 在请求成功后
显示“正在取消”，只有收到 `run.finished(status=canceled)` 后显示“已取消”。

### 7.2 API

保留当前：

```http
DELETE /api/v1/runs/{run_id}
```

首次有效请求返回 `202`：

```json
{
  "status": "cancel_requested",
  "run_id": "run-123"
}
```

重复请求：

- 已处于 cancel requested：返回同一结果；
- 已 canceled：返回 `200` 和权威 canceled 状态；
- 已 completed/failed：返回当前终态，不得篡改为 canceled；
- Run 不存在：`404 RUN_NOT_FOUND`。

### 7.3 取消传播

```text
HTTP Cancel
→ Run Context cancel
→ Provider request context
→ Tool Batch context
→ Render request context
→ ask_user wait
→ staging cleanup
→ terminal event
```

要求：

- Provider HTTP 请求能立即观察 `ctx.Done()`；
- 等待并发槽位的调用退出；
- 已开始的 Render Request 支持请求级取消；
- Browser Worker 和 Browser Process 保留，只关闭对应 Page/Context/本地 HTTP server；
- 等待用户回答时取消必须解除等待；
- 取消后禁止进入 Completion Gate 接受路径和 Commit；
- 未提交 staging 全部清理；
- 已经 committed 的历史项目状态不回滚。

### 7.4 Tool Event 配对

已产生 `tool.started` 的调用，在 `run.finished` 前必须产生对应 `tool.completed`：

```text
status = failed
error.code = RUN_CANCELED
retryable = false
```

尚未启动且因批次取消跳过的调用，不产生虚假的 `tool.started`。内部 Trace 记录 skipped。

公共终态顺序：

```text
所有已开始 Tool 的 completed
→ run.finished(canceled)
```

取消不产生 `message.final` 成功消息。

### 7.5 Render Worker 取消协议

Worker 命令需要区分：

```json
{"type":"render","request_id":"render-1", "...":"..."}
```

```json
{"type":"cancel","request_id":"render-1"}
```

Worker 为每个 active request 保存 Page/Context/Server cleanup handle。取消只清理目标请求并返回 canceled
响应，不终止共享 Browser；Worker 崩溃仍由现有健康重启机制处理。

## 8. 端到端幂等

### 8.1 目标

以下情况不得产生重复副作用：

- 用户双击创建 Run；
- 前端重试 Steering；
- Cancel 响应丢失后重发；
- Provider 或 Runtime 重放 Tool Call；
- Commit 成功但 HTTP/进程响应链路中断；
- SSE 重连重复收到历史事件。

### 8.2 幂等范围

| Scope | Key | 权威结果 |
| --- | --- | --- |
| Create Run | `thread_id + client_request_id` | 已创建的 Run |
| Steering | `thread_id + client_message_id` | accepted/rejected 状态 |
| Cancel | `run_id + cancel` | 当前取消/终态状态 |
| Tool Call | `run_id + call_id` | Tool Result |
| Commit | `run_id + commit` | Commit Result 和 Version IDs |

Create Run 请求新增：

```json
{
  "client_request_id": "req-123",
  "instruction": "...",
  "target": {},
  "interaction": {}
}
```

### 8.3 请求哈希

幂等记录必须同时保存 canonical request hash。

```text
同一 key + 同一 hash
→ 返回第一次权威结果

同一 key + 不同 hash
→ IDEMPOTENCY_KEY_REUSED
```

Canonical JSON 必须稳定排序并排除传输层无关字段。

### 8.4 存储模型

概念表：

```text
idempotency_records
├── scope
├── owner_id
├── key
├── request_hash
├── status        // in_progress | completed | failed
├── result_json
├── created_at
└── updated_at
```

唯一约束：

```text
(scope, owner_id, key)
```

本轮仍是单进程 SQLite，但获取或创建记录必须位于数据库事务中，不能只靠进程内 Map。

### 8.5 Tool Call 幂等

在实际执行前：

1. 计算 `tool name + canonical args + disclosed scope` 的 request hash；
2. 以 `run_id + call_id` 查询；
3. 同 hash 已完成则重放原 Tool Result，不重新执行；
4. 不同 hash 返回 `IDEMPOTENCY_KEY_REUSED`；
5. `in_progress` 不启动第二份副作用，返回或等待权威结果；
6. 执行完成后原子保存安全可序列化 Tool Result。

Runtime 内部 Evidence 必须能够从幂等结果重建，或和 Tool Result 同事务保存。不能只重放面向模型的
简化文本而丢失 Gate 所需 Evidence。

### 8.6 Commit 幂等

- `run_id` 对 Artifact Commit 唯一；
- 同一 Run 只能创建一次对应 Version 集合；
- Commit 重放返回原有 Version 和 Revision，不再次递增；
- 文件已经落盘、数据库提交失败时继续沿用现有原子恢复；
- 数据库已经提交、调用方未收到结果时，再次提交查询并返回原结果；
- 不允许通过随机生成新 Version ID 绕过重复检测。

### 8.7 SSE

现有 Run 内连续 `seq` 和 Last-Event-ID 继续作为事件幂等依据。前端 Reducer 以
`run_id + seq` 去重；Tool Item 额外以 `run_id + call_id` 合并 started/completed。

## 9. 统一错误模型

### 9.1 一个事实，四种投影

内部只定义一个权威错误：

```go
type AgentError struct {
    Code        string
    Category    ErrorCategory
    Operation   string
    Resource    *Resource
    CallID      string
    Retryable   bool
    SafeMessage string
    ModelMessage string
    Details     map[string]any
    Cause       error
}
```

生成：

```text
AgentError
├── Model Observation：具体、可修正
├── Public Event Error：安全、可理解
├── HTTP API Error：稳定机器码
└── Trace Error：完整 cause 和内部诊断
```

禁止 HTTP、Tool、Runtime 和 Public Projector 各自维护互相漂移的错误码含义。

### 9.2 错误类别

```text
transient
agent_repairable
user_action_required
conflict
canceled
terminal
```

| Category | 典型错误 | 行为 |
| --- | --- | --- |
| `transient` | Provider 429/5xx、短暂连接失败、Worker 重启 | Runtime 限次自动重试 |
| `agent_repairable` | Schema、HTML、锚点、Render 诊断失败 | 返回同一 ReAct Loop |
| `user_action_required` | 缺少必需信息、等待回答、能力未配置 | `ask_user` 或公开失败 |
| `conflict` | Revision、幂等键、活动 Run 冲突 | 停止副作用并要求重新读取/重新发起 |
| `canceled` | Run 或 Tool 被取消 | 权威 canceled 收尾，不重试 |
| `terminal` | 数据损坏、内部不变量失败 | 结束 Run，保留 Trace |

### 9.3 Model Observation

Agent 可修正错误至少包含：

```json
{
  "ok": false,
  "code": "EDIT_ANCHOR_AMBIGUOUS",
  "category": "agent_repairable",
  "resource": {
    "type": "slide",
    "slide_id": "slide-03",
    "part": "html"
  },
  "reason": "old_text matched 3 locations",
  "location": null,
  "next_action": "read_ppt and choose a longer unique anchor",
  "retryable": true
}
```

Schema 错误提供 JSON Pointer；HTML 错误提供检查项；冲突提供当前 Revision；不得只返回
“操作失败”。

### 9.4 Public/API 投影

Public Error 固定保留：

```text
code
message
retryable
```

HTTP Error 固定：

```json
{
  "error": {
    "code": "RUN_NOT_STEERABLE",
    "message": "当前任务已经进入完成阶段，请作为新要求发送。",
    "retryable": false,
    "details": {}
  }
}
```

公共投影禁止包含：

- Provider 原始响应；
- 数据库错误；
- 绝对路径；
- raw HTML/JSON；
- API key；
- Provider reasoning；
- stack trace。

### 9.5 自动重试

只对 `transient` 自动重试：

- Provider 429/5xx 和连接建立失败；
- 明确可重启的 Render Worker 崩溃；
- 无副作用或已由幂等层保护的请求。

禁止自动重试：

- `mutate_ppt/mutate_ppt` 的业务错误；
- Schema 和锚点错误；
- Revision Conflict；
- Idempotency Key Reused；
- Cancel；
- Commit 的未知结果，除非先查询幂等提交记录。

重试次数、退避和最终错误进入内部 Trace，普通 UI 只显示低噪声“正在重试”状态。

## 10. 前端体验

### 10.1 Active Composer

Active Run 期间 Composer 不禁用：

- 默认提交转为 Steering；
- 等待 `ask_user` 时 Composer 进入回答模式，不发送 Steering；
- Run 不可 Steering 时保留文本并提示作为下一请求发送；
- 每条消息使用前端生成的稳定 `client_message_id`。

### 10.2 Cancel

- Active Run 显示取消按钮；
- 点击后按钮禁用，状态显示“正在取消”；
- 只有权威 `run.finished(canceled)` 后显示“已取消”；
- 超时未收到终态时通过 `GET /runs/{id}` 对账；
- 不在前端自行伪造 terminal event。

### 10.3 Error

- `retryable=true` 的 Run 失败显示“重试”；
- Retry 创建新 Run，继承同一 Thread、Project 和原用户意图；
- Retry 使用新的 `client_request_id`，不能复用失败 Run ID；
- Agent 可修正 Tool Error 保持低噪声 Timeline，不弹用户级错误；
- 用户需处理的错误提供明确动作。

### 10.4 视觉

继续使用现有 `tool.completed.preview` 展示截图，不改变三栏和 Timeline 协议。多模态模型输入是
Runtime 内部能力，不新增前端“视觉 Agent”概念。

## 11. 公共事件

仍然只有：

```text
run.started
run.progress
run.finished
plan.updated
message.reasoning
message.milestone
message.final
tool.started
tool.completed
question.asked
question.answered
```

复用规则：

- Steering 接收：用户消息进入 Thread History；可用 `run.progress` 短暂显示“已接收追加要求”；
- Cancel requested：`run.progress(stage=finalizing, text=正在取消)`；
- Tool canceled：`tool.completed(status=failed, error.code=RUN_CANCELED)`；
- Run canceled：`run.finished(status=canceled)`；
- 自动重试：`run.progress`，不新增 retry event；
- 多模态截图：仍只投影安全 preview URL。

公共事件 `schema_version` 继续为 2，除非实际 payload 结构发生不兼容变化；本设计不要求升级。

## 12. 数据与迁移

最小新增数据：

- Steering Inbox；
- Idempotency Records；
- Run cancel requested timestamp；
- Tool Call 幂等结果及内部 Evidence；
- Commit 幂等结果。

迁移必须：

- 对已有数据库可一次升级；
- 为唯一键建立数据库约束；
- 不修改 PPT 项目 Resource 文件布局；
- 不迁移或复制现有截图正文到数据库；
- 失败时保持原数据库可启动或事务回滚。

## 13. 测试

### 13.1 多模态

- Render Observation 同时包含 diagnostics 和对应截图；
- 多个 Render Tool Calls 的图片按 `call_id` 正确配对；
- Provider wire request 真实包含图片输入；
- Provider 不支持 Vision 时明确拒绝，不静默降级；
- 不能通过 ImageRef 读取其他 Project/Run 或任意路径；
- 旧截图在 Context Compaction 后不重复发送；
- 真实视觉 Provider 集成测试至少证明模型收到图片；无凭证时明确标记未运行。

### 13.2 Steering

- pending/running Run 接受 Steering；
- Steering 在下一个安全边界进入同一个 ReAct Loop；
- 多条 Steering 保序；
- 同 ID 同内容不重复注入；
- 同 ID 不同内容拒绝；
- waiting/canceling/completing/terminal 状态正确拒绝；
- Steering 与 Run 完成竞态只有一个权威归属；
- 页面刷新后已 accepted 消息仍存在于 Thread History；
- Simple 可按现有规则升级为 Complex。

### 13.3 Cancel

- Provider 请求、等待槽位、Tool Batch、Render 和 ask_user 均响应取消；
- 已 started Tool 在 Run terminal 前得到 completed；
- 取消不进入 Commit；
- staging 被清理；
- Browser Worker 保留并能继续服务下一 Run；
- 重复 Cancel 幂等；
- Cancel 与自然完成竞态只产生一个 terminal event；
- Cancel 后没有成功 `message.final`。

### 13.4 幂等

- Create Run、Steering、Cancel、Tool Call、Commit 分别覆盖同 hash 重放；
- 不同 hash 复用 key 返回 `IDEMPOTENCY_KEY_REUSED`；
- 并发相同请求只产生一个副作用；
- Tool Result 重放恢复 Completion Gate 所需 Evidence；
- Commit 重放不增加 Revision、Version 或文件写入；
- SSE 重连不重复 Timeline Item。

### 13.5 错误

- 每个稳定错误码只有一个注册定义；
- Model/Public/API/Trace 投影符合各自可见范围；
- Agent 修正错误包含可执行 next action；
- Public Error 不泄露路径、raw content、Provider body 或 Cause；
- 只有 transient 自动重试；
- Cancel/Conflict/Schema 错误不被错误重试。

### 13.6 回归

- 后端全量测试；
- Go race；
- 真实 Chromium；
- 前端测试、TypeScript、Lint 和 Build；
- talk/ask/execute；
- chat/simple/complex；
- 多 Tool Calls；
- Completion Gate、Materialization Proof、原子 Commit；
- 11 种公共事件和 SSE 续传。

## 14. 实施顺序

工程依赖顺序：

1. 统一 `AgentError` 和投影；
2. 幂等 Store、API 命令幂等和 Commit 幂等；
3. 取消状态、传播和 Worker 请求级取消；
4. Steering API、Inbox、前端 Active Composer；
5. 多模态 Message、Provider Capability 和 Render Observation。

多模态模块可以在接口确定后与 2～4 并行开发，但最终必须共同进行竞态和回归测试。

## 15. 非目标

- 多 Agent 或 Handoff；
- 通用 Shell、文件系统或 MCP 工具扩张；
- Workflow DAG；
- 独立视觉 Reviewer/Verifier；
- 自动审美评分；
- 进程重启后的 Run 恢复；
- 参考资料上传管线；
- 元素级 DOM 选中编辑；
- Eval Harness、版本对比和指标平台；
- 新公共事件协议；
- 页面整体视觉重构；
- 打字机流。

## 16. 完成定义

只有同时满足以下条件才算完成：

1. Agent 的真实 Provider 请求包含 Render 截图，而不只是截图 URL 文本；
2. Steering 有后端持久化、状态判断、幂等和安全边界注入；
3. Cancel 能传播到 Provider、Tool 和 Render，并以唯一权威终态结束；
4. Create Run、Steering、Tool Call 和 Commit 的重复请求不产生重复副作用；
5. 错误码、分类和重试策略来自同一内部错误模型；
6. 公共事件仍为 11 种且不泄露内部图片引用和错误 Cause；
7. 前端保留现有布局并支持运行中追加要求、取消和失败重试；
8. 全量、Race、真实 Chromium 和前端构建通过；
9. 有视觉模型凭证时完成真实多模态主路径；无凭证时明确报告未验证；
10. 当前工作区中与本实施无关的修改没有被覆盖、删除或 reset。

## 17. 设计参考

- Codex App Server 的 Thread/Turn/Item、`turn/steer`、`turn/interrupt` 和 client message identity：
  <https://github.com/openai/codex/blob/main/codex-rs/app-server/README.md>
- OpenAI Agents SDK 的 Sessions、Tool Error 和 Tool Guardrails：
  <https://openai.github.io/openai-agents-python/sessions/>
  <https://openai.github.io/openai-agents-python/tools/>
  <https://openai.github.io/openai-agents-python/guardrails/>
- MCP 的 Progress 与 Cancellation：
  <https://modelcontextprotocol.io/specification/2025-03-26/basic/utilities/progress>
  <https://modelcontextprotocol.io/specification/2025-06-18/schema>
- LangGraph 对持久执行副作用幂等的说明：
  <https://docs.langchain.com/oss/python/langgraph/functional-api>
