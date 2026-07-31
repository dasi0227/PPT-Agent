# Plan–Execute–Verify Runtime v1 一次性执行 Prompt

你是当前仓库的 Agent Runtime 技术负责人。请自主制定计划并一次性完成
`docs/superpowers/specs/2026-08-01-plan-execute-verify-runtime-v1-design.md` 的全部代码实现。

这是一项 clean-cut 架构重构：项目处于 0→1，不要求兼容旧内部协议、旧数据或旧事件。

## 1. 前置条件

Context Engineering v1 必须已经完成且测试通过。开始前完整阅读：

1. `docs/superpowers/specs/2026-08-01-plan-execute-verify-runtime-v1-design.md`
2. `docs/superpowers/specs/2026-08-01-context-engineering-v1-design.md`
3. `docs/superpowers/specs/2026-08-01-artifact-target-blueprint-refactor-design.md`
4. 当前 WorkSpec、Context Engine、Blueprint、Run Engine、Harness、Tool、所有 Agent Runner、
   revision/version、事件/history、前端 reducer/timeline 和预览代码
5. 最近至少 15 条 commit
6. 仓库所有适用 `AGENTS.md`

如果 Context v1 未完成，不要伪造实现；先完成其缺失 DoD，再进入本任务。

## 2. 自主执行

- 先只读调查和跑基线。
- 建立内部计划并持续更新。
- 不要只输出方案、伪代码或待办。
- 不要在命名、目录、接口或迁移细节上询问用户。
- 只有同一外部阻塞连续出现至少三次，尝试至少三种安全方案仍无法进展时才请求输入。

## 3. 不兼容原则

明确允许并要求：

- 删除 Kind/Scope/Mode。
- 删除旧 Gate。
- 删除 outline/generate/edit/overview Runner。
- 删除旧事件。
- 删除 slidejson 兼容层。
- 删除 outline_dirty。
- 删除旧数据 migration 和备份逻辑。
- 重写开发数据库 schema，要求开发者重建 DB。
- 更新所有前后端调用、测试和文档。

禁止：

- LegacyAdapter。
- dual read/write。
- 旧字段 deprecated fallback。
- 为了测试通过继续保留旧生产路径。
- 新 Workflow import 旧 Runner 作为永久实现。

可以临时移动旧代码以复用有效逻辑，但最终 DoD 前必须完成删除。

## 4. 必须实现

### 4.1 Canonical Domain

实现并集中定义：

- Stage
- WorkflowStatus
- WorkflowState
- WorkflowPlan
- WorkflowStep
- StepKind/StepStatus
- Operation
- ArtifactRef
- Assumption
- Criterion
- ExecutionBudget
- Issue/Severity/EvidenceRef
- ChangeSet
- VerifyResult

类型不得依赖 Gin/GORM/React。

### 4.2 Workflow Engine

固定生命周期：

```text
normalize
→ context
→ plan
→ execute
→ verify
→ repair
→ commit
→ deliver
```

实现：

- 合法状态迁移。
- stage/step hooks。
- cancellation。
- duration/turn/repair budget。
- structured outcome。
- serializable state。
- terminal uniqueness。

不建设通用 DAG；只支持规格中的固定 Playbook。

### 4.3 Planner

- Playbook 给出骨架。
- Planner 填充 Goal/Assumptions/Affected/Steps/Criteria。
- PlanValidator 检查 target、step kind、依赖、capability、范围。
- LLM 失败有 deterministic fallback。
- `before_apply` 在合法 Plan 生成后、Execute 前请求一次确认。
- `consult` 使用无写入 Plan。

### 4.4 四种 Playbook

完整实现并接入生产：

- blueprint/deck
- blueprint/slide
- presentation/deck
- presentation/slide

迁移现有有效能力：

- Design Director 原则。
- 逐页隔离生成。
- HTML lint。
- cross-page lint。
- asset search/read/mount。
- version/revision。
- HITL、取消、熔断。

但不得保留旧 Runner 作为路由目标。

### 4.5 Step-scoped Tool Disclosure

建立：

- ToolRegistry
- ToolDescriptor
- Capability
- Risk
- ToolSelector
- execution-time policy check

选择必须是以下交集：

```text
WorkSpec base
∩ intent
∩ stage
∩ step capabilities
∩ risk policy
```

模型只收到筛选后的 schemas。Executor 对每次调用二次验证。

测试必须证明：

- consult 无写工具。
- verify 无正式写工具。
- repair 不能改无关 artifact。
- commit 不作为 LLM工具。
- 未披露工具和 capability 越权被拒绝。

### 4.6 Staging Transaction

- 所有写入先到 project 内受控 staging。
- 正式文件在 Verify/Commit 前不变。
- staging artifact 可供静态和浏览器 verifier 读取。
- Commit 原子替换文件并更新 DB/revision/version。
- DB 失败补偿恢复文件。
- failed/canceled 清理 staging。
- revision 只增一次。

不得让现有 patch/write 工具绕过 staging。

### 4.7 Executor

- 确定性 Step 使用 Go executor。
- 认知 Step 可使用受限 ReAct executor。
- ReAct 只是 Step 内部实现，不管理顶层生命周期。
- 每一步只接收 ContextPack、Step 和 staged state。
- 工具结果结构化，包含 summary/artifacts/issues/retryable。

### 4.8 Verifier

实现：

- BlueprintVerifier
- PresentationStaticVerifier
- BrowserVerifier
- CrossSlideVerifier

BrowserVerifier 使用安全隔离环境验证 staging：

- load
- runtime/console error
- overflow
- viewport
- broken image/font
- timeout

确定性规则决定 Commit；可选非确定性 reviewer 不得成为 v1 必需依赖。

### 4.9 Repair

- 从 Issue 生成受限 RepairPlan。
- 每 artifact 默认最多2轮。
- 每轮重新 Verify。
- 无改善提前熔断。
- 超限不 Commit。
- Issue/Evidence 进入结果和 timeline。

### 4.10 新事件

实现规格 §14 的事件并替换旧协议：

- run/context/plan/stage/step/tool/verification/repair/artifact/terminal

删除 raw thought，使用 `status.summary`。

更新：

- Bus
- SSE
- history writer
- history replay
- API types
- frontend reducer/store
- Timeline/Cards

旧 plan/progress/done/error reducer 不保留兼容分支。

### 4.11 前端

- Timeline 分层展示 Stage/Step/Verify/Repair。
- 工具详情默认折叠。
- Issue 可展开 evidence。
- 四种 Target 使用新 Workflow。
- before_apply/needs_input/cancel 正确工作。
- artifact.committed 精确刷新 Blueprint/Design/Preview/Badge。
- history replay 与实时事件一致。

### 4.12 Canonical 清理

完成前全仓搜索并处理：

```text
KindOutline
KindGenerate
KindEdit
KindCommand
ScopeCurrent
ScopePage
ScopeOverview
ScopeRepo
ModeNormal
ModeTalk
ModeAsk
outline_dirty
PlanUpdatePayload
slidejson
```

允许的残留仅限历史设计文档中的说明；生产代码、当前 API 文档和测试不得残留。

统一 DesignSpec，以 Blueprint canonical 类型为基础改成强类型字段；删除 generate 专用 DesignSpec。

## 5. 测试

实现规格 §18 全部测试。至少提供四种 Target 的端到端成功与失败用例：

1. blueprint/deck 更新并提交。
2. blueprint/slide 只影响目标页。
3. presentation/deck 首次物化。
4. presentation/deck 全局修改。
5. presentation/slide materialize。
6. presentation/slide revise。
7. presentation/slide stale 后 rebuild。
8. Verify 失败后 Repair 成功。
9. Repair 两轮失败不提交。
10. cancel 不提交。
11. commit DB 失败补偿。
12. consult 磁盘零变化。
13. tool disclosure 越权。
14. 新事件实时与 replay 一致。

不得通过删除、跳过或放宽断言完成重构。

## 6. 实施顺序

1. 基线调查和全量测试。
2. Workflow domain/events。
3. Tool Registry/Selector。
4. staging transaction。
5. Engine/Planner/PlanValidator。
6. 四种 Playbook。
7. Executors。
8. Verifiers。
9. Repair/Commit/Deliver。
10. Run Service 和 API 接入。
11. 前端新事件。
12. 迁移有效旧逻辑并删除旧结构。
13. 重写 canonical DB schema/docs。
14. 单元/集成/E2E。
15. 全仓残留审计和全量验证。

## 7. Git

建议小提交：

1. workflow domain + events
2. tool disclosure + staging transaction
3. engine + planner + playbooks
4. executor + verifier + repair
5. API + frontend workflow timeline
6. canonical cleanup + migrations + docs + e2e

不提交运行数据库、`.run`、staging、日志、缓存、截图或密钥。检查 `.gitignore`。

不得使用 `git reset --hard`、`git checkout --` 或覆盖用户改动。

## 8. Definition of Done

严格满足设计规格 §20，并运行：

```text
cd backend && gofmt + go test ./...
cd frontend && pnpm test && pnpm tsc && pnpm lint && pnpm build
git diff --check
git status --short
```

最终汇报：

1. 实际完成结果。
2. Workflow/Playbook/Tool Disclosure/Staging/Verify/Repair 架构。
3. 删除的旧结构。
4. 数据库重建说明。
5. 测试命令和结果。
6. 提交列表。
7. 未完成风险；不得把 Task Recovery、完整 Eval 或 Model Router 冒充为已完成。

现在开始只读调查，然后持续执行直至全部 DoD 满足。
