# Agent 工作流开发规范

一次 Run 从 `POST /api/runs/append` 受理开始，由 `RunService` 持有执行状态，交给 `HarnessLoop` 推进，最后由 `RunService` 提交终态。调用方在受理事务提交后拿到 Run ID，在 `tool_call` 提交后查到工具调用，只有在文件变更和对应的 `tool_result` 均提交后才能读到修改成功的结果。[主循环图](WORKFLOW.html#main-loop) · [类职责表](WORKFLOW.html#class-catalog)

## 1. 受理与执行材料

### 新开 Run

1. `ExecutionController` 调用 DTO 的 `toEntity()` 转换请求，`RunService` 协调 `InputService` 按全局唯一的 `message_id` 检查归属。同一会话同参重发返回原 Run ID，跨会话复用或异参返回 `IDEMPOTENCY_CONFLICT`。先识别重发，再对新请求做占用和分支校验，避免网络重发创建第二次执行。
2. 校验所属会话、模式、模型、引用和授权范围。同一会话最多一个非终态 Run，且与本会话的手动压缩互斥。同项目不同会话可以并行，但新 Run 须避开项目历史切换、Git 提交等排他操作。处于回退位置时，同时核对 `branch_revision`。
3. 在项目写锁内，`CheckpointService` 从同一次一致读取取得 Run 开始前的会话数据和五类文稿文件。这份读取同时提供用于历史恢复的持久检查点和本 Run 审查用的内存文件基准。`RunService` 从 `SettingService` 取得全局设置，按本次模型选择和路由组装 `RunConfig`，固定主模型、旁路及备用模型、两项审批开关和自动命名／建议开关，再将配置和审查基准放入本次 `RunState`。
4. `IRunRepository` 在同一 SQLite 事务中写入普通检查点、`pending` Run、首条 `queued` 指令和 `create_run` 凭据，同时更新会话版本。需要放弃返回入口时，在该事务内清理。事务失败不调度执行。
5. 执行材料就绪且事务成功后，`RunService` 释放项目写锁，调度后台循环。耗时的模型请求与人工等待均在锁外进行。[执行接口](API_SPEC.md#执行) · [检查点](DATABASE_SPEC.md#checkpoints) · [提交凭据](DATABASE_SPEC.md#receipts)

| 材料 | 生存期与用途 |
|---|---|
| 普通检查点 | 保存 Run 前的业务与文件状态。历史切换可以使其失效。 |
| `RunState` 的固定材料 | 模型配置、备用路由、执行开关、审查基准固定一次，整次 Run 复用。检查点失效后仍可审查。 |
| 每轮执行材料 | 使用当前模式、已获授权、待处理输入和最新上下文组装请求。`RunState` 保存当轮实际路由及最近一次完整请求的估算。 |
| `runs.changes` | 在收尾时汇总本 Run 成功提交的净修改。每次提交就记录归属，避免将并行 Run 或人工写入算入本 Run。 |

### 追加输入

追加使用同一入口，传 `run_id`，沿用该 Run 的模式与模型。仅 `running` 或 `waiting` 且输入仍开放时接收新消息。已受理消息的同参重发返回受理该消息的 Run ID，即使追加入口已关闭。原 Run 已被回退移除时，重复请求返回 `RUN_NOT_FOUND`。

`InputService` 将原文、有序引用、显式 `scope` 和会话内 `seq` 写入 `messages`。DOM 引用保留提交时的 `DomSelection`，提示词拼装和回退回填均使用该选区。新开和追加均按受理时的大纲展开授权。`section_ids` 取章直属页面和下属各节的页面，`subsection_ids` 只取所选节的页面，页序沿用大纲。追加省略范围时沿用当前授权，显式提交时与现有授权取并集，`null` 放开本项目全部幻灯片。消息与 `runs.scope` 的变更同事务提交。引用本身不扩大授权。等待审批期间收到的追加授权也立即生效。[指令持久化](DATABASE_SPEC.md#messages) · [请求字段](API.html#runs)

## 2. 主循环与生命周期 Hook

1. **取输入。** `RunGuard` 检查取消、轮次与活动时间预算、连续失败和重复无进展等停止条件。人工等待时间从活动时间中扣除。`InputService` 按 `seq` 处理已受理输入，对每条调用 `beforeInputProcessed`，应用判定并保存处理状态。
2. **准备请求。** `ContextService` 准备会话与项目材料，`ToolRegistry` 取当前模式、意图及授权允许的工具。主循环调用 `beforeModelCall`，用返回的工具子集组装请求，保留 Assistant 与工具结果的配对和顺序。达到 `threshold_max` 时通过 `CommandService` 发起自动压缩，压缩成功后重新准备请求。
3. **确认发送边界。** 发送前，`HarnessLoop` 与 `InputService` 检查准备、裁剪或压缩期间到达的新输入。有新输入则回到第 1 步，处理新增指令并重建请求。检查之后才受理的输入留到下一轮。
4. **调用模型。** `HarnessLoop` 将组装好的 `LlmRequest` 交给 `ILlmGateway`，从 `LlmResponse` 读取正文、推理内容、有序工具调用及各次实际请求可取得的用量。基础设施实现 `LlmGateway` 使用 `OpenAiResponsesChatModel` 对接 `responses`，使用 `AnthropicChatModel` 对接 `anthropic`，并关闭 Spring AI 的自动工具执行。请求允许有限重试，切换备用模型时按目标协议重建请求。返回工具调用时交给 `ToolExecutor`，将配对结果带入下一轮。
5. **尝试完成。** 得到候选回复后，主循环原子封闭追加入口并检查剩余输入。有已受理输入时恢复入口并继续处理。队列已清空时才准备 `BeforeRunCompletedEvent`。完成条件未满足就反馈具体缺项、恢复受理并继续。满足时保持入口关闭，将收尾交给 `RunService`。[主循环图](WORKFLOW.html#main-loop) · [模型与目录](STRUCTURE.html#dependencies-heading)

### 上下文估算与实际用量

首轮请求组装完成前，会话 `context` 沿用该会话的空闲 Create 估算。每轮完整组装后，`ContextService` 将总量与分类明细封装为 `ContextEstimate`，整体替换 `RunState` 中的上一份估算。下一轮准备期间读取上一份完整值。Run 结束后恢复空闲估算。`runs` 的用量只累计主循环实际发出的对话请求及重试，每次计一次，未报告 Token 数的请求仍计入 `call_count`。JEV、旁路命令和独立 Reviewer 调用均排除，取消保留已发生的用量。[会话统计](API_SPEC.md#会话) · [Run 用量](DATABASE_SPEC.md#runs)

### 每条输入的判定与去向

`IntentHook` 只接受 [0, 1] 内的有限数值作为评分，先校验必需的 `disclosure` 和 `bypass`。JEV 调用失败或任一必需评分无效时，按技术失败结束 Run。两项有效后依次检查 `disclosure ≥ 0.5`、`bypass ≥ 0.5`、`irrelevant ≥ 0.8`，取首个命中类别。可选的 `irrelevant` 缺失或无效时跳过。均未命中时，仅当 `intent.choice=gossip` 且 `confidence`、`probabilities.gossip` 都是 [0, 1] 内的有限数值且均 ≥ 0.8，才判为 `gossip`。其余判为 `task`。

| 结果 | 当前输入 | 后续执行 |
|---|---|---|
| `task` | 按当前模式组装提示词和授权工具。 | 继续当前任务。 |
| `gossip` | 同一主循环、同一模型路由，使用 `ChatPromptAssembler` 的闲聊分支，只提交 `prompts/gossip.md` 与该条输入，工具为空。 | 纯闲聊正常完成为 `done`。任务中插入的闲聊答复记为 `commentary`，随后恢复原任务。 |
| 首条命中拒绝类别 | 保存判定与拒绝回复，将 Run 收尾为 `rejected`。 | 关闭追加入口。已受理但未处理的输入以 `queued` 留在原 Run 历史，排除在模型上下文之外。 |
| 追加命中拒绝类别 | 在原指令上写 `instruction_decision`、`instruction_status=sent`、`context_position=NULL`，递增版本并通知 `history`。正文和引用只留在历史。 | 跳过这一条，保留已生效的授权，同一 Run 继续按序处理后面的输入。 |

例如 A、B、C、D 先后受理，A 是首条，B 被拒。历史保留 B 及拒绝原因，模型继续收到 C、D。若 A 被拒，执行在 A 处结束。[输入处理](API_SPEC.md#执行) · [闲聊拼装](ASSEMBLE.html#gossip-title)

### Hook 的调用方和结果

`HookDispatcher` 为前三个阶段分别创建初始结果，按固定顺序传递 `currentResult`。同步契约为 `SyncHook<E, R>` 的 `R execute(E event, R currentResult)`，每个 Hook 返回新的不可变结果并保留前序信息。异步契约为 `AsyncHook<E>` 的 `void execute(E event)`。阶段含异步动作时，Dispatcher 完成投递后将最终同步结果返回调用方。事件及集合均不可变，异步 Hook 只接收事件。执行器由 `bootstrap` 的 `ExecutorConfiguration` 装配。[Hook 流程图](WORKFLOW.html#hook-flow)

| 入口 | 同步结果 | 由谁应用、失败如何处理 |
|---|---|---|
| `beforeInputProcessed(BeforeInputProcessedEvent)` | `IntentHook` → `BeforeInputProcessedResult` | `InputService` 落实判定。输入开始处理时另按规则投递 `AutoRenameHook`。命名条件独立判断，拒绝判定本身不统一短路命名。 |
| `beforeModelCall(BeforeModelCallEvent)` | `ToolPruneHook` → `BeforeModelCallResult` | 仅覆盖 Harness 主模型请求。初始结果为已允许的工具，空集直接沿用。裁剪不可用时保留进入该 Hook 前的集合。执行时仍由 `ToolGuard` 校验。 |
| `beforeRunCompleted(BeforeRunCompletedEvent)` | `CompletionHook` → `BeforeRunCompletedResult` | 初始为尚未确认完成。检查任务所需的计划、授权、产物及明确要求的截图等。Plan 任务须经计划审批。纯闲聊可直接完成。检查异常不能按通过处理。输入开关、继续循环及终态提交由主流程决定。 |
| `afterRunCompleted(RunCompletedEvent)` | 返回 `void` | 仅在成功完成事务提交后投递 `AutoSuggestHook`。事件带触发 Run 的最后输入 ID、固定开关与路由。`RunService` 交付材料后释放 `RunState`。 |

`ToolPruneHook` 分别按 `meta → edit_meta`、`constraint → edit_constraint`、`outline → edit_outline` 处理三道 JEV 选择题。每题可选 `needed`（任务需要）、`possible`（可能需要）或 `not_needed`（整个任务都不需要）。只有该题同时满足 `choice == "not_needed"`、`probabilities.not_needed ≥ 0.95` 和 `confidence ≥ 0.85`，才从本轮候选集中移除对应工具。其余情况保留该工具。单题答案缺失或无效时，继续处理其他题的有效答案。整次调用失败时沿用进入 Hook 前的候选集。[Prune 拼装与裁剪规则](ASSEMBLE.html#prune-title)

阶段事件仅在领域内传递。JEV、旁路命令及独立审查不递归触发主模型 Hook。异步命名和建议经过 `CommandService` 的正常受理校验，调度或执行失败保持已提交的主流程结果。

## 3. 工具执行、人工等待与提交

主循环将 `LlmResponse` 中的 `ToolCall` 按模型给出的顺序交给 `ToolExecutor`。执行器先把调用 ID、原始工具名和参数写入 `tool_call` 消息，再解析 `ToolName`。未知名称走工具校验失败流程。已识别的调用经 `ToolGuard` 核对参数、可见工具、模式、调用身份与授权后，按 `ToolRegistry` 的映射分派给 `AgentTool`。16 个工具名对应 11 个实现，其中五个编辑工具共用 `EditResourceTool`，两个资产加载工具共用 `LoadAssetTool`。独立只读调用可并行，配对和返回顺序沿用模型原调用。重复的 `(run_id, tool_call_id)` 沿用已有调用记录和已确认结果。缺少结果记录时，不能据此重新执行可能已产生副作用的工具。[工具执行图](WORKFLOW.html#tool-flow) · [工具详情字段](API.html#history)

### 人工答复如何生效

`InteractionService` 用 `run_id + tool_call_id` 定位内存中的 `PendingInteraction`，表单的 `version` 绑定本次展示的内容。答复只受理一次，已关闭或已答复返回 `INTERACTION_CLOSED`，仍待答但版本变化返回 `STALE_INTERACTION`。空响应体表示答复已受理。执行在原 Run、原调用中继续，最终结果从 `GET /api/tools/{message_id}` 读取，参数是 **tool_call 消息 ID**。[人工接口字段](API.html#interactions) · [人工交互契约](API_SPEC.md#人工)

| 表单 | 答复生效后的行为 |
|---|---|
| `question` | 按题保存选项、自填文本或跳过结果，唤醒原调用。 |
| `plan` | `create_plan` 获批时，同一事务提交会话计划、配对结果、Run 从 `plan` 切换到 `create` 及会话版本。拒绝时保留原模式和已有计划，继续规划。 |
| `permission` | `allow_once` 只批准已展示的命令及候选差异。申请期间不持项目写锁，真正应用前复核基准和权限。 |
| `scope` | 批准将提议合并到当前授权，拒绝保留现有范围，`revise` 放开本项目全部幻灯片。追加授权完全满足申请时关闭事项，保留 `answer=null` 与实际 `applied_scope`。部分满足时更新提议和版本。 |
| `resource` | `approve` 采用完整 `draft`，省略则采用原 `proposal`。`reject` 保留原文件。批准后的定稿仍须通过基准检查。 |

Create 中的 `update_plan` 直接替换生效计划的步骤。非空数组使用 `pending / processing / completed`，最多一步 `processing`。`steps=null` 清空 `sessions.plan`，缺少字段或空数组均无效。计划、配对的工具结果与版本一起提交。[会话计划](DATABASE_SPEC.md#sessions)

### 修改文稿的七步提交顺序

权限审批为每项候选修改生成 `DeckChange`，用 `ChangeTarget` 标明文件，并附上变更状态和差异明细。提交成功后，工具结果记录实际提交的 `DeckChange`。`RunChangeSet` 按目标合并本 Run 成功提交的变更，抵消本 Run 自身撤销的改动。`review_ppt` 读取截至审查时的净修改，`RunService` 收尾时将最终净修改写入 `runs.changes`。[目标字段](API_SPEC.md#消息)

项目文稿的多文件修改、检查点和会话版本必须对应同一次提交。人工保存、Agent 写入、采集检查点及历史切换共用项目写锁。普通项目 GET 无需获取项目锁。每个文件用同一份字节生成正文和哈希，允许跨文件读到不同提交的内容。模型调用、候选准备和人工等待在锁外执行。[项目目录与日志](STRUCTURE_SPEC.md#项目目录) · [目录树](STRUCTURE.html#project-heading)

1. **记录调用。** `ToolExecutor` 通过 `IRunRepository` 在 SQLite 事务中写入原始 `messages.type=tool_call`，递增所属会话版本。提交后即能查询这次尝试。
2. **准备候选。** `DeckService` 为每个待修改的文稿文件准备一份 `DeckEditCandidate`。`run_command` 的候选来自 `IBashGateway` 返回的沙箱修改。需要审批时，`InteractionService` 在等待期间保留候选。资料审批若提交完整 `draft`，以该定稿替换候选内容，沿用原读取基准。Chat、Ask、Plan 中只读。Create 执行阶段仅能提交授权范围内的项目资料（`.meta.json`、`.constraint.json`、`.outline.json`）和页面规格／源码（`{slide_id}.json`、`{slide_id}.html`）。辅助文件留在沙箱，`.git/`、`.runtime/` 受保护。
3. **持锁复核。** 拿到项目写锁后，复查取消状态、当前授权、获批候选及文件基准。基准冲突时保留当前文件，按工具失败处理。人工保存在同一提交边界校验项目占用和 `base_hash`。项目有非终态 Run、正在 Git 提交或历史切换时返回 `PROJECT_BUSY`。导出采集快照时只短暂持锁，锁释放后即可保存。
4. **先写日志。** `DeckRepository` 将原凭据键、请求摘要及文件修改前后的状态组装为 `MutationLog`。由 `PathResolver` 定位 `.runtime/mutations/{operation_hash}.json`，通过 `FileStorer` 原子写入完整日志。确认日志落盘后才改动项目文件。
5. **应用文件。** `DeckRepository` 持锁通过 `FileStorer` 新建、替换或删除候选文件，记录本次来源及实际差异。资源编辑使用完整候选文档。
6. **提交数据库。** 同一 SQLite 事务写入 `agent` 凭据、配对的 `tool_result` 及实际差异，使受影响的检查点失效，每个受影响会话的 `revision` 递增一次。人工写入使用 `manual` 凭据并提交相应业务变更。两种文件凭据的 `receipts.result` 都为 SQL `NULL`。[凭据字段](DATABASE.html#receipts) · [事务约束](DATABASE_SPEC.md#receipts)
7. **收尾与通知。** 数据库提交成功后清理日志、释放锁。`ToolExecutor` 从已保存的结果生成配对的模型 observation。结果可读后发送变化通知。只读结果和确定的受阻或失败结果由 `ToolExecutor` 保存。

`agent` 的凭据键是 `(agent, run_id, tool_call_id)`，`manual` 的凭据键是 `(manual, project_id, 后端操作 ID)`。若文件已替换而进程在第 6 步前后退出，重启时用原凭据键核对摘要。凭据匹配则补齐新版，确认无凭据则还原旧版，读取失败或摘要不符则保留日志、阻止该项目后续写入。日志收尾前保留凭据。

资料编辑成功后，模型收到的 `content` 和工具详情的 `result.content` 均取实际定稿。基本信息与约束返回对象，大纲返回含正式节点 ID 的 JSON 源码字符串。历史保留原 `request.proposal` 和审批决定，受阻或失败时省略 `content`。[工具结果契约](API_SPEC.md#消息)

### 错误由哪一层处理

| 情况 | 处理 |
|---|---|
| 参数、授权、基准或渲染等产生可反馈的工具失败 | 工具返回带错误码、原因及修正提示的 `ToolResult`，或由 `ToolExecutor` 转换已识别的 `BusinessException`。保存对应的 `blocked` 或 `failed` 工具结果，向模型提供配对反馈。 |
| 文件或数据库提交结果不确定、内部状态损坏、未知异常 | 传播到 `RunService` 处理运行失败。持久化结果尚不能确认时保留日志和凭据，留待核对。不把未知结果当成工具失败继续循环。 |
| 用户取消 | 取消请求只作用于指定 Run，可重复提交。`RunService` 协调停止在途执行并关闭待答项。保留已确认的写入、结果和用量，未处理输入维持 `queued`。 |

工具专属原因归 `ToolErrorCode`，运行协调错误归 `HarnessErrorCode`，跨领域错误沿用 `shared/exception/ErrorCode`。HTTP 的 `GlobalExceptionHandler` 把已识别业务异常映射为 `ErrorResponse`。工具边界将相同业务错误码保留在工具结果中。[领域目录](STRUCTURE.html#domain-package-heading)

## 4. 命令、提示词拼装与模型调用

`CommandService` 统一受理手动命令、自动命令和修订。先按会话与 `request_id` 识别重发，再校验占用、分支确认和修订基准。`ICommandRepository` 在同一事务写入执行记录与 `create_command` 凭据，提交后才分派对应的 `CommandHandler`。每个执行有独立 ID，修订沿用 `operation_id`，仅 `polish`、`handoff` 可基于最近成功执行的 `base_id` 修订。进度与取消都针对单次执行。[命令流程图](WORKFLOW.html#support-flow) · [命令持久化](DATABASE_SPEC.md#commands)

| 命令 | 执行与提交条件 |
|---|---|
| `rename` | 自动命名开启时，首条会话输入遇到默认标题「新会话」，可直接发起首次命名。其余输入由 `AutoRenameHook` 调用 JEV，`rename` 评分有效且 ≥ 0.85 时发起命名。评分无效或调用失败时跳过。写回时仍须匹配受理时的原标题。 |
| `suggest` | 自动建议受理前核对触发 Run 的最后输入。写回时依据的输入须仍为最新，且本次执行的 `seq` 须为会话中所有 `suggest` 的最大值。 |
| `polish`、`handoff` | 各自的 Handler 使用原文或交接材料，修订时加入上次成功产物和反馈。修订失败保留之前的成功产物。 |
| `compact` | 手动压缩与本会话新 Run 互斥。自动压缩由主循环协调，经 `CompactCommandHandler` 调用 `ContextCompactor`。摘要写入、`context_position` 调整与命令 `done` 同事务提交，保留历史正文和完整工具轮次。 |
| `commit` | 项目内须无非终态 Run。`CommitCommandHandler` 经 `IGitGateway` 暂存、取得差异、由模型分析，再执行并核实 Git 提交。已确认的提交保留为 `done`，无法确认时标记 `interrupted`。 |

`CommandService` 在同一事务核对写回条件、保存产物和关联业务变更、提交命令终态。过期的 `rename` 或 `suggest` 写为 `cancelled`、`result=NULL`。处于可返回最新的位置时不自动发起旁路命令。手动命令在自身受理时取配置，自动命令使用触发 Run 已固定的配置。

### 提示词拼装与模型调用

`ContextService` 根据 Chat、Ask、Plan、Create 选择对应的 `PromptAssembler`，将会话历史与当轮材料按拼装顺序转换为 Spring AI 的 `List<Message>`。一次模型响应中的全部工具调用都有配对结果后，整轮才进入会话上下文，按原序保留调用 ID。若 Run 中断时仍缺结果，下次请求及压缩跳过整轮，历史仍可查询每个调用和已取得的结果。这组消息和本轮工具定义组成 `LlmRequest`，由 `HarnessLoop` 交给 `ILlmGateway`。六种命令使用各自的拼装器，由 Handler 或 `ContextCompactor` 调用模型。对可程序校验的格式、长度、重复项及业务约束错误，处理器在该命令上下文内有限纠错，受取消与超时约束。只有校验通过才提交产物。传输重试由网关负责。[命令处理与拼装图](WORKFLOW.html#handler-assembler-flow)

`review_ppt` 由 `ReviewPptTool` 使用 `ReviewPromptAssembler`，结合用户要求、Run 的内存审查基准、本 Run 已提交的差异和当前源码独立审查。使用该 Run 固定的主模型和 `main_fallback`，备用配置为 `null` 时关闭备用。旧截图只证明调用当时的页面，任务明确要求检查新版时须取得对应版本的检查结果。

JEV 的 Intent、Prune、Rechristen、Preview 分别由 `IntentHook`、`ToolPruneHook`、`AutoRenameHook`、`RenderSlideTool` 使用对应拼装器后调用 `IJevGateway`。Rechristen 判定是否需要改名，生成标题交给 Rename 命令。`RenderSlideTool` 将评分结果与复用基准封装为 `PreviewAssessment`，随工具结果保存。评分完成时记录三项分数和独立的 `PreviewMismatch`、`PreviewIssue` 判定。评分不可用时保留截图，记录 `unavailable` 和原因。[提示词拼装](ASSEMBLE.html) · [渲染文件](STRUCTURE_SPEC.md#项目目录)

`JevGateway` 统一通过 HTTP 向 `POST /v1/systemone` 发送判定请求。纯文本使用 `application/json`。含截图的 Preview 使用 `multipart/form-data`，第一部分 `request` 是 JSON，`assets` 按分段名引用后面的截图文件，请求头携带 `TypeSafe-Danger-Zone-Images: true`。[JEV 图片协议](https://docs.typesafe.ai/images)

`SlidePreviewService` 通过 `IFileGateway` 保存历史截图。切换页面的最新截图时，先删除旧缩略图，再原子替换渲染索引。删除失败时保留原索引。[预览文件约定](STRUCTURE_SPEC.md#项目目录)

## 5. 收尾、历史与实时通知

`RunService` 通过 `IRunRepository` 将最终回复、`runs.changes` 和终态一起提交。`done` 与首条被拒的 `rejected` 保存对应回复。`failed`、`cancelled`、`interrupted` 可没有最终回复。每个 Run 最多一条 `response`。失败或取消照实保存此前已提交的净修改。无净变化为 `[]`，无法确定为 `NULL`。成功完成事务后才投递完成 Hook，所有终止路径均释放本 Run 的内存材料。[Run 持久化](DATABASE_SPEC.md#runs)

`HistoryService` 从持久记录返回历史分组、执行详情和差异。工具调用先可查询、结果后可查询，因此 `result=null` 仅代表尚无已记录结果。四类历史命令为 `polish / handoff / compact / commit`。`rename` 更新会话名称，`suggest` 更新会话建议。分组的 `command.id` 指向最新执行，`last_success_id` 指向最近成功产物。阶段只报实际进入的 `phase`，无活动阶段或终态为 `null`。`show_tool_failures` 即时控制展示，失败记录仍持久化。[历史与阶段编号](API_SPEC.md#消息)

项目和 Agent 服务通过 `domain/shared/sse/EventPublisher` 指定投递目标并发布 `SseEvent`。`interfaces/sse/SseEventPublisher` 管理连接，将具体事件映射到 SSE 的 `event` 行，将该事件的数据编码为 `data` 行的 JSON。断线后后台执行继续。[订阅字段](API.html#history)

| 具体事件 | SSE 事件名 | `data` |
|---|---|---|
| `SseEvent.ProjectChanged` | `project_changed` | 仅含非空、无重复的 `areas`（`ProjectChangeArea` 数组）。 |
| `SseEvent.SessionChanged` | `session_changed` | 接收会话的 `revision`（十进制字符串）和非空、无重复的 `areas`（`SessionChangeArea` 数组）。 |
| `SseEvent.RunProgress` | `progress` | `run_id`、`activity`、`commentary`。后两项可为 `null`，`activity` 与 Run 详情共用 `RunActivity`。 |
| `SseEvent.InteractionChanged` | `interaction` | `run_id`。接收方重读该 Run 的待答项。 |
| `SseEvent.CommandProgress` | `command_progress` | 单次命令执行的 `id` 和当前阶段序号 `phase`（整数）。 |
| `SseEvent.Reset` | `reset` | 空对象 `{}`。接收方废弃旧快照和分页游标后重新同步。 |

| 变化 | 接收方及重新读取的数据 |
|---|---|
| 项目会话列表变化 | 同项目所有已连接会话收到 `project_changed` 的 `sessions` 区域，重读项目会话列表。 |
| 项目外观实际变化 | 同项目所有已连接会话收到 `project_changed` 的 `appearance` 区域，读取项目外观，必要时用 ETag 复验 Logo。 |
| 人工保存、Agent 写入或历史切换实际改变共享文稿 | 同项目所有已连接会话（含来源会话）收到 `project_changed` 的 `content` 区域，读取文稿。 |
| 工具过程、最终回复、净差异或终态 | 所属会话读取 `run`。影响历史分组时同时通知 `history`。命令修订或结果改变分组时通知 `commands` 与 `history`。 |
| 计划、上下文估算或检查点 | 只通知相应数据受影响的会话。`session_changed` 使用接收会话自身的 `revision`。上下文估算更新可发送同版本通知。 |
| 历史切换、重连 | 历史切换向受影响会话发送 `reset`。收到 `reset` 或重新建连后废弃旧快照与游标，按订阅契约重读。 |

## 6. 回退、返回与启动恢复

1. **核对预览。** 客户端用相应预览的 `revision` 提交 `expected_revision`。`CheckpointService` 在项目写锁下复核目标和会话版本。项目有非终态 Run、目标会话有非终态命令或项目处于互斥操作时拒绝切换。
2. **准备恢复材料。** 从目标快照取得五类文稿文件，以及目标会话的标题、计划、消息、Run、命令和 `agent` 凭据。首次回退另从当前状态采集返回快照，连续回退沿用原返回行。目标快照决定文稿文件的内容和存在状态，即使磁盘经过外部编辑也按快照覆盖。项目主题、装饰位置和 Logo 沿用当前值。附件、历史截图和其他会话的业务历史保留。
3. **先写日志，再改文稿。** 有文件变化时，在 `.runtime/mutations/{operation_hash}.json` 记录 `type=switch`、目标 `session_id`、锁内核对的 `base_revision` 及各变更路径的前后字节或缺失状态。完整日志原子替换并同步到磁盘后才增、改、删文稿文件。文件及目录项全部落盘后再提交数据库。仅涉及数据库的切换直接进入第 4 步。[日志字段](STRUCTURE_SPEC.md#项目目录)
4. **提交 SQLite。** 同一事务恢复目标快照内的会话业务记录，并按 `session_id` 和 `revision = base_revision` 更新目标会话，恰好命中一行并将版本加一。首次回退在此事务插入 `run_id IS NULL` 的返回行，返回最新时删除该行。只有共享文稿文件实际变化时，才清除同项目其他会话的检查点及返回行，并将实际失去恢复点的会话版本各加一。会话建议从恢复后的输入和命令读取。[检查点事务](DATABASE_SPEC.md#checkpoints)
5. **收尾并通知。** 确认数据库提交且文件与目标一致后，持久移除日志，释放项目写锁，再按第 5 节通知。回退成功响应带可回填的原始输入。回退与返回都不续跑历史 Run。提交失败或结果未知时，按下表核对文件后才能开放后续写入。[检查点接口](API_SPEC.md#检查点)

从锁内核对版本到日志持久移除，其他会推进目标会话版本或删除该会话的操作也须经过同一项目写锁。这样，即使 SQLite 已提交而日志尚在，恢复时读到的版本也只可能由本次切换推进。目标会话的版本始终向前递增，不从快照还原。

历史位置新开 Run、发起非 `polish` 命令或修订 `handoff`，须在受理事务中核对 `branch_revision` 并清理返回行及脱离当前历史的检查点。`polish` 保留返回入口。手动改名和删除计划使用 `expected_revision`。实际改名或删除计划成功后放弃返回入口，同名或失败时保留。无会话归属的人工保存实际改变共享文稿文件时，项目内**所有会话**的检查点和返回行失效。历史切换保留原有的 `create_run`、`create_command` 受理凭据。[检查点事务](DATABASE_SPEC.md#checkpoints)

新建导出同样需要采集一致的项目数据。项目内有非终态 Run、正在 Git 提交或切换历史时，创建返回 `PROJECT_BUSY`。`ExportService` 在项目写锁内读取文稿，并从同一数据库读事务取得外观和项目标题，组装不可变的 `ExportSnapshot`。快照含项目管理名称、页序、逐页标题与 HTML、装饰位置及 Logo 字节。已选主题时同时读取并固定主题 CSS。与预览相同，`deck_title` 取 `Meta.title`，每页的 `section_title` 取所属章标题，两者都在锁内固定。后台渲染和打包在释放锁后使用这份快照，按基础样式、可选主题 CSS、单页样式的顺序应用。[导出接口](API_SPEC.md#导出) · [项目读取约束](DATABASE_SPEC.md#projects)

### 启动恢复

桌面客户端的每个工作目录由单个后端进程管理。启动时检查公共 SQLite 数据库，再逐项目恢复。

1. `RunRecoveryService` 协调 `DeckService` 在项目写锁下恢复文稿。`DeckRepository` 读取遗留的 `MutationLog`，按下表核对普通写入的 `receipts` 或历史切换的 `sessions.revision`。确定版本后，通过 `FileStorer` 恢复文件，确认落盘后再持久移除日志。重复恢复只校正文稿文件，会话版本保持 SQLite 已提交的值。
2. 将遗留非终态 Run 和命令标为 `interrupted`，保留已确认的工具结果、文稿修改、用量和 Git 提交。不恢复旧循环，也不重放结果未知的工具或 Git 操作。
3. 通过 `ExportService` 和 `IFileGateway` 清理遗留导出任务目录。原导出输入只存在于任务内存，重启后原任务失效。
4. 所有项目都已检查、可恢复项目完成上述步骤后，后端报告就绪。仅有未解决日志的项目继续暂停写入。公共数据库不可用时，整个后端保持未就绪。[启动恢复图](WORKFLOW.html#support-flow) · [项目文件布局](STRUCTURE.html#project-heading)

| 日志类型（`MutationLogType`） | 从 SQLite 读到的结果 | 文件处理 |
|---|---|---|
| `agent` / `manual` | 原凭据存在，且请求摘要匹配 | 将各路径恢复到 `after`。 |
| `agent` / `manual` | 查询成功，原凭据不存在 | 将各路径恢复到 `before`。 |
| `switch` | 目标会话的 `revision = base_revision` | 切换尚未提交，将各路径恢复到 `before`。 |
| `switch` | 目标会话的 `revision = base_revision + 1` | 切换已经提交，将各路径恢复到 `after`。 |
| 任一类型 | 查询失败、摘要不符、目标会话缺失或版本超出上述两值等无法判定的情况 | 保留日志，阻止该项目写入。 |

每条路径按所选状态创建、替换或删除。任何文件未能恢复时保留整份日志，以便重试。存在未解决日志的项目停在第 1 步，继续检查其余项目。该项目恢复成功后再中断遗留执行、清理导出目录并开放写入。运行期间也须等前一次文稿操作收尾，才能开始该项目的下一次写入或历史切换。[文件布局](STRUCTURE_SPEC.md#项目目录) · [提交判据](DATABASE_SPEC.md#checkpoints)

## 实现位置与命名

领域服务、主循环和 Guard 保持具体类。领域 `Repository` 及以 `Gateway` 命名的接口加 `I` 前缀，单实现去掉 `I`，多实现按实际特征命名。`AgentTool`、`EventPublisher`、`SyncHook`、`AsyncHook`、`CommandHandler`、`PromptAssembler` 沿用图中名称。业务仓储可以协调多表和文件系统，调用方按一次业务提交使用仓储。[类职责与实现](WORKFLOW.html#class-catalog) · [领域目录](STRUCTURE.html#domain-package-heading) · [接口目录](STRUCTURE.html#interface-package-heading)
