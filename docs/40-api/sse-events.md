---
id: API-SSE
title: SSE 事件协议
status: approved
owner: backend
depends_on: [API-OVERVIEW, ARCH-RUNTIME]
verifies: []
---

# SSE 事件协议

Run 的流式输出经 `GET /runs/{id}/events`（`text/event-stream`）。本文件定义事件格式与语义。

## 帧格式

每个事件遵循 SSE 标准帧：

```
id: <seq>
event: <type>
data: <json>

```

- `id`：单调递增 `seq`，用于 `Last-Event-ID` 断线续传。
- `event`：事件类型（见下表）。
- `data`：JSON 负载。
- 心跳：服务端周期发送注释行 `: ping`，保持连接。

## 事件类型与负载

| event | data 字段 | 说明 |
|---|---|---|
| `run.started` | `{ run_id, kind, scope, mode }` | Run 开始 |
| `progress` | `{ stage, current, total, message? }` | 进度（如逐页生成 current/total） |
| `token` | `{ text }` | LLM 流式增量文本 |
| `artifact` | `{ artifact_type, ref, page_index? }` | 一个产物落盘（`slide_html`/`common_style`/`version`） |
| `needs_input` | `{ id, prompt, schema?, choices? }` | 暂停等待输入；客户端用 `reply_to=id` 应答 |
| `info` | `{ text }` | 信息性输出（`/talk` 的分析、提示） |
| `done` | `{ result }` | 完成；result 含产物引用（deck_id/slide_id/version_no 等） |
| `error` | `{ code, message }` | 错误，对应错误码表 |

## 序列约定

| ID | 约束 |
|---|---|
| `API-SSE-001` | 同一 Run 事件 `seq` MUST 单调递增、连续 |
| `API-SSE-002` | 终态事件 MUST 为 `done`、`error` 或连接因 `canceled` 关闭，之一且仅一次 |
| `API-SSE-003` | 客户端带 `Last-Event-ID` 重连，服务端 MUST 从该 seq 之后续发（依赖 run_events 持久化） |
| `API-SSE-004` | `/talk` 模式 MUST NOT 发 `artifact` |
| `API-SSE-005` | `needs_input` 的 `id` MUST 唯一，供 `reply_to` 精确匹配 |

## 示例流（整套生成 + needs_input）

```
id: 1
event: run.started
data: {"run_id":"r1","kind":"generate","scope":"deck","mode":"normal"}

id: 2
event: progress
data: {"stage":"generate","current":1,"total":8}

id: 3
event: artifact
data: {"artifact_type":"slide_html","ref":"slides/000/index.html","page_index":0}

id: 4
event: needs_input
data: {"id":"evt_4","prompt":"第4页缺少数据，用示例数据还是等你提供？","choices":["示例数据","我来提供"]}

id: 5
event: info
data: {"text":"已采用示例数据继续。"}

id: 6
event: done
data: {"result":{"deck_id":"d1","slide_count":8}}
```

## 验收标准（Given-When-Then）

- **AC-SSE-001**（`API-SSE-001/002`）
  - GIVEN 一次正常生成
  - WHEN 读取事件流
  - THEN seq 连续递增，且以恰好一个 `done` 结束

- **AC-SSE-003**（`API-SSE-003`）
  - GIVEN 中断后带 `Last-Event-ID: 3` 重连
  - WHEN 续订
  - THEN 收到 seq≥4 的事件，无重复 seq≤3

## 校验方式

```bash
go test ./internal/run -run 'TestSSESequence|TestSSEResume'
```

## 依赖

- [API-OVERVIEW](api-overview.md)、[ARCH-RUNTIME](../20-architecture/agent-runtime.md)
