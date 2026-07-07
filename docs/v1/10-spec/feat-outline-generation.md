---
id: SPEC-OUTLINE
title: 大纲生成
status: approved
owner: agent
depends_on: [DATA-SLIDE, API-RUN, AGENT-PROMPTS]
verifies: []
---

# 功能规格：大纲生成（主题 → slide-json）

## 目标

用户输入一个主题（可附带简报/要点），Agent 产出结构化的 PPT 大纲，即一个 `slide-json[]` 数组，
描述每页的标题、要点、建议版式、图表意图。大纲是后续整套生成的输入。

## 范围与非目标

- **范围**：主题解析、页数建议、每页 slide-json 草稿、风格候选推荐（Show-don't-tell）。
- **非目标**：不在本阶段生成最终 html（属 [feat-slide-generation](feat-slide-generation.md)）；不做 PPTX 解析。

## 规格正文

| ID | 需求 | 优先级 |
|---|---|---|
| `SPEC-OUTLINE-001` | 给定主题文本，Agent MUST 产出符合 [slide-json schema](../30-data-model/slide-json.schema.json) 的 `slide-json[]` | P0 |
| `SPEC-OUTLINE-002` | 大纲 MUST 至少含封面页（cover）与结尾页（thanks/cta），中间页数据 SHOULD 在 5–20 之间，可由用户指定 | P0 |
| `SPEC-OUTLINE-003` | 每页 slide-json MUST 含 `title`、`layout`（来自版式库枚举）、`bullets` 或 `content_intent` | P0 |
| `SPEC-OUTLINE-004` | 含数据型内容的页 SHOULD 标注 `chart_intent`（图表意图，来自图表枚举） | P1 |
| `SPEC-OUTLINE-005` | Agent SHOULD 推荐 1–3 个风格候选（主题 token 组合），供用户选择（Show-don't-tell） | P1 |
| `SPEC-OUTLINE-006` | 大纲生成 MUST 通过 Run 引擎执行并经 SSE 流式返回进度 | P0 |
| `SPEC-OUTLINE-007` | 用户可在大纲生成的 checkpoint 注入控制输入（如「加一页竞品分析」），Agent MUST 纳入 | P0 |
| `SPEC-OUTLINE-008` | 大纲落库后 MUST 创建初始版本（version 0），支持后续回滚 | P0 |

## 输入 / 输出

- **输入**：`{ topic: string, brief?: string, slide_count?: number, language?: "zh"|"en" }`
- **输出**：`{ project_id, slides: SlideJSON[], style_candidates?: ThemeRef[] }`

## 验收标准（Given-When-Then）

- **AC-OUTLINE-001**（`SPEC-OUTLINE-001/003`）
  - GIVEN 主题 "云原生可观测性实践"
  - WHEN 调用大纲生成
  - THEN 返回的每个 slide-json 通过 slide-json schema 校验，且含 `title`/`layout`/(`bullets`|`content_intent`)

- **AC-OUTLINE-002**（`SPEC-OUTLINE-002`）
  - GIVEN 未指定页数
  - WHEN 生成大纲
  - THEN 首页 `layout=cover`，末页 `layout∈{thanks,cta}`，中间页数 5–20

- **AC-OUTLINE-007**（`SPEC-OUTLINE-007`）
  - GIVEN 大纲生成 Run 进行中
  - WHEN 注入「在第 2 页后加一页竞品分析」
  - THEN 最终大纲在第 2 页后出现一页与竞品分析相关的 slide-json

## 校验方式

```bash
go test ./internal/agent/outline -run TestOutlineGeneration
# schema 校验：对返回 slides 逐条 validate against slide-json.schema.json
```

## 依赖

- [DATA-SLIDE](../30-data-model/data-model.md)
- [API-RUN](../40-api/run-lifecycle.md)
- [AGENT-PROMPTS](../50-agent/prompt-templates.md)
- [DS-LAYOUTS](../60-design-system/layouts.md)
