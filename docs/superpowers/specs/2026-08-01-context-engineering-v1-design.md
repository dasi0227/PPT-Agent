---
id: CONTEXT-ENGINEERING-V1
title: HTML PPT Agent Context Engineering v1
status: implemented
owner: shared
date: 2026-08-01
depends_on:
  - ARTIFACT-TARGET-BLUEPRINT-REFACTOR
  - docs/v2/40-api-and-data-contracts.md
---

# HTML PPT Agent Context Engineering v1

## 1. 决策

在现有 `WorkSpec` 与 Blueprint v2 之上建立独立的 Context Engineering 层。它是所有
`blueprint/presentation × slide/deck` Runner、Planner、Executor、Verifier 和 consult
流程获取业务上下文的唯一入口。

本项目处于 0→1 开发阶段，不为旧 `kind/scope/mode`、旧 `slidejson`、旧项目数据或旧 Prompt
拼装方式建设兼容层。Context v1 先提供新内核；后续 PEV Runtime 重构完成后删除仍依赖旧模型的
Runner 实现。

核心原则：

1. **Context 是编译产物**：由 WorkSpec、项目快照、Thread Memory 和 Policy 确定性组装。
2. **足够但不过量**：目标产物优先，相关摘要次之，无关完整内容禁止进入。
3. **先地图、后细节**：大型内容先披露索引或摘要，需要时通过受控引用展开。
4. **可预算**：以估算 token，而不是消息条数作为预算单位。
5. **可复现**：每个片段携带来源、revision、hash 和选择原因。
6. **不可绕过**：业务 Agent 不得自行从文件系统读取并拼接项目上下文。

## 2. 最新代码基线

截至 `3b335ff`：

- 公共 Run 协议已是 `WorkSpec`。
- 四种 Artifact Target 已由 `RunnerResolver` 路由。
- `deck.json`、per-slide `slide.json`、`design-spec.json` 和 revision 已落地。
- 单页编辑会手工读取目标页 HTML 与 JSON。
- 整份生成会给逐页子循环注入设计摘要。
- 大纲编辑会生成 `id/idx/layout/title` 文本摘要。
- Harness 以 40 条消息为阈值，超限后插入固定占位摘要。
- Thread history 已写入 JSONL，但不会进入下一次 Run 的模型上下文。

这些能力说明项目已有 Context Engineering 雏形，但缺少统一模型、统一预算、真实 Memory、来源追踪
和按需披露协议。

## 3. 目标与非目标

### 3.1 目标

- 四种 Target 使用同一个 `ContextAssembler`。
- 每种 Target 有明确的 `ContextProfile`。
- 同一项目状态与 WorkSpec 生成稳定的 Context Pack。
- 支持 Thread Memory 与最近相关历史。
- 支持 token budget、优先级裁剪和预留输出预算。
- 支持 ContextRef 渐进披露。
- 记录 ContextManifest，能够解释为何加载或舍弃某项内容。
- 为 PEV Runtime 提供结构化输入，不与某个具体 Prompt 模板绑定。

### 3.2 非目标

- 不建设向量数据库或通用 RAG 平台。
- 不做跨项目长期用户画像。
- 不接入多模型路由。
- 不建设完整 Run Trace 平台。
- 不在本阶段实现 Durable Run。
- 不把所有项目文件都转换为 embedding。
- 不让模型通过 ContextRef 读取任意磁盘路径。

## 4. 总体架构

```text
WorkSpec + Run Identity
          │
          ▼
ContextProfileResolver
          │
          ▼
ContextAssembler
  ├── ProjectLoader
  ├── BlueprintLoader
  ├── DesignLoader
  ├── PresentationLoader
  ├── AssetIndexLoader
  ├── ThreadMemoryLoader
  └── RevisionLoader
          │
          ▼
Relevance + Budget + Compression
          │
          ▼
ContextPack + ContextManifest
          │
          ├── PromptCompiler
          └── ContextRefResolver
```

建议后端包：

```text
backend/internal/contextengine/
├── assembler.go
├── profile.go
├── pack.go
├── manifest.go
├── budget.go
├── estimate.go
├── compiler.go
├── memory.go
├── refs.go
├── loaders/
└── testdata/
```

不要命名为 `context`，避免与 Go 标准库 `context` 混淆。

## 5. 核心模型

### 5.1 ContextRequest

```go
type ContextRequest struct {
    RunID     string
    ThreadID  string
    ProjectID string
    WorkSpec  model.WorkSpec
    Budget    TokenBudget
}
```

`ProjectID` 必须由 Thread 反查，不能从客户端 body 信任。

### 5.2 ContextPack

```go
type ContextPack struct {
    SchemaVersion string
    Profile       ProfileID
    WorkSpec      model.WorkSpec
    Project       ProjectContext
    Deck          DeckContext
    Target        TargetContext
    RelatedSlides []SlideSummary
    Design        DesignContext
    Presentation  PresentationContext
    Assets        []AssetCandidate
    Memory        ThreadMemory
    Revisions     RevisionRefs
    Manifest      ContextManifest
}
```

约束：

- slice 使用空数组，不使用 `null`。
- `Target` 必须与 WorkSpec 一致。
- slide target 只能有一个完整目标页。
- related slides 只允许摘要，除非通过 ContextRef 后续披露。
- Pack 不保存工具列表；工具披露由 PEV Runtime 的 Step Policy 决定。
- Pack 中不能包含密钥、任意绝对路径和原始 chain-of-thought。

### 5.3 TargetContext

```go
type TargetContext struct {
    Artifact            model.Artifact
    Level               model.TargetLevel
    Slide               *blueprint.Slide
    Materialization     *blueprint.Materialization
    PresentationSummary *HTMLSummary
    PresentationRef     *ContextRef
}
```

### 5.4 ContextManifest

```go
type ContextManifest struct {
    ContextID       string
    Profile         ProfileID
    EstimatedTokens int
    BudgetTokens    int
    OutputReserve   int
    PackHash        string
    Segments        []ContextSegment
    Refs            []ContextRef
    Dropped         []DroppedSegment
}
```

每个 `ContextSegment` 至少记录：

- `id`
- `kind`
- `source_ref`
- `revision`
- `content_hash`
- `estimated_tokens`
- `priority`
- `selection_reason`
- `detail_level`

Manifest 可以持久化；默认不持久化完整 HTML、完整工具结果或敏感原文。

### 5.5 RevisionRefs

```go
type RevisionRefs struct {
    Deck          int
    Design        int
    Slides        map[string]int
    Presentations map[string]int
    ThreadMemory  int
}
```

ContextRef 展开前必须校验当前 revision；过期则返回 `CONTEXT_REF_STALE`，要求重新 Assemble。

## 6. Context Profile

### 6.1 `blueprint/deck`

必须加载：

- WorkSpec。
- 完整 Deck。
- 所有 slide 的轻量摘要。
- section/subsection 结构。
- DesignSpec 摘要。
- Thread Memory。
- revision refs。

不得默认加载：

- 任意页面完整 HTML。
- 资产源码。
- 所有页面完整 Blueprint。

### 6.2 `blueprint/slide`

必须加载：

- Deck goal、audience、core thesis、narrative arc。
- 目标页完整 Blueprint。
- 所属 section/subsection。
- 前一页、后一页摘要。
- 同 subsection 其他页的 title/key message。
- DesignSpec 摘要。
- Thread Memory。

不得默认加载：

- 其他页面完整 Blueprint。
- 任意页面完整 HTML。

### 6.3 `presentation/deck`

必须加载：

- Deck。
- 所有 slide Blueprint 摘要。
- 完整 DesignSpec 或预算内的结构化设计切片。
- 所有页面 materialization state。
- 每页 HTMLSummary。
- 相关资产索引。
- Thread Memory。

完整 HTML 只能通过 ContextRef 按页读取。

### 6.4 `presentation/slide`

必须加载：

- Deck 核心目标和目标页章节位置。
- 目标页完整 Blueprint。
- 前后页摘要。
- 完整 DesignSpec。
- 目标页 materialization state。
- 目标页 HTMLSummary。
- 若预算允许且操作需要精确 patch，加载目标页完整 HTML；否则提供 ContextRef。
- 与 `visual_intent.asset_queries` 匹配的资产候选。
- Thread Memory。

不得加载其他页完整 HTML。

### 6.5 `consult`

`consult` 使用与相同 target 对应的 Profile，但：

- 默认不加载写入实现细节。
- 允许读取与回答直接相关的项目信息。
- ContextManifest 标记 `read_only=true`。
- 后续 Tool Policy 不得披露写工具。

## 7. Context Loaders

每个 Loader 必须是独立、可测试的小接口：

```go
type Loader interface {
    Load(ctx context.Context, req LoadRequest) ([]CandidateSegment, error)
}
```

候选 Loader：

- `ProjectLoader`
- `DeckLoader`
- `SlideBlueprintLoader`
- `RelatedSlideLoader`
- `DesignSpecLoader`
- `PresentationSummaryLoader`
- `AssetCandidateLoader`
- `ThreadMemoryLoader`
- `RevisionLoader`

Loader 只负责读取与结构化，不负责最终裁剪，不拼 Prompt。

错误分类：

- required segment 缺失：Assembly 失败。
- optional segment 缺失：记录 warning 后继续。
- revision 冲突：Assembly 失败并重读一次。
- 内容损坏：返回结构化 `CONTEXT_SOURCE_INVALID`。

## 8. HTML 与大型内容摘要

### 8.1 HTMLSummary

目标 HTML 默认先转换为：

```go
type HTMLSummary struct {
    Title          string
    Structure      []DOMBlock
    TextDigest     []string
    AssetRefs      []string
    TokenRefs      []string
    ScriptFeatures []string
    Warnings       []string
    SourceHash     string
}
```

摘要由确定性解析器生成，不先调用 LLM：

- 提取 section/article/header 等语义块。
- 提取标题与主要文本。
- 提取 class/id/data-* 锚点。
- 提取图片、图表、外部资源。
- 提取 CSS token 引用。
- 标记 inline script/style 大小。

只有 Thread Memory 的语义压缩可以调用 LLM；项目文件摘要应尽量确定性。

### 8.2 工具 Observation

- 小结果直接进入当前步骤上下文。
- 大结果只保留 summary、artifact refs、issue codes。
- 完整结果注册为 ContextRef。
- 后续回合需要时再展开。

## 9. Token Budget

### 9.1 模型

```go
type TokenBudget struct {
    ContextWindow int
    InputLimit    int
    OutputReserve int
    SegmentCaps   map[SegmentKind]int
}
```

预算来自当前模型配置；没有 provider 精确 tokenizer 时使用稳定估算器，并预留安全系数。

### 9.2 默认分配

建议基线：

| Segment | 比例 |
|---|---:|
| system/policy | 15% |
| WorkSpec | 5% |
| deck/section | 15% |
| target artifact | 30% |
| design | 10% |
| thread memory/recent turns | 10% |
| tool observations | 10% |
| safety reserve | 5% |

Profile 可覆盖比例，但总输入不得侵占 `OutputReserve`。

### 9.3 裁剪顺序

1. 删除低相关资产候选。
2. 删除同 subsection 之外的页面摘要。
3. 完整 HTML 降级为 structure。
4. structure 降级为 summary。
5. 早期 recent turns 合并到 Memory。
6. 保留目标 Blueprint、WorkSpec、安全约束和 revision。

目标产物与安全约束永不被裁掉。

## 10. 相关性选择

不引入向量数据库。v1 使用确定性与轻量文本相关性：

- stable slide id 精确命中。
- section/subsection 关系。
- 前后页关系。
- `asset_queries` 与资产 tags/name/description 关键词匹配。
- 用户指令中的显式页面、主题、组件名称。
- 最近一次影响目标 artifact 的 Run。

每个选择必须写入 `selection_reason`。

## 11. Thread Memory

### 11.1 存储

每个 thread 保存：

```text
threads/<thread-id>.memory.json
```

结构：

```json
{
  "schema_version": "1.0",
  "revision": 3,
  "user_preferences": [],
  "confirmed_decisions": [],
  "brand_constraints": [],
  "content_facts": [],
  "open_questions": [],
  "recent_changes": []
}
```

### 11.2 更新规则

- 只在成功 Run 后更新。
- failed/canceled 不写入新决定。
- consult 可记录用户明确确认的偏好，但不记录 Agent 猜测。
- 每条 memory 携带来源 run id 和时间。
- 同一事实按 key 合并，避免无限增长。
- 冲突事实保留最新已确认值，并记录 supersedes。
- 不持久化 chain-of-thought。

### 11.3 历史使用

- Memory 是长期语义状态。
- RecentTurns 是最近少量可见对话。
- 原始 history 仅作为可按需读取的证据，不默认全量注入。
- 固定占位“早期轮次已摘要”必须删除。

## 12. 渐进披露

### 12.1 ContextRef

```go
type ContextRef struct {
    ID              string
    Kind            RefKind
    ProjectID       string
    TargetID        string
    Revision        int
    ContentHash     string
    Summary         string
    AvailableLevels []DetailLevel
    EstimatedTokens map[DetailLevel]int
}
```

`ID` 是不透明引用，不得是可控文件路径。

Detail level：

- `summary`
- `structure`
- `full`

### 12.2 `read_context_ref`

工具参数：

```json
{
  "ref_id": "ctxref_xxx",
  "detail": "structure"
}
```

执行规则：

- ref 必须属于当前 Run 的 ContextManifest。
- ref 的 project/thread 必须匹配当前 Run。
- 当前 Step 必须具备 `read_context` capability。
- 校验 revision/hash。
- 检查剩余 token budget。
- 不允许通过参数指定路径。
- 返回内容、token 估算、revision 和新的子引用。

### 12.3 与工具动态披露的区别

- 渐进上下文披露：决定某个已授权信息读取多少。
- 工具动态披露：决定本轮模型能看见哪些工具。

Context v1 实现 `read_context_ref`；PEV Runtime 按 Step 决定是否把该工具 schema 暴露给模型。

## 13. Prompt Compiler

ContextPack 不直接等于 Prompt。`PromptCompiler` 将它转换为稳定分区：

```text
<work_spec>...</work_spec>
<project_context>...</project_context>
<target_context>...</target_context>
<related_context>...</related_context>
<design_context>...</design_context>
<memory>...</memory>
<available_context_refs>...</available_context_refs>
```

规则：

- System policy 与用户内容分离。
- 用户原文只放 user 层。
- Context 中的项目文本视为数据，不允许覆盖 system policy。
- 片段顺序稳定。
- 空字段不生成噪音段落。
- 编译结果可 snapshot test。

## 14. Run 集成

目标数据流：

```text
POST Run
  → resolve thread/project
  → validate WorkSpec
  → Assemble ContextPack
  → persist ContextManifest
  → emit context.assembled
  → RunnerResolver / Workflow
```

`RunnerResolver` 构造参数必须接收 `ContextPack`。新 Runner 禁止重复从磁盘装配相同上下文。

事件：

```json
{
  "type": "context.assembled",
  "context_id": "ctx_xxx",
  "profile": "presentation/slide",
  "estimated_tokens": 6200,
  "budget_tokens": 12000,
  "segments": 9,
  "refs": 3,
  "warnings": []
}
```

前端只展示简短状态，不展示完整 Context 内容。

### 14.1 Manifest 存储

使用 SQLite 保存可审计但不含完整敏感载荷的 manifest：

```text
run_contexts
├── run_id              TEXT PRIMARY KEY
├── context_id          TEXT UNIQUE NOT NULL
├── profile             TEXT NOT NULL
├── pack_hash           TEXT NOT NULL
├── estimated_tokens    INTEGER NOT NULL
├── budget_tokens       INTEGER NOT NULL
├── manifest_json       TEXT NOT NULL
└── created_at          INTEGER NOT NULL
```

- `manifest_json` 保存 segments、refs 元数据、dropped 和 warnings。
- 不保存完整 HTML、完整资产源码或原始历史全文。
- ContextRef 的正文在读取时根据 manifest 中的受控 source identity 重新加载。
- 当前 Run 的 Ref Registry 保存在进程内；未来 Durable Run 可从 manifest 重建。
- 本项目无需兼容旧数据库，直接更新 canonical migration 并要求开发数据库重建。

## 15. 与 PEV Runtime 的边界

Context Engine 负责：

- 读取、选择、压缩、引用、编译业务上下文。
- 管理 Context Budget 与 Thread Memory。
- 产生 ContextManifest。

PEV Runtime 负责：

- 选择 Playbook。
- 制定 Plan。
- 决定每个 Step 的 capabilities。
- 动态披露工具。
- Execute、Verify、Repair 和 Commit。

Context Engine 不决定写工具，不执行项目修改。

## 16. 清理策略

本阶段不建设兼容适配器。最终 PEV 完成后删除：

- `model.Kind`
- `model.Scope`
- `model.Mode`
- `harness.Gate(scope, mode)`
- Runner 内部手工 Context 拼接
- 固定条数消息压缩
- `slidejson` 兼容投影
- `outline_dirty`
- 旧 Context 文档

为了两个实施 Prompt 可以按顺序独立提交，Context 阶段允许旧 Runner 暂时存在，但新
Context API、存储和测试不得依赖旧协议；PEV 阶段必须完成删除。

## 17. 测试

### 17.1 单元测试

- 四种 Profile 的 required/forbidden segments。
- slide target 不含其他页完整 HTML。
- deck target 不含所有页完整 HTML。
- 相同输入产生相同 PackHash。
- revision 变化导致 PackHash 变化。
- token estimation 和裁剪顺序。
- target artifact 永不被裁剪。
- ContextRef 不接受任意路径。
- ContextRef 跨 project/run 被拒绝。
- stale ref 被拒绝。
- Thread Memory 合并、冲突和 revision。
- PromptCompiler 稳定快照。

### 17.2 集成测试

- 四种 WorkSpec 创建 Run 前均完成 Context Assembly。
- consult pack 正确但不会带来写 capability。
- presentation/slide 可通过 ref 读取目标 HTML full。
- blueprint/slide 无法读取其他页 HTML full。
- history 影响下一次 Run 的 Memory。
- 超预算时返回可诊断 manifest，而不是截断 JSON。
- context.assembled 可经 SSE 和 history replay。

### 17.3 回归

- Go 全量测试。
- 前端全量测试、TypeScript 和 lint。
- `/render` 与预览隔离测试继续通过。

## 18. Definition of Done

- `ContextAssembler` 是四种 Target 的统一上下文入口。
- 每种 Target 有显式 Profile 与隔离测试。
- ContextPack 类型化、可 hash、可 snapshot。
- ContextManifest 记录来源、revision、token 和选择原因。
- Thread Memory 能跨 Run 生效且不无限增长。
- 固定消息条数压缩和假摘要被移除。
- 大型 HTML、资产和历史支持 ContextRef 渐进披露。
- `read_context_ref` 无任意路径读取能力。
- 所有新 Prompt 从 ContextPack 编译，不再手工拼项目上下文。
- 新代码只使用 WorkSpec/Blueprint v2，不新增旧协议适配。
- 全量测试通过。
