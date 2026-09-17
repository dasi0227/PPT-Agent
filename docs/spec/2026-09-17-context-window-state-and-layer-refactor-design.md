# 上下文窗口状态、分桶与展示重构设计

- 状态：部分实现（状态与标签简化已完成；六桶协议和正式前端展示待实现）
- 日期：2026-09-17
- 关联：`2026-09-04-context-window-compaction-design.md`

## 1. 变更目标

上下文窗口继续表示最近一次或当前一次完整模型输入的估算快照。运行期间每轮模型调用前更新快照；运行结束后最后一次快照自然成为空闲时展示的最新快照。

本次简化两类不必要的领域概念：

1. 删除上下文明细、持久 transcript 和公开协议中的 `SEED / TRANSCRIPT` 标签。
2. 删除上下文窗口状态中的 `running / warning`，只保留压缩器状态 `idle / compacting`。

## 2. 压缩边界

是否可压缩由数据通路直接决定，不再依赖标签：

- 每轮编译得到的 system prompt、runtime/user prompt、ContextPack 和工具 Schema 不进入 Compactor。
- thread 级持久消息历史作为 `state.messages` 进入 Compactor。

因此删除标签不会改变 token 估算、自动压缩阈值、摘要组织或 transcript 替换行为。

手动压缩后更新空闲快照时，以“上一次完整快照减去压缩前消息快照，再加压缩后消息快照”计算新值。明细通过结构化差集保留；差集无法精确对应时，退化为一条 `last_snapshot` 聚合明细，不重新引入来源层标签。

## 3. 状态与压力

公开快照状态收敛为：

```text
idle | compacting
```

- `idle` 表示压缩器空闲，与 Agent Run 是否正在执行无关。
- `compacting` 表示正在生成摘要并替换持久 transcript。

Agent Run 活动状态继续由 Run Store 管理。运行期间上下文窗口仍通过 `context.window.updated` 持续接收最新快照，但不再切换到 `running`。

接近阈值不是生命周期状态。前端直接从 `ratio` 推导压力提示和橙色视觉反馈，不写入或持久化 `warning` 状态。

手动压缩按钮是否可用继续由 Run、提交、简报、润色和压缩操作的互斥状态决定，不由上下文窗口状态推导。

## 4. 协议切换

- 顶层桶协议固定为六个稳定 key，并按下表顺序返回：

| key | 中文展示名 |
| --- | --- |
| `system_prompt` | 系统提示词 |
| `runtime` | 运行时 |
| `chat_history` | 对话历史 |
| `read_file` | 读文件 |
| `run_command` | 跑命令 |
| `other` | 其它 |

- `ContextWindowDetail` 最终只保留 `name / tokens`；删除 `layer / source`。灰色说明由前端根据稳定的 `name` 映射，不接受后端自由文本。
- transcript JSONL 条目删除 `layer`。
- `ContextWindowSnapshot.status` 仅接受 `idle / compacting`。
- `buckets` 必须始终返回全部六个 key；`details` 必须始终返回全部六组、共十四条固定明细。值为 `0` 时仍返回对应 bucket 和 detail。
- 公开事件、查询接口、持久快照、前端类型与 SSE 校验一次性切换到新 key，不接受或兼容 `read_ppt / user_prompt / uploaded_file` 等旧顶层桶。
- 运行中普通快照发送 `idle`，压缩开始发送 `compacting`，压缩完成后的新快照发送 `idle`。
- 开发期直接切换新协议，不保留旧字段兼容分支。

## 5. Context bucket 设计

### 5.1 系统提示词

系统提示词桶只保留两条聚合明细，展示文案固定为小写：

```text
system prompts
tool definitions
```

- `system prompts`：合并完整 system message，包括 core、mode、playbook 及按请求动态装配的其他 Prompt Module，并计入对应消息包装开销。
- `tool definitions`：合并本轮实际披露给模型的全部工具名称、描述、参数 Schema 及序列化开销。
- Prompt Module 或工具数量不影响明细数量；同类贡献必须在估算结果中求和，不得生成多个同名明细。
- 系统提示词桶的明细数量恒为两条；没有披露工具时 `tool definitions` 仍以 `0` 展示，不得隐藏或拆成逐工具明细。

### 5.2 读文件

读文件桶固定聚合为三条明细，展示文案保持小写：

```text
read_ppt
read_image
read_project
```

- `read_ppt`：合并显式 `read_ppt` 调用的参数与返回内容。
- `read_image`：合并显式 `read_image` 调用的参数与返回内容，以及用户直接上传并实际进入模型上下文的图片内容。
- `read_project`：合并系统自动读取并注入的 PPT 项目内容，包括页面 HTML、设计规范、主题和项目清单等。
- 同一次读取的工具调用与工具结果必须归入同一类；不得把调用参数留在对话历史、只把结果归入读文件。
- 明细按上述类别求和，不按文件名、调用次数或页面拆分；读文件桶的明细数量恒为三条。
- `read_ppt` 与 `read_image` 的工具定义仍属于 `tool definitions`，不属于读文件。
- 通过 `run_command` 读取文件所产生的调用与输出仍属于跑命令，不因输出内容是文件而改归读文件。
- `available_context_refs` 等只描述可用资源的索引不属于文件内容，归入运行时桶的 `runtime resources`。

### 5.3 跑命令

跑命令桶只保留一条聚合明细，展示文案保持小写：

```text
run_command
```

- `run_command`：合并所有 `run_command` 工具调用参数，以及对应的 stdout、stderr、退出状态和其他返回内容。
- 所有命令及其结果统一求和，不按命令次数、命令内容或输出类型拆分。
- 即使命令实际执行 `cat`、`rg` 等文件读取操作，也仍归入 `run_command`，不得根据命令语义改归读文件。
- `run_command` 的工具定义属于 `tool definitions`，不属于跑命令。
- Agent 调用命令前后的自然语言属于对话历史，不属于跑命令。
- Prompt 中用于描述运行配置的 `<run_command>` 包装不代表真实工具执行，属于运行时桶的 `runtime state`，不得计入跑命令。

### 5.4 对话历史

对话历史桶固定聚合为四条明细，展示文案保持小写：

```text
user messages
assistant messages
other tools
context summary
```

- `user messages`：合并所有已经提交的用户指令、追问、补充、steering 和计划反馈。文本框中尚未提交的内容不进入模型上下文，因此不参与统计；系统中不再保留独立的“当前用户提示词”概念。
- `assistant messages`：合并 Agent 已生成并进入模型上下文的自然语言内容，不包含其中的工具调用参数。
- `other tools`：合并除 `read_ppt`、`read_image`、`run_command` 以外的其他工具调用参数及对应返回结果。同一次工具调用与工具结果必须归入同一类。
- `context summary`：合并压缩生成并注入 transcript 的 `<context_summary>`。摘要是历史对话的替代表示，因此属于对话历史；不得因其使用 `user` role 承载而计入 `user messages`。
- 混合消息必须按内容拆分：用户消息中的文字归入 `user messages`，附件图片归入 `read_image`；Assistant 自然语言归入 `assistant messages`，工具调用按工具名称归入对应的工具类别。
- 工具返回结果必须根据 `tool_call_id` 跟随对应工具调用归类，不得仅依据消息 role 统一计入对话历史。
- 压缩后不再统计已被替换的原始消息，只统计 `context summary` 与压缩器保留的近期消息。
- 运行时自动插入且并非用户实际提交的控制消息不属于 `user messages`，归入运行时桶的 `runtime messages`。

### 5.5 运行时

顶层桶展示文案固定为中文“运行时”，并固定聚合为三条明细：

```text
runtime state
runtime resources
runtime messages
```

- `runtime state`：合并当前执行状态，包括 mode、phase、scope、plan、changes、evidence、requirements、work ledger，以及 `<run_command>` 中仅用于描述运行配置的 scope、mode 和 options。
- `runtime resources`：合并运行时提供的资源目录、context ref、页面引用 ID、已加载 skill 和组件引用等运行辅助信息。PPT 项目、页面、设计和主题的实际内容仍属于 `read_project`。
- `runtime messages`：合并 Runtime 自动生成并注入消息列表的控制指令，例如要求 Agent 继续调用工具，或通知 Agent 开始执行已批准计划。这些内容不属于真实用户消息。
- 运行时桶只描述应用为当前执行动态装配的状态、资源和控制信息，不包含稳定系统提示词、真实对话或工具执行结果。

### 5.6 其它

顶层桶展示文案固定为中文“其它”，并只保留一条聚合明细：

```text
other
```

- `other`：承接无法归入其他五个顶层桶的剩余 token，包括 XML/JSON 包装、消息协议开销，以及未来尚未识别的新内容。
- `other` 用于保证分类结果与总 token 守恒，不得作为已知内容的默认长期归宿。
- 实现必须通过测试确保当前所有已知内容进入明确分类，并监控 `other` 的占比；当其明显增长时，应识别新增内容类型并更新分类规则。

最终顶层桶及统一展示顺序固定为：系统提示词、运行时、对话历史、读文件、跑命令、其它。中文展示文案不得替换为 `runtime` 或 `other`。

## 6. 前端展示设计

### 6.1 顺序与结构

弹窗中的顶层桶导航及进度条分段必须使用同一顺序：

```text
系统提示词 → 运行时 → 对话历史 → 读文件 → 跑命令 → 其它
```

- 删除大标题右侧的“空闲 / 压缩中”状态标签；压缩中的反馈只保留在压缩按钮自身。
- 顶层桶即使 token 为 `0` 也必须展示；选中后，其全部固定子类同样必须展示，不得以零值为由省略。
- 顶层桶 token 统一换算为 `x.x k`，保留一位小数并在数字与单位之间保留空格，例如 `15.9 k`、`0.0 k`。
- 子类 token 统一换算为 `x.xx k`，保留两位小数并在数字与单位之间保留空格，例如 `24` tokens 显示为 `0.02 k`，零值显示为 `0.00 k`。
- 明细行使用固定列布局，图标、文案区和 token 数字垂直居中；token 列宽固定并使用 tabular numerals，确保各行数字与左侧文案处于同一水平中心线。

### 6.2 子类说明文案

每条明细继续使用黑色标题和灰色说明，说明文案固定如下：

| 顶层桶 | 子类标题 | 灰色说明 |
| --- | --- | --- |
| 系统提示词 | `system prompts` | 定义 Agent 行为、模式与任务约束 |
| 系统提示词 | `tool definitions` | 本轮可用工具及参数结构 |
| 运行时 | `runtime state` | 当前模式、阶段、计划与执行进度 |
| 运行时 | `runtime resources` | 可用资源、引用与已加载能力 |
| 运行时 | `runtime messages` | 运行时自动注入的控制指令 |
| 对话历史 | `user messages` | 已提交的用户指令与补充 |
| 对话历史 | `assistant messages` | Agent 已生成的自然语言回复 |
| 对话历史 | `other tools` | 其余工具的调用与返回结果 |
| 对话历史 | `context summary` | 压缩历史生成的结构化摘要 |
| 读文件 | `read_ppt` | 通过 read_ppt 读取的页面与项目内容 |
| 读文件 | `read_image` | 读取或上传并送入模型的图片 |
| 读文件 | `read_project` | 每轮自动注入的项目上下文 |
| 跑命令 | `run_command` | 终端命令、输出与执行状态 |
| 其它 | `other` | 协议包装及尚未归类的剩余内容 |

### 6.3 图标选择

图标统一使用 Lucide 线性图标，最终选择及备选如下：

| 顶层桶 | 最终图标 | 其他候选 |
| --- | --- | --- |
| 系统提示词 | `ShieldCheck` | `Shield`、`ScrollText`、`NotebookText` |
| 运行时 | `Activity` | `Cpu`、`Workflow`、`Orbit` |
| 对话历史 | `History` | `MessagesSquare`、`MessageSquareText`、`MessageCircle` |
| 读文件 | `FileText` | `Files`、`FolderOpen`、`BookOpenText` |
| 跑命令 | `Terminal` | `SquareTerminal`、`Code2`、`Command` |
| 其它 | `Ellipsis` | `Boxes`、`CircleHelp`、`Shapes` |

- 明细说明必须作为独立行展示在标题正下方，不得与标题并排。
- 运行时保持蓝色 `#2563EB`；读文件改用琥珀色 `#D97706`，避免两个相邻分桶同时使用蓝色。
- 六个分桶在弹窗内部不得使用重复图标。跨产品区域可以复用相同 Lucide 图标，但必须保持场景语义清晰。
- 为避免与读文件最终使用的 `FileText` 重复，Agent 时间线中的 Kickoff / Handoff 简报卡片统一改用 `BookOpenText`。

交互原型参考：[上下文窗口弹窗 Demo](../demo/2026-09-17-context-window-panel-demo.html)。仓库根目录下的 `tmp/2026-09-17-context-window-panel-prototype.html` 是同一原型的本地临时预览副本，不属于正式前端实现或版本控制内容。

## 7. 实现状态

当前已实现：

- 删除 `SEED / TRANSCRIPT` 标签及 `layer` 协议字段。
- 上下文窗口状态收敛为 `idle / compacting`，普通运行快照继续使用 `idle`。
- 前端不再展示 `running / warning` 状态，压力提示由 `ratio` 推导。
- Kickoff / Handoff 简报卡片图标改为 `BookOpenText`。

后续实现任务：

- 后端估算器、transcript 分类、公开事件与测试切换到六个顶层桶及固定子类。
- 前端类型、SSE 校验、Store、弹窗顺序、配色、格式化、零值展示和固定说明文案同步切换。
- 正式实现应以本 Spec 和关联 Demo 为共同验收依据，不为旧桶协议保留兼容层。
