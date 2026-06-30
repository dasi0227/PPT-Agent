---
id: DS-CHARTS
title: 图表样式
status: approved
owner: shared
depends_on: [DS-TOKENS, DS-LAYOUTS]
verifies: []
---

# 图表样式

定义可用图表类型及其渲染约定。slide-json 的 `chart_intent.type` 从本目录取值
（与 [slide-json schema](../30-data-model/slide-json.schema.json) 一致）。

## 图表清单

| type | 用途 | 备注 |
|---|---|---|
| `bar` | 类别对比 | 柱状/条形 |
| `line` | 趋势 | 折线 |
| `area` | 趋势+量 | 面积 |
| `pie` | 占比 | 饼/环 |
| `radar` | 多维评估 | 雷达 |
| `scatter` | 分布/相关 | 散点 |
| `gantt` | 排期 | 甘特 |
| `flow` | 流程关系 | 流程/有向 |
| `arch` | 架构关系 | 架构/拓扑 |

## 渲染方式

| ID | 约束 |
|---|---|
| `DS-CHARTS-001` | 图表 MUST 使用 token 配色（`--color-primary`/`--color-accent` 等），随主题换肤 |
| `DS-CHARTS-002` | 数据型图表（bar/line/pie/radar/area/scatter）MAY 用轻量库（如 chart.js，CDN）或纯 SVG；优先零依赖 SVG，复杂时允许 CDN 库 |
| `DS-CHARTS-003` | 关系型图（flow/arch/gantt）SHOULD 用 SVG/HTML 结构 + token 样式，避免重运行时 |
| `DS-CHARTS-004` | 图表 MUST 在 16:9 内自适应且具基本可访问性（标题、可读对比度） |
| `DS-CHARTS-005` | chart type 枚举 MUST 与 slide-json schema 的 `chart_intent.type` 一致 |

## 数据缺失处理

- 生成时若无数据：`ask` 模式应提问（见 [ask](../50-agent/commands/ask.md)）；`normal` 模式可用合理示例数据并在 `info` 标注。

## 验收标准（Given-When-Then）

- **AC-CHARTS-001**（`DS-CHARTS-001`）
  - GIVEN 一个 bar 图表页 + 切换主题
  - WHEN 重渲染
  - THEN 图表配色随主题 token 变化

- **AC-CHARTS-005**（`DS-CHARTS-005`）
  - GIVEN 本表 type 集合与 schema enum
  - WHEN 对比
  - THEN 一致

## 校验方式

```bash
node scripts/check-chart-enum.mjs
```

## 依赖

- [DS-TOKENS](design-tokens.md)、[DS-LAYOUTS](layouts.md)
