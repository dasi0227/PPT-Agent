# Agent Runtime Core Capabilities Implementation

> 日期：2026-08-06  
> 对应设计：`2026-08-06-agent-runtime-core-capabilities-design.md`

## 核心模块

- Checkpoint：`internal/workflow/checkpoint.go` 扩展 Runtime checkpoint 白名单结构，保存 requirement ledger、context briefing、latest tool results、context index ref、provider continuation hash。
- Context Retrieval 2.0：`internal/workflow/context_retrieval.go` 提供 ContextIndex、EmbeddingProvider、HybridContextRetriever；`search_refs` 保持工具名不变，内部切换为 hybrid retrieval。
- Semantic Reviewer：`internal/workflow/semantic_reviewer.go` 提供 reviewer interface、LLM reviewer、严格 JSON parser、issue mapping；prompt 文件位于 `prompts/semantic_reviewer/`。
- Resume / Recovery：`internal/workflow/recovery.go` 提供 direct-write reconciliation；`internal/run.Engine.Resume` 和 `service.ResumeRun` 提供恢复入口。

## 数据库

新增 migration：`backend/migrations/0007_runtime_core_capabilities.sql`。

新增表：

- `run_checkpoints`
- `context_index_snapshots`
- `semantic_reviews`

SQLite store 已实现 checkpoint、context index、semantic review 的保存和读取接口。

## Runtime 状态流

Runtime 仍是 strategy-routed single ReAct loop。

新增流程：

1. strategy initialized 后构建 ContextIndex snapshot 并 checkpoint。
2. 每轮 `Agent.Next` 前执行 retrieval，并把 selected refs 注入 ContextBriefing。
3. 工具执行后记录 latest tool result；write/render 成功保存 checkpoint。
4. `finish(message)` 先过 deterministic CompletionGate，再调用 Semantic Reviewer。
5. reviewer reject 转成 completion observation，回到同一 loop。
6. commit 前后和 terminal 边界保存 checkpoint。

## 边界

- Reviewer 不接收工具列表，不调用工具，不修改 Runtime 状态，只返回结构化判断。
- Retriever 只从当前 ContextPack / Manifest 生成索引，并通过 WorkSpec scope 过滤。
- Checkpoint 不保存 provider 私有 reasoning、本地绝对路径或完整消息 dump；只保存摘要、hash 和白名单状态。
- Resume 不绕过 completion gate；恢复后仍进入同一 Runtime loop。

## 测试

新增覆盖：

- retrieval scope / freshness / ordering / budget。
- semantic review JSON parse 和 issue mapping。
- direct-write reconciliation 分类。
- checkpoint / context index / semantic review SQLite round trip。
