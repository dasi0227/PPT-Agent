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
- `infrastructure/persistence/po/` 为每张业务表定义一个 `XxxPo`。
- `infrastructure/persistence/mapper/` 为每张业务表定义一个同名前缀的 `XxxMapper` 接口。
- Mapper XML 位于该模块的 `resources/mapper/`。
- SQLite `TEXT` 列在 PO、Mapper 参数及结果映射中统一使用 `String`，包括状态、类型、模式和协议。
- `infrastructure/persistence/ReceiptType` 定义数据库凭据类型 `create_run / create_command / agent / manual`，由仓储与 `ReceiptPo.type` 的字符串值相互转换。[凭据类型](DATABASE_SPEC.md#receipts)
- JSON 存储列按 `String` 读写，仓储负责与领域对象转换。
- `infrastructure/filesystem/` 存放文件系统实现，外部能力的适配实现分别位于 `llm/`、`jev/`、`git/`、`platform/` 和 `worker/`。
- `infrastructure` 模块的 `resources/render/` 存放随应用分发的内置字体、样式、脚本和预览样例。

- `interfaces/controller/` 按功能分为 `project`、`asset`、`setting`、`export`、`file`、`session`、`history`、`execution`、`interaction`、`checkpoint` 和 `runtime`。
- 每个功能包就近存放 `XxxController.java`、请求类型 `dto/` 和响应类型 `vo/`。
- 健康检查的 `HealthController.java` 位于 `controller/` 根目录。
- 项目文件打开接口归 `controller/file/`。
- 会话历史、Run 与命令详情、工具详情和会话订阅归 `controller/history/`。
- Run 与旁路命令的受理及取消入口归 `controller/execution/`。
- `interfaces/handler/` 存放 `GlobalExceptionHandler.java` 和 `ErrorResponse.java`。
- `interfaces/sse/` 存放领域接口 `EventPublisher` 的实现 `SseEventPublisher.java`。

- `domain/` 下的业务包为 `project/`、`agent/`、`asset/` 和 `setting/`。
- `domain/shared/` 与业务包平级，`exception/` 存放共用异常与错误码，`constant/` 和 `util/` 存放跨领域常量和辅助函数。
- 每个领域的 `model/` 存放值对象及业务输入和结果，`model/entity/` 存放实体，`model/enumeration/` 存放枚举。
- 领域仓储接口位于各自的 `repository/`，外部能力接口位于各自的 `gateway/`。
- `project/model/entity/` 存放项目实体 `Project` 和页面实体 `Slide`。
- `Slide` 以稳定的 `slide_id` 标识一页，关联该页的 `Spec` 和 HTML 源码。
- `project/model/Deck` 按所属项目聚合 `Meta`、`Constraints`、`Outline` 及按 `slide_id` 索引的 `Slide`。[项目读取契约](API.html#projects)
- `project/model/Meta` 对应 `.meta.json`，保存文稿标题、语言、页数要求、受众和目标。
- `project/model/Spec` 对应 `{slide_id}.json`，描述单页的用途、核心信息、内容元素和布局。
- `project/model/Outline` 对应 `.outline.json`，维护章、节与页面的层级、顺序、标题和稳定 ID。
- `project/model/Constraints` 对应 `.constraints.json`，分别保存内容约束和视觉约束的要求与禁用项。
- 项目装饰及导出任务模型同属 `project/model/`。
- `IProjectRepository` 负责项目查询与创建、改名、删除，文稿提交及恢复由文稿仓储负责。
- `agent/model/entity/` 存放 `Session`、`Run`、`Message`、`Command` 和 `Checkpoint`。
- `agent/model/` 存放会话的 `Plan` 和 `PlanStep`。
- `agent/model/RunScope` 表示已生效的页面授权，供 `InputService` 合并范围、`ToolGuard` 核对目标页。
- 追加请求的 DTO 保留 `scope` 是否出现。省略时沿用当前 `RunScope`，显式 `null` 授权全部页面，显式 `slide_ids=[]` 不增加授权。[执行契约](API_SPEC.md#执行)
- `agent/model/InstructionReference` 表示一条 `skill / component / slide / image / dom` 引用。指令按输入顺序保存引用，供提示词拼装和回退回填使用。[引用字段](API.html#runs)
- `agent/model/CheckpointSnapshot` 是 `Checkpoint` 的恢复内容，包括目标会话记录与 `agent` 凭据、五类文稿文件、主题 ID、装饰位置及 Logo 字节。普通检查点另含用于回填的原始输入。[快照范围](DATABASE_SPEC.md#checkpoints)
- `asset/model/` 存放资产、标签及其类型。
- `IAssetRepository` 负责资产、标签及关联的持久化。
- `setting/model/` 存放模型配置和各组全局设置。
- `ISettingRepository` 负责分组设置的持久化。
- `domain/agent/gateway/` 存放 `IBashGateway` 和 `EventPublisher`，Bash 实现位于 `infrastructure/worker/BashGateway.java`。
- `agent/model/enumeration/` 存放工具与 Harness 的专属错误码。
- `agent/model/enumeration/InstructionDecision` 表示输入判定结果。有效结果为 `task / gossip / disclosure / bypass / irrelevant`，未判定或检测失败时字段为空。[指令判定字段](DATABASE.html#messages-instruction-decision)
- `agent/model/enumeration/InteractionType` 标识交互表单类型，待答项与工具结果共用 `question / plan / permission / scope / resource`。[人工交互契约](API_SPEC.md#人工)
- `agent/model/enumeration/ToolResultStatus` 标识已记录的工具结果为 `completed / blocked / failed`。结果尚未记录时，工具详情返回 `result=null`。[工具详情](API_SPEC.md#消息)
- `agent/model/event/` 存放生命周期事件。
- `agent/model/result/` 存放生命周期阶段结果及 `ToolResult`。
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
- HTTP 请求与响应类型留在 `interfaces`，数据库 PO 及基础设施配置类型留在 `infrastructure`。
- 第三方依赖的版本由父 POM 或其引入的 BOM 管理，使用依赖的模块在自身 POM 声明。[后端依赖表](STRUCTURE.html#dependencies-heading)

## 项目目录

- 每个项目使用独立的 `{project_id}/` 根目录，由同一工作目录下的单个后端进程管理。
- 项目级文稿存于根目录的 `.meta.json`、`.constraints.json`、`.outline.json`。
- 单页规格和源码按幻灯片 ID 存为根目录的 `{slide_id}.json` 和 `{slide_id}.html`。
- 文稿回退和返回只处理上述五类文件，其他项目文件保留原状。
- 项目主题正文和 Logo 存在数据库中，项目目录不另存永久副本。
- `.git/` 存放项目 Git 仓库，`.runtime/` 存放项目运行文件。
- `.runtime/mutations/{operation_hash}.json` 保存一次涉及文稿文件的操作日志。
- `type=agent` 和 `type=manual` 的日志保存原凭据键 `(type, owner_id, request_id)` 及 `request_hash`。
- `type=history_switch` 的日志保存目标 `session_id` 和切换前的 `base_revision`（十进制字符串）。回退和返回最新共用此格式。
- 文件名中的 `operation_hash` 取自操作的唯一键。文稿写入使用原凭据键，历史切换使用 `(history_switch, session_id, base_revision)`。
- `files` 每项记录唯一的项目相对路径 `path` 及其 `before`、`after`，路径限于本次实际改动的五类文稿文件。
- 文件状态用 Base64 字符串保存完整字节，`""` 表示空文件，`null` 表示路径不存在。
- 完整日志在第一个文稿文件改动前原子落盘。
- 文稿文件与数据库核对一致后，持久移除日志。
- 恢复时无法判定应保留哪一版，或文件恢复失败时，保留日志并暂停该项目写入。[启动恢复](WORKFLOW_SPEC.md#启动恢复)
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
- 导出使用创建时固定的完整输入，页序、逐页标题与 HTML、项目标题、主题 CSS、装饰和 Logo 字节只在任务内存保留。
- PNG 和 PDF 导出的逐页图片存入任务的 `work/{序号}-{标题}.png`，序号补零，标题规范化。
- HTML 导出直接生成单文件，播放器、字体、样式、脚本和 Logo 内嵌，项目内不创建播放目录。
- 导出成品写完并关闭后清理 `work/`，保留待下载成品。
- 导出失败或取消时，工作停止后清理临时文件和不完整成品。
- 完整发送下载文件后清理整个导出任务目录，启动恢复时清理遗留导出目录。
