# PPT Agent Tool Surface 设计

> 日期：2026-08-02  
> 状态：设计确认稿  
> 范围：面向 HTML PPT 业务 Agent 的模型可见工具、Runtime 控制动作与后续资产工具  
> 文档性质：仅定义产品与架构语义，不包含开发步骤、代码改造清单、排期或兼容方案

## 1. 背景

当前 Runtime 已具备 Context Engineering、执行策略路由、动态工具披露、staging、验证能力与
事件投影等基础结构，但模型可见工具仍带有较强的内部实现色彩，例如
`read_staged_artifact`、`write_staged_slide_blueprint`、`write_staged_presentation`。这些名称
把 staging、artifact kind 和内部数据分层直接暴露给模型，使模型必须理解 Runtime 的实现细节，
也扩大了工具选择空间。

本设计将模型的操作界面收敛为 PPT 领域语言。模型只需要理解“读、完整写入、局部编辑、检索参考、
渲染预览”，具体文件路径、staging 位置、版本、事务、验证和提交继续由 Runtime 管理。

## 2. 设计目标

- 用尽量少且稳定的工具覆盖 Blueprint、全局设计与单页 HTML 的主要读写场景。
- 将操作类型放在工具名中，将全局或单页目标放在严格参数中。
- 保持全局 PPT 模型与单页模型在数据层分离，不把二者合成一个巨型 JSON。
- 区分 PPT 业务工具、Runtime 控制动作和后续资产仓库工具。
- 延续当前能力、阶段、策略、风险和目标范围共同决定工具披露的机制。
- 保留 staging、版本控制、验证和事务能力，但不让模型直接操作这些内部概念。
- 支持 talk、ask、Simple、Complex 及 Completion Gate 拒绝后的继续执行使用不同的最小工具集合。

## 3. 非目标

- 本设计不提供通用 Shell、代码仓库编辑或任意文件系统能力。
- 本设计不包含资产仓库的增删改。
- 本设计不定义 PDF、PPTX 等导出能力。
- 本设计不把计划执行包装成模型可递归调用的工具。
- 本设计不考虑旧工具名称或旧参数协议的兼容。
- 本设计不规定具体 Go 类型、数据库迁移、接口路由或前端组件实现。

## 4. 总体分层

### 4.1 MVP 业务工具

```text
PPT
├── read_ppt
├── write_ppt
└── edit_ppt

Reference
└── search_refs

Preview
└── render_slide
```

### 4.2 Runtime 控制动作

```text
Runtime Control
├── update_plan
├── ask_user
└── finish
```

控制动作可以使用与 Function Calling 工具相同的技术协议，但在产品和权限模型中不属于 PPT
业务工具。前端也不应把它们展示为普通“工具执行卡片”。

`update_plan` 只在 Complex 策略中披露，用于同一个 ReAct Loop 内创建、修订和更新计划状态；
`ask_user` 与 `finish` 根据 interaction 和当前运行状态披露。

### 4.3 后续资产仓库工具

```text
Asset Repository
├── find_assets
└── load_asset
```

资产仓库不属于本次 MVP，只在本设计中保留未来边界。

## 5. PPT 资源模型

`ppt` 是模型操作的领域总称，内部仍然包含两类不同目标。

### 5.1 Global

Global 对应整份演示文稿的全局 JSON，而不是所有页面内容的集合，典型信息包括：

- 演示文稿标题与说明；
- 画布比例；
- 章节与页面顺序；
- 全局设计语言；
- 颜色、字体、间距等设计约束；
- 公共样式与全局引用；
- 全局 revision 和内容状态。

### 5.2 Slide

Slide 对应一张稳定 `slide_id` 标识的页面，包含：

- 页面 JSON 模型；
- 所属 section/subsection；
- 标题、核心表达和内容结构；
- 页面级视觉结构；
- HTML；
- 来源 revision 与页面状态。

### 5.3 目标判别

所有 PPT 工具通过严格判别联合选择目标：

```json
{
  "type": "global"
}
```

或：

```json
{
  "type": "slide",
  "slide_id": "slide-03"
}
```

规则：

- `type=global` 时禁止携带 `slide_id`。
- `type=slide` 时必须携带有效 `slide_id`。
- 模型不能传入文件路径、staging 路径或数据库主键。
- Runtime 必须校验目标是否位于当前 WorkSpec 和 Run Target Scope 声明的可读或可写范围内。
- `global` 不代表“全部页面”，因此不能借此绕过逐页 scope。

## 6. `read_ppt`

### 6.1 目的

读取当前 PPT 的全局模型或指定页面。读取时优先返回当前 Run 的 staged 版本；不存在 staged
版本时回退到已提交版本。模型不感知实际存储位置。

### 6.2 概念参数

读取全局信息：

```json
{
  "target": {
    "type": "global"
  },
  "include": ["model"]
}
```

读取单页模型和 HTML：

```json
{
  "target": {
    "type": "slide",
    "slide_id": "slide-03"
  },
  "include": ["model", "html"]
}
```

`include` 只能从目标允许的内容类型中选择。Global 不允许请求 `html`。

### 6.3 返回语义

返回目标标识、revision、内容、内容状态和来源视图。大段 HTML 可以根据 Context Budget 返回摘要
与受控引用，但不能返回可用于读取任意路径的地址。

### 6.4 边界

- 只读，不修改 revision。
- 不能读取当前 Run/Project 之外的目标。
- 不能通过 `global` 一次返回全部页面 HTML。
- 读取大型内容仍受 Context Budget 限制。

## 7. `write_ppt`

### 7.1 目的

创建一个不存在的 Global/Slide，或完整替换当前 Run 被授权写入的目标内容。它适合从零创建、
页面大幅重建和完整设计语言替换。

### 7.2 概念参数

写入 Global：

```json
{
  "target": {
    "type": "global"
  },
  "content": {
    "model": {}
  }
}
```

写入 Slide：

```json
{
  "target": {
    "type": "slide",
    "slide_id": "slide-03"
  },
  "content": {
    "model": {},
    "html": "<section>...</section>"
  }
}
```

### 7.3 语义

- “write”表示提交一个完整的新版本，不表示绕过 staging 直接落正式文件。
- Runtime 根据当前 WorkSpec、Strategy、Phase、capability 和目标状态判断是 create 还是 replace。
- Blueprint 任务只能写模型字段；Presentation 任务才能写 HTML。
- 单页完整物化通常同时提交页面模型和 HTML，避免二者不一致。
- 整份演示文稿生成由 Runtime 按 Plan 驱动多个目标明确的写入动作，不使用一个无限大的
  `write_ppt(global)` 承载所有页面 HTML。

### 7.4 安全边界

- 只能写当前 Run Target Scope 已声明的目标。
- 写入结果进入 staging，并由 Runtime 执行 schema、静态或浏览器级验证。
- 模型不能控制版本号、磁盘路径、提交位置和事务边界。
- `finish` 不能使未验证的写入直接提交。

## 8. `edit_ppt`

### 8.1 目的

对已经存在的 Global/Slide 进行范围明确的局部编辑。它适合改标题、文案、主题 token、页面结构
片段和 HTML/CSS 局部内容。

### 8.2 编辑类型

JSON 模型使用结构化路径操作：

```json
{
  "target": {
    "type": "global"
  },
  "edits": [
    {
      "type": "json_edit",
      "op": "replace",
      "path": "/theme/primary_color",
      "value": "#2457E6"
    }
  ]
}
```

HTML 使用锚定文本替换：

```json
{
  "target": {
    "type": "slide",
    "slide_id": "slide-03"
  },
  "edits": [
    {
      "type": "text_edit",
      "field": "html",
      "old_text": "<h1>旧标题</h1>",
      "new_text": "<h1>新标题</h1>"
    }
  ]
}
```

### 8.3 语义

- JSON 编辑只能操作 schema 允许的路径。
- 文本编辑必须有唯一、可验证的锚点；匹配为零或多处时失败，而不是猜测。
- 多个 edits 在同一目标内原子应用，任一失败则整组不进入 staged 成果。
- 大范围重构应使用 `write_ppt`，而不是堆积大量脆弱的局部编辑。
- Runtime 对修改后的完整目标重新校验，不能只校验局部片段。

## 9. `search_refs`

### 9.1 目的

从当前 Run 已授权的参考资料、项目记忆和 Context Manifest 中检索与问题相关的内容。它是
Context Engineering 的渐进披露入口，不是文件搜索工具。

### 9.2 概念参数

```json
{
  "query": "客户对品牌色和字体提出了哪些要求？",
  "kinds": ["reference", "memory", "history"],
  "limit": 5
}
```

### 9.3 返回语义

每条结果包含：

- 不透明 `ref_id`；
- 来源类型与标题；
- 与 query 相关的内容片段；
- revision/hash 等追溯信息；
- 是否还存在更详细层级。

若结果内容较小，可以直接返回相关正文；若内容较大，只返回片段和受控引用。模型不得通过
`ref_id` 推导磁盘路径。

### 9.4 与 `read_ppt` 的区别

- `read_ppt` 读取当前 PPT 的确定目标。
- `search_refs` 根据语义问题检索外部参考、记忆和被压缩的上下文。
- PPT 的正式状态不能通过 `search_refs` 修改，也不能以参考资料覆盖当前项目事实。

## 10. `render_slide`

### 10.1 目的

在受控浏览器环境中渲染指定页面，形成“写入—观察—修正”的视觉反馈闭环。

### 10.2 概念参数

```json
{
  "slide_id": "slide-03"
}
```

画布尺寸、设备比例、字体等待策略、资源白名单、超时和动画稳定策略由 Runtime 固定，模型不能
任意控制浏览器启动参数。

### 10.3 返回语义

- `screenshot_ref`：受控截图引用；
- viewport 与实际内容尺寸；
- 水平/垂直溢出；
- 元素越界或裁切；
- 控制台错误；
- 资源加载失败；
- 字体加载状态；
- 渲染耗时与诊断摘要。

### 10.4 边界

- 默认渲染当前 staged 页面，必要时回退到 committed 页面。
- 工具只产生预览和诊断，不修改 PPT。
- HTML 在隔离 Origin/Browser Context 中执行，默认禁止任意外部网络访问。
- `render_slide` 不能被替代成模型可自由调用的浏览器 Shell。

## 11. `finish`

### 11.1 目的

显式结束当前 Agent Loop，并提交当前循环的结构化结果与面向用户的文本。

```json
{
  "status": "completed",
  "message": "已完成第 3 页的重新设计，并统一了标题层级。",
  "summary": "更新 slide-03 的页面模型与 HTML"
}
```

### 11.2 关键语义

- `finish` 提交当前 Agent Loop 的完成候选，不直接判定整个 Run 成功。
- Runtime 先执行 Completion Gate；拒绝时把问题作为 observation 返回同一个 ReAct Loop。
- 整个 Run 是否完成，由 Runtime 根据 Plan、ChangeSet、evidence 与事务状态决定。
- 最终用户文本只在 Run 真正进入完成状态时交付。
- `finish` 的 tool call/tool result 默认不作为普通工具卡展示，避免与最终消息重复。
- 如果模型调用 `finish` 但没有满足当前目标的产出或证据条件，Runtime 应拒绝完成并让同一个
  ReAct Loop 继续处理。

## 12. `ask_user`

### 12.1 目的

在无法通过当前上下文安全做出关键决策时，向用户提出结构化问题并暂停 Run。

```json
{
  "question": "这份演示文稿更希望采用哪种叙事方式？",
  "options": [
    {
      "id": "problem_solution",
      "label": "问题—解决方案",
      "description": "先呈现业务痛点，再逐步给出方案"
    },
    {
      "id": "conclusion_first",
      "label": "结论先行",
      "description": "首先展示核心结论，再展开论据"
    }
  ],
  "multiple": false,
  "allow_custom": true
}
```

### 12.2 关键语义

- 调用后产生 `needs_input`，Run 进入可恢复的等待状态，而不是完成或失败。
- 用户回答必须和 run/question id 绑定，恢复后回到原 Strategy、Phase 和同一个 ReAct Loop。
- `options` 支持单选或多选；`allow_custom=true` 时允许自由文本。
- ask 模式允许主动使用；执行模式只在缺失信息会实质改变结果、且无法安全假设时使用。
- 普通视觉偏好或非关键细节由 Agent 根据项目上下文自主决策，不能频繁打断用户。

## 13. `update_plan` 与 Plan 所有权

### 13.1 目的

`update_plan` 是 Complex 策略下的 Runtime 控制工具。它让 Agent 在同一个 ReAct Loop 中创建计划、
更新步骤状态，并根据执行中新获得的事实调整后续计划。Plan 是结构化工作记忆和进度投影，不是
Runtime 强制调度的 Workflow DAG。

### 13.2 概念参数

```json
{
  "explanation": "已完成全局设计，开始逐页生成",
  "steps": [
    {
      "id": "design",
      "title": "确定全局设计语言",
      "status": "completed"
    },
    {
      "id": "generate",
      "title": "逐页生成 HTML",
      "status": "in_progress"
    },
    {
      "id": "review",
      "title": "检查视觉一致性",
      "status": "pending"
    }
  ]
}
```

`update_plan` 每次提交当前计划的完整快照，而不是执行某个步骤。首次有效调用创建 Plan；后续调用
更新同一个 Plan。

### 13.3 Planning 到 Executing

Complex Run 创建后，Runtime 将同一个 ReAct Loop 置为 `phase=planning`，只披露只读工具和
`update_plan`。首次有效的 `update_plan` 成功后，Runtime 自动切换为 `phase=executing`，下一轮
开始披露当前任务允许的写工具。这里没有新的 Agent、没有新的 ReAct Loop，也不需要用户批准计划。

### 13.4 执行步骤

不提供 `execute_plan_step`。Agent 读取 Plan 后，直接调用 `read_ppt/write_ppt/edit_ppt/render_slide`
完成当前工作；Plan Step 只帮助 Agent 保持方向、展示进度和支持恢复。

一个 Complex Run 只有一个主 ReAct Loop。步骤切换不会重新装配一套 Agent，也不会为每个 Step
建立独立输入、输出和 Verify/Repair 节点。

### 13.5 步骤状态

允许状态：

```text
pending → in_progress → completed
                      ↘ failed
```

Agent 通过 `update_plan` 提议状态变化，Runtime 负责校验和持久化：

- Step ID 唯一且在首次出现后保持稳定。
- 同一时刻最多一个 `in_progress`。
- `completed` Step 默认不能被删除或静默改回 `pending`。
- 新事实确实推翻已完成结论时，必须在 `explanation` 中说明，并保留修订记录。
- Agent 不能仅靠更新状态制造业务成果；实际写入和诊断证据仍来自 PPT 工具。

### 13.6 调整计划

不单独提供 `replan` 或 `revise_plan`。执行中发现范围、顺序或实现方式变化时，Agent继续调用
`update_plan` 修改尚未完成的步骤。Runtime 发出 `plan.updated`，但不启动另一个 Planner 节点。

### 13.7 已完成计划

不提供 `clear_completed_plan`。已完成步骤是运行审计、恢复、前端进度和问题排查依据。前端可以
折叠或隐藏，但持久化状态不能删除。

Agent 调用 `finish` 时，Completion Gate 检查 Complex Plan 是否仍有关键的 `pending` 或
`in_progress` Step。未完成时把问题作为 observation 返回同一个 ReAct Loop，而不是进入独立
Verify/Repair Workflow。

## 14. 资产仓库的后续设计

### 14.1 `find_assets`

根据 query、资产类型、标签和适用场景搜索仓库，仅返回轻量元信息：

- `asset_id`；
- 名称、类型和摘要；
- 标签和适用场景；
- 预览引用；
- 大致依赖与复杂度。

### 14.2 `load_asset`

根据一个 `asset_id` 加载资产源码、用法、参数、依赖、约束和示例到当前 Agent 上下文。

之所以使用 `load` 而不是 `read`，是为了表达“通过渐进披露把已选资产加载给 Agent 使用”，并与
`read_ppt` 区分。默认一次加载一个资产，避免多个大型 HTML/CSS 同时占满上下文。

`load_asset` 是只读动作，不写入当前 PPT。Agent 获得资产内容后，通过 `write_ppt` 或
`edit_ppt` 将其应用到页面。

### 14.3 保留 `mount_asset` 的严格语义

`mount_asset` 只在未来确实发生以下副作用时使用：

- 复制或登记资产到项目；
- 注入公共 CSS 或依赖；
- 建立资源路径映射；
- 固化资产版本引用；
- 使资产在后续运行中持续挂载。

如果只是读取内容，不使用 `mount` 命名。这样为未来“加载”和“正式绑定”保留清晰边界。

## 15. 为什么不提供 `run_command`

MVP 不向模型提供 `run_command`，原因包括：

- 它可以绕过 `write_ppt/edit_ppt` 直接修改文件；
- 它可以绕过 staging、事务、版本和目标 scope；
- 它会把大量不可预测行为压缩为一个难以审计的命令；
- 它扩大到进程、网络、文件系统和删除等非 PPT 权限；
- 它会削弱动态工具披露和能力控制的价值；
- 它使业务 Agent 退化成权限过大的通用 Coding Agent。

浏览器渲染、静态检查、格式导出等操作即使底层使用命令行，也应由 Runtime 的确定性实现封装成
领域工具，而不是接受模型生成的任意 Shell。

未来如开发独立的资产开发或代码维护 Agent，可以在另一套身份、沙箱和策略中评估受限命令工具，
不与 PPT 制作 Agent 共用权限。

## 16. 动态工具披露

工具注册表可以包含全部工具，但每个模型调用只获得当前阶段必需的 schema：

```text
WorkSpec 基础能力
∩ Interaction Intent
∩ Execution Strategy
∩ Current Phase
∩ Current Target Scope
∩ Runtime Risk Policy
= 本轮披露工具
```

建议披露矩阵：

| 场景 | 模型可见工具 |
|---|---|
| talk / 只读分析 | `read_ppt`、`search_refs`、按需 `render_slide`、`finish` |
| ask / 设计讨论 | `read_ppt`、`search_refs`、按需 `render_slide`、`ask_user`、`finish` |
| Simple | 当前目标允许的业务工具、阻塞时按需 `ask_user`、`finish` |
| Complex Planning | `read_ppt`、`search_refs`、按需 `render_slide`、`update_plan`、按需 `ask_user` |
| Complex Executing | 当前 scope 允许的业务工具、`update_plan`、阻塞时按需 `ask_user`、`finish` |
| Completion Gate 拒绝后 | 与当前执行 phase 相同；问题作为 observation 回到同一个 ReAct Loop |
| Commit | 不向 LLM 暴露，由 Runtime 确定性执行 |

工具 schema 不只在披露时过滤，真正执行时还必须再次校验 intent、strategy、phase、capability、
risk 和 target。

## 17. 统一返回与错误语义

模型可见工具结果使用统一外壳：

```json
{
  "ok": true,
  "summary": "slide-03 staged",
  "data": {},
  "changed_targets": [],
  "issues": [],
  "retryable": false
}
```

设计要求：

- `summary` 是给模型继续推理的简洁 observation。
- `data` 承载读取或诊断结果。
- `changed_targets` 只描述本次改变的领域目标，不要求模型理解内部 artifact path。
- `issues` 使用稳定错误码、严重度、目标和可操作说明。
- 可重试错误与权限/协议错误明确区分。
- 失败结果不得隐式产生部分正式写入。

核心错误类别：

| 类别 | 含义 |
|---|---|
| `TOOL_NOT_DISCLOSED` | 当前轮次没有披露该工具 |
| `CAPABILITY_DENIED` | 当前 intent/strategy/phase 不允许该能力 |
| `TARGET_OUT_OF_SCOPE` | 目标不在当前 Run Target Scope |
| `TARGET_NOT_FOUND` | 指定 Global/Slide 不存在 |
| `TARGET_ALREADY_EXISTS` | 创建语义下目标已经存在 |
| `EDIT_ANCHOR_NOT_FOUND` | 文本编辑锚点不存在 |
| `EDIT_ANCHOR_AMBIGUOUS` | 文本编辑锚点匹配多处 |
| `MODEL_INVALID` | Global/Slide JSON 不符合领域 schema |
| `REVISION_CONFLICT` | 基线 revision 已变化 |
| `CONTEXT_BUDGET_EXCEEDED` | 参考资料展开超过剩余上下文预算 |
| `RENDER_FAILED` | 浏览器启动、加载或截图失败 |
| `STAGING_REQUIRED` | 写操作缺少受控 staging 事务 |

## 18. 最终工具清单

### 18.1 本次 MVP

```text
read_ppt
write_ppt
edit_ppt
search_refs
render_slide
update_plan
finish
ask_user
```

其中前五个是业务工具，后三个是 Runtime 控制动作。它们不会在同一轮全部披露。

### 18.2 后续资产仓库

```text
find_assets
load_asset
```

### 18.3 明确不提供

```text
run_command
execute_plan_step
clear_completed_plan
submit_plan
enter_plan
exit_plan
```

仅加载资产内容时也不提供 `mount_asset`；该名称保留给未来真正建立项目绑定并产生持久副作用的能力。

## 19. 设计结论

新的工具面向模型表达 PPT 业务意图，Runtime 继续掌握实际执行边界：

```text
模型决定：读什么、写什么、如何局部修改、查什么参考、看哪页预览
Runtime 决定：能否调用、可操作哪些目标、写到哪里、最低证据是否满足、何时提交、Run 是否完成
```

这一分工既能减少模型选错工具和参数的概率，又不会牺牲现有 Harness Runtime 的专业能力。
