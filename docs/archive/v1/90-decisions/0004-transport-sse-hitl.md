---
id: ADR-0004
title: SSE + 控制 POST 实现流式与 HITL
status: accepted
owner: backend
depends_on: [ADR-0001]
verifies: []
---

# ADR-0004：SSE + 控制 POST 实现流式与 Human-in-the-loop

## 背景

LLM 生成需流式呈现；用户希望支持 human-in-the-loop——中途注入输入、回答 Agent 提问、影响后续执行。需在单用户本地场景下选择简单、可靠、HTTP 原生的方案。

## 决策

**输出走 SSE，输入走独立控制 POST**，围绕 **Run 抽象**：
- `GET /runs/{id}/events`（SSE）单向推送 token/progress/needs_input/done/error。
- `POST /runs/{id}/input` 注入控制输入，进入队列，在 Run 的 **checkpoint** 消费。
- Run 可发 `needs_input` 暂停（`waiting`），收到应答后恢复。

## 选项与权衡

| 选项 | 优点 | 缺点 |
|---|---|---|
| SSE + 控制 POST（选中） | HTTP 原生、实现简单、重连友好（Last-Event-ID）、契合 DeepSeek 流式、解耦输入输出 | 输入输出双通道需关联（用 run_id） |
| WebSocket | 单一双向通道、HITL 自然 | 重连/心跳/帧管理复杂，对单用户本地偏重 |
| 轮询 | 最简单 | 体验差、不实时 |

## 后果

- Run 引擎实现状态机 + 事件总线 + 输入队列 + checkpoint（[agent-runtime](../20-architecture/agent-runtime.md)）。
- 事件协议见 [sse-events](../40-api/sse-events.md)；生命周期见 [run-lifecycle](../40-api/run-lifecycle.md)。
- 控制输入不打断正在进行的 LLM 调用，仅在 checkpoint 生效（顺序可控、行为可预测）。
- 支持断线重连续传（持久化 run_events + Last-Event-ID）。

## 状态

accepted（2026-06-30）。
