---
id: V2-DEV-PLAN
title: v2 开发计划（里程碑）
status: draft
owner: shared
depends_on: [V2-INDEX, DEV-PLAN, DEV-DOD]
verifies: [V2-G1, V2-G2, V2-G3, V2-G4, V2-G5]
---

# v2 开发计划（里程碑）

迭代式推进，每个里程碑给出交付、验证的 AC、DoD 与依赖。**推进策略：先解阻塞（前端路由）→ 再补广度（多窗口）
→ 后加深度（Agent 流水线）**。前两个里程碑纯前端即可让产品"活过来"，第三、四个里程碑加深 Agent 与可视化。

## 里程碑总览

| # | 里程碑 | 目标 | 关键交付 | verifies | 端 |
|---|---|---|---|---|---|
| V2-M1 | 交互模式打通 | 修复"新建后动不了"，四模式可用 | 类型补齐、modeMapping、ModeSwitcher、EmptyState、智能默认 | V2-G1, V2-G2 | 前端 |
| V2-M2 | 多对话窗口 | 一个 project 多 thread 独立运行 | runStore 分片、ThreadTabs、threadStore、SSE 多路复用 | V2-G3 | 前端 |
| V2-M3 | plan 事件与流水线可视化 | 前端 PlanCard 接真实计划 | 后端 plan/plan.update 事件 + 规划节点；前端 reducer/PlanCard | V2-G4 | 后端+前端 |
| V2-M4 | 设计总监节点 + design_spec | 生成更专业、跨页一致 | submit_design_spec 工具、设计总监子循环、frontend-design 蒸馏 prompt、tokens 由 spec 生成 | V2-G5 | 后端 |
| V2-M5 | 全局校验/交付强化 + 打磨 | 端到端可用、结构化交付 | 跨页一致性 lint、有限修复子循环、结构化 done.result、全量回归 | 全量 | 后端+前端 |

## 依赖顺序

```
V2-M1 ──▶ V2-M2 ──▶ V2-M3 ──▶ V2-M4 ──▶ V2-M5
(前端解阻塞) (多窗口)  (plan事件) (设计总监)  (校验交付+打磨)
```

- V2-M1、V2-M2 是**纯前端**，不依赖后端改动，可立即让产品可用。
- V2-M3 起需要后端配合（新增事件/节点）；前端在 M3 消费 plan。
- V2-M4/M5 加深 Agent，前端只需消费新产物/结构化结果。

---

## 里程碑详情

### V2-M1 — 交互模式打通（P0，解阻塞）

**目标**：任何人新建 project 后能顺利走 outline→generate→edit，不再撞 `BAD_STATE`；四模式可自由切换。

**交付**：
- `api/types.ts`：`RunKind` 补 `outline/command`，`RunPayload` 补 `command/brief/slide_count/language/theme`。
- `features/agent/modeMapping.ts`：映射矩阵纯函数 + 单测。
- `features/agent/ModeSwitcher.tsx`：四档 + talk/ask 副开关 + 智能默认。
- `features/agent/CommandComposer.tsx`：接 ModeSwitcher；行首 `/` 语义交后端；提交正确 payload。
- `features/viewer/EmptyState.tsx`：空项目引导生成大纲。

**verifies**：AC-V2-MODE-001~004、AC-V2-FE-001、AC-V2-CTR-001。

**DoD**：
- 空项目 → Outline → 生成大纲 → Page 生成页 → Page 编辑，全链路无死路（手动 + e2e 冒烟）。
- `pnpm test / tsc --noEmit / build` 全绿；modeMapping 覆盖矩阵每行。

---

### V2-M2 — 多对话窗口（P1，补广度）

**目标**：一个 project 下多 thread 独立运行、独立 SSE、切换零重载。

**交付**：
- `stores/runStore.ts`：重构为 `sessions: Record<threadId, RunSession>`；所有 action 带 threadId。
- `stores/threadStore.ts`（或扩 projectStore）：打开态/聚焦态、`ensureActiveThread`。
- `api/threads.ts`：list/create/history/delete。
- `features/agent/ThreadTabs.tsx`：标签页 + 状态点 + 新建/关闭/历史下拉。
- SSE 多路复用：每 run 一连接，写入对应分片；切 project 关连接。

**verifies**：AC-V2-THREAD-001~004、AC-V2-FE-003。

**DoD**：
- 两 thread 并行 run，状态互不干扰；切 project 再回，打开态与聚焦恢复、活跃 run 续传。
- `pnpm test / tsc / build` 全绿；runStore 分片隔离单测通过。

---

### V2-M3 — plan 事件与流水线可视化（后端+前端）

**目标**：后端在整套生成时发真实 `plan`/`plan.update`；前端 PlanCard 显示真实步骤。

**交付（后端）**：
- `model/event.go`：新增 `EventPlan`、`EventPlanUpdate`（Terminal=false）。
- `internal/agent/generate`：新增**规划节点**（确定性从大纲生成 steps，可选轻 LLM 润色），emit `plan`；逐页生成时 emit `plan.update`。
- `progress.stage` 增 `design`/`validate` 语义预留。
- 事件持久化 + Last-Event-ID 续传测试。

**交付（前端）**：
- `eventReducer.ts`：plan 创建/替换、plan.update 命中 step、未命中忽略。
- `PlanCard.tsx`：接 `session.plan`，五状态渲染 + 折叠；删除 mock。

**verifies**：AC-V2-PIPE-003、AC-V2-CTR-002/003、AC-V2-FE-002。

**DoD**：
- 整套生成事件流出现恰好一个 `plan` + 若干命中的 `plan.update`；断线续传无重复。
- `go test ./internal/run ./internal/agent/generate`；前端 reducer/PlanCard 单测通过。

---

### V2-M4 — 设计总监节点 + design_spec（后端，加深度）

**目标**：整套生成先产出项目专属设计语言（design_spec），逐页据此生成，跨页一致且专业。

**交付**：
- `submit_design_spec` 工具（校验字段完整性 + 落盘 `design/design-spec.json` + `design` 版本 + emit `artifact{design_spec}`）。
- 设计总监子循环（harness.Loop，工具集 `{submit_design_spec, finish}`）作为 Stage 1。
- `internal/agent/prompt`：`design.director@v1`（frontend-design 离线蒸馏）、`slide.gen@v2`（注入 design_spec 摘要）。
- `writeCommon` 升级：tokens.css 可由 design_spec 生成；用户指定主题时以主题为基底叠加 signature。

**verifies**：AC-V2-PIPE-001、AC-V2-PIPE-002。

**DoD**：
- 未指定主题时 design-spec.json 含 palette(≥3)/type(display+body)/signature，且不落入 AI 默认三件套。
- 8 页共享同一字体/主色 token；`go test ./internal/agent/generate -run 'TestDesignSpec*|TestPipelineStages'`。
- 生成结果人工目检：视觉明显区别于模板默认（截图对比）。

---

### V2-M5 — 全局校验/交付强化 + 打磨（后端+前端，收口）

**目标**：跨页一致性校验 + 有限修复 + 结构化交付；端到端可用。

**交付（后端）**：
- Stage 4 跨页一致性 lint（公共层引用/无硬编码色/16:9/alt/设计语言一致）。
- 有限次修复子循环（`MAX_FIX_ROUNDS` 默认 2），超限标 failed 入 warnings。
- Stage 5 结构化 `done.result`（slide_count/design_spec_ref/signature/warnings）。
- 停止条件 V2-STOP-001~003。

**交付（前端）**：
- FinalResultCard 兼容结构化/summary 两种 result；warnings 展示。
- 全量回归 + 视觉打磨（模式色、卡片层级、空态）。

**verifies**：AC-V2-PIPE-004、AC-V2-PIPE-005，全量 P0 AC。

**DoD**：
- 端到端：主题→大纲→设计语言→逐页→校验→交付，含一次失败页的 warnings 展示。
- 全量 `go vet ./... && go test ./...`；`cd frontend && pnpm test && pnpm tsc && pnpm build` 全绿。
- 对照 [definition-of-done](../v1/80-dev/definition-of-done.md) 自检；文档与实现对齐。

---

## 每里程碑通用 DoD（证据驱动）

沿用用户偏好与 v1 纪律：

- **命令输出为证**：贴出 `go test`/`go vet`/`pnpm test`/`tsc`/`build` 的实际结果（file:line/日志）。
- **里程碑前自审**：完成对应 verifies 的 AC 校验命令。
- **提交规范**：`feat(V2-Mx): ...` / `fix(V2-Mx): ...`。
- **最小补丁优先**：局部 diff，跨模块同步（前端/后端/config/docs）消除冗余。
- **不破坏 v1**：每里程碑跑 v1 回归测试（harness/run/httpapi 既有用例）。

## 风险与缓解

| 风险 | 缓解 |
|---|---|
| 前端映射矩阵与后端解析漂移 | 斜杠命令统一交后端解析；矩阵用纯函数单测锁定 |
| runStore 分片改动影响面大 | 先加分片结构、保留旧 API 薄封装，逐组件迁移；充分单测 |
| plan 事件与 progress 语义混淆 | 文档明确二者正交（清单 vs 进度条）；前端分开渲染 |
| design_spec 质量不稳定 | 规划/校验有确定性兜底；修复子循环有限次；warnings 兜底不阻塞 |
| frontend-design 运行时依赖 | 采用离线蒸馏进 prompt，不在运行时读外部 skill 文件 |

## 依赖

- [V2-INDEX](README.md)、[DEV-PLAN](../v1/80-dev/dev-plan.md)、[DEFINITION-OF-DONE](../v1/80-dev/definition-of-done.md)
