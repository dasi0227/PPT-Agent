---
id: SPEC-INDEX
title: 规格索引
status: approved
owner: shared
depends_on: [OVERVIEW-SCOPE]
verifies: []
---

# 规格索引

本目录是功能规格的事实源。每个 `feat-*.md` 聚焦一个功能域，含稳定 `SPEC-ID` 与 Given/When/Then 验收标准。

## 文件清单

| 文件 | 功能域 | ID 前缀 |
|---|---|---|
| [functional-spec.md](functional-spec.md) | 总规格（能力全景 + 参考项目能力表） | — |
| [feat-outline-generation.md](feat-outline-generation.md) | 主题 → 大纲 slide-json | `SPEC-OUTLINE-*` |
| [feat-slide-generation.md](feat-slide-generation.md) | 大纲 → slide html | `SPEC-GEN-*` |
| [feat-nl-editing.md](feat-nl-editing.md) | 自然语言引导编辑 | `SPEC-EDIT-*` |
| [feat-online-viewer.md](feat-online-viewer.md) | 在线查看 | `SPEC-VIEWER-*` |
| [feat-personal-repo.md](feat-personal-repo.md) | 个人仓库 | `SPEC-REPO-*` |
| [acceptance-criteria.md](acceptance-criteria.md) | 全量验收场景汇总 | 引用各 SPEC-ID |

指令系统的规格在 [50-agent/commands](../50-agent/commands/)，ID 前缀 `SPEC-CMD-*`。

## SPEC-ID 命名规范

- 格式：`SPEC-<域>-<三位编号>`，如 `SPEC-OUTLINE-001`。
- ID 一经分配**永不复用、永不变更含义**；废弃用 status 标 `deprecated`，不回收编号。
- 每条 `SPEC-ID` 必须可被某条验收场景 + 某个校验命令覆盖。

## 优先级标记

规格条目用 `[P0]`（MVP 必须）/ `[P1]`（MVP 应有）/ `[P2]`（可延后）标注。

## 依赖

- [OVERVIEW-SCOPE](../00-overview/scope.md)
