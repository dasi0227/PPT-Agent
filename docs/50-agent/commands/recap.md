---
id: SPEC-CMD-RECAP
title: 指令 /recap
status: approved
owner: agent
depends_on: [AGENT-CMD-INDEX, DATA-VERSION, AGENT-CONTEXT]
verifies: []
---

# 指令 `/recap`

## 目标

给出当前 PPT 的**执行进度回顾**，帮助用户回忆「做到哪了、改过什么、接下来可做什么」，以便继续开发。

## 语法

```
/recap
```

## 规格正文

| ID | 需求 | 优先级 |
|---|---|---|
| `SPEC-CMD-RECAP-001` | `/recap` MUST 汇总当前状态：项目主题、页数、各页标题/版式、当前主题、最近若干次变更（来自版本与 run 记录） | P0 |
| `SPEC-CMD-RECAP-002` | MUST 为只读操作，不产生任何产物变更 | P0 |
| `SPEC-CMD-RECAP-003` | SHOULD 给出「可继续的下一步建议」 | P1 |
| `SPEC-CMD-RECAP-004` | 输出 MUST 结构化（列表/表格），便于人快速扫读（参考用户偏好：避免散文式总结） | P1 |

## 数据来源

- `state.json`（当前状态游标）。
- SQLite：`slides`（页标题/版式）、`versions`（最近变更）、`runs`（最近执行）。

## 输出形态（示例结构）

```
项目：云原生可观测性实践 | 主题：tokyo-night | 8 页
进度：编辑中（current_state=editing）
页面：
  0 cover    封面
  1 toc      目录
  ...
最近变更：
  - 第3页 v2（深色背景）  run r12
  - 公共层 v1（主色改蓝）  run r10
建议下一步：补充第5页图表数据 / 统一字号
```

## 验收标准（Given-When-Then）

- **AC-CMD-RECAP-001**（`SPEC-CMD-RECAP-001/002`）
  - GIVEN 已编辑数次的项目
  - WHEN `/recap`
  - THEN 输出含主题/页数/各页/最近变更的结构化摘要，且无文件变更

## 校验方式

```bash
go test ./internal/agent/command -run TestRecapReadOnly
```

## 依赖

- [AGENT-CMD-INDEX](README.md)、[DATA-VERSION](../../30-data-model/versioning.md)、[AGENT-CONTEXT](../context-assembly.md)
