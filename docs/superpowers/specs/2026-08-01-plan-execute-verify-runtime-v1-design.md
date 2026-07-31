---
id: PLAN-EXECUTE-VERIFY-RUNTIME-V1
title: HTML PPT Agent Plan–Execute–Verify Runtime v1
status: approved-for-planning
owner: shared
date: 2026-08-01
depends_on:
  - CONTEXT-ENGINEERING-V1
  - ARTIFACT-TARGET-BLUEPRINT-REFACTOR
---

# HTML PPT Agent Plan–Execute–Verify Runtime v1

## 1. 决策

建立一个面向 HTML PPT 的固定业务 Workflow Runtime，以
`WorkSpec → Context → Plan → Execute → Verify → Repair → Commit → Deliver`
取代当前多个业务 Runner 各自编排 ReAct Loop 的结构。

四种 Target 共用同一个 Engine：

- `blueprint/deck`
- `blueprint/slide`
- `presentation/deck`
- `presentation/slide`

差异由 Playbook、Planner、Executor 和 Verifier 表达，不再由
`outline/generate/edit/overview` 包和 `Kind/Scope/Mode` 表达。

项目处于 0→1 阶段：

- 不兼容旧 Run 内部协议。
- 不保留旧 Runner 适配层。
- 不保留旧事件兼容投影。
- 不保留旧数据迁移与双读写。
- 开发数据库和示例项目允许按新模型重建。

## 2. 最新代码基线

现有优势：

- WorkSpec 和四种 Target 已落地。
- Blueprint、DesignSpec、materialization revision 已落地。
- `presentation/deck` 首次物化已有 Design、Plan、Generate、Validate、Deliver 五阶段流水线。
- 逐页生成使用隔离的 Harness Loop。
- 已有静态 HTML lint、有限修复、版本快照、SSE、HITL、取消和熔断。
- 已有 Capability 类型雏形。

现有结构问题：

- 顶层仍由 `outline/generate/edit/overview` 四套旧 Runner 实现。
- Harness Config 和工具 Gate 仍依赖 Kind/Scope/Mode。
- Capability Gate 尚未成为所有生产 Loop 的唯一入口。
- 只有整份首次物化拥有显式流水线；其他路径直接进入 ReAct。
- Plan 是页面进度列表，不是可执行的结构化计划。
- Execute 直接写正式文件，Verify 失败后的事务边界不统一。
- 单页生成跳过整份流水线，也缺少统一单页 Verify/Repair 状态。
- Blueprint 与 presentation 的 Verify 协议不同且分散。
- `generate.DesignSpec` 与 `blueprint.DesignSpec` 双模型并存。
- 事件仍包含原始 `thought`，阶段关系表达不足。

## 3. 目标与非目标

### 3.1 目标

- 一个固定、可测试、可观察的 Workflow Engine。
- 四种 Target 均有明确 Playbook。
- Plan 是 Runtime 可执行的结构化对象。
- 每个 Step 使用最小工具集合。
- Execute 写入 staging，Verify 通过后才 Commit。
- Verifier 统一输出 `Issue`。
- Repair 只针对验证证据，且有明确轮次预算。
- Context Engineering 是所有阶段的唯一业务上下文来源。
- 前端以 Stage/Step 展示真实执行过程。
- 删除旧领域语言和旧 Runner 结构。

### 3.2 非目标

- 不建设可配置通用 DAG 平台。
- 不允许用户编辑 DAG。
- 不实现分布式调度。
- 不实现进程重启后的 Durable Resume。
- 不实现完整 Eval 平台。
- 不实现多模型路由。
- 不为第三方任意 Agent 提供 SDK。
- 不并行写同一个项目。

## 4. 总体架构

```text
RunService
   │
   ├── Validate WorkSpec
   ├── Assemble ContextPack
   └── Select Playbook
            │
            ▼
      Workflow Engine
       ├── Normalize
       ├── Plan
       ├── Execute
       ├── Verify
       ├── Repair
       ├── Commit
       └── Deliver
            │
            ▼
      Run Events / History
```

建议结构：

```text
backend/internal/
├── contextengine/
├── workflow/
│   ├── engine.go
│   ├── state.go
│   ├── plan.go
│   ├── event.go
│   ├── budget.go
│   ├── playbook/
│   │   ├── blueprint_deck.go
│   │   ├── blueprint_slide.go
│   │   ├── presentation_deck.go
│   │   └── presentation_slide.go
│   ├── planner/
│   ├── executor/
│   ├── verifier/
│   ├── repair/
│   └── transaction/
├── artifact/
├── blueprint/
├── presentation/
├── harness/
│   └── react.go
└── tools/
```

`harness/react.go` 是 Execute Step 可选的认知循环，不再拥有顶层业务生命周期。

## 5. Workflow 状态

```go
type Stage string

const (
    StageNormalize Stage = "normalize"
    StageContext   Stage = "context"
    StagePlan      Stage = "plan"
    StageExecute   Stage = "execute"
    StageVerify    Stage = "verify"
    StageRepair    Stage = "repair"
    StageCommit    Stage = "commit"
    StageDeliver   Stage = "deliver"
)
```

```go
type WorkflowState struct {
    WorkflowID    string
    RunID         string
    WorkSpec      model.WorkSpec
    ContextID     string
    Stage         Stage
    Plan          WorkflowPlan
    CurrentStepID string
    Attempt       int
    RepairRound   int
    Issues        []Issue
    Changes       ChangeSet
    Status        WorkflowStatus
}
```

状态必须可 JSON 序列化，为未来 Durable Run 预留，但 v1 不要求进程重启恢复。

合法阶段迁移由 Engine 控制，Runner 和 LLM 不得任意跳阶段。

## 6. WorkflowPlan

### 6.1 Plan

```go
type WorkflowPlan struct {
    ID              string
    Version         int
    Goal            string
    Target          model.RunTarget
    Operation       Operation
    Assumptions     []Assumption
    Affected        []ArtifactRef
    Steps           []WorkflowStep
    SuccessCriteria []Criterion
    Budget          ExecutionBudget
}
```

`Operation`：

- `create`
- `revise`
- `materialize`
- `rebuild`
- `consult`

Operation 根据 artifact 当前状态推导，不从客户端传入。

### 6.2 Step

```go
type WorkflowStep struct {
    ID            string
    Kind          StepKind
    Title         string
    Instruction   string
    Targets       []ArtifactRef
    DependsOn     []string
    Preconditions []Condition
    Capabilities  []tools.Capability
    Verifiers     []VerifierRef
    Status        StepStatus
}
```

`StepKind` 最小集合：

- `analyze`
- `patch_blueprint`
- `replace_deck`
- `apply_design`
- `materialize_slide`
- `revise_slide`
- `validate`
- `repair`
- `commit`
- `deliver`

### 6.3 固定 Playbook + 受约束规划

不让 LLM 自由设计任意 DAG：

1. Playbook 先提供合法阶段骨架、可用 StepKind 和依赖约束。
2. Planner 根据 ContextPack 填写目标、假设、受影响 artifact、步骤说明和成功条件。
3. PlanValidator 确认无越权 target、无环、无未知 StepKind、capability 合法。
4. LLM 规划失败时使用确定性 fallback plan。

## 7. 四种 Playbook

### 7.1 `blueprint/deck`

```text
analyze narrative
→ replace/patch deck
→ patch affected slide blueprints
→ verify schema + references + narrative
→ commit blueprint transaction
→ deliver
```

验证：

- Deck Schema。
- slide order 与 stable ids。
- section/subsection 引用。
- 每页 title/key message。
- 页面角色与叙事基本完整性。
- 无重复或孤立页面。

### 7.2 `blueprint/slide`

```text
analyze target + neighbors
→ patch target slide blueprint
→ verify schema + section coherence + local narrative
→ commit slide blueprint
→ deliver
```

默认只影响目标页。若用户明确要求新增、删除或重排页面，Planner 必须升级为 deck 级 Plan 或返回
`TARGET_LEVEL_MISMATCH`，不得悄悄扩大范围。

### 7.3 `presentation/deck`

首次物化：

```text
analyze deck
→ apply/confirm design spec
→ materialize pages
→ static verify per page
→ cross-slide verify
→ repair failed pages
→ commit deck presentation
→ deliver
```

已有 presentation 的整份修改：

```text
analyze global intent
→ decide design-level vs page-level changes
→ apply design and/or revise affected pages
→ verify affected pages + cross-slide consistency
→ repair
→ commit
→ deliver
```

现有 Design Director、逐页生成、lint 和有限修复迁入此 Playbook，不保留独立 generate/overview Runner。

### 7.4 `presentation/slide`

Operation 推导：

- 无 HTML：`materialize`
- HTML fresh 且局部指令：`revise`
- Blueprint/design stale 或结构性重做：`rebuild`

流程：

```text
analyze target
→ materialize/revise/rebuild in staging
→ static verify
→ browser verify
→ repair
→ commit target page
→ deliver
```

## 8. Consult 路径

`interaction.intent=consult` 不走写入 PEV：

```text
Context
→ Analyze
→ Respond
```

仍使用 Playbook 的业务视角，但：

- Plan operation 为 `consult`。
- 不创建 staging transaction。
- 不披露写工具。
- 不递增 revision。
- 不产生 artifact commit。

## 9. Step-scoped Tool Disclosure

### 9.1 决策

保留并升级“系统注册很多工具，但每次只向 Agent 披露最小集合”的设计。

新版选择公式：

```text
Base capabilities for WorkSpec
∩ Interaction intent policy
∩ Playbook stage policy
∩ WorkflowStep declared capabilities
∩ Runtime risk policy
= disclosed tool schemas
```

### 9.2 Tool Registry

所有工具注册在 Registry，但模型永远不直接看到 Registry 全集。

```go
type ToolDescriptor struct {
    Name         string
    Capabilities []Capability
    Risk         Risk
    Stages       []Stage
    Mutates      []ArtifactKind
    Tool         Tool
}
```

### 9.3 Capability

建议 canonical capability：

- `read_context`
- `read_blueprint`
- `write_blueprint`
- `read_design_spec`
- `write_design_spec`
- `read_presentation`
- `write_presentation`
- `search_assets`
- `read_assets`
- `mount_assets`
- `render_preview`
- `validate_blueprint`
- `validate_presentation`
- `commit_artifacts`
- `control`

### 9.4 最小披露示例

| Step | 披露工具 |
|---|---|
| Analyze blueprint slide | `read_context_ref` |
| Patch blueprint slide | `patch_slide_blueprint` |
| Materialize slide | `read_context_ref`, `search_assets`, `read_asset`, `write_staged_slide` |
| Verify HTML | `validate_staged_slide`, `render_staged_slide` |
| Repair HTML | `read_staged_slide`, `patch_staged_slide`, `validate_staged_slide` |
| Commit | 不交给 LLM；Runtime 直接调用 transaction |
| Consult | 只读 Context/Artifact 工具 |

### 9.5 双层拒绝

1. 未披露工具不进入 LLM function schema。
2. Executor 在执行前再次校验 Run、Stage、Step、Capability 和 target。

伪造工具名返回 `TOOL_NOT_DISCLOSED`；capability 不符返回 `CAPABILITY_DENIED`。

删除旧 `Gate(scope, mode)`；新生产路径只能使用 `ToolSelector`。

## 10. Execute

### 10.1 StepExecutor

```go
type StepExecutor interface {
    Execute(ctx context.Context, input StepInput) (StepResult, error)
}
```

Step 可由：

- 确定性 Go executor。
- 受限 ReAct executor。
- 逐页子 workflow。

LLM 只负责认知和选择已披露工具；副作用仍由工具执行。

### 10.2 Staging

所有写操作进入：

```text
<project>/.staging/<run-id>/
```

或等价受控临时目录。

要求：

- staging 与 project 同文件系统，支持原子替换。
- 读取正式文件，写入 staged copy。
- Verify 只验证 staged artifact。
- Commit 前正式文件不变。
- failed/canceled 清理 staging。
- 调试模式可保留 manifest，但不保留敏感内容。

### 10.3 ChangeSet

```go
type ChangeSet struct {
    Created  []ArtifactChange
    Updated  []ArtifactChange
    Deleted  []ArtifactChange
    Warnings []Issue
}
```

每项记录 before hash、after hash、artifact ref 和 planned step id。

## 11. Verify

### 11.1 统一接口

```go
type Verifier interface {
    Name() string
    Verify(ctx context.Context, input VerifyInput) VerifyResult
}
```

```go
type VerifyResult struct {
    Passed   bool
    Issues   []Issue
    Evidence []EvidenceRef
    Metrics  map[string]float64
}
```

### 11.2 Issue

```go
type Issue struct {
    Code       string
    Severity   Severity
    Artifact   ArtifactRef
    Location   string
    Evidence   string
    RepairHint string
    Verifier   string
}
```

Severity：

- `info`
- `warning`
- `error`
- `fatal`

Commit policy：

- info/warning 可交付。
- error 必须修复。
- fatal 立即停止。

### 11.3 Blueprint Verifier

- JSON Schema。
- stable id。
- deck order。
- section/subsection 引用。
- role/title/key message/content。
- 目标页与章节关系。
- deck 基本叙事完整性。

### 11.4 Presentation Static Verifier

- HTML 文档结构。
- 16:9 stage。
- tokens/base 引用。
- 禁止危险脚本/能力。
- CSS token。
- alt/accessibility。
- 资源引用。
- Blueprint 关键文本覆盖。

### 11.5 Browser Verifier

使用隔离预览 Runtime 或无同源 sandbox：

- load success。
- console/runtime error。
- text/element overflow。
- viewport。
- broken images/fonts。
- animation protocol。
- timeout。

Browser 验证必须针对 staging，不得把未验证 HTML暴露到主预览。

### 11.6 Cross-slide Verifier

- DesignSpec 一致性。
- 字体、颜色和 signature。
- 相邻页密度与节奏。
- 页面重复。
- materialization source revisions。

确定性规则与可选 LLM/vision reviewer 分开记录；v1 的 Commit 不依赖非确定性视觉评分。

## 12. Repair

```text
Original Plan
+ failing Step
+ staged artifact
+ Issues/Evidence
+ allowed capabilities
→ RepairPlan
```

规则：

- 每个 artifact 默认最多2轮。
- Repair Step 只能操作 Issue 指向的 artifact。
- 只披露修复所需工具。
- 每轮必须重新 Verify。
- Issue 集合无改善时提前熔断。
- Blueprint error 回到 Blueprint repair。
- HTML error 不回写 Blueprint，除非 verifier 明确报告语义源错误。
- 超限后不 Commit，并返回结构化失败。

## 13. Commit

Commit 是 Runtime 确定性阶段，不由 LLM调用：

1. 再次校验 staging hashes。
2. 校验 project lock 与 source revisions。
3. 原子替换文件。
4. 更新数据库 metadata/revision。
5. 写 version snapshot。
6. 清理 staging。
7. emit artifact.committed。

若文件替换成功而数据库失败，Transaction 必须补偿恢复 before snapshot；不得留下文件/数据库半提交。

Revision 只在 Commit 成功后递增。

## 14. 事件协议

直接使用新协议，不兼容旧事件：

- `run.started`
- `context.assembled`
- `plan.created`
- `stage.started`
- `stage.completed`
- `step.started`
- `step.completed`
- `step.failed`
- `tool.called`
- `tool.completed`
- `verification.completed`
- `repair.started`
- `repair.completed`
- `artifact.staged`
- `artifact.committed`
- `needs_input`
- `status.summary`
- `run.completed`
- `run.failed`
- `run.canceled`

删除：

- `thought`
- 泛化 `progress`
- 旧 `plan`
- 旧 `plan.update`
- 旧 `done/error` 模糊结果

禁止持久化或展示原始 chain-of-thought。

事件共同字段：

```json
{
  "run_id": "run-id",
  "workflow_id": "workflow-id",
  "stage": "verify",
  "step_id": "verify-slide-03",
  "attempt": 1,
  "ts": 0
}
```

## 15. 错误与停止

错误码至少包括：

- `CONTEXT_ASSEMBLY_FAILED`
- `PLAN_INVALID`
- `TARGET_LEVEL_MISMATCH`
- `TOOL_NOT_DISCLOSED`
- `CAPABILITY_DENIED`
- `STEP_FAILED`
- `VERIFICATION_FAILED`
- `REPAIR_EXHAUSTED`
- `REVISION_CONFLICT`
- `COMMIT_FAILED`
- `WORKFLOW_CANCELED`
- `WORKFLOW_BUDGET_EXCEEDED`

预算：

- max plan attempts
- max step LLM turns
- max repair rounds
- max tool failures
- max total duration

取消在每个 stage/step 边界检查，清理 staging，不提交 revision。

## 16. Frontend

### 16.1 Timeline

右侧对话栏按以下层级显示：

```text
Run
├── Context ready
├── Plan
├── Stage
│   ├── Step
│   ├── Verification
│   └── Repair
└── Final result
```

默认折叠工具参数和技术证据；错误时允许展开 Issue evidence。

### 16.2 Store

- Run store 使用新事件 reducer。
- plan/state 按 workflow id 隔离。
- history replay 恢复完整 stage/step 状态。
- 切换 thread/project 不串状态。
- artifact.committed 后精确刷新 Blueprint、Design、目标预览和 materialization badge。

### 16.3 用户交互

- 默认 `when_blocked`，不增加每步确认。
- `before_apply` 只在 Plan 已生成、执行前请求一次确认。
- `never` 采用合理假设执行。
- destructive deck 结构变化若 policy 禁止自动执行，返回 needs_input。

## 17. 删除与收敛

完成后删除或迁移其有效逻辑：

- `backend/internal/agent/outline`
- `backend/internal/agent/generate`
- `backend/internal/agent/edit`
- `backend/internal/agent/overview`
- 旧 `agent/assist` Runner 分派
- `model.Kind`
- `model.Scope`
- `model.Mode`
- `harness.Gate`
- 旧 Harness Config 的 Kind/Scope/Mode
- 旧 `PlanPayload/PlanUpdatePayload`
- `agent/slidejson`
- `outline_dirty`
- legacy data migration/backup
- 双 DesignSpec 类型
- 旧事件 reducer/history compatibility
- 旧文档中的公共旧协议

有效的 prompt、tool、lint 和 pipeline 逻辑移动到新领域包，不通过 import 旧包保留。

开发数据库 schema 可直接重写为新 canonical schema；明确要求开发者重建本地 DB，不增加兼容 migration。

## 18. 测试

### 18.1 Workflow Engine

- 合法阶段迁移。
- 非法跳转被拒绝。
- cancellation。
- budget。
- step dependency。
- deterministic fallback plan。
- PlanValidator。
- consult 零写入。

### 18.2 四种 Playbook

每种至少覆盖：

- 正常成功。
- Context 缺失。
- Plan 失败。
- Execute 失败。
- Verify 失败后修复成功。
- 修复超限。
- Commit 失败补偿。
- revision conflict。

### 18.3 Tool Disclosure

- 每个 Step 只暴露期望工具。
- consult 无写工具。
- Verify 无修改正式 artifact 工具。
- Repair 不能修改无关页。
- 未披露工具调用被拒绝。
- capability 与 target 二次校验。

### 18.4 Artifact Transaction

- 正式文件在 Verify 前不变。
- canceled 不提交。
- failed 不提交。
- 成功 Commit revision 只增一次。
- 数据库失败恢复文件。
- staging 清理。

### 18.5 Verifier

- Blueprint schema/reference。
- HTML static lint。
- sandbox render。
- overflow。
- broken resource。
- cross-slide consistency。
- issue severity/repair hint。

### 18.6 Frontend/E2E

- 新事件完整渲染与 replay。
- 四种 Target。
- before_apply。
- needs_input。
- cancellation。
- repair timeline。
- artifact commit 精确刷新。
- 安全预览不回归。

## 19. 实施阶段

### Phase A：Canonical Runtime Domain

- Workflow/Plan/Step/Issue/Event。
- canonical DesignSpec。
- 新 Tool Registry/Selector。
- 新 staging transaction。

### Phase B：Engine 与 Playbook

- Engine。
- 四种 Playbook。
- Planner/PlanValidator。
- Context Engine 接入。

### Phase C：Executor/Verifier/Repair

- 确定性和 ReAct StepExecutor。
- Blueprint/Static/Browser/Cross-slide verifier。
- Repair loop。
- Commit。

### Phase D：前端事件与历史

- 新协议。
- Timeline/reducer/history。
- 精确刷新。

### Phase E：旧结构删除

- 移动有效逻辑。
- 删除旧 Runner/模型/事件/测试。
- 重写 canonical migration 和文档。

## 20. Definition of Done

- 四种 Target 只通过 Workflow Engine 执行。
- ContextPack 是所有 Planner/Executor/Verifier 的业务上下文入口。
- Plan 是结构化、受 Playbook 约束且可验证的对象。
- 所有写入先 staging，Verify 通过后 Commit。
- Repair 使用结构化 Issue，最多有限轮次。
- Tool schema 按 WorkSpec + Intent + Stage + Step 最小披露。
- 未披露工具在执行层也被拒绝。
- consult 无写能力和 revision。
- 原始 thought 不再进入事件或历史。
- 前端正确显示 Stage/Step/Verify/Repair。
- 旧 Runner、Kind/Scope/Mode、Gate、slidejson、outline_dirty、旧事件全部删除。
- canonical DesignSpec 只有一个。
- 不存在兼容 adapter、双读或双写。
- 四种 Playbook、事务、工具披露、Verifier 和 E2E 测试通过。
- Go/前端全量测试、类型检查和 lint 通过。
