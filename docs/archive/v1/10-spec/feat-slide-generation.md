---
id: SPEC-GEN
title: slide html 生成
status: approved
owner: agent
depends_on: [SPEC-OUTLINE, DS-TOKENS, DS-LAYOUTS, DS-CHARTS, DS-ANIMATIONS, DS-HTML-OUTPUT]
verifies: []
---

# 功能规格：slide html 生成（大纲 → html）

## 目标

以 `slide-json[]` + 选定主题为输入，套用设计系统（主题 token / 版式 / 图表 / 动效），
生成每页的 slide html（及必要的 css/js），写入文件系统，可被预览。

## 范围与非目标

- **范围**：整套生成、单页生成/重生成、主题套用、版式渲染、图表渲染、动效挂载。
- **非目标**：不在此定义编辑（属 [feat-nl-editing](feat-nl-editing.md)）；不做导出。

## 规格正文

| ID | 需求 | 优先级 |
|---|---|---|
| `SPEC-GEN-001` | 给定大纲与主题，Agent MUST 为每页生成符合 [html-output-spec](../60-design-system/html-output-spec.md) 的 slide html | P0 |
| `SPEC-GEN-002` | 生成 MUST 引用公共样式层（design tokens），不得在单页内联硬编码主题色值 | P0 |
| `SPEC-GEN-003` | 页的 `layout` MUST 映射到版式库中的真实结构；未知 layout MUST 回退到安全默认（bullets） | P0 |
| `SPEC-GEN-004` | 含 `chart_intent` 的页 MUST 渲染对应图表（图表样式见 [charts](../60-design-system/charts.md)） | P1 |
| `SPEC-GEN-005` | 动效 MUST 通过约定属性（如 `data-animate` / `data-fx`）挂载，可被运行时启用 | P1 |
| `SPEC-GEN-006` | 整套生成 MUST 通过 Run 引擎，逐页产出并经 SSE 报告进度（`progress` 含当前页/总页数） | P0 |
| `SPEC-GEN-007` | 生成过程支持 HITL：用户中途注入要求 MUST 在下一页 checkpoint 生效 | P0 |
| `SPEC-GEN-008` | 每页生成完成 MUST 落文件并记录版本；整套完成 MUST 更新 project 状态 | P0 |
| `SPEC-GEN-009` | 单页重生成 MUST 只影响该页文件，不动其它页与公共样式层 | P0 |
| `SPEC-GEN-010` | 产出 MUST 固定 16:9 舞台、零运行时强依赖（CDN webfont/highlight/chart 允许） | P0 |

## 验收标准（Given-When-Then）

- **AC-GEN-001**（`SPEC-GEN-001/010`）
  - GIVEN 一份 8 页大纲 + 主题 `tokyo-night`
  - WHEN 执行整套生成
  - THEN 产出 8 个 slide html，全部通过 html-output-spec 校验清单（含 16:9 与依赖检查）

- **AC-GEN-002**（`SPEC-GEN-002`）
  - GIVEN 已选主题
  - WHEN 生成单页
  - THEN 该页 html 不含硬编码主题十六进制色值，而是引用 `var(--token)`

- **AC-GEN-006**（`SPEC-GEN-006`）
  - GIVEN 整套生成中
  - WHEN 监听 SSE
  - THEN 收到 N 个 `progress` 事件（N=页数），每个含 `current`/`total`

- **AC-GEN-009**（`SPEC-GEN-009`）
  - GIVEN 已生成的 8 页
  - WHEN 重生成第 5 页
  - THEN 仅第 5 页文件 hash 变化，其余 7 页与公共样式层 hash 不变

## 校验方式

```bash
go test ./internal/agent/generate -run TestSlideGeneration
node scripts/lint-slide.mjs <slide.html>   # html-output-spec 校验清单
```

## 依赖

- [SPEC-OUTLINE](feat-outline-generation.md)
- [DS-TOKENS](../60-design-system/design-tokens.md)、[DS-LAYOUTS](../60-design-system/layouts.md)、[DS-CHARTS](../60-design-system/charts.md)、[DS-ANIMATIONS](../60-design-system/animations.md)
- [DS-HTML-OUTPUT](../60-design-system/html-output-spec.md)
