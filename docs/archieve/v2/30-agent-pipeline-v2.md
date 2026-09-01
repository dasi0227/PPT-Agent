---
id: V2-AGENT-PIPELINE
title: Agent 生成流水线 v2（多节点 + 设计总监 + plan 事件）
status: draft
owner: agent
depends_on: [V2-BACKGROUND, ARCH-HARNESS, ARCH-TOOLS, AGENT-PROMPTS]
verifies: [V2-G4, V2-G5]
---

# Agent 生成流水线 v2

> R0 状态（2026-08-01）：公共 Run 协议已切换为 Artifact Target。本文后续出现的 `generate/edit` 只描述 presentation Runner 内部的物化/修改实现，不再是 API kind；`overview/page` 只描述既有内部执行器，不再是公共 level。

需求 3：把 Agent 从"单 ReAct 循环骨架"升级为"多节点前端构建流水线"，集成 Claude 官方 `frontend-design`
skill，实现更完整的分步处理与更完善的校验/交付。本文件是后端 Agent 层的 v2 核心设计。

## 1. 设计原则（继承 v1，不推翻）

- **三层结构不变**：Run 外壳 → Harness（ReAct + capability 门控）→ Tools。流水线由
  `RunnerResolver` 按 `blueprint|presentation × slide|deck` 选择，再由 presentation Runner 内部推导 materialize/revise。
- **LLM 只做认知，工具做副作用**（ARCH-HARNESS-002）：新节点同样通过工具落盘。
- **可观测**：每个阶段/步骤经 SSE 投影（新增 `plan`/`plan.update`，复用 `progress`）。
- **不做通用 DAG 引擎**（用户确认）：固定阶段序列，用 Go 编排，简单可测。

## 2. 流水线全景

所有四种 Artifact Target 在进入本流水线或单页精简路径前，统一经过 Context Engineering v1：

```text
resolve thread/project → validate WorkSpec → assemble ContextPack
→ persist ContextManifest → context.assembled → resolve runner
```

Runner builder 接收类型化 `ContextPack`；Prompt 由稳定分区的 `PromptCompiler` 生成，Runner
不得再次从文件系统手工拼装项目上下文。大型 HTML、相邻 Blueprint、历史证据和大工具结果使用
当前 Run 绑定的 `ContextRef` 渐进披露。Context Engine 只负责读、选、预算与编译，不负责
PEV 的步骤规划、写 capability 或提交事务。

把 v1 的"逐页单发"升级为 5 阶段流水线。**只作用于 `presentation/deck` 的首次物化**（单页物化走精简路径，见 §7）：

```text
Run(target=presentation/deck, intent=apply)
  │
  ▼  Stage 1: DESIGN  ── 设计总监节点（LLM 子循环）
  │     产出 design_spec（palette/type/layout/signature）→ 落盘 design/design-spec.json
  │     注入 frontend-design 原则；写公共层 tokens.css/base.css
  │
  ▼  Stage 2: PLAN    ── 规划节点（确定性 + 轻 LLM）
  │     基于大纲 + design_spec，生成逐页构建计划 → emit plan 事件
  │     每页一个 step（含 layout 决策、内容要点、图表意图、动效意图）
  │
  ▼  Stage 3: GENERATE ── 逐页生成（每页一个 harness 子代理）
  │     每 step：write_slide → validate_slide → finish；emit plan.update(step)
  │     子代理注入 design_spec 摘要，保证跨页设计语言一致
  │
  ▼  Stage 4: VALIDATE ── 全局校验节点（确定性）
  │     跨页一致性 lint（token 使用/16:9/alt/引用公共层/无硬编码色）
  │     不合格页 → 回灌 Stage 3 修复子循环（有限次）
  │
  ▼  Stage 5: DELIVER  ── 交付节点
  │     汇总产出（页数、主题、signature、告警）→ done(result 结构化)
  ▼
done
```

### 2.1 各阶段的 SSE 投影

| 阶段 | 主要事件 | 说明 |
|---|---|---|
| DESIGN | `progress{stage:"design"}` → `artifact{design_spec}` → `thought/tool_call/tool_result` | 设计语言确定 |
| PLAN | **`plan`（新增）** | 一次性发出完整步骤列表，驱动前端 PlanCard |
| GENERATE | `progress{stage:"page"}` + **`plan.update`（新增）** + `artifact{slide_html}` | 每页开始/完成更新对应 step |
| VALIDATE | `progress{stage:"validate"}` + `plan.update`（修复步） | 校验与修复 |
| DELIVER | `done{result}` | 结构化交付 |

## 3. Stage 1：设计总监节点（Design Director）

### 3.1 职责

把 **Claude 官方 `frontend-design` skill**（安装于 `~/.claude/plugins/marketplaces/claude-plugins-official/plugins/frontend-design/skills/frontend-design/SKILL.md`，仓库外资源）
的原则**落成本项目的一份具体设计语言**，作为后续所有页的统一契约。这解决 v1"各页各写各的、无整体设计统筹"。

### 3.2 frontend-design 集成方式

**注入而非调用**：frontend-design 是一份"设计方法论 SKILL"，v2 不在运行时去"执行 Claude skill"，而是把它的
**核心原则蒸馏进设计总监节点的 system prompt**（见 [60-prompts-v2](60-prompts-v2.md#design-director-system)），要求 LLM：

1. **锚定主题世界**（Ground it in the subject）：从 PPT 主题的真实语汇/材料/受众出发，先命名 subject + audience + 单一目标。
2. **排版即人格**（Typography carries personality）：为 display / body / utility 三个角色选择**刻意的、非默认**字体配对与字号阶梯。
3. **结构即信息**（Structure is information）：编号/眉标/分割线只在真的承载信息时使用，不做装饰。
4. **克制的动效**（Motion deliberately）：一个编排好的时刻胜过散落特效。
5. **避开 AI 默认三件套**：cream+serif+terracotta / near-black+acid-green / broadsheet hairline——除非 brief 明确要求，否则不落入这三种默认。
6. **signature 元素**：为整份 PPT 定义一个"被记住的独特元素"。

> 约束 `V2-DESIGN-001`：设计总监 system prompt MUST 蒸馏上述原则；frontend-design 原文以**离线蒸馏**方式进入 prompt 模板，
> 不在运行时依赖外部 skill 文件（保持后端零外部运行时依赖）。

### 3.3 产出：design_spec（新产物）

设计总监通过新工具 `submit_design_spec` 提交一份结构化设计语言，落盘 `design/design-spec.json` 并 emit `artifact{design_spec}`：

```json
{
  "subject": { "topic": "云原生可观测性", "audience": "工程决策者", "job": "说服采用统一观测方案" },
  "palette": [
    { "name": "ink",     "hex": "#12161C", "role": "背景/正文" },
    { "name": "signal",  "hex": "#3BA7A0", "role": "主强调（数据/链路）" },
    { "name": "amber",   "hex": "#E0A340", "role": "告警/次强调" },
    { "name": "paper",   "hex": "#F5F3EC", "role": "浅底" }
  ],
  "type": {
    "display": { "family": "Space Grotesk", "weights": [500,700], "usage": "大标题，克制使用" },
    "body":    { "family": "Inter",         "weights": [400,600], "usage": "正文与要点" },
    "utility": { "family": "IBM Plex Mono",  "weights": [400],     "usage": "数据/标签/代码" }
  },
  "layout": { "grid": "12-col", "rhythm": "宽留白 + 左对齐信息块", "concept": "指标先行，链路为骨架" },
  "signature": "每页右下角一条细的‘链路脉冲’SVG，随主题色律动，串联全篇观测叙事",
  "motion": { "policy": "仅首屏 rise-in + 关键数字 counter-up，其余静默" }
}
```

### 3.4 与公共层的关系

- 设计总监节点在产出 design_spec 后，**据此写公共层** `common/tokens.css`（把 palette/type 落成 CSS 变量）与 `common/base.css`（16:9 舞台基座，沿用 v1）。
- 这替换 v1 `writeCommon`（原为"拷贝所选主题 tokens"）的一部分：v2 允许 design_spec 生成**项目专属 tokens**，也允许在用户指定主题时以主题为基底再叠加 signature（见 §6 主题协同）。
- 落盘仍产 `design` 版本（对齐 v1 versioning）。

## 4. Stage 2：规划节点（Plan）与 plan 事件

### 4.1 职责

读大纲 slide-json[] + design_spec，产出**逐页构建计划**。每页一个 step，明确该页的 layout 决策、内容职责、图表/动效意图。

### 4.2 plan 事件（新增契约）

```text
event: plan
data: {
  "id": "plan_<runId>",
  "title": "构建 8 页 · 主题 云原生可观测性",
  "steps": [
    { "id": "s0", "title": "封面 · 链路脉冲开场", "status": "pending", "detail": "hero=大标题+signature" },
    { "id": "s1", "title": "第2页 · 问题现状（bullets）", "status": "pending" },
    ...
    { "id": "sN", "title": "全局校验与交付", "status": "pending" }
  ]
}
```

```text
event: plan.update
data: { "id": "plan_<runId>", "step_id": "s1", "status": "in_progress" | "completed" | "failed" | "skipped", "detail?": "..." }
```

- 状态枚举：`pending | in_progress | completed | failed | skipped`（对齐 m7-design-spec §4.3 预留）。
- **约束 `V2-PLAN-001`**：`plan` 在 PLAN 阶段发且仅发一次；此后仅用 `plan.update` 增量更新单个 step。
- **约束 `V2-PLAN-002`**：`plan.update.step_id` MUST 命中已发 `plan` 中的 step；否则前端忽略并记告警。
- **约束 `V2-PLAN-003`**：`plan`/`plan.update` 与 `progress{stage:page}` 并存不冲突——plan 面向"步骤清单"，progress 面向"当前进度条"。

### 4.3 规划的确定性兜底

规划节点优先用**确定性 Go**从大纲直接生成 steps（每页一条 + design/validate/deliver 各一条），可选用一次轻量 LLM 调用润色 step 标题与 layout 决策。LLM 失败时回退纯确定性计划，保证 `plan` 事件始终可发。

## 5. Stage 3：逐页生成（子代理）+ 设计语言一致性

沿用 v1 "每页一个 harness 子代理，独立上下文"（ARCH-HARNESS-005），但增强：

- **注入 design_spec 摘要**：每个子代理 system/user prompt 携带 design_spec 的 palette/type/signature/该页 layout 决策（见 [60-prompts-v2](60-prompts-v2.md#slide-gen-v2)）。这保证 8 页共享同一设计语言，而非各写各的。
- **step 生命周期**：开始该页 → `plan.update(step, in_progress)`；写盘+校验通过 → `plan.update(step, completed)` + `artifact{slide_html}`；失败 → `plan.update(step, failed)`。
- **工具集不变**：`write_slide` + `validate_slide` + `finish`（+ 可选 `search_assets`/`mount_asset` 复用资产）。
- **防空页**：沿用 v1"finish 但未写入视为失败"防守。

## 6. 主题协同（design_spec × 已有主题资产）

| 场景 | 行为 |
|---|---|
| 用户未指定主题 | 设计总监**自由创作** design_spec 并生成项目专属 tokens |
| 用户指定了 theme 资产 | 设计总监以该主题 tokens 为**基底**，只在 signature/layout/motion 层做项目化增量，不覆盖主题色板 |
| 用户在 Overview 换主题 | 走 v1 `apply_theme`；design_spec 的 signature 保留（记录在 design-spec.json） |

> 这让 v2 既能"AI 出彩"（无主题时），又尊重"用户已选主题"（不喧宾夺主），与 v1 仓库主题体系兼容。

## 7. 单页物化 / 修改的精简路径（不过度设计）

- **单页首次物化**（`presentation/slide` 且无 HTML）：**不跑完整流水线**。读取该页 Blueprint 与 design spec → 单页子代理生成 → 校验 → done。
- **单页修改**（`presentation/slide` 且已有 HTML）：复用确定性的 HTML patch/validate/version 实现，不引入流水线。
- **整份修改**（`presentation/deck` 且已有 HTML）：使用 deck runner 处理全局设计或跨页修改。
- repo 不属于 target；资产仅通过受控 `search_assets/read_assets/mount_assets` capability 提供。

> 原则：流水线只在"整套从大纲到成品"的高价值场景启用；高频小编辑保持低延迟。

## 8. 校验与交付强化（Stage 4/5）

### 8.1 Stage 4 全局校验

在逐页 lint（v1 `designsystem.LintSlide`）之上，增加**跨页一致性**检查（确定性）：

| 检查项 | 规则 |
|---|---|
| 公共层引用 | 每页均正确 `<link>` tokens.css + base.css，顺序正确 |
| 无硬编码主题色 | 复用 v1 lint（禁 `#rrggbb`/`rgb()` 等主题色硬编码） |
| 16:9 舞台结构 | 每页含 `slide-scaler > slide-stage` |
| img alt | 每个 `<img>` 有 alt |
| 设计语言一致 | 抽样校验字体族/主色 token 是否落在 design_spec 声明集合内 |

不合格页 → 触发一次**有限次修复子循环**（复用 Stage 3 子代理 + 校验错误 observation 回灌），超过阈值则该页标 `failed` 并在交付告警中列出，不阻塞整体 done。

### 8.2 Stage 5 结构化交付

`done` 的 result 从 v1 的 `{summary}` 升级为结构化：

```json
{
  "project_id": "p1",
  "slide_count": 8,
  "theme": "project-custom | swiss-modern",
  "design_spec_ref": "design/design-spec.json",
  "signature": "链路脉冲",
  "warnings": [ { "page_index": 3, "code": "FIX_EXCEEDED", "message": "示意图未按意图生成，已用占位" } ]
}
```

前端据此渲染 FinalResultCard（最终交付卡，视觉层级最高）。

## 9. 停止条件与熔断（继承 + 扩展）

沿用 v1 MAX_TURNS/finish/circuit/canceled 四条（ARCH-HARNESS-STOP）。流水线级补充：

| ID | 条件 |
|---|---|
| `V2-STOP-001` | DESIGN 阶段失败（无法产出合法 design_spec）→ 整体 failed，不进入 PLAN |
| `V2-STOP-002` | 单页连续修复超过 `MAX_FIX_ROUNDS`（默认 2）→ 该页标 failed，计入 warnings，继续其余页 |
| `V2-STOP-003` | Run 取消 → 各阶段安全终止，保留已落盘页与 design_spec |

## 10. 新增工具与节点清单（供后端实现）

| 名称 | 类型 | 归属包（建议） | 作用 |
|---|---|---|---|
| `submit_design_spec` | 写·design | `internal/agent/generate`（或新 `designdirector`） | 校验并落盘 design-spec.json，产 `design` 版本，emit artifact |
| 设计总监子循环 | 节点 | 同上 | 一个 harness.Loop，工具集 `{submit_design_spec, finish}` |
| 规划节点 | 确定性+轻LLM | 同上 | 产 steps，emit `plan` |
| 全局校验节点 | 确定性 | 复用 `designsystem` | 跨页 lint，触发修复 |
| 交付节点 | 确定性 | 同上 | 汇总 result |

> 事件侧新增 `EventPlan`、`EventPlanUpdate`（`model/event.go`）；`plan`/`plan.update` **非终态**。契约详见 [40-api-and-data-contracts](40-api-and-data-contracts.md)。

## 11. 验收标准（Given-When-Then）

- **AC-V2-PIPE-001**（`V2-G4`）
  - GIVEN 一个已有大纲的 project
  - WHEN 发起整套 generate
  - THEN SSE 依次出现 `progress{design}` → `artifact{design_spec}` → `plan` → 若干 `plan.update` → `done{结构化 result}`

- **AC-V2-PIPE-002**（`V2-G5`）
  - GIVEN 未指定主题
  - WHEN 设计总监节点完成
  - THEN design-spec.json 含非空 palette(≥3)/type(display+body)/signature，且不等于 AI 默认三件套之一

- **AC-V2-PIPE-003**（`V2-G4`）
  - GIVEN 生成进行中
  - WHEN 第 k 页开始与完成
  - THEN 分别收到 `plan.update{step_k, in_progress}` 与 `plan.update{step_k, completed}`，step_id 命中已发 plan

- **AC-V2-PIPE-004**（`V2-STOP-002`）
  - GIVEN 某页两次修复仍不合规
  - WHEN 校验阶段结束
  - THEN 该页标 failed 并进入 `done.result.warnings`，其余页正常交付

- **AC-V2-PIPE-005**
  - GIVEN 单页重生成（带 page_index）
  - WHEN 执行
  - THEN 走精简路径，不发跨页 `plan`（或仅单步），只动该页

## 12. 校验方式

```bash
go test ./internal/agent/generate -run 'TestPipelineStages|TestDesignSpecEmitted|TestPlanEvents|TestFixLoopBounded'
go test ./internal/harness -run 'TestReActLoop|TestStopConditions'   # 回归 v1 不破坏
```

## 13. 依赖

- [V2-BACKGROUND](00-background-and-goals.md)、[ARCH-HARNESS](../v1/20-architecture/agent-harness.md)、[ARCH-TOOLS](../v1/20-architecture/tools.md)、[60-prompts-v2](60-prompts-v2.md)、[40-api-and-data-contracts](40-api-and-data-contracts.md)
