---
id: SPEC-EDIT
title: 自然语言引导编辑
status: approved
owner: agent
depends_on: [SPEC-GEN, AGENT-CONTEXT, SPEC-CMD-PAGE, SPEC-CMD-OVERVIEW]
verifies: []
---

# 功能规格：自然语言引导编辑

## 目标

用户用自然语言（或配合指令）对已生成的 slide html 进行精确编辑，Agent 理解意图、定位目标、
产出修改并保留可回滚版本。

## 范围与非目标

- **范围**：单页编辑、全局编辑、局部元素编辑、编辑的版本化与回滚。
- **非目标**：框选多模态编辑（backlog，预留 `selection` 字段）；导出。

## 规格正文

| ID | 需求 | 优先级 |
|---|---|---|
| `SPEC-EDIT-001` | 给定自然语言编辑请求 + 目标范围（当前页/指定页/全局），Agent MUST 产出对应修改 | P0 |
| `SPEC-EDIT-002` | 编辑 MUST 通过 Run 引擎执行，经 SSE 流式返回 | P0 |
| `SPEC-EDIT-003` | 单页编辑（默认或 `/page x`）MUST 仅修改目标页，不影响其它页与公共样式层 | P0 |
| `SPEC-EDIT-004` | 全局编辑（`/overview`）MUST 仅修改公共样式层，不逐页改 slide html | P0 |
| `SPEC-EDIT-005` | 每次编辑 MUST 生成新版本，支持回滚到任意历史版本 | P0 |
| `SPEC-EDIT-006` | 编辑请求体 SHOULD 支持可选 `selection` 字段（backlog：框选范围），MVP 可忽略其内容 | P2 |
| `SPEC-EDIT-007` | 编辑前 Agent SHOULD 在 `/ask` 或不确定时通过 `needs_input` 澄清，再执行 | P1 |
| `SPEC-EDIT-008` | 编辑 MUST 保持产出仍满足 [html-output-spec](../60-design-system/html-output-spec.md) | P0 |

## 输入 / 输出

- **输入**：`{ project_id, scope: "page"|"overview", page_index?: number, instruction: string, selection?: SelectionRef }`
- **输出**：SSE 事件流 + 落库的新版本引用。

## 验收标准（Given-When-Then）

- **AC-EDIT-003**（`SPEC-EDIT-003`）
  - GIVEN 8 页演示文稿，当前在第 3 页
  - WHEN 执行「把标题改大一号」（page scope）
  - THEN 仅第 3 页 html 变化，公共样式层与其它页不变，且生成新版本

- **AC-EDIT-004**（`SPEC-EDIT-004`）
  - GIVEN 8 页演示文稿
  - WHEN 执行 `/overview 把主色改成品牌蓝`
  - THEN 仅公共样式层（design tokens）变化，8 页 html 文件不变，但渲染后全局变蓝

- **AC-EDIT-005**（`SPEC-EDIT-005`）
  - GIVEN 一次编辑产生 version N
  - WHEN 回滚到 version N-1
  - THEN 目标文件恢复为 N-1 内容，且回滚动作本身记录为新版本

## 校验方式

```bash
go test ./internal/agent/edit -run TestNLEditScope
# hash diff 断言：编辑前后受影响文件集合与预期一致
```

## 依赖

- [SPEC-GEN](feat-slide-generation.md)
- [AGENT-CONTEXT](../50-agent/context-assembly.md)
- [SPEC-CMD-PAGE](../50-agent/commands/page.md)、[SPEC-CMD-OVERVIEW](../50-agent/commands/overview.md)
- [DATA-VERSION](../30-data-model/versioning.md)
