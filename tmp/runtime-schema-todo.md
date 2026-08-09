# Runtime Schema 决策 TODO

本文档记录 Runtime Schema 审查过程中由用户拍板确认的修改点。

## 记录规则

- 只有用户明确确认的修改才进入“已拍板”。
- 尚在讨论的方案进入“待确认”，不得提前实施。
- 实施前应将全部已拍板项整理为正式设计文档并进行一致性审查。

## 已拍板

### 1. `WorkSpec` 重命名为 `RunCommand`

- 当前类型名：`WorkSpec`
- 目标类型名：`RunCommand`
- 语义：表示调用方发给 Runtime 的一次完整运行命令。
- `RunCommand` 继续包含作用域、交互意图、自然语言指令和运行选项。

目标结构：

```json
{
  "scope": {
    "artifact": "ppt",
    "level": "slide",
    "slide_id": "sli_8n4wcp"
  },
  "intent": "execute",
  "instruction": "优化当前页面",
  "options": {
    "language": "zh-CN",
    "range": "9-15"
  }
}
```

### 2. `target` 重命名为 `scope`

- 当前字段路径：`WorkSpec.target`
- 目标字段路径：`RunCommand.scope`
- 语义：声明本次命令允许影响的产物阶段和资源范围。
- 这是外层字段重命名，不是把内部 `level` 改成 `scope`。

### 3. `RunTarget` 重命名为 `RunScope`

- 当前类型名：`RunTarget`
- 目标类型名：`RunScope`
- `RunScope` 包含 `artifact`、`level` 和条件必填的 `slide_id`。
- `level` 保持原名，枚举值保持为 `slide | deck`。
- 先前记录的 `level → scope` 决策撤销，避免形成 `RunScope.Scope` 或 `scope.scope`。

目标结构：

```json
{
  "artifact": "ppt",
  "level": "slide",
  "slide_id": "sli_8n4wcp"
}
```

### 4. RunScope `presentation` 重命名为 `ppt`

- 当前字段路径：`WorkSpec.target.artifact`
- 目标字段路径：`RunCommand.scope.artifact`
- 当前枚举值：`presentation`
- 目标枚举值：`ppt`
- 语义不变：表示以最终 PPT 为目标的执行阶段，允许处理 Spec、HTML、渲染和物化验证。

示例：

```json
{
  "artifact": "ppt",
  "level": "slide",
  "slide_id": "sli_8n4wcp"
}
```

### 5. 删除内部 `Scope` 权限包装类型

- 当前 `Scope` 只包装 `RunTarget`，不产生独立状态，不属于 Runtime Schema。
- 删除 `Scope` 和 `ScopeFromSpec`。
- 权限模块直接接收 `RunScope`。
- 将含义不明确的 `Allows` 拆成显式的读写判断：
  - `AllowsRead(scope RunScope, resource Resource) bool`
  - `AllowsWrite(scope RunScope, resource Resource) bool`
- 权限判断继续属于 Workflow Runtime，不下沉到领域模型，避免 `model` 依赖工具层 `Resource`。

### 6. RunOptions 收敛为 `language` 和 `range`

- 删除 `theme_id`。
- 删除 `desired_slide_count`。
- 新增 `range`，用于表达期望的整份 PPT 页面规模。
- `RunOptions` 最终只保留：
  - `language`
  - `range`

目标结构：

```json
{
  "language": "zh-CN",
  "range": "9-15"
}
```

`language` 枚举：

```text
zh-CN | en-US
```

`language` 规则：

- 空 Deck 首次生成时，可以决定并写入 `outline.language`。
- 已有 Outline 时，默认使用 `outline.language`。
- 如果与现有 `outline.language` 不一致，不得静默切换；必须在 `instruction` 中明确要求翻译或切换语言。

`range` 枚举：

```text
5-8 | 9-15 | 16-25 | 26+
```

`range` 规则：

- 表示期望的整份 PPT 页面数量区间，不是精确页数。
- 各区间不重叠。
- 最终页面数量应落入所选区间。

主题归属：

- 当前主题的唯一权威来源是 `design.theme`。
- 切换主题通过修改 `design.theme` 完成。
- 新增或删除主题属于全局资产仓库管理，不属于单次 Run。
- `RunOptions` 不再保存任何主题字段。

### 7. 删除 RunInteraction，将 RunIntent 扁平化

- 删除单字段包装类型 `RunInteraction`。
- `WorkSpec.interaction.intent` 迁移为 `RunCommand.intent`。
- 当前枚举类型 `InteractionIntent` 重命名为 `RunIntent`。
- `intent` 继续由调用方或 UI 提供，表示授予本次 Run 的交互权限。
- Runtime 不再维护独立的 `ExecutionStrategy`；Agent 通过可选 Plan 自主决定执行编排。

`RunIntent` 枚举：

```text
talk | ask | plan | execute
```

目标结构：

```json
{
  "scope": {
    "artifact": "ppt",
    "level": "slide",
    "slide_id": "sli_8n4wcp"
  },
  "intent": "execute",
  "instruction": "优化当前页面",
  "options": {
    "language": "zh-CN",
    "range": "9-15"
  }
}
```

### 8. 删除 StrategyDecision 和自动策略路由

- 删除 `StrategyDecision`、`DecisionSignal`、`StrategyRouter` 和 `ExecutionStrategy`。
- 删除 `confidence`、任务级 `risk`、`reason`、`signals` 和 `execute → fulfill` 自动升级。
- `talk | ask | plan | execute` 只由 `RunIntent` 表达，不再镜像为 Strategy。
- `execute` 使用单一写能力 Harness Loop。
- `update_plan` 对 execute Run 始终可用，由 Agent 判断是否创建或更新 Plan。
- Plan 不授予新权限，也不扩大 RunScope。
- Agent 一旦创建 Plan，Completion Gate 必须确认全部步骤完成。
- Runtime 继续确定性控制 RunIntent、RunScope、Phase、工具级风险、证据、版本与 Completion Gate。
- checkpoint、SemanticReviewInput 和 StructuredOutcome 不再保存 Strategy。
- 本项已于本轮完成实现。

## 待确认

暂无。

## 实施状态

- 第 1～8 项均已实施。
- 前后端运行时只接受最新 `RunCommand.scope/intent/ppt` 协议，不保留双读或旧事件兼容层。
- SQLite 通过 `0011_run_command_contract.sql` 一次性升级已有 `runs` 表和持久化 JSON；迁移完成后只使用新列。
