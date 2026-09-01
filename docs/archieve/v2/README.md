---
id: V2-INDEX
title: 第二版设计文档索引
status: draft
owner: shared
depends_on: [OVERVIEW-SCOPE, AGENT-OVERVIEW, ARCH-HARNESS, ARCH-FRONTEND]
verifies: []
---

# AI PPT Builder — 第二版（v2）设计文档

> v2 的定位：**在 v1 已跑通的后端能力之上，补齐前端交互闭环、多对话窗口、并把 Agent 从"单循环骨架"升级为"多节点前端构建流水线"。**

本目录（`docs/v2/`）是第二版的增量事实源。R0 已直接覆盖开发期旧 Run 公共协议：

- 复用 v1 的三层结构（Run 外壳 → Harness → Tools）、SSE 协议、SQLite+FS 持久化。
- 公共目标统一为 `blueprint|presentation × slide|deck`，交互统一为 `apply|consult` 与 clarification policy。
- 明确 v1 **实现与设计设想不一致**的地方（主要在前端路由与 Agent 深度），给出可落地的修正方案。
- 对需要**新增**的契约（`plan`/`plan.update` 事件、`design_spec` 产物、`kind=outline/command` 前端暴露、多 thread 状态）给出精确定义。

**当前阶段：R0 已实现。** R1–R7 仍以 Runtime 路线图为边界，未在本轮扩张。

---

## 为什么要做 v2（一句话根因）

v1 后端 SOP（outline→generate→edit→overview→repo）与命令解析、多 thread 能力**已全部实现**，但**前端从未把它们暴露出来**：`CommandComposer` 把所有输入硬编码成 `kind=generate/scope=current`，空项目必然被后端拒为 `BAD_STATE`。因此"新建 presentation 后动不了"是**前端路由缺口**，不是后端缺能力。详见 [00-background-and-goals](00-background-and-goals.md#根因诊断)。

---

## 三大需求 → 文档映射

| 用户需求 | 本质 | 主文档 |
|---|---|---|
| 1. 四种交互（大纲/单页/全局/仓库）可用、可自由切换 | 前端补全 `kind`/`scope` 路由 + 显式模式切换器 + 智能默认 | [10-interaction-modes](10-interaction-modes.md) |
| 2. 一个 presentation 支持多个独立对话窗口 | 前端暴露多 thread + 状态从单例改为按 thread 隔离 | [20-multi-thread-windows](20-multi-thread-windows.md) |
| 3. Agent harness 循环增强 + 集成 frontend-design skill | 多节点前端构建流水线 + 设计总监节点 + 真实 plan 事件 | [30-agent-pipeline-v2](30-agent-pipeline-v2.md) |

---

## 文档地图

| 文件 | 职责 | 主要读者 |
|---|---|---|
| [00-background-and-goals](00-background-and-goals.md) | 背景、根因诊断、需求、目标与非目标、术语增量 | 全体 / PM |
| [10-interaction-modes](10-interaction-modes.md) | 四交互模式的前端呈现、切换、意图→Run 字段映射矩阵、状态机 | 前端 / Agent |
| [20-multi-thread-windows](20-multi-thread-windows.md) | 多 thread 标签页 UX、runStore 隔离重构、SSE 多路复用 | 前端 |
| [30-agent-pipeline-v2](30-agent-pipeline-v2.md) | 多节点生成流水线、设计总监节点、frontend-design 集成、plan 事件、校验/交付 | Agent / 后端 |
| [40-api-and-data-contracts](40-api-and-data-contracts.md) | v2 契约变更（新增事件/产物/字段），与 v1 的差异清单 | 前后端 |
| [45-context-engineering-v1](45-context-engineering-v1.md) | 四 Profile、预算、Manifest、Memory、ContextRef 与 Prompt 编译运行时契约 | Agent / 后端 |
| [50-frontend-architecture-v2](50-frontend-architecture-v2.md) | 前端目录/store/组件升级、eventReducer v2、类型对齐 | 前端 |
| [60-prompts-v2](60-prompts-v2.md) | 各节点 prompt 模板（design/plan/generate/validate/deliver） | Agent / 后端 |
| [70-dev-plan-v2](70-dev-plan-v2.md) | 分里程碑（V2-M1~M5）开发计划、DoD、验收、依赖顺序 | 全体 |

---

## 已对齐的关键决策（本轮）

| 维度 | 决策 | 依据 |
|---|---|---|
| 交互模式 UX | **显式模式切换器 + 智能默认**（空项目默认大纲、有页默认单页）；斜杠命令保留为高级快捷 | 用户确认 |
| 多窗口形态 | **Agent 面板内多 Thread 标签页**（JetBrains 风格，一次聚焦一个，切换零重载） | 用户确认 |
| Agent 流程 | **多节点生成流水线 + 真实 `plan`/`plan.update` 事件**；设计总监节点产出 `design_spec` 并注入 frontend-design 指导 | 用户确认 |
| 本轮范围 | **仅完整 v2 设计文档**（文档可规划后端增强，本轮不写代码） | 用户确认 |

---

## 阅读顺序（建议）

1. [00-background-and-goals](00-background-and-goals.md) — 建立问题边界与目标。
2. [10-interaction-modes](10-interaction-modes.md) + [20-multi-thread-windows](20-multi-thread-windows.md) — 前端两大交互升级。
3. [30-agent-pipeline-v2](30-agent-pipeline-v2.md) + [60-prompts-v2](60-prompts-v2.md) — Agent 大脑升级。
4. [40-api-and-data-contracts](40-api-and-data-contracts.md) + [50-frontend-architecture-v2](50-frontend-architecture-v2.md) — 契约与前端落地。
5. [70-dev-plan-v2](70-dev-plan-v2.md) — 分里程碑推进。
