# 模型工具协议统一与恢复边界

本次实现以根目录 `TODO.md` 的已确认需求为准。模型实际输入、输出以 [TOOL.md](../../TOOL.md) 为接口说明；本文只补充实现、内部状态和恢复规则。本文覆盖此前资源工具、计划工具、页面扩权、截图按需读取、运行期图片保留及 JEV 文档中与本次协议冲突的部分。旧文档中的过程记录保留，不构成兼容接口。

## 模型与内部结构的分界

- `ToolResult.OK`、Data、ChangedTargets、证据、资源指纹、图片索引和命令统计仍用于后端执行、UI 事件、审计及幂等回执。它们不是直接发送给模型的协议。
- 发送模型前使用工具专属 Observation / ObservationParts 或通用业务投影；递归剔除 `ok`、资源哈希及内部定位信息。Git 提交标识 `hash` 保留。
- `edit_html` 只返回保存摘要。内容预检依旧执行并保存在内部回执、检查点和前端 `tool.content_prechecked` / `tool.completed` 事件中，不再附加到模型工具回复。
- 不保留旧工具别名、旧参数、旧审批枚举和步骤状态兼容分支，不迁移历史开发数据。前后端需一起更新并重启。

## 资源版本与原子编辑

`read_resource`、资源编辑结果以及完整资源上下文使用不进入供应商模型消息正文的 `MessageMetadata.Resources` 记录版本。Runtime 将真正提供过的版本保存在 `seen_versions` 检查点字段中，文字压缩不会清空这份授权依据。

请求准备阶段按新增消息元数据识别新版本，不能依赖消息数组长度：图片去重可能删除旧恢复消息。已有上下文不得重新覆盖检查点中更新的已见版本。

Manifest、Design 和单页 Spec 使用规范化 JSON 内容指纹；Outline、HTML 使用原始源码字节指纹。包含展示序号的 Outline 上下文不是原始源码，不作为精确替换依据，需要先读取。完整单文件 `cat` 成功且输出未变换、未截断时也记录文件版本及对应 PPT 资源版本；部分读取、搜索和截断输出不能授权覆盖整个文件。

所有资源编辑使用模型已见版本，不在写入前读取新版本来替代它。已有但未读的资源或版本冲突均要求重新读取。缺失资源可以首次创建；Outline 的 `init` 仍独立检查文件不存在，已有空大纲也不能重新初始化。`run_command` 的 `sed -i` 在审批前及执行时校验模型所见版本和审批前版本。

运行期间复用项目级写锁，与 UI 编辑和历史切换共享排他边界。模型版本校验、暂存、提交基线校验和写入在这一边界内完成；源码与关联删除通过现有 mutation buffer、RunSession 和 mutation journal 原子提交。恢复使用相同幂等回执与日志，不添加数据迁移。

## 图片与渲染

`render_slide` 返回诊断文字块与真实图片块。即使诊断阻断完成，只要截图有效仍保留截图；通用错误绑定不得将它替换成内部 Data。诊断仅公开尺寸溢出说明、越界元素、控制台错误和失败资源。Worker 保持滚动尺寸比较、1px 容差、50 项越界上限和字体就绪等待。

`read_image` 仅接受附件 ID 或页面 ID。附件读取原图；截图通过项目和页面的最新索引查找，并校验 HTML、共享引用及运行时页框依赖是否过期。截图缺失或过期时明确要求重新渲染。供 HTML 使用的附件 `original_path` 位于附件上下文中。

`read_images` 保存调用 ID、页面／附件 ID、不可变图片引用及内部图片路径。原调用回复直接提供图片；同一截图重复读取时压缩较早的重复像素，压缩后按引用恢复图片。恢复标签只提供页面或附件身份及截图有效性，不暴露路径与哈希。线程历史不继承上一 Run 的工具读图像素；同一 Run 的恢复由检查点保留图片，新 Run 保留用户自行选择的附件消息。

组件和技能正文在首次加载、预置和恢复路径均采用 `id + content`；选择目录继续提供名称和简介。`already_available` 依据当前上下文中完整正文的可见性判断，不把磁盘快照误认成模型已经看到的正文。

## 计划、授权与问答

`create_plan` 持久化 `pending_plan_call`，包括原调用、参数、伴随文本和原始模式。挂起期间不发布工具成功结果。用户批准后原子保存冻结计划、执行模式和 `pending_plan_publication`；若在发布前中断，恢复仅完成原调用的回复，不再次请求批准。

要求修改计划时保留未获批状态并返回规划阶段，执行模式中的计划也一样。此阶段只能阅读、提问和重新提交草案。草案目标先校验，批准后才加入执行工作清单，避免被替换草案的步骤遗留为待完成工作。修改建议原文通过原工具调用返回，空建议使用固定文案。拒绝是正常决定，禁止执行或自动重提，允许简短结束回复。

`update_plan` 只更新已有步骤的 `pending / processing / completed / failed` 状态，至多一个 processing，completed 不可回退。获批、完成、拒绝后的结构均锁定。状态与工具回复持久化成功后才发布进度；保存失败产生明确错误。审批提交失败有有限重试，持续失败停止执行，不假装批准已生效。

页面权限的 `pending_scope_expansion` 保留原调用和参数。批准扩大所申请页面；revise 由后端直接解析为 all_pages，包含运行期间新建页面，不再接收任意 adjusted_scope；拒绝保留原范围。范围和检查点原子保存，已应用状态恢复只发布对应结果，避免重复扩权。已拥有的权限直接返回无需审批的正常结果。

问答的 `pending_question` 保留原调用与后端生成的问题／选项 ID。输入队列按 ID 校验答案，跳过与选项／自定义文本互斥，保留用户原文。模型输出按原问题顺序转换为 question / answer，不包含 UI 内部 ID、显示文本或选项解释。跳过只是一条非授权回答；计划、命令、页面权限仍使用独立审批。

Reviewer 使用 decision + 非空 reasons。内层 submit_review 成功直接结束审查，不追加工具结果；外层 review_task 返回相同结论。审查服务失败保持工具错误，不伪造成业务结论。

## 检查与手工验收

本次使用 Go 定向测试验证资源冲突、编辑原子性、参数互斥、审批和问答状态转换、原调用关联、检查点恢复、图片保留、诊断过滤及命令／Git 输出投影。前端使用类型检查及组件、事件解析、状态和历史恢复定向测试。渲染 Worker 只做语法检查，不运行浏览器自动化。

实际检查结果：

- `go build ./...`、前端 `pnpm exec tsc -b --pretty false`、Worker 的 `node --check` 和 `git diff --check` 通过。
- 后端协议、资源编辑、审批、检查点恢复、幂等回执、图片生命周期、模型事件、输入队列和 SQLite 定向测试通过；最终追加的图片去重与版本记录回归也通过。
- 前端审批、扩权、问答、审查、计划进度、事件解析和状态管理 8 个测试文件共 90 项通过；历史恢复相关定向测试另有 3 项通过。
- 后续完整运行 `go test ./internal/workflow ./internal/model ./internal/contextengine ./internal/llm ./internal/contextcompact`，五个包全部通过。此前单独保留的后端失败已逐项核实并处理，见下文；没有为了通过测试增加旧协议兼容。
- 前端完整 `historyHydrator.test.ts` 还有两项未解决断言：自动命令历史期望旧的重命名卡片，以及中断历史期望旧文案“暂停，已停止执行”而当前实现为“结束”。对应历史拼装实现未在本次修改中调整；相关新协议恢复测试已通过。

前端上述检查问题仍单独保留，不以本次后端五个包通过宣称全仓库测试通过。界面、真实模型及 Chromium 渲染的端到端效果仍由用户手工验收。

用户手工验收：

1. 新建大纲、编辑页面，检查保存提示、预览及渲染诊断；包含越界或资源失败的页面应同时返回截图。
2. 分别批准、无建议修改、有建议修改和拒绝计划；修改后不得执行，拒绝后不得自动重提。挂起期间重启后端，检查原审批继续及结果不重复。
3. 扩权分别选择批准、拒绝和允许全部项；全部项授权后，新建页面应能直接编辑。
4. 问题展示 reason 和选项说明，验证预设选项、自定义原文、单题跳过、取消和挂起恢复。
5. 读取原附件；修改 HTML 或共享视觉依赖后，旧截图应拒绝作为当前截图读取；重新渲染后恢复正常。压缩及重启后图片仍能关联正确页面。

不使用 Computer Use、浏览器自动化或子代理；未执行 Git 提交或推送。


## 后续：完整包测试失败排查

用户完整运行 workflow、model、contextengine 后报告失败。本次复现共 9 个失败，按当前设计与实现分别处理；“HEAD 中也失败”只能说明并非本轮新增，不能作为删除或忽略的依据。

| 失败用例 | 原因及处理 |
|---|---|
| `TestTurnContextRetrievalReusesStableQueryAndInjectsSummary` | 旧样例以线程历史充当检索正文，而当前 briefing 只追加 context_manifest 的有效补充内容。改用当前来源的正文，保留查询复用与查询变化重新检索断言。 |
| `TestChatPlainTextRequiresLaterExplicitFinish` | 断言整个上下文只有两条消息，未考虑初始指令与增量上下文。保留“纯文本不得完成、下轮保留正文并提示 finish_task”的行为验证。 |
| `TestFinishPersistsFinalReplyInModelHistory` | 使用已废弃的页面目录 `sli_1/spec.json` 引用。改用当前内部资源引用 `slide:sli_1:spec`，继续验证公开回复、模型历史及终态内容一致。 |
| `TestFinishPublishesNormalizedSuggestions` | 输入包含四项及数字，违反现有最多三条字符串 Schema。改为合法输入，保留空白规范化和去重断言。 |
| `TestSteeringInjectsIndependentUserMessagesInAcceptanceOrderBeforeFirstNext` | 旧断言忽略初始请求；同时发现首次模型请求前有补充输入时，初始请求会晚于补充输入追加。实现改为先确保初始请求，再追加补充指令；验证用户输入顺序及逐条注入。 |
| `TestSteeringWaitsUntilCompleteToolBatchObservation` | 误把工具结果和补充输入限定为整个上下文末尾两条。改为验证补充输入在对应工具结果之后，允许后续追加状态上下文。 |
| `TestProviderUnavailableUsesAuthoritativeTransientProjection` | 最新运行异常设计使用 `run.error`，旧断言查找 `run.failed`。改为检查唯一的 `run.error`，继续验证错误码、公开消息、retryable 及敏感信息不泄露。 |
| `TestPublicTextUsesPageContextAndPreservesTechnicalContent` | 实现缺陷：用户输入 `outline.json` 时未保护 `.outline.json`。文件名比较统一忽略已知隐藏资源名的前导点，保留原测试。 |
| `TestTranscriptNormalizesExistingRenderCallsOnReplayAndSave` | 要求读取旧的裸 JSONL 格式并删除旧渲染参数，与不迁移、不兼容旧协议的决定冲突，删除。同步删除 llm 中对应旧参数归一化测试和 `visual_review` 清理分支。 |

截图历史隔离及附件保留测试继续保留，样例改为当前工具消息和 Journal 写入方式，并校验原工具调用与回复的关联。没有放松 Journal 身份与格式校验，没有增加旧数据读取分支。
