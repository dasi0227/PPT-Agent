---
id: SPEC-CMD-ASK
title: 指令 /ask（模式）
status: approved
owner: agent
depends_on: [AGENT-CMD-INDEX, AGENT-MODES, ARCH-RUNTIME]
verifies: []
---

# 指令 `/ask`（必须提问模式）

## 目标

进入一种模式：Agent **必须就可能存在的疑惑与不确定点提问**，以对齐颗粒度。
若确实没有疑问，允许不提问直接进入执行——核心是「不在含糊处擅自假设」。

## 语法

```
/ask <任务描述>
```

## 规格正文

| ID | 需求 | 优先级 |
|---|---|---|
| `SPEC-CMD-ASK-001` | `/ask` 模式下，若存在不确定点，Agent MUST 通过 `needs_input` 提问并暂停（`waiting`） | P0 |
| `SPEC-CMD-ASK-002` | 若无任何不确定点，MAY 不提问直接执行 | P0 |
| `SPEC-CMD-ASK-003` | 提问 MUST 具体、可回答（避免空泛），SHOULD 提供候选项（`choices`） | P1 |
| `SPEC-CMD-ASK-004` | 收到应答后 MUST 据此继续，不再就同一点重复提问 | P0 |
| `SPEC-CMD-ASK-005` | 提问 MUST 聚焦影响产出的关键分歧，避免过度打扰 | P1 |

## 与 needs_input / waiting 的关系

`/ask` 是触发 `needs_input` 的高频场景：模式提高 Agent 的「提问倾向阈值」。
机制复用 Run 引擎的 `needs_input → waiting → input → running`（见 [run-lifecycle](../../40-api/run-lifecycle.md)）。

## 验收标准（Given-When-Then）

- **AC-CMD-ASK-001**（`SPEC-CMD-ASK-001/003`）
  - GIVEN `/ask 帮我把这页做成图表` 但未给数据
  - WHEN 执行
  - THEN Agent 发 `needs_input` 询问数据来源/图表类型（带候选），Run 进入 `waiting`

- **AC-CMD-ASK-002**（`SPEC-CMD-ASK-002`）
  - GIVEN `/ask 把封面标题居中`（无歧义）
  - WHEN 执行
  - THEN 可不提问直接执行并完成

## 校验方式

```bash
go test ./internal/agent/command -run 'TestAskQuestionsWhenUncertain|TestAskProceedsWhenClear'
```

## 依赖

- [AGENT-CMD-INDEX](README.md)、[AGENT-MODES](../modes.md)、[ARCH-RUNTIME](../../20-architecture/agent-runtime.md)
