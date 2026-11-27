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
- Mapper 读写整组字段时使用 PO，返回多条记录时使用 PO 列表。按 ID 修改单项、查询单值或批量绑定 ID 时直接使用对应的标量或集合。领域实体、枚举与持久化表示的转换由 Repository 完成。
- 单独读取项目 Logo 时，Mapper 通过 SQLite JDBC 的 `getBytes()` 读取 BLOB 并返回 `byte[]`。
- Mapper XML 位于 `ppt-agent-infrastructure/src/main/resources/mapper/`。
- SQLite `TEXT` 列在 PO、Mapper 参数及结果映射中统一使用 `String`，包括状态、类型、模式和协议。
- JSON 存储列按 `String` 读写，仓储负责与领域对象转换。
- `infrastructure/persistence/filesystem/FileStorer` 负责项目删除目录的暂存、状态检查和恢复，按数据库提交结果恢复或清除暂存目录。
- `domain/shared/service/PathService` 按固定布局计算路径，目标可尚未创建。`projectsRoot`、`projectDirectory`、`deletingDirectory` 定位目录，`meta`、`constraint`、`outline`、`slideSpec`、`slideHtml` 定位文稿，`attachmentOriginal`、`attachmentThumbnail` 定位附件，`slideThumbnailDirectory`、`slideThumbnail` 定位单页预览目录和指定渲染哈希的 WebP。
- `PathService` 统一维护五类文稿的命名规则，`isProjectFile(projectDirectory, path)` 按指定项目根目录和文件名判断文稿路径，目标可以尚未创建。保存大纲时用 `slideIdFromFileName` 从页面文件名提取 ID。`OpenProjectFileDto` 复用文稿命名规则校验请求的 `file_name`。`DeckService` 打开文稿前解析符号链接，确认真实目标是该项目根目录下的普通文稿文件。撤销记录只接受文件名，仓储验证整批目标路径后执行恢复。首次保存文稿时允许创建文件。
- `infrastructure` 模块将 `base.css`、`decorations.js` 和 `theme-bridge.js` 直接存放在 `resources/runtime/` 下。基础样式定义系统字体、颜色、字号和间距等默认值，预览和导出都先应用基础样式，再应用已选主题和单页样式。

- `interfaces/controller/` 按功能分为 `project`、`asset`、`setting`、`export`、`file`、`session`、`history`、`execution`、`interaction`、`checkpoint` 和 `system`。
- 每个功能包就近存放 `XxxController.java`。有请求体或结构化响应时，分别将顶层请求类型和响应类型放在 `dto/` 和 `vo/`。
- DTO 用 Jakarta Validation 声明单字段约束，Controller 通过 `@Valid` 触发校验，失败返回 `INVALID_ARGUMENT`。Service 在对应写方法中依次检查业务条件，包括默认路由需绑定模型、名称唯一性和正文版本冲突。
- JSON 请求的未知字段由统一的 Jackson 反序列化配置处理。
- GET 的路径和查询参数由 Controller 直接接收。
- 图片、下载和静态资源使用各自的 HTTP 响应类型。无响应体的接口直接返回空响应体。
- `controller/system/SystemController.java` 提供健康检查和内置预览资源读取接口。
- 项目文件打开、幻灯片缩略图和图片附件接口归 `controller/file/`。项目导出归 `controller/export/`。
- 会话历史、Run 与命令详情、工具详情和会话订阅归 `controller/history/`。
- Run 与旁路命令的受理及取消入口归 `controller/execution/`。
- `interfaces/handler/` 存放 `GlobalExceptionHandler.java` 和 `ErrorResponse.java`。
- `interfaces/sse/SseEventPublisher` 实现 `domain/shared/sse/EventPublisher`，管理订阅连接，并将事件名和数据写入目标会话的 SSE 流。

Java 后端的数据类型使用普通类，禁止使用 `record`。下表中的子结构在所属顶层类中声明为静态嵌套类。同一响应结构可由多个接口共用，序列化字段以接口契约为准。[顶层 DTO 和 VO](STRUCTURE.html#interface-package-heading) · [API 契约](API.html)

| 包 | 所属顶层类 | 静态嵌套类 |
| --- | --- | --- |
| `project` | `DeckInfoVo` | `SlideContentVo`、`DeckHashesVo`、`SlideHashesVo` |
| `project` | `ProjectAppearanceVo` | `DecorationPositionsVo` |
| `session` | `SessionListVo` | `SessionSummaryVo` |
| `session` | `SessionStateVo` | `PlanVo`、`PlanStepVo`、`ContextWindowVo`、`ContextDetailsVo`、`TokenBreakdownItemVo` |
| `history` | `HistoryListVo` | `HistoryItemVo`、`RunHistoryVo`、`CommandHistoryVo`、`HistoryInstructionVo`、`HistoryReplyVo`、`PublicReferenceVo`、`CommandSummaryResultVo`、`ChangeSummaryVo` |
| `history` | `RunChangesVo` | `ChangeTargetVo`、`RunChangeDetailVo`、`ChangeDiffVo`、`FieldDiffVo`、`FieldDiffRowVo`、`OutlineDiffGroupVo`、`OutlineDiffRowVo`、`TextDiffHunkVo`、`TextDiffRowVo` |
| `history` | `RunInfoVo` | `RunMessageVo`、`RunUsageVo`、`PendingInteractionVo`、`ExecutionFailureVo` |
| `history` | `CommandInfoVo` | `CommandInputVo`、`CommandResultVo` |
| `history` | `ToolInfoVo` | `ToolResultVo`、`ToolChangedTargetVo`、`ToolImageVo`、`LoadedResourceVo`、`ToolCommandVo`、`ToolInteractionVo`、`PreviewAssessmentVo`、`PreviewScoresVo`、`PreviewJudgesVo` |
| `execution` | `AppendRunDto` | `RunScopeDto`、`InstructionReferenceDto`、`DomRegionDto`、`DomTargetDto`、`DomDecorationDto`、`DomFingerprintDto`、`DomAncestorDto`、`CanvasRectDto` |
| `interaction` | `AnswerQuestionsDto` | `QuestionAnswerDto` |
| `interaction` | `InteractionFormVo` | `QuestionVo`、`QuestionOptionVo`、`PlanFormVo`、`PermissionFormVo`、`ScopeFormVo`、`InteractionRequestVo`、`InteractionAnswerVo`、`QuestionAnswerVo` |
| `checkpoint` | `RollbackVo` | `RollbackInputVo`、`InitialScopeVo`、`FullReferenceVo` |

- 普通 CRUD 的创建接口共用 `IdVo`，更新接口返回 `void`。列表和详情响应分别使用 `XxxListVo` 和 `XxxInfoVo`。
- `CreateProjectDto` 创建项目，`UpdateProjectDto` 修改项目名称。`ProjectListVo.items` 使用 `ProjectInfoVo` 返回项目 ID、名称和更新时间。
- `SaveProjectThemeDto` 提交项目主题，`SaveProjectPositionsDto` 提交四项装饰位置，两者分别使用独立的顶层请求结构。
- 标签列表项使用 `TagListVo.TagInfoVo`。
- 资产列表项和详情共用 `AssetInfoVo`，列表中 `content` 为 `null`，详情中为正文。更新成功返回空响应体。
- 选择本地应用使用 `PickApplicationVo` 返回 `path`，取消选择时该字段为 `null`。
- 上传新附件使用 `IdVo` 返回资源 ID。项目导出返回文件流。
- `CreateSessionDto` 创建会话，`UpdateSessionDto` 修改会话标题。
- `DeleteSessionPlanDto` 对应 `DeleteSessionPlanVo`，返回提交后的会话 `revision`。
- `AppendRunDto` 对应 `AppendRunVo`，返回指令受理后所属的 `run_id`。
- `AppendCommandDto` 对应 `AppendCommandVo`，返回受理的单次命令执行 `id`。
- `DeleteProjectDto` 和 `DeleteProjectLogoDto` 分别用于删除项目和删除项目 Logo，请求体均只含 `project_id`。删除成功返回空响应体。
- `ContentHashVo` 供基本信息、约束、大纲、单页规格和单页 HTML 的保存接口共用。
- 文稿正文沿用 `Meta`、`Constraint`、`Outline`、`Spec` 对应的 JSON 结构。
- 文件打开设置的读取和保存共用 `FileOpenSettingsVo`，返回实际生效的打开方式和自定义应用列表。
- `CreateModelDto` 创建模型配置，`UpdateModelDto` 局部更新已有配置，密钥均只写，更新成功返回空响应体。模型详情返回 `ModelConfigInfoVo`。读取模型设置时，`ModelSettingsVo` 组合 `List<ModelConfigInfoVo>` 和 `ModelRoutesVo`，后二者均为顶层类。`DeleteModelDto` 按 ID 删除模型，`SaveRouteDto` 指定路由项及目标模型 ID。[模型设置字段](API.html#settings)
- `RunChangesVo` 在差异条目上直接返回 `type` 及对应明细字段。工具结果与权限审批候选将同一结构放入各条目的 `diff`。[变更与工具详情](API.html#history)
- `InteractionRequestVo` 表示表单内容。待答表单附带 `type` 与 `version`，工具详情通过 `interaction.request` 返回当时的表单内容。[人工与工具详情](API.html#interactions)
- `PublicReferenceVo` 仅用于历史展示。回退回填使用 `FullReferenceVo` 保留 DOM 原文与注释。[历史与回退](API_SPEC.md#消息)

- `domain/` 下的业务包为 `project/`、`agent/`、`asset/` 和 `setting/`。
- `domain/gateway/` 与业务包平级，集中存放领域层调用外部能力的接口，实现类位于 `infrastructure/gateway/`。
- `domain/shared/` 与业务包平级，`exception/` 存放共用异常与错误码，`constant/` 和 `util/` 存放跨领域常量和辅助函数。
- `domain/shared/service/NotifyService` 按项目 ID 或已取得的会话 ID 列表广播项目内容、外观及会话列表的变更。
- `domain/shared/sse/EventPublisher` 接收投递目标和 `SseEvent`，供项目与 Agent 服务发布订阅通知。
- `domain/shared/sse/SseEvent` 是普通 Java 接口，六个实现均在接口内声明为嵌套类，分别为 `ProjectChanged`、`SessionChanged`、`RunProgress`、`InteractionChanged`、`CommandProgress` 和 `Reset`。[事件名与字段](API.html#history)
- `domain/shared/sse/ProjectChangeArea` 对应 `project_changed.areas`，取 `content / appearance / sessions`。`domain/shared/sse/SessionChangeArea` 对应 `session_changed.areas`，取 `history / run / commands / plan / suggestions / context / checkpoints`。[重读范围](API_SPEC.md#消息)
- `domain/shared/enumeration/RunActivity` 供 Run 详情的 `activity` 与 SSE 的 `progress.activity` 共用。当前无活动或 Run 已终态时返回 `null`。[活动取值与订阅字段](API.html#history)
- 每个领域的 `model/` 存放值对象及业务输入和结果，`model/entity/` 存放实体，`model/enumeration/` 存放枚举。
- 领域仓储接口位于各自的 `repository/`，外部能力接口位于各自的 `gateway/`。
- `project/model/entity/` 存放项目实体 `Project` 和页面实体 `Slide`。
- `Slide` 以稳定的 `slide_id` 标识一页，关联该页的 `Spec` 和 HTML 源码。
- `SchemaService` 在初始化时加载资料、约束、大纲和页面规格的 JSON Schema。`DeckService` 读取文稿时通过 `SchemaService.decode` 校验并解码正文，用同一份文件字节计算哈希，再组装 `Deck`。保存时将 JSON 正文编码为字节并调用 `SchemaService.validate`。`ExportService` 读取渲染快照时复用 `decode` 解析资料和大纲。
- `project/model/Deck` 按所属项目聚合 `Meta`、`Constraint`、`Outline` 及按大纲页序排列的 `List<Slide>`。`DeckInfoVo` 按每页的 `slide_id` 组装响应中的 `slides` 和 `hashes.slides`。[项目读取契约](API.html#projects)
- `project/model/Meta` 对应 `.meta.json`，保存文稿标题、语言、页数要求、受众和目标。
- `project/model/Spec` 对应 `{slide_id}.json`，描述单页的用途、核心信息、内容元素和布局。
- `project/model/enumeration/SlidePurpose` 对应 `Spec.purpose`，取 `cover / introduction / transition / content_explanation / content_comparison / content_example / content_guidance / content_other / conclusion / other`。
- `project/model/enumeration/SlideElementType` 对应 `Spec.elements[].type`，取 `text / list / metric / quote / table / chart / diagram / code / asset`。
- `project/model/Outline` 对应 `.outline.json`。根维护有序的直属页和章列表，最多一个列表非空。`slideIds()` 按大纲顺序收集根、章、节中的页面 ID。
- `Outline.java` 在 `Outline` 内声明 `Chapter`、`Section` 和 `Page`。
- `Outline.Chapter` 保存章 ID、标题、有序的直属页面和有序的节。[大纲字段](API.html#projects)
- `Outline.Section` 保存节 ID、标题和有序页面。章和节均以页面 ID 和标题引用页面。[大纲字段](API.html#projects)
- `project/model/Constraint` 对应 `.constraint.json`，分别保存内容约束和视觉约束的要求与禁用项。
- `project/model/ChangeTarget` 是文稿变更目标的值对象，包含 `part`、项目内相对路径 `path` 和页面目标的 `slide_id`。
- `project/model/DeckEditCandidate` 表示一个 `ChangeTarget` 的待提交修改，携带待写入的完整文件内容或明确的删除操作。
- `DeckEditCandidate` 的读取基准记录原文件哈希或文件当时不存在，供 `DeckService` 在提交前持锁复核。
- `project/model/DeckChange` 表示一项文稿变更，包含 `ChangeTarget`、`ChangeStatus` 和由 `DiffType` 区分的差异明细。[变更字段](API.html#history)
- `project/model/enumeration/ChangePart` 对应 `ChangeTarget.part`。项目级取 `meta / constraint / outline`，页面级取 `spec / html`。
- `project/model/enumeration/ChangeStatus` 表示文稿目标的新增、修改或删除，取 `added / modified / deleted`。审批候选、工具实际提交和 Run 净修改使用同一组取值。
- `project/model/enumeration/DiffType` 决定文稿差异明细的结构，取 `fields / outline / text / binary / unavailable`。[差异与工具详情](API.html#history)
- `project/model/enumeration/OutlineNodeType` 标识大纲差异行的节点，取 `chapter / section / slide`。
- `project/model/enumeration/DiffRowType` 标识差异行，字段差异使用 `added / removed`，大纲和文本差异还可使用 `context`。[差异字段](API.html#history)
- `project/model/enumeration/DecorationType` 标识装饰种类，取 `slide_pagination / deck_title / chapter_title / logo`，项目外观与 DOM 选区共用。每种装饰的位置由 `DecorationPosition` 表示。[项目外观](API.html#projects) · [DOM 引用](API.html#runs)
- `project/model/entity/ExportSnapshot` 是在项目锁内采集的渲染输入，保存项目管理名称、文稿标题、按页序排列的页面标题与 HTML、每页所属章的标题、可为空的主题 CSS、装饰位置和 Logo 字节。`ExportFile` 携带 worker 返回的成品文件名、字节数和输入流。[导出契约](API_SPEC.md#导出)
- `IProjectRepository` 负责项目查询与创建、改名、删除，文稿提交及恢复由文稿仓储负责。
- `domain/shared/gateway/IFileGateway` 接收调用方解析好的文件或目录 `Path`，写入时另传 `byte[] content`。文稿、附件和缩略图共用此接口。列举返回目录直属的普通文件名。`DeckService` 保存大纲时按文件名顺序识别并删除已移出大纲的页面文件，`ThumbnailService` 选择需要清理的旧 WebP。[项目文件布局](#项目目录)
- `infrastructure/gateway/FileGateway` 新建和替换都先完整写入同目录临时文件并刷盘。新建以硬链接创建正式文件名，目标已存在或文件系统不支持硬链接时失败。替换通过原子移动发布。写入完成后清理临时文件名并同步父目录。
- `agent/model/entity/` 下的持久化实体有 `Session`、`Run`、`Message`、`Command` 和 `Checkpoint`。
- `agent/model/entity/PendingInteraction` 是由 `InteractionService` 管理的内存实体，以 `run_id + tool_call_id` 标识一项待答交互，持有 `InteractionType`、当前表单及其 `version`。[待答与答复](API_SPEC.md#人工)
- `agent/model/` 存放会话的 `Plan` 和 `PlanStep`。
- `agent/model/RunConfig` 由 `RunService` 在受理时根据全局设置组装，包含本次 Run 的主模型、旁路及备用模型、审批开关和自动命令开关，由 `RunState` 持有并在整次 Run 中复用。[受理与执行材料](WORKFLOW_SPEC.md#新开-run)
- `agent/model/RunScope` 表示已生效的页面授权。`InputService` 在受理时将 `chapter_ids`（章）或 `section_ids`（节）展开为页面 ID，`ToolGuard` 按生效范围核对目标页。
- `agent/model/RunChangeSet` 按 `ChangeTarget` 合并归属本 Run 的已提交 `DeckChange`，抵消本 Run 自身撤销的修改，供独立审查和 Run 收尾使用。[净修改归属](DATABASE_SPEC.md#runs)
- `agent/model/ContextEstimate` 保存一次完整请求的上下文 Token 总量与分类明细，由 `ContextService` 整体更新，供压缩判断和会话状态查询使用。[会话上下文](API.html#sessions)
- `AppendRunDto` 保留 `scope` 是否出现。追加指令时，省略则沿用当前 `RunScope`，显式 `null` 授权全部页面，显式 `slide_ids=[]` 不增加授权。[执行契约](API_SPEC.md#执行)
- `agent/model/InstructionReference` 表示一条引用。指令按输入顺序保存引用，供提示词拼装和回退回填使用。
- `agent/model/DomSelection` 是 DOM 引用的选区内容，保存页面与选区标识、标注序号、提交时的 HTML、源码哈希、注释及元素或装饰定位。[选区字段](API.html#runs)
- `agent/model/enumeration/InstructionReferenceType` 对应 `references[].type`，取 `skill / component / slide / image / dom`。追加输入只接受 `image` 和 `dom`。[引用字段](API.html#runs)
- `agent/model/CheckpointSnapshot` 保存五类文稿、有效计划、有序上下文消息 ID 和当前分支的历史位置。`Checkpoint` 按消息 ID 定位用户发送前或最终回复后的快照。原始输入从关联指令回填，返回快照只记录恢复材料。[快照范围](DATABASE_SPEC.md#checkpoints)
- `asset/model/` 存放资产、标签及其类型。
- `IAssetRepository` 负责资产、标签及关联的持久化。
- `setting/model/` 存放模型配置和各组全局设置。
- `ISettingRepository` 负责分组设置的持久化。
- `domain/shared/gateway/IPlatformGateway` 提供本地应用选择和按路径打开文件的能力，由 `infrastructure/gateway/PlatformGateway` 实现。
- `DeckService.openFile` 从 `SettingService` 读取偏好，按文件后缀和继承规则确定打开方式。`IPlatformGateway.openFile(path, application)` 接收已校验的文件路径和应用路径，`application` 为 `null` 时使用系统默认应用。`SettingController` 直接调用 `IPlatformGateway.pickApp()` 选择应用。[文件打开契约](API_SPEC.md#其它)
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

- 创建项目只写 SQLite。首次写入文稿或附件时创建独立的 `{project_id}/` 目录，由同一工作目录下的单个后端进程管理。
- 删除带文件的项目时，在事务内将项目目录原子改名为同级的 `.deleting-{project_id}/`。事务提交后清除暂存目录，回滚时恢复原名。若进程中断或事务结果未知，`FileStorer` 在启动时遍历项目根目录，将条目名传给 `PathService.projectIdFromDeletingDirectory` 取得项目 ID，再按 SQLite 中项目记录是否存在决定恢复或清除。删除标记的 ID 为空、`.` 或 `..` 时，记录警告、保留该条目并继续处理其余项目。
- 项目级文稿存于根目录的 `.meta.json`、`.constraint.json`、`.outline.json`。
- 单页规格和源码按幻灯片 ID 存为根目录的 `{slide_id}.json` 和 `{slide_id}.html`。
- 文稿回退和返回只处理上述五类文件，其他项目文件保留原状。
- 项目主题正文和 Logo 存在数据库中，项目目录不另存永久副本。
- `.git/` 存放项目 Git 仓库，`.runtime/` 存放项目运行文件。
- 人工保存由 `DeckService` 受理，通过 `IProjectRepository.commitFiles` 提交文稿变更。项目锁覆盖基准校验、记录旧内容、修改文件和业务事务。内存中的 `project/model/entity/FileChange` 包含文件名和前后字节，用于落盘及计算实际差异。后续接入 Agent 写入时须在项目锁内复核授权和文件基准，接入历史恢复时须在锁内复核目标和会话版本。[工作流规范](WORKFLOW_SPEC.md)
- `ProjectRepository` 通过 `FileUndoMapper` 把整批旧内容写入 SQLite 的 `file_undo`，然后调用 `IFileGateway` 原子替换或删除文稿。业务结果与删除该项目撤销记录在同一事务提交。
- 启动时按 `file_undo.project_id` 查找待恢复项目，在项目写锁内还原整批文件。恢复失败的项目保留记录并暂停写入，其他项目可继续使用。[撤销表](DATABASE_SPEC.md#file_undo) · [启动恢复](WORKFLOW_SPEC.md#启动恢复)
- `.runtime/attachments/{attachment_id}.png` 保存附件原图，同名 `.webp` 保存附件缩略图。
- `.runtime/screenshots/{screenshot_id}.png` 保存调用当时的截图，截图 ID 全局唯一，已有文件不覆盖。
- 附件原图、附件缩略图和历史截图保留至项目删除，已上传但未发送的附件适用同一规则。
- `.runtime/thumbnails/{slide_id}/{render_hash}.webp` 保存页面预览。哈希覆盖 HTML、主题、装饰、Logo 及影响画面的内置资源。命中时直接读取 WebP，缺失时渲染并从内存中的 PNG 生成 WebP。
- 同页预览串行处理，缓存命中即返回。渲染期间释放项目锁。对仍存在的项目，持项目锁按取样时的哈希原子保存 WebP，再清理同页旧版本。
- 截图评分及复用所需的基准存入数据库的工具结果，评估中的临时状态仅保留在内存。

### 渲染与导出

- `ThumbnailService` 通过 `IRenderGateway.renderHash` 取得渲染哈希。`RenderGateway` 为缩略图和整稿导出组装相同的页面 HTML、基础及主题 CSS 和装饰参数。文稿装饰标题取 `Meta.title`，章标题取该页所属章，根直属页省略章标题。
- `RenderGateway` 调用外部 Node worker。开发时在 `backend_java/render-worker/` 执行 `pnpm install --frozen-lockfile`，从 `backend_java/` 启动 Java，默认脚本为 `render-worker/worker.mjs`。`PPT_RENDER_WORKER` 和 `PPT_RENDER_NODE` 可指定脚本和可执行文件。
- 单页截图使用 `--job <请求文件>`，worker 按给定的页面输入写入 PNG 并返回版面检查结果。
- 项目导出使用 `--export`。Java 向标准输入一次写入格式、项目名称、按序排列的页面及共用外观。worker 生成完整文件后向标准输出依次写入四字节大端长度、该长度的 UTF-8 JSON（`filename`、`size`）和原始文件字节。Java 将文件流直接写入本次 HTTP 响应。
- HTML 由 worker 将页面和装饰嵌入同目录的 `export-player.html`，可直接播放。PNG 和 PDF 复用一次 Chromium 实例逐页截图，依序打包为 ZIP 或合成为 PDF。所需临时文件由 worker 在系统临时目录中创建并在本次调用结束时删除。
