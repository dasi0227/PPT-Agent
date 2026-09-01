---
id: V2-BACKGROUND
title: 背景、根因诊断与目标
status: draft
owner: shared
depends_on: [V2-INDEX, OVERVIEW-SCOPE]
verifies: []
---

# 背景、根因诊断与目标

## 1. 背景

v1 已完成 M0–M7 编码：后端具备 Project / Thread / Run / SSE / Slide / Asset 全链路，Agent 侧
outline / generate / edit / overview / repo / assist（prompt/recap/talk/ask）**全部落地并通过单测**；前端
搭出三栏工作台（顶部项目标签、左侧 Deck、中间 iframe 预览、右侧 Agent 流）。

但实际体验中出现"新建 presentation 后无法交互"的问题，且设计设想中的**四类交互自由切换**、**多对话窗口**、
**充实的 Agent 构建流程**均未真正达成。本轮对全部代码与文档做了系统审查，结论如下。

## 2. 根因诊断

> 结论先行：**这不是后端缺能力，而是前端没有把后端能力接出来，同时 Agent 循环停留在最小骨架。**

### 2.1 「新建后动不了」= 前端路由缺口（P0）

| 证据 | 位置 | 问题 |
|---|---|---|
| Composer 硬编码请求 | `frontend/src/features/agent/CommandComposer.tsx` `handleSubmit` | 所有输入都发 `kind:'generate', scope:'current', page_index:currentPage` |
| 前端类型缺 kind | `frontend/src/api/types.ts` `RunPayload` | `kind` 仅 `'generate' \| 'edit' \| 'analyze'`，**没有 `outline`、`command`**；后端真实 kind 是 `outline/generate/edit/command` |
| 后端拒绝空项目生成 | `backend/internal/agent/generate/runner.go` | `len(slides)==0` → 返回 `BAD_STATE：无可生成的 slide（请先完成大纲）` |
| 逻辑依赖 | `docs/50-agent` + project_memory | 没有大纲不能 `generate`；没有 HTML 不能 `edit`，否则 `BAD_STATE` |

**链路复盘**：新建 project → slides 为空 → 用户输入任意内容 → 前端发 `kind=generate` → 后端 `generate.Runner`
发现无 slide → `BAD_STATE`。用户看到报错，误以为"产品坏了"。**正确首步应是 `kind=outline`**，但前端**没有任何入口能发出 outline 请求**。

### 2.2 四类交互无法自由切换 = 前端未暴露 scope/kind（P0）

- 后端 `agent/command/parse.go` 已能把 `/current /page x /overview /repo /prompt /recap /talk /ask` 权威解析为
  Run 字段；`service/run.go` 已按 `kind + scope + command + mode` 路由到对应 runner。
- 前端只识别 `/talk` `/ask` 两个前缀（且仍强制 `kind=generate`），**scope 永远是 `current`**，
  `/page` `/overview` `/repo` 与 `kind=outline` 完全无法触达。
- 结果：设计设想的"大纲/单页/全局/仓库四类交互"在 UI 上**不存在切换手段**。

### 2.3 多对话窗口缺失 = 前端只用 threads[0] + 单例 runStore（P1）

| 证据 | 位置 | 问题 |
|---|---|---|
| 只取第一个 thread | `CommandComposer.tsx` | `let threadId = threads[0]?.id`，永远复用/新建单一 thread |
| runStore 是全局单例 | `frontend/src/stores/runStore.ts` | `activeRunId/status/timelineItems/eventSourceClose` 全局唯一，天然无法并存多个对话 |
| 后端已支持多 thread | `httpapi/thread_handler.go` + router | `POST/GET /projects/{id}/threads`、`/threads/{id}/history` 均已实现，Thread 隔离历史、共享产物 |

**结论**：多窗口是纯前端工作——暴露多 thread + 把 run 状态从"全局单例"改为"按 thread 隔离"。

### 2.4 Agent 循环是最小骨架 = 缺前端构建的专业深度（P1）

| 现状 | 位置 | 局限 |
|---|---|---|
| 生成 = 逐页单发 | `generate/runner.go` `generatePage` | 每页 `write_slide → validate → finish`，无"整体设计语言"统筹，各页各写各的 |
| 无设计总监环节 | `prompt/slide.go` | slide.gen 仅塞 slide-json + token 清单，无 frontend-design 级别的排版/字体/signature 指导 |
| 无 plan 事件 | `model/event.go` | 只有 `run.started/thought/tool_call/tool_result/progress/token/artifact/needs_input/info/done/error`；前端 `PlanCard` 已预留却拿不到真实数据 |
| harness 通用但浅 | `harness/loop.go` | 单一 ReAct 循环，能力足够但没有"分步骤流水线 + 阶段校验 + 交付确认"的编排 |

## 3. v2 需求（用户原话 → 拆解）

### 需求 1：四类交互可用且可自由切换
- 大纲创建/编辑 → `outline` 模式
- 单页创建/编辑 → `page` 模式（含默认当前页 current）
- 全局创建/编辑 → `overview` 模式
- 仓库创建/编辑 → `repo` 模式
- 前端需在这些模式间**自由切换转换**。

### 需求 2：多对话窗口
- 一个 presentation 同时搭建**多个独立对话窗口**，每个窗口独立交互。

### 需求 3：Agent 循环增强
- 完善 harness loop，**增加更多处理节点**、优化提示词。
- **新增节点集成 Claude 官方 `frontend-design` skill** 做专业处理。
- 更完整的分步处理流程 + 更完善的校验与交付逻辑。

## 4. v2 目标（可验收）

| 编号 | 目标 | 验收信号 |
|---|---|---|
| `V2-G1` | 新建 project 后能一站式走完 outline→generate→edit 全流程，无 `BAD_STATE` 死路 | 空项目输入 → 自动/引导发 `kind=outline`；有大纲后可 generate；有页后可 edit |
| `V2-G2` | 前端显式暴露四交互模式，可自由切换，映射到正确的 `kind/scope/mode` | 切换器切到 overview → 发 `scope=overview`；斜杠命令仍可用 |
| `V2-G3` | 一个 project 下可创建/切换多个对话 thread，各自独立运行与 SSE | 新建第 2 个 thread，两个 thread 的 run 状态互不干扰 |
| `V2-G4` | 生成流程升级为多节点流水线：设计总监 → 计划 → 逐页生成 → 校验 → 交付 | SSE 出现 `plan`/`plan.update`；产出 `design_spec`；前端 PlanCard 显示真实步骤 |
| `V2-G5` | frontend-design skill 指导注入设计节点，产出更专业的 slide | design_spec 含 palette/type/layout/signature；slide 视觉明显区别于模板默认 |

## 5. 非目标（v2 明确不做）

- **不推翻 v1 架构**：三层结构、SSE 帧格式、SQLite+FS、scope/mode 四件套语义保持不变。
- **不改单机无鉴权定位**：不引入登录/协作/多用户。
- **不做导出（PNG/PDF/PPTX）**：仍在 backlog。
- **不引入前端可拖拽浮动窗口**：多窗口本轮以"Agent 面板内多 Thread 标签页"实现（用户确认）。
- **不做可配置节点图编排引擎**：流水线用固定阶段（设计→计划→生成→校验→交付），不做通用 DAG 引擎（用户确认）。
- **本轮不写业务代码**：仅产出设计文档。

## 6. 术语增量（相对 v1 glossary）

| 术语 | 含义 |
|---|---|
| 交互模式（Interaction Mode） | 前端概念，把 v1 的 `kind + scope` 折叠成用户可感知的四档：**Outline / Page / Overview / Repo**。见 [10-interaction-modes](10-interaction-modes.md) |
| 对话窗口（Thread Window） | 前端对后端 Thread 的一次可视化承载；一个 project 可有多个，每个独立 run/SSE/timeline |
| 设计总监节点（Design Director Node） | 生成流水线首节点，产出 `design_spec`，把 frontend-design 原则落成本项目的设计语言 |
| design_spec | 一个结构化产物（palette/type/layout/signature），作为逐页生成的统一设计契约 |
| 流水线阶段（Pipeline Stage） | design → plan → generate → validate → deliver 的固定阶段序列，经 `plan`/`progress` 事件外投 |

## 7. 依赖

- [V2-INDEX](README.md)、[OVERVIEW-SCOPE](../v1/00-overview/scope.md)、[AGENT-OVERVIEW](../v1/50-agent/agent-overview.md)、[ARCH-HARNESS](../v1/20-architecture/agent-harness.md)
