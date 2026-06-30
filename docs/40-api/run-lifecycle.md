---
id: API-RUN
title: Run 生命周期与控制输入
status: approved
owner: backend
depends_on: [API-OVERVIEW, ARCH-RUNTIME, API-SSE]
verifies: []
---

# Run 生命周期与控制输入

本文件定义 Run 的状态转移与控制输入（HITL）语义，是 [agent-runtime](../20-architecture/agent-runtime.md) 的 API 侧契约。

## 状态机

```
pending ──start──▶ running ──┬── checkpoint ──▶ running
                             ├── needs_input ──▶ waiting ──input──▶ running
                             ├── finish ───────▶ done
                             ├── fail ─────────▶ failed
                             └── cancel ───────▶ canceled
waiting ──cancel──▶ canceled
```

| 状态 | 含义 | 可注入输入 |
|---|---|---|
| `pending` | 已创建未开始 | 否（入队保留） |
| `running` | 执行中 | 是（入队，checkpoint 消费） |
| `waiting` | 等待用户应答 needs_input | 是（应答推进） |
| `done` | 成功完成 | 否（409） |
| `failed` | 失败 | 否（409） |
| `canceled` | 已取消 | 否（409） |

## 控制输入语义

| ID | 约束 |
|---|---|
| `API-RUN-001` | `POST /runs/{id}/input` 仅在 `running`/`waiting` 接受，否则 409 `RUN_NOT_RUNNING` |
| `API-RUN-002` | 注入返回 202（已入队），不保证立即生效——在下一个 checkpoint 消费 |
| `API-RUN-003` | 带 `reply_to` 的输入 MUST 匹配某个未应答的 `needs_input`，否则 409 `CONFLICT` |
| `API-RUN-004` | `waiting` 状态收到匹配应答后 MUST 转回 `running` |
| `API-RUN-005` | `DELETE /runs/{id}` 取消：停止后续 LLM 调用，保留已落盘产物与版本，转 `canceled` |

## 输入消费时序

```
running ──(生成 page2 中)── 收到 input(入队) ──▶ 继续 page2 不打断
        ──(page2 done, checkpoint)── 排空队列 ──▶ 将 input 纳入 page3 上下文
```

## 与模式（mode）的关系

- `mode=ask`：Agent 在不确定处更倾向发 `needs_input`，进入 `waiting`。
- `mode=talk`：只发 `info`/`token`/`done`，不产 `artifact`，但仍可被注入输入以调整分析方向。
- `mode=normal`：默认，按需 checkpoint，少打扰。

详见 [50-agent/modes](../50-agent/modes.md)。

## 验收标准（Given-When-Then）

- **AC-RUN-API-001**（`API-RUN-001`）
  - GIVEN `done` 的 Run
  - WHEN 注入 input
  - THEN 409 `RUN_NOT_RUNNING`

- **AC-RUN-API-004**（`API-RUN-003/004`）
  - GIVEN Run 处于 `waiting`（发过 `needs_input` id=evt_4）
  - WHEN `POST input {reply_to:"evt_4"}`
  - THEN 202 且 Run 转回 `running`

- **AC-RUN-API-005**（`API-RUN-005`）
  - GIVEN running 且已落盘 2 页
  - WHEN `DELETE /runs/{id}`
  - THEN Run 转 `canceled`，已落盘 2 页与其版本保留

## 校验方式

```bash
go test ./internal/run -run 'TestRunStateMachine|TestInputGating|TestCancelKeepsArtifacts'
```

## 依赖

- [API-OVERVIEW](api-overview.md)、[ARCH-RUNTIME](../20-architecture/agent-runtime.md)、[API-SSE](sse-events.md)
