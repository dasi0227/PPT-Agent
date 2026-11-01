---
id: DESIGN-DOCS-SCAFFOLD
title: AI PPT Builder — docs 脚手架设计
status: approved
date: 2026-06-30
owner: VibeCoding
---

# AI PPT Builder — docs 脚手架设计

## 目标

为「专注 frontend 的 PPT Coding Agent」项目生成一套 SDD 驱动的 `docs/` 目录。
所有文档对**人类与 Coding Agent 双可读、可执行、可验收**，作为后续 VibeCoding 实现的「事实源（source of truth）」。
本设计仅产出文档脚手架，不写业务代码。

## 范围与非目标

### 范围
- 生成 `docs/` 全量 SDD 文档（overview / spec / architecture / data-model / api / agent / design-system / plugins / dev / decisions）。
- 机器可校验产物写真实内容：JSON Schema、OpenAPI、SQLite DDL。
- 统一文档骨架，使「可读/可执行/可验收」可落地。

### 非目标
- 不实现任何后端 / 前端业务代码。
- 不实现图片导出、框选编辑、PPTX 转换（列入 backlog，文档预留接口）。
- 不做演讲者模式。

## 已锁定技术决策（来源：2026-06-30 头脑风暴）

| 维度 | 决策 |
|---|---|
| 后端 | Go + net/http + chi 路由 |
| 持久化 | SQLite（元数据/版本/插件索引）+ 文件系统（slide html/css/js） |
| 前端 | React + Vite + TypeScript + Tailwind + shadcn/ui；Zustand；沙箱 iframe 预览 |
| LLM | DeepSeek API |
| 传输 / HITL | SSE（输出）+ POST /runs/{id}/input（控制输入）；run 抽象 + checkpoint + needs_input |
| 主题系统 | design tokens + 公共 CSS 层；/overview 改的就是这层 |
| 插件协议 | 统一 manifest（manifest.json + 隔离 html/css/js + 参数 schema + 挂载约定） |
| 文档语言 | 中文（代码/标识符/API 路径/Schema 保持英文） |

## 参考项目提炼（能力目标，非组织范本）

参考 `zarazhangrui/frontend-slides` 与 `lewislulu/html-ppt-skill`，提炼出本 Agent 要交付的产品能力：
主题换肤（token 驱动）、版式库、图表样式、动效库（CSS + Canvas FX）、iframe 隔离预览 +
postMessage 切页不刷新、Overview 网格总览 + 深链跳页、主题/简报一键生成整套 HTML slides、
「Show don't tell」风格发现、零依赖单 HTML、固定 16:9、中英文一等公民。
这些作为**功能规格输入**写入 `10-spec`，不模仿其 SKILL 组织方式。

## MVP 范围

- 输入主题 → 生成 PPT 大纲 slide-json。
- 按设计系统（主题/版式/图表/动效）生成 slide html。
- 自然语言引导编辑 slide html。
- 在线查看：上下页、上下步骤、缩放总览后点击跳转。
- 个人仓库（统一 manifest 插件协议）。
- 自定义指令：`/page x`、`/overview`、`/prompt`、`/recap`、`/talk`、`/ask`。
- 无登录注册。

## 文档架构（Approach B — 分层 SDD）

按关注点分目录，每个文件单一职责、可被 Agent 独立按需加载。完整文件清单见
`docs/README.md` 的文档地图。顶层目录：

```
docs/
├── README.md            文档地图、ID/front-matter 约定、状态图例
├── 00-overview/         愿景、范围、术语、用户旅程、backlog
├── 10-spec/             功能规格 + 验收标准（带 SPEC-ID）
├── 20-architecture/     系统/后端/前端结构、agent runtime、预览机制、LLM 接入
├── 30-data-model/       实体、SQLite DDL、文件布局、slide-json schema、版本模型
├── 40-api/              REST + SSE 契约、OpenAPI、run 生命周期
├── 50-agent/            指令系统、模式、prompt 模板、上下文装配
├── 60-design-system/    token、主题、版式、图表、动效、HTML 产出规范
├── 70-plugins/          manifest 协议 + schema + 挂载流程
├── 80-dev/              开发计划、规则、编码规范、测试策略、DoD
└── 90-decisions/        ADR 决策记录
```

## 全局约定（落地「可读/可执行/可验收」）

### 可读
- 每个规格类文件含 YAML front-matter：`id` / `title` / `status` / `owner` / `depends_on` / `verifies`。
- 正文标准 Markdown + 路由表；术语统一引用 `00-overview/glossary.md`。
- Agent 依据 front-matter 依赖图按需加载切片。

### 可执行
- 行为规格写成 **Given/When/Then** 验收场景。
- 提供可运行校验命令（`go test`、JSON Schema 校验、`curl` 契约示例）。
- 数据模型 / API / 插件协议配 **JSON Schema 或 OpenAPI 片段**作为机器可校验产物。

### 可验收
- 每条需求有稳定 ID（如 `SPEC-OUTLINE-001`）。
- 开发计划里程碑用 `verifies: [SPEC-xxx]` 反向追溯。
- `80-dev/test-strategy.md` + `definition-of-done.md` 给出「完成」客观判据。

### 统一文档骨架
规格类文档统一含：`目标` / `范围与非目标` / `规格正文` / `验收标准(Given-When-Then)` / `校验方式` / `依赖`。

## 需求 ID 命名规范

- 功能需求：`SPEC-<域>-<编号>`，如 `SPEC-OUTLINE-001`、`SPEC-VIEWER-003`、`SPEC-CMD-PAGE-001`。
- 架构约束：`ARCH-<域>-<编号>`。
- 数据约束：`DATA-<域>-<编号>`。
- API 契约：`API-<域>-<编号>`。
- ADR：`ADR-<四位编号>`。

## 验收标准（本设计本身）

- GIVEN 空仓库，WHEN 执行文档生成，THEN `docs/` 下生成约 55 个文件且目录结构与上文一致。
- GIVEN 任一 `.schema.json`，WHEN 用 JSON Schema 校验器加载，THEN 为合法 Draft 2020-12 schema。
- GIVEN `40-api/openapi.yaml`，WHEN 用 OpenAPI 校验器加载，THEN 为合法 OpenAPI 3.1 文档。
- GIVEN `30-data-model/sqlite-schema.sql`，WHEN 在 SQLite 执行，THEN 无语法错误建表成功。
- GIVEN 任一规格文档，THEN 含 front-matter 且包含验收标准小节。

## 校验方式

```bash
# 文件数与结构
find docs -type f | wc -l
# JSON Schema 合法性（示例）
python -c "import json,glob; [json.load(open(f)) for f in glob.glob('docs/**/*.schema.json', recursive=True)]"
# SQLite DDL 可执行性
sqlite3 :memory: < docs/30-data-model/sqlite-schema.sql
```

## 依赖

无（首个文档脚手架，零代码依赖）。
