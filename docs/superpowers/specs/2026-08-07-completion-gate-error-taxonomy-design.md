# Completion Gate Error Taxonomy Design

> 日期：2026-08-07  
> 范围：后端 Agent Runtime Completion Gate 错误码与确定性门禁逻辑  
> 状态：已讨论定稿，直接实施

## 背景

当前 Completion Gate 混合了确定性校验、语义需求覆盖判断和 Reviewer 问题投影，导致错误码语义不清晰，也让 `execute` 被错误理解为“必须产生写变更”。这会伤害 Agent 的灵活性：语义问题不应通过程序规则写死，否则会让 LLM + Agent 的判断能力退化成僵硬流程。

本次改动将 Gate 收敛为确定性门禁：只检查 Runtime 状态、工具并发、Run 生命周期、写入会话、计划闭环、资源证据和资源同步。语义质量、用户意图、最终回答完整性等问题独立留给 Semantic Review 后续重新设计。

## 目标错误码

### Finish

| Code | 语义 |
|---|---|
| `FINISH_MESSAGE_EMPTY` | `finish.message` 为空 |
| `FINISH_NOT_ALLOWED` | 当前 strategy/phase 不允许 finish |

`FINISH_MESSAGE_EMPTY` 只做非空校验。删除原先基于 Markdown 标题、列表、表格、长度比例等启发式判断的 `FINISH_CONTRACT_VIOLATION`。

### Tool

| Code | 语义 |
|---|---|
| `TOOLS_STILL_RUNNING` | 仍有工具调用未完成 |

### Run

| Code | 语义 |
|---|---|
| `RUN_ALREADY_CANCELED` | Run 已取消，不能走成功完成路径 |
| `RUN_FATAL_EXIST` | Runtime 已存在 fatal issue |
| `RUN_SESSION_MISSING` | 写入/commit 路径缺少 RunSession |
| `RUN_REVISION_CONFLICT` | 乐观并发或恢复场景下，baseline revision 已变化 |

删除 `RUN_SESSION_INVALID`。如果未来需要恢复更细会话错误，应拆成具体错误码，而不是保留抽象兜底。

### Plan

| Code | 语义 |
|---|---|
| `PLAN_NOT_COMPLETE` | fulfill 计划仍有 pending / in_progress / failed |

### Evidence

| Code | 语义 |
|---|---|
| `EVIDENCE_SCHEMA_MISSING` | 变更的 outline / design / slide spec 缺少结构校验证据 |
| `EVIDENCE_HTML_MISSING` | 变更的 HTML 缺少静态校验或渲染校验证据 |

`EVIDENCE_HTML_MISSING` 对外合并原 `STATIC_EVIDENCE_REQUIRED` 和 `VISUAL_EVIDENCE_REQUIRED`。HTML evidence 绑定 HTML 自身 hash，不再对外使用 `Design + Spec + HTML` 组合 hash 概念。Spec/Design 到 HTML 的同步由 Async 错误表达。

### Async

| Code | 语义 |
|---|---|
| `ASYNC_SPEC_HTML` | presentation 目标下，Spec 或 Design 变更未同步到 HTML |
| `ASYNC_DECK_SLIDE` | outline 与 slide specs 缺失、不同步或引用非法 |

`ASYNC_SPEC_HTML` 暂时合并 Spec->HTML 与 Design->HTML 两类同步问题。后续需要结合工作目标设计继续细化：如果用户只要求修改设计稿/spec/design，不应强制 HTML 同步。

### Block / Error

| Code | 语义 |
|---|---|
| `COMPLETION_GATE_BLOCK` | Gate 问题的 wrapper observation |
| `COMPLETION_REVIEW_BLOCK` | Review 问题的 wrapper observation |
| `COMPLETION_BLOCK_REPEAT` | 同一 block 连续超过阈值，终止 Run |

## 删除项

- 删除 Gate 中的 `REQUIREMENT_UNADDRESSED` 程序判断。
- 删除 `SemanticCompletionPolicy` 作为 Gate policy 的接入。
- 删除 `FINISH_CONTRACT_VIOLATION` 的启发式最终交付判断。
- 删除 `RUN_SESSION_INVALID`。
- 不再对外暴露 `SCHEMA_EVIDENCE_REQUIRED`、`STATIC_EVIDENCE_REQUIRED`、`VISUAL_EVIDENCE_REQUIRED`、`REFERENCE_EVIDENCE_REQUIRED`、`SLIDE_HTML_SYNC_REQUIRED`、`SLIDE_SPEC_REQUIRED`。

## 实施要求

1. Completion Gate 仅返回新错误码。
2. Review 错误码保持独立，不与 Gate wrapper 混用。
3. Runtime 对 Gate 拒绝使用 `COMPLETION_GATE_BLOCK`。
4. Runtime 对 Review 拒绝使用 `COMPLETION_REVIEW_BLOCK`。
5. 重复 block 熔断使用 `COMPLETION_BLOCK_REPEAT`。
6. Prompt repair guide 与单元测试同步更新。
