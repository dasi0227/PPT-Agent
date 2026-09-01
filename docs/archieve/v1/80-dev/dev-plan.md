---
id: DEV-PLAN
title: 开发计划（里程碑）
status: approved
owner: shared
depends_on: [SPEC-ACCEPTANCE]
verifies: []
---

# 开发计划（里程碑）

迭代式推进。每个里程碑用 `verifies:` 列出其交付所验证的验收场景（来自 [acceptance-criteria](../10-spec/acceptance-criteria.md)），
完成判据 = 这些场景的校验命令通过 + 满足 [definition-of-done](definition-of-done.md)。

## 里程碑总览

> 推进策略：**后端一路做到底**（M0→M6 全在后端，命令行/测试可验），**前端预览集中到 M7**。避免中途在前后端间来回切换。

| # | 里程碑 | 目标 | 关键交付 |
|---|---|---|---|
| M0 | 骨架与契约 | 跑通空架子 | 后端分层目录、SQLite 建表、OpenAPI、前端脚手架、CI |
| M1 | Run 外壳 + Harness + LLM + SSE | 能跑一次工具化 ReAct 循环 | Run 状态机、SSE、DeepSeek `CallTool`、HITL、harness loop+门控+停止条件 |
| M2 | 大纲生成（两阶段第一步） | 主题→slide-json | outline agent、slide-json 校验、落库+版本 |
| M3 | 资产协议 + 设计系统 + slide 生成 | 大纲→slide html | tokens/base、seed 资产、generate 子代理、write/validate_slide、lint-slide |
| M4 | 编辑核心（单页 scope） | 能精确改单页 | patch_slide(锚定)、`/current` `/page` 工具门控、版本回滚 |
| M5 | 跨页/仓库 scope + 模式 | 全局改 + 只说不做 | `/overview` 子代理跨页 patch、`/repo` 工具、prompt/recap/talk/ask 四模式 |
| M6 | 个人仓库（4 类资产） | 资产增删改 + 移植 | assets CRUD、asset-manifest 校验、search/mount/apply_theme |
| M7 | 在线预览（前端） | 看得见、能翻页 | iframe 预览、postMessage、总览、深链 |
| M8 | 打磨与验收 | 端到端可用 | e2e、性能、文档对齐、全量 AC 通过 |

## 里程碑详情

### M0 — 骨架与契约
- verifies: [AC-DATA-001, AC-API-001]
- 交付：`backend/` 分层目录、`migrations` 执行 `sqlite-schema.sql`、`openapi.yaml` 通过 lint、`frontend/` Vite 起步、基础 CI（vet/test/tsc）。

### M1 — Run 外壳 + Harness + LLM + SSE
- verifies: [AC-GLOBAL-001, AC-GLOBAL-002, AC-RUN-002, AC-RUN-005, AC-RUN-006, AC-SSE-001, AC-SSE-003, AC-LLM-002, AC-LLM-003, AC-RUN-API-001, AC-RUN-API-004, AC-RUN-API-005, AC-HARNESS-001, AC-HARNESS-002, AC-HARNESS-004, AC-TOOLS-003, AC-TOOLS-004]
- 交付：Run 状态机/事件总线/控制输入队列/checkpoint；harness ReAct loop、动态工具门控、停止条件；工具框架（registry + patch/validate 原语）；DeepSeek `CallTool`/流式；SSE 端点与续传（含 thought/tool_call/tool_result）。

### M2 — 大纲生成（两阶段第一步）
- verifies: [AC-OUTLINE-001, AC-OUTLINE-002, AC-OUTLINE-007]
- 交付：outline agent、prompt 模板 `outline.gen`、slide-json schema 校验、落库 + version 0。

### M3 — 资产协议 + 设计系统 + slide 生成
- verifies: [AC-GEN-001, AC-GEN-002, AC-GEN-006, AC-GEN-009, AC-TOKENS-001, AC-TOKENS-002, AC-LAYOUTS-001, AC-CHARTS-001, AC-HTML-001, AC-ASSET-001, AC-ASSET-002, AC-SEED-001]
- 交付：`tokens.css`/`base.css`、asset 协议与 schema、seed 资产载入、5 套主题、版式/组件、generate 子代理（write_slide）、lint-slide 脚本。

### M4 — 编辑核心（单页 scope）
- verifies: [AC-EDIT-003, AC-EDIT-005, AC-VERSION-004, AC-CMD-CURRENT-001, AC-CMD-PAGE-001, AC-CMD-PAGE-003, AC-CTX-001, AC-HARNESS-001]
- 交付：command 解析基础（scope 识别）、`patch_slide` 锚定编辑、`/current` 与 `/page x` 工具门控隔离（只动目标页）、上下文 scope 隔离、版本回滚。
- 说明：单页编辑是最高频场景，先跑通「怎么精确改一页」；跨页与模式留到 M5。全部经命令行/测试验证，无需 UI。

### M5 — 跨页/仓库 scope + 模式
- verifies: [AC-EDIT-004, AC-CMD-OVERVIEW-001, AC-CMD-OVERVIEW-003, AC-CMD-REPO-001, AC-CMD-PROMPT-001, AC-CMD-RECAP-001, AC-CMD-TALK-001, AC-CMD-ASK-001, AC-CMD-ASK-002, AC-MODE-005]
- 交付：`/overview` 改公共层 + 必要时子代理逐页 patch、`/repo` 资产工具门控、四个 mode（prompt/recap/talk/ask）的行为与工具收窄。
- 说明：在 M4 编辑引擎之上补齐「改哪里、用什么模式」的完整指令系统。

### M6 — 个人仓库（4 类资产）
- verifies: [AC-REPO-001, AC-REPO-004, AC-REPO-006, AC-ASSET-001, AC-ASSET-002, AC-ASSET-003, AC-COMP-002, AC-SEED-004]
- 交付：assets CRUD（`/assets` + `/repo` 工具）、asset-manifest schema 校验、search_assets/mount_asset/apply_theme、component 移植、参数填充、seed 冷启动兜底。

### M7 — 在线预览（前端）
- verifies: [AC-VIEWER-001, AC-VIEWER-005, AC-VIEWER-006, AC-PREVIEW-002, AC-PREVIEW-003]
- 交付：预览运行时（切页/分步/缩放）、iframe host、postMessage 协议、总览网格、深链。
- 说明：后端 M0–M6 全部完成后，前端集中一次做完；此前用命令行验证后端产物。

### M8 — 打磨与验收
- verifies: [全量 P0 AC]
- 交付：端到端流程（主题→生成→预览→编辑→沉淀）、性能与稳定性、文档与实现对齐核对。

## 依赖顺序

```
M0 → M1 → M2 → M3 → M4 → M5 → M6 → M7 → M8
                     （编辑核心）（跨页/模式）（仓库）（前端预览）（打磨）
```
- M4/M5/M6 是纯后端，均可用命令行 + `go test` 验证，不依赖预览 UI。
- M7 前端预览依赖 M3 起的 slide 产物；放到后端收口后集中做。
- M8 依赖全部前序。

## 校验方式

每里程碑结束运行其 `verifies` 列出场景的校验命令（见各 AC 的「校验方式」）。

## 依赖

- [SPEC-ACCEPTANCE](../10-spec/acceptance-criteria.md)
