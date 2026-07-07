---
id: AGENT-CMD-INDEX
title: 指令系统总览
status: approved
owner: agent
depends_on: [AGENT-OVERVIEW, ARCH-HARNESS]
verifies: []
---

# 指令系统总览

用户通过指令控制 Agent。指令分**两个正交维度**：**scope（改哪里）** 与 **mode（怎么做）**。
指令在前端输入层初步解析（标注 scope/mode），后端 `agent/command` 包做权威解析，
最终落为 Run 的字段，由 [harness](../../20-architecture/agent-harness.md) 据此**动态裁剪工具集**。

## 两个维度

```
维度A scope（改哪里）: /current(默认) | /page x | /overview | /repo
维度B mode（怎么做）:   正常 | /prompt | /recap | /talk | /ask
```

MVP 约定：**scope 指令单选互斥，mode 指令单选互斥，且本期不做组合**（一次输入只识别一个主指令）。

## scope 指令

| 指令 | 语义 | 作用对象 | 文档 |
|---|---|---|---|
| `/current`（默认） | 当前停留页 | 正在预览的那一页 html | [current](current.md) |
| `/page x` | 指定第 x 页 | 第 x 页 html | [page](page.md) |
| `/overview` | 整个演示文稿层面，跨多页 | 公共层优先，必要时跨页 | [overview](overview.md) |
| `/repo` | 只修改个人仓库资产 | 仓库 layout/component/theme/fx | [repo](repo.md) |

## mode 指令

| 指令 | 语义 | 文档 |
|---|---|---|
| `/prompt` | 改写用户输入，不改任何产物 | [prompt](prompt.md) |
| `/recap` | 进度回顾（只读） | [recap](recap.md) |
| `/talk` | 只说不做，吐分析对齐颗粒度 | [talk](talk.md) |
| `/ask` | 必须就疑点提问对齐颗粒度 | [ask](ask.md) |

## scope → harness 工具门控（核心）

每个 scope 决定 harness **注册哪些写工具**（详见 [agent-harness](../../20-architecture/agent-harness.md) 与 [tools](../../20-architecture/tools.md)）：

| scope | 注册的写工具 | 子代理 | 隔离断言 |
|---|---|---|---|
| `/current` / `/page x` | `patch_slide(目标页)`、`mount_asset(目标页)` | 否 | 只目标页变 |
| `/overview` | `patch_design`、`apply_theme`（优先）；必要时 `patch_slide(*)` | 是（逐页 patch） | 改动面最小化 |
| `/repo` | `create_asset`、`patch_asset`、`delete_asset` | 否 | 只资产变，不碰 PPT 页 |

mode 进一步收窄：`/talk` 不注册任何产物工具；`/prompt` 只产文本；`/recap` 只读。

## 解析约定

| ID | 约束 |
|---|---|
| `AGENT-CMD-001` | 指令 MUST 出现在用户输入**行首**才识别为指令 |
| `AGENT-CMD-002` | `/page` MUST 携带页号；缺参 MUST 报错或在 ask 模式追问 |
| `AGENT-CMD-003` | 指令 MUST 映射为 Run 的 `scope`/`mode`/`page_index`/`command` 字段 |
| `AGENT-CMD-004` | 未知 `/xxx` MUST 提示可用指令，不静默执行破坏性操作 |
| `AGENT-CMD-005` | 指令后可跟自然语言作为 instruction（如 `/page 3 把标题改大`） |
| `AGENT-CMD-006` | 无 scope 指令时，scope MUST 默认为 `/current`（当前预览页） |
| `AGENT-CMD-007` | MVP 不支持指令组合；同时出现多个主指令 MUST 报错要求单选 |

## 指令 → Run 字段映射

| 输入 | scope | mode | page_index | command |
|---|---|---|---|---|
| `把标题改大`（无指令） | current | normal | 当前页 | — |
| `/current 把标题改大` | current | normal | 当前页 | — |
| `/page 3 ...` | page | normal | 3 | — |
| `/overview ...` | overview | normal | — | — |
| `/repo 改我的卡片组件` | repo | normal | — | — |
| `/prompt ...` | current | normal | — | prompt |
| `/recap` | current | normal | — | recap |
| `/talk ...` | current | talk | — | talk |
| `/ask ...` | current | ask | — | ask |

## 验收标准（Given-When-Then）

- **AC-CMD-INDEX-001**（`AGENT-CMD-005/006`）
  - GIVEN 用户在第 3 页输入 `把标题改大`（无指令）
  - WHEN 解析
  - THEN Run 为 `scope=current, page_index=3, mode=normal`

- **AC-CMD-INDEX-007**（`AGENT-CMD-007`）
  - GIVEN 输入 `/page 3 /overview ...`（两个 scope）
  - WHEN 解析
  - THEN 报错要求单选，不执行

## 校验方式

```bash
go test ./internal/agent/command -run 'TestCommandParse|TestDefaultCurrentScope|TestNoCombo'
```

## 依赖

- [AGENT-OVERVIEW](../agent-overview.md)、[ARCH-HARNESS](../../20-architecture/agent-harness.md)
