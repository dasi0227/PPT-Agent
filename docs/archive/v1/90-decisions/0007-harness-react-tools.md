---
id: ADR-0007
title: Agent Harness 采用 ReAct + 工具化
status: accepted
owner: backend
depends_on: [ADR-0004]
verifies: []
---

# ADR-0007：Agent Harness 采用 ReAct 循环 + function calling + 工具化

## 背景

要做的不是 demo，而是生产级、可写进简历的 agent 项目，须遵循最新 harness 范式。
核心需求「局部 patch 编辑」决定了不能用「一次 LLM 调用吐整页 HTML」，必须「读→定位→精确改→校验」。
原 docs 只描述了 Run 的生命周期外壳，缺少真正的 agent 循环与工具抽象。

## 决策

引入 **Harness**：Run 内部的 **ReAct 循环**（思考→调用工具→观察→再思考），由 **function calling** 驱动。
- 工具化：read/patch/write/validate/mount/asset 操作均为带 JSON schema 的确定性脚本。
- 动态工具门控：按 scope/mode 裁剪注册给 LLM 的工具集，从机制上实现隔离。
- 子代理：逐页生成等批量任务用隔离上下文的子代理；编辑用主循环。
- 停止条件：MAX_TURNS + finish + 失败熔断 + 取消。

## 选项与权衡

| 选项 | 优点 | 缺点 |
|---|---|---|
| ReAct + 工具化（选中） | 支持局部 patch、机制级隔离、可观测、生产级 | 实现复杂度高 |
| 单次 LLM 吐 HTML | 简单 | 无法局部 patch、无隔离、非生产级 |
| 固定脚本流水线（无 LLM 自主） | 可控 | 失去 agent 灵活性 |

## 后果

- 新增 [agent-harness](../20-architecture/agent-harness.md) 与 [tools](../20-architecture/tools.md)。
- Run（[agent-runtime](../20-architecture/agent-runtime.md)）退为外壳，内部驱动 harness。
- scope 隔离从「约定」变为「工具集物理裁剪」，hash diff 可证明。
- SSE 新增 `thought`/`tool_call`/`tool_result` 事件，全程可追溯。
- LLM 客户端需 `CallTool`（function calling）能力。

## 状态

accepted（2026-06-30）。
