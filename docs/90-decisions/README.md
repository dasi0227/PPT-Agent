---
id: ADR-INDEX
title: 架构决策记录索引
status: approved
owner: shared
depends_on: []
verifies: []
---

# 架构决策记录（ADR）索引

记录关键技术决策的背景、选项、结论与后果。ADR 一经接受不删除；变更用新 ADR 取代并标注。

## 索引

| ADR | 决策 | 状态 |
|---|---|---|
| [ADR-0001](0001-backend-go-chi.md) | 后端 Go + net/http + chi | accepted |
| [ADR-0002](0002-persistence-sqlite-fs.md) | SQLite + 文件系统混合持久化 | accepted |
| [ADR-0003](0003-frontend-react-vite.md) | 前端 React + Vite + TS + Tailwind + shadcn/ui | accepted |
| [ADR-0004](0004-transport-sse-hitl.md) | SSE + 控制 POST 实现流式与 HITL | accepted |
| [ADR-0005](0005-theme-tokens-css-layer.md) | design tokens + 公共 CSS 层 | accepted |
| [ADR-0006](0006-plugin-manifest-protocol.md) | 统一 manifest 插件协议 | superseded by ADR-0009 |
| [ADR-0007](0007-harness-react-tools.md) | Harness 采用 ReAct + 工具化 | accepted |
| [ADR-0008](0008-llm-interface-abstraction.md) | LLM interface 抽象，假设 DeepSeek 良好 | accepted |
| [ADR-0009](0009-unified-asset-protocol.md) | 个人仓库统一资产协议（4 类） | accepted |
| [ADR-0010](0010-project-thread-run-isolation.md) | 三层隔离模型（Project/Thread/Run） | accepted |

## ADR 模板

每个 ADR 含：`背景` / `决策` / `选项与权衡` / `后果` / `状态`。

## 依赖

无。
