---
id: SPEC-CMD-OVERVIEW
title: 指令 /overview
status: approved
owner: agent
depends_on: [AGENT-CMD-INDEX, SPEC-EDIT, DS-TOKENS, ARCH-HARNESS]
verifies: []
---

# 指令 `/overview`

## 目标

站在**整个演示文稿层面、跨多页**地调整 PPT。优先通过修改**公共样式层（design tokens）**一次性影响全局；
当目标无法仅靠 token 表达时，才**逐页 patch**（走子代理）。一句话：**改动面最小化**。

## 语法

```
/overview <自然语言全局调整指令>
```
例：`/overview 主色改成品牌蓝`（→ 改公共层）、`/overview 每页右下角加页脚 logo`（→ 跨页 patch）

## 规格正文

| ID | 需求 | 优先级 |
|---|---|---|
| `SPEC-CMD-OVERVIEW-001` | `/overview` MUST 设 `scope=overview`，harness 注册 `patch_design`/`apply_theme`/`patch_slide(*)` | P0 |
| `SPEC-CMD-OVERVIEW-002` | 能用公共层 token 表达的调整 MUST 优先改公共层，不逐页改（改动面最小化） | P0 |
| `SPEC-CMD-OVERVIEW-003` | 仅当 token 无法表达（如逐页结构性添加）时，才 `patch_slide(*)`，且 MUST 走**子代理逐页**执行 | P0 |
| `SPEC-CMD-OVERVIEW-004` | 公共层变更 MUST 产生 `design` 版本；跨页 patch 每页各自版本，可回滚 | P0 |
| `SPEC-CMD-OVERVIEW-005` | 换主题 MUST 经 `apply_theme`，写入公共层必需 token 全集，不残缺 | P0 |
| `SPEC-CMD-OVERVIEW-006` | 跨页 patch MUST 每页各自跑 `validate_slide`，任一页失败不影响其它页落盘 | P1 |

> **M5 实现偏差（已记录）**：M5 落 `patch_design`（公共层优先）+ `fanout_page_patch`（跨页逐页子代理，复用 `edit.PatchSlideTool` 锁定单页，每页各自版本、失败隔离）。`apply_theme`（`SPEC-CMD-OVERVIEW-005`）依赖 M6 资产系统且其 AC 不在 M5 verifies，**留 M6**。M5 `/overview` 注册的工具集为 `read_design`/`patch_design`/`fanout_page_patch`/`finish`。实现见 `internal/agent/overview`。

## 三种典型操作的落点

| 用户意图 | harness 行为 | 改动面 |
|---|---|---|
| 「主色改蓝」 | `patch_design` 改 `--color-primary` | 1 个文件（公共层） |
| 「换 ocean 主题」 | `apply_theme(ocean)` 写公共层 token 全集 | 1 个文件（公共层） |
| 「每页加页脚 logo」 | 子代理逐页 `patch_slide` | N 个页文件 |

## 与 /page、/current 的对偶

| | /current·/page | /overview |
|---|---|---|
| 粒度 | 单页 | 跨多页 / 全局 |
| 优先改 | 该页 html | 公共层 token |
| 子代理 | 否 | 跨页 patch 时是 |

共同满足 [SPEC-GLOBAL-006](../../10-spec/functional-spec.md)（样式分层隔离）。

## 验收标准（Given-When-Then）

- **AC-CMD-OVERVIEW-001**（`SPEC-CMD-OVERVIEW-002`）
  - GIVEN 8 页演示文稿
  - WHEN `/overview 主色改成品牌蓝`
  - THEN 仅 `common/tokens.css` 变更，8 页 html 文件 hash 不变，但渲染后全局变蓝

- **AC-CMD-OVERVIEW-003**（`SPEC-CMD-OVERVIEW-003/006`）
  - GIVEN 8 页演示文稿
  - WHEN `/overview 每页右下角加页脚 logo`
  - THEN 子代理逐页 patch，8 页各生成新版本，公共层不变

## 校验方式

```bash
go test ./internal/agent/overview -run 'TestOverviewPatchDesignOnlyCommonLayer|TestOverviewFanoutSubAgentPerPage|TestOverviewFanoutPerPageFailureIsolation'
```

## 依赖

- [AGENT-CMD-INDEX](README.md)、[SPEC-EDIT](../../10-spec/feat-nl-editing.md)、[DS-TOKENS](../../60-design-system/design-tokens.md)、[ARCH-HARNESS](../../20-architecture/agent-harness.md)
