---
id: SPEC-CMD-CURRENT
title: 指令 /current（默认 scope）
status: approved
owner: agent
depends_on: [AGENT-CMD-INDEX, SPEC-CMD-PAGE]
verifies: []
---

# 指令 `/current`（当前停留页，默认 scope）

## 目标

把编辑锁定在**用户当前正在预览的那一页**。它是**默认 scope**——用户不打任何 scope 指令时即按 `/current` 处理。

## 语法

```
/current [自然语言编辑指令]
（或省略 /current，直接输入编辑指令）
```

## 与 /page 的关系

`/current` ≡ **自动填入当前预览页号的 `/page`**。二者完全等价，只是 `/current` 省去手打页号。
因此其 scope 隔离、工具门控、版本化语义与 [page](page.md) 完全一致。

## 规格正文

| ID | 需求 | 优先级 |
|---|---|---|
| `SPEC-CMD-CURRENT-001` | 无 scope 指令时 MUST 默认 `/current`，page_index = 前端上报的当前预览页 | P0 |
| `SPEC-CMD-CURRENT-002` | 编辑 MUST 只改当前页，隔离同 `/page`（单页写工具如 `patch_slide`/`mount_asset` 均锁定当前页） | P0 |
| `SPEC-CMD-CURRENT-003` | 前端 MUST 在发起 Run 时上报当前页号；缺失 MUST 报错或追问 | P0 |
| `SPEC-CMD-CURRENT-004` | 当前页号 MUST 与 `/page x` 走同一存储基数（0 基） | P0 |

> **M6 资产移植**：`/current` 可通过 `search_assets` + `mount_asset` 把 layout/component/fx 移植到当前页；`mount_asset` 与 `patch_slide` 一样锁定前端上报的当前页，写入前必须通过 html-output-spec，成功后产该页版本。fx 的脚本路径依赖 M7 预览静态服务暴露 `_assets/`。

## 验收标准（Given-When-Then）

- **AC-CMD-CURRENT-001**（`SPEC-CMD-CURRENT-001/002`）
  - GIVEN 用户停在第 3 页
  - WHEN 输入 `把标题改大`（无指令）
  - THEN 等价于 `/page 3 把标题改大`，仅第 3 页变更，其它页与公共层 hash 不变

## 校验方式

```bash
go test ./internal/agent/command -run 'TestCurrentEqualsPage|TestDefaultScope'
```

## 依赖

- [AGENT-CMD-INDEX](README.md)、[SPEC-CMD-PAGE](page.md)
