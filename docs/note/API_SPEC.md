[接口表](API.html#api-heading)。

- 业务接口使用 `/api` 前缀。
- `GET` 用路径参数定位资源、用查询参数指定读取选项。
- `POST` 的业务参数全部放入请求体，默认使用 JSON，上传文件使用 `multipart/form-data`，字段使用 `snake_case`。
- 普通业务响应使用 HTTP 200。图片、下载、静态资源、SSE 建连及健康检查使用各自的 HTTP 状态码。
- 普通业务接口默认不单独设置 `Cache-Control` 响应头。图片、文件下载和静态资源的缓存策略由各接口单独约定。
- 项目名称的长度上限为 60，资产名称和模型配置名为 80，资产描述为 500。按提交字符串的 UTF-16 长度校验，前端可直接使用 `value.length`。首尾空白计入长度，校验通过后再去除。纯空白值无效。
- 成功时直接返回业务对象、含 `items` 的列表对象或空响应体。失败时返回统一错误格式 `{"error":{"code":"…","message":"…"}}`。
- ID、版本、哈希、`seq` 和字符串枚举按字符串传输。会话版本和 `seq` 使用十进制字符串。
- 项目、会话、资产和模型配置分别使用 `create` 创建、`update` 更新。创建请求仅接受创建字段，资源自身的 ID 由服务端生成。更新请求需提供已有资源的非空白 ID。各接口的请求字段见接口表。
- 作用域内只有一份的设置或内容使用 `save`，例如全局行为开关、项目大纲和单页 HTML。
- 局部更新时，可选字段省略或显式传 `null` 默认保留原值。`false` 和 `[]` 都属于已提交的值，分别表示假和空集合。若 `null` 表示清空、恢复默认等操作，在对应接口中单独说明。

## 项目

- 项目列表按 `updated_at` 降序、`id` 升序返回。
- `POST /api/projects/create` 仅接收 `title`，新项目默认无主题。`POST /api/projects/update` 接收 `project_id` 和 `title`，修改已有项目的名称。两者均返回与项目列表项相同的 `id`、去除首尾空白后保存的 `title` 及数据库中的 `updated_at`。规范化后的标题与原值相同时，保留原更新时间。
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
- `GET /api/projects/{project_id}/appearance` 返回 `theme_id`，值为 `null` 表示项目未选择主题。
- `POST /api/projects/theme/save` 需提交 `project_id` 和 `theme_id`，只保存项目主题。每次保存都校验目标资产存在、类型为主题、已启用且正文非空白，同值保存也需通过校验。
- `POST /api/projects/positions/save` 需在请求顶层提交 `project_id`、`slide_pagination`、`deck_title`、`section_title` 和 `logo`，只保存四项装饰位置。四项位置一并生效，允许在一次保存中交换。显示位置不可重叠，`none` 表示隐藏，可在多项中使用。
- 两个接口的字段缺失或为 `null` 时返回 `INVALID_ARGUMENT`，成功时返回 HTTP 200 和空响应体。
- 主题、装饰位置或 Logo 实际变化时，在同一事务中更新时间、清理项目所有会话的检查点和返回快照，并将受影响会话的版本各加一。事务提交后向项目会话发送 `content` 通知，检查点或返回入口发生变化的会话同时收到 `checkpoints`。合法的同值保存、重复删除 Logo 和失败请求均保留更新时间、检查点、返回入口与版本。

| 资源 | 响应格式 | 读取失败 |
|---|---|---|
| Logo | PNG | 图片不存在时返回 HTTP 404。 |
| 幻灯片缩略图 | WebP，对应当前源码和外观 | 幻灯片或 HTML 缺失时返回 HTTP 404，渲染服务不可用时返回 HTTP 503。 |

- 外观的 `logo` 表示是否已上传图片，隐藏 Logo 时保留图片。
- 上传、替换或删除 Logo 图片时，保留其显示位置。
- 替换 Logo 沿用原图片地址，重复删除成功。
- Logo 和缩略图均返回 `Cache-Control: private, no-cache` 和 `ETag`，`If-None-Match` 匹配时返回 HTTP 304。

## 资产

- 列表包含启用和停用资产，列表项的 `hash` 为 `null`。
- `query` 匹配名称、描述或标签名，短语还匹配正文。
- 搜索沿用列表顺序，依次按 `updated_at` 降序、`name` 升序、`id` 升序。
- `POST /api/assets/create` 需提供 `type`、`name`、`description` 和 `content`。新资产默认启用，`tag_ids` 省略或为 `null` 时使用空列表。
- `POST /api/assets/update` 需提供已有资产的 `asset_id` 和至少一个非 `null` 的可修改字段。省略或为 `null` 的可修改字段保留原值，`tag_ids=[]` 清空标签，`enabled=true` 启用资产，`enabled=false` 停用资产。
- 创建与更新均返回与列表项相同的字段，`hash` 为保存后的正文哈希。
- 资产类型创建后固定。
- 更新正文须提交读取时的 `base_hash`，冲突时整次保存失败。创建或仅修改摘要字段时省略 `base_hash`。
- 正文为非空 UTF-8 原文。主题 CSS 上限为 64 KiB，短语上限为 16 KiB。
- 创建标签需提交 `type` 和 `name`。名称去除首尾空白后需非空，保留原大小写显示。同一类型内按 Unicode 小写值比较，重名返回 `INVALID_ARGUMENT`，不同类型允许同名。成功返回新标签的 `id` 和 `name`。
- 删除标签需提交非空白的 `tag_id`。成功返回 HTTP 200 和空响应体。标签与其在所有资产上的关联在同一事务内删除，并更新受影响资产的 `updated_at`。资产正文和其他标签保留。ID 缺失或为空白时返回 `INVALID_ARGUMENT`，标签不存在时返回 `TAG_NOT_FOUND`。
- 标签必须存在、同类且不重复。
- 四类资产的名称均须在同一类型内唯一，不同类型允许同名。比较时去除首尾空白并转换为 Unicode 小写，更新时排除当前资产。
- `hash` 只随正文变化，`updated_at` 只随资产实际变化。
- 已被项目引用的主题禁止从启用改为停用，也禁止删除。两种操作均返回 `ASSET_IN_USE`，分别提示“项目正在使用该主题，无法停用”和“项目正在使用该主题，无法删除”。
- 修改主题正文会改变引用项目的外观和缩略图。
- 编辑全局主题正文保留引用项目的检查点。

## 设置

- 文件打开设置和快捷键各自按组完整保存。缺少任一字段、字段为 `null` 或字符串为空白时，返回 `INVALID_ARGUMENT`。`custom_apps` 可传 `[]` 清空，列表中的路径需为非空白字符串。保存快捷键、行为开关、单个路由以及删除模型成功时，响应体为空。
- `POST /api/settings/behavior/save` 按字段更新五个行为开关。`true` 开启，`false` 关闭，省略或传 `null` 时保留该开关的当前值。空对象或所有字段均为 `null` 时返回成功，设置保持原值。
- 五个输入触发符各取 `/~!@#$%^&*>\+` 中的一个字符，且互不重复。
- 操作快捷键由一个修饰键（`Primary`、`Option` 或 `Alt`）和一个普通键用 `+` 连接，例如 `Primary+ArrowLeft`。`select_suggestion` 只提交修饰键，默认为 `Option`，分别与 `Digit1`、`Digit2`、`Digit3` 组成建议快捷键。所有组合均需避开保留快捷键且互不重复，检查冲突时 `Option` 与 `Alt` 等价。
- `POST /api/settings/fileopen/pick` 在 macOS 选择 `.app`，在 Windows 选择 `.exe`。选中后返回应用绝对路径，取消时返回 `{"path":null}`。选择应用和保存打开设置均只接受本机同源调用。
- 应用路径需为当前平台的绝对路径，Windows 接受 `.exe`，其他平台接受 `.app`，后缀不区分大小写。路径无效时返回 `INVALID_ARGUMENT`。
- `default_open` 接受 `system` 或应用路径，`json_open`、`html_open` 还接受 `inherit`。合法路径若未在本次 `custom_apps` 中登记，`default_open` 回退为 `system`，`json_open`、`html_open` 回退为 `inherit`。
- `POST /api/settings/fileopen/save` 返回生效的整组设置，字段与 `GET /api/settings/fileopen` 一致。`custom_apps` 中的路径规范化后按首次出现的顺序去重。前端直接用响应替换表单值，以显示最终的打开方式和应用列表。
- `GET /api/settings/models` 一次返回可公开的模型列表和路由，密钥只写。路由引用稳定的模型配置 ID。
- `POST /api/settings/models/create` 创建一个模型配置，需提供 `name`、`protocol`、`provider`、`base_url`、`model` 和非空白的 `key`。配置名去除首尾空白后需全局唯一。
- `provider` 接受 deepseek、kimi、mimo、gpt、claude、gemini、qwen、minimax、glm、grok，创建时需显式指定，更新时省略或传 `null` 保留原值。单独修改 `model` 时保留已选品牌。
- `base_url` 创建时必填，更新时省略或为 `null` 则保留原值。提供时只校验字符串非空白。保存时去除首尾空白及地址末尾的 `/`，仅含斜杠的值保留。
- 创建和更新均返回保存后的单个公开配置，字段与 `GET /api/settings/models` 中的 `models[]` 一致。响应包含规范化后的 `base_url` 和用户选择的模型品牌 `provider`，密钥始终只写。前端按响应中的 `id` 添加或替换对应的模型卡片，无需为此重查模型列表。
- `POST /api/settings/models/update` 需提供已有模型配置的 `id`。可修改字段省略或传 `null` 时保留原值，包括原密钥。只传 ID 时返回当前公开配置。改名保留路由引用，未知 ID 返回 `MODEL_NOT_FOUND`。
- `POST /api/settings/models/delete` 通过 `id` 显式删除单个模型。最后一个模型不能删除，返回 `INVALID_ARGUMENT`。仍被任一路由引用时返回 `MODEL_IN_USE`，需先调整该路由。未知 ID 返回 `MODEL_NOT_FOUND`。
- `POST /api/settings/routes/save` 只接受 `route` 和 `model_id`。未声明的字段（如 `modelId`）、空白 ID 或非法路由名返回 `INVALID_ARGUMENT`，未知模型 ID 返回 `MODEL_NOT_FOUND`。
- `main_default`、`side_default` 需指定有效模型 ID。对 `main_fallback`、`side_fallback`，省略 `model_id` 或传 `null` 关闭备用模型。对 `rename`、`compact`、`commit`、`polish`、`handoff`、`suggest`，同样的操作恢复跟随 `side_default`。只有指定的路由项会更新。
- 运行中的 Run 及其自动命令沿用受理时的模型与路由。

模型配置创建与更新共用的成功响应示例

```json
{
  "id": "a2k9m4q1",
  "name": "主模型",
  "protocol": "responses",
  "base_url": "https://example.org/v1",
  "model": "gpt-4.1",
  "provider": "gpt"
}
```

`POST /api/settings/fileopen/save` 成功响应示例

```json
{
  "default_open": "system",
  "json_open": "/Applications/Editor.app",
  "html_open": "inherit",
  "custom_apps": ["/Applications/Editor.app"]
}
```

## 导出

- 创建要求大纲有效且至少有一页。
- 返回任务 ID 时，本次导出的页序、内容、外观和 `total_slides` 已固定。
- 导出和预览使用同一份 `runtime/base.css`，按基础样式、已选主题的 CSS、单页样式的顺序应用。未选主题时跳过资产查找和主题 CSS。
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

- `POST /api/sessions/create` 需提供所属项目的 `project_id`，可省略 `title` 以使用「新会话」。
- `POST /api/sessions/update` 改名时需提供 `session_id` 和去首尾空白后非空的 `title`。
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
- `edit_meta`、`edit_constraint` 的定稿为对象，`edit_outline` 的定稿为含正式节点 ID 的 JSON 源码字符串。
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
- 快照主题为 `null` 时恢复到未选主题的状态。快照引用的主题已删除时整次切换失败。历史恢复允许使用仍存在的已停用主题，正文取该资产的当前版本。
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

- 未选主题时使用 `runtime/base.css` 提供的排版、配色和间距，再应用单页样式。已选主题的 CSS 加载在基础样式与单页样式之间。
- `GET /api/runtime/{file}` 按文件名读取打包在 `runtime/` 根目录下的公开资源。响应类型按扩展名确定，CSS 为 `text/css`，JS 为 `text/javascript`，无法识别时使用 `application/octet-stream`。找不到可读文件时返回 HTTP 404。
- 预览资源使用 `Cache-Control: no-cache` 和 `ETag`，条件请求命中返回 HTTP 304。

## 其它

- `GET /api/healthz` 在启动检查期间或公共数据库不可用时返回 HTTP 503。
- 启动检查完成且公共数据库可用时返回 HTTP 200，两种状态的响应体均为空。
- 单个项目有未解决的恢复日志时，该项目暂停写入，其他项目正常使用。
- `POST /api/files/open` 只接受本机同源调用，提交 `project_id` 和项目内相对路径 `path`。目标需为普通文件，解析符号链接后仍需位于该项目内。支持 JSON、HTML/HTM、CSS、Markdown（.md）、文本（.txt）、PNG、WebP、JPEG、SVG、PDF 和 ZIP。
- 打开 JSON 使用 `json_open`，HTML/HTM 使用 `html_open`，其他文件使用 `default_open`。文件类型的配置为 `inherit` 时沿用 `default_open`，`system` 使用系统关联的应用。系统接受打开请求后返回空响应。文件缺失返回 `FILE_NOT_FOUND`，应用启动失败返回 `FILE_OPEN_FAILED`。
