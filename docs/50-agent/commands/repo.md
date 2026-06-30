---
id: SPEC-CMD-REPO
title: 指令 /repo
status: approved
owner: agent
depends_on: [AGENT-CMD-INDEX, ASSET-PROTOCOL, ARCH-TOOLS]
verifies: []
---

# 指令 `/repo`

## 目标

把操作锁定在**个人仓库的资产本身**——增、删、改仓库里的 layout / component / theme / fx 资产，
而**不触碰任何 PPT 页**。这是与 `/current`、`/page`、`/overview` 平行的第四种 scope。

## 语法

```
/repo <对仓库资产的自然语言操作>
```
例：`/repo 把我的"霓虹卡片"组件圆角调大`、`/repo 新增一个深色主题`、`/repo 删除没用的星空特效`

## 规格正文

| ID | 需求 | 优先级 |
|---|---|---|
| `SPEC-CMD-REPO-001` | `/repo` MUST 将 Run 设为 `scope=repo`，harness 只注册资产工具（`search/read/create/patch/delete/validate_asset`），**不注册任何 PPT 页/公共层写工具** | P0 |
| `SPEC-CMD-REPO-002` | 支持对四类资产（layout/component/theme/fx）的**增、删、改** | P0 |
| `SPEC-CMD-REPO-003` | 资产变更 MUST 通过资产协议 schema 校验（[asset-protocol](../../60-design-system/asset-protocol.md)）后才落库 | P0 |
| `SPEC-CMD-REPO-004` | `/repo` 操作 MUST NOT 改变任何 PPT 项目的页或公共样式层 | P0 |
| `SPEC-CMD-REPO-005` | 资产变更 SHOULD 版本化（资产也有版本，便于回滚） | P1 |
| `SPEC-CMD-REPO-006` | 新增资产的来源不限定（可手写、可从某页提炼，但不强制从页提炼） | P1 |

## 与 mount_asset 的区别

| | `/repo` | 在 PPT 里用资产 |
|---|---|---|
| 改什么 | 仓库资产本身 | 把资产 `mount_asset` 进某页 |
| scope | repo | current/page/overview |
| 影响 | 资产库 | PPT 页 |

## 验收标准（Given-When-Then）

- **AC-CMD-REPO-001**（`SPEC-CMD-REPO-001/004`）
  - GIVEN 一个已有项目 + 仓库中有组件 `neon-card`
  - WHEN `/repo 把 neon-card 圆角调大`
  - THEN 仅该资产载荷变更，任何 PPT 项目的页与公共层 hash 不变

- **AC-CMD-REPO-002**（`SPEC-CMD-REPO-002/003`）
  - GIVEN `/repo 新增一个名为 ocean 的深色主题`
  - WHEN 执行
  - THEN 新增一个 `kind=theme` 资产，通过 token 全集校验，出现在仓库

## 校验方式

```bash
go test ./internal/agent/command -run TestRepoScopeAssetOnly
go test ./internal/asset -run 'TestAssetCRUD|TestAssetValidate'
```

## 依赖

- [AGENT-CMD-INDEX](README.md)、[ASSET-PROTOCOL](../../60-design-system/asset-protocol.md)、[ARCH-TOOLS](../../20-architecture/tools.md)
