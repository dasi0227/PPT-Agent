# PPT Agent 产品经理交接说明

> 日期：2026-08-02  
> 角色：后续会话中的高级产品经理 / Agent 产品架构协作者  
> 用途：延续本项目的产品判断、设计约束、文档优先级与开发协作方式  
> 注意：本文件记录已确认决策，不是新的需求提案；实现真实性始终以最新代码和测试为准。

## 1. 一句话产品定义

这是一个专门制作 HTML PPT 的业务 Agent 工作台：中间是接近 Office/WPS PowerPoint 的画布与页面导航，
右侧是 Agent 对话和执行活动区。它不是通用 Agent，也不应停留在 Toy Demo；底层需要具备专业 Harness
Runtime 的上下文、工具、运行、恢复、证据、事件和可观测性结构。

用户要的是：

```text
自然语言协作
        ↓
Agent 设计 PPT 蓝图与 / 或生成、修改 HTML PPT
        ↓
安全预览、可追溯运行、稳定交付
```

## 2. 本交接的使用方法

后续 PM 每次接到需求，应按以下顺序工作：

1. 先读本文件和“权威设计文档”。
2. 再读最新 commit、`git status`、相关代码和测试，确认设计是否已经实现。
3. 区分“已确认产品设计”“当前已实现能力”“尚未实现的未来能力”。
4. 先给产品判断、规格或开发 Prompt；除非用户明确要求，不直接改产品代码。
5. 需要当前行业事实时，只使用官方文档、官方协议或一手源码做网页检索。
6. 对开发 Agent 的 Prompt 要求其自主规划、一次性实施、测试验证，不引入无关兼容层或大重构。

## 3. 文档优先级

从高到低：

1. 本文件，以及用户在当前或后续对话中最新明确确认的决定。
2. 2026-08-02 三份设计稿：
   - [Adaptive ReAct Execution Strategy](../specs/2026-08-02-adaptive-react-execution-strategy-design.md)
   - [PPT Agent Tool Surface](../specs/2026-08-02-ppt-agent-tool-surface-design.md)
   - [Public Events and Timeline](../specs/2026-08-02-agent-public-events-and-timeline-design.md)
3. 2026-08-01 的 Artifact/Target、Context Engineering、前端保留式完善设计。
4. 当前代码、测试和最新 commit，作为“实际实现状态”的依据。
5. `docs/v1`、`docs/v2` 与 7 月设计稿仅作历史背景。它们可能仍包含已废弃的 `kind`、PEV、
   `needs_input` 或旧事件协议，不能覆盖上述新设计。

## 4. 已确认的产品模型

### 4.1 不再使用旧 `kind`

旧的 `outline | generate | edit | command` 混合了产物类型、操作动作、作用域与交互方式，已经废弃。

新的公开 Run 协议由正交维度组成：

```text
target.artifact = blueprint | presentation
target.level    = deck | slide
interaction     = talk | ask | execute
```

四种目标组合：

| artifact | level | 用户语义 |
| --- | --- | --- |
| `blueprint` | `deck` | 整份 PPT 的目标、目录、一级/二级章节、叙事结构和页面顺序 |
| `blueprint` | `slide` | 一页的标题、核心句、内容表达和视觉意图 |
| `presentation` | `deck` | 按蓝图生成整份 HTML PPT，或整体修改视觉系统 |
| `presentation` | `slide` | 生成或修改单页 HTML PPT |

`generate`、`edit`、`materialize`、`rebuild` 是 Runtime 内部如何处理目标的动作，不是用户面对的顶层
类型。`render` 是预览/检查能力，也不是用户任务 kind。

### 4.2 Blueprint 与 Presentation 的边界

Blueprint 是文本和设计意图，不是低层页面元素 AST：

- `deck.json`：整份目标、受众、核心命题、叙事弧、一级 section、二级 subsection、页面顺序。
- 每页 `slide.json`：所属 section/subsection、标题、核心亮点句、内容摘要/要点、visual intent、notes。
- `design-spec.json`：全局色彩、字体、间距、布局节奏、签名视觉、动效规范。
- `index.html`：Presentation 的页面产物。

全局 JSON 与单页 JSON 都存在，但职责不同；不要把全部页面内容塞入一个巨型 deck JSON，也不要把
每一页拆成刚性的 `parts[]` 或坐标 DSL。

“当前页”只是一种前端选择状态。提交后端前必须解析为稳定 `slide_id`，后端不得接受漂移的 `current`
或仅靠 page index 的写入目标。

### 4.3 资产仓库边界

资产仓库不是当前 PPT MVP 的可写目标：

- 本次 MVP 不让 PPT Agent 增删改 Repo 资产。
- Agent 可以在后续通过检索和加载使用现有组件、主题、图片和参考。
- 未来资产管理 Agent 应成为独立 bounded context，不再塞回 PPT Run 的 target scope。

## 5. 交互意图与执行策略

### 5.1 用户选择的 interaction

```text
talk    只读分析、解释和交流
ask     只读设计讨论、brainstorming / grilling 式澄清
execute 授权在目标范围内修改 PPT
```

- `talk` 不是“简单问答”，它可以进行多轮只读 ReAct、读取 PPT、检索参考后再回答。
- `ask` 不是“执行前确认”。它是设计讨论模式，允许 Agent 在确实需要用户选择时调用 `ask_user`。
- `execute` 不要求每次写入前由人确认；Agent 应在授权范围内自主完成。只有关键信息缺失时才提问。

### 5.2 Runtime 选择的 strategy

```text
talk / ask → chat
execute    → simple 或 complex
```

| strategy | 写权限 | Plan | 适用情况 |
| --- | --- | --- | --- |
| `chat` | 无 | 无 | 只读咨询、分析、设计讨论 |
| `simple` | 有 | 无 | 明确、局部、少量连续修改 |
| `complex` | 有 | 有 | 空项目整份生成、多页协调、全局设计、结构性修改 |

`talk/ask/execute` 是用户授权边界；`chat/simple/complex` 是 Runtime 的处理复杂度。两者不能混为一谈。
Router 可以在 execute 内选择或把 simple 单向升级为 complex，但永远不能把 talk/ask 升级为写权限。

## 6. 运行机制的最终心智模型

### 6.1 一个 Run 对应一个连续 ReAct Loop

所有 strategy 都进入同一个连续逻辑 Loop：

```text
当前 Context + Plan + Observation + 已披露工具
        ↓
LLM
        ↓
工具 / 更新计划 / 提问 / finish
        ↓
Observation 写回上下文
        ↓
下一次 LLM 调用
```

Plan 的每个 Step 不是独立 Agent、独立 ReAct Loop 或 Workflow 节点。Plan 是 Complex 长任务的可更新
工作记忆；Agent 可以根据 Observation 调整、拆分、重排或完成步骤。

禁止把 Runtime 又改回固定的 PEV、Playbook、逐 Step 调度 DAG、独立 Verify Stage 或 Repair Stage。
验证和修复应在 ReAct 中按需要发生。

### 6.2 统一的显式结束规则

这是刚刚确认并已经写入 8 月 2 日设计稿的关键决定：

```text
没有显式 finish → 正常情况下继续同一个 ReAct Loop
finish 被拒绝   → Observation 写回，继续同一个 ReAct Loop
finish 被接受   → 成功 return
```

该规则对 `chat/simple/complex` 一致。Chat 不能因普通 assistant 文本自动结束；最终分析也必须调用：

```json
{
  "name": "finish",
  "arguments": {
    "message": "最终交付给用户的回答"
  }
}
```

普通文本但没有 tool call 时，Runtime 保留文本和 provider reasoning 到后续模型上下文，提示 Agent 调用
`finish(message=...)` 或继续调用已披露工具。用户取消、预算耗尽、连续失败、Runtime/LLM/Commit 致命错误
仍可异常终止 Run。

### 6.3 `finish` 与 Completion Gate

`finish` 是 Runtime 控制动作，不是业务工具，也不是“立即提交”。它表达：

> Agent 认为任务可以交付，请 Runtime 检查是否允许结束。

Completion Gate 是显式 finish 的程序化 Exit Guard，不是第二个 LLM、Verifier Agent 或固定 Workflow 节点。
它只在 Agent 尝试退出时检查：

- phase、取消、预算、运行中工具、fatal issue；
- 写任务是否存在 staging transaction、ChangeSet 和 scope 合法性；
- 基线版本是否冲突；
- Complex Plan 是否无阻塞 step；
- schema、引用关系、静态检查、render evidence 是否存在且仍然新鲜。

拒绝时 Runtime 生成内部 Observation：`COMPLETION_GATE_BLOCKED`，包含问题和推荐工具/目标，交回同一 Loop。
接受时才 commit，然后发送 final 事件并结束。

### 6.4 Plan 的完成与任务的完成不同

Complex Plan 的结构性完成条件是：当前 Plan 的所有 step 都是 `completed`。`pending`、`in_progress`、
`failed` 任一存在都会让 Completion Gate 返回 `PLAN_INCOMPLETE`。

但：

```text
Plan 全部 completed
≠
任务已经可以交付
```

Plan 是 Agent 的自我管理状态，Runtime 不能从自然语言 step 标题中证明语义完成；实际事实由 ChangeSet、
Scope、schema、引用与渲染证据验证。不要给每个 Plan Step 绑定固定函数和 verifier，否则会退化为 Workflow。

当前 Plan 更新的结构性约束包括：至少一个 step、ID/标题唯一、状态合法、至多一个 in_progress、已 completed
的 step 不可删除或回退。未完成步骤仍可随着重规划被替换，这是动态 Plan 的刻意设计。

### 6.5 暂存与提交

staging 是 Run 私有的候选版本，commit 是把通过校验的候选成果原子地变为正式 PPT：

```text
正式 PPT → Run 暂存候选 → 多次修改/渲染/修复 → Gate → Commit 或丢弃
```

其价值是整份 PPT 的原子性、失败回滚、渲染前检查、版本冲突控制与运行恢复。它类似数据库事务，不是用户
应该理解或操作的 Workflow 步骤。

- 后端保留 staging/commit。
- 中间预览可以读取 staged view，展示正在制作的候选成果。
- 前端不展示“暂存”“提交”“证据记录”“完成门控”等内部节点。

## 7. 模型可见工具

### 7.1 MVP 业务工具

```text
read_ppt
write_ppt
edit_ppt
search_refs
render_slide
```

语义：

- `read_ppt`：读取当前 PPT 的 global 或指定 slide，优先读取本 Run staged view。
- `write_ppt`：创建或完整替换被授权的 global/slide 内容，写入 staging。
- `edit_ppt`：对已有 global/slide 做可验证的局部 JSON 或锚定文本编辑，写入 staging。
- `search_refs`：从 Context Manifest、记忆、历史和已授权参考中渐进披露信息，不是任意文件搜索。
- `render_slide`：渲染指定 slide 的 staged HTML，产出受控预览和布局证据。

工具参数的资源目标是：

```text
global
slide + stable slide_id
```

这与 API 层的 `artifact + level` 不冲突：前者是模型执行时的资源定位，后者是用户任务的业务目标。

### 7.2 Runtime 控制动作

```text
update_plan
ask_user
finish
```

它们可以使用 function calling 协议，但产品上不属于业务工具：

- `update_plan`：Complex 才披露；更新一份完整 Plan snapshot。
- `ask_user`：阻塞提问并暂停同一个 Run；前端返回回答后在同一 Loop 恢复。
- `finish`：所有 strategy 的唯一成功出口。

前端不能把这三者显示为普通工具卡。

### 7.3 后续资产工具，不属于本次 MVP

```text
find_assets
load_asset
```

它们的定位是寻找并加载可复用资产内容；不允许在 PPT Agent 中修改资产仓库。

## 8. Context Engineering

Context Engineering 是 Runtime 自动组装和控制上下文，不是让 Agent 手动“读摘要”来替代自动注入。

它负责：

- 按当前 Run、目标、权限和预算构建 Context Pack；
- 注入必要的项目、蓝图、设计、历史和引用摘要；
- 保存 Manifest 与受控 ref；
- 按需通过 `search_refs` 做渐进披露；
- 在长运行中压缩历史；
- 防止把无关全文、文件路径或敏感内部实现直接塞给模型。

`search_refs` 是自动 Context Pack 之外的“受控按需读取入口”，不是 Context Engineering 的全部。

## 9. 公共事件与前端时间线

### 9.1 唯一公共事件集合

```text
run.started
run.progress
run.finished

plan.updated

message.reasoning
message.milestone
message.final

tool.started
tool.completed

question.asked
question.answered
```

不要复活或新增旧事件作为前端协议，例如：

```text
context.assembled
strategy.selected
phase.changed
plan.created
target.staged
evidence.recorded
completion.checked
target.committed
status.summary
run.completed / run.failed / run.canceled
needs_input
```

这些应为内部 Trace，或被投影到上述 11 种公共事件。

### 9.2 三类 Message 的职责

| 事件 | 含义 | 生产方式 |
| --- | --- | --- |
| `message.reasoning` | 为什么要进行下一步动作 | Agent 的公开行动说明，不是 Chain of Thought |
| `message.milestone` | 已完成什么有意义的阶段 | Runtime 根据 Plan Diff 确定性生成 |
| `message.final` | 最终给用户的回答 | `finish.message` 在 Gate 接受后的交付 |

禁止把 provider `reasoning_content`、系统提示词、原始 Observation、隐藏思维链投影给用户。

### 9.3 事件和运行顺序

成功路径必须满足：

```text
run.started first
...
message.final
run.finished last
```

每个 tool started/completed 和 question asked/answered 必须成对关联；`run.progress` 是实时可替换状态，
不追加到长期 Timeline 或 Thread History。SSE 继续使用连续 seq、持久化优先和 Last-Event-ID 恢复。

## 10. 前端产品与视觉原则

### 10.1 不变的整体结构

必须保留：

- 顶部 Project Tabs；
- 左侧 Deck Navigator；
- 中间 PPT 预览工作区；
- 右侧 Agent Panel 和 Thread Tabs；
- 左右侧栏独立展开、收起、拖动；
- 底部 Composer。

任何前端改进都是“保留式完善”，不是重新设计整个产品。

### 10.2 右侧 Agent Panel 的信息层级

右侧应像成熟 Agent 的低噪声活动线，而不是聊天气泡、日志卡和开发者面板混合：

- `run.progress`：末尾临时 Live Progress Row，不持久化。
- Plan：独立、扁平、可折叠的 Plan Panel，只显示最新版本。
- Reasoning：无卡片的轻量自然语言行。
- Tool：紧凑可原位更新的活动行；连续同类成功调用可分组。
- Milestone：比 Tool 强、比 Final 弱的阶段性分隔行。
- Question：唯一显著的交互容器，使用 radio/checkbox 和自定义回答。
- Final：普通 Agent 最终回答，不再加重复“执行结果”卡。
- 失败/取消：紧凑 Terminal Notice。

不展示 Context、Strategy、Runtime Phase、staging、evidence、completion gate、原始 tool args/result JSON 或本地路径。

### 10.3 审美与实现习惯

- 保持 Office/WPS 式克制、可靠、专业的工作台气质。
- 使用现有 Tailwind token、现有 accent 蓝色、现有 Lucide 图标库；不引入第二套设计系统或图标库。
- 不把每种事件染成不同颜色；状态色只用于真实 success/warning/danger。
- 不制造 Card Soup、重阴影、大标题、装饰性渐变或落地页风格。
- 工具成功后回归中性，而不是满屏绿色。
- Question 是协作而非告警，不能用大面积黄色警告卡。
- 移除输入框常驻 focus border，只保留键盘 `focus-visible` 可访问性提示。
- 动效克制：短淡入、状态原位变化、spinner；禁止打字机、弹跳气泡和抢眼动画。
- 自动滚动只在用户接近底部时跟随；用户向上阅读时显示“回到最新”，不能强制抢滚动位置。

### 10.4 前端设计工作方式

用户要求审视或改进 UI 时：

1. 先检查现有页面与组件，保持上述结构。
2. 如果需要提出明显视觉方向，先生成临时 HTML 预览供用户拍板；未经确认不直接做大视觉改造。
3. 使用 `frontend-design` 与 `design-taste-frontend` 的克制、层级、可访问性原则，但不要套用其落地页范式。
4. 输出开发 Prompt 时，把“保留三栏、侧栏伸缩、顶部 Tab”写成不可违反约束。

## 11. 当前实现状态与工作区注意事项

截至本文件编写时：

- `6decbd6 Refactor application architecture and streamline implementation` 已大规模落地新的 ReAct Runtime、
  11 个公共事件、工具面、staging、render worker 与新的前端 Timeline 结构。
- `38986ed Require explicit finish tool calls for all runtime completions` 已把“所有 strategy 显式 finish”作为
  代码与测试目标提交。
- 当前工作区仍存在未提交的前端修改，主要涉及 Composer、Target Selector、Agent Activity Rows、Deck
  Navigator 与 Preview Workspace。它们属于进行中的用户工作，后续 Agent 必须先 `git status` 和 diff，
  不得 reset、checkout 或覆盖。
- 当前启动脚本为 [restart.sh](../../../restart.sh)，默认会询问是否清空数据；没有用户明确授权时禁止使用
  `--reset`。

任何“已完成”判断都必须重新运行相关后端、前端和集成测试，而不是仅根据 commit 名称或文档判断。

## 12. MVP 边界与后续优先级

当前 MVP 已确认不必优先实现：

- 语义 Planner 模型注入；
- BrowserVerifier 升级为更复杂的智能判断；
- Repo 资产的增删改 Agent；
- 通用 shell / 任意文件系统工具；
- 多 Agent 并行页面生成；
- 打字机 `message.delta`；
- 完整 Trace 开发者控制台。

这些可以在 Runtime 稳定、质量评测、上下文工程和恢复能力需要进一步提升时再讨论，但不能为了“看起来像
专业 Agent”提前膨胀成 Workflow Engine。

## 13. 后续 PM 的设计习惯与判断标准

### 13.1 先分层，再命名

每个新概念先判断它属于哪一层：

```text
用户业务对象      → blueprint / presentation / deck / slide
用户授权与协作    → talk / ask / execute
Runtime 执行复杂度 → chat / simple / complex
模型操作能力      → business tools / control actions
内部可靠性机制    → context / staging / evidence / gate / trace
用户可见反馈      → public events / timeline components
```

如果一个名称同时表达两层含义，例如旧 `kind=edit`，优先拆分而不是继续堆字段。

### 13.2 不把内部机制产品化

以下能力可以很专业，但默认应隐藏：

- Context Assembly；
- Strategy Router；
- Phase；
- staging / commit；
- evidence ledger；
- completion gate；
- provider reasoning；
- 原始工具参数与输出。

用户应看到“正在读取第 3 页”“第 6 页渲染通过”“需要选择视觉方向”，而不是 Runtime 内部术语。

### 13.3 不把 Plan 做成 Workflow

Plan 是 Agent 的工作记忆；它不应拥有：

- 固定执行函数；
- 独立工具权限；
- per-step 子 Agent；
- DAG 依赖调度；
- 每步独立 Verifier；
- 硬编码 Playbook。

质量保证来自工具本身的确定性检查、证据新鲜度与 finish 时的 Completion Gate，而不是每一步强制经过
Verify/Repair 节点。

### 13.4 权限和自主性的平衡

- `talk/ask` 绝不写 PPT。
- `execute` 在当前授权 scope 内可自主行动，不要默认增加“每次执行前确认”。
- `ask_user` 仅在阻塞且用户确实需要作选择时使用。
- 当前没有人为确认流程的需求，不要把 Ask 误实现成“确认执行”。

### 13.5 设计和 Prompt 的交付习惯

- 用户要“开发 Prompt”时，直接在对话输出可复制 Prompt；除非用户明确要求，不把 Prompt 只藏进文件。
- 用户要“设计文件”时，写入 `docs/superpowers/specs/` 或明确要求的位置，并在回复中给绝对路径链接。
- Prompt 要写清：阅读哪些权威文件、非目标、不可破坏约束、测试要求、一次性自主执行、保留 dirty worktree。
- 不要在产品设计阶段擅自实施；用户要求分析、审查或设计时，先给证据和规格。
- 代码已更新时，优先做验收、接口适配和风险判断，不重复设计已经落地的内容。

## 14. 交接验收清单

新的 PM 会话在开始实际工作前，应能够准确复述：

- 产品是 HTML PPT 业务 Agent，不是通用 Agent；
- 公开目标模型是 artifact + level + interaction，而不是 kind；
- `chat/simple/complex` 是 strategy，不是用户模式；
- 所有 strategy 都是单一连续 ReAct Loop，Complex 只是多了一份动态 Plan；
- 所有正常成功出口都必须显式 `finish`；
- Completion Gate 是程序化 exit guard，不是 LLM verifier 或固定 Workflow；
- staging 内部保留、前端隐藏；
- 模型业务工具只有 5 个，控制动作 3 个；
- 公共事件只有 11 种，Trace 与 UI 分层；
- 前端必须保留三栏与顶部 Tab，只做克制完善；
- 当前 Repo/资产写入、多 Agent、通用 shell、打字机流和复杂智能 verifier 不属于本 MVP。

如果上述任一条不确定，先回到第 3 节列出的权威文档和最新代码核对，再对用户给出建议。
