---
id: AGENT-CMD-INDEX
title: 指令系统总览
status: approved
owner: agent
depends_on: [AGENT-OVERVIEW]
verifies: []
---

# 指令系统总览

用户通过以 `/` 开头的指令显式控制 Agent 行为。指令在前端输入层初步解析（标注 scope/mode），
后端 `agent/command` 包做权威解析与分派。

## 指令清单

| 指令 | 语义 | scope | mode | 文档 |
|---|---|---|---|---|
| `/page x` | 跳转到第 x 页并**锁定只改该页** | page | normal | [page](page.md) |
| `/overview` | 站在全局改**公共样式层**（影响所有页） | overview | normal | [overview](overview.md) |
| `/prompt` | 让 AI **改写用户输入**，不改任何产物 | deck | normal | [prompt](prompt.md) |
| `/recap` | 给出当前执行**进度回顾** | deck | normal | [recap](recap.md) |
| `/talk` | 模式：**只说不做**，吐分析对齐颗粒度 | deck | talk | [talk](talk.md) |
| `/ask` | 模式：**必须就疑点提问**对齐颗粒度（无疑点可不问） | deck | ask | [ask](ask.md) |

## 解析约定

| ID | 约束 |
|---|---|
| `AGENT-CMD-001` | 指令 MUST 出现在用户输入**行首**才识别为指令，否则按普通文本 |
| `AGENT-CMD-002` | `/page` MUST 携带页号参数（`/page 3`）；缺参 MUST 报错或在 ask 模式追问 |
| `AGENT-CMD-003` | 指令解析 MUST 映射为 Run 的 `kind`/`scope`/`mode`/`page_index`/`command` 字段 |
| `AGENT-CMD-004` | 未知 `/xxx` 指令 MUST 提示可用指令，不静默当普通文本执行破坏性操作 |
| `AGENT-CMD-005` | 指令可与自然语言混用：`/page 3 把标题改大`，指令后的文本作为 instruction |

## 指令 → Run 字段映射

| 指令 | kind | scope | mode | page_index |
|---|---|---|---|---|
| `/page 3 ...` | edit | page | normal | 3 |
| `/overview ...` | edit | overview | normal | — |
| `/prompt ...` | command | deck | normal | — |
| `/recap` | command | deck | normal | — |
| `/talk ...` | command | deck | talk | — |
| `/ask ...` | command | deck | ask | — |

## 验收标准（Given-When-Then）

- **AC-CMD-INDEX-001**（`AGENT-CMD-005`）
  - GIVEN 输入 `/page 3 把标题改大`
  - WHEN 解析
  - THEN Run 字段为 `kind=edit, scope=page, page_index=3, instruction="把标题改大"`

- **AC-CMD-INDEX-004**（`AGENT-CMD-004`）
  - GIVEN 输入 `/unknown 做点什么`
  - WHEN 解析
  - THEN 返回可用指令提示，不执行任何产物变更

## 校验方式

```bash
go test ./internal/agent/command -run TestCommandParse
```

## 依赖

- [AGENT-OVERVIEW](../agent-overview.md)
