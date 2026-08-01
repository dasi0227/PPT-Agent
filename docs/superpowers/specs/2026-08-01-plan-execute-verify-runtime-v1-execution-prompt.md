# Adaptive Execution Runtime v1 一次性执行 Prompt

你是当前仓库的 Agent Runtime 技术负责人。请自主制定内部计划并一次性完成
`docs/superpowers/specs/2026-08-01-adaptive-execution-runtime-v1-design.md`。

本 Prompt 覆盖旧版本中“所有请求统一走 Full Plan–Execute–Verify”的要求。PEV 必须保留，
但只作为复杂任务的 `FullPEVWorkflow` 策略。

## 1. 前置调查

完整阅读 Adaptive Runtime、Context Engineering v1 和 Artifact Target/Blueprint 设计，检查
git status/diff/最近提交和现有计划。先确认 Context Engineering 已完整实现且测试通过。
不要 reset、回滚或丢弃现有代码；在当前实现上增量完成。

## 2. 必须实现的数据流

```text
WorkSpec
→ Context Engineering
→ ExecutionStrategyRouter
→ Respond | DirectAction | CompactWorkflow | FullPEVWorkflow
→ Execute
→ Verify when writing
→ Commit/Deliver
```

确定性 Router 必须优先使用 Target、intent、指令、materialization、HTML 状态、预计影响范围、
资产需求、跨 artifact/跨页需求、风险和明确程度。不得默认调用 LLM 分类。

## 3. 四种策略

- Respond：只读分析与答复；无 Plan、staging、Verify、Repair、Commit、revision。
- DirectAction：单 artifact 局部 patch；无 Planner/`plan.created`；必须 staging、最小 Verify、
  Commit；验证失败不提交；范围扩大时放弃 staging 并升级 Compact。
- CompactWorkflow：确定性 lightweight plan 优先；2–3 个关联步骤；允许最多两轮 Repair。
- FullPEVWorkflow：deck、多页、新增/删除/重排、全局、批量、跨 artifact 和高风险任务；
  复用完整 WorkflowPlan/Step、Playbook、Verifier、Repair、Commit。

只有 Compact/Full 创建结构化 Plan。

## 4. 统一底座

- 所有请求先经过 Context Engineering 和 Strategy Router。
- 所有模型工具执行 Strategy/Stage/Step-scoped Tool Disclosure，执行层二次校验。
- 所有写入先进入 staging，至少一次目标 Verify，通过后才 Commit。
- Commit 不是模型工具；revision 只在 Commit 成功后递增。
- failed/canceled 不修改正式 artifact。
- consult 无写 capability。
- 不暴露或持久化 chain-of-thought。

## 5. Clean-cut

删除旧 Runner、`Kind/Scope/Mode`、旧 Gate、旧事件、`slidejson` 兼容层、
`outline_dirty` 和旧数据迁移。禁止 LegacyAdapter、双读写、deprecated fallback 或旧生产路径。

## 6. 事件与前端

增加 `strategy.selected`。Respond/DirectAction 允许无 `plan.created`；Compact/Full 展示 Plan。
前端实时 reducer 和 history replay 必须支持四种策略，不渲染空 PlanCard，不制造虚假阶段 UI。

## 7. 完成标准

持续执行到 Adaptive Runtime 设计的 Definition of Done 全部满足，并通过：

- 全量 Go test/build。
- 全量前端 test。
- TypeScript typecheck。
- lint。
- production build。

不得只输出方案或等待额外人工确认。
