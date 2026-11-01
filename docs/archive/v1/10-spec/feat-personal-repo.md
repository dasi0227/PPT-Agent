---
id: SPEC-REPO
title: 个人仓库（统一资产）
status: approved
owner: shared
depends_on: [ASSET-PROTOCOL, ARCH-TOOLS, SPEC-CMD-REPO]
verifies: []
---

# 功能规格：个人仓库（4 类统一资产）

## 目标

个人仓库是**唯一中心概念**，持有 4 类资产（layout / component / theme / fx），遵循统一协议。
产品出厂预置（preset）一批，用户可增删改（user）。资产可被 AI 检索并移植/应用进 PPT。

## 范围与非目标

- **范围**：资产的增删改查、schema 校验、AI 检索与移植/应用、出厂预置 seed。
- **非目标**：资产市场/分享/远程仓库（不做）；远程加载（MVP 仅本地信任）。

## 规格正文

| ID | 需求 | 优先级 |
|---|---|---|
| `SPEC-REPO-001` | 仓库 MUST 支持 4 类资产，统一信封 + 类型化载荷（[asset-protocol](../60-design-system/asset-protocol.md)） | P0 |
| `SPEC-REPO-002` | 资产 manifest MUST 通过 [asset-manifest schema](../60-design-system/asset-manifest.schema.json) 校验 | P0 |
| `SPEC-REPO-003` | MUST 支持资产的增、删、改、查、检索（含按 kind 过滤） | P0 |
| `SPEC-REPO-004` | 出厂 MUST 预置 seed 资产，冷启动开箱可用（[seed-assets](../60-design-system/seed-assets.md)） | P0 |
| `SPEC-REPO-005` | preset 与 user 资产 MUST 同协议、同表、同校验，仅 `source` 不同 | P0 |
| `SPEC-REPO-006` | Agent MUST 能检索仓库并按 manifest 把 layout/component/fx 移植进页、把 theme 应用到公共层 | P1 |
| `SPEC-REPO-007` | 移植/应用后 MUST 不破坏目标 slide 的 html-output-spec 合规性 | P0 |
| `SPEC-REPO-008` | 资产变更 SHOULD 版本化，可回滚（含回滚 preset 到出厂态） | P1 |

## 验收标准（Given-When-Then）

- **AC-REPO-001**（`SPEC-REPO-001/002`）
  - GIVEN 四类资产各一个 manifest
  - WHEN 用 asset-manifest schema 校验
  - THEN 全部通过

- **AC-REPO-004**（`SPEC-REPO-004/005`）
  - GIVEN 全新环境首次启动
  - WHEN `GET /assets`
  - THEN 返回 seed 资产（source=preset），覆盖 4 类

- **AC-REPO-006**（`SPEC-REPO-006`）
  - GIVEN 仓库中有 fx 资产 `particle-burst`
  - WHEN 用户「给封面加上粒子特效」
  - THEN Agent `search_assets` 命中并 `mount_asset` 注入封面，渲染生效

## 校验方式

```bash
go test ./internal/asset -run 'TestAssetCRUD|TestAssetValidate|TestSeedLoad|TestMount'
```

## 依赖

- [ASSET-PROTOCOL](../60-design-system/asset-protocol.md)、[ARCH-TOOLS](../20-architecture/tools.md)、[SPEC-CMD-REPO](../50-agent/commands/repo.md)
