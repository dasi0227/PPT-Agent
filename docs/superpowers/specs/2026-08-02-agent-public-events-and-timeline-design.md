# PPT Agent 公共事件协议与右侧时间线设计

> 日期：2026-08-02  
> 状态：设计确认稿  
> 范围：Run 公共事件、内部 Trace 分层、消息生产机制、SSE/历史投影、右侧 Agent 时间线展示  
> 文档性质：设计规格与验收依据，不是旧协议兼容说明  
> 配套设计：
> [Adaptive ReAct Execution Strategy](./2026-08-02-adaptive-react-execution-strategy-design.md)、
> [PPT Agent Tool Surface](./2026-08-02-ppt-agent-tool-surface-design.md)

## 1. 结论

本项目对前端只公开 11 种事件：

```text
Run
├── run.started
├── run.progress
└── run.finished

Plan
└── plan.updated

Message
├── message.reasoning
├── message.milestone
└── message.final

Tool
├── tool.started
└── tool.completed

Question
├── question.asked
└── question.answered
```

核心原则：

- 公共事件服务产品体验，不等于 Runtime 全量日志。
- Context、Strategy、Phase、Staging、Evidence、Completion Gate 等属于内部运行机制，不直接展示。
- `message.reasoning` 是面向用户的简洁行动思路，不是模型原始 Chain of Thought。
- `message.milestone` 是有业务意义的阶段成果，不是每个工具调用后的流水账。
- `run.progress` 是可替换的实时状态，不进入长期对话时间线。
- `message.final` 是用户可读的最终回答，`run.finished` 是机器可判定的唯一终态。
- `update_plan`、`ask_user`、`finish` 是 Runtime 控制动作，不显示为普通工具卡片。
- chat、simple、complex 都只能由显式 `finish` 经 Completion Gate 接受后成功退出；普通文本不是隐式 finish。
- 本次直接替换旧协议，不保留旧事件名称、旧前端分支或双写逻辑。

## 2. 当前代码设计读取结论

### 2.1 当前 Runtime

当前后端已经从固定 PEV 收敛为：

```text
Context Engineering
        ↓
Strategy Router
        ├── chat
        ├── simple
        └── complex + optional plan
        ↓
Single ReAct Loop
        ↓
Completion Gate
        ↓
Commit
```

已有的关键能力包括：

- `talk/ask/execute` 交互意图；
- `chat/simple/complex` 执行策略；
- 单一 ReAct Loop；
- Complex 动态 Plan；
- 动态工具披露；
- staging 与 commit；
- Evidence Ledger 与 Completion Gate；
- SSE 事件持久化、连续 seq 和 Last-Event-ID 续传；
- `ask_user` 的暂停与继续。

这些内部能力应继续保留，但不应全部成为前端事件组件。

### 2.2 当前公共事件的问题

当前代码公开 17 种事件：

```text
run.started
context.assembled
strategy.selected
phase.changed
plan.created
plan.updated
tool.called
tool.completed
target.staged
evidence.recorded
completion.checked
target.committed
status.summary
run.completed
run.failed
run.canceled
needs_input
```

主要问题：

- 内部 Trace 与用户界面协议混在同一个 EventType 中。
- 前端需要理解 Runtime 的 context、strategy、phase、staging 和 completion 概念。
- `plan.created` 与 `plan.updated` 是同一资源的不同动作，增加了无价值分支。
- `run.completed/run.failed/run.canceled` 可以由一个终态事件表达。
- `status.summary` 与 `run.completed.outcome` 在前端形成重复结果。
- `needs_input` 只有问题，没有权威的已回答事件。
- `tool.called` 暴露原始参数，`tool.completed` 暴露完整 observation，产品界面过于技术化。
- 当前 `ToolCallResponse.Text` 在带工具调用时没有进入时间线，也没有完整保留在下一轮上下文中。
- 当前没有真正生产 `message.reasoning` 和 `message.milestone` 的机制。

### 2.3 当前前端的问题

当前右侧 Agent Panel 已经具备 Thread Tabs、Run Summary、Timeline、Plan、工具、提问和最终结果组件，
但事件层级不统一：

- Context、Strategy、Completion Gate 被做成产品可见状态。
- Markdown、工具卡、Artifact 卡、Final 卡、Question 卡和错误卡并列，形成 Card Soup。
- 成功的工具调用仍占用较大的卡片高度。
- 原始 args、observation、路径和诊断 JSON 默认可展开，偏向开发控制台。
- `ThinkingBubble` 与顶部运行状态重复。
- Timeline 每次更新都会强制平滑滚到底部，用户阅读历史时会被抢走位置。
- Question 使用 warning 色，容易被理解为错误或风险，而不是正常协作。
- Question 回答后仅靠组件本地 state 显示“已回答”，刷新和断线重放不可靠。
- `status.summary` 和 `run.completed` 可能展示两份最终结果。

本设计只完善右侧事件体验，不改变三栏布局、侧栏伸缩、顶部工作区 Tab、Thread Tabs 和输入区组织。

## 3. 市面产品带来的设计启发

### 3.1 Codex

Codex App Server 区分 Turn 生命周期、Plan 更新、Agent Message、Reasoning、Command/File Change 等
语义 Item，同时把 `rawResponse/completed` 明确标为 internal-only。Plan 更新使用最新快照，
而不是要求客户端拼接计划变更。

参考：
[Codex App Server protocol](https://github.com/openai/codex/blob/main/codex-rs/app-server/README.md)

对本项目的启发：

- 生命周期、计划、消息、工具应该是不同语义域。
- 原始模型事件与产品事件必须分层。
- Plan 应发权威完整快照。
- Reasoning 可以存在，但必须是可公开的摘要层，而不是原始推理。

### 3.2 Claude Managed Agents

Claude Managed Agents 区分持久化的权威 Agent Message 与仅用于实时预览的 Delta；开发者 Trace
和最终用户事件也有不同可见范围。Thinking 可以只作为“正在思考”的信号，不要求把隐藏推理暴露给用户。

参考：
[Claude Managed Agents events and streaming](https://platform.claude.com/docs/en/managed-agents/events-and-streaming)

对本项目的启发：

- 权威完成事件比打字机 Delta 更重要。
- 当前 MVP 没有必要为打字机效果引入 `message.delta`。
- Runtime Trace 应进入开发者观测面，不应塞入业务对话。

### 3.3 Kimi

Kimi Wire 将 Status、Content、Tool Call/Result、Plan Display、Question Request/Response 分开；
Question 使用结构化选项、单选或多选，并要求客户端显式支持后才披露提问能力。

参考：
[Kimi Wire Mode](https://moonshotai.github.io/kimi-cli/en/customization/wire-mode.html)

对本项目的启发：

- 提问和回答必须成对建模。
- 选项要有稳定 ID、标题和可选说明。
- Tool Result 应提供用户可读 display 数据，而不是要求 UI 理解原始工具输出。

### 3.4 Trae Agent

Trae Agent 同时提供面向使用者的简洁步骤总结和面向调试分析的完整 Trajectory Recording。

参考：
[Trae Agent](https://github.com/bytedance/trae-agent)

对本项目的启发：

- `message.milestone` 与内部 Trace 是两套不同粒度的产品。
- “专业”不等于把所有内部日志显示在界面上，而是既有简洁体验，也有完整可追踪底座。

### 3.5 DeepSeek

DeepSeek Thinking Mode 将 `reasoning_content` 与普通 `content` 分开；在工具调用场景下，
`reasoning_content` 还可能需要作为模型连续推理的内部上下文传回 API。

参考：
[DeepSeek Thinking Mode](https://api-docs.deepseek.com/guides/thinking_mode/)

对本项目的启发：

- Provider 的 `reasoning_content` 是模型协议数据，不等于产品的 `message.reasoning`。
- 即使内部需要保存并回传模型推理字段，也禁止直接投影到前端。
- 产品 Reasoning 应来自模型明确生成的公开行动说明，或由安全摘要层生成。

## 4. 事件分层

### 4.1 公共事件

公共事件进入：

- Run Event Store；
- SSE；
- 前端状态管理；
- 必要时进入对话 History。

公共事件必须满足：

- 用户可理解；
- 前端有明确消费方式；
- 可以断线重放；
- 不包含隐私、密钥、原始 HTML、完整工具结果或隐藏推理；
- 不要求前端理解 Runtime 内部实现。

### 4.2 内部 Trace

以下信息只进入 Trace Store、结构化日志或开发者诊断接口：

```text
context.assembled
strategy.selected
phase.changed
target.staged
target.committed
evidence.recorded
completion.checked
raw model request/response
provider reasoning_content
full tool arguments/results
token usage and latency
context compaction details
router signals
budget counters
checkpoint internals
```

内部 Trace 可以拥有比公共事件更高的保真度，但必须有访问控制，不能通过普通 SSE 泄漏。

### 4.3 目标架构

```text
                    ┌──────────────────────────────┐
                    │ Single ReAct Runtime         │
                    │ Context / Plan / Tools / Gate│
                    └──────────────┬───────────────┘
                                   │
                    ┌──────────────┴───────────────┐
                    │ Runtime Observer             │
                    └──────────────┬───────────────┘
                                   │
              ┌────────────────────┴────────────────────┐
              │                                         │
    ┌─────────▼─────────┐                    ┌──────────▼──────────┐
    │ Public Projector  │                    │ Trace Recorder       │
    │ safe, semantic    │                    │ full, diagnostic     │
    └─────────┬─────────┘                    └──────────┬──────────┘
              │                                         │
    ┌─────────▼─────────┐                    ┌──────────▼──────────┐
    │ Event Bus / Store │                    │ Trace Store / Logs   │
    │ SSE / History     │                    │ Developer-only       │
    └─────────┬─────────┘                    └─────────────────────┘
              │
    ┌─────────▼─────────┐
    │ Right Agent Panel │
    └───────────────────┘
```

Runtime 可以在明确位置发布公共语义，但不能继续把同一份内部 payload 原样发给 Bus。

## 5. 公共事件通用信封

SSE 使用：

```text
id: <run 内连续递增 seq>
event: <事件名>
data: <JSON payload>
```

每个 payload 具有共同字段：

```json
{
  "schema_version": 1,
  "run_id": "run_123",
  "occurred_at": "2026-08-02T10:30:00.000Z"
}
```

规则：

- `id` 使用当前连续 seq，继续支持 Last-Event-ID。
- `schema_version` 是整数，MVP 固定为 `1`。
- `occurred_at` 使用 RFC 3339 UTC 时间。
- Event Store 先持久化再扇出。
- 同一 Run 的公共事件严格有序。
- 未知事件由前端忽略并在开发环境记录 warning。
- payload 校验失败的事件不进入产品状态，但要进入前端诊断日志。
- 本次不接受旧事件和新事件双写。

## 6. Run 事件

### 6.1 `run.started`

含义：Run 已被 Runtime 接受并开始执行。

示例：

```json
{
  "schema_version": 1,
  "run_id": "run_123",
  "occurred_at": "2026-08-02T10:30:00.000Z",
  "target": {
    "artifact": "presentation",
    "level": "deck"
  },
  "interaction": {
    "intent": "execute"
  },
  "user_input": "生成一份 12 页的产品发布会 PPT"
}
```

规则：

- 每个 Run 恰好一次，且必须是第一条公共事件。
- 用于恢复用户 Turn 和初始化 Run Session。
- 不展示为 Agent 时间线卡片。
- 前端已经乐观插入 user turn 时，应通过 run ID 或本地 client ID 去重。

### 6.2 `run.progress`

含义：Run 当前正在做什么，是一个可替换的实时状态。

示例：

```json
{
  "schema_version": 1,
  "run_id": "run_123",
  "occurred_at": "2026-08-02T10:30:03.000Z",
  "stage": "rendering",
  "text": "正在检查第 6 页的布局",
  "target": {
    "type": "slide",
    "slide_id": "slide-06"
  },
  "progress": {
    "current": 6,
    "total": 12,
    "unit": "slide"
  }
}
```

`stage` 只能是：

```text
thinking
planning
reading
writing
rendering
finalizing
```

规则：

- 同一 Run 可以发多次。
- 前端只保留最新一条，不追加为长期 Timeline Item。
- Event Store 保留以支持 SSE seq 和断线重放。
- History 不持久化该事件。
- 只有真实可计算时才发送 `current/total`，禁止伪造百分比。
- 相同 stage、target 和 text 的连续事件应去重或节流。
- `phase.changed` 不再公开；Public Projector 根据 Runtime 状态和工具语义生成进度。

建议生成时机：

- 首次模型调用前：`thinking`；
- Complex 首次规划时：`planning`；
- `search_refs/read_ppt` 前：`reading`；
- `write_ppt/edit_ppt` 前：`writing`；
- `render_slide` 前：`rendering`；
- Completion Gate 与 Commit 前：`finalizing`。

### 6.3 `run.finished`

含义：Run 进入唯一终态。

成功示例：

```json
{
  "schema_version": 1,
  "run_id": "run_123",
  "occurred_at": "2026-08-02T10:31:42.000Z",
  "status": "completed",
  "affected_targets": [
    {
      "type": "global"
    },
    {
      "type": "slide",
      "slide_id": "slide-01"
    }
  ],
  "duration_ms": 102000
}
```

失败示例：

```json
{
  "schema_version": 1,
  "run_id": "run_123",
  "occurred_at": "2026-08-02T10:31:42.000Z",
  "status": "failed",
  "error": {
    "code": "RENDER_FAILED",
    "message": "页面渲染未通过，请调整内容后重试。",
    "retryable": true
  },
  "duration_ms": 102000
}
```

`status` 只能是：

```text
completed
failed
canceled
```

规则：

- 每个 Run 恰好一次。
- 发送后禁止再发任何公共事件。
- 成功时必须先发送 `message.final`，再发送 `run.finished`。
- 失败或取消可以没有 `message.final`，错误由 `run.finished.error` 提供。
- `run.finished` 驱动 Session 终态、SSE 关闭和目标刷新。
- 成功时前端不额外展示“运行已完成”卡片，避免与 `message.final` 重复。
- 失败或取消时前端展示一个紧凑终态 Notice。

## 7. Plan 事件

### 7.1 `plan.updated`

含义：Complex Run 的当前计划权威快照。

示例：

```json
{
  "schema_version": 1,
  "run_id": "run_123",
  "occurred_at": "2026-08-02T10:30:06.000Z",
  "plan": {
    "plan_id": "plan_123",
    "revision": 3,
    "explanation": "全局设计已确定，开始生成各页内容。",
    "steps": [
      {
        "id": "global-design",
        "title": "确定全局设计与叙事结构",
        "status": "completed"
      },
      {
        "id": "generate-slides",
        "title": "生成并渲染 12 张页面",
        "status": "in_progress"
      },
      {
        "id": "final-check",
        "title": "完成整份一致性检查",
        "status": "pending"
      }
    ]
  }
}
```

规则：

- 删除 `plan.created`，首次计划也是 `plan.updated`。
- payload 每次携带完整快照，不发送 patch。
- `revision` 单调递增。
- 至多一个 step 为 `in_progress`。
- completed step 不能回退或被删除。
- Chat 与 Simple 不发送计划。
- Plan 是 Agent 工作记忆，不是 Workflow DAG。
- 前端以 `plan_id` upsert，永远只显示最新 revision。

## 8. Message 事件

### 8.1 三类消息的职责

| 事件 | 回答的问题 | 生产者 | 是否持久化 |
| --- | --- | --- | --- |
| `message.reasoning` | 为什么接下来这样做 | Agent 的公开行动说明 | 是 |
| `message.milestone` | 刚刚完成了什么有意义的阶段 | Runtime 的计划差异投影 | 是 |
| `message.final` | 最终给用户什么结论或结果 | Agent 的 finish 内容 | 是 |

三者不是同一条 Message 加 kind，而是三个直接事件类型。

### 8.2 `message.reasoning`

含义：Agent 面向用户解释下一步动作或重要取舍。

示例：

```json
{
  "schema_version": 1,
  "run_id": "run_123",
  "occurred_at": "2026-08-02T10:30:07.000Z",
  "message_id": "msg_01",
  "text": "我会先读取全局蓝图和设计规范，确认整份叙事与视觉约束，再开始逐页生成。"
}
```

严格边界：

- 不是原始 Chain of Thought。
- 不得读取或直接转发 provider 的 `reasoning_content`。
- 不包含隐式安全策略、密钥、系统提示词、完整上下文或内部评分。
- 不描述每一个显而易见的工具动作。
- 推荐 1 句，最多 2 句，中文建议不超过 160 字。
- 每个 LLM turn 最多一条。
- 内容为空或与上一条高度重复时不发送。

生产机制：

1. System Prompt 要求模型在重要工具调用前，可以通过普通 assistant `content`
   给出一条简洁的公开行动说明。
2. Provider Adapter 将普通 `content` 与 `tool_call` 一起返回。
3. Runtime 在执行工具或控制动作前，把非空、通过安全检查的 `content`
   发布为 `message.reasoning`。
4. 同一段普通 `content` 必须连同 `tool_call` 一起追加回 ReAct 上下文。
5. Provider 的隐藏 reasoning 字段只用于模型连续性和内部 Trace，永不进入公共事件。
6. 当模型没有给出公开说明时，不额外调用一次 LLM 补写，允许该事件缺省。

不应该发送的例子：

```text
我现在要调用 read_ppt。
我需要一步一步思考。
根据系统提示词，我判断风险为 medium。
```

应该发送的例子：

```text
这次修改会影响整份视觉一致性，我先确认全局样式，再处理当前页面。
渲染结果显示标题区拥挤，我会收紧信息层级并重新检查。
```

### 8.3 `message.milestone`

含义：Complex 长任务完成了一个用户能理解的阶段成果。

示例：

```json
{
  "schema_version": 1,
  "run_id": "run_123",
  "occurred_at": "2026-08-02T10:30:32.000Z",
  "message_id": "msg_02",
  "text": "全局设计与 12 页叙事结构已经确定。",
  "completed_step_ids": [
    "global-design"
  ]
}
```

生产机制：

- 由 Runtime 比较相邻两次 `plan.updated` 快照生成，不依赖额外 LLM 调用。
- 仅当一个或多个 Plan Step 首次从未完成进入 `completed` 时生成。
- 优先使用本次 Plan Update 的 `explanation` 作为自然语言；为空时根据完成的 step title 生成。
- 同一 plan revision 至多一条。
- 多个 step 同时完成时合并为一条。
- 计划首次创建不自动产生 milestone，Plan Card 已经表达“计划已建立”。
- 最后一个 step 完成后如果紧接 `message.final`，可以抑制重复 milestone。
- Chat 和通常的 Simple 不发送 milestone。

不应该触发 milestone 的情况：

- 完成一次 `read_ppt`；
- 完成一次 `render_slide`；
- Context 已装配；
- Strategy 已选择；
- Completion Gate 执行了一次；
- staged 文件已提交；
- 计划仅修改了措辞，没有新完成 step。

### 8.4 `message.final`

含义：本次 Run 的最终用户可读回答。

示例：

```json
{
  "schema_version": 1,
  "run_id": "run_123",
  "occurred_at": "2026-08-02T10:31:41.000Z",
  "message_id": "msg_03",
  "text": "整份 12 页演示文稿已经生成并完成渲染检查。整体采用深蓝科技风格，叙事从市场机会推进到产品方案与落地路径。",
  "affected_targets": [
    {
      "type": "global"
    },
    {
      "type": "slide",
      "slide_id": "slide-01"
    }
  ]
}
```

规则：

- Chat 成功结束时恰好一条。
- Simple/Complex 成功通过 Completion Gate 并完成 Commit 后恰好一条。
- 内容来自 `finish.message`，为空时 Runtime 生成安全兜底文本。
- Chat 的普通 assistant 文本没有 tool call 时，Runtime 必须要求 Agent 继续调用 `finish(message=...)`，不能把该文本自动转换为 finish candidate。
- 必须在成功的 `run.finished` 之前发送。
- 前端将它作为最终回答，不再同时渲染一份 `run.finished.outcome` 卡片。
- 支持受控 Markdown，不允许原始 HTML。
- `affected_targets` 只放领域目标，不放文件路径、hash、staging 路径。

## 9. Tool 事件

### 9.1 只公开业务工具

前端只展示以下工具：

```text
read_ppt
write_ppt
edit_ppt
search_refs
render_slide
```

以下控制动作永远不显示为普通工具：

```text
update_plan → plan.updated
ask_user   → question.asked / question.answered
finish     → message.final / run.finished
```

### 9.2 `tool.started`

含义：一个业务工具开始执行。

示例：

```json
{
  "schema_version": 1,
  "run_id": "run_123",
  "occurred_at": "2026-08-02T10:30:08.000Z",
  "call_id": "call_01",
  "tool": "read_ppt",
  "plan_step_id": "global-design",
  "target": {
    "type": "global"
  },
  "display": {
    "label": "读取全局蓝图",
    "detail": "确认叙事结构与设计约束"
  }
}
```

规则：

- `call_id` 在 Run 内唯一。
- `display` 由后端 Tool Public Projector 生成，不依赖前端解释原始 args。
- 不发送完整 args。
- 只允许必要的领域目标和安全查询摘要。
- `plan_step_id` 可选，用于长任务工具分组。

### 9.3 `tool.completed`

含义：对应业务工具成功或失败。

成功示例：

```json
{
  "schema_version": 1,
  "run_id": "run_123",
  "occurred_at": "2026-08-02T10:30:09.000Z",
  "call_id": "call_01",
  "tool": "read_ppt",
  "status": "completed",
  "display": {
    "label": "已读取全局蓝图",
    "detail": "已获得叙事结构与设计规范"
  }
}
```

渲染示例：

```json
{
  "schema_version": 1,
  "run_id": "run_123",
  "occurred_at": "2026-08-02T10:31:10.000Z",
  "call_id": "call_12",
  "tool": "render_slide",
  "status": "completed",
  "display": {
    "label": "第 6 页渲染通过",
    "detail": "未发现溢出或裁切"
  },
  "preview": {
    "slide_id": "slide-06",
    "image_url": "/api/v1/runs/run_123/previews/slide-06",
    "warnings": []
  }
}
```

失败示例：

```json
{
  "schema_version": 1,
  "run_id": "run_123",
  "occurred_at": "2026-08-02T10:31:10.000Z",
  "call_id": "call_12",
  "tool": "render_slide",
  "status": "failed",
  "display": {
    "label": "第 6 页渲染未通过",
    "detail": "标题区域发生裁切"
  },
  "error": {
    "code": "RENDER_FAILED",
    "message": "标题区域发生裁切",
    "retryable": true
  }
}
```

规则：

- 必须与一个 `tool.started.call_id` 匹配。
- `status` 只能是 `completed` 或 `failed`。
- 前端根据 `call_id` 更新同一个 Tool Item，不追加第二张卡。
- 不发送完整 observation、HTML、schema、日志、hash 或本地文件路径。
- `preview.image_url` 必须是受控 HTTP 资源，不是本机文件路径。
- 工具失败不等于 Run 失败；ReAct 可以继续修正。

### 9.4 Tool Display Projector

后端负责把工具名和受控参数转换为产品文案：

| 工具 | started 文案示例 | completed 文案示例 |
| --- | --- | --- |
| `read_ppt` | 读取第 3 页 | 已读取第 3 页 |
| `write_ppt` | 生成第 3 页 | 已生成第 3 页 |
| `edit_ppt` | 编辑第 3 页 | 已更新第 3 页 |
| `search_refs` | 查找品牌设计参考 | 已找到 4 条相关参考 |
| `render_slide` | 检查第 3 页布局 | 第 3 页渲染通过 |

前端不得内置另一套工具业务文案作为主逻辑。未知工具可以显示后端 `display.label`，
开发环境额外记录 raw tool name。

## 10. Question 事件

### 10.1 `question.asked`

含义：Agent 缺少一个必须由用户决定的信息，Run 暂停等待。

示例：

```json
{
  "schema_version": 1,
  "run_id": "run_123",
  "occurred_at": "2026-08-02T10:30:10.000Z",
  "question_id": "question_01",
  "header": "视觉方向",
  "prompt": "这份发布会更适合哪一种视觉气质？",
  "selection": "single",
  "options": [
    {
      "id": "technology",
      "label": "克制科技",
      "description": "深色背景、精细线条和高密度数据表达"
    },
    {
      "id": "editorial",
      "label": "现代编辑",
      "description": "大字号排版、留白和少量高质图片"
    }
  ],
  "allow_custom": true
}
```

规则：

- `selection` 为 `single` 或 `multiple`。
- Option 必须有稳定 `id` 和 `label`，`description` 可选。
- `header` 可选，建议不超过 12 个中文字符。
- 同一 Run 同时只允许一个 pending question。
- 发布后 Run 状态转为 waiting。
- 只有客户端支持结构化 Question 时才向模型披露 `ask_user`。

### 10.2 `question.answered`

含义：后端已经接受与 pending question 匹配的回答。

示例：

```json
{
  "schema_version": 1,
  "run_id": "run_123",
  "occurred_at": "2026-08-02T10:30:22.000Z",
  "question_id": "question_01",
  "answer": {
    "selected_option_ids": [
      "technology"
    ],
    "custom_text": ""
  },
  "display_text": "克制科技"
}
```

规则：

- 只在后端通过 `reply_to` 校验并接受输入后发送。
- 前端根据 `question_id` 更新原 Question Item。
- 回答状态不再依赖 React 组件本地 state。
- 断线重放后必须能恢复“已回答”状态。
- 发出后 Run 从 waiting 回到 running，并继续同一 ReAct Loop。

## 11. 事件顺序与不变量

### 11.1 普通 Simple Run

```text
run.started
run.progress
message.reasoning?
tool.started
tool.completed
run.progress
message.reasoning?
tool.started
tool.completed
run.progress
message.final
run.finished
```

### 11.2 Complex Run

```text
run.started
run.progress
message.reasoning?
plan.updated
run.progress
message.reasoning?
tool.started
tool.completed
plan.updated
message.milestone
...
message.final
run.finished
```

### 11.3 需要提问

```text
message.reasoning?
question.asked
    Run waiting
question.answered
    Run running
run.progress
...
```

### 11.4 终态不变量

- `run.started` 恰好一次且最先。
- `run.finished` 恰好一次且最后。
- `run.finished.status=completed` 前必须有且只有一条 `message.final`。
- 每个 `tool.completed` 必须匹配 `tool.started`。
- 每个 `question.answered` 必须匹配未回答的 `question.asked`。
- Plan revision 单调递增。
- 终态后 Bus 静默拒绝新事件。
- SSE 重放不应产生重复 Timeline Item。

## 12. 旧事件的归宿

| 旧事件 | 新归宿 |
| --- | --- |
| `run.started` | 保留为 `run.started` |
| `context.assembled` | Internal Trace |
| `strategy.selected` | Internal Trace |
| `phase.changed` | Internal Trace，必要状态投影为 `run.progress` |
| `plan.created` | 合并到 `plan.updated` |
| `plan.updated` | 保留为 `plan.updated` |
| `tool.called` | 改为 `tool.started` |
| `tool.completed` | 保留名称但替换为安全公共 payload |
| `target.staged` | Internal Trace |
| `evidence.recorded` | Internal Trace |
| `completion.checked` | Internal Trace |
| `target.committed` | Internal Trace，目标摘要进入 final/finished |
| `status.summary` | 改为 `message.final` |
| `run.completed` | 合并到 `run.finished` |
| `run.failed` | 合并到 `run.finished` |
| `run.canceled` | 合并到 `run.finished` |
| `needs_input` | 改为 `question.asked` |
| 无 | 新增 `question.answered` |
| 无 | 新增 `message.reasoning` |
| 无 | 新增 `message.milestone` |
| 无 | 新增 `run.progress` |

## 13. SSE、History 与恢复

### 13.1 Run Event Store

11 种公共事件全部进入 Run Event Store，以保持：

- 连续 seq；
- Last-Event-ID；
- SSE 重连；
- Run 级事件审计。

`run.progress` 虽然不进入对话 History，仍进入 Run Event Store。前端重放时不断覆盖当前 progress，
最终只保留最后一条。

### 13.2 Thread History

History 保存用户未来打开任务时需要恢复的产品内容：

```text
user_turn
plan.updated
message.reasoning
message.milestone
message.final
tool.started
tool.completed
question.asked
question.answered
run.finished
```

`run.progress` 不写入 History。

内部 Trace 不写入普通 History。

History 中的工具记录只保存公共安全 payload，不保存原始 args/result。

### 13.3 恢复规则

- History Hydrator 与 SSE Reducer 必须共享同一组领域 reducer 或同一数据模型。
- `tool.completed` upsert 已存在 call。
- `question.answered` upsert 已存在 question。
- `plan.updated` 覆盖旧 revision。
- `run.finished` 决定 Session 终态。
- 重放 `message.final + run.finished` 只展示一份最终回答。
- 前端 processed event id 继续用于实时去重。

## 14. 前端 Design Read

### 14.1 产品类型

这是一个专业 HTML PPT 业务 Agent 工作台，不是通用聊天机器人，也不是开发者 Trace 控制台。

### 14.2 保留边界

必须保留：

- 左侧 Deck Navigator；
- 中间 PPT 工作区；
- 右侧 Agent Panel；
- 左右侧栏独立展开、收起和拖动；
- 顶部 Project Tabs；
- 右侧 Thread Tabs；
- 底部 Composer 及其 talk/ask/execute 入口；
- 当前 Office/WPS 式克制工作区气质。

不进行：

- 页面大改版；
- 三栏结构重构；
- 新导航体系；
- 全新颜色系统；
- 落地页式大标题和装饰图形；
- 每种事件一种颜色；
- 大量浮动卡片和阴影。

### 14.3 视觉参数

```text
DESIGN_VARIANCE = 4
MOTION_INTENSITY = 3
VISUAL_DENSITY = 6
```

设计目标：

- 接近 Office 的秩序；
- 接近成熟 Coding Agent 的低噪声活动时间线；
- 适合长时间工作；
- 运行状态清楚，但不抢 PPT 画布注意力；
- 信息密度较高，但层级稳定。

### 14.4 视觉语言

继续使用当前 token：

```text
panel       #F8F9FB
surface     #FFFFFF
ink         #17202B
muted       #5F6B7A
subtle      #95A0AE
border      #D5DBE3
accent      #2F67F6
success     #2F7D65
warning     #B96D1F
danger      #C84953
```

规则：

- Accent 仍然只有蓝色。
- Success、Warning、Danger 只表达真实语义状态。
- Question 不使用大面积 Warning Soft。
- 工具成功后尽量回到中性灰，不让整条时间线布满绿色。
- 继续使用项目已安装的 Lucide React，不引入第二套图标库，不手绘 SVG。
- 图标统一使用 16px、`strokeWidth=1.75`。
- 圆角收敛为 8px 或 10px，避免同一区域出现过多圆角等级。
- 右侧时间线默认无阴影，只有需要用户操作的 Question 可以使用极轻层级。

## 15. 右侧 Agent 时间线总体结构

右侧面板保持：

```text
┌────────────────────────────┐
│ Agent Header               │
├────────────────────────────┤
│ Thread Tabs                │
├────────────────────────────┤
│ Run Summary                │
├────────────────────────────┤
│                            │
│ User Turn                  │
│                            │
│ Plan                       │
│ Reasoning                  │
│ Tool Activity              │
│ Milestone                  │
│ Question                   │
│ Final                      │
│                            │
│ Live Progress              │
│                            │
├────────────────────────────┤
│ Composer                   │
└────────────────────────────┘
```

核心展示原则：

- Timeline 不是一堆等权卡片。
- 用户消息继续右对齐。
- Agent 活动共享一列 16px 图标和一列正文，形成稳定的“安静活动列”。
- Reasoning、Tool、Milestone 是不同信息强度，但不各自包成大卡。
- Plan 与 Question 因为需要结构和交互，可以使用边框容器。
- Final 是普通 Agent 回答，不再做重复成功卡。

## 16. 事件到组件的映射

| 事件 | 前端位置 | 组件 | 是否进入时间线 |
| --- | --- | --- | --- |
| `run.started` | Run Store | 无独立组件 | 否 |
| `run.progress` | Timeline 底部临时区 | `LiveProgressRow` | 临时，不持久 |
| `run.finished` | Run Summary / terminal notice | `TerminalNotice` | 仅失败或取消 |
| `plan.updated` | 当前用户 Turn 下方 | `PlanPanel` | 独立固定一份 |
| `message.reasoning` | Agent 活动列 | `ReasoningRow` | 是 |
| `message.milestone` | Agent 活动列 | `MilestoneRow` | 是 |
| `message.final` | Agent 回答区 | `FinalMessage` | 是 |
| `tool.started` | Agent 活动列 | `ToolActivityRow` | 是，upsert |
| `tool.completed` | Agent 活动列 | `ToolActivityRow` | 是，upsert |
| `question.asked` | Agent 活动列 | `QuestionPanel` | 是，upsert |
| `question.answered` | Agent 活动列 | `QuestionPanel` | 是，upsert |

## 17. 各组件视觉规格

### 17.1 Run Summary

保留当前顶部位置，但只展示产品信息：

- 当前目标；
- `运行中/等待回答/已完成/失败/已取消`；
- 有真实 Plan 时显示 `2/4`；
- 重连状态；
- 运行中显示停止按钮。

删除：

- Strategy 名称；
- Runtime Phase 名称；
- risk、confidence；
- context profile；
- completion gate 状态。

高度保持紧凑。运行完成后可以在短时间内保留，也可以在重新进入 Thread 时显示最近终态，
但不与 Final Message 重复。

### 17.2 Live Progress Row

替换当前 `ThinkingBubble`：

```text
◌  正在检查第 6 页的布局                 6 / 12
```

样式：

- 左侧 14px Loader；
- 12px muted 文本；
- 无气泡、无边框、无背景；
- 位于 Timeline 当前末尾；
- `aria-live="polite"`；
- 新 progress 原位替换，不追加；
- question pending 时隐藏，Question Panel 本身已表达等待。

### 17.3 User Turn

保留右对齐气泡：

- 最大宽度 88%；
- `panel-muted` 背景；
- 1px border；
- 10px 圆角；
- target label 作为 10px 次级信息；
- 不添加头像。

### 17.4 Plan Panel

Plan 仍然是结构化容器，但降低卡片感：

- 无 shadow；
- 1px border；
- 10px 圆角；
- Header 高度 36px；
- 当前 step 使用 `accent-soft` 的轻底色；
- completed 使用 CheckCircle2，不使用删除线，避免可读性下降；
- pending 使用 Circle；
- failed 使用 XCircle；
- 运行中默认展开，完成后默认收起；
- 标题右侧显示 `2/4`；
- revision 更新原位变化，不重复插入。

计划首次出现时使用 120ms opacity + translateY(2px)；后续 revision 只更新内容，不整体闪动。

### 17.5 Reasoning Row

示意：

```text
◈  我会先读取全局蓝图和设计规范，确认整份叙事与视觉约束。
```

样式：

- 图标：`BrainCircuit`；
- 图标颜色：`text-400`；
- 正文：13px、`text-600`、行高 1.55；
- 无 label、无背景、无边框；
- 与相邻 Tool Row 的垂直间距 6px；
- 允许受控 Markdown，但不显示代码块；
- 超过 4 行时默认折叠，提供“展开思路”；
- 文案禁止展示“内部推理”“思维链”等字样。

它应比 Final 弱，比 Live Progress 稳定，比 Tool 更像自然语言。

### 17.6 Tool Activity Row

单个工具示意：

```text
◌  正在生成第 6 页
✓  已生成第 6 页
×  第 6 页生成失败                         查看
```

样式：

- 高度 32 至 36px；
- 无外层卡片；
- running 使用 Loader2 + accent；
- completed 使用 CheckCircle2，但正文保持中性色；
- failed 使用 XCircle + danger；
- label 13px；
- detail 12px muted；
- started 与 completed 通过 call ID 原位更新；
- running 可以显示轻微 accent 背景，完成后背景消失；
- 失败时展开一行用户可读错误和重试提示；
- 默认不展示 raw tool name、args、observation JSON。

`render_slide` 成功且有 preview 时，展开区域可以显示：

- 16:9 小缩略图；
- 页面编号；
- 是否有 overflow/clipping warning；
- 点击缩略图聚焦中间预览工作区。

不得显示本地 screenshot path。

### 17.7 Tool 分组

整份 PPT 可能产生大量页面工具调用，连续成功项需要折叠：

```text
✓  已生成 8 张页面                         展开
```

分组规则：

- 相同 `plan_step_id`；
- 相同 tool；
- 连续且 completed；
- 数量大于等于 3。

展开后显示每个页面的紧凑行。

以下项不得折叠隐藏：

- running 项；
- failed 项；
- 带 warning 的 render 项；
- 当前用户正在查看的页面项。

### 17.8 Milestone Row

示意：

```text
✓  全局设计与 12 页叙事结构已经确定。
```

样式：

- 图标：`CheckCircle2`；
- 图标颜色：success；
- 正文：13px、medium、`text-900`；
- 可选 detail：12px、`text-600`；
- 使用上方 1px 淡 border 或更大的 12px top spacing 形成阶段分隔；
- 不使用完整卡片；
- 新出现时 140ms 淡入；
- 不显示“里程碑”标签，事件语义由样式表达。

### 17.9 Question Panel

Question 是整个 Timeline 中最强的交互组件：

```text
┌────────────────────────────┐
│ ? 需要你的选择             │
│ 这份发布会更适合哪种风格？ │
│                            │
│ ○ 克制科技                 │
│   深色背景与高密度数据     │
│ ○ 现代编辑                 │
│   大字号排版与更多留白     │
│                            │
│ [补充你的想法……]   [提交]  │
└────────────────────────────┘
```

样式：

- surface 背景；
- border-strong；
- 10px 圆角；
- 左上使用 `CircleHelp` 或 `MessageCircleQuestion`；
- 不使用大面积 warning-soft；
- Header 12px medium；
- Prompt 14px medium；
- 有 description 的 option 使用整行 radio/checkbox，不做小 chip；
- 没有 description 且文案很短时可以使用紧凑 chip；
- 单选使用 Radio 语义，多选使用 Checkbox 语义；
- 提交按钮使用当前 accent；
- 自定义回答输入框沿用 Composer 的无聚焦外框方向，只保留 focus-visible ring；
- Enter 提交单行自定义内容，Shift+Enter 不适用单行输入；
- 提交中禁用重复操作。

回答后原位收起为：

```text
✓  你选择了：克制科技
```

并保留“查看问题”Disclosure。

Question pending 时：

- 焦点移到 Question Panel；
- Composer 保留位置但禁用；
- Composer 提示“请先回答上方问题”；
- 回答成功以 `question.answered` 为准，不以本地 state 为准。

### 17.10 Final Message

Final 是普通 Agent 回答，不做第二个“执行结果卡”：

```text
整份 12 页演示文稿已经生成并完成渲染检查。

整体采用深蓝科技风格，叙事从市场机会推进到产品方案与落地路径。

✓ 已更新全局设计和 12 张页面
```

样式：

- 左对齐；
- 无气泡或使用极轻透明背景；
- 正文 14px、`text-900`、行高 1.65；
- 支持 Markdown；
- affected targets 使用一行 12px success footer；
- 不显示“执行结果”“最终结果”大标题；
- 不重复展示 strategy、completion check、issue 列表；
- 与 Composer 之间保留 16px 底部空间。

### 17.11 Terminal Notice

仅用于 `run.finished.status=failed/canceled`：

- failed：左侧 AlertCircle，danger 文本，提供可读错误和重试提示；
- canceled：中性 StopCircle 文本“运行已取消”；
- 采用紧凑 Inline Notice；
- 技术错误码放入 Disclosure；
- 不展示 stack、原始 provider error 或请求体。

## 18. Timeline 状态模型

目标 Timeline Item：

```text
user_turn
reasoning
milestone
final
tool
question
terminal_notice
```

单独 Session State：

```text
run status
stream status
current progress
latest plan
pending question id
last event id
processed event ids
```

不再存在于产品 Timeline：

```text
context_status
strategy_status
completion_status
artifact
generic markdown
duplicate final_result
```

Reducer 规则：

- `message.*` 按 event id append。
- `tool.started` 创建，`tool.completed` 按 call ID upsert。
- `question.asked` 创建，`question.answered` 按 question ID upsert。
- `plan.updated` 更新 Session Plan，不进入普通 items 数组。
- `run.progress` 更新 Session Progress，不进入普通 items 数组。
- `run.finished` 更新终态，成功不追加 item，失败或取消追加 terminal notice。

## 19. 滚动、动效与无障碍

### 19.1 自动滚动

当前每次事件都强制 `scrollIntoView({behavior: smooth})`，应改为：

- 用户距离底部不超过 80px 时自动跟随。
- 用户主动向上滚动后停止自动跟随。
- 有新事件时在底部显示“回到最新”按钮。
- 点击后滚到底部并恢复自动跟随。
- `run.progress` 高频更新不触发整页 smooth scroll。

### 19.2 动效

- 新稳定 item：120 至 160ms opacity + 2px translateY。
- Tool status 原位变化：150ms color/background。
- Loader 可以旋转。
- 禁止打字机动画、弹跳气泡、整卡缩放和大面积 shimmer。
- 遵循 `prefers-reduced-motion`。

### 19.3 无障碍

- `run.progress` 使用 `aria-live=polite`，但对重复状态节流。
- Question 使用 fieldset/legend、radio/checkbox 语义。
- 所有 icon-only button 有 aria-label 和 tooltip。
- Focus visible 继续使用现有 accent ring。
- 文本和背景满足 WCAG AA。
- 颜色不是唯一状态信号，必须同时有 icon 和文字。

## 20. 前后端适配范围

### 20.1 后端

需要完成：

- 用 11 个公共 EventType 替换旧 EventType。
- 引入 Public Event Payload 类型和校验。
- 将内部 Runtime Trace 与 Public Event Bus 分离。
- 实现 `run.progress` 的确定性投影与去重。
- 将普通 assistant content + tool call 投影为安全 `message.reasoning`。
- 在 ReAct 消息历史中保留该 content。
- 根据 Plan Diff 生成 `message.milestone`。
- 首次 Plan 也发送 `plan.updated`。
- Tool Public Projector 生成安全 display payload。
- 控制工具不发送 tool events。
- `ask_user` 发 `question.asked`，输入被后端接受后发 `question.answered`。
- Commit 成功后发 `message.final`，随后发唯一 `run.finished`。
- 失败和取消统一为 `run.finished`。
- 更新 Bus terminal 判断、History Projector、Checkpoint/Prompter 和 Engine fallback。
- 保留 SSE seq、持久化优先、Last-Event-ID 和一次终态保证。

Provider 注意事项：

- 普通 assistant `content` 与 provider 隐藏 reasoning 必须分开。
- 如果启用 DeepSeek Thinking Mode，内部消息模型要能按 provider 要求回传 `reasoning_content`。
- `reasoning_content` 不进入公共 Event、普通 History 或前端。

### 20.2 前端

需要完成：

- 将 SSE union 和 parser 收敛到 11 个事件。
- 重写 reducer、run store、history hydrator。
- 引入新的 Timeline Item 模型。
- 替换 ThinkingBubble 为 LiveProgressRow。
- 改造 PlanCard 为扁平 PlanPanel。
- 改造 ToolCallCard 为可 upsert、可分组的 ToolActivityRow。
- 改造 NeedsInputCard 为 QuestionPanel。
- 统一 FinalResultCard/FinishBubble 为 FinalMessage。
- 删除 ExecutionMetaRow、ArtifactCard 的产品时间线用途、CompletionBlocker。
- RunSummary 删除 strategy/phase/context 等内部信息。
- 实现近底部自动跟随和“回到最新”。
- 保留现有三栏、侧栏伸缩、顶部 Tab、Thread Tab 和 Composer 组织。

### 20.3 不在本次范围

- `message.delta` 或打字机流式文本；
- 多 Agent 子时间线；
- 通用审批事件；
- Tool args/result 开发者控制台 UI；
- Trace 查询后台；
- 资产仓库事件；
- 旧事件兼容或迁移桥接；
- 页面整体视觉重构。

## 21. 测试与验收

### 21.1 后端测试

- EventType 只有 11 个公共事件。
- `run.started` 第一条，`run.finished` 最后一条。
- 终态恰好一次，Engine fallback 不产生重复终态。
- 首次计划和后续计划都使用 `plan.updated`。
- Plan revision 单调递增。
- 新完成 step 只生成一次 milestone。
- 没有新完成 step 时不生成 milestone。
- 带工具调用的普通 content 生成 reasoning，并保留到后续 LLM context。
- provider reasoning 不会生成公共 reasoning。
- 控制工具不生成 tool events。
- 业务工具 started/completed 成对。
- Tool public payload 不包含 args、HTML、文件路径、hash 或完整 result。
- ask_user 生成 asked，合法回答生成 answered。
- 不匹配 reply 不生成 answered。
- success 顺序为 `message.final` 后 `run.finished`。
- failure/cancel 统一为 `run.finished`。
- public event replay 保持 seq 连续。
- `run.progress` 不写入 Thread History。
- 内部 Trace 事件不进入 SSE。

### 21.2 前端测试

- Parser 只注册 11 个事件。
- 未知事件被安全忽略。
- `tool.completed` 原位更新 started item。
- `question.answered` 原位更新 asked item。
- `plan.updated` 只保留最新 revision。
- `run.progress` 替换而不是追加。
- successful run 只显示一份 final。
- failed/canceled run 显示一个 terminal notice。
- context/strategy/phase/staging/evidence/completion 不再渲染。
- 控制工具没有工具行。
- 3 个以上连续成功工具可以折叠。
- running/failed/warning tool 不被折叠隐藏。
- pending question 禁用 Composer。
- answered 状态刷新后仍可恢复。
- 只有接近底部时自动跟随。
- 用户离开底部后出现“回到最新”。
- reduced motion 下没有持续动画。

### 21.3 视觉验收

- 三栏结构、展开收起、拖动宽度和顶部 Tab 完全保留。
- 右侧没有 Context/Strategy/Completion Gate 技术卡片。
- 成功工具默认是紧凑行，不是大卡片。
- Question 是唯一明显的交互容器。
- Reasoning、Milestone、Final 的信息强度明显不同。
- 同一屏幕只使用现有 accent，状态色不泛滥。
- Final 不与 Run Finished 重复。
- 长达 12 页的生成任务不会产生几十张等权卡片。
- 右侧在 20% 至 40% 可调宽度内都可读。

## 22. 完成定义

只有同时满足以下条件才算完成：

- 后端真实生产 11 个公共事件，而不只是修改常量。
- `message.reasoning` 有明确生产路径，且不是隐藏推理。
- `message.milestone` 有明确 Plan Diff 生产路径。
- Question 回答拥有后端权威事件。
- 公共 Event 与内部 Trace 已分层。
- SSE 重放、History 恢复和唯一终态继续成立。
- 前端组件已经按新层级展示，不再暴露内部 Runtime 节点。
- 现有页面组织和交互框架未被重构。
- 后端、前端、集成与视觉验收测试全部通过。
