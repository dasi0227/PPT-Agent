---
id: SPEC-CMD-OVERVIEW
title: 指令 /overview
status: approved
owner: agent
depends_on: [AGENT-CMD-INDEX, SPEC-EDIT, DS-TOKENS]
verifies: []
---

# 指令 `/overview`

## 目标

站在**全局**视角调整整份 PPT 的公共前端样式——修改的是**公共样式层（design tokens）**，而非逐页改 html。
一次修改，全局换肤/统一风格。

## 语法

```
/overview <自然语言全局调整指令>
```
例：`/overview 主色改成品牌蓝，字体换成更现代的无衬线`

## 规格正文

| ID | 需求 | 优先级 |
|---|---|---|
| `SPEC-CMD-OVERVIEW-001` | `/overview` MUST 将 Run 设为 `scope=overview`，且只修改公共样式层文件（`common/tokens.css`） | P0 |
| `SPEC-CMD-OVERVIEW-002` | MUST NOT 修改任何单页 `slides/*/index.html` | P0 |
| `SPEC-CMD-OVERVIEW-003` | 修改后所有页渲染 MUST 反映新样式（因引用公共层） | P0 |
| `SPEC-CMD-OVERVIEW-004` | 公共层变更 MUST 产生 `common_style` 版本，可回滚 | P0 |
| `SPEC-CMD-OVERVIEW-005` | 调整 MUST 保持 token 语义完整（不删必需 token，避免页面失样） | P1 |

## 与 /page 的对偶

| | /page | /overview |
|---|---|---|
| 改动对象 | 单页 html | 公共样式层 |
| 影响范围 | 仅该页 | 所有页 |
| 版本类型 | `slide` | `common_style` |

二者共同满足 [SPEC-GLOBAL-006](../../10-spec/functional-spec.md)（样式分层隔离）。

## 验收标准（Given-When-Then）

- **AC-CMD-OVERVIEW-001**（`SPEC-CMD-OVERVIEW-001/002/003`）
  - GIVEN 8 页 Deck
  - WHEN `/overview 主色改成品牌蓝`
  - THEN 仅 `common/tokens.css` 变更，8 页 html 文件 hash 不变，但渲染后全局主色变蓝

- **AC-CMD-OVERVIEW-004**（`SPEC-CMD-OVERVIEW-004`）
  - GIVEN 公共层处于 v0
  - WHEN 执行 `/overview` 改色
  - THEN 生成 `common_style` v1，可回滚至 v0

## 校验方式

```bash
go test ./internal/agent/command -run TestOverviewCommonLayerOnly
# hash diff：受影响文件集合 == {common/tokens.css}
```

## 依赖

- [AGENT-CMD-INDEX](README.md)、[SPEC-EDIT](../../10-spec/feat-nl-editing.md)、[DS-TOKENS](../../60-design-system/design-tokens.md)
