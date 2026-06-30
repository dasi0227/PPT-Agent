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

| # | 里程碑 | 目标 | 关键交付 |
|---|---|---|---|
| M0 | 骨架与契约 | 跑通空架子 | 后端分层目录、SQLite 建表、OpenAPI、前端脚手架、CI |
| M1 | Run 外壳 + Harness + LLM + SSE | 能跑一次工具化 ReAct 循环 | Run 状态机、SSE、DeepSeek `CallTool`、HITL、harness loop+门控+停止条件 |
| M2 | 大纲生成（两阶段第一步） | 主题→slide-json | outline agent、slide-json 校验、落库+版本 |
| M3 | 资产协议 + 设计系统 + slide 生成 | 大纲→slide html | tokens/base、seed 资产、generate 子代理、write/validate_slide、lint-slide |
| M4 | 在线预览 | 看得见、能翻页 | iframe 预览、postMessage、总览、深链 |
| M5 | 局部 patch 编辑 + 指令系统 | 能精确改 | patch_slide(锚定)、current/page/overview/repo 工具门控、prompt/recap/talk/ask、回滚 |
| M6 | 个人仓库（4 类资产） | 资产增删改 + 移植 | assets CRUD、asset-manifest 校验、search/mount/apply_theme |
| M7 | 打磨与验收 | 端到端可用 | e2e、性能、文档对齐、全量 AC 通过 |

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

### M4 — 在线预览
- verifies: [AC-VIEWER-001, AC-VIEWER-005, AC-VIEWER-006, AC-PREVIEW-002, AC-PREVIEW-003]
- 交付：预览运行时（切页/分步/缩放）、iframe host、postMessage 协议、总览网格、深链。

### M5 — 局部 patch 编辑 + 指令系统
- verifies: [AC-EDIT-003, AC-EDIT-004, AC-EDIT-005, AC-VERSION-004, AC-CMD-CURRENT-001, AC-CMD-PAGE-001, AC-CMD-PAGE-003, AC-CMD-OVERVIEW-001, AC-CMD-OVERVIEW-003, AC-CMD-PROMPT-001, AC-CMD-RECAP-001, AC-CMD-TALK-001, AC-CMD-ASK-001, AC-CMD-ASK-002, AC-CMD-REPO-001, AC-MODE-005, AC-CTX-001, AC-HARNESS-001]
- 交付：command 解析（scope 四件套 + mode）、patch_slide 锚定编辑、current/page/overview/repo 工具门控隔离、/overview 子代理跨页 patch、prompt/recap/talk/ask、版本回滚。

### M6 — 个人仓库（4 类资产）
- verifies: [AC-REPO-001, AC-REPO-004, AC-REPO-006, AC-ASSET-001, AC-ASSET-002, AC-ASSET-003, AC-COMP-002, AC-SEED-004]
- 交付：assets CRUD（`/assets` + `/repo` 工具）、asset-manifest schema 校验、search_assets/mount_asset/apply_theme、component 移植、参数填充、seed 冷启动兜底。

### M7 — 打磨与验收
- verifies: [全量 P0 AC]
- 交付：端到端流程（主题→生成→预览→编辑→沉淀）、性能与稳定性、文档与实现对齐核对。

## 依赖顺序

```
M0 → M1 → M2 → M3 → M4
                 └────→ M5 → M6 → M7
```
M4 依赖 M3 产物（有 slide 才能预览）；M5 依赖 M3/M4；M6 依赖 M5。

## 校验方式

每里程碑结束运行其 `verifies` 列出场景的校验命令（见各 AC 的「校验方式」）。

## 依赖

- [SPEC-ACCEPTANCE](../10-spec/acceptance-criteria.md)
