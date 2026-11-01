---
id: SPEC-CMD-PROMPT
title: 指令 /prompt
status: approved
owner: agent
depends_on: [AGENT-CMD-INDEX]
verifies: []
---

# 指令 `/prompt`

## 目标

让 AI **改写/优化用户的输入**（使其更清晰、更适合驱动后续生成），但**不改动任何产物**。
用于用户「我想这么说，但帮我说得更好」的场景。

## 语法

```
/prompt <用户的原始想法/草稿>
```

## 规格正文

| ID | 需求 | 优先级 |
|---|---|---|
| `SPEC-CMD-PROMPT-001` | `/prompt` MUST 仅输出改写后的文本（`info` 事件），MUST NOT 产生任何 `artifact` 或文件变更 | P0 |
| `SPEC-CMD-PROMPT-002` | 改写 MUST 保持用户原意，仅优化表达/结构/明确度 | P0 |
| `SPEC-CMD-PROMPT-003` | 输出 SHOULD 可被用户直接复制为下一条指令的输入 | P1 |
| `SPEC-CMD-PROMPT-004` | MUST NOT 自行执行改写后的指令（只给文本，不行动） | P0 |

## 验收标准（Given-When-Then）

- **AC-CMD-PROMPT-001**（`SPEC-CMD-PROMPT-001/004`）
  - GIVEN `/prompt 我想让这页好看点`
  - WHEN 执行
  - THEN 仅返回改写后的清晰指令文本（如「将本页改为两栏布局，主标题加粗、配图右置…」），文件系统无任何变更，且未自动执行

## 校验方式

```bash
go test ./internal/agent/assist -run TestPromptRewriteNoSideEffect
# 断言：执行前后无 artifact，work_dir 无文件变更
```

## 依赖

- [AGENT-CMD-INDEX](README.md)
