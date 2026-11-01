---
id: ADR-0009
title: 个人仓库统一资产协议（4 类）
status: accepted
owner: shared
depends_on: [ADR-0005, ADR-0006]
verifies: []
---

# ADR-0009：个人仓库统一资产协议（layout/component/theme/fx）

## 背景

用户明确：没有「自带 vs 收藏」二分，一切统一为**个人仓库的资产**，含 4 类：整页板式、组件样式、配色主题、动态特效，
每类都要统一协议与数据结构，便于移植复用。原 docs 把「自带设计系统(60)」与「用户插件(70)」分成两套，且缺 component 规格。

## 决策

合并为**一套资产协议**：统一信封（id/name/version/kind/source/description/tags/mount）+ 4 个类型化载荷（按 kind）。
- 产品出厂预置（source=preset），用户新增（source=user），同协议同表同校验。
- 取代原 ADR-0006 的 plugin 协议（plugin 是其子集，已废弃）。
- 图表不独立成类：静态 SVG/HTML → component；动态 canvas → fx。

## 选项与权衡

| 选项 | 优点 | 缺点 |
|---|---|---|
| 统一信封 + 类型化载荷（选中） | 一套协议覆盖 4 类、加新类只加载荷 schema、preset/user 一致 | 信封需兼容 4 类差异 |
| 4 套独立协议 | 各自最贴合 | 重复、难维护、CRUD 要写 4 遍 |
| 自带与用户两套（原方案） | 各自简单 | 割裂、违背用户「统一」意图 |

## 后果

- 合并 60+70 → `60-design-system/asset-protocol.md` + `asset-manifest.schema.json`。
- 新增 `components.md`（原缺）、`seed-assets.md`（出厂预置 + 冷启动）。
- 数据层 `plugins` 表 → `assets` 表（kind 四类 + source）。
- API `/plugins` → `/assets`（含 PATCH 改资产）。
- 新增 `/repo` scope 操作资产本身。
- 取代 [ADR-0006](0006-plugin-manifest-protocol.md)（标记为被本 ADR 取代）。

## 状态

accepted（2026-06-30），取代 ADR-0006。
