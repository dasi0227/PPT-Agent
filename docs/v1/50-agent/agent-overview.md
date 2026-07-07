---
id: AGENT-OVERVIEW
title: Agent 总览
status: approved
owner: agent
depends_on: [ARCH-RUNTIME, ARCH-HARNESS, ARCH-TOOLS, ARCH-LLM, SPEC-FUNCTIONAL]
verifies: []
---

# Agent 总览

## 定位

本 Agent 是**专用 frontend PPT Coding Agent**：不做通用任务，所有能力围绕「生成/编辑/查看 PPT 前端」收敛。
它由 **Run 外壳** 承载、内部跑一个 **Harness（ReAct 循环 + 工具化）**，借 LLM（DeepSeek）做认知决策，
通过**工具**改动产物，受设计系统与产出规范约束。

## 三层结构（务必分清）

```
Run（生命周期外壳） → Harness（ReAct 循环/动态工具门控/子代理） → Tools（带 schema 的确定性脚本）
   见 agent-runtime.md        见 agent-harness.md                     见 tools.md
```

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
解析指令 → 确定 scope（current/page/overview/repo）与 mode
  │
  ▼
按 scope/mode 动态裁剪工具集（harness 门控）+ 装配上下文（context-assembly）
  │
  ▼
进入 Harness ReAct 循环：
  thought → tool_call → observation → … → finish
  ├─ talk  → 不注册任何产物工具（只说不做）
  ├─ ask   → 不确定即 needs_input，对齐后再做
  └─ normal→ 直接执行，checkpoint 处理 HITL
  │
  ▼
工具执行 → 校验 html-output-spec → 落盘 + 版本（均为工具副作用）
  │
  ▼
done
```

详见 [agent-harness](../20-architecture/agent-harness.md)。

## 关键约束

| ID | 约束 |
|---|---|
| `AGENT-001` | 所有 Agent 行为 MUST 经 Run 外壳 + Harness，遵循 SSE 事件协议 |
| `AGENT-002` | prompt MUST 由 [prompt-templates](prompt-templates.md) 装配，禁止散落硬编码 |
| `AGENT-003` | 产出 MUST 经 html-output-spec 校验后才落盘（由写工具隐式触发） |
| `AGENT-004` | scope 隔离 MUST 由**工具门控机制**保证：current/page 只给目标页工具；overview 优先公共层；repo 只给资产工具，不给 PPT 页工具 |
| `AGENT-005` | 用户输入 MUST 经注入边界处理（见 [llm-integration](../20-architecture/llm-integration.md)） |
| `AGENT-006` | LLM MUST 通过工具改产物，MUST NOT 把内容当对话文本直接落盘（[ARCH-HARNESS-002](../20-architecture/agent-harness.md)） |

## 验收标准（Given-When-Then）

- **AC-AGENT-004**（`AGENT-004`）
  - GIVEN 一条 page/current scope 请求
  - WHEN Harness 构造工具集
  - THEN 工具集不含改其它页/公共层的函数，仅目标页文件可能变更

## 校验方式

```bash
go test ./internal/agent/... -run 'TestScopeIsolation|TestModeBehavior'
```

## 依赖

- [ARCH-RUNTIME](../20-architecture/agent-runtime.md)、[ARCH-HARNESS](../20-architecture/agent-harness.md)、[ARCH-TOOLS](../20-architecture/tools.md)、[ARCH-LLM](../20-architecture/llm-integration.md)、[SPEC-FUNCTIONAL](../10-spec/functional-spec.md)
