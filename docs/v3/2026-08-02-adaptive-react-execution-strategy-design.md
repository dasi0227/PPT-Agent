# Adaptive ReAct Execution Strategy 设计

> 日期：2026-08-02  
> 状态：设计确认稿  
> 范围：PPT Agent 的执行策略路由、可选 Plan、单一 ReAct Loop、完成门控与运行状态  
> 文档性质：仅定义目标架构与产品语义，不包含开发步骤、代码改造清单、排期或兼容方案

## 1. 背景

当前 Runtime 将复杂任务组织为固定的 Plan–Execute–Verify 流程，并进一步引入 Playbook、Plan
DAG、逐 Step 调度、独立 Verify Stage、Repair Stage 和再次 Verify。它可以提供确定性流程，
但对于以模型自主判断和工具反馈为核心的业务 Agent，已经表现出明显的 Workflow Engine 特征：

- Runtime 预先决定任务应当经历哪些节点；
- Playbook 固化不同目标的操作步骤；
- 每个 Step 拥有独立能力、目标、依赖和验证器；
- Verify 与 Repair 被放在 ReAct Loop 之外；
- 相近的 Compact Workflow 与 Full PEV 产生重复概念；
- Agent 难以根据新观察自然改变工作顺序；
- 简单任务和复杂任务共享过多 Workflow 基础设施；
- 新增业务能力容易继续增加节点、Stage 和 Playbook。

新的执行架构保留已经被证明有价值的 Context Engineering、执行策略路由、动态工具披露、
staging、权限边界、运行事件和可恢复状态，但删除固化 PEV。Runtime 的核心重新收敛为一个持续的
ReAct Loop，复杂任务只额外获得一份可动态更新的 Plan。

配套工具边界见
[PPT Agent Tool Surface 设计](./2026-08-02-ppt-agent-tool-surface-design.md)。

## 2. 核心结论

目标架构定义为：

> **Strategy-Routed ReAct Runtime with Optional Planning and Evidence-Gated Completion**

中文：

> **策略路由、可选规划、证据门控的 ReAct Runtime**

其核心结构是：

```text
Context Engineering
        ↓
Strategy Router
        ├── chat    → Read-only ReAct
        ├── simple  → Direct ReAct
        └── complex → Plan-guided ReAct
                         ↓
                  Completion Gate
                         ↓
                       Commit
```

整个普通 Run 只有一个主 ReAct Loop。Complex Plan 中的步骤不是独立 Agent、独立 Loop 或
Workflow 节点。

## 3. 设计目标

- 使用 `chat/simple/complex` 三种足够稳定的执行策略覆盖 MVP。
- 所有策略共享一个 ReAct Loop 抽象和统一工具执行边界。
- Complex 在同一个 Loop 中先规划、再执行，不创建逐 Step 子流程。
- Plan 是 Agent 的结构化工作记忆，不是 Runtime 的执行 DAG。
- 删除固定 Playbook，不把业务操作顺序硬编码在 Runtime。
- 删除独立 Verify/Repair Stage，把修正吸收回 ReAct。
- 保留不能依赖模型自觉的工具校验、证据新鲜度和提交门控。
- 允许 Simple 在发现范围扩大后升级为 Complex。
- 保持 talk/ask 与执行策略分离，避免把交互意图和执行复杂度混成同一维度。
- 保持运行可追踪、可中断、可恢复和有预算上限。

## 4. 非目标

- 不建立 BPMN、DAG Workflow 或可视化流程编排器。
- 不为 Spec/Presentation、Deck/Slide 分别维护固定 Playbook。
- 不为每个 Plan Step 启动新的 ReAct Loop。
- 不引入独立 Reviewer Agent 或 Verifier Agent。
- 不要求所有任务先生成 Plan。
- 不要求 Plan 在执行前由用户审批。
- 不把 `update_plan` 变成能力授权或目标授权机制。
- 不在本次设计中引入多 Agent 页面并行生成。
- 不考虑旧 PEV 名称、事件或状态的兼容。

## 5. 概念分层

### 5.1 Interaction Intent

Interaction 表达用户当前希望如何与 Agent 协作：

```text
talk
ask
execute
```

- `talk`：只读分析和交流，不修改 PPT。
- `ask`：只读探索，并允许通过 `ask_user` 做设计讨论和需求澄清。
- `execute`：授权在当前业务目标内修改 PPT。

### 5.2 Execution Strategy

Strategy 表达 Runtime 应当用多重的执行方式处理任务：

```text
chat
simple
complex
```

- `chat`：只读 ReAct。
- `simple`：没有 Plan 的直接 ReAct。
- `complex`：同一 ReAct Loop 中带动态 Plan。

### 5.3 Runtime Phase

Phase 只描述当前 Loop 的权限和提示词状态：

```text
chat
planning
executing
waiting_input
completion_check
committing
terminal
```

Phase 不是 Workflow 节点清单。普通情况下只发生少量状态切换，Runtime 不为业务步骤建立 Phase。

### 5.4 Plan Step

Plan Step 是 Agent 自己维护的工作清单条目，只包含目标描述和进度。它不声明：

- 工具能力；
- Artifact/Target 写入权限；
- Verifier；
- 执行函数；
- 依赖 DAG；
- 独立预算；
- 独立 ReAct Loop。

工具权限始终来自 WorkSpec、Strategy、Phase、整体 Target Scope 和 Runtime Policy。

## 6. Intent 与 Strategy 的关系

基础映射：

```text
interaction=talk → strategy=chat
interaction=ask  → strategy=chat
interaction=execute → Router 选择 simple 或 complex
```

规则：

- talk/ask 是用户明确的只读边界，Router 不能升级为写策略。
- execute 不等于 complex；局部明确修改默认优先 simple。
- Strategy 是 Run 级选择，但 simple 可以在运行中单向升级为 complex。
- complex 不因中途发现任务较简单而重新降级，避免权限和事件反复震荡。

## 7. Strategy Router

### 7.1 输入

Router 基于已经装配完成的 ContextPack 进行判断，至少使用：

- interaction intent；
- 用户指令；
- target type 与 target level；
- 当前 PPT 是否为空；
- Deck/Slide Resource 的存在与 materialization 状态；
- 预估受影响目标数量；
- 是否涉及整份、全局、多页、章节或页面顺序；
- 是否需要从零建立整体设计语言；
- 是否存在结构性或高影响修改；
- 指令明确度；
- 风险等级；
- 历史失败或范围扩张信号。

### 7.2 输出

```json
{
  "strategy": "complex",
  "reason": "需要从零生成整份演示文稿并协调多页设计",
  "confidence": 0.96,
  "risk": "medium",
  "signals": [
    {
      "name": "target_count",
      "value": "12"
    },
    {
      "name": "empty_project",
      "value": "true"
    }
  ]
}
```

Strategy Decision 必须可追踪，但不暴露模型 chain-of-thought。

### 7.3 Simple 判定

满足以下整体特征时选择 simple：

- 用户授权执行；
- 目标明确；
- 操作范围是一个 Deck Resource 或一个 Slide Resource；
- 不涉及页面增删、重排、章节重组或整份设计语言；
- 不需要多个相互依赖的业务决定；
- 可以在一次连续 ReAct 中通过少量工具调用完成；
- 即使执行失败，也可以通过 staging 安全放弃。

典型例子：

- 修改某页标题或核心句；
- 调整某页局部 HTML/CSS；
- 替换某一页的局部结构；
- 修改一个明确的全局主题 token；
- 在现有页面中应用一个已经选定的组件。

### 7.4 Complex 判定

满足任一关键特征时选择 complex：

- 从空项目生成整份 PPT；
- 多页生成或多页修改；
- 页面增删、重排或章节结构变化；
- 需要先确定全局设计语言再逐页生成；
- Deck Resource 与多个 Slide Resource 必须协调修改；
- 指令包含多个相互依赖的目标；
- 需要在执行中持续追踪长任务进度；
- 初始目标不明确到足以直接安全写入；
- Simple 运行中发生 scope expansion。

### 7.5 Router 实现原则

Router 可以组合确定性规则与一次轻量语义分类：

- 明确 interaction 和高置信度结构信号优先使用确定性规则。
- 规则无法可靠判断时，使用只读结构化分类。
- 分类结果不能扩大用户授权，只能在 execute 内选择 simple/complex。
- Router 不生成业务 Plan，也不决定具体操作顺序。
- Router 不通过固定关键词直接替代语义判断。

## 8. 统一 ReAct Loop

所有策略共享以下逻辑循环：

```text
compile current context + phase + disclosed tools
        ↓
LLM response
        ├── tool call
        │      ↓
        │   validate policy
        │      ↓
        │   execute tool
        │      ↓
        │   append observation
        │      └──────────────┐
        │                     │
        ├── ask_user          │
        │      ↓              │
        │   suspend/resume ───┘
        │
        └── finish
               ↓
        completion check
          ├── rejected → observation → loop
          └── accepted → commit/deliver
```

“一个 Loop”指一个连续的逻辑 Agent 会话。它仍然包含多次 LLM turn、工具调用和 observation，
但不会因为 Plan Step 改变而重建 Agent。

### 8.1 Loop Context

同一个 Run 中持续保留：

- 用户原始目标；
- Context Manifest 与已加载引用；
- Strategy Decision；
- 当前 Phase；
- 当前 Plan；
- 工具调用和精简 observation；
- staged ChangeSet；
- 渲染和校验证据；
- 预算与停止状态；
- 用户中途 steering 消息。

### 8.2 动态工具披露

工具 schema 可以在相邻 LLM turn 之间变化。例如 Complex Planning 首次成功创建 Plan 后，
下一 turn 开始开放写工具。上下文和 Agent 身份不变。

### 8.3 结束条件

Loop 只在以下条件之一满足时退出：

- Completion Gate 接受 `finish`；
- 用户取消；
- 达到 turn/token/time/tool budget；
- 连续不可恢复错误触发熔断；
- 外部终止；
- Runtime 自身发生致命错误。

所有策略都必须通过显式 `finish` 才能以成功状态退出。模型输出一段没有 tool call 的文本不自动
代表任务完成，Chat 也不例外；Runtime 将文本保留在上下文中，并要求 Agent 调用 `finish(message=...)`
交付最终回答，或调用已披露工具继续。这样“没有 `finish` 就继续，`finish` 被 Completion Gate 接受才
成功退出”在 chat、simple 与 complex 中完全一致。

## 9. Chat Strategy

### 9.1 运行方式

```text
Context
  ↓
phase=chat
  ↓
Read-only ReAct
  ↓
finish
  ↓
Deliver
```

### 9.2 工具

基础工具：

```text
read_ppt
search_refs
render_slide
finish
```

`interaction=ask` 时额外允许：

```text
ask_user
```

### 9.3 约束

- 不创建 staging transaction。
- 不披露 `mutate_ppt/mutate_ppt/update_plan`。
- `render_slide` 只产生读取型视觉证据。
- 普通文本不是隐式退出信号；最终分析必须作为 `finish.message` 提交。
- talk 不因 Agent 判断“最好顺手改一下”而升级为 simple。
- ask_user 的回答继续进入同一个 Chat ReAct Loop。

## 10. Simple Strategy

### 10.1 运行方式

```text
Context
  ↓
phase=executing
  ↓
Direct ReAct
  ↓
finish
  ↓
Completion Gate
  ↓
Commit / Return Observation
```

### 10.2 工具

根据目标和指令披露以下子集：

```text
read_ppt
mutate_ppt
mutate_ppt
search_refs
render_slide
finish
```

Simple 不披露 `update_plan`，也不制造一条只有一个步骤的虚假 Plan。

### 10.3 范围控制

Simple 创建一个 Run 级 staging transaction，并限定初始 Target Scope。工具调用只有在该 scope
内才允许写入。

### 10.4 升级为 Complex

发生以下情况时，Runtime 放弃继续按 Simple 完成，切换为 Complex Planning：

- Agent 尝试写入初始 scope 之外的第二个业务目标；
- 一个局部编辑被证明需要 Deck Resource 与 Slide Resource 协同修改；
- 需要页面增删、重排或章节结构变化；
- 同一目标连续六个工具 round trip 后仍无法形成可完成路径；
- Completion Gate 连续拒绝且原因显示需要多步骤协调；
- 工具返回明确的 `SCOPE_EXPANSION_REQUIRED`。

升级规则：

- 保留同一 Run、Context、对话和工具 observation。
- 已有改动仍留在 staging，并在 Planning Context 中明确标记为 tentative。
- Planning 必须把已有 staged changes 纳入计划，而不是假设项目仍是 committed 状态。
- 如果新 Plan 放弃已有改动，Runtime 可以丢弃对应 staged target，但不能影响正式文件。
- 发出新的 `strategy.selected`，原因标记为 runtime upgrade。
- 不要求用户再次确认。

## 11. Complex Strategy

### 11.1 单一 Loop、两个 Phase

```text
一个 Complex ReAct Loop
│
├── phase=planning
│   ├── 读取 PPT
│   ├── 检索参考
│   ├── 按需渲染现状
│   └── update_plan
│
│   首次有效 update_plan
│              ↓
├── phase=executing
│   ├── PPT 读写工具
│   ├── render_slide
│   ├── update_plan
│   └── finish
│
└── Completion Gate
```

Planning/Executing 是同一 Agent Loop 的权限状态，不是两个独立 Agent，也不是两个需要重新装配
全部上下文的 Workflow。

### 11.2 Planning Phase

Planning Phase 的目标是让 Agent在真实上下文基础上建立一份足够指导执行的轻量 Plan。

允许：

- `read_ppt`；
- `search_refs`；
- `render_slide`；
- `update_plan`；
- 在确实阻塞时使用 `ask_user`。

禁止：

- `mutate_ppt`；
- `mutate_ppt`；
- `finish` 一个尚未建立有效 Plan 的 complex 写任务；
- 通过计划内容扩大 Target Scope 或工具权限。

### 11.3 Planning → Executing

首次有效 `update_plan` 成功后：

1. Runtime 持久化 Plan revision。
2. 发出 `plan.created`。
3. Phase 自动切换为 `executing`。
4. 下一 LLM turn 重新编译当前 Prompt 和工具集合。
5. Agent 继续使用同一 conversation context 执行。

没有 Enter Plan、Exit Plan、Approve Plan 或重新启动 Executor 的步骤。

### 11.4 Executing Phase

Agent 根据 Plan 决定下一步，但实际操作仍然是普通 ReAct：

```text
读取目标
→ 写入或编辑
→ 查看 observation
→ 按需渲染
→ 发现问题后继续编辑
→ 更新 Plan
→ 处理下一个目标
```

Runtime 不读取 Step 标题并调用某个预定义 handler，也不逐 Step 创建工具集合。整体 scope 和
当前 phase 决定权限。

### 11.5 Replanning

执行中新事实导致计划变化时，Agent 直接调用 `update_plan`：

- 增加尚未预见但仍在整体 scope 内的步骤；
- 调整未完成步骤顺序；
- 拆分过大的步骤；
- 合并已经可以一次完成的步骤；
- 将失败路径替换为新路径；
- 更新 explanation。

这仍发生在 executing phase，不创建 Replan Stage，不重启 Planner，也不丢失现有上下文。

## 12. Plan 数据模型

模型可编辑部分保持轻量：

```json
{
  "explanation": "根据现有品牌规范，先统一全局设计，再逐页生成",
  "steps": [
    {
      "id": "deck-design",
      "title": "建立全局设计语言",
      "status": "completed"
    },
    {
      "id": "materialize-slides",
      "title": "逐页生成 HTML",
      "status": "in_progress"
    },
    {
      "id": "visual-review",
      "title": "检查全部页面视觉结果",
      "status": "pending"
    }
  ]
}
```

Runtime 管理但模型不能伪造的字段：

```text
plan_id
revision
created_at
updated_at
created_by_run
history
```

### 12.1 Plan 约束

- 至少一个 Step。
- Step ID 在 Plan 内唯一且稳定。
- 同时最多一个 `in_progress`。
- 状态只能是 `pending/in_progress/completed/failed`。
- 已完成步骤默认不能删除。
- Plan 不包含 dependencies、capabilities、verifiers 和 handler。
- Plan 不能引用整体 WorkSpec scope 之外的可写目标。
- Plan 更新不产生 PPT 业务写入。
- 前端展示 Plan revision 的最新投影，历史 revision 用于审计和恢复。

### 12.2 Plan 与真实执行

Plan 状态不是业务成果的证明：

- Agent 把 Step 标为 completed，不代表 PPT 已经成功写入。
- 工具结果、ChangeSet 和证据记录才是 Completion Gate 的事实来源。
- Plan 可以帮助 Gate 发现遗漏，但不能让 Gate 跳过客观检查。

## 13. 验证的新定位

### 13.1 删除固定 V Stage

新版不再执行：

```text
Execute
→ Verify Stage
→ Repair Stage
→ Verify Stage
```

验证被拆成三个更自然的层次：

```text
工具执行边界的确定性校验
        +
ReAct 中由 Agent 主动观察和修正
        +
finish 时的 Completion Gate
```

### 13.2 为什么不能只依赖 Prompt

System Prompt 可以要求 Agent 在适当时验证，但 Runtime 仍必须处理模型无法可靠保证的事实：

- 是否真的调用过渲染工具；
- 渲染证据是否对应最后一次修改后的 revision；
- JSON/HTML 是否符合确定性约束；
- 是否存在未解决的工具错误；
- staged baseline 是否发生 revision conflict；
- Complex Plan 是否仍有关键步骤未完成；
- 修改是否超出授权目标。

这些事实应由代码和运行状态判断，而不是让模型口头声明。

## 14. 工具内置校验

工具内置校验不是独立 Verifier 节点，而是成功写入 staging 的前置或后置条件。

### 14.1 `mutate_ppt/mutate_ppt`

至少执行：

- 参数 schema 校验；
- intent/strategy/phase/capability 校验；
- Target Scope 校验；
- JSON 领域 schema 校验；
- Outline 与 Slide Spec 引用完整性校验；
- HTML 基础解析；
- text edit 唯一锚点校验；
- staging 写入完整性；
- revision baseline 记录。

### 14.2 `render_slide`

至少记录：

- 被渲染 slide_id；
- staged/committed 来源；
- source revision/hash；
- screenshot ref；
- viewport；
- overflow、裁切和资源错误；
- console error；
- 完成时间。

### 14.3 Observation

任何校验或工具失败都直接形成结构化 observation 回到相同 ReAct Loop。Agent根据结果决定修正、
换方法、更新 Plan 或在无法继续时结束失败。

## 15. Completion Gate

### 15.1 定位

Completion Gate 是 `finish` 的 Runtime 接受条件，不是独立模型节点，也不是另一个 Agent。

```text
finish requested
      ↓
Completion Gate
      ├── accepted → commit/deliver
      └── rejected → structured observation → same ReAct Loop
```

### 15.2 通用检查

- 当前策略允许 finish。
- 没有正在执行的工具调用。
- 没有未处理的 fatal issue。
- 所有 staged target 均在 WorkSpec scope 内。
- 每个 staged target 的 baseline revision 仍有效。
- 写任务确实形成符合用户目标的 ChangeSet。
- 必需证据存在且对应当前最新 revision/hash。
- Complex Plan 没有关键 `pending/in_progress/failed` Step。
- 没有超出预算或取消状态。

### 15.3 拒绝语义

Gate 拒绝不进入 Repair Stage，而是返回：

```json
{
  "ok": false,
  "code": "VISUAL_EVIDENCE_REQUIRED",
  "summary": "slide-03 在最后一次 HTML 修改后尚未重新渲染",
  "required_actions": [
    {
      "tool": "render_slide",
      "target": "slide-03"
    }
  ]
}
```

下一轮模型继续收到原 Plan、staged changes 和该 observation。

### 15.4 拒绝上限

同一 Completion Gate 原因连续出现三次且没有新证据时触发熔断，以结构化失败结束 Run，防止模型
在 finish 与相同错误之间无限循环。

## 16. Evidence Policy

证据要求根据实际 ChangeSet 计算，不由固定 Playbook 声明。

| 实际变化 | 最低证据 |
|---|---|
| Chat，无写入 | 无 |
| Slide/Deck JSON Resource | 最新 revision 的领域 Schema 校验 |
| 章节、顺序或引用关系 | schema + 全局引用完整性 |
| 新建或修改 Slide HTML/CSS | 静态解析 + 最新 revision 的 `render_slide` |
| 新建 Slide | 页面模型、HTML、静态检查和最新渲染 |
| 修改影响所有页面的公共样式 | 所有受影响 Slide 的最新渲染 |
| 多页或整份生成 | 每个改变 Slide 的静态检查和最新渲染，外加全局引用完整性 |

规则：

- evidence 绑定 target、revision/hash 和产生时间。
- target 再次修改后，旧 evidence 自动变为 stale。
- stale evidence 可以保留审计，但不能满足 Completion Gate。
- warning 可以随成功结果交付；blocking issue 必须修复。
- Agent 可根据视觉判断主动多次渲染，Gate 只要求最低证据，不限制更充分的自检。

## 17. Repair 的新定位

Repair 不再是 Runtime Stage。以下过程就是普通 ReAct：

```text
mutate_ppt
→ render_slide 返回 overflow
→ Agent 分析 observation
→ mutate_ppt 修正布局
→ render_slide
→ update_plan
→ finish
```

只有确定性工具执行和 Completion Gate 返回问题证据；如何修正由 Agent自主决定。

因此不需要：

- `repairStepFor`；
- Repair Playbook；
- `MaxRepairRounds` 作为独立循环；
- Repair 专用工具集合；
- Verify/Repair/Verify 状态往返。

仍然保留全局 turn、token、time、tool call 和 completion rejection 预算。

## 18. Staging 与 Commit

### 18.1 Chat

Chat 不创建 staging transaction，不进入 commit。

### 18.2 Simple/Complex

Simple 和 Complex 在第一次业务写入前拥有 Run 级 staging transaction：

- 所有 PPT 写工具只操作 staged view；
- 同一 ReAct Loop 中的后续读取优先读取 staged view；
- `render_slide` 默认渲染 staged view；
- Plan 更新和参考资料读取不属于 PPT staging；
- Completion Gate 接受后才原子提交；
- Run 失败、取消或预算耗尽时不污染正式版本。

### 18.3 Commit

Commit 是 Runtime 确定性动作，不是 LLM 工具：

```text
Completion accepted
→ revision conflict check
→ atomic file + metadata commit
→ emit committed events
→ deliver final message
```

Commit 失败时 Run 失败并保留可诊断信息，不能要求 Agent 通过 ReAct 猜测基础设施修复方式。

## 19. `ask_user` 与无人值守

- talk 不需要 ask_user。
- ask 允许 Agent 主动使用 ask_user。
- simple/complex 默认自主决策。
- 只有缺失信息会实质改变用户目标、且无法从 Context/Reference 得出安全默认值时，执行策略才能
  调用 ask_user。
- `ask_user` 产生 checkpoint 并把 phase 切为 `waiting_input`。
- 用户回答后恢复原 Strategy、原 Phase、同一个 ReAct Loop、Plan 和 staging。
- 非交互运行中，Runtime 使用预先声明的默认决策政策；没有安全默认时结构化失败，不无限等待。

## 20. 中断与恢复

Run checkpoint 至少保存：

- strategy decision；
- current phase；
- conversation/context cursor；
- current plan 与 revision history；
- staged ChangeSet；
- tool call ledger；
- evidence ledger；
- budgets；
- waiting question；
- completion rejection history。

恢复后：

- 不重新运行已经成功且幂等确认的工具调用；
- 不把 completed Plan Step 重置为 pending；
- 重新校验 staged baseline revision；
- 重新计算 evidence freshness；
- 从最后一个完整 observation 后继续同一个逻辑 ReAct Loop。

恢复不是重新执行整个 Plan，也不是为剩余 Step 新建 Workflow。

## 21. 事件模型

建议保留面向 Run、Strategy、Plan、Tool、Evidence 和 Terminal 的事件，而不再以 PEV Stage 为主轴：

```text
run.started
context.assembled
strategy.selected
phase.changed
plan.created
plan.updated
tool.called
tool.completed
target.staged
evidence.recorded
needs_input
completion.checked
target.committed
status.summary
run.completed
run.failed
run.canceled
```

### 21.1 事件原则

- `phase.changed` 只用于 planning/executing/waiting/completion/committing 等少量状态。
- Plan Step 状态变化通过 `plan.updated` 投影，不制造 `step.started/step.completed` Workflow 事件。
- Gate 拒绝使用 `completion.checked{accepted:false, issues}`。
- 工具失败和 Gate 拒绝都可回放为 Agent observation。
- 不持久化或展示原始 chain-of-thought。
- SSE 回放和实时流使用同一事件语义。

### 21.2 前端呈现

- Chat 不显示空 Plan。
- Simple 不显示伪造的单步骤 Plan。
- Complex 在首次 `plan.created` 后显示 Plan。
- Plan 只展示进度，不暗示每个 Step 是一个后台 Workflow 节点。
- Planning/Executing 可以显示为轻量状态标签。
- Completion Gate 问题呈现为可理解的当前阻塞，不展示内部 Verifier 节点。

## 22. 预算与熔断

每个 Run 统一配置：

- max LLM turns；
- max tool calls；
- max token budget；
- max wall-clock time；
- max consecutive tool failures；
- max identical completion rejections；
- context compaction threshold。

策略可以选择不同默认预算，但不建立 Step 级预算。Complex 的 Plan 长度不能自动扩大总预算。

预算耗尽时：

- 不提交未完成 staging；
- 保留 checkpoint、Plan、ChangeSet 和最后问题；
- 发出结构化失败；
- 不把“预算耗尽”伪装成成功 finish。

## 23. 与当前设计的替换关系

| 当前概念 | 新设计 |
|---|---|
| `respond` | `chat` |
| `direct_action` | `simple` |
| `compact_workflow` | 合并为 `complex` |
| `full_pev` | 合并为 `complex` |
| `PlaybookFor` | 删除；由 Agent 根据 Context 和 observation 自主决定操作 |
| 固定 Planner 节点 | 同一 ReAct Loop 的 planning phase |
| Workflow Plan DAG | 轻量、可更新的 Plan checklist |
| 逐 Step Executor 调度 | 删除；Agent 直接在同一 Loop 调用业务工具 |
| Step capability/target | 整体 WorkSpec scope + phase + tool policy |
| Verify Stage | 工具内校验 + Completion Gate |
| Repair Stage | 相同 ReAct Loop 根据 observation 自主修正 |
| Commit Stage | 保留为 Runtime 确定性提交动作 |
| Verifier Registry 编排 | 删除；有价值的检查逻辑下沉为工具校验或 Gate evidence policy |

## 24. 保留的专业 Runtime 能力

删除固化 PEV 不代表退化为 Toy Demo。以下能力继续保留：

- Context Manifest 与渐进式参考检索；
- 策略路由与决策可解释性；
- 动态工具披露；
- 工具执行时二次权限校验；
- Target Scope；
- staging transaction；
- revision conflict；
- 结构化 tool observation；
- Plan 持久化与 revision history；
- evidence freshness；
- Completion Gate；
- 中断、checkpoint 与恢复；
- budgets、取消和熔断；
- 事件总线、持久化与 SSE 续传；
- 原子 Commit；
- 可观测性和审计。

专业性来自清晰的控制边界、证据和恢复能力，而不是 Stage 数量。

## 25. 架构不变量

1. 一个普通 Run 只有一个主 ReAct Loop。
2. Plan Step 不启动新的 Loop。
3. talk/ask 永远不能写 PPT。
4. simple 不创建 Plan。
5. complex 在首次业务写入前必须有有效 Plan。
6. Plan 不授予工具或目标权限。
7. 所有业务写入必须进入 staging。
8. 任何工具调用在执行时都必须重新校验权限和 scope。
9. HTML/CSS 最新修改必须拥有同 revision 的渲染证据才能完成。
10. Completion Gate 拒绝后返回同一 Loop，不启动 Repair Workflow。
11. 模型不能直接执行 Commit。
12. 未完成、失败、取消和预算耗尽的 Run 不修改正式版本。
13. 已完成 Plan Step 和 evidence history 不因后续更新被静默删除。
14. 前端只展示结构化 action summary，不展示 chain-of-thought。

## 26. 最终运行结构

```text
Run
├── Context Engineering
├── Strategy Router
│   ├── chat
│   ├── simple
│   └── complex
├── One ReAct Loop
│   ├── dynamic phase
│   ├── dynamic tool disclosure
│   ├── optional update_plan
│   ├── PPT tool calls
│   ├── observations
│   └── finish candidate
├── Completion Gate
├── Atomic Commit
└── Final Delivery
```

这套结构保留 Agent 的自主性，也保留业务系统必须具备的确定性边界。Plan 只在复杂任务中出现，
Verify 不再是固定流程节点，Repair 由同一个 ReAct Loop 自然完成。
