---
id: SPEC-ACCEPTANCE
title: 全量验收场景汇总
status: approved
owner: shared
depends_on: [SPEC-FUNCTIONAL, SPEC-OUTLINE, SPEC-GEN, SPEC-EDIT, SPEC-VIEWER, SPEC-REPO]
verifies: []
---

# 全量验收场景汇总

本文件汇总所有可执行验收场景，作为「完成」的客观判据来源。每条场景映射到 SPEC-ID 与校验命令。
开发计划（[dev-plan](../80-dev/dev-plan.md)）的里程碑通过 `verifies:` 引用这些场景 ID。

## 场景索引

| 场景 ID | 覆盖 SPEC | 简述 | 校验方式 |
|---|---|---|---|
| AC-GLOBAL-001 | SPEC-GLOBAL-001 | 生成走 Run + SSE | 集成测试 |
| AC-GLOBAL-002 | SPEC-GLOBAL-002 | Run 中途注入控制输入 | 集成测试 |
| AC-GLOBAL-006 | SPEC-GLOBAL-006 | 单页/全局样式分层隔离 | hash diff |
| AC-OUTLINE-001 | SPEC-OUTLINE-001/003 | 大纲符合 slide-json schema | schema 校验 |
| AC-OUTLINE-002 | SPEC-OUTLINE-002 | 首尾页与页数约束 | 单测 |
| AC-OUTLINE-007 | SPEC-OUTLINE-007 | 大纲阶段 HITL 注入 | 集成测试 |
| AC-GEN-001 | SPEC-GEN-001/010 | 整套产出合规 16:9 | lint-slide |
| AC-GEN-002 | SPEC-GEN-002 | 不硬编码主题色 | lint-slide |
| AC-GEN-006 | SPEC-GEN-006 | 逐页 progress 事件 | 集成测试 |
| AC-GEN-009 | SPEC-GEN-009 | 单页重生成隔离 | hash diff |
| AC-EDIT-003 | SPEC-EDIT-003 | 单页编辑隔离 | hash diff |
| AC-EDIT-004 | SPEC-EDIT-004 | 全局编辑只改公共层 | hash diff |
| AC-EDIT-005 | SPEC-EDIT-005 | 编辑版本化与回滚 | 单测 |
| AC-VIEWER-001 | SPEC-VIEWER-001 | iframe 一致渲染 | e2e |
| AC-VIEWER-005 | SPEC-VIEWER-004/005 | 总览点击跳转 | e2e |
| AC-VIEWER-006 | SPEC-VIEWER-006 | 切页无刷新 | e2e（load 计数） |
| AC-REPO-001 | SPEC-REPO-001/002 | 收藏插件 + manifest 校验 | 单测 |
| AC-REPO-004 | SPEC-REPO-004 | Agent 移植插件 | 集成测试 |
| AC-CMD-PAGE-001 | SPEC-CMD-PAGE-001 | /page 锁定单页 | 集成测试 |
| AC-CMD-OVERVIEW-001 | SPEC-CMD-OVERVIEW-001 | /overview 改公共层 | 集成测试 |
| AC-CMD-PROMPT-001 | SPEC-CMD-PROMPT-001 | /prompt 只改写不动文件 | 集成测试 |
| AC-CMD-RECAP-001 | SPEC-CMD-RECAP-001 | /recap 输出进度 | 集成测试 |
| AC-CMD-TALK-001 | SPEC-CMD-TALK-001 | /talk 只说不做 | 集成测试 |
| AC-CMD-ASK-001 | SPEC-CMD-ASK-001 | /ask 必要时提问 | 集成测试 |

## 详细场景

各场景的完整 Given/When/Then 写在对应 `feat-*.md` 与 `50-agent/commands/*.md`，本文件只做汇总索引，避免重复维护两份真相。新增场景时：先在功能文档写细节，再在此登记一行。

## 「完成」判据

一个功能视为完成，当且仅当：

1. 其全部 P0 场景的校验命令通过。
2. 满足 [definition-of-done](../80-dev/definition-of-done.md) 的通用 DoD。

## 依赖

- 上游全部功能规格（见 front-matter）。
