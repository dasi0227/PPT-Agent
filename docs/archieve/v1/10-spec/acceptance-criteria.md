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

> 本表覆盖 [dev-plan](../80-dev/dev-plan.md) 各里程碑 `verifies:` 引用的全部场景，按里程碑分组。`源文档` 指该场景完整 Given/When/Then 的所在文件。

### M0 — 骨架与契约
| 场景 ID | 覆盖 SPEC | 简述 | 校验方式 | 源文档 |
|---|---|---|---|---|
| AC-DATA-001 | DATA-MODEL-001 | sqlite-schema 建表 + 外键生效 | `sqlite3` 执行 | 30-data-model/data-model.md |
| AC-API-001 | API-OVERVIEW-001 | openapi.yaml 合法 3.1 且含全部端点 | OpenAPI 校验器 | 40-api/api-overview.md |

### M1 — Run 外壳 + Harness + LLM + SSE
| 场景 ID | 覆盖 SPEC | 简述 | 校验方式 | 源文档 |
|---|---|---|---|---|
| AC-GLOBAL-001 | SPEC-GLOBAL-001 | 生成走 Run + SSE | 集成测试 | 10-spec/functional-spec.md |
| AC-GLOBAL-002 | SPEC-GLOBAL-002 | Run 中途注入控制输入 | 集成测试 | 10-spec/functional-spec.md |
| AC-RUN-002 | ARCH-RUN-002 | 注入在下一页 checkpoint 生效 | 集成测试 | 20-architecture/agent-runtime.md |
| AC-RUN-005 | ARCH-RUN-005 | SSE 断线带 Last-Event-ID 续传 | 集成测试 | 20-architecture/agent-runtime.md |
| AC-RUN-006 | ARCH-RUN-006 | /talk 无 artifact、无文件变更 | 集成测试 | 20-architecture/agent-runtime.md |
| AC-SSE-001 | API-SSE-001/002 | seq 连续、恰好一个 done | 集成测试 | 40-api/sse-events.md |
| AC-SSE-003 | API-SSE-003 | Last-Event-ID 续订无重复 | 集成测试 | 40-api/sse-events.md |
| AC-LLM-002 | ARCH-LLM-002 | 取消 ctx 流式 goroutine 无泄漏 | `go test -race` | 20-architecture/llm-integration.md |
| AC-LLM-003 | ARCH-LLM-003 | 日志/表无明文 API Key | 检索校验 | 20-architecture/llm-integration.md |
| AC-RUN-API-001 | API-RUN-001 | done 的 Run 注入 input → 409 | 集成测试 | 40-api/run-lifecycle.md |
| AC-RUN-API-004 | API-RUN-003/004 | waiting reply_to → 202 转 running | 集成测试 | 40-api/run-lifecycle.md |
| AC-RUN-API-005 | API-RUN-005 | 取消 Run 保留已落盘产物 | 集成测试 | 40-api/run-lifecycle.md |
| AC-HARNESS-001 | ARCH-HARNESS-001 | 动态工具门控（scope 隔离） | 单测 | 20-architecture/agent-harness.md |
| AC-HARNESS-002 | ARCH-HARNESS-002 | 经工具改产物，非直吐 | 集成测试 | 20-architecture/agent-harness.md |
| AC-HARNESS-004 | ARCH-HARNESS-STOP | MAX_TURNS/finish 退出 | 单测 | 20-architecture/agent-harness.md |
| AC-TOOLS-003 | ARCH-TOOLS-003 | patch 锚点唯一性 | 单测 | 20-architecture/tools.md |
| AC-TOOLS-004 | ARCH-TOOLS-004 | 落盘前 validate | 单测 | 20-architecture/tools.md |

### M2 — 大纲生成
| 场景 ID | 覆盖 SPEC | 简述 | 校验方式 | 源文档 |
|---|---|---|---|---|
| AC-OUTLINE-001 | SPEC-OUTLINE-001/003 | 大纲符合 slide-json schema | schema 校验 | 10-spec/feat-outline-generation.md |
| AC-OUTLINE-002 | SPEC-OUTLINE-002 | 首尾页与页数约束 | 单测 | 10-spec/feat-outline-generation.md |
| AC-OUTLINE-007 | SPEC-OUTLINE-007 | 大纲阶段 HITL 注入 | 集成测试 | 10-spec/feat-outline-generation.md |

### M3 — 资产协议 + 设计系统 + slide 生成
| 场景 ID | 覆盖 SPEC | 简述 | 校验方式 | 源文档 |
|---|---|---|---|---|
| AC-GEN-001 | SPEC-GEN-001/010 | 整套产出合规 16:9 | lint-slide | 10-spec/feat-slide-generation.md |
| AC-GEN-002 | SPEC-GEN-002 | 不硬编码主题色 | lint-slide | 10-spec/feat-slide-generation.md |
| AC-GEN-006 | SPEC-GEN-006 | 逐页 progress 事件 | 集成测试 | 10-spec/feat-slide-generation.md |
| AC-GEN-009 | SPEC-GEN-009 | 单页重生成隔离 | hash diff | 10-spec/feat-slide-generation.md |
| AC-TOKENS-001 | DS-TOKENS-001 | 无硬编码主题色/字号 | lint-tokens | 60-design-system/design-tokens.md |
| AC-TOKENS-002 | DS-TOKENS-002 | 必需 token 清单齐全 | lint-tokens | 60-design-system/design-tokens.md |
| AC-LAYOUTS-001 | DS-LAYOUTS-001 | layout 集合与 schema enum 一致 | 一致性校验 | 60-design-system/layouts.md |
| AC-CHARTS-001 | DS-CHARTS-001 | 图表配色随主题 token 变化 | e2e/渲染 | 60-design-system/charts.md |
| AC-HTML-001 | DS-HTML-001/003/004 | slide html 全部 lint 通过 | lint-slide | 60-design-system/html-output-spec.md |
| AC-ASSET-001 | ASSET-001 | 资产 manifest 校验 | 单测 | 60-design-system/asset-protocol.md |
| AC-ASSET-002 | ASSET-002 | theme token 全集 | 单测 | 60-design-system/asset-protocol.md |
| AC-SEED-001 | DS-SEED-001 | seed 载入仓库 | 集成测试 | 60-design-system/seed-assets.md |

### M4 — 编辑核心（单页 scope）
| 场景 ID | 覆盖 SPEC | 简述 | 校验方式 | 源文档 |
|---|---|---|---|---|
| AC-EDIT-003 | SPEC-EDIT-003 | 单页编辑隔离 | hash diff | 10-spec/feat-nl-editing.md |
| AC-EDIT-005 | SPEC-EDIT-005 | 编辑版本化与回滚 | 单测 | 10-spec/feat-nl-editing.md |
| AC-VERSION-004 | DATA-VERSION-004 | 回滚生成新版本 | 单测 | 30-data-model/versioning.md |
| AC-CMD-CURRENT-001 | SPEC-CMD-CURRENT-001 | /current=当前页，默认 scope | 集成测试 | 50-agent/commands/current.md |
| AC-CMD-PAGE-001 | SPEC-CMD-PAGE-001 | /page 锁定单页 | 集成测试 | 50-agent/commands/page.md |
| AC-CMD-PAGE-003 | SPEC-CMD-PAGE-003 | /page 越界页号 → BAD_REQUEST | 集成测试 | 50-agent/commands/page.md |
| AC-CTX-001 | AGENT-CTX-001 | page scope 不注入其它页 html | 单测 | 50-agent/context-assembly.md |
| AC-HARNESS-001 | ARCH-HARNESS-001 | 动态工具门控（scope 隔离） | 单测 | 20-architecture/agent-harness.md |

### M5 — 跨页/仓库 scope + 模式
| 场景 ID | 覆盖 SPEC | 简述 | 校验方式 | 源文档 |
|---|---|---|---|---|
| AC-EDIT-004 | SPEC-EDIT-004 | 全局编辑只改公共层 | hash diff | 10-spec/feat-nl-editing.md |
| AC-CMD-OVERVIEW-001 | SPEC-CMD-OVERVIEW-002 | /overview 优先改公共层 | 集成测试 | 50-agent/commands/overview.md |
| AC-CMD-OVERVIEW-003 | SPEC-CMD-OVERVIEW-003 | /overview 跨页走子代理 | 集成测试 | 50-agent/commands/overview.md |
| AC-CMD-REPO-001 | SPEC-CMD-REPO-001/004 | /repo 只改资产不碰页 | 集成测试 | 50-agent/commands/repo.md |
| AC-CMD-PROMPT-001 | SPEC-CMD-PROMPT-001 | /prompt 只改写不动文件 | 集成测试 | 50-agent/commands/prompt.md |
| AC-CMD-RECAP-001 | SPEC-CMD-RECAP-001 | /recap 输出进度 | 集成测试 | 50-agent/commands/recap.md |
| AC-CMD-TALK-001 | SPEC-CMD-TALK-001 | /talk 只说不做（无 artifact） | 集成测试 | 50-agent/commands/talk.md |
| AC-CMD-ASK-001 | SPEC-CMD-ASK-001 | /ask 必要时提问 | 集成测试 | 50-agent/commands/ask.md |
| AC-CMD-ASK-002 | SPEC-CMD-ASK-002 | /ask 无歧义时直接执行 | 集成测试 | 50-agent/commands/ask.md |
| AC-MODE-005 | AGENT-MODE-005 | 模式不跨 Run 继承 | 集成测试 | 50-agent/modes.md |

### M6 — 个人仓库（4 类资产）
| 场景 ID | 覆盖 SPEC | 简述 | 校验方式 | 源文档 |
|---|---|---|---|---|
| AC-REPO-001 | SPEC-REPO-001/002 | 4 类资产 + manifest 校验 | 单测 | 10-spec/feat-personal-repo.md |
| AC-REPO-004 | SPEC-REPO-004/005 | seed 冷启动开箱可用 | 集成测试 | 10-spec/feat-personal-repo.md |
| AC-REPO-006 | SPEC-REPO-006 | Agent 检索并移植资产 | 集成测试 | 10-spec/feat-personal-repo.md |
| AC-ASSET-001 | ASSET-001 | 资产 manifest 校验 | 单测 | 60-design-system/asset-protocol.md |
| AC-ASSET-002 | ASSET-002 | theme token 全集 | 单测 | 60-design-system/asset-protocol.md |
| AC-ASSET-003 | ASSET-003 | 移植后仍过 html-output-spec | lint-slide | 60-design-system/asset-protocol.md |
| AC-COMP-002 | DS-COMP-002 | 组件配色随主题 token 变化 | e2e/渲染 | 60-design-system/components.md |
| AC-SEED-004 | DS-SEED-004 | 无资产时回退 preset 不崩溃 | 集成测试 | 60-design-system/seed-assets.md |

### M7 — 在线预览（前端）
| 场景 ID | 覆盖 SPEC | 简述 | 校验方式 | 源文档 |
|---|---|---|---|---|
| AC-VIEWER-001 | SPEC-VIEWER-001 | iframe 一致渲染 | e2e | 10-spec/feat-online-viewer.md |
| AC-VIEWER-005 | SPEC-VIEWER-004/005 | 总览点击跳转 | e2e | 10-spec/feat-online-viewer.md |
| AC-VIEWER-006 | SPEC-VIEWER-006 | 切页无刷新 | e2e（load 计数） | 10-spec/feat-online-viewer.md |
| AC-PREVIEW-002 | ARCH-PREVIEW-002 | 翻页 iframe src 不变、无 load 重触发 | e2e | 20-architecture/preview-mechanism.md |
| AC-PREVIEW-003 | ARCH-PREVIEW-003 | 总览与主视图渲染一致 | e2e | 20-architecture/preview-mechanism.md |

### 其它（非里程碑直接引用，功能级补充）
| 场景 ID | 覆盖 SPEC | 简述 | 校验方式 | 源文档 |
|---|---|---|---|---|
| AC-GLOBAL-006 | SPEC-GLOBAL-006 | 单页/全局样式分层隔离 | hash diff | 10-spec/functional-spec.md |

## 详细场景

各场景的完整 Given/When/Then 写在对应 `feat-*.md` 与 `50-agent/commands/*.md`，本文件只做汇总索引，避免重复维护两份真相。新增场景时：先在功能文档写细节，再在此登记一行。

## 「完成」判据

一个功能视为完成，当且仅当：

1. 其全部 P0 场景的校验命令通过。
2. 满足 [definition-of-done](../80-dev/definition-of-done.md) 的通用 DoD。

## 依赖

- 上游全部功能规格（见 front-matter）。
