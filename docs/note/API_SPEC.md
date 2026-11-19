[接口表](API.html#api-heading)。

- 业务接口使用 `/api` 前缀。
- `GET` 用路径参数定位资源、用查询参数指定读取选项。
- `POST` 的业务参数全部放入请求体，默认使用 JSON，上传文件使用 `multipart/form-data`，字段使用 `snake_case`。
- 普通业务响应使用 HTTP 200。图片、下载、静态资源、SSE 建连及健康检查使用各自的 HTTP 状态码。
- 成功时直接返回业务对象、含 `items` 的列表对象或空响应体。失败时返回统一错误格式 `{"error":{"code":"…","message":"…"}}`。
- ID、版本、哈希、`seq` 和字符串枚举按字符串传输。会话版本和 `seq` 使用十进制字符串。
- 局部更新时，省略表示保留原值，`null` 仅适用于可空字段，`[]` 表示空集合。整组保存的接口另行注明。

## 项目

- 项目列表按 `updated_at` 降序、`id` 升序返回，使用 `Cache-Control: no-store`。
- `POST /api/projects/save` 省略 `project_id` 表示创建，传入则修改已有项目的名称。两者都返回项目 `id`。
- `GET /api/projects/{project_id}` 一次返回资料、大纲及大纲内各页的规格、HTML 和对应哈希。
- 普通项目读取允许不同文件来自相邻两次提交。每个文件的正文和哈希取自同一次读取。
- 规格损坏时正文为 `null`，仍返回其哈希。文件缺失时正文与哈希均为 `null`。
- 项目内有非终态 Run、正在 Git 提交或历史切换时，人工保存文稿返回 `PROJECT_BUSY`。
- 导出采集快照时，人工保存等候项目锁。快照固定后即可编辑。
- 每次保存提交完整正文。保存成功返回新 `hash`，基准不符返回 `CONTENT_CONFLICT` 并保留当前内容。
- 规格和 HTML 分别使用 `hashes.slides[id].spec` 和 `.html` 作为 `base_hash`。`base_hash=null` 仅在文件尚不存在时匹配。
- 大纲保存提交整棵树，大纲与相关文件的修改一起生效。
- 新节点由客户端分配唯一 ID，已有节点改名、排序或移动时沿用原 ID。
- 从整棵大纲移除幻灯片时，同时删除其规格和 HTML。大纲中仍有该 ID 时，保留对应文件。
- 非空章不能直接删除。删除含幻灯片的节，仅允许移除章内唯一的节，并将其全部幻灯片保留为章直属节点。
- `appearance/save` 至少提交 `theme_id`、`positions` 之一，均不接受 `null`。同时提交时，主题与位置一并生效。
- `positions` 须完整提交四项。启用的位置不可重叠，允许在一次保存中交换，`none` 表示隐藏。
- 新选择的主题必须已启用。已引用的主题停用后仍可渲染。

| 资源 | 响应格式 | 读取失败 |
|---|---|---|
| Logo | PNG | 图片不存在时返回 HTTP 404。 |
| 幻灯片缩略图 | WebP，对应当前源码和外观 | 幻灯片或 HTML 缺失时返回 HTTP 404，渲染服务不可用时返回 HTTP 503。 |

- 外观的 `logo` 表示是否已上传图片，隐藏 Logo 时保留图片。
- 上传、替换或删除 Logo 图片时，保留其显示位置。
- 替换 Logo 沿用原图片地址，重复删除成功。
- Logo 和缩略图均返回 `Cache-Control: private, no-cache` 和 `ETag`，`If-None-Match` 匹配时返回 HTTP 304。

## 资产

- 列表包含启用和停用资产。
- `query` 匹配名称、描述或标签名，短语还匹配正文。
- 搜索沿用列表顺序，依次按 `updated_at` 降序、`name` 升序、`id` 升序。
- `assets/save` 省略 `asset_id` 表示创建，传入则局部更新已有资产，至少提交一个可修改字段。
- 资产类型创建后固定。
- 更新正文须提交读取时的 `base_hash`，冲突时整次保存失败。创建或仅修改摘要字段时省略 `base_hash`。
- 正文为非空 UTF-8 原文。主题 CSS 上限为 64 KiB，短语上限为 16 KiB。
- 标签必须存在、同类且不重复。
- 短语名称须唯一，比较时忽略大小写。
- `hash` 只随正文变化，`updated_at` 只随资产实际变化。
- 已被项目引用的主题不能删除，修改其正文会改变引用项目的外观和缩略图。
- 编辑全局主题正文保留引用项目的检查点。

## 设置

- 文件打开、快捷键、模型与路由、Agent 行为均按组完整保存，成功返回空响应体。
- 文件选择器只返回应用路径，取消时 `path=null`。
- 移除自定义应用后，引用它的 `default_open` 恢复为 `system`，`json_open`、`html_open` 恢复为 `inherit`。
- 模型列表与路由一起保存。至少保留一个模型，路由引用保存后的配置名，可空路由显式传 `null`。
- 已有模型通过 `previous_name` 标识，未列入的模型会被删除。
- `key` 只写。新建时必填，更新时省略沿用原值，提供时须为非空白字符串。
- 运行中的 Run 及其自动命令沿用受理时的模型与路由。

## 导出

- 创建要求大纲有效且至少有一页。
- 返回任务 ID 时，本次导出的页序、内容、外观和 `total_slides` 已固定。
- 幻灯片不能引用会话附件或用户本地文件。HTML 成品可保留外部资源引用。
- 同一 `project_id + request_id` 和格式重发时返回现有任务 ID，改用其他格式返回 `REQUEST_CONFLICT`。
- 每个项目同时只允许一个活动导出。若项目有非终态 Run、正在 Git 提交或切换历史，新建导出返回 `PROJECT_BUSY`。
- 新请求通过创建校验后替换同项目尚未下载的 `ready` 任务。原任务仍在生成或下载时返回 `EXPORT_ACTIVE`。`failed`、`canceled` 均允许新建。
- 进度通过状态接口读取。HTML 可从 `snapshotting` 直接进入 `packaging`，组装期间继续更新 `completed_slides`。
- 生成超过 30 分钟时任务转为 `failed`。状态接口通过 `status=failed` 和 `error_message` 返回失败原因。
- `ready` 成品没有等待下载时限。
- 每个任务同时只接受一个下载请求。任务未就绪或正在下载时返回 HTTP 409，已失效返回 HTTP 410。
- 下载期间任务仍为 `ready`，完整发送后失效。
- 传输中断且成品完整时仍可下载，否则转为 `failed`。
- 下载使用 `Cache-Control: no-store`，以 `Content-Type`、`Content-Length` 和 `Content-Disposition` 返回格式、长度和文件名。
- HTML 是单文件，PNG 是 ZIP，PDF 是单个 PDF 文件。
- 取消可重复，下载期间拒绝取消。
- 项目删除或后端退出、重启后任务失效。状态接口查询已失效任务返回 HTTP 200 和业务错误 `EXPORT_GONE`。

## 附件

- 附件属于项目。图片引用须携带 `attachment_id` 和非空的显示名称 `original_name`。
- 读取 `original` 返回上传的 PNG 原文，默认的 `thumbnail` 返回最长边为 512 像素的 WebP。
- 工具截图按 `project_id + screenshot_id` 返回调用当时的 PNG。
- 附件和历史截图均保留至项目删除，图片不存在时返回 HTTP 404。

## 会话

- `sessions/save` 创建时传 `project_id`，可省略 `title` 以使用「新会话」。
- `sessions/save` 改名时传 `session_id` 和去首尾空白后非空的 `title`，省略 `project_id`。
- 创建和改名均返回会话 `id`，标题从会话列表读取。
- `can_return=true` 且标题实际改变时，改名须提交检查点列表的 `revision` 作为 `expected_revision`。缺少必填版本返回 `INVALID_ARGUMENT`。
- 改名请求只要携带 `expected_revision` 就校验版本，过期返回 `STALE_REVISION`。
- 实际改名成功后放弃「返回最新」，同名或失败时保留。
- 改名通知 `sessions`，返回入口变化时同时通知 `checkpoints`。
- 删除计划须提交 `expected_revision`。当前会话有非终态 Run 时返回 `SESSION_BUSY`。
- 删除计划成功后 `plan=null`，返回会话版本。若处于回退位置，同时放弃返回入口。
- 删除计划按实际变化通知 `plan`、`context`、`checkpoints`。
- 会话删除可重复。成功时该会话活动已结束，同项目其他会话的 Run 继续。
- 会话状态的 `context` 返回当前 Token 估算及分类明细。
- 空闲时按当前会话的 Create 模式估算，Run 结束后恢复该估算。
- 活动 Run 首轮请求组装完成前沿用空闲估算，之后总量和明细始终对应最近一次完整的主模型请求。
- 同一会话 `revision` 下估算可以变化。
- `current` 达到 `threshold_min` 可手动压缩，达到 `threshold_max` 触发自动压缩。
- 压缩进度和取消目标由 `active_command_ids` 中的 `compact` 命令提供。

## 消息

- 历史按 Run 或命令分组，每项仅含 `run`、`command` 之一。
- 首次取最新的 `limit` 组，后续取分组首条 `seq < before` 的最近一批，批内按 `seq` 升序。
- 还有更早记录时 `next_before` 为本批最小分组序号，否则为 `null`。
- 每批有独立的读取版本，已返回的分组仍可能更新。
- 历史只公开引用的显示信息和稳定 ID，DOM 原文仅在回退回填原始输入时返回。
- 历史只收录 `polish`、`handoff`、`compact`、`commit` 四类命令。
- `operation_id` 标识整个命令操作，`command.id`、状态、阶段和失败原因对应最新一次执行。
- `last_success_id` 和产物概况对应最近一次成功执行，可以与 `command.id` 指向不同执行。
- 命令详情只返回指定执行自己的结果，未成功时为 `null`。
- 命令详情的 `phase` 与 `command_progress` 使用下表的阶段编号。
- 命令终态或无活动阶段时，详情返回 `phase=null`。`suggest` 的 `phase` 始终为 `null`。

| 命令 | 0 | 1 | 2 |
|---|---|---|---|
| `polish` | 准备原文及修订要求 | 生成润色文稿 | 整理并提交结果 |
| `handoff` | 收集项目与会话材料 | 组装交接材料 | 生成交接文稿 |
| `compact` | 读取上下文 | 生成摘要 | 更新上下文 |
| `commit` | 暂存 | 分析 | 提交 |
| `rename` | 读取会话 | 生成名称 | 核对并写回名称 |

- Run 详情一次返回按 `seq` 升序的全部过程消息。指令与最终回复从历史分组读取，压缩摘要从成功的 `compact` 详情读取。
- Run 结束时 `activity`、`commentary` 为 `null`，`pending=[]`。
- `usage` 和历史 `total_tokens` 只统计主循环对话请求及其实际重试。取消保留已发生的用量，`call_count` 包含未报告 Token 数的请求。
- `run.changes` 只含本 Run 成功提交的净修改。差异接口按相同目标和顺序一次返回明细。
- 变更目标的 `part` 和 `path` 必返，`path` 为项目内相对路径。
- `part` 为 `meta / constraint / outline` 时省略 `slide_id`，为 `spec / html` 时返回所属页面的 `slide_id`。
- Run 变更概况和差异明细通过 `target` 返回目标，工具调用有主要目标时也使用 `target`。
- 工具结果 `result.targets[]` 与权限审批 `permission.changes[]` 将目标字段直接放在数组项上。
- 整份清单未知为 `null`，确认无变化为 `[]`，单个目标无法展示时为 `type=unavailable` 并附 `reason`。
- 工具详情使用 `tool_call` 消息的 `id` 查询。同次调用与结果按 `run_id + tool_call_id` 配对。
- `arguments` 无公开参数时为 `{}`，`result=null` 表示尚无已记录结果。
- 文件修改成功后，结果中的 `targets` 给出实际差异，`path` 可直接用于打开项目文件。
- 工具详情的 `interaction.request` 保留当时的表单内容，省略 `type` 和 `version`。
- 资料审批保留原 `request.proposal`，`answer` 仅含决定及可选的拒绝反馈。
- 资料编辑工具成功时，`result.content` 返回实际保存的完整定稿。受阻或失败时省略 `content`。
- `edit_meta`、`edit_constraints` 的定稿为对象，`edit_outline` 的定稿为含正式节点 ID 的 JSON 源码字符串。
- 预览评分完成时返回接口表定义的全部评分与判定项。评分缺失、越界或判定值无效时，`result.preview.status=unavailable` 并附 `reason`。
- SSE 按会话订阅，事件名在 `event`，`data` 为 UTF-8 JSON。
- `changed` 在变更可读取后发送，携带接收会话的 `revision` 和非空、无重复的 `areas`。

| `areas` | 对应的读取接口 |
|---|---|
| `content` | 项目文稿和项目外观。外观含 Logo 时，使用原地址和 ETag 重新验证图片。 |
| `history`、`run`、`commands` | 会话历史、Run 详情、命令详情。会话状态提供当前活动 ID。 |
| `plan`、`suggestions`、`context` | 会话状态。 |
| `checkpoints`、`sessions` | 本会话检查点列表、项目会话列表。 |

- 共享文稿或外观实际改变后，`content` 通知同项目所有已连接的会话，包括发起方。人工保存、Agent 写入、历史切换适用同一规则。同值保存或失败时不发 `content`。
- `history`、`plan`、`checkpoints` 只通知各自数据发生变化的会话。
- 仅增加工具过程消息时通知 `run`。最终回复、变更概况或 Run 终态改变时同时通知 `history` 和 `run`。
- 命令修订和结果影响历史分组时同时通知 `commands` 和 `history`。
- 版本仅在同一会话内有意义。同版本仍可能收到 `content`、`context` 或实时进度，先到的一个区域通知不能替代另一区域的通知。
- 订阅从建连时开始，不补发旧事件。首次同步先订阅再读取快照。
- 重连或 `reset` 后原快照和分页游标失效，重新读取项目文稿、外观、会话列表和状态、所需的活动详情，以及当前会话的历史和检查点。
- 会话删除后订阅结束。

## 执行

- `runs/append` 新开时传 `session_id`、`mode`、`model`、`scope`。
- 追加时改传 `run_id`，沿用模式和模型。仅 `running`、`waiting` 且输入开放的 Run 接受新追加。
- 同一会话最多有一个非终态 Run，与该会话的手动压缩互斥。同项目不同会话可以并行。
- 项目正在历史切换、Git 提交或执行排他命令时拒绝新开。
- `message_id` 由客户端生成，全局唯一。新开和追加各使用独立的消息 ID。
- 同一会话同参重发返回原 Run ID，追加入口关闭后仍可重发。原 Run 已被回退移除时返回 `RUN_NOT_FOUND`。
- 已使用的 `message_id` 用于其他会话或不同请求时返回 `IDEMPOTENCY_CONFLICT`。
- `text` 去首尾空白后为空时，有 DOM 选区须至少一条选区注释非空，无 DOM 选区须附图片。
- 追加仅接受图片和 DOM 引用。附件须属于当前项目，引用的幻灯片和 DOM 源码基准须有效。
- `scope=null` 授权全项目。范围对象只含 `current_slide_id`、`slide_ids`、`section_ids`、`subsection_ids` 之一。
- `section_ids` 选章，包含章直属页面和下属各节的页面。`subsection_ids` 选节，只包含所选节的页面。
- 章、节在受理时按大纲顺序展开为页面 ID。
- 引用本身不扩大授权。
- 追加省略 `scope` 沿用当前范围，显式提交与原范围取并集。实际扩大授权后通知 `run`、`history`。
- `slide_ids=[]` 在新开时表示未授权任何页，在追加时表示不增加范围。
- 指令 `queued` 表示待处理，`sent` 表示已处理。
- 闲聊在同一 Run 内答复。
- `disclosure`、`bypass`、`irrelevant` 拒绝当前输入。
- 首条被拒时 Run 转为 `rejected`。已受理但尚未处理的追加输入以 `queued` 留在原 Run 历史，处理到此结束。
- 追加被拒时，该条在历史中标为 `sent` 并返回 `instruction_decision`。已授权范围保留，后续输入继续在同一 Run 内按序处理。
- 输入判定失败时 Run 转为 `failed`。
- 取消仅作用于指定 Run，可重复提交。空响应体表示取消请求已受理或 Run 已终态，最终状态以 Run 详情为准。
- 后端重启后原 Run 不续跑。
- `commands/append` 首次提交 `session_id`、`type`，仅 `polish` 另传 `text`。
- 修订仅支持 `polish`、`handoff`，改传 `base_id` 和 `feedback`。
- 两种形式均须 `request_id`，每次执行有独立 ID，修订沿用 `operation_id`。
- 同会话内同一 `request_id`、同参重发返回原执行 ID。异参返回 `IDEMPOTENCY_CONFLICT`，原执行已被回退移除时返回 `COMMAND_NOT_FOUND`。
- 修订的 `base_id` 须指向最近成功执行。该执行不存在时返回 `COMMAND_NOT_FOUND`，已过期时返回 `STALE_BASE`。
- 同一操作同时只允许一个活动执行。
- 进度和取消均使用单次执行 ID。取消可重复，最终状态以命令详情为准。
- 手动 `compact` 同一会话最多一个。
- `commit` 要求项目内无非终态 Run，提交期间与新 Run、历史切换和导出创建互斥。
- `rename` 的原标题已改变时，该次结果为 `cancelled`，保留当前标题。
- `suggest` 的依据输入已过期或已有更新的建议命令时，该次结果为 `cancelled`，保留当前建议。

## 人工

- 以 `run_id + tool_call_id` 读取当前表单，答复须提交该表单的 `version`。
- 每项只受理一次。事项关闭或已答复返回 `INTERACTION_CLOSED`，待答内容变化返回 `STALE_INTERACTION`。
- 答复成功的空响应体仅表示已受理，执行结果从工具详情读取。
- 批准计划后，同一 Run 由 `plan` 切到 `create`，会话计划更新，通知 `run`、`plan`。拒绝时保留原模式和已有计划。
- 命令权限的 `allow_once` 只批准表单中展示的命令及候选修改。
- 资料审批的 `approve` 可提交完整 `draft`，省略则采用原 `proposal`。`draft=null` 或拒绝时携带 `draft` 均无效。基准已改变或审批失败时保留当前内容。
- 范围审批与当前授权合并，`applied_scope` 返回实际生效的范围。
- 追加授权已满足提议时，事项关闭，历史中的 `answer=null`。仅满足一部分时，更新表单及 `version`。
- 待答事项创建、变化或关闭时发送 `interaction`，最新待答列表从 Run 详情的 `pending` 读取。
- 工具结果可读取时通知 `run`。

## 检查点

- `run_ids` 只列当前历史中可回退的 Run。`can_return` 表示能否返回首次回退前的最新状态。
- 回退预览的 `timestamp` 是目标 Run 的创建时间，返回预览使用返回目标的最后活动时间。
- 回退和返回都须提交对应预览的 `revision` 作为 `expected_revision`，版本过期返回 `STALE_REVISION`。
- 回退和返回按目标快照覆盖文稿文件，外部编辑也会被覆盖。
- 历史主题已删除时整次切换失败。已停用的主题仍可恢复，主题正文取该资产的当前版本。
- `rollback` 返回版本及可回填的原始输入。连续回退保留同一返回目标。
- `return` 成功时响应体为空，随后 `can_return=false`。
- 回退和返回均恢复目标位置的文稿、外观及本会话数据，附件与历史截图保留。
- 项目内有非终态 Run、目标会话有非终态命令或项目正在执行互斥操作时，切换返回 `PROJECT_BUSY`。切换期间暂停项目新 Run 及目标会话新命令的受理。
- 可返回最新时，新开 Run、发起非 `polish` 命令或修订 `handoff`，须传检查点列表的 `revision` 作为 `branch_revision`。
- 缺少分支确认返回 `BRANCH_CONFIRMATION_REQUIRED`，`branch_revision` 过期返回 `STALE_REVISION`。
- 需分支确认的请求受理成功后，放弃返回目标及已脱离当前历史的检查点。受理失败时保留。
- 发起或修订 `polish` 保留返回入口。
- 可返回最新期间不自动发起旁路命令。
- 历史位置手动改名和删除计划使用 `expected_revision`。成功且有实际修改时，同样放弃返回目标及已脱离当前历史的检查点。
- 人工修改共享文稿或外观会使项目全部会话的检查点和返回目标失效。
- 会话实际改变共享文稿或外观时（含回退或返回），同项目其他会话的检查点和返回目标失效。
- 检查点失效时，受影响会话的版本随之更新，活动 Run 继续。
- 历史切换后通过 `reset` 通知受影响的会话。

## 预览

- 预览资源按接口表返回原文或二进制，未知资源返回 HTTP 404。
- 预览资源使用 `Cache-Control: no-cache` 和 `ETag`，条件请求命中返回 HTTP 304。

## 其它

- `GET /api/healthz` 在启动检查期间或公共数据库不可用时返回 HTTP 503。
- 启动检查完成且公共数据库可用时返回 HTTP 200，两种状态的响应体均为空。
- 单个项目有未解决的恢复日志时，该项目暂停写入，其他项目正常使用。
- `POST /api/files/open` 仅接受同源本地调用。`path` 为指定项目内可打开文件的相对路径，解析后须仍位于项目内。
