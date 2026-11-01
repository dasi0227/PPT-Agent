---
id: ADAPTIVE-EXECUTION-RUNTIME-V1
title: HTML PPT Agent Adaptive Execution Runtime v1
status: implemented
owner: shared
date: 2026-08-01
depends_on:
  - CONTEXT-ENGINEERING-V1
  - ARTIFACT-TARGET-BLUEPRINT-REFACTOR
supersedes:
  - PLAN-EXECUTE-VERIFY-RUNTIME-V1
---

# HTML PPT Agent Adaptive Execution Runtime v1

## 1. 核心决策

顶层 Runtime 是 Adaptive Execution Runtime。Plan–Execute–Verify 不是所有请求的固定
生命周期，而是复杂任务使用的一种执行策略。

```text
WorkSpec
  → Context Engineering
  → ExecutionStrategyRouter
  → Respond | DirectAction | CompactWorkflow | FullPEVWorkflow
  → Execute
  → Verify when writing
  → Commit or Deliver
```

四种 Target 继续是统一业务边界：

- `blueprint/deck`
- `blueprint/slide`
- `presentation/deck`
- `presentation/slide`

项目处于 0→1 阶段，不保留旧 Runner、`Kind/Scope/Mode`、旧 Gate、旧事件、
`slidejson` 投影、`outline_dirty`、旧数据迁移、双读写或 LegacyAdapter。

## 2. 统一不变量

1. 每个请求都先经过 Context Engineering。
2. 每个请求都经过确定性优先的 `ExecutionStrategyRouter`。
3. 模型工具按 Strategy、Stage 和 Step 动态披露，并在执行时二次校验。
4. 所有写入先进入 run-scoped staging transaction。
5. 每个写策略至少执行目标 artifact 对应的 Verify。
6. 只有 Verify 通过才能 Commit；Commit 由 Runtime 确定性执行，不是模型工具。
7. revision 仅在 Commit 成功后递增。
8. failed/canceled 不改变正式 artifact。
9. consult 永远没有写 capability。
10. 不持久化或向前端暴露原始 chain-of-thought。

## 3. 四种执行策略

### 3.1 Respond

用于 `interaction.intent=consult`、解释、建议和状态回顾。

```text
Context → Analyze → Respond
```

- 不创建 `WorkflowPlan`，不发出 `plan.created`。
- 不创建 staging transaction。
- 只披露只读 ContextRef 和 artifact 工具。
- 不执行 Verify、Repair、Commit，不递增 revision。
- 事件序列允许为：
  `run.started → context.assembled → strategy.selected → status.summary → run.completed`。

### 3.2 DirectAction

用于目标明确、单 artifact、单字段或唯一锚点、低风险的局部更新。

```text
Context → Execute → Verify → Commit → Deliver
```

- 不调用 Planner，不发出 `plan.created`。
- Runtime 可持有不可见的单步描述，用于 capability 和 target scope 校验。
- Blueprint 只披露单字段 patch；Presentation 只披露唯一 DOM/文本锚点 patch。
- 写入 staging，执行最小 Verifier，验证失败不得 Commit。
- 不进入 Repair。
- 若锚点不唯一、目标扩大或需要多步，放弃 staging，发出新的
  `strategy.selected(compact_workflow)`，从 CompactWorkflow 重新执行。

### 3.3 CompactWorkflow

用于单页两到三个关联步骤、首次物化、结构重建、资产/组件挂载等中等复杂度任务。

```text
Context → Lightweight Plan → Execute → Verify → Optional Repair → Commit → Deliver
```

- Plan 优先由确定性 Target Playbook 生成。
- 仅当语义决策无法由规则确定时才调用 Planner。
- 只有确实存在多个步骤时发出 `plan.created`。
- Repair 最多两轮，且每轮只处理 Verifier 证据所指向的 artifact。

### 3.4 FullPEVWorkflow

用于 deck、多页、新增/删除/重排、全局叙事或设计、批量修改、跨 artifact 和高风险任务。

```text
Context
  → Full Plan
  → Execute Steps
  → Verify Each Target
  → Cross-artifact / Cross-slide Verify
  → Repair
  → Commit
  → Deliver
```

该策略复用 `WorkflowPlan`、`WorkflowStep`、四种 Target Playbook、`StepExecutor`、
Verifier、Repair、staging transaction、artifact commit 和 Workflow events。

## 4. ExecutionStrategyRouter

```go
type ExecutionStrategy string

const (
    StrategyRespond         ExecutionStrategy = "respond"
    StrategyDirectAction    ExecutionStrategy = "direct_action"
    StrategyCompactWorkflow ExecutionStrategy = "compact_workflow"
    StrategyFullPEV         ExecutionStrategy = "full_pev"
)

type StrategyDecision struct {
    Strategy   ExecutionStrategy
    Reason     string
    Risk       RiskLevel
    Complexity ComplexityLevel
    Signals    []DecisionSignal
}
```

Router 位于 `ContextAssembler` 之后、Workflow Engine 之前。基线规则：

| 信号 | 策略 |
|---|---|
| `intent=consult` | Respond |
| `blueprint/slide` 单字段更新 | DirectAction |
| `presentation/slide` 唯一锚点局部 patch | DirectAction |
| 单页首次物化、stale rebuild、结构或资产工作 | CompactWorkflow |
| `blueprint/deck`、`presentation/deck` | FullPEVWorkflow |
| 新增、删除、重排、多页、批量、全局设计 | FullPEVWorkflow |

路由信号包括 Target artifact/level、intent、指令关键词、materialization state、HTML
是否存在、预计 artifact 数、资产需求、Blueprint/Presentation 联动、跨页验证、风险和指令
明确程度。规则无法判断时可以进行只读轻量分类，但分类结果必须再次通过 Runtime 校验。

## 5. Target Playbook 与 Workflow Engine

四种 Target Playbook 保持独立：

- Blueprint Deck Playbook
- Blueprint Slide Playbook
- Presentation Deck Playbook
- Presentation Slide Playbook

CompactWorkflow 与 FullPEVWorkflow 共用 Workflow Engine。Playbook 声明 operation、
affected artifacts、step dependencies、capabilities、verifiers、成功条件和预算。Plan 中的
artifact scope 是写入上限；新建 deck 的 slide identity 必须在执行前进入 Plan。

Respond 和 DirectAction 绕过 Planner/Plan stage，但不绕过 Context、策略选择、工具策略、
staging/Verify/Commit 不变量。

## 6. Strategy-scoped Tool Disclosure

选择公式：

```text
WorkSpec base capabilities
∩ Interaction intent
∩ Execution Strategy
∩ Current Stage
∩ Current Step capabilities
∩ Runtime risk policy
= disclosed tool schemas
```

- Respond：只读 Context/Artifact 工具。
- DirectAction Execute：目标读取工具和一个目标 patch 工具。
- CompactWorkflow：按 lightweight plan 的当前 step 披露。
- FullPEVWorkflow：按完整 plan 的当前 step 披露。
- Verify 和 Commit 不由 LLM 调用。

工具执行层必须重新校验 disclosed tool、strategy、stage、step capability、risk 和 mutation
target。模型不得通过伪造 tool name 或 artifact id 扩大权限。

## 7. Staging、Verify、Repair 与 Commit

每个写 Run 使用 `.staging/<run_id>`：

- Stage 记录正式 artifact 的 before hash 和 staged after hash。
- Commit 前再次校验 before/after hash，检测并发 revision 冲突。
- 正式文件与数据库 metadata 作为一个逻辑提交；metadata 失败时恢复正式文件。
- Verify 读取 staged 视图，不读取未经验证的正式投影。
- DirectAction 使用最小 Blueprint 或 Presentation static/browser Verify。
- Compact/Full 使用 Playbook Verifier；deck 额外进行 cross-slide/cross-artifact Verify。
- Repair 只用于 Compact/Full，受预算限制且每轮后重新 Verify。

## 8. 事件协议与前端

统一事件包括：

- `run.started`
- `context.assembled`
- `strategy.selected`
- `plan.created`
- `stage.started` / `stage.completed`
- `step.started` / `step.completed` / `step.failed`
- `tool.called` / `tool.completed`
- `artifact.staged` / `artifact.committed`
- `verification.completed`
- `repair.started` / `repair.completed`
- `status.summary`
- `run.completed` / `run.failed` / `run.error` / `run.canceled`

`strategy.selected` 至少包含 strategy、reason、risk、complexity 和 signals。

前端必须把 Plan 视为可选数据：

- Respond 不显示空 PlanCard。
- DirectAction 不制造虚假多阶段 UI。
- Compact/Full 才展示 Plan。
- history replay 与实时事件使用同一 reducer 语义。
- UI 不展示原始 chain-of-thought。

## 9. Definition of Done

- Context Engineering v1 完整实现且测试通过。
- 所有请求进入 ContextAssembler 和 ExecutionStrategyRouter。
- 四种策略均可执行。
- Respond/DirectAction 不强制 Plan。
- Compact/Full 创建结构化 Plan。
- 所有写策略使用 staging + Verify + Commit。
- DirectAction 作用域扩大时放弃 staging 并升级。
- Compact Repair 不超过两轮。
- Full PEV 保留完整多步骤与跨 artifact 验证。
- Tool Disclosure 包含 Strategy 维度并执行二次校验。
- 四种 Target Playbook 可用。
- 前端支持有 Plan 和无 Plan 的 Run、实时与 history replay。
- 旧 Runner、协议、兼容层和旧迁移全部删除。
- 后端、前端、TypeScript、lint 和 build 全量通过。
