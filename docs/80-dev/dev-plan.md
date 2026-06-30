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
| M0 | 骨架与契约 | 跑通空架子 | 后端目录、SQLite 建表、OpenAPI、前端脚手架、CI |
| M1 | Run 引擎 + LLM + SSE | 能流式跑一次 LLM | Run 状态机、SSE、DeepSeek 客户端、HITL 输入 |
| M2 | 大纲生成 | 主题→slide-json | outline agent、slide-json 校验、落库+版本 |
| M3 | 设计系统 + slide 生成 | 大纲→slide html | tokens/base、5 主题、版式库、生成 agent、lint-slide |
| M4 | 在线预览 | 看得见、能翻页 | iframe 预览、postMessage、总览、深链 |
| M5 | 引导编辑 + 指令系统 | 能改 | page/overview 隔离、prompt/recap/talk/ask、版本回滚 |
| M6 | 个人仓库 | 插件收藏与移植 | manifest 校验、CRUD、AI 移植 |
| M7 | 打磨与验收 | 端到端可用 | e2e、性能、文档对齐、全量 AC 通过 |

## 里程碑详情

### M0 — 骨架与契约
- verifies: [AC-DATA-001, AC-API-001]
- 交付：`backend/` 分层目录、`migrations` 执行 `sqlite-schema.sql`、`openapi.yaml` 通过 lint、`frontend/` Vite 起步、基础 CI（vet/test/tsc）。

### M1 — Run 引擎 + LLM + SSE
- verifies: [AC-GLOBAL-001, AC-GLOBAL-002, AC-RUN-002, AC-RUN-005, AC-RUN-006, AC-SSE-001, AC-SSE-003, AC-LLM-002, AC-LLM-003, AC-RUN-API-001, AC-RUN-API-004, AC-RUN-API-005]
- 交付：Run 状态机、事件总线、控制输入队列、checkpoint、DeepSeek 流式客户端、SSE 端点与续传。

### M2 — 大纲生成
- verifies: [AC-OUTLINE-001, AC-OUTLINE-002, AC-OUTLINE-007]
- 交付：outline agent、prompt 模板 `outline.gen`、slide-json schema 校验、落库 + version 0。

### M3 — 设计系统 + slide 生成
- verifies: [AC-GEN-001, AC-GEN-002, AC-GEN-006, AC-GEN-009, AC-TOKENS-001, AC-TOKENS-002, AC-LAYOUTS-001, AC-CHARTS-001, AC-HTML-001]
- 交付：`tokens.css`/`base.css`、5 套主题、版式库实现、generate agent、lint-slide 脚本。

### M4 — 在线预览
- verifies: [AC-VIEWER-001, AC-VIEWER-005, AC-VIEWER-006, AC-PREVIEW-002, AC-PREVIEW-003]
- 交付：预览运行时（切页/分步/缩放）、iframe host、postMessage 协议、总览网格、深链。

### M5 — 引导编辑 + 指令系统
- verifies: [AC-EDIT-003, AC-EDIT-004, AC-EDIT-005, AC-VERSION-004, AC-CMD-PAGE-001, AC-CMD-PAGE-003, AC-CMD-OVERVIEW-001, AC-CMD-OVERVIEW-004, AC-CMD-PROMPT-001, AC-CMD-RECAP-001, AC-CMD-TALK-001, AC-CMD-ASK-001, AC-CMD-ASK-002, AC-MODE-005, AC-CTX-001]
- 交付：command 解析分派、page/overview scope 隔离、prompt/recap/talk/ask、版本回滚。

### M6 — 个人仓库
- verifies: [AC-REPO-001, AC-REPO-004, AC-PLUGIN-001, AC-PLUGIN-002, AC-PLUGIN-AUTH-001]
- 交付：plugin CRUD、manifest schema 校验、检索与移植、参数填充。

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
