---
id: SPEC-CMD-TALK
title: 指令 /talk（模式）
status: approved
owner: agent
depends_on: [AGENT-CMD-INDEX, AGENT-MODES]
verifies: []
---

# 指令 `/talk`（只说不做模式）

## 目标

进入一种模式：Agent **只输出分析与思考**，不改任何产物。目标是与用户**对齐颗粒度**——在动手前先把理解、方案、取舍讲清楚。

## 语法

```
/talk <要讨论/分析的话题>
```

## 规格正文

| ID | 需求 | 优先级 |
|---|---|---|
| `SPEC-CMD-TALK-001` | `/talk` 模式 Run MUST NOT 产生任何 `artifact` 或文件变更 | P0 |
| `SPEC-CMD-TALK-002` | MUST 输出分析/思考内容（`info`/`token` 事件） | P0 |
| `SPEC-CMD-TALK-003` | 即使用户在 `/talk` 中描述了可执行操作，Agent MUST NOT 执行，仅给出「将如何做」的分析 | P0 |
| `SPEC-CMD-TALK-004` | `/talk` 期间仍 MAY 接受控制输入以调整讨论方向（但不产物） | P1 |

## 与 /prompt 的区别

| | /prompt | /talk |
|---|---|---|
| 产出 | 改写后的「指令文本」 | 分析/思考/方案讨论 |
| 目的 | 优化用户表达 | 对齐理解与颗粒度 |
| 是否执行 | 否 | 否 |

## 验收标准（Given-When-Then）

- **AC-CMD-TALK-001**（`SPEC-CMD-TALK-001/003`）
  - GIVEN `/talk 我想把整体改成深色并加动效`
  - WHEN 执行
  - THEN 仅输出分析（如深色方案取舍、动效建议、影响范围），文件系统无变更，未执行任何修改

## 校验方式

```bash
go test ./internal/agent/command -run TestTalkNoArtifact
# 断言：事件流无 artifact，work_dir 无变更
```

## 依赖

- [AGENT-CMD-INDEX](README.md)、[AGENT-MODES](../modes.md)
