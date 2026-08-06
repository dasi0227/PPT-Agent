# Agent Runtime Cognitive Upgrade Design

> 日期：2026-08-06
> 范围：PPT Agent Runtime 的 Prompt Engineering、Context Engineering 与 Loop Engineering 升级
> 状态：可直接执行

## 1. 背景

当前 Agent Runtime 已经具备生产级执行骨架：策略路由、动态工具披露、单一 ReAct Loop、显式 `finish(message)`、Completion Gate、Evidence Ledger、Context Manifest、HITL 与幂等控制。

但现有实现更偏“安全执行器”，而不是“强思维执行器”。模型主要依赖自身推理完成任务分解、上下文聚焦、质量判断与最终交付。Runtime 对模型的认知支架不足，导致复杂任务中容易出现以下问题：

- 系统提示词是单段硬编码字符串，难以版本化、灰度、回滚和评测。
- 不同运行模式主要靠工具披露差异，缺少模式专属 prompt。
- 任务 playbook、错误修复指南、PPT 质量标准没有结构化注入。
- `finish(message)` 虽是唯一成功出口，但缺少严格协议防止完整交付物误放在 assistant text。
- 上下文注入是固定 JSON section，缺少每轮动态任务简报。
- Completion Gate 强调证据新鲜度，但没有独立的需求覆盖检查。

本次升级目标是补齐 Agent 的“思维能力工程层”，让 Runtime 不只是限制模型，而是持续帮助模型聚焦、规划、修复、验收和交付。

## 2. 目标

1. 将系统提示词拆成可组合、可追踪的版本化模块。
2. 为 `plan/talk/ask/execute-direct/execute-planned` 构建专属 prompt。
3. 注入任务 playbook，引导模型按 PPT 任务类型选择行动路径。
4. 为 Completion Gate issue 增加模型可读 repair guide。
5. 增加 `finish` 严格协议，确保最终交付物必须在 `finish.message`。
6. 引入 PPT 质量 rubric，提升生成和修复判断的一致性。
7. 每轮生成 `context_briefing`，包括动态 working set summary。
8. 增加 requirement ledger，跟踪用户需求覆盖状态。
9. 增加 semantic completion check，作为 Completion Gate 的语义覆盖层。

## 3. 非目标

- 不引入固定多节点 workflow。
- 不恢复 Plan-Execute-Verify DAG。
- 不为每个 plan step 启动独立 loop。
- 不把 semantic completion check 做成无限制第二模型 verifier。
- 不在本批实现周期性 checkpoint；该能力后续与 resume/compaction 统一设计。

## 4. 总体架构

本次升级保持原有单一 ReAct Loop 架构不变，在三个位置增强：

```text
Context Assembler
  -> ContextPack

Runtime Loop
  -> RequirementLedger
  -> ContextBriefing
  -> PromptModule Assembly
  -> Agent.Next
  -> Tool / Control Execution
  -> Evidence + Requirement updates
  -> Completion Gate
       -> Evidence policies
       -> Plan policy
       -> Semantic completion policy
       -> Finish contract guard
```

核心原则：

- Prompt 负责指导，不负责授权。
- ToolRegistry 和 CompletionGate 仍是硬边界。
- RequirementLedger 是语义覆盖记录，不是执行 DAG。
- ContextBriefing 是每轮动态简报，WorkingSetSummary 是其中一节。
- Semantic completion check 只判断“需求是否明显未覆盖”，不替代业务工具的确定性校验。

## 5. Prompt Engineering

### 5.1 版本化模块

新增文件化 Prompt Registry。所有稳定提示词正文放在 `backend/prompts/runtime/**/*.md`，Go 代码只负责 `go:embed` 加载、模块选择、动态变量注入和最终拼装。

新增 `PromptModule` 抽象：

```go
type PromptModule struct {
    ID      string
    Version string
    Path    string
    Hash    string
    Body    string
}
```

系统提示词由多个模块拼装，每个模块在输出中保留 `id/version/path/hash`，便于 trace、评测、回滚和差异定位：

```text
<prompt_module id="core_runtime_policy" version="2026-08-06.v1" path="core/core_runtime_policy.md" hash="...">
...
</prompt_module>
```

文件结构：

```text
backend/prompts/runtime/
  registry.go
  core/
    core_runtime_policy.md
  business/
    ppt_business_policy.md
  resources/
    resource_contracts.md
  modes/
    talk.md
    ask.md
    plan.md
    execute_direct.md
    execute_planned.md
  playbooks/
    read_only_planning.md
    read_only_collaboration.md
    spec_edit.md
    slide_presentation_edit.md
    empty_deck_generation.md
    deck_coordinated_edit.md
    default.md
  guides/
    completion_repair_guide.md
    finish_contract.md
  rubrics/
    ppt_quality_rubric.md
```

首批模块：

| 模块 | 作用 |
|---|---|
| `core_runtime_policy` | 单一 ReAct、工具边界、普通文本不结束 |
| `mode_policy_*` | talk/ask/plan/execute-direct/execute-planned 专属策略 |
| `playbook_*` | 当前任务类型的行动建议 |
| `completion_repair_guide` | Gate issue 到修复动作的映射 |
| `finish_contract` | `finish.message` 完整交付协议 |
| `ppt_quality_rubric` | PPT 质量标准 |
| `ppt_business_policy` | Outline/Design/Spec/HTML 职责 |
| `resource_contracts` | 资源命名和 schema contract |
| `context_briefing` | 每轮动态简报 |
| `runtime_state` | 当前 plan/change/evidence/requirement 状态 |

### 5.2 模式专属 Prompt

按 `strategy + execute_mode + phase` 选择 mode module：

| 模式 | Prompt 重点 |
|---|---|
| `talk` | 只读分析，可读资源和检索，禁止承诺写入 |
| `ask` | 只读协作，优先提出阻塞问题，问题必须原子化 |
| `plan` | 只读规划，禁止 `update_plan`，完整方案放入 `finish.message` |
| `execute-direct` | 小范围直接执行，尽快读-写-验-交付 |
| `execute-planned` | 先维护轻量计划，再分阶段执行和验收 |

### 5.3 任务 Playbook

根据 WorkSpec、目标层级、artifact 类型和 materialization 状态生成任务 playbook。

首批 playbook：

- Empty deck generation：Outline -> Design -> Slide Spec -> Slide HTML -> Render。
- Deck-level coordinated edit：先识别影响面，再按依赖顺序修改。
- Slide presentation edit：读取目标 HTML/Spec，优先精确 edit，必要时 full write，随后 render。
- Spec-only edit：只修改 schema 资源，不写 HTML。
- Read-only planning：读取必要上下文，输出可执行方案，不声称已修改。

### 5.4 Completion Repair Guide

将 CompletionGate issue 转成模型可读修复指南：

| Issue | 指南 |
|---|---|
| `SCHEMA_EVIDENCE_REQUIRED` | 对目标资源重新 `write_ppt` 或修复 JSON schema |
| `STATIC_EVIDENCE_REQUIRED` | 修复 HTML 后重新写入或编辑 |
| `VISUAL_EVIDENCE_REQUIRED` | 对目标 slide 调用 `render_slide`，根据诊断修复 |
| `REFERENCE_EVIDENCE_REQUIRED` | 保证 outline/spec 引用一致，重新写相关资源 |
| `PLAN_INCOMPLETE` | 更新计划，将完成项标记 completed，修复 pending/failed |
| `TARGET_OUT_OF_SCOPE` | 停止越权目标，改为当前授权目标或询问用户 |
| `REQUIREMENT_UNADDRESSED` | 回到需求 ledger，完成缺失需求后再 finish |

### 5.5 Finish 严格协议

`finish` 是最终交付，不是信号。

新增 Runtime guard：

- 若 `finish.message` 为空，拒绝。
- 若 assistant text 中存在明显完整交付物，而 `finish.message` 只是摘要，拒绝。
- 若 assistant text 比 `finish.message` 实质性更长，拒绝。
- 拒绝以 tool observation 形式返回同一 ReAct Loop，要求模型重新把完整内容放入 `finish.message`。

### 5.6 PPT 质量 Rubric

注入固定质量标准：

- Narrative：叙事主线清晰。
- Hierarchy：标题、重点、辅助信息层级分明。
- Density：信息密度受控。
- Consistency：字体、颜色、间距、组件风格一致。
- Layout：1600x900 画布内布局稳定。
- Accessibility：语义 HTML、alt text、可读字号。
- Visual craft：不只是堆文本，有明确视觉结构。
- Render proof：无溢出、遮挡、控制台错误和资源失败。

## 6. Context Engineering

### 6.1 Context Briefing

每轮调用模型前，Runtime 根据当前状态生成 `context_briefing`：

```text
<context_briefing>
Objective: ...
Mode: ...
Authority: ...
Requirement ledger: ...
Working set:
  - Plan: ...
  - Changes: ...
  - Evidence: ...
  - Latest issues: ...
Next focus: ...
</context_briefing>
```

它不是替代 ContextPack，而是把模型从大量 JSON 中拉回当前任务焦点。

### 6.2 Working Set Summary

`working_set_summary` 本批不作为独立系统，只作为 `context_briefing` 内部小节：

- 当前 plan 状态。
- 已改目标。
- 最新 evidence。
- 最新 gate/tool issue。
- 下一步建议。

后续若引入 compaction/resume/parallel worker，再抽象成独立结构。

## 7. Loop Engineering

### 7.1 Requirement Ledger

新增 `RequirementLedger`：

```go
type RequirementLedger struct {
    Items []RequirementItem
}

type RequirementItem struct {
    ID       string
    Text     string
    Status   string // pending | in_progress | satisfied
    Evidence []string
}
```

初始化来源：

- 用户 instruction 的分句。
- WorkSpec target 与 interaction。
- desired slide count / theme / language 等 options。

更新规则：

- 成功读取相关资源：可作为上下文证据。
- 成功写入授权目标：需求进入 `in_progress` 或 `satisfied`。
- 成功 render 且无 blocking issue：可作为视觉完成证据。
- CompletionGate 拒绝：保留为最新阻塞。

### 7.2 Semantic Completion Check

首批实现为确定性语义覆盖检查，不另起 LLM verifier：

- `execute` 任务若没有任何变更，不允许成功 finish。
- requirement ledger 存在 pending 项时，返回 `REQUIREMENT_UNADDRESSED`。
- `finish.message` 太空泛时，返回可修复问题。

后续可扩展为可插拔 `SemanticCompletionChecker`，仅在复杂 planned run 或高风险写任务中调用模型评审。

## 8. 数据与接口变更

新增内部结构：

- `PromptModule`
- `backend/prompts/runtime` 文件化 prompt registry
- `ContextBriefing`
- `RequirementLedger`
- `RequirementItem`

扩展：

- `AgentRequest` 增加 `ContextBriefing string`。
- `runtimeState` 增加 `requirements *RequirementLedger`。
- `CompletionContext` 增加 `Requirements *RequirementLedger` 与 `FinishMessage string`。
- `CompletionGate` 增加 semantic policy。

无外部 API breaking change。

## 9. 兼容性

- 现有 ToolSchema 不变。
- 现有 SSE 事件不变。
- 现有 WorkSpec 不变。
- 现有 ContextManifest 不变。
- Prompt 输出变长，但仍在当前 context budget 范围内。

## 10. 测试计划

新增或更新单测：

1. Prompt 包含从 `.md` 文件加载的版本化模块，并记录 path/hash。
2. 不同 strategy/execute mode 选择不同 mode policy。
3. Prompt 包含 playbook、repair guide、PPT rubric。
4. Context briefing 包含目标、模式、计划、变更、证据和 requirement ledger。
5. `finish` 严格协议拒绝 assistant text 承载完整交付物。
6. execute 无变更 finish 被 semantic gate 拒绝。
7. 成功写入/渲染后 requirement ledger 不阻塞完成。
8. 现有 CompletionGate、工具披露、plan 模式测试继续通过。

## 11. 实施顺序

1. 新增设计文档。
2. 新增 `backend/prompts/runtime/**/*.md` 与 `go:embed` registry，替换硬编码 system prompt。
3. 新增 prompt module builder，仅负责选择、动态变量注入和拼装。
4. 新增 context briefing，并注入 AgentRequest。
5. 新增 requirement ledger，并在工具结果后更新。
6. 新增 finish strict guard。
7. 新增 semantic completion policy。
8. 更新测试并运行后端 workflow/contextengine 相关测试。

## 12. 风险与控制

| 风险 | 控制 |
|---|---|
| Prompt 过长 | 模块保持短文本，ContextPack 仍走预算机制 |
| Semantic gate 误拒绝 | 首批只做保守确定性检查 |
| 模型不遵守 finish 协议 | Runtime guard 拒绝并回灌 observation |
| planned 模式更慢 | 不增加额外 loop，只增加 prompt 与 gate 检查 |
| 现有测试快照变动 | 改为检查模块关键 contract，不做全文快照 |

## 13. 完成标准

- Runtime system prompt 由 `backend/prompts/runtime/**/*.md` 文件化版本模块生成。
- 五类运行模式具备专属 mode policy。
- Prompt 中包含 playbook、repair guide、finish contract、PPT rubric。
- 每轮模型请求包含 context briefing。
- Runtime 初始化并维护 requirement ledger。
- Completion Gate 能拒绝明显未执行的 execute finish。
- `finish.message` 严格承载最终交付物。
- 相关 Go 单测通过。
