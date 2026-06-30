---
id: AGENT-MODES
title: Agent 模式
status: approved
owner: agent
depends_on: [AGENT-OVERVIEW, ARCH-RUNTIME]
verifies: []
---

# Agent 模式

模式（mode）决定 Agent 的**行为风格**：是否产物、是否提问、是否只分析。模式与指令解耦——
`/talk` `/ask` 是切换模式的指令，但模式本身是 Run 的一等属性（`runs.mode`）。

## 模式矩阵

| mode | 产 artifact | 主动提问倾向 | 典型触发 | 目的 |
|---|---|---|---|---|
| `normal` | 是 | 低（仅关键处 checkpoint） | 默认 | 高效执行 |
| `talk` | **否** | 中 | `/talk` | 对齐理解、讨论方案 |
| `ask` | 是（应答后） | **高** | `/ask` | 对齐颗粒度、消除歧义 |

## 行为规则

| ID | 约束 |
|---|---|
| `AGENT-MODE-001` | `talk` 模式 MUST NOT 产生 artifact / 文件变更 |
| `AGENT-MODE-002` | `ask` 模式遇不确定 MUST 发 `needs_input`；无不确定可直接执行 |
| `AGENT-MODE-003` | `normal` 模式 SHOULD 减少打扰，仅在破坏性/高歧义处提问 |
| `AGENT-MODE-004` | 模式 MUST 记录在 `runs.mode`，并在 SSE `run.started` 中暴露 |
| `AGENT-MODE-005` | 模式作用于**单次 Run**，不隐式延续到下一次（除非用户再次指定） |

## 模式与 checkpoint 的协同

- `normal`：checkpoint 主要用于排空控制输入队列。
- `ask`：checkpoint 额外评估「是否存在需澄清的分歧」，倾向发 `needs_input`。
- `talk`：不落产物，但 checkpoint 仍可消费控制输入以调整分析方向。

## 验收标准（Given-When-Then）

- **AC-MODE-005**（`AGENT-MODE-005`）
  - GIVEN 一次 `/talk` Run 完成
  - WHEN 用户发下一条普通指令（无模式）
  - THEN 该 Run 为 `normal`，不继承 talk

## 校验方式

```bash
go test ./internal/agent -run TestModeMatrix
```

## 依赖

- [AGENT-OVERVIEW](agent-overview.md)、[ARCH-RUNTIME](../20-architecture/agent-runtime.md)
