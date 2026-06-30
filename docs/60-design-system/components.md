---
id: DS-COMPONENTS
title: 组件样式（页内可复用小块）
status: approved
owner: shared
depends_on: [ASSET-PROTOCOL, DS-TOKENS, DS-HTML-OUTPUT]
verifies: []
---

# 组件样式（component 资产）

## 定位

`component` = **页内可复用的小块**（粒度 < 一页），区别于 `layout`（整页骨架）。
例如：一张漂亮的卡片、标题徽标、引用块、统计数字块、静态 SVG 图表。组件作为资产存于个人仓库，
经 `mount_asset` 注入页内某处。

## 与 layout 的粒度区分

| | layout | component |
|---|---|---|
| 粒度 | 整页盒子布局 | 页内小块 |
| 用途 | 决定一页的整体结构 | 填充/点缀页内某处 |
| 挂载 | 作为新页骨架 | 注入已有页的某个位置 |
| 例子 | two-column、kpi-grid | neon-card、stat-badge、quote-block |

## 组件载荷

| 文件 | 内容 |
|---|---|
| `template.html` | 组件 html 片段（自包含小块） |
| `style.css`（可选） | 组件私有样式（引用 token） |
| manifest `params` | 可参数化字段（文本、颜色、尺寸等） |

## 组件设计约束

| ID | 约束 |
|---|---|
| `DS-COMP-001` | 组件 html MUST 自包含、可注入任意页而不破坏其布局 |
| `DS-COMP-002` | 组件视觉 MUST 用 `var(--token)`，随主题换肤 |
| `DS-COMP-003` | 组件 MUST 通过 `mount` 约定注入（target + position） |
| `DS-COMP-004` | 组件 params MUST 有默认值，无参可渲染（[ASSET-006](asset-protocol.md)） |
| `DS-COMP-005` | 注入后页 MUST 仍满足 [html-output-spec](html-output-spec.md) |
| `DS-COMP-006` | 静态 SVG/HTML 图表归入 component（动态 canvas 图表归 fx，见 [charts](charts.md)） |

## 预置组件（seed 建议）

| name | 用途 |
|---|---|
| `stat-badge` | 关键数字徽标 |
| `quote-block` | 引用块 |
| `feature-card` | 特性卡片 |
| `kv-list` | 键值列表 |
| `svg-bar` | 静态柱状图（SVG） |

## 验收标准（Given-When-Then）

- **AC-COMP-002**（`DS-COMP-002`）
  - GIVEN 一个 component 注入页 + 切换主题
  - WHEN 重渲染
  - THEN 组件配色随主题 token 变化

- **AC-COMP-005**（`DS-COMP-005`）
  - GIVEN 把 `feature-card` 注入两栏页
  - WHEN lint-slide
  - THEN 页仍合规

## 校验方式

```bash
go test ./internal/asset -run TestComponentMount
node scripts/lint-slide.mjs <slide-with-component.html>
```

## 依赖

- [ASSET-PROTOCOL](asset-protocol.md)、[DS-TOKENS](design-tokens.md)、[DS-HTML-OUTPUT](html-output-spec.md)
