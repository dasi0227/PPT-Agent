# Agent Runtime Core Capabilities Design

> 日期：2026-08-06  
> 范围：LLM Semantic Completion Reviewer、Context Retrieval 2.0、Checkpoint / Resume / Recovery  
> 状态：可交付执行  
> 前置背景：已完成 Runtime 认知升级第一阶段，包括 prompt registry、context briefing、requirement ledger、finish strict、semantic gate v1。

## 1. 背景

当前 PPT Agent Runtime 已经具备基础生产骨架：

- Strategy-routed single ReAct loop。
- 动态工具披露与 scope enforcement。
- 显式 `finish(message)` 成功出口。
- Completion Gate 负责 evidence freshness、plan 完成度与 session 基线检查。
- ContextPack / ContextManifest 负责输入上下文组装。
- Prompt Registry 已文件化为 `backend/prompts/runtime/**/*.md`。
- Requirement Ledger 和 Context Briefing 已作为第一阶段认知支架接入。

但系统仍然缺少三个强 Agent 产品的核心能力：

1. **LLM Semantic Completion Reviewer**：判断是否真正满足用户需求，而不是只判断工具和证据状态。
2. **Context Retrieval 2.0**：让 Agent 每轮看到最相关上下文，而不是依赖固定注入和关键词搜索。
3. **Checkpoint / Resume / Recovery**：让长任务可以中断恢复，避免失败后只能重跑或留下不可解释半成品。

这三个模块共同构成下一阶段的“智能闭环”：

```text
看对上下文 -> 做对任务 -> 断了可恢复
Context Retrieval -> Semantic Reviewer -> Checkpoint / Recovery
```

## 2. 目标

### 2.1 Semantic Completion Reviewer

在 `finish` 被 Runtime 接受前，使用 LLM reviewer 检查：

- 用户需求是否被覆盖。
- 最终交付是否完整、自洽、没有遗漏关键约束。
- PPT 内容/设计是否满足 rubric。
- 工具证据与最终回答是否一致。
- 是否存在“工具跑过但任务没做对”的情况。

### 2.2 Context Retrieval 2.0

把当前 `search_refs` 从关键词检索升级为受 scope、revision、freshness 约束的 hybrid retrieval：

- keyword retrieval。
- semantic retrieval。
- metadata filter。
- scope filter。
- freshness/revision filter。
- top-k context briefing injection。
- 按需展开 `summary -> structure -> full`。

### 2.3 Checkpoint / Resume / Recovery

实现可恢复 Runtime：

- 周期性 checkpoint。
- 关键状态 checkpoint。
- Resume 读取 checkpoint 重建 runtimeState。
- Recovery run 处理崩溃、provider failure、tool failure、commit failure。
- 对 direct-write 风险进行 reconciliation。

## 3. 非目标

- 不恢复固定 PEV workflow。
- 不把 Semantic Reviewer 做成总是阻塞的第二 Agent。
- 不让 Retriever 越过 WorkSpec scope。
- 不让 checkpoint 记录 provider 私有 reasoning 或敏感信息。
- 不在第一阶段实现复杂分布式任务调度。
- 不改变现有前端公共 SSE 协议，除非新增可选 debug/trace 字段。

## 4. 总体架构

```text
CreateRun
  -> ContextAssembler
  -> ContextIndex snapshot
  -> Runtime.Run
       -> load/resume checkpoint?
       -> route strategy
       -> build requirement ledger
       -> loop
            -> checkpoint boundary
            -> retrieve relevant context
            -> build context briefing
            -> Agent.Next
            -> execute tools / controls
            -> update evidence + ledger + context index
            -> checkpoint boundary
            -> finish candidate
                 -> deterministic CompletionGate
                 -> LLM Semantic Reviewer
                 -> accept -> commit
                 -> reject -> observation -> same loop
```

三个模块的边界：

| 模块 | 职责 | 不负责 |
|---|---|---|
| Context Retrieval | 找到当前最相关、可授权、最新的上下文 | 判断任务是否完成 |
| Semantic Reviewer | 判断需求和质量是否满足 | 修改文件、绕过工具证据 |
| Checkpoint/Recovery | 保存/恢复 Runtime 状态 | 替代 Completion Gate |

## 5. Semantic Completion Reviewer

### 5.1 核心原则

LLM reviewer 是 Completion Gate 的语义层，不是独立执行者。

它只能返回：

```go
type SemanticReviewResult struct {
    Accepted bool
    Issues []SemanticReviewIssue
    Coverage []RequirementCoverage
    Confidence float64
    Summary string
}
```

它不能：

- 调用写工具。
- 修改 plan。
- 修改 requirement ledger。
- 直接提交 commit。
- 绕过 deterministic gate。

Runtime 根据 reviewer 结果决定：

- accepted：继续 commit。
- rejected：把 reviewer issues 转成 tool observation，回到同一 ReAct loop。

### 5.2 启用条件

第一版不需要每个 run 都调用 reviewer。建议启用条件：

- `StrategyExecute` 且 finish 已通过 deterministic gate。
- `StrategyPlan` 且用户请求明确要求完整计划/报告。
- `execute_planned` 总是启用。
- deck-level 或 multi-target 任务总是启用。
- direct single-slide 小改动可按配置启用。

配置建议：

```go
type SemanticReviewPolicy struct {
    Enabled bool
    ReviewPlan bool
    ReviewTalk bool
    ReviewExecuteDirect bool
    ReviewExecutePlanned bool
    ReviewDeckLevel bool
    MinRisk RiskLevel
}
```

默认：

```text
plan: enabled
talk/ask: disabled
execute-direct: disabled unless high risk or gate repaired before
execute-planned: enabled
deck-level execute: enabled
```

### 5.3 Reviewer 输入

Reviewer 不直接吃全量 message history。输入必须是结构化包：

```go
type SemanticReviewInput struct {
    RunID string
    Strategy ExecutionStrategy
    ExecuteMode ExecuteMode
    WorkSpec model.WorkSpec
    RequirementLedger RequirementLedger
    Plan *Plan
    Changes ChangeSet
    Evidence []Evidence
    LatestIssues []Issue
    ContextBriefing string
    RetrievedContext []ReviewContextItem
    FinishMessage string
    Rubric string
}
```

`RetrievedContext` 来自 Context Retrieval 2.0，必须带 source metadata：

```go
type ReviewContextItem struct {
    RefID string
    Kind string
    Source string
    Target Resource
    Revision int
    Hash string
    DetailLevel string
    Summary string
    Reason string
}
```

### 5.4 Reviewer Prompt

新增文件：

```text
backend/prompts/semantic_reviewer/
  core/semantic_reviewer_policy.md
  rubrics/ppt_completion_rubric.md
  schemas/review_output_contract.md
```

Prompt 核心要求：

- 只评审，不执行。
- 基于输入证据判断，不臆测未提供上下文。
- 优先找遗漏和不一致。
- 用严格 JSON 输出。
- 只有关键需求满足、最终回答完整、证据一致时才 accepted。

### 5.5 Reviewer 输出 Schema

建议 JSON schema：

```json
{
  "accepted": true,
  "confidence": 0.0,
  "summary": "",
  "coverage": [
    {
      "requirement_id": "req_01",
      "status": "satisfied | partial | missing | unverifiable",
      "evidence_refs": ["..."],
      "reason": ""
    }
  ],
  "issues": [
    {
      "code": "REQUIREMENT_UNADDRESSED | QUALITY_RUBRIC_FAILED | FINAL_ANSWER_INCOMPLETE | USER_INTENT_MISMATCH | EVIDENCE_CONTRADICTION | CONTEXT_INSUFFICIENT",
      "severity": "warning | error | fatal",
      "requirement_id": "req_01",
      "target": {"type": "slide", "slide_id": "slide-01", "part": "html"},
      "summary": "",
      "required_action": {
        "tool": "read_ppt | write_ppt | edit_ppt | render_slide | search_refs | ask_user | finish",
        "target": {"type": "slide", "slide_id": "slide-01", "part": "html"}
      }
    }
  ]
}
```

### 5.6 Reviewer 和 Completion Gate 的关系

顺序：

```text
finish candidate
  -> finish strict guard
  -> deterministic CompletionGate
  -> SemanticCompletionReviewer
  -> commit
```

原因：

- deterministic gate 先过滤明显不完整状态，避免浪费 reviewer token。
- reviewer 只处理 deterministic gate 看不懂的语义质量。

Reviewer issue 映射到 CompletionIssue：

| Reviewer code | CompletionIssue |
|---|---|
| `REQUIREMENT_UNADDRESSED` | `REQUIREMENT_UNADDRESSED` |
| `QUALITY_RUBRIC_FAILED` | `QUALITY_RUBRIC_FAILED` |
| `FINAL_ANSWER_INCOMPLETE` | `FINAL_ANSWER_INCOMPLETE` |
| `USER_INTENT_MISMATCH` | `USER_INTENT_MISMATCH` |
| `EVIDENCE_CONTRADICTION` | `EVIDENCE_CONTRADICTION` |
| `CONTEXT_INSUFFICIENT` | `CONTEXT_INSUFFICIENT` |

### 5.7 失败策略

Reviewer provider 调用失败时：

- execute-planned / deck-level：阻塞 finish，返回 `SEMANTIC_REVIEW_UNAVAILABLE`，可重试。
- execute-direct low-risk：可配置为 warning-only。
- plan：阻塞，避免交付不完整计划。

建议默认 conservative：

```text
reviewer unavailable -> reject finish unless policy explicitly allows bypass
```

## 6. Context Retrieval 2.0

### 6.1 核心原则

Context Retrieval 的目标不是“给模型更多内容”，而是“每轮给模型最相关且可信的内容”。

约束：

- 所有 retrieval 必须受 WorkSpec scope 约束。
- 所有结果必须带 revision/hash。
- 过期内容不能作为事实，只能作为历史参考。
- 检索结果必须记录 selection reason。
- 大内容通过 ContextRef 分级展开。

### 6.2 Context Index

新增 Context Index：

```go
type ContextIndex struct {
    RunID string
    ThreadID string
    ProjectID string
    BuiltAt int64
    Items []ContextIndexItem
}
```

```go
type ContextIndexItem struct {
    RefID string
    Kind string
    Source string
    Target Resource
    Revision int
    Hash string
    Summary string
    Keywords []string
    Embedding []float32
    TokenCost map[DetailLevel]int
    AvailableLevels []DetailLevel
    Scope ScopeDescriptor
    Freshness string
    UpdatedAt int64
}
```

首批 index item：

- outline。
- design。
- slide specs。
- slide HTML summaries。
- target full HTML ref。
- thread memory。
- recent turns。
- assets。
- render diagnostics。
- latest tool failures。
- accepted materialization proofs。

### 6.3 Embedding Provider

新增接口：

```go
type EmbeddingProvider interface {
    Embed(ctx context.Context, input []string) ([][]float32, error)
}
```

第一版允许三种实现：

- `NoopEmbeddingProvider`：只做 keyword retrieval，便于本地测试。
- `HashEmbeddingProvider`：确定性伪 embedding，用于单测。
- `LLMEmbeddingProvider`：生产 embedding 服务。

### 6.4 Hybrid Retrieval

新增检索接口：

```go
type ContextRetriever interface {
    Retrieve(ctx context.Context, query RetrievalQuery) (RetrievalResult, error)
}
```

```go
type RetrievalQuery struct {
    RunID string
    WorkSpec model.WorkSpec
    RequirementLedger RequirementLedger
    LatestIssues []Issue
    Phase RuntimePhase
    Strategy ExecutionStrategy
    QueryText string
    Kinds []string
    Limit int
    DetailBudget int
}
```

Scoring：

```text
score =
  semantic_similarity * 0.45
  + keyword_score * 0.25
  + target_scope_boost * 0.15
  + freshness_boost * 0.10
  + issue_relevance_boost * 0.05
```

必须先过滤：

- project mismatch。
- run unauthorized。
- scope forbidden。
- stale revision when current fact required。
- token budget overflow。

### 6.5 search_refs 升级

当前 `search_refs` 保留模型可见 API，不改工具名。

内部实现从：

```text
keyword candidates
```

升级为：

```text
Retriever.Retrieve(query)
```

输出增强：

```json
{
  "query": "...",
  "results": [
    {
      "ref_id": "...",
      "kind": "slide_html",
      "source": "context_index",
      "snippet": "...",
      "revision": 12,
      "hash": "...",
      "detail_available": true,
      "score": 0.86,
      "selection_reason": "matches target slide and latest render issue"
    }
  ]
}
```

### 6.6 Context Briefing 集成

每轮 Agent.Next 前：

```text
WorkSpec + RequirementLedger + LatestIssues + Plan current step
  -> RetrievalQuery
  -> top-k context
  -> ContextBriefing
  -> Prompt module context_briefing
```

Briefing 应包含：

- selected refs。
- why selected。
- freshness。
- whether full content is available via read/search。

### 6.7 按需展开

支持 detail levels：

```text
summary: 低 token 摘要
structure: HTML/JSON 结构摘要
full: 完整内容
```

规则：

- 默认注入 summary。
- repair 精确 HTML anchor 时允许 structure/full。
- full 内容超过预算时返回 ref + 建议 `read_ppt`。

## 7. Checkpoint / Resume / Recovery

### 7.1 核心原则

Checkpoint 是 Runtime 事实快照，不是完整 message dump。

它必须足够恢复：

- 当前 phase。
- plan。
- requirement ledger。
- changes。
- evidence。
- latest observations。
- context index revision。
- provider continuation if safe。

它不应该保存：

- provider 私有 reasoning。
- secrets。
- 本地绝对路径。
- 未脱敏用户隐私之外的信息。

### 7.2 Checkpoint 数据结构

扩展现有 `RuntimeCheckpoint`：

```go
type RuntimeCheckpoint struct {
    RunID string
    LoopID string
    Strategy ExecutionStrategy
    ExecuteMode ExecuteMode
    Phase RuntimePhase
    ResumePhase RuntimePhase
    Plan *Plan
    Requirements *RequirementLedger
    Changes ChangeSet
    Evidence []Evidence
    ContextIndexRef string
    ContextBriefing string
    MessageSummary []CheckpointMessage
    LatestToolResults []CheckpointToolResult
    WaitingQuestionID string
    ProviderContinuation *ProviderContinuationSnapshot
    Turns int
    ToolCalls int
    CompletionFailures int
    CreatedAt int64
}
```

新增：

```go
type CheckpointMessage struct {
    Role string
    Summary string
    ToolCallID string
    ToolName string
    Hash string
}
```

```go
type CheckpointToolResult struct {
    CallID string
    Tool string
    OK bool
    Code string
    Summary string
    ChangedTargets []ChangedTarget
    EvidenceIDs []string
}
```

### 7.3 Checkpoint 触发点

必须触发：

- strategy initialized。
- first valid plan accepted。
- every plan update。
- before ask_user wait。
- after user answer。
- after successful write tool。
- after render result。
- after Completion Gate rejection。
- before commit。
- after commit accepted。
- terminal failure/cancel。

周期触发：

```text
every N turns
every M tool calls
every T seconds
```

默认：

```text
N = 2 turns
M = 5 tool calls
T = 30 seconds
```

### 7.4 Resume 模式

新增 API / service 能力：

```go
ResumeRun(ctx, runID string) (model.Run, error)
```

恢复步骤：

1. 读取 run。
2. 读取最新 checkpoint。
3. 检查 run 是否 resumable。
4. 重建 ContextPack。
5. 校验 checkpoint 中 target hashes 与当前磁盘状态。
6. 重建 runtimeState。
7. 重新披露当前 phase 工具。
8. 继续 same logical run 或创建 recovery run。

### 7.5 Recovery Run

建议区分两种恢复：

| 模式 | 用途 |
|---|---|
| same-run resume | provider/network/server crash，状态一致 |
| recovery run | direct-write 文件状态已变、commit 失败、checkpoint 不完整 |

Recovery run 应带：

```text
parent_run_id
recovery_reason
checkpoint_id
reconciled_changes
```

### 7.6 Direct-write Reconciliation

当前 direct-write 会在失败后留下文件。Resume 前必须 reconcile：

```text
checkpoint changes
  vs
current disk hash
  vs
store revision metadata
```

结果：

- `clean`: 可继续。
- `dirty_same_run`: 当前 run 写过但未 commit，需要重新 validate/render。
- `external_modified`: 需要阻塞或创建 recovery run。
- `missing_artifact`: 需要重新生成或失败。

### 7.7 Provider Continuation

Provider continuation 可以保存，但必须：

- 作为 opaque snapshot。
- 不进入普通 message history。
- 带 provider/model/profile identity。
- resume 时验证 profile 一致。
- 如果 provider 不支持 continuation，则从 checkpoint message summary 恢复。

### 7.8 Recovery 状态机

新增内部状态：

```text
running
waiting_input
recovering
resuming
completion_check
committing
terminal
```

公共状态仍保持：

```text
pending / running / waiting / done / failed / canceled
```

## 8. 数据库与存储

### 8.1 run_checkpoints

新增表：

```sql
CREATE TABLE run_checkpoints (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,
  loop_id TEXT NOT NULL,
  seq INTEGER NOT NULL,
  phase TEXT NOT NULL,
  checkpoint_json TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  UNIQUE(run_id, seq)
);
```

索引：

```sql
CREATE INDEX idx_run_checkpoints_run_seq ON run_checkpoints(run_id, seq DESC);
```

### 8.2 context_index_snapshots

新增表或文件快照：

```sql
CREATE TABLE context_index_snapshots (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,
  pack_hash TEXT NOT NULL,
  index_json TEXT NOT NULL,
  created_at INTEGER NOT NULL
);
```

### 8.3 semantic_reviews

新增表：

```sql
CREATE TABLE semantic_reviews (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,
  finish_call_id TEXT NOT NULL,
  accepted INTEGER NOT NULL,
  confidence REAL NOT NULL,
  input_hash TEXT NOT NULL,
  output_json TEXT NOT NULL,
  prompt_manifest_json TEXT NOT NULL,
  created_at INTEGER NOT NULL
);
```

## 9. Public Events 与 Trace

默认不增加用户可见事件，避免暴露内部实现。

Trace 增加：

- `context.retrieved`
- `semantic.review.started`
- `semantic.review.completed`
- `checkpoint.saved`
- `checkpoint.loaded`
- `recovery.started`
- `recovery.reconciled`

调试面板可后续消费这些 trace。

## 10. Prompt 文件

新增：

```text
backend/prompts/semantic_reviewer/
  registry.go
  core/semantic_reviewer_policy.md
  rubrics/ppt_completion_rubric.md
  schemas/review_output_contract.md
```

Context Retrieval 不需要模型 prompt，除非后续做 query rewriting。若做 query rewriting，新增：

```text
backend/prompts/context_retrieval/
  query_rewrite.md
```

## 11. 实施计划

### Phase 1: Checkpoint 基础

目标：不实现 resume，先稳定保存 checkpoint。

改动：

- 扩展 `RuntimeCheckpoint`。
- 新增 store interface。
- 新增 SQLite migration。
- 在关键边界保存 checkpoint。
- 单测覆盖 checkpoint 内容。

### Phase 2: Context Retrieval 2.0

目标：替换 `search_refs` 内部检索实现，并接入 context briefing。

改动：

- 新增 ContextIndex。
- 新增 Retriever interface。
- 新增 Noop/Hash embedding provider。
- 改造 `reference_tool.go`。
- ContextBriefing 注入 selected refs。

### Phase 3: LLM Semantic Reviewer

目标：finish deterministic gate 后调用 reviewer。

改动：

- 新增 reviewer provider interface。
- 新增 prompt registry。
- 新增 JSON schema parse/validate。
- CompletionGate 接入 semantic reviewer result。
- reviewer 失败策略可配置。

### Phase 4: Resume / Recovery

目标：支持从 checkpoint 恢复。

改动：

- 新增 ResumeRun service。
- 新增 reconciliation。
- 支持 same-run resume 和 recovery run。
- 补全状态转移和 idempotency。

### Phase 5: Observability

目标：让问题可复盘。

改动：

- trace 查询增强。
- semantic review 结果落库。
- context retrieval selection reason 可查。
- checkpoint diff 工具。

## 12. 测试计划

### Unit Tests

- Reviewer JSON parse。
- Reviewer issue -> CompletionIssue mapping。
- Retrieval scope filter。
- Retrieval freshness filter。
- ContextIndex deterministic hash。
- Checkpoint serialization。
- Resume state reconstruction。
- Direct-write reconciliation。

### Integration Tests

- execute planned finish -> deterministic gate pass -> reviewer reject -> same loop repair。
- reviewer unavailable -> policy reject。
- search_refs returns semantic result with selection reason。
- crash simulation after write -> resume -> render evidence refreshed。
- commit failure -> recovery run created。

### Regression Tests

- talk/ask/plan read-only boundaries unchanged。
- `finish(message)` strict contract unchanged。
- existing CompletionGate evidence requirements unchanged。
- old ContextPack assembly remains compatible。

## 13. 风险

| 风险 | 控制 |
|---|---|
| Reviewer 幻觉误拒绝 | 强 JSON schema、只允许证据内判断、低 confidence 走 retry/人工策略 |
| token 成本上升 | 只在高风险 finish 启用 reviewer，review input 结构化压缩 |
| retrieval 引入过期事实 | revision/hash/freshness filter 硬约束 |
| checkpoint 泄漏敏感内容 | checkpoint 内容白名单，不保存 provider 私有 reasoning |
| resume 写坏文件 | reconciliation 先行，dirty 状态重新 validate/render |

## 14. 完成标准

- Semantic Reviewer 可在 configured runs 中阻止语义未完成 finish。
- Reviewer rejection 能回到同一 ReAct loop，并给出可执行 issue。
- search_refs 使用 ContextRetriever，结果带 score、selection reason、revision、hash。
- ContextBriefing 展示 retrieved refs。
- Runtime 能保存关键 checkpoint。
- Resume 能从 clean checkpoint 恢复。
- Recovery 能识别 direct-write dirty 状态并要求重新验证。
- 所有关联测试通过。
