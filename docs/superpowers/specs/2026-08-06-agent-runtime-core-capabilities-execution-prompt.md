# Agent Runtime Core Capabilities Execution Prompt

> 用途：交给另一个执行 Agent，按设计文档实现 Semantic Reviewer、Context Retrieval 2.0、Checkpoint / Resume / Recovery。  
> 权威设计文档：`docs/superpowers/specs/2026-08-06-agent-runtime-core-capabilities-design.md`

## 启动 Prompt

```text
你是负责 PPT_Agent 后端 Agent Runtime 升级的执行 Agent。请在当前仓库中完整实现以下设计文档：

docs/superpowers/specs/2026-08-06-agent-runtime-core-capabilities-design.md

你必须先读取该设计文档全文，并以该文档为唯一权威。如果本文 prompt 与设计文档冲突，以设计文档为准。

当前项目背景：
- 项目路径：/Users/bytedance/Desktop/ByteDance/PPT_Agent
- 后端 Go module：backend/
- Runtime 核心包：backend/internal/workflow
- Context 工程包：backend/internal/contextengine
- Run 生命周期包：backend/internal/run
- Run service：backend/internal/service
- SQLite store：backend/internal/store/sqlite
- Prompt registry 已文件化：backend/prompts/runtime/**/*.md
- 现有 Runtime 是 strategy-routed single ReAct loop，不允许改回固定 PEV workflow。

目标：
1. 实现 LLM Semantic Completion Reviewer。
2. 实现 Context Retrieval 2.0。
3. 实现 Checkpoint / Resume / Recovery。

硬性原则：
- 不破坏现有 single ReAct loop。
- 不绕过 ToolRegistry、scope、CompletionGate、finish(message)。
- 不把 plan step 变成独立 workflow node。
- 不让 reviewer 调工具或修改状态；reviewer 只返回结构化判断。
- 不让 retrieval 越过 WorkSpec scope。
- checkpoint 不保存 provider 私有 reasoning、secret、本地绝对路径。
- 对已有用户/其他 Agent 改动保持尊重；不要回滚无关文件。
- 使用 apply_patch 修改文件，不要用 shell 重写源码。

推荐实施顺序：

Phase 1: Checkpoint 基础
- 扩展 RuntimeCheckpoint，包含 requirement ledger、context briefing、latest tool results、context index ref、provider continuation snapshot。
- 新增 CheckpointStore interface。
- 新增 SQLite migration 和 store 方法。
- 在 strategy initialized、plan update、ask_user 前后、write 成功、render 完成、Completion Gate 拒绝、commit 前后、terminal 等边界保存 checkpoint。
- 添加单测，验证 checkpoint 内容、序列化和触发时机。

Phase 2: Context Retrieval 2.0
- 新增 ContextIndex / ContextIndexItem。
- 新增 EmbeddingProvider interface，至少实现 NoopEmbeddingProvider 和 HashEmbeddingProvider。
- 新增 ContextRetriever interface 和 hybrid scorer。
- 改造 search_refs 内部实现，保留工具名和模型可见 API。
- 检索结果必须包含 ref_id、kind、source、revision、hash、score、selection_reason、detail_available。
- ContextBriefing 注入 selected refs 和 selection reason。
- 添加单测覆盖 scope filter、freshness filter、score ordering、budget handling。

Phase 3: LLM Semantic Reviewer
- 新增 backend/prompts/semantic_reviewer/**/*.md 和 go:embed registry。
- 新增 SemanticReviewer interface。
- 新增 reviewer 输入结构和 JSON 输出结构。
- deterministic CompletionGate 通过后，再调用 reviewer。
- reviewer reject 时转成 CompletionIssue / ToolResult observation，回到同一 ReAct loop。
- reviewer unavailable 的默认策略：plan、execute-planned、deck-level execute 阻塞；低风险 execute-direct 可配置。
- 添加单测覆盖 reviewer accepted、rejected、invalid JSON、unavailable policy、issue mapping。

Phase 4: Resume / Recovery
- 新增 ResumeRun service 入口。
- 从最新 checkpoint 重建 runtimeState。
- 实现 direct-write reconciliation：clean、dirty_same_run、external_modified、missing_artifact。
- 支持 same-run resume 和 recovery run。
- 恢复时校验 provider profile / model selection / context pack hash / artifact hash。
- 添加崩溃模拟和恢复测试。

Phase 5: Observability
- 增加 trace events：context.retrieved、semantic.review.started、semantic.review.completed、checkpoint.saved、checkpoint.loaded、recovery.started、recovery.reconciled。
- semantic review 结果落库。
- retrieval selection reason 可查。

关键文件参考：
- backend/internal/workflow/runtime.go
- backend/internal/workflow/completion.go
- backend/internal/workflow/context_briefing.go
- backend/internal/workflow/requirement_ledger.go
- backend/internal/workflow/reference_tool.go
- backend/internal/workflow/evidence.go
- backend/internal/contextengine/assembler.go
- backend/internal/contextengine/types.go
- backend/internal/run/engine.go
- backend/internal/service/run.go
- backend/internal/store/sqlite/run_store.go
- backend/internal/store/sqlite/po.go
- backend/migrations/
- backend/prompts/runtime/

测试要求：
- 每完成一个 phase，至少运行受影响包测试。
- 最终必须运行：
  go test ./internal/workflow ./internal/contextengine ./internal/run ./internal/service ./internal/store/sqlite ./migrations ./prompts/runtime
- 如果运行 go test ./...，已知 internal/llm 可能存在与本任务无关的 httptest cancellation 超时；若遇到该问题，必须在最终报告中明确说明，不要修改无关 llm 测试，除非设计文档要求。

交付要求：
- 完成代码实现、迁移、测试。
- 更新或新增必要设计文档。
- 最终报告必须包含：
  1. 核心实现模块。
  2. 数据库/migration 改动。
  3. Runtime 状态流改动。
  4. Reviewer/Retriever/Checkpoint 的边界说明。
  5. 测试命令和结果。
  6. 未解决风险或后续建议。

不要向用户发起确认请求。遇到合理实现选择时，按设计文档和现有代码风格自主决策。
```

## 执行注意事项

- 本 prompt 不是设计文档替代品，必须先读设计文档。
- 若设计范围过大，可以按 Phase 分批 commit，但每个 batch 必须可编译、可测试。
- 不要把 Semantic Reviewer 做成常驻第二 Agent；它只在 finish 候选阶段评审。
- 不要让 Context Retrieval 修改业务状态；它只选择上下文。
- 不要让 Resume 绕过 evidence freshness；恢复后必要时必须重新 validate/render。
