# Agent Public Events and Timeline Design

**日期：** 2026-08-02
**状态：** Accepted，公共事件 Schema v3

## 1. 目标

后端拥有唯一运行事实，前端只消费稳定、低噪声、可恢复的公共投影。staging、Evidence
Ledger、Context Pack、strategy 内部判断、phase、trace、Provider reasoning 和 raw
Resource content 不进入公共事件。

公共事件严格只有 17 种：

```text
run.started
run.progress
run.completed
run.failed
run.error
run.canceled
plan.updated
plan.approval_requested
plan.approval_answered
run.mode_changed
message.reasoning
message.milestone
message.final
tool.started
tool.completed
question.asked
question.answered
```

不恢复旧 SSE 协议，不做旧/新终态事件双写。

## 2. 通用信封

SSE：

```text
id: <run 内连续递增 seq>
event: <事件名>
data: <JSON payload>
```

每个 payload 必须包含：

```json
{
  "schema_version": 3,
  "run_id": "run_123",
  "occurred_at": "2026-08-02T10:30:00.000Z"
}
```

- `id` 是 Run 内连续 seq，支持 Last-Event-ID。
- `occurred_at` 使用 RFC 3339 UTC。
- Event Store 先持久化再扇出。
- 同一 Run 严格有序。
- payload 校验失败不进入产品状态，但进入开发诊断。

## 3. Target 投影

`run.started.target` 使用公开任务模型：

```json
{
  "artifact": "spec",
  "level": "slide",
  "slide_id": "slide-03"
}
```

其中：

```text
artifact = spec | presentation
level    = deck | slide
```

Tool Event 和 affected target 使用 Resource 语义：

```json
{"type":"deck","part":"outline"}
{"type":"deck","part":"design"}
{"type":"slide","slide_id":"slide-03","part":"spec"}
{"type":"slide","slide_id":"slide-03","part":"html"}
```

公共 target 不包含路径、hash、revision、staging、数据库 ID 或 raw content。

## 4. Run 事件

### `run.started`

每个 Run 恰好一次且为第一条公共事件。payload 包含 `target`、`interaction` 和
`user_input`，用于恢复用户 Turn 与初始化 Session。

### `run.progress`

表示可替换实时状态。`stage` 只能是：

```text
thinking | planning | reading | writing | rendering | finalizing
```

可选 `target` 使用 Resource 投影，可选 `progress` 使用
`{current,total,unit}`。该事件不是持久时间线卡片。

### Run terminal events

每个 Run 恰好一次终态事件且为最后一条公共事件。终态不再使用
`run.finished + status`，而是直接从外层事件名区分：

```text
run.completed
run.failed
run.error
run.canceled
```

四个终态事件共用统一 payload：

```json
{
  "schema_version": 3,
  "run_id": "run_123",
  "occurred_at": "2026-08-02T10:30:00.000Z",
  "duration_ms": 12000,
  "affected_targets": [],
  "error": null,
  "trace_id": "run_123"
}
```

- `run.completed`：LLM 调用 finish 后，通过后端完成检查与提交校验。
- `run.failed`：运行逻辑正常收口，但被规则、预算、门禁或已知依赖失败判定为失败。
- `run.error`：工程运行时异常，Runtime 状态不可信或发生不可恢复后端错误。
- `run.canceled`：用户或系统取消。

`run.failed` 和 `run.error` 必须带安全的 `PublicError`；`run.completed` 和
`run.canceled` 的 `error` 为 `null`。`affected_targets` 只表达后端确认已经落实或能
定位的改动，不能表达仅尝试过的改动。`trace_id` 是安全排查索引，不包含内部路径、
Prompt、密钥或原始模型内容。

## 5. Plan 事件

### `plan.updated`

只在 Complex 策略使用，表示当前动态 Plan 的完整快照。Plan 是同一 ReAct Loop 内的
轻量检查表，不是 Workflow DAG。前端按 `plan_id + revision` 覆盖投影，不显示 Runtime
内部依赖或验证阶段。

## 6. Message 事件

### `message.reasoning`

只发送简短、用户可理解的当前意图或方向，不发送 Provider reasoning、chain-of-thought、
Prompt、路径、HTML 或 JSON 正文。重复或高相似文本由后端降噪。

### `message.milestone`

只在一个或多个 Plan Step 首次转为 completed 时发送，包含
`completed_step_ids`。同一 Step 不重复生成里程碑。

### `message.final`

只有 `finish` 通过 Completion Gate 且 Commit 成功后发送。文本来自安全化后的
`finish.message`，可带 Resource 语义的 `affected_targets`。普通 assistant 文本不是
最终消息。

## 7. Tool 事件

每个实际 Tool Call 都有独立 `call_id`，且严格成对：

```json
{
  "schema_version": 3,
  "run_id": "run_123",
  "occurred_at": "2026-08-02T10:30:08.000Z",
  "call_id": "call_01",
  "tool": "read_ppt",
  "target": {
    "type": "deck",
    "part": "outline"
  },
  "display": {
    "label": "读取整份结构",
    "detail": "确认叙事结构与设计约束"
  }
}
```

`tool.started` 可带 `plan_step_id`；`tool.completed` 带
`status=completed|failed`。失败包含安全 `PublicError`。`render_slide` 成功可带
Run-scoped screenshot preview URL 与安全 warning。

Tool Event 不发送完整 args、`content`、`edits`、HTML/JSON 正文、路径、staging、Evidence
或 Provider 输出。业务展示使用“整份结构”“全局设计”“第 N 页设计稿”“第 N 页 HTML”等
语言。

同一模型响应的多个 Tool Calls 仍分别投影事件。并发调用可以先发送多条 started，再按
Runtime 确定的稳定顺序投影 completed；每个 `call_id` 必须且只能完成一次。被写失败
fail-fast 跳过的调用也要收到对应 failed completed，错误码为依赖失败。

## 8. Question 事件

### `question.asked`

Agent 缺少必须由用户决定的信息时发送，并把 Run 转为 waiting。同一 Run 同时最多一个
pending question。payload 包含稳定 `question_id`、`prompt`、单选/多选、选项和是否允许
自定义文本。

### `question.answered`

后端接受与 pending question 匹配的回答后发送，包含结构化 answer 与安全展示文本，然后
恢复原连续 ReAct Loop。

## 9. 前端投影

前端保持：

- 顶部 Project Tabs；
- 左中右三栏；
- 侧栏拖动、伸缩与收起；
- 右侧 Thread Tabs；
- 底部 Composer；
- 低噪声 Timeline。

Reducer 以事件为事实源：

- `run.started` 创建 Session；
- `run.progress` 替换 live status；
- `plan.updated` 覆盖 Plan；
- Message 事件追加低噪声消息；
- Tool 事件按 `call_id` 合并 activity；
- Question 事件维护唯一 pending question；
- `run.completed` / `run.failed` / `run.error` / `run.canceled` 关闭 Session。

历史 hydration 与实时 SSE 必须投影为相同状态。未知事件忽略并在开发环境 warning。

## 10. 安全与验收

- 公共 schema version 固定为 3。
- 公共事件集合严格为上述 17 种。
- `run.started.target.artifact` 只接受 `spec | presentation`。
- Tool target 只接受四类 Resource 投影。
- 公共 payload 递归拒绝 raw args/content、内部路径、reasoning、Context、Evidence 和 trace。
- 每个 Tool Call 的 started/completed 严格成对。
- `message.final` 先于成功 `run.completed`，两者只出现一次。
- 断线重连通过 seq 恢复，不重复投影已消费事件。
