---
id: AGENT-OVERVIEW
title: Agent 总览
status: approved
owner: agent
depends_on: [ARCH-RUNTIME, ARCH-LLM, SPEC-FUNCTIONAL]
verifies: []
---

# Agent 总览

## 定位

本 Agent 是**专用 frontend PPT Coding Agent**：不做通用任务，所有能力围绕「生成/编辑/查看 PPT 前端」收敛。
它通过 Run 引擎执行，借 LLM（DeepSeek）完成推理与代码生成，受设计系统与产出规范约束。

## 职责边界

| Agent 负责 | Agent 不负责 |
|---|---|
| 解析用户意图与指令（`/page` 等） | 通用编程任务 |
| 装配上下文（当前页/全局样式/选区/插件） | 直接暴露 LLM 原始能力给用户 |
| 调用 LLM 生成/编辑 slide 前端 | 越过设计系统硬编码样式 |
| 保证产出符合 html-output-spec | 修改系统级安全约束 |
| 在 checkpoint 处理 HITL、提问对齐 | 在 `/talk` 模式下改文件 |

## 子系统

| 子系统 | 文档 |
|---|---|
| 指令系统（解析/分派/语义） | [commands/README](commands/README.md) |
| 模式（normal/talk/ask） | [modes](modes.md) |
| prompt 模板（各阶段） | [prompt-templates](prompt-templates.md) |
| 上下文装配 | [context-assembly](context-assembly.md) |

## 执行主循环（概念）

```
接收请求(instruction/command/scope/mode)
  │
  ▼
解析指令 → 确定 scope（page/overview/deck）与 mode
  │
  ▼
装配上下文（context-assembly）：系统约束 + 设计系统切片 + 目标产物 + 选区/插件
  │
  ▼
按 mode 决策：
  ├─ talk  → 只产 info/token，禁止 artifact
  ├─ ask   → 不确定即 needs_input，对齐后再做
  └─ normal→ 直接执行，checkpoint 处理 HITL
  │
  ▼
调用 LLM（流式）→ 产出 → 校验 html-output-spec → 落盘 + 版本
  │
  ▼
done
```

## 关键约束

| ID | 约束 |
|---|---|
| `AGENT-001` | 所有 Agent 行为 MUST 经 Run 引擎，遵循 SSE 事件协议 |
| `AGENT-002` | prompt MUST 由 [prompt-templates](prompt-templates.md) 装配，禁止散落硬编码 |
| `AGENT-003` | 产出 MUST 经 html-output-spec 校验后才落盘 |
| `AGENT-004` | scope 隔离 MUST 严格：page 不改公共层，overview 只改公共层 |
| `AGENT-005` | 用户输入 MUST 经注入边界处理（见 [llm-integration](../20-architecture/llm-integration.md)） |

## 验收标准（Given-When-Then）

- **AC-AGENT-004**（`AGENT-004`）
  - GIVEN 一条 page scope 请求
  - WHEN Agent 执行
  - THEN 仅目标页文件变更（与 [SPEC-EDIT-003](../10-spec/feat-nl-editing.md) 一致）

## 校验方式

```bash
go test ./internal/agent/... -run 'TestScopeIsolation|TestModeBehavior'
```

## 依赖

- [ARCH-RUNTIME](../20-architecture/agent-runtime.md)、[ARCH-LLM](../20-architecture/llm-integration.md)、[SPEC-FUNCTIONAL](../10-spec/functional-spec.md)
