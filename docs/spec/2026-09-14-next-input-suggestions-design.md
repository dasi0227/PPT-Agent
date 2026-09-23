# 下一步输入预测与快捷填入设计

> 2026-09-23 更新：有效性、持久化字段、公共事件版本、前端展示与历史恢复规则，以[下一步输入建议生命周期简化设计](2026-09-23-next-input-suggestions-lifecycle-design.md)为准。下文项目 revision 绑定及相关 QA 是原始决策记录，已被最新用户决定替代；生成与快捷填入交互继续沿用。

- 状态：已实现并通过确定性测试与真实模型观察；按本次开发要求未做真实 Chromium 验收
- 日期：2026-09-14
- 对应 TODO：十、预测并快捷填入下一步需求
- 关联设计：
  - 2026-09-12-creation-prompt-upgrade-design.md
  - 2026-09-12-project-checkpoint-rollback-design.md

## 1. 需求解释

Agent 真正成功完成一次任务后，根据当前项目状态、会话上下文、刚完成的目标和仍可继续推进的工作，给出零至三个可直接作为下一条用户指令的输入候选。

候选显示在输入框内部，视觉上承担动态 placeholder 的作用。用户可以点击候选、使用键盘逐项聚焦，或按 `Option/Alt + 1/2/3` 将候选填入原输入框；选择只产生普通草稿，不自动发送，也不改变模式、模型、技能、附件、引用或修改范围。用户也可以忽略候选直接输入。

本功能的核心不是给最终回复增加“你还可以做什么”的固定尾注，而是降低连续创作时组织下一条有效指令的成本。候选属于最终消息的持久化 UI 元数据，不属于回复正文，也不进入后续模型上下文。

## 2. 产品目标

1. 利用完成当前任务的同一次模型推理生成有上下文的后续输入，不增加独立 Model Call。
2. 候选兼顾用户最可能继续提出的需求和当前项目最值得推进的工作，宁缺毋滥。
3. 在输入框完全空白时提供不打断创作的轻量提示，并支持鼠标、键盘和辅助技术操作。
4. 候选与产生它的成功 Run、最终消息和项目 Checkpoint history revision 建立稳定关联。
5. 新请求、跨会话修改、项目回退、失败恢复和 SSE 重放后仍保持一致，不展示已过期候选。
6. 候选缺失或异常不得阻断主体任务完成，不增加 Completion Gate 失败或模型重试。

## 3. 已确认范围

### 3.1 纳入范围

- Chat、Grill、Execute 模式通过 `finish` 成功结束后的下一步输入候选。
- Plan 获批并进入 Execute 后，在最终成功 `finish` 时生成候选。
- `finish` 的结构化候选参数、Runtime 规范化、Run 元数据及公共事件协议。
- 候选随 `message.final` 事件持久化、SSE 重放和会话历史 hydration。
- 项目 Checkpoint history revision 绑定、跨会话失效、回退及恢复后的重新计算。
- 输入框内可交互建议层、自然高度、点击、Tab、Enter、空格及平台化数字快捷键。
- 输入法、弹窗、其他可编辑控件、菜单和临时 `Escape` 隐藏的冲突处理。
- 确定性自动化测试和一次真实模型效果观察；真实 Chromium 交互验收按 2026-09-14 的最新开发要求豁免。

### 3.2 不纳入范围

- 为候选额外发起 Model Call。
- 新增独立 Agent 工具或在 `finish` 后继续 ReAct 循环。
- “换一批”、刷新候选或手工编辑候选列表。
- 候选自动发送，或自动切换模式、Scope、模型、技能和引用。
- 候选分类、推荐理由、隐藏的执行配置或结构化动作参数。
- 在历史时间线的最终消息下渲染候选。
- 将候选加入 transcript、上下文压缩、线程记忆或后续模型消息。
- 记录候选点击、快捷键选择、编辑或采纳率等产品分析事件。
- 用向量或额外模型判断语义重复。
- 为旧公共事件版本保留双读、双写或兼容分支。

## 4. 当前实现与问题定位

### 4.1 `finish` 与终态顺序

当前 `backend/internal/workflow/runtime.go` 只向 Chat、Grill、Execute 的合法阶段披露 `finish(message)`。候选通过 Completion Gate 和 Execute 提交检查后，Runtime 依次发出：

1. `message.final`，携带最终回复正文和受影响对象；
2. `run.completed`，携带耗时和终态信息。

普通 assistant text 不是完成信号，Plan 模式审批阶段也没有 `finish`。因此下一步候选应成为现有 `finish` 的附属结构化参数，并只随最终被接受的 `finish` 发布。新建候选工具会制造额外控制动作和终态排序问题；独立 Model Call 会增加延迟、成本、取消和过期状态。

### 4.2 事件与历史恢复

Run 事件以 JSON payload 持久化在 `run_events`，支持 Last-Event-ID 重连和会话历史 hydration。前端目前把 `message.final` 归约成 `FinalMessageItem`，把 `run.completed` 作为同一 Run 的终态补充。

候选应复用这条事件真相链路，而不是新建“当前候选”数据库表。前端通过事件顺序派生当前候选，避免服务端事件与单独最新值发生双写漂移。

### 4.3 Composer placeholder

当前 `PromptComposerEditor` 是受约束的 `contenteditable`。普通 placeholder 通过 `data-placeholder` 和 CSS `::before` 绘制，只能显示不可交互文本，也不会自然进入 Tab 顺序或形成可点击候选。

本需求需要在同一输入框容器内增加真正的建议层。它保持 placeholder 的弱视觉层级，但候选项是按钮；原 `data-placeholder` 只在没有有效候选时继续使用。

### 4.4 项目历史版本

项目 Checkpoint 已提供项目级单调 `revision`，Run 建立发送前基线时会推进该 revision。它覆盖跨会话创作、手动修改、回退、恢复和旧未来丢弃，是判断候选是否仍基于当前项目状态的权威边界。

页面、Manifest、Outline、Design 各自的 revision 只描述局部资源，`updated_at` 也不能可靠表达完整项目历史，因此不另建复合内容指纹。

## 5. 候选生成语义

### 5.1 生成时机

仅在模型认为任务已完成并调用 `finish` 时生成。若 Completion Gate 拒绝该次 `finish`，候选随同候选回复一起留在内部循环，不产生公共事件。Agent 修复任务并再次调用 `finish` 时重新决定候选。

候选不在以下节点生成：

- `ask_user` 等待用户回答；
- Plan 创建或更新后等待审批；
- 命令权限或 Scope 扩权等待；
- Run 失败、错误、取消或可恢复暂停；
- Engine 为非生产自定义执行补发的失败终态。

### 5.2 数量与排序

模型自主返回零至三个候选：

- 没有自然且有价值的后续动作时返回空数组或省略字段；
- 不为填满三个而生成通用、重复或产品无法执行的内容；
- 第一项是最自然且预期价值最高的延续；
- 后续项尽量提供不同方向，但不固定为检查、修改、导出等机械类别；
- Runtime 按模型顺序保留前三个有效项，不重新排序。

### 5.3 内容要求

每项候选必须：

- 跟随用户当前主要语言；
- 使用用户可直接发送的命令句；
- 优先延续刚完成的任务和当前项目；
- 只建议当前产品真实可执行的操作；
- 需要定位时使用用户可见的页码或页面标题；
- 不出现内部 ID、推荐理由、Markdown、编号或快捷键说明；
- 不使用“你可以……”“是否要……”等 Agent 建议口吻；
- 不超过 80 个 Unicode 字符。

候选只是输入文本，不绑定 Composer 配置。用户改变模式、Scope、模型或技能后候选仍可显示，选择候选不得覆盖这些设置。候选若提出更大范围的工作，实际发送仍由现有 Scope、权限和扩权机制约束。

## 6. `finish` 与 Runtime 协议

### 6.1 Tool Schema

`finish` 增加可选属性：

```json
{
  "type": "object",
  "required": ["message"],
  "properties": {
    "message": { "type": "string" },
    "suggested_next_inputs": {
      "type": "array",
      "maxItems": 3,
      "items": { "type": "string" }
    }
  },
  "additionalProperties": false
}
```

`message` 继续遵守现有非空规则。`suggested_next_inputs` 不进入 required；缺失、`null`、错误类型或单项错误按可恢复辅助数据处理，不得把合法的主体完成转换为 Tool 参数失败。

若通用 Tool 参数校验会在 Runtime 读取前拒绝错误类型，实现必须为 `finish.suggested_next_inputs` 建立宽容解析边界，或只对该可选字段移除后继续校验 `message`。不能为了容错放宽其他工具或 `finish.message` 的现有合同。

### 6.2 规范化

Runtime 在接受主体 `finish.message` 后，对候选按原顺序逐项处理：

1. 非字符串项丢弃；
2. 移除不可见控制字符；
3. 把 Unicode 换行、Tab 和连续空白折叠为一个普通空格；
4. 去除首尾空格；
5. 空字符串丢弃；
6. 使用 Unicode rune 数计算长度，超过 80 的整项丢弃，不截断；
7. 使用规范化文本的 Unicode 大小写折叠值去重，保留首次出现项；
8. 得到三个有效项后停止处理。

不做标点改写、内容补全、语义近似判断或自动翻译。规范化后的零至三个字符串作为唯一公开结果。

### 6.3 Completion Gate

Completion Gate 继续只检查主体任务、计划、工作账本、证据、Scope、提交和最终回复。候选：

- 不新增 CompletionIssue；
- 不参与相同 Gate rejection 计数；
- 不触发 Semantic Reviewer；
- 不影响 Execute materialization commit；
- 不改变 `run.completed`、`run.failed`、`run.error` 或 `run.canceled` 的判定。

### 6.4 Runtime 状态与 Outcome

`RunState` 保存已接受并规范化的候选。`StructuredOutcome` 增加候选及项目 history revision，使 Engine 的受保护完成 fallback 仍能构造 v4 `message.final`，但失败和取消 Outcome 不携带可激活候选。

Runtime Checkpoint 无需保存未接受的候选。候选只在最终 `finish` 已通过 Gate、Execute 提交已完成后成为终态数据；进程在此之前恢复时由模型继续原循环并重新生成。

## 7. Run 与项目 history revision

### 7.1 Run 元数据

在项目历史启用时，`projecthistory.Manager.Baseline` 应返回创建基线后权威的项目 history revision。`RunService.CreateRun` 将其写入新 Run 的 `ProjectHistoryRevision`，并随 Run 一起持久化。

建议增加：

```text
model.Run.ProjectHistoryRevision int64
runs.project_history_revision    INTEGER NOT NULL
runPO.ProjectHistoryRevision     int64
```

该值是 Run 创建时的不可变快照，不随 Scope revision、页面 revision 或 Run 状态变化。RuntimeInput 从持久化 Run 获得该值，重启恢复不得重新查询并覆盖。

没有启用项目历史的隔离单元测试或自定义 Execution 可以使用零值；零值候选不得在生产 Composer 中激活。

### 7.2 创建失败与幂等

- 同一 `client_request_id` 重放必须返回同一 Run 和同一 history revision。
- Baseline 成功但 Run 创建或启动失败时，沿用现有项目历史恢复与启动屏障语义，不额外推进第二次 revision。
- Run 恢复、进程重启和暂停继续使用持久化 revision。
- Scope 扩权和 steering 属于原 Run，不改变候选绑定的项目 history revision。

### 7.3 公共事件 v4

`PublicEventSchemaVersion` 从 3 直接升级为 4。`message.final` 的 v4 payload 为：

```json
{
  "schema_version": 4,
  "run_id": "run-id",
  "occurred_at": "2026-09-14T00:00:00Z",
  "message_id": "message-id",
  "text": "最终回复",
  "affected_targets": [],
  "suggested_next_inputs": [],
  "project_history_revision": 12
}
```

规则：

- `suggested_next_inputs` 在 v4 中始终序列化为数组，零候选时为 `[]`；
- `project_history_revision` 始终存在，生产 Run 应为正整数；
- `message.final.text` 保持现有安全处理和 Markdown 展示语义；
- 候选只作为纯字符串传输，前端不得按 HTML 解释；
- `run.completed` 不重复候选，保持纯运行终态职责；
- 不保留 v3/v4 双读、双写或降级事件。

Run 事件 JSON 已经是持久化来源，不新增候选表。项目 Checkpoint 快照现有的 `runs`、`run_events` 资源清单随新列和 v4 payload 自然保存、回退及恢复。

## 8. Prompt 设计

新增 `backend/prompts/runtime/next-input-suggestions.md`，注册为 `runtime.next-input-suggestions`。仅在当前模式和阶段披露 `finish` 时加载，加载条件与 `finish` Tool Schema 保持同一函数事实，避免 Prompt 要求模型输出一个当前不存在的字段。

模块应说明：

- 候选是用户下一条输入，不是最终回复附言；
- 返回零至三个，宁缺毋滥；
- 第一项优先，其他项提供有意义的不同方向；
- 结合当前任务、项目状态、未解决工作和用户语言；
- 只提出当前产品和已披露能力可以继续处理的需求；
- 使用直接命令句，不含编号、Markdown、解释、内部 ID 或换行；
- 每项不超过 80 个 Unicode 字符；
- 没有合适建议时返回空数组；
- 不因为生成候选而继续工具调用或延迟 `finish`。

`finish` Tool Description 只补充 `suggested_next_inputs` 是可选的零至三条下一步用户输入。Chat、Grill、Execute 不分别复制同一规则。

Prompt Catalog 的协调版本、正文 SHA-256、嵌入文件、模块顺序快照及现有“未知/空模块失败”测试必须同步。候选模块不加入 Reviewer、polish、kickoff、handoff、compact 或 commit 等专用调用。

## 9. 前端派生状态

### 9.1 类型

`MessageFinalPayload` 和 `FinalMessageItem` 增加：

```ts
suggestedNextInputs: string[];
projectHistoryRevision: number;
```

Timeline 继续只渲染 `text`、affected targets 和耗时，不渲染候选。候选元数据可保留在 FinalMessageItem，或在 reducer 同时投影为 RunSession 的派生候选；不得复制到 composerStore 或 localStorage 草稿场景。

建议的会话候选结构：

```ts
interface NextInputSuggestionState {
  runId: string;
  messageId: string;
  items: string[];
  projectHistoryRevision: number;
  status: 'staged' | 'eligible';
}
```

`staged` 表示只观察到 `message.final`，`eligible` 表示同一 Run 的 `run.completed` 已到达。是否展示仍由项目 history revision、项目刷新状态、草稿和本地隐藏状态共同派生，不额外保存 `active` 真相。

### 9.2 实时事件状态机

对每个会话按权威 SSE 顺序处理：

1. `run.started` 清除该会话此前候选，表示旧候选已被可靠接受的新请求消费；
2. `message.final` 暂存该 Run 的候选、message ID 和 history revision，`terminal=false`；
3. 同一 Run 的 `run.completed` 把暂存候选标记为 `terminal=true`；
4. 同一 Run 的失败、错误或取消清除暂存候选，不恢复此前候选；
5. 重复事件按现有 event ID 幂等，不得重复候选或回退状态；
6. 新旧 Run 的迟到事件必须按 run ID 关联，旧 Run 不得覆盖新 Run 已消费的状态。

`message.final` 到达但 `run.completed` 尚未到达时，最终回复可以照常出现在时间线，候选不得提前显示。

### 9.3 历史 hydration

`historyHydrator` 对已持久化事件运行同一状态机，而不是简单寻找最后一条带候选的最终消息。由此保证：

- 成功 Run 后可恢复候选；
- 后续 Run 一经开始即消费旧候选；
- 后续 Run 失败、错误或取消后不复活旧候选；
- 只有 final 而没有 completed 的异常历史不激活；
- Checkpoint 回退删除未来事件后，可恢复过去现场中最近的有效候选；
- 恢复到最新后，按恢复的完整事件历史得到原现场候选。

### 9.4 项目版本确认

前端收到 `run.completed` 后，在现有项目内容刷新之外同步刷新 `GET /projects/:id/history`，把权威 revision 保存到 `projectHistoryStore.states[projectId]`。候选只有在以下条件成立时才有效：

```text
candidate.terminal
&& candidate.projectHistoryRevision > 0
&& candidate.projectHistoryRevision === currentHistory.revision
&& projectContentRefreshSucceeded
&& historyRefreshSucceeded
```

任一刷新进行中或失败时使用普通 placeholder。之后重试或其他既有刷新成功，selector 重新计算并自动显示仍匹配的候选，不永久丢弃。

跨会话写入使项目 history revision 变化时，旧会话候选无需逐一删除；统一 selector 比较会让它们立即失效。纯切页、展开面板或预览模式变化不推进 history revision，因此不影响候选。

## 10. 建议层与 Composer 行为

### 10.1 展示条件

建议层只在以下条件全部满足时渲染：

- 当前存在活动项目和活动会话；
- 当前会话有一至三个已完成候选；
- Run 不处于 creating、running、waiting、paused、recovering 或 canceling；
- 项目内容和 history revision 已成功刷新并与候选匹配；
- 文本为空；
- 没有图片附件、DOM 标记、组件/页面引用、恢复输入或其他消息资源；
- 当前 Composer 聚焦周期没有被 `Escape` 临时隐藏；
- 输入框未 disabled 或 readOnly。

Scope、模式、模型和技能不是消息草稿内容，它们变化时不隐藏或作废候选。

### 10.2 展示结构

建议层位于 `PromptComposerEditor` 的编辑区域内部，视觉层级接近普通 placeholder：

```text
按「Option + 序号」选择或直接输入下一步需求
1. 统一检查整套演示的视觉一致性
2. 继续优化当前页的信息层级
3. 为关键页面补充更有说服力的数据表达
```

非 macOS 平台把 Option 动态显示为 Alt。只有一个或两个候选时按实际数量显示。零候选不渲染标题或空列表，继续显示“输入你的想法与目标”。

候选完整换行展示，编辑器容器按实际内容自然增高；不做单行省略、行数截断、内部滚动或仅悬停可见的隐藏文本。现有 Composer 外层布局继续负责页面级可用空间。

### 10.3 鼠标与普通键盘

- 每项候选使用原生 button 语义并进入 Tab 顺序；
- 点击、Enter 或空格调用同一个选择函数；
- 选择函数只把候选纯文本写入现有受控 editor value 和 thread draft；
- 写入后把光标移动到文本末尾并聚焦编辑器；
- 不调用 submit，不执行 slash command，不更新其他 Composer 控制项；
- 候选按钮使用完整文本作为无障碍名称；
- 标题和按钮组通过 `aria-describedby` 或等价关系说明快捷键和“仅填入、不发送”。

点击建议层候选之外的编辑区域应正常聚焦 contenteditable。聚焦本身不隐藏建议；首次实际输入或加入草稿资源后才隐藏。

### 10.4 `Option/Alt + 1/2/3`

当建议层可见时，`CommandComposer` 在当前工作区监听键盘：

- macOS 要求 `event.altKey`，其他平台同样使用 Alt 物理修饰键；
- 使用 `event.code === Digit1/Digit2/Digit3` 选择对应索引，避免 macOS Option 组合后的 `event.key` 特殊字符；
- 只允许 Alt/Option，不同时接受 Meta、Ctrl 或 Shift；
- 索引超过当前候选数量时不处理；
- 命中合法候选后 `preventDefault`、填入文本并聚焦末尾；
- `event.isComposing`、编辑器内部组合态、弹窗、候选菜单、斜杠菜单或其他可编辑元素活动时不拦截；
- 事件来自当前 Composer 自身的空编辑器时允许处理；来自其他 input、textarea、contenteditable 或可编辑组件时拒绝处理。

监听器随建议层资格挂载和卸载，不建立永久全局快捷键。项目、会话或候选变化时闭包必须使用最新候选，旧监听器不得填入旧文本。

### 10.5 `Escape` 临时隐藏

`Escape` 只在建议层可见且 Composer 当前聚焦时生效：

1. 阻止建议层相关默认动作，但不让编辑器失焦；
2. 设置 CommandComposer 本地 `suggestionsDismissedForFocus=true`；
3. 当前聚焦周期内隐藏建议，数字快捷键同步停用；
4. 用户可以立刻在空编辑器中输入；
5. 输入框 blur 时重置该布尔值；
6. 再次聚焦或失焦后，只要其他展示条件仍成立，建议重新出现。

该状态不写入 runStore、composerStore、sessionStorage、localStorage 或服务端。项目切换、会话切换和候选身份变化也重置它。

### 10.6 草稿生命周期

- 候选选择后成为普通 thread draft；清空文本且没有其他资源时，同一组候选重新出现。
- 添加图片、DOM 标记、页面/组件引用或恢复输入后隐藏；用户在未提交前把这些资源全部移除，候选可以重新出现。
- 新 Run 的 `run.started` 是消费边界；不以点击候选、输入文字或客户端发起请求作为消费成功依据。
- 创建 Run 请求失败、尚未获得 `run.started` 时，原草稿和候选来源仍在；由于草稿非空，候选层保持隐藏，用户清空后可以再次看到。
- 已收到 `run.started` 后，即使新 Run 最终失败或取消，也不恢复旧候选。
- steering 发生在现有活动 Run 内，建议层本来不可见，不创建独立候选消费边界。

## 11. 并发、恢复与安全

### 11.1 SSE 顺序与迟到事件

候选状态必须同时校验 thread ID、run ID 和 message ID。旧 EventSource、重复 `message.final`、迟到 `run.completed` 或取消重连的终态补偿不得覆盖当前 Run 状态。沿用现有 processed event ID 去重，并对没有对应 pending final 的 completed 保持空候选。

### 11.2 项目切换与跨会话

候选来源按会话隔离，版本有效性按项目共享：

- 切换会话读取目标会话自己的派生候选；
- 会话 B 推进项目 history revision 后，会话 A 的旧候选因 selector 不匹配而失效；
- 不把 B 的候选复制给 A；
- 切换项目时清理本地聚焦隐藏状态和键盘监听，不能短暂展示上一个项目的候选。

### 11.3 Checkpoint 回退与恢复

项目回退会恢复 runs、run_events、项目 history state 和前端 scene，并触发整页重新加载。重新 hydration 后只可能得到恢复历史内的候选，再与恢复后的 history revision 比较。未来分支候选不得通过前端缓存、旧 SSE 或 localStorage 回流。

恢复到最新遵循相同逻辑。候选不单独写入 scene，因为事件历史已经是其来源；`Escape` 临时隐藏也不恢复。

### 11.4 文本安全

候选由模型生成，始终按不可信纯文本处理：

- 后端只保存规范化字符串；
- JSON 编码由标准库完成；
- React 只以文本节点渲染；
- 不设置 `innerHTML`；
- 填入 contenteditable 时复用 `setPlainText`/纯文本适配器；
- 候选中的 slash、页面或组件触发符只在成为草稿后按用户正常编辑行为解释，不直接执行；
- 最终发送仍经过现有 Run 输入、Scope、权限和工具安全检查。

## 12. 预计改动范围

### 12.1 后端

- `backend/prompts/runtime/next-input-suggestions.md`
- `backend/embedded_prompts.go`
- `backend/internal/prompt/catalog.go`
- `backend/internal/prompt/loader_test.go`
- `backend/internal/workflow/prompt_modules.go`
- `backend/internal/workflow/runtime.go`
- `backend/internal/workflow/runtime_test.go`
- `backend/internal/workflow/domain.go`
- `backend/internal/workflow/public_events.go`
- `backend/internal/workflow/public_events_test.go`
- `backend/internal/model/public_event.go`
- `backend/internal/model/run_entity.go`
- `backend/internal/projecthistory/history.go`
- `backend/internal/service/run.go`
- `backend/internal/run/engine.go`
- `backend/internal/store/sqlite/po.go`
- `backend/internal/store/sqlite/run_store.go`
- `backend/internal/store/sqlite/migrate.go`
- 相关项目历史、Run 恢复、SQLite 和 HTTP E2E 测试

实现时可把规范化函数放入独立 workflow 文件及测试文件。若实际依赖关系表明更合适，可调整文件位置，但不得改变本设计的数据所有权和终态语义。

### 12.2 前端

- `frontend/src/api/types.ts`
- `frontend/src/api/sse.ts`
- `frontend/src/api/sse.test.ts`
- `frontend/src/api/projectHistory.ts`
- `frontend/src/stores/runStore.ts`
- `frontend/src/stores/runStore.test.ts`
- `frontend/src/stores/projectHistoryStore.ts`
- `frontend/src/features/agent/eventReducer.ts`
- `frontend/src/features/agent/eventReducer.test.ts`
- `frontend/src/features/agent/historyHydrator.ts`
- `frontend/src/features/agent/historyHydrator.test.ts`
- `frontend/src/features/agent/CommandComposer.tsx`
- `frontend/src/features/agent/PromptComposerEditor.tsx`
- `frontend/src/features/agent/PromptComposerEditor.test.tsx`
- `frontend/src/features/agent/promptComposer.css`
- 必要时新增建议层组件、selector 及其测试

## 13. 开发顺序

1. 扩展项目 Baseline 返回值、Run 元数据、SQLite Schema 与恢复测试，建立不可变 history revision 关联。
2. 新增 Prompt 模块，扩展 `finish` Schema、宽容解析、规范化和 Runtime Outcome。
3. 升级公共事件到 v4，贯通 `message.final` 实时事件、持久化、SSE 解析与历史 hydration。
4. 在 runStore/projectHistoryStore 建立候选状态机和版本确认 selector，验证跨会话及回退恢复。
5. 实现输入框内建议层、草稿资格、点击、Tab、数字快捷键、`Escape` 和无障碍说明。
6. 完成确定性自动化和真实模型观察并记录证据；真实 Chromium 验收按本次开发要求省略，完成其余验收后勾选 TODO 10。

阶段划分不是部分协议长期并存许可。按 AGENTS.md 直接切换 Run、事件、前端类型、持久化、测试和 Prompt，不增加旧字段兼容层。

## 14. 验收与测试

### 14.1 后端确定性测试

- `finish` 只有 `message` required，候选字段可选且最多三项。
- 缺失、`null`、错误类型、空项、控制字符、空白折叠、Unicode 长度、大小写重复和超过三项的规范化结果。
- 候选异常不阻止合法 `finish`，不增加 Gate issue 或模型轮次。
- Gate 拒绝的 `finish` 不产生公开候选；后续成功 `finish` 只发布最新候选。
- Chat、Grill、Execute 加载新 Prompt 模块；Plan 审批等待阶段不加载。
- Prompt Catalog 路径、协调版本、正文哈希和模块顺序稳定。
- Baseline revision 正确写入 Run、SQLite round-trip、幂等重放、暂停恢复和进程重启。
- v4 `message.final` 始终包含候选数组和 history revision；失败、错误和取消不产生可激活候选。
- Engine fallback、事件 JSON、Last-Event-ID 重放和项目 Checkpoint 快照包含新字段。

### 14.2 前端确定性测试

- v4 SSE 解析接受合法字段，拒绝错误类型、超量数组、非法 revision 和旧 schema。
- reducer 按 run ID 暂存 final，并只在 completed 后形成终态候选。
- `run.started` 消费旧候选；失败、错误、取消不恢复；重复和迟到事件幂等。
- history hydration 与实时 reducer 对同一事件序列得到相同候选状态。
- 当前项目 revision 匹配、变化、未知、加载中和刷新失败的 selector 结果。
- 项目和会话切换、跨会话项目修改、Checkpoint 回退及恢复后的候选隔离。
- 文本、附件、DOM、资源引用、恢复输入和 disabled/readOnly 状态控制展示。
- Scope、模式、模型和技能变化不隐藏候选。
- 零至三个候选、平台标题、完整换行和容器自然增高。
- 点击、Tab、Enter、空格只填入并聚焦末尾，不提交或修改控制项。
- `Option/Alt + Digit1/2/3`、越界序号、Meta/Ctrl/Shift、IME、其他编辑器、弹窗和菜单冲突。
- `Escape` 当前聚焦周期隐藏、blur 恢复、候选/项目/会话变化重置。
- React 纯文本渲染与 contenteditable 纯文本写入，不执行候选 HTML。

### 14.3 真实浏览器验证（本轮豁免）

用户在 2026-09-14 的实现请求中明确“不需要做真实 Chromium 验收”，因此本轮不启动浏览器。以下条目保留为后续人工体验回归清单，不作为 TODO 10 本轮完成门槛：

至少验证：

- 右侧面板常见宽度和窄宽度下三个长候选完整可见，输入框自然增高且不遮挡控制栏；
- 鼠标、Tab、Enter、空格及 macOS Option 数字键完整路径；
- 中文输入法组合、slash/page/component 菜单、附件和 DOM 标记不被快捷键干扰；
- 新任务完成、失败、取消、刷新、SSE 重连、项目/会话切换和 Checkpoint 回退恢复；
- 项目内容或 history 查询暂时失败时不闪现未经确认的候选，恢复后正确出现。

### 14.4 真实模型观察

固定一个可用模型和项目快照，至少执行一次 Chat、Grill 和 Execute 成功结束场景，记录：

- 返回候选数量；
- 第一项是否自然且有价值；
- 各项是否重复；
- 是否延续当前任务并符合真实能力；
- 是否使用用户语言和直接命令句；
- 是否出现内部 ID、解释、Markdown、超长或无意义填充。

模型合理返回零项不视为功能失败。当前没有候选采纳分析链路，不设置采纳率门槛，也不据一次观察宣称质量提升。

## 15. 实施记录

### 15.1 后端与协议

- `finish` 保持 `message` 为唯一必填字段，新增可选 `suggested_next_inputs`；Runtime 对错误类型、空项、控制字符、空白、重复、超长及超量输入执行 best-effort 规范化，候选不进入 Completion Gate。
- Prompt Catalog 升级为 `2026-09-14.v6`，新增 `runtime.next-input-suggestions` 模块，并与 `finish` 的实际披露阶段共用条件。
- Run 新增不可变 `ProjectHistoryRevision`，Baseline 返回并持久化权威 revision；SQLite 通过 `0011_next_input_suggestions.sql` 一次性加入 `project_history_revision`。
- 公共事件直接升级为 v4。`message.final` 始终携带 `affected_targets`、`suggested_next_inputs` 和 `project_history_revision`；Runtime Outcome 与 Engine fallback 同步携带和规范化候选。

### 15.2 前端行为

- SSE 仅接受 v4；实时事件与历史 hydration 共用 started 消费、final 暂存、completed 激活、失败终态清除的候选状态机，并以 run ID 和 message ID 阻止迟到事件覆盖。
- Composer 在候选、项目 history revision、内容刷新、会话、Run 状态和完整草稿资格全部匹配后才展示建议；刷新失败使用普通 placeholder，后续成功可重新验证。
- 建议层使用完整换行的原生按钮，支持点击、Tab、Enter、空格以及 `Option/Alt + Digit1/2/3`；选择只写入现有 thread draft 并聚焦文本末尾。
- IME、额外修饰键、弹窗、菜单、其他可编辑控件会阻止快捷键；`Escape` 只隐藏当前聚焦周期，清空普通草稿后同一候选重新出现。

### 15.3 验证证据

- `go test ./...`：后端全量通过。
- `pnpm test`：前端 61 个测试文件、261 项测试全部通过；新增测试覆盖 v4 解析、规范化、Runtime/Engine fallback、SQLite round-trip、实时与历史状态机、迟到事件、快捷键冲突、纯文本与完整换行按钮。
- `pnpm tsc --noEmit` 与 `pnpm lint`：通过；lint 仅保留 5 条本需求外既有 Fast Refresh warning，无 error。
- Impeccable UI detector 对三个本次 UI 文件返回空问题列表。
- `RUN_DEEPSEEK_LIVE=1 go test ./internal/workflow -run TestDeepSeekLiveProducesBoundedNextInputSuggestions -count=1 -v`：Chat、Grill、Execute 均成功调用新 `finish` Schema，各返回 3 条中文直接命令，未出现内部 ID、Markdown、换行或无意义填充。
- 真实 Chromium：按用户明确要求未执行。

附带发现：仓库既有全工具 live schema 测试的 Execute 分支仍会被 DeepSeek 因 `mutate_ppt` 中某个空数组 Schema 拒绝；Plan 分支和本需求隔离后的 `finish` 三模式观察通过。该问题与新增候选字段无关，本轮未修改其协议。

## 16. 已确认 Grilling QA

以下记录是本设计的产品决策依据。后续实现如需改变其中任一项，应先更新设计，不得把工程便利视为默认授权。

| 编号 | 问题 | 用户确认的决定 |
| --- | --- | --- |
| Q1 | 候选是预测还是推荐？ | 采用混合语义：写成用户最可能直接发送的下一句话，同时优先有价值的后续动作。 |
| Q2 | 哪些结果生成候选？ | Chat、Grill、Execute 真正成功结束时生成；等待交互、失败和取消不生成。 |
| Q3 | 三个候选是否固定分类？ | 不固定类别；默认提供不同方向，只有自然需要时才集中到同一方向。 |
| Q4 | 使用纯 placeholder 还是可交互建议层？ | 使用输入框内、视觉类似 placeholder 的可交互建议层。 |
| Q5 | 数字快捷键的作用范围？ | 建议可见时在当前工作区生效，但避开弹窗、其他编辑控件、输入法和菜单。 |
| Q6 | 候选是否持久化？ | 附在最终消息事件上持久化，刷新、重连和会话切换后可恢复。 |
| Q7 | 候选异常时是否必须凑满三个？ | 不要求三个；允许零、一个、两个或三个，最多三个，且不得阻断任务完成。 |
| Q8 | 候选文本格式和长度？ | 单段纯文本，无编号、Markdown 或换行；每项最多 80 个 Unicode 字符。 |
| Q9 | 项目变化后是否失效？ | 绑定权威项目版本；项目创作状态变化即失效，纯 UI 状态不影响；Q34 进一步确定使用 Checkpoint history revision。 |
| Q10 | 新请求失败后是否恢复旧候选？ | 请求被服务端可靠接受后永久消费；新任务失败或取消也不恢复。 |
| Q11 | 文本为空但存在附件或引用时是否展示？ | 只有整个消息草稿完全空白时展示。 |
| Q12 | 候选可以推荐哪些内容？ | 优先延续当前任务和项目，只建议产品当前真实可执行的操作。 |
| Q13 | 历史时间线是否渲染候选？ | 不渲染；服务端仍随最终消息保存，供恢复和审计。 |
| Q14 | `finish` 候选字段使用什么结构？ | 使用按展示顺序排列的字符串数组，不增加类别、理由、Scope 或模式元数据。 |
| Q15 | 零至三个的数量由谁决定？ | 模型基于上下文自主决定，宁缺毋滥；程序只做格式校验与规范化。 |
| Q16 | 候选如何排序？ | 第一项是最自然且最有价值的延续，后续项提供不同方向备选。 |
| Q17 | 候选使用什么措辞？ | 跟随用户语言，采用可直接发送的命令句，使用可见页码或标题，不出现内部 ID 和解释。 |
| Q18 | 输入框聚焦后是否继续显示？ | 继续显示，直到出现文本或其他消息草稿资源。 |
| Q19 | 长候选如何展示？ | 初答选择单行省略；随后明确改为全部展示，以 Q22 和 Q32 的最终决定为准。 |
| Q20 | 是否提供“换一批”？ | 第一版不提供，不为此增加独立 Model Call。 |
| Q21 | 是否记录候选采用情况？ | 不记录；候选填入后就是普通草稿。 |
| Q22 | 省略文本如何查看全文？ | 用户撤销省略方案，改为候选全部展示，因此不需要悬停全文机制。 |
| Q23 | 是否允许主动隐藏建议？ | 使用 `Escape` 临时隐藏，具体生命周期由 Q33 确认。 |
| Q24 | Checkpoint 回退后如何处理？ | 按恢复后的事件历史选择最近有效候选，并校验恢复后的项目 revision。 |
| Q25 | 候选何时激活？ | 暂存 `message.final`，在 `run.completed` 且项目内容刷新确认版本后激活。 |
| Q26 | 项目状态刷新失败怎么办？ | 使用普通 placeholder；刷新成功并确认版本后再展示。 |
| Q27 | 候选是否进入 Tab 顺序？ | 每项可聚焦；Enter 和空格只填入，不发送。 |
| Q28 | 零候选时显示什么？ | 使用现有“输入你的想法与目标”。 |
| Q29 | `finish` 候选字段是否必填？ | 可选，允许零至三个；缺失、`null` 或解析失败按空数组处理。 |
| Q30 | 模型返回超过三个怎么办？ | 按原顺序规范化，只保留前三个有效候选。 |
| Q31 | 空白、控制字符、重复和超长如何处理？ | 规范化空白和控制字符后去重；超过 80 字符整项丢弃，不截断。 |
| Q32 | 完整展示时如何处理高度？ | 全部换行展示，输入框自然增高，不截断、不在候选层内部滚动。 |
| Q33 | `Escape` 隐藏到什么时候？ | 只隐藏当前聚焦周期，保持编辑器聚焦；blur 后重置，隐藏期间数字快捷键停用。 |
| Q34 | 使用哪个项目 revision？ | 使用项目 Checkpoint history revision，不使用资源 revision 指纹或 `updated_at`。 |
| Q35 | 会话 B 修改项目后，会话 A 是否保留旧候选？ | A 的旧候选失效；历史元数据保留，但不复制 B 的候选。 |
| Q36 | Scope、模式、模型或技能变化是否影响候选？ | 不影响；候选只预测输入文本，也不覆盖这些设置。 |
| Q37 | 是否升级公共事件 Schema？ | 从 v3 升到 v4，前后端和历史 hydration 一次性切换，不保留兼容分支。 |
| Q38 | Run 如何关联项目 history revision？ | 建立 Baseline 时取得并持久化到 Run，成功时写入 `message.final`。 |
| Q39 | 候选 Prompt 放在哪里？ | 新增独立 Runtime Prompt 模块，仅在披露 `finish` 时加载。 |
| Q40 | 候选是否进入 Completion Gate？ | 不进入；Runtime 独立规范化，不产生 Gate issue 或模型重试。 |
| Q41 | 历史如何重建当前候选？ | 使用事件状态机：started 消费、final 暂存、completed 激活、失败终态不恢复，再校验项目 revision。 |
| Q42 | 候选与临时隐藏状态归谁管理？ | 候选属于 Run Session/历史派生状态；`Escape` 隐藏只属于 CommandComposer 本地状态。 |
| Q43 | 测试和效果验收范围？ | 原决定为后端、前端、真实浏览器和真实模型观察；2026-09-14 最新实现请求明确豁免真实 Chromium，本轮已完成其余三项，合理零候选仍不算失败。 |
