# AI PPT Builder — 项目文档（事实源）

> 一个**专注 frontend 的 PPT Coding Agent**：用户用自然语言驱动，Agent 用前端技术（HTML/CSS/JS）生成、编辑、预览 PPT。

本目录是整个项目的**事实源（source of truth）**。所有文档对**人类与 Coding Agent 双可读、可执行、可验收**，作为后续 VibeCoding 实现的依据。**当前阶段只有文档，没有业务代码。**

---

## 如何阅读（人类）

按以下顺序通读即可建立全貌：

1. [00-overview/vision.md](00-overview/vision.md) — 我们在做什么、为什么。
2. [00-overview/scope.md](00-overview/scope.md) — MVP 边界。
3. [10-spec/functional-spec.md](10-spec/functional-spec.md) — 完整功能规格。
4. [20-architecture/system-overview.md](20-architecture/system-overview.md) — 系统怎么搭。
5. [30-data-model/data-model.md](30-data-model/data-model.md) → [40-api/api-overview.md](40-api/api-overview.md) — 数据与接口契约。
6. [50-agent/agent-overview.md](50-agent/agent-overview.md) — Agent 的大脑。
7. [80-dev/dev-plan.md](80-dev/dev-plan.md) — 怎么落地。

## 如何使用（Coding Agent）

- **先读本文件 + [00-overview/scope.md](00-overview/scope.md) + [80-dev/dev-rules.md](80-dev/dev-rules.md)**，建立约束边界。
- 每个文件头部有 YAML front-matter，含 `depends_on` 依赖图。**按需加载**当前任务相关的切片，不要一次吞下全部。
- 实现某功能前，先读对应 `10-spec/feat-*.md` 的**验收标准**与 `30-data-model` / `40-api` 的**机器可校验产物**（Schema / OpenAPI / DDL）。
- 完成后对照 [80-dev/definition-of-done.md](80-dev/definition-of-done.md) 自检。

---

## 文档地图

| 目录 | 职责 | 关键文件 |
|---|---|---|
| [00-overview/](00-overview/) | 愿景、范围、术语、用户旅程、backlog | vision、scope、glossary、personas-and-flows、backlog |
| [10-spec/](10-spec/) | 功能规格 + 验收标准（带 SPEC-ID） | functional-spec、feat-\*、acceptance-criteria |
| [20-architecture/](20-architecture/) | 系统/后端/前端结构、**harness（ReAct+工具）**、Run 外壳、预览、LLM | system-overview、backend-structure、frontend-structure、**agent-harness**、**tools**、agent-runtime、preview-mechanism、llm-integration |
| [30-data-model/](30-data-model/) | 实体、SQLite DDL、文件布局、slide-json schema、版本 | data-model、sqlite-schema.sql、filesystem-layout、slide-json.schema.json、versioning |
| [40-api/](40-api/) | REST + SSE 契约、OpenAPI、run 生命周期 | api-overview、openapi.yaml、rest-endpoints、sse-events、run-lifecycle |
| [50-agent/](50-agent/) | 指令系统（scope 四件套+mode）、prompt、上下文装配 | agent-overview、commands/\*（current/page/overview/repo/prompt/recap/talk/ask）、modes、prompt-templates、context-assembly |
| [60-design-system/](60-design-system/) | **统一资产协议**（4 类）、token、主题/版式/组件/图表/动效、HTML 产出、seed | asset-protocol、asset-manifest.schema.json、design-tokens、themes、layouts、components、charts、animations、html-output-spec、seed-assets |
| [80-dev/](80-dev/) | 开发计划、规则、编码规范、测试策略、DoD | dev-plan、dev-rules、coding-standards、test-strategy、definition-of-done |
| [90-decisions/](90-decisions/) | ADR 架构决策记录 | 0001~0010 |

---

## 文档约定

### front-matter

每个规格类 `.md` 文件头部含：

```yaml
---
id: SPEC-OUTLINE-001        # 稳定唯一 ID
title: 大纲生成
status: draft               # draft | approved | implemented | deprecated
owner: backend              # backend | frontend | agent | shared
depends_on: [DATA-SLIDE-001, API-RUN-001]   # 依赖的其它文档 ID
verifies: []                # 本文档验证了哪些上游需求（开发计划用）
---
```

### 需求 ID 命名

| 前缀 | 含义 | 示例 |
|---|---|---|
| `SPEC-<域>-<编号>` | 功能需求 | `SPEC-VIEWER-003`、`SPEC-CMD-PAGE-001` |
| `ARCH-<域>-<编号>` | 架构约束 | `ARCH-BACKEND-002` |
| `DATA-<域>-<编号>` | 数据约束 | `DATA-SLIDE-001` |
| `API-<域>-<编号>` | API 契约 | `API-RUN-001` |
| `ADR-<四位编号>` | 架构决策 | `ADR-0001` |

### 状态图例

| status | 含义 |
|---|---|
| `draft` | 草稿，可能变更 |
| `approved` | 已评审通过，可据此开发 |
| `implemented` | 已实现并通过验收 |
| `deprecated` | 已废弃 |

### 统一规格骨架

规格类文档统一含：`目标` / `范围与非目标` / `规格正文` / `验收标准(Given-When-Then)` / `校验方式` / `依赖`。

---

## 已锁定技术栈

| 维度 | 选型 | ADR |
|---|---|---|
| 后端 | Go + Gin | [ADR-0001](90-decisions/0001-backend-go-gin.md) |
| 后端库栈 | GORM + modernc(sqlite) + Viper + zap + wire | [ADR-0012](90-decisions/0012-backend-lib-stack.md) |
| 持久化 | SQLite + 文件系统 | [ADR-0002](90-decisions/0002-persistence-sqlite-fs.md) |
| 前端 | React + Vite + TS + Tailwind + shadcn/ui | [ADR-0003](90-decisions/0003-frontend-react-vite.md) |
| 传输 / HITL | SSE + 控制 POST | [ADR-0004](90-decisions/0004-transport-sse-hitl.md) |
| 主题系统 | design tokens + 公共 CSS 层 | [ADR-0005](90-decisions/0005-theme-tokens-css-layer.md) |
| Agent Harness | ReAct 循环 + function calling + 动态工具门控 + 子代理 | [ADR-0007](90-decisions/0007-harness-react-tools.md) |
| LLM 接入 | DeepSeek（interface 抽象，假设 fc 良好） | [ADR-0008](90-decisions/0008-llm-interface-abstraction.md) |
| 个人仓库 | 统一资产协议（4 类：layout/component/theme/fx） | [ADR-0009](90-decisions/0009-unified-asset-protocol.md) |
