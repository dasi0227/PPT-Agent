# Plan 交互模式设计

## 背景

当前系统已经有动态 Plan：复杂执行任务会被动进入 Planning phase，由 Agent 调用 `update_plan` 生成计划，并由前端 `PlanIndicator` 展示进度。

本次需求是把“计划”提升为用户可主动选择的一等交互模式：用户可以只要求 Agent 写计划并回显完整解释，不执行任何 PPT 业务写入。

## 产品语义

公开请求仍使用 `target + interaction`。新增：

```txt
interaction.intent = plan
```

四种用户交互语义如下：

| 用户模式 | interaction | 业务写入 | 可追问 | 主要产出 |
|---|---|---:|---:|---|
| 讨论 | talk | 否 | 否 | 普通只读答复 |
| 盘问 | ask | 否 | 是 | 澄清问题或只读答复 |
| 计划 | plan | 否 | 是 | 精简计划表 + 完整计划解释 |
| 执行 | execute | 是 | 是 | 产物修改与最终答复 |

`plan` 模式不修改项目文件，不创建或提交业务变更，不更新 slide/spec/html revision。

## 策略模型

策略命名改为面向用户模式：

```go
StrategyTalk
StrategyAsk
StrategyPlan
StrategyExecute
```

执行内部是否带计划不再用顶层 Strategy 表达，而由 Runtime 内部状态表示：

```go
ExecuteModeDirect
ExecuteModePlanned
```

`ExecuteMode` 只在 `StrategyExecute` 内生效：

- `direct`：直接进入 executing，可按权限写入。
- `planned`：先进入 planning，只读 + `update_plan`，首个有效计划后进入 executing。

触发 `planned` 的路径：

1. Router 根据整份生成、批量修改、结构调整等高风险信号强制要求计划。
2. Agent 在 direct 执行中主动调用 `update_plan`，Runtime 校验通过后切换为 planned。
3. Completion Gate 多次发现需要协同修复时，Runtime 可把 direct 升级为 planned。

Agent 不能直接修改 `ExecuteMode` 字段，只能通过 `update_plan` 间接请求进入 planned。

## Runtime 行为

### StrategyTalk

- phase: `chat`
- 工具：只读业务工具 + `finish`
- 不允许 `ask_user`
- 不创建 `RunSession`

### StrategyAsk

- phase: `chat`
- 工具：只读业务工具 + `ask_user` + `finish`
- 不创建 `RunSession`

### StrategyPlan

- phase: `planning`
- 工具：只读业务工具 + `update_plan` + `ask_user` + `finish`
- 不创建 `RunSession`
- `finish` 前必须已经有有效 Plan
- 不要求 Plan step 全部 completed
- `finish(message)` 必须输出完整计划解释，不能只输出“已完成”

### StrategyExecute

- 默认 `ExecuteModeDirect`
- Router 可将初始 mode 设为 `ExecuteModePlanned`
- 只有 execute 才创建 `RunSession`
- 只有 execute 可披露写工具
- `ExecuteModePlanned` 下 completion 必须检查 Plan 无 pending/in_progress/failed step

## Plan 内容分层

`update_plan` 是 UI 计划表，必须精简：

- step title 应为短动作短语。
- 不承载完整背景、论证和长解释。
- 每次最多一个 `in_progress`。

`finish(message)` 是面向用户的完整计划说明：

- 解释为什么这么安排。
- 说明每一步影响哪些对象。
- 标明执行时的关键注意事项。
- 可以使用 Markdown 列表，但不暴露内部路径、session、证据账本等技术细节。

## 前端交互

计划入口常驻在 Composer 左侧，与“讨论”“盘问”保持同样按钮样式，不添加额外颜色。

无计划时：

```txt
[ListChecks] 计划
```

点击切换 `composer.intent = plan`。再次点击回到 `execute`。

有计划时：

```txt
[ListChecks] 计划 2 / 5
```

点击打开现有计划浮层，展示执行进度。此时按钮不再切换 intent。已有计划时主动进入计划模式可使用 `/plan ...` 快捷命令。

计划按钮文本与主体文字保持中性色。步骤状态 icon 可继续状态染色。

## 系统提示词调整

Runtime system prompt 需要新增以下约束：

- `talk`、`ask`、`plan` 都是只读。
- 只有 `execute` 通过 active run session 写入。
- `StrategyPlan` 必须先调用 `update_plan` 建立精简计划，再通过 `finish(message)` 输出完整解释。
- `StrategyExecute` 可以 direct 执行，也可以在需要协调时调用 `update_plan` 进入 planned mode。
- `update_plan` 的步骤必须简短；详细解释放入 `finish(message)`。

## 非目标

- 不引入新的 Runner 架构。
- 不把 Plan step 变成独立 Workflow 节点。
- 不改变 `target`、资源模型、工具业务接口。
- 不把 `ExecuteMode` 暴露到前端 UI。
- 不重构整体前后端架构。

## 验收标准

- 后端接受 `interaction.intent=plan` 并持久化。
- plan Run 只生成计划和最终说明，不写项目文件。
- execute Run 仍能 direct 执行；复杂任务仍能先计划后执行。
- 前端计划按钮常驻；无计划时可选择 plan，有计划时打开进度浮层。
- `/plan` 快捷命令发送 `interaction.intent=plan`。
- 前后端相关单元测试、lint/build 通过。
