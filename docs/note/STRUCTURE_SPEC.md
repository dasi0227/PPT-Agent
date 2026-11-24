[目录设计](STRUCTURE.html#backend-heading)

## 后端目录

- `backend_java/pom.xml` 聚合 `ppt-agent-bootstrap`、`ppt-agent-interfaces`、`ppt-agent-domain`、`ppt-agent-infrastructure` 四个 Maven 模块。
- 各模块的 Java 源码位于 `src/main/java/com/dasi/ppt/agent/`，模块资源位于各自的 `src/main/resources/`。
- `ppt-agent-bootstrap` 的启动类为 `com.dasi.ppt.agent.PptAgentApplication`。
- 应用配置位于 `ppt-agent-bootstrap/src/main/resources/application.yml`。
- `bootstrap` 的 `configuration/` 存放 Web、JSON、持久化、后台执行器及模型客户端的装配配置。
- `backend_java/sql/schema.sql` 保存 SQLite 建表与索引定义，由应用外部执行。
- `backend_java/sql/init.sql` 保存预置资产、标签、初始设置及本地模型配置，由 Git 忽略。
- `domain` 模块的 `resources/prompts/` 存放 Agent 提示词与工具定义。
- `domain` 模块的 `resources/schemas/` 存放项目文档的 JSON Schema。
- `infrastructure/properties/` 存放 `JevProperties` 和 `SandboxProperties`，分别绑定 JEV 与命令沙箱参数。
- `infrastructure/repository/` 存放仓储实现，负责领域实体与数据库 PO 的转换。
- `infrastructure/gateway/` 存放 `FileGateway`、`LlmGateway`、`JevGateway`、`GitGateway`、`PlatformGateway`、`BashGateway` 和 `RenderGateway`。
- `JevGateway` 实现 `IJevGateway`，统一负责 JEV 的 HTTP 请求编码和判定结果解析。
- `infrastructure/persistence/` 按存储介质分为 `database/` 和 `filesystem/`。
- `infrastructure/persistence/database/po/` 为每张业务表定义一个 `XxxPo`。
- `infrastructure/persistence/database/mapper/` 为每张业务表定义一个同名前缀的 `XxxMapper` 接口。
- Mapper XML 位于 `ppt-agent-infrastructure/src/main/resources/mapper/`。
- SQLite `TEXT` 列在 PO、Mapper 参数及结果映射中统一使用 `String`，包括状态、类型、模式和协议。
- `infrastructure/persistence/database/ReceiptType` 定义数据库凭据类型 `create_run / create_command / agent / manual`，由仓储与 `ReceiptPo.type` 的字符串值相互转换。[凭据类型](DATABASE_SPEC.md#receipts)
- JSON 存储列按 `String` 读写，仓储负责与领域对象转换。
- `infrastructure/persistence/filesystem/FileStorer` 负责文件的读取、仅新建、原子替换、列举和删除。
- `infrastructure/persistence/filesystem/PathResolver` 生成项目文稿和运行文件的路径。
- `PathResolver` 校验传入的相对路径，确保解析结果仍位于指定项目根目录内。
- `infrastructure/persistence/filesystem/MutationLog` 对应一次文稿写入或历史切换的恢复日志文件，记录核对提交所需的字段及每个文件的 `before` 和 `after`。[日志字段](#项目目录)
- `infrastructure/persistence/filesystem/MutationLogType` 定义文稿文件恢复日志的 `type`，取 `agent / manual / switch`。
- `infrastructure` 模块将 `base.css`、`decorations.js` 和 `theme-bridge.js` 直接存放在 `resources/runtime/` 下。主题用 `--font-sans`、`--font-display`、`--font-mono` 指定系统字体族。

- `interfaces/controller/` 按功能分为 `project`、`asset`、`setting`、`export`、`file`、`session`、`history`、`execution`、`interaction`、`checkpoint` 和 `system`。
- 每个功能包就近存放 `XxxController.java`。有请求体或结构化响应时，分别将顶层请求类型和响应类型放在 `dto/` 和 `vo/`。
- DTO 声明请求的必填、长度和格式约束，Controller 通过 `@Valid` 触发校验，失败返回 `INVALID_ARGUMENT`。Service 负责业务规则，包括默认路由需绑定模型、名称唯一性和正文版本冲突。
- 项目、会话、资产与模型配置的创建和更新 DTO，以及保存单个模型路由的 `SaveRouteDto`，均通过 `controller/dto/StrictRequestDto` 拒绝未声明的字段。
- GET 的路径和查询参数由 Controller 直接接收。
- 图片、下载和静态资源使用各自的 HTTP 响应类型。无响应体的接口直接返回空响应体。
- `controller/system/SystemController.java` 提供健康检查和内置预览资源读取接口。
- 项目文件打开接口归 `controller/file/`。
- 会话历史、Run 与命令详情、工具详情和会话订阅归 `controller/history/`。
- Run 与旁路命令的受理及取消入口归 `controller/execution/`。
- `interfaces/handler/` 存放 `GlobalExceptionHandler.java` 和 `ErrorResponse.java`。
- `interfaces/sse/SseEventPublisher` 实现 `domain/shared/sse/EventPublisher`，管理订阅连接，并将事件名和数据写入目标会话的 SSE 流。

Java 后端的数据类型使用普通类，禁止使用 `record`。下表中的子结构在所属顶层类中声明为静态嵌套类。同一响应结构可由多个接口共用，序列化字段以接口契约为准。[顶层 DTO 和 VO](STRUCTURE.html#interface-package-heading) · [API 契约](API.html)

| 包 | 所属顶层类 | 静态嵌套类 |
| --- | --- | --- |
| `project` | `SaveProjectAppearanceDto` | `DecorationPositionsDto` |
| `project` | `ProjectListVo` | `ProjectSummaryVo` |
| `project` | `DeckVo` | `SlideContentVo`、`DeckHashesVo`、`SlideHashesVo` |
| `project` | `ProjectAppearanceVo` | `DecorationPositionsVo` |
| `session` | `SessionListVo` | `SessionSummaryVo` |
| `session` | `SessionStateVo` | `PlanVo`、`PlanStepVo`、`ContextWindowVo`、`ContextDetailsVo`、`TokenBreakdownItemVo` |
| `history` | `HistoryPageVo` | `HistoryItemVo`、`RunHistoryVo`、`CommandHistoryVo`、`HistoryInstructionVo`、`HistoryReplyVo`、`PublicReferenceVo`、`CommandSummaryResultVo`、`ChangeSummaryVo` |
| `history` | `RunChangesVo` | `ChangeTargetVo`、`RunChangeDetailVo`、`ChangeDiffVo`、`FieldDiffVo`、`FieldDiffRowVo`、`OutlineDiffGroupVo`、`OutlineDiffRowVo`、`TextDiffHunkVo`、`TextDiffRowVo` |
| `history` | `RunDetailVo` | `RunMessageVo`、`RunUsageVo`、`PendingInteractionVo`、`ExecutionFailureVo` |
| `history` | `CommandDetailVo` | `CommandInputVo`、`CommandResultVo` |
| `history` | `ToolDetailVo` | `ToolResultVo`、`ToolChangedTargetVo`、`ToolImageVo`、`LoadedResourceVo`、`ToolCommandVo`、`ToolInteractionVo`、`PreviewAssessmentVo`、`PreviewScoresVo`、`PreviewJudgesVo` |
| `execution` | `AppendRunDto` | `RunScopeDto`、`InstructionReferenceDto`、`DomRegionDto`、`DomTargetDto`、`DomDecorationDto`、`DomFingerprintDto`、`DomAncestorDto`、`CanvasRectDto` |
| `interaction` | `AnswerQuestionsDto` | `QuestionAnswerDto` |
| `interaction` | `InteractionFormVo` | `QuestionVo`、`QuestionOptionVo`、`PlanFormVo`、`PermissionFormVo`、`ScopeFormVo`、`InteractionRequestVo`、`InteractionAnswerVo`、`QuestionAnswerVo` |
| `checkpoint` | `RollbackVo` | `RollbackInputVo`、`InitialScopeVo`、`FullReferenceVo` |

- 单个 POST 接口专用的成功响应 VO 按动作命名。有对应请求 DTO 时，两者使用相同的动作名。
- `CreateProjectDto` 创建项目，`UpdateProjectDto` 修改项目名称，两者共用 `ProjectIdVo` 返回项目 `id`。
- 创建标签接口返回顶层 `TagInfoVo`，标签列表的 `TagListVo.items` 复用该类型。
- 创建与更新资产共用顶层 `AssetInfoVo`，返回保存后的资产信息和正文哈希。`AssetListVo.items` 复用该类型，列表项的 `hash` 为 `null`。
- 选择本地应用使用 `PickApplicationVo` 返回 `path`，取消选择时该字段为 `null`。
- `CreateExportDto` 对应 `CreateExportVo`，返回导出任务 `id`。
- `UploadAttachmentDto` 对应 `UploadAttachmentVo`，返回附件 `id`。
- `CreateSessionDto` 创建会话，`UpdateSessionDto` 修改会话标题，两者共用 `SessionIdVo` 返回会话 `id`。
- `DeleteSessionPlanDto` 对应 `DeleteSessionPlanVo`，返回提交后的会话 `revision`。
- `AppendRunDto` 对应 `AppendRunVo`，返回指令受理后所属的 `run_id`。
- `AppendCommandDto` 对应 `AppendCommandVo`，返回受理的单次命令执行 `id`。
- `DeleteProjectDto` 和 `DeleteProjectLogoDto` 分别用于删除项目和删除项目 Logo，请求体均只含 `project_id`。删除成功返回空响应体。
- `ContentHashVo` 供基本信息、约束、大纲、单页规格和单页 HTML 的保存接口共用。
- 文稿正文沿用 `Meta`、`Constraint`、`Outline`、`Spec` 对应的 JSON 结构。
- 文件打开设置的读取和保存共用 `FileOpenSettingsVo`，返回实际生效的打开方式和自定义应用列表。
- `CreateModelDto` 创建模型配置，`UpdateModelDto` 局部更新已有配置，密钥均只写，两者共用 `ModelConfigVo` 返回公开配置。读取模型设置时，`ModelSettingsVo` 组合 `List<ModelConfigVo>` 和 `ModelRoutesVo`，后二者均为顶层类。`DeleteModelDto` 按 ID 删除模型，`SaveRouteDto` 指定路由项及目标模型 ID。[模型设置字段](API.html#settings)
- `RunChangesVo` 在差异条目上直接返回 `type` 及对应明细字段。工具结果与权限审批候选将同一结构放入各条目的 `diff`。[变更与工具详情](API.html#history)
- `InteractionRequestVo` 表示表单内容。待答表单附带 `type` 与 `version`，工具详情通过 `interaction.request` 返回当时的表单内容。[人工与工具详情](API.html#interactions)
- `PublicReferenceVo` 仅用于历史展示。回退回填使用 `FullReferenceVo` 保留 DOM 原文与注释。[历史与回退](API_SPEC.md#消息)

- `domain/` 下的业务包为 `project/`、`agent/`、`asset/` 和 `setting/`。
- `domain/gateway/` 与业务包平级，集中存放领域层调用外部能力的接口，实现类位于 `infrastructure/gateway/`。
- `domain/shared/` 与业务包平级，`exception/` 存放共用异常与错误码，`constant/` 和 `util/` 存放跨领域常量和辅助函数。
- `domain/shared/sse/EventPublisher` 接收投递目标和 `SseEvent`，供项目与 Agent 服务发布订阅通知。
- `domain/shared/sse/SseEvent` 是普通 Java 接口，五个实现均在接口内声明为嵌套类，分别为 `Changed`、`RunProgress`、`InteractionChanged`、`CommandProgress` 和 `Reset`。[事件名与字段](API.html#history)
- `domain/shared/sse/ChangeArea` 对应 `changed.areas`，取 `history / run / commands / plan / suggestions / context / checkpoints / content / sessions`。[重读范围](API_SPEC.md#消息)
- `domain/shared/enumeration/RunActivity` 供 Run 详情的 `activity` 与 SSE 的 `progress.activity` 共用。当前无活动或 Run 已终态时返回 `null`。[活动取值与订阅字段](API.html#history)
- 每个领域的 `model/` 存放值对象及业务输入和结果，`model/entity/` 存放实体，`model/enumeration/` 存放枚举。
- 领域仓储接口位于各自的 `repository/`，外部能力接口位于各自的 `gateway/`。
- `project/model/entity/` 存放项目实体 `Project` 和页面实体 `Slide`。
- `Slide` 以稳定的 `slide_id` 标识一页，关联该页的 `Spec` 和 HTML 源码。
- `project/model/Deck` 按所属项目聚合 `Meta`、`Constraint`、`Outline` 及按 `slide_id` 索引的 `Slide`。[项目读取契约](API.html#projects)
- `project/model/Meta` 对应 `.meta.json`，保存文稿标题、语言、页数要求、受众和目标。
- `project/model/Spec` 对应 `{slide_id}.json`，描述单页的用途、核心信息、内容元素和布局。
- `project/model/enumeration/SlidePurpose` 对应 `Spec.purpose`，取 `cover / introduction / transition / content_explanation / content_comparison / content_example / content_guidance / content_other / conclusion / other`。
- `project/model/enumeration/SlideElementType` 对应 `Spec.elements[].type`，取 `text / list / metric / quote / table / chart / diagram / code / asset`。
- `project/model/Outline` 对应 `.outline.json`，维护章、节与页面的层级、顺序、标题和稳定 ID。
- `Outline.java` 在 `Outline` 内声明 `Section` 和 `Subsection` 两个内部类。
- `Outline.Section` 保存章 ID、标题、有序的直属页面和有序的节。[大纲字段](API.html#projects)
- `Outline.Subsection` 保存节 ID、标题和有序页面。章和节均以页面 ID 和标题引用页面。[大纲字段](API.html#projects)
- `project/model/Constraint` 对应 `.constraint.json`，分别保存内容约束和视觉约束的要求与禁用项。
- `project/model/ChangeTarget` 是文稿变更目标的值对象，包含 `part`、项目内相对路径 `path` 和页面目标的 `slide_id`。
- `project/model/DeckEditCandidate` 表示一个 `ChangeTarget` 的待提交修改，携带待写入的完整文件内容或明确的删除操作。
- `DeckEditCandidate` 的读取基准记录原文件哈希或文件当时不存在，供 `DeckService` 在提交前持锁复核。
- `project/model/DeckChange` 表示一项文稿变更，包含 `ChangeTarget`、`ChangeStatus` 和由 `DiffType` 区分的差异明细。[变更字段](API.html#history)
- `project/model/enumeration/ChangePart` 对应 `ChangeTarget.part`。项目级取 `meta / constraint / outline`，页面级取 `spec / html`。
- `project/model/enumeration/ChangeStatus` 表示文稿目标的新增、修改或删除，取 `added / modified / deleted`。审批候选、工具实际提交和 Run 净修改使用同一组取值。
- `project/model/enumeration/DiffType` 决定文稿差异明细的结构，取 `fields / outline / text / binary / unavailable`。[差异与工具详情](API.html#history)
- `project/model/enumeration/OutlineNodeType` 标识大纲差异行的节点，取 `section / subsection / slide`。
- `project/model/enumeration/DiffRowType` 标识差异行，字段差异使用 `added / removed`，大纲和文本差异还可使用 `context`。[差异字段](API.html#history)
- `project/model/enumeration/DecorationType` 标识装饰种类，取 `slide_pagination / deck_title / section_title / logo`，项目外观与 DOM 选区共用。每种装饰的位置由 `DecorationPosition` 表示。[项目外观](API.html#projects) · [DOM 引用](API.html#runs)
- `project/model/ExportSnapshot` 是 `ExportTask` 在内存中持有的不可变输入，保存项目管理名称、文稿标题、按页序排列的页面标题与 HTML、每页所属章的标题、主题 CSS、装饰位置和 Logo 字节。[导出契约](API_SPEC.md#导出)
- `IProjectRepository` 负责项目查询与创建、改名、删除，文稿提交及恢复由文稿仓储负责。
- `domain/gateway/IFileGateway` 供 `AttachmentService`、`SlidePreviewService` 和 `ExportService` 存取各自的项目运行文件。[项目文件布局](#项目目录)
- `infrastructure/gateway/FileGateway` 实现 `IFileGateway`，通过 `PathResolver` 解析项目路径，通过 `FileStorer` 创建、读取、替换、列举和删除文件。
- `agent/model/entity/` 下的持久化实体有 `Session`、`Run`、`Message`、`Command` 和 `Checkpoint`。
- `agent/model/entity/PendingInteraction` 是由 `InteractionService` 管理的内存实体，以 `run_id + tool_call_id` 标识一项待答交互，持有 `InteractionType`、当前表单及其 `version`。[待答与答复](API_SPEC.md#人工)
- `agent/model/` 存放会话的 `Plan` 和 `PlanStep`。
- `agent/model/RunConfig` 由 `RunService` 在受理时根据全局设置组装，包含本次 Run 的主模型、旁路及备用模型、审批开关和自动命令开关，由 `RunState` 持有并在整次 Run 中复用。[受理与执行材料](WORKFLOW_SPEC.md#新开-run)
- `agent/model/RunScope` 表示已生效的页面授权。`InputService` 在受理时将 `section_ids`（章）或 `subsection_ids`（节）展开为页面 ID，`ToolGuard` 按生效范围核对目标页。
- `agent/model/RunChangeSet` 按 `ChangeTarget` 合并归属本 Run 的已提交 `DeckChange`，抵消本 Run 自身撤销的修改，供独立审查和 Run 收尾使用。[净修改归属](DATABASE_SPEC.md#runs)
- `agent/model/ContextEstimate` 保存一次完整请求的上下文 Token 总量与分类明细，由 `ContextService` 整体更新，供压缩判断和会话状态查询使用。[会话上下文](API.html#sessions)
- `AppendRunDto` 保留 `scope` 是否出现。追加指令时，省略则沿用当前 `RunScope`，显式 `null` 授权全部页面，显式 `slide_ids=[]` 不增加授权。[执行契约](API_SPEC.md#执行)
- `agent/model/InstructionReference` 表示一条引用。指令按输入顺序保存引用，供提示词拼装和回退回填使用。
- `agent/model/DomSelection` 是 DOM 引用的选区内容，保存页面与选区标识、标注序号、提交时的 HTML、源码哈希、注释及元素或装饰定位。[选区字段](API.html#runs)
- `agent/model/enumeration/InstructionReferenceType` 对应 `references[].type`，取 `skill / component / slide / image / dom`。追加输入只接受 `image` 和 `dom`。[引用字段](API.html#runs)
- `agent/model/CheckpointSnapshot` 是 `Checkpoint` 的恢复内容，包括目标会话记录与 `agent` 凭据、五类文稿文件、主题 ID、装饰位置及 Logo 字节。普通检查点另含用于回填的原始输入。[快照范围](DATABASE_SPEC.md#checkpoints)
- `asset/model/` 存放资产、标签及其类型。
- `IAssetRepository` 负责资产、标签及关联的持久化。
- `setting/model/` 存放模型配置和各组全局设置。
- `ISettingRepository` 负责分组设置的持久化。
- `domain/gateway/IPlatformGateway` 提供本地应用选择与项目文件打开能力，由 `infrastructure/gateway/PlatformGateway` 实现。
- `ProjectService.openFile` 从 `SettingService` 读取偏好，按文件后缀和继承规则确定打开方式，向 `PlatformGateway` 传入应用路径或用 `null` 指定系统默认应用。网关使用 `PathResolver` 校验项目文件的真实路径后调用桌面应用。设置页通过 `SettingService.pickApplication` 调用同一网关选择应用。[文件打开契约](API_SPEC.md#其它)
- `domain/gateway/IBashGateway` 由 `infrastructure/gateway/BashGateway` 实现。
- `agent/model/enumeration/` 存放工具与 Harness 的专属错误码。
- `agent/model/enumeration/InstructionDecision` 表示输入判定结果。有效结果为 `task / gossip / disclosure / bypass / irrelevant`，未判定或检测失败时字段为空。[指令判定字段](DATABASE.html#messages-instruction-decision)
- `agent/model/enumeration/InteractionType` 标识交互表单类型，待答项与工具结果共用 `question / plan / permission / scope / resource`。[人工交互契约](API_SPEC.md#人工)
- `agent/model/enumeration/PermissionDecision` 取 `allow_once / deny`。一次批准仅授权当前表单展示的命令及候选修改。
- `agent/model/enumeration/ScopeDecision` 取 `approve / refuse / revise`，分别表示合并提议范围、保留已有授权、授权本项目全部页面。
- `agent/model/enumeration/PlanDecision` 取 `approve / refuse`。批准使同一 Run 从 Plan 切换至 Create，拒绝时继续规划并保留原计划。
- `agent/model/enumeration/ResourceDecision` 取 `approve / reject`。只有批准时可提交完整 `draft`，省略时采用原提案。[审批答复字段](API.html#interactions)
- `agent/model/enumeration/ToolName` 取四种运行模式公开工具名的并集，`ToolRegistry` 按名称映射到对应的 `AgentTool` 实现。[Create 工具清单](ASSEMBLE.html#create-tools-title) · [Chat、Ask、Plan 工具清单](ASSEMBLE.html#other-tools-title)
- `agent/model/LlmRequest` 的消息字段使用 Spring AI 的 `List<org.springframework.ai.chat.messages.Message>`，角色由消息的 `getMessageType()` 提供。
- `LlmRequest` 同时携带本次调用可用的工具定义。
- `agent/model/LlmResponse` 是 `ILlmGateway` 的领域响应，携带正文、推理内容、有序的 `ToolCall`、各次实际请求可取得的用量及后续请求所需的协议回传数据。
- `agent/model/ToolCall` 保存模型返回的 `tool_call_id`、原始工具名和参数，供 `ToolExecutor` 记录和执行。
- `agent/model/enumeration/ToolResultStatus` 标识已记录的工具结果为 `completed / blocked / failed`。结果尚未记录时，工具详情返回 `result=null`。[工具详情](API_SPEC.md#消息)
- `agent/model/enumeration/PreviewStatus` 对应 `result.preview.status`，取 `completed / unavailable / stale`，分别表示截图评分已完成、不可用或已过期。
- `agent/model/result/PreviewAssessment` 保存单页截图的评分状态、评分与判定或原因，以及复用所需的基准，随工具结果持久化。[评分字段](API.html#history)
- `agent/model/enumeration/PreviewMismatch` 对应 `result.preview.judges.mismatch`，取 `meta_mismatch / content_mismatch / spec_mismatch / visual_mismatch / none`。
- `agent/model/enumeration/PreviewIssue` 对应 `result.preview.judges.issue`，取 `expression_issue / layout_issue / visual_issue / none`。
- `agent/model/event/` 存放生命周期事件。
- `agent/model/result/` 存放生命周期阶段结果、`ToolResult` 和 `PreviewAssessment`。
- `agent/service/` 按 `run`、`input`、`hook`、`context`、`tool`、`interaction`、`command`、`session`、`history` 分包，其余领域的 `service/` 保持单层。
- `agent/service/run/` 存放 Run 生命周期、主循环和 `RunState`。
- `agent/service/hook/` 存放 `SyncHook`、`AsyncHook` 和负责四个阶段的 `HookDispatcher`。
- `hook/beforeinputprocessed/` 存放 `beforeInputProcessed` 阶段的 `IntentHook` 和 `AutoRenameHook`。
- `hook/beforemodelcall/` 存放 `beforeModelCall` 阶段的 `ToolPruneHook`。
- `hook/beforeruncompleted/` 存放 `beforeRunCompleted` 阶段的 `CompletionHook`。
- `hook/afterruncompleted/` 存放 `afterRunCompleted` 阶段的 `AutoSuggestHook`。
- `agent/service/context/` 存放 `ContextService`、`ContextCompactor` 和 `PromptAssembler` 接口。
- `context/assembler/` 存放各场景的 `XxxPromptAssembler` 实现。
- `agent/service/command/` 存放 `CommandService` 和 `CommandHandler` 接口。
- `command/handler/` 存放各命令的 `XxxCommandHandler` 实现。
- `agent/service/tool/` 存放 `AgentTool` 接口、`ToolRegistry`、`ToolGuard` 和 `ToolExecutor`。
- `tool/builtin/` 存放内置工具的 `AgentTool` 实现。

## 后端依赖

- `bootstrap` 装配接口层、领域服务和基础设施实现。
- `interfaces` 与 `infrastructure` 都依赖 `domain`，领域模块独立于两者。
- `domain` 直接依赖 `spring-ai-model` 以组装模型消息，供应商客户端由 `infrastructure` 使用。
- HTTP 请求与响应类型留在 `interfaces`，数据库 PO 及基础设施配置类型留在 `infrastructure`。
- 第三方依赖的版本由父 POM 或其引入的 BOM 管理，使用依赖的模块在自身 POM 声明。[后端依赖表](STRUCTURE.html#dependencies-heading)

## 项目目录

- 每个项目使用独立的 `{project_id}/` 根目录，由同一工作目录下的单个后端进程管理。
- 项目级文稿存于根目录的 `.meta.json`、`.constraint.json`、`.outline.json`。
- 单页规格和源码按幻灯片 ID 存为根目录的 `{slide_id}.json` 和 `{slide_id}.html`。
- 文稿回退和返回只处理上述五类文件，其他项目文件保留原状。
- 项目主题正文和 Logo 存在数据库中，项目目录不另存永久副本。
- `.git/` 存放项目 Git 仓库，`.runtime/` 存放项目运行文件。
- `.runtime/mutations/{operation_hash}.json` 保存一次涉及文稿文件的操作日志。
- `type=agent` 和 `type=manual` 的日志保存原凭据键 `(type, owner_id, request_id)` 及 `request_hash`。
- `type=switch` 的日志保存目标 `session_id` 和切换前的 `base_revision`（十进制字符串）。回退和返回最新共用此格式。
- 文件名中的 `operation_hash` 取自操作的唯一键。文稿写入使用原凭据键，历史切换使用 `(switch, session_id, base_revision)`。
- `files` 每项记录唯一的项目相对路径 `path` 及其 `before`、`after`，路径限于本次实际改动的五类文稿文件。
- 文件状态用 Base64 字符串保存完整字节，`""` 表示空文件，`null` 表示路径不存在。
- 完整日志在第一个文稿文件改动前原子落盘。
- 文稿文件与数据库核对一致后，持久移除日志。
- 恢复时无法判定应保留哪一版，或文件恢复失败时，保留日志并暂停该项目写入。其他项目独立完成启动恢复。[启动恢复](WORKFLOW_SPEC.md#启动恢复)
- `.runtime/attachments/{attachment_id}.png` 保存附件原图，同名 `.webp` 保存附件缩略图。
- `.runtime/screenshots/{screenshot_id}.png` 保存调用当时的截图，截图 ID 全局唯一，已有文件不覆盖。
- 附件原图、附件缩略图和历史截图保留至项目删除，已上传但未发送的附件适用同一规则。
- `.runtime/render-slides.json` 按 `slide_id` 保存最新的 `{screenshot_id, hash}`，无记录的页面省略该键。
- 渲染哈希覆盖 HTML、主题、装饰、Logo 及影响画面的内置资源。
- 渲染索引按项目串行更新，以原子替换发布新文件。
- `.runtime/thumbnails/{slide_id}.webp` 只保存该页当前缩略图，从与当前渲染哈希匹配的截图生成。
- 同一页的缩略图生成与索引切换串行执行，缩略图文件采用原子替换。
- 切换最新截图前先删除旧缩略图，删除失败时保留原索引。
- 截图评分及复用所需的基准存入数据库的工具结果，评估中的临时状态仅保留在内存。
- `.runtime/exports/{export_id}/` 的根目录只存一个最终成品，文件名为 `{filename}.pdf`、`{filename}.zip` 或 `{filename}.html`。
- 导出使用创建时固定的完整输入，`ExportSnapshot` 只在任务内存保留。预览和导出都用 `Meta.title` 填充 `deck_title`，用页面所属 `Outline.Section.title` 填充 `section_title`。
- PNG 和 PDF 导出的逐页图片存入任务的 `work/{序号}-{标题}.png`，序号补零，标题规范化。
- HTML 导出直接生成单文件，内嵌播放器、样式、脚本和 Logo。文本按主题指定的系统字体族排版，项目内不创建播放目录。
- 导出成品写完并关闭后清理 `work/`，保留待下载成品。
- 导出失败或取消时，工作停止后清理临时文件和不完整成品。
- 完整发送下载文件后清理整个导出任务目录，启动恢复时清理遗留导出目录。
