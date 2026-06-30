---
id: SPEC-CMD-PAGE
title: 指令 /page
status: approved
owner: agent
depends_on: [AGENT-CMD-INDEX, SPEC-EDIT]
verifies: []
---

# 指令 `/page x`

## 目标

跳转到第 x 页并把本次编辑**严格锁定在该页**，防止 Agent 误改其它页或公共样式层。

## 语法

```
/page <index> [自然语言编辑指令]
```
- `<index>`：页号（约定 0 基还是 1 基由前端统一，见下约束）。
- 后续文本作为编辑 instruction（可省略，仅跳转）。

## 规格正文

| ID | 需求 | 优先级 |
|---|---|---|
| `SPEC-CMD-PAGE-001` | `/page x` MUST 将 Run 设为 `scope=page, page_index=x` | P0 |
| `SPEC-CMD-PAGE-002` | 编辑 MUST 只修改第 x 页文件，不动其它页与公共样式层 | P0 |
| `SPEC-CMD-PAGE-003` | x 越界 MUST 报错（`BAD_REQUEST`），不创建破坏性 Run | P0 |
| `SPEC-CMD-PAGE-004` | 页号基数（0/1 基）MUST 在前端展示与后端存储间一致转换（存储 0 基） | P0 |
| `SPEC-CMD-PAGE-005` | 仅 `/page x`（无指令文本）MUST 仅跳转预览，不触发编辑 Run | P1 |

## 验收标准（Given-When-Then）

- **AC-CMD-PAGE-001**（`SPEC-CMD-PAGE-001/002`）
  - GIVEN 8 页 Deck
  - WHEN `/page 3 把标题改大一号`
  - THEN 仅第 3 页 html 变更，公共样式层与其它 7 页 hash 不变，并生成新版本

- **AC-CMD-PAGE-003**（`SPEC-CMD-PAGE-003`）
  - GIVEN 8 页 Deck
  - WHEN `/page 99 ...`
  - THEN 返回 `BAD_REQUEST`，无文件变更

## 校验方式

```bash
go test ./internal/agent/command -run TestPageScopeLock
# hash diff 断言受影响文件集合 == {目标页}
```

## 依赖

- [AGENT-CMD-INDEX](README.md)、[SPEC-EDIT](../../10-spec/feat-nl-editing.md)
