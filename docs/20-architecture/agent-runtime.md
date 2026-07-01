---
id: ARCH-RUNTIME
title: Agent Run 引擎（生命周期外壳，SSE + HITL）
status: approved
owner: backend
depends_on: [ARCH-SYSTEM, ARCH-HARNESS, ADR-0004, API-RUN]
verifies: []
---

# Agent Run 引擎（生命周期外壳：SSE 流式 + Human-in-the-loop）

## 核心抽象：Run 是「外壳」，Harness 是「大脑」

一次 Agent 执行 = 一个 **Run**。Run 是**生命周期外壳**：负责状态机、事件流（SSE）、控制输入队列、取消。
Run 内部驱动一个 **Harness**（ReAct 循环 + 工具化），后者才是真正做事的 agent 大脑。

```
Run（本文件：状态机 + SSE + 控制输入 + 取消）
  └── 内部驱动 ──▶ Harness（见 agent-harness.md：thought→tool_call→observation 循环）
```

职责切分（不要混淆）：

| 关注点 | 归属 |
|---|---|
| 状态机、对外 API、SSE 帧、断线续传、控制输入队列、取消 | **Run（本文件）** |
| ReAct 循环、工具集与动态门控、停止条件、上下文压缩、子代理 | **Harness（[agent-harness](agent-harness.md)）** |
| 具体工具的参数 schema 与执行语义 | **Tools（[tools](tools.md)）** |

所有生成/编辑/指令/资产操作都创建一个 Run，Run 据 scope/mode 构造对应 Harness。

## 生命周期状态机

```
        create
          │
          ▼
      ┌────────┐  start   ┌──────────┐
      │ pending│ ───────▶ │ running  │
      └────────┘          └────┬─────┘
                               │
            ┌──────────────────┼───────────────────┐
            │                  │                    │
       needs_input        checkpoint            error/cancel
            │                  │                    │
            ▼                  ▼                    ▼
      ┌──────────┐  input  ┌──────────┐       ┌──────────┐
      │ waiting  │ ──────▶ │ running  │       │ failed/  │
      │ (HITL)   │         └────┬─────┘       │ canceled │
      └──────────┘              │             └──────────┘
                                ▼
                           ┌────────┐
                           │ done   │
                           └────────┘
```

状态枚举：`pending | running | waiting | done | failed | canceled`。

## SSE 事件协议（详见 [sse-events](../40-api/sse-events.md)）

| 事件 | 含义 |
|---|---|
| `run.started` | Run 开始 |
| `thought` | Harness 一轮推理（ReAct 的 Reason） |
| `tool_call` | LLM 发起一次工具调用（名 + 参数） |
| `tool_result` | 工具执行的 observation（ReAct 的 Act 结果） |
| `progress` | 进度（含 `current`/`total`/`stage`） |
| `token` | LLM 流式 token（用于实时显示） |
| `artifact` | 一个产出落盘（slide html / 公共样式层 / 资产 / 版本） |
| `needs_input` | 暂停等待用户输入（含 `prompt` 与可选 `schema`） |
| `info` | 信息性消息（如 `/talk` 的分析输出） |
| `done` | 完成（含结果引用） |
| `error` | 错误（含错误码） |

> `thought`/`tool_call`/`tool_result` 是 Harness ReAct 循环每一轮的可观测投影（[ARCH-HARNESS-003](agent-harness.md)），使整个执行可追溯。

## Human-in-the-loop 机制

### 两种输入注入方式

1. **主动注入（中途追加）**：用户随时 `POST /runs/{id}/input`，消息进入**控制输入队列**。
2. **被动应答（Agent 提问）**：Run 发 `needs_input` → 进入 `waiting` → 用户 `POST input` 应答 → 回到 `running`。

### Checkpoint 语义

- Run 在关键节点设 checkpoint（如：每页生成后、执行破坏性编辑前、`/ask` 提问点）。
- 到达 checkpoint 时：**排空控制输入队列**，将其纳入后续 LLM 上下文；若处于 `/ask` 模式或存在不确定性，则发 `needs_input` 并暂停。

| ID | 约束 |
|---|---|
| `ARCH-RUN-001` | 每个 Run MUST 有唯一 `run_id` 与明确状态游标（单一真相，无冗余标志） |
| `ARCH-RUN-002` | 控制输入 MUST 入队，仅在 checkpoint 被消费，不打断正在进行的 LLM 调用 |
| `ARCH-RUN-003` | `needs_input` MUST 携带足够上下文（prompt，必要时附 schema），客户端据此应答 |
| `ARCH-RUN-004` | Run 取消 MUST 安全终止：停止后续 LLM 调用，保留已落盘产物与版本 |
| `ARCH-RUN-005` | 同一 Run 的事件 MUST 单调有序（带递增 `seq`），支持断线重连续传（`Last-Event-ID`） |
| `ARCH-RUN-006` | `/talk` 模式 Run MUST NOT 产生 `artifact`（只发 `info`/`token`/`done`） |

## 并发与隔离（多 PPT / 多线程）

三层隔离见 [data-model](../30-data-model/data-model.md)：Project（PPT/work_dir）→ Thread（对话）→ Run（turn）。

| ID | 约束 |
|---|---|
| `ARCH-RUN-LOCK-001` | **每 project 一把执行锁**：同一 project 的 Run 串行执行，防 `state.json`/产物文件竞态 |
| `ARCH-RUN-LOCK-002` | **跨 project 并行**：不同 project 的 Run 可并发（work_dir 物理隔离，无共享状态） |
| `ARCH-RUN-LOCK-003` | 同 project 多 thread **共享产物**，因此仍受同一把 project 锁约束：不同 thread 的写 Run 串行 |
| `ARCH-RUN-LOCK-004` | Run 挂在 thread 下；thread 提供对话历史，Run 执行前由 [context-assembly](../50-agent/context-assembly.md) 载入该 thread 历史 |
| `ARCH-RUN-LOCK-005` | 锁等待 MUST 有上限；超时 Run 转 `failed` 并提示（不无限阻塞） |

> 结论：多个 PPT 可真正并行处理；同一个 PPT 内即便开多条对话线程，写操作也会排队，保证产物一致。

## 时序：整套生成 + 中途注入

```
Client                     Backend(Run Engine)            LLM
  │ POST /threads/{id}/runs                                 │
  │ ─────────────────────────▶ 取 project 锁 → create Run   │
  │ ◀──────────── 201 {run_id, events_url}                  │
  │ GET /runs/{id}/events (SSE)                              │
  │ ─────────────────────────▶ start → running              │
  │ ◀── run.started                                         │
  │ ◀── progress(1/8)         ─────────────────────────────▶│ gen page1
  │ ◀── artifact(page1)                                     │
  │ POST /runs/{id}/input "第3页用深色"                       │
  │ ─────────────────────────▶ enqueue                      │
  │ ◀── progress(2/8)                                       │
  │                            [checkpoint] drain queue      │
  │ ◀── progress(3/8 深色生效)  ─────────────────────────────▶│ gen page3(dark)
  │ ...                                                      │
  │ ◀── done {project_id}                                    │
```

## 验收标准（Given-When-Then）

- **AC-RUN-002**（`ARCH-RUN-002`）
  - GIVEN running 的整套生成 Run
  - WHEN 在第 2 页生成期间注入输入
  - THEN 输入不打断第 2 页，在第 3 页 checkpoint 生效

- **AC-RUN-005**（`ARCH-RUN-005`）
  - GIVEN SSE 连接中断后带 `Last-Event-ID` 重连
  - WHEN 重新订阅
  - THEN 从断点之后的 `seq` 续传，无重复无丢失

- **AC-RUN-006**（`ARCH-RUN-006`）
  - GIVEN `/talk` 模式 Run
  - WHEN 执行完成
  - THEN 事件流中无 `artifact`，文件系统无新增/变更

- **AC-RUN-LOCK-001**（`ARCH-RUN-LOCK-001/002`）
  - GIVEN 项目 A 有一个 running 的 Run
  - WHEN 同时对项目 A 发第二个写 Run、对项目 B 发一个写 Run
  - THEN 项目 A 的第二个 Run 排队等待，项目 B 的 Run 立即并行执行

## 校验方式

```bash
go test ./internal/run -run 'TestRunLifecycle|TestHITLInput|TestSSEResume|TestPerProjectLock'
```

## 依赖

- [ARCH-SYSTEM](system-overview.md)、[ADR-0004](../90-decisions/0004-transport-sse-hitl.md)、[API-RUN](../40-api/run-lifecycle.md)
