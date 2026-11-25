[数据库字段表](DATABASE.html#database-heading) · [SQLite 建表脚本](../../backend_java/sql/schema.sql)

- 字符串枚举存为 `TEXT`，由写入方校验取值。
- 布尔值存为 `INTEGER`，由 `CHECK` 限定为 `0` 或 `1`。
- SQLite 连接启用外键约束。
- 初始化或恢复预置数据时保留已有的全局设置。
- 显式重置设置时，在同一事务内将所选字段更新为默认值。

## projects

- 项目创建时由 Service 设置 `updated_at`。按 ID 改名时，数据库取传入时间与原 `updated_at + 1` 的较大值，提交原名也递增。Service 按更新行数判断项目是否存在。
- 新项目的 `theme` 为 SQL `NULL`，表示未选择主题。非空值引用 `assets.id`。普通保存主题时在写事务内校验目标资产存在、类型为 `theme`、已启用且正文非空白，同值保存也需校验。
- `logo` 保存 PNG 字节，未上传时为 SQL `NULL`。
- 四个装饰位置使用列默认值，所有非 `none` 位置互不重复。
- 外观写入在项目写锁内开启写事务，先检查文件状态，再确认项目存在。有未完成的文件操作时优先返回 `PROJECT_BUSY`，项目缺失时返回 `PROJECT_NOT_FOUND`。
- 通过检查后，主题、装饰位置和 Logo 用 SQL 条件更新判定实际变更。任一装饰位置变化时，同一条语句更新四个位置列。更新影响零行表示同值保存。
- 外观实际变更时在同一事务内更新项目的 `updated_at`。各会话的检查点、返回快照和 `revision` 保留。
- 采集导出数据时，在同一项目写锁内读取文稿文件，并用同一数据库读事务取得项目标题、装饰位置和 Logo 字节。仅在选定主题时读取对应资产正文。

## sessions

- `revision` 是会话的提交版本，初始值为 `0`。
- 业务记录、检查点或历史恢复有变更时，同一事务内每个受影响会话的 `revision` 只递增一次。
- 会话业务数据与对应的 `revision` 在同一读事务取得。
- `revision` 只记录持久状态变更，实时上下文估算及项目通知本身不递增版本。
- `plan` 保存 `{run_id, steps}`，无生效计划时存 SQL `NULL`。
- 计划步骤状态为 `pending`、`processing` 或 `completed`，最多一步为 `processing`。
- 非空 `steps` 数组替换全部步骤，显式 `steps=null` 清空计划，缺少字段或空数组均无效。
- 计划、相关工具结果及对应的 Run 模式变更在同一事务提交。

## runs

- 对 `status` 为 `pending / running / waiting / cancelling` 的行建立 `session_id` 部分唯一索引，每个会话最多一个非终态 Run。
- `model` 独立保存受理时的配置名，不设模型配置外键，配置改名后保留历史值。
- `changes` 只汇总本 Run 已提交且未被自身撤销的净修改，每次文件提交时确定归属与实际差异。
- 每项差异分别保存目标 `target`、增删改类别和差异明细。
- `target` 保存文稿部位 `part` 和项目内相对路径 `path`，页面目标另存 `slide_id`。
- 差异明细按目标保存为文本、结构化字段、大纲或二进制差异，无法可靠归属的目标标为不可用。
- `changes` 未生成或整体不可用时存 SQL `NULL`，确认无净变化时存 `[]`。
- `changes` 与 Run 终态同事务保存，失败、取消或中断的 Run 也保留已提交的差异。
- `input_tokens`、`output_tokens`、`total_tokens`、`call_count` 只累计主循环对话模型的实际请求及其重试，每次请求只计一次。
- 取消 Run 时保留已累计的用量。
- `error` 只保存导致 Run 失败的错误。可继续执行的单次工具错误存入对应的 `tool_result`。

## messages

- `messages.seq` 与 `commands.seq` 在会话内共同唯一，由写事务逐条分配，创建后固定。两张表各自对 `(session_id, seq)` 建立唯一索引。
- `run_id` 非空时，消息通过 `(run_id, session_id)` 引用所属会话的 Run。
- `messages.id` 全局唯一，指令受理时固定 `session_id`、`run_id` 和 `seq`。
- 重复受理时先核对会话、请求类型和原始参数，包括正文、引用与显式授权范围。
- 指令记录保留显式授权的原值，区分省略 `scope` 和显式 `null`。指令与对应的 `runs.scope` 变更同事务写入。
- `instruction_status` 初始为 `queued`，处理完成时与判定结果一起提交为 `sent`。
- 追加指令被拒时，`instruction_decision`、`instruction_status=sent`、`context_position=NULL` 与会话版本递增同事务提交。
- 追加拒绝仅更新该指令的处理结果，保留原 `id`、`seq`、`run_id` 和正文，以及其余输入的状态、归属与顺序。
- Run 首条被拒、取消或异常结束时，已处理指令保留 `sent`，未处理指令在原 Run 保留 `queued`、`context_position=NULL`。
- 同一 Run 最多一条 `response`，与 `done` 或 `rejected` 同事务写入。失败、取消或中断时允许没有回复记录。
- `raw` 只保存后续模型调用必需的原生协议数据及来源，同一次原始输出仅保存一份。
- `tool_call` 在执行前保存原始请求，与所属会话的版本递增同事务提交。
- `tool_call` 与 `tool_result` 按 `(run_id, tool_call_id)` 配对，每种记录各最多一条。
- `tool_result` 保存最终审批决定、实际结果及单次差异，后续取消保留已确认的结果。
- 资料审批记录保留原始提案，答复只保存决定及可选反馈。
- `edit_meta`、`edit_constraint`、`edit_outline` 成功时，`tool_result` 的 `content` 保存实际写入的完整定稿，包括用户批准的修改。
- `context_position` 非空时在会话内唯一，SQL `NULL` 表示该消息不在当前上下文。
- 同一轮模型返回的所有 `tool_call` 都有配对结果后，该工具轮次才整体进入上下文。缺少结果的轮次保留历史记录，整轮的 `context_position` 为 SQL `NULL`。
- 压缩时，摘要写入与 `context_position` 调整同事务提交，保留原消息正文、引用和历史顺序。

## commands

- 同一 `operation_id` 的记录属于同一会话和 `type`，每次执行有独立的 `id`、`seq` 和终态。
- `run_id` 非空时，命令通过 `(run_id, session_id)` 引用所属会话的 Run。
- 对 `pending / running / cancelling` 行的 `operation_id` 建立部分唯一索引，每个操作最多一个活动执行。
- 手动压缩由 `type='compact' AND run_id IS NULL` 标识，部分唯一索引限制同一会话最多一个活动手动压缩。
- 手动压缩的占用校验与受理记录同事务提交。
- 修订的 `base_id` 指向同一操作最近成功的执行，修订结果写入新记录。
- `rename` 仅在当前会话标题仍与受理时的原标题一致时写回，比对基准只保留在执行内存中。
- `suggest` 以受理时的最后一条用户指令 ID 为比对基准，无输入时为空，自动建议取触发 Run 的最后输入。
- 仅当依据的输入仍为最新、且本次执行的 `seq` 仍是会话内所有 `suggest` 的最大值时，才写回建议。
- 写回条件与结果在同一事务核对并提交，条件失效时存 `status=cancelled`、`result=NULL`，保留原有标题或建议。
- 查询当前建议时，取最新用户指令之后最近一次成功的 `suggest` 记录的 `result.requests`。无输入时取最近成功结果，无匹配时取 `[]`。
- 压缩摘要正文只存入 `messages`，`commands.result` 保存摘要引用。
- 命令产物、相关业务变更和 `done` 状态同事务提交。

## checkpoints

- 普通检查点对非空的 `(session_id, run_id)` 建立唯一索引。
- 返回快照以 `run_id IS NULL` 标识，`session_id` 的部分唯一索引限制每个会话最多一条。
- `session_id` 随会话级联删除。
- `run_id` 在写入时校验 Run 归属，不设 Run 外键。回退移除 Run 后可保留其检查点，当前可回退目标只取仍在历史中的 Run。
- 普通检查点保存 Run 开始前的业务与文件基准，在项目写锁下与 Run、首条消息和受理凭据同事务提交。
- `snapshot` 保存本会话的标题、计划、消息、Run、命令和 `agent` 凭据。
- 普通检查点和返回快照中的项目数据仅包含五类文稿文件，文件范围见[项目目录](STRUCTURE_SPEC.md#项目目录)。
- 普通检查点另存原始输入正文、引用、初始模式、模型名和范围。
- 会话版本、检查点表、全局资产与设置、项目管理名称、项目外观（主题 ID、四项装饰位置及 Logo）及 Run 内存配置均排除在快照之外。
- 首次回退前写入返回快照，连续回退保留原行，返回成功后只删除返回行。
- 恢复时还原本会话快照内的记录，移除之后新增的对应记录，保留消息身份、`seq`、输入状态和上下文位置。
- 快照中的 `pending / running / cancelling` 命令恢复为 `interrupted`，保留原身份和业务输入。
- 全局主题正文沿用当前资产版本，编辑正文保留引用项目的检查点。
- 历史切换在项目写锁下协调数据库与文稿文件，校验失败时保留切换前的状态。
- 历史切换事务按切换前的版本更新目标会话，必须恰好命中一行。
- 该事务将目标会话当前的 `revision` 加一，同时提交快照恢复、返回行变更和受影响的其他会话检查点及版本变更。
- 恢复遗留历史切换时，目标会话版本等于日志的 `base_revision` 表示未提交，等于 `base_revision + 1` 表示已提交。
- 版本无法读取、会话缺失或版本为其他值时，提交结果未知，保留日志。
- 回退位置实际改名时，在同一事务比对规范化标题与预期版本、更新标题、删除返回行及脱离当前历史的检查点，并递增会话版本。
- 同名改名成功时只递增会话的 `updated_at`，保留 `revision`、检查点和返回快照。改名失败时保留原状态。
- 回退位置删除计划时，清空计划、清理返回行及脱离当前历史的检查点、递增会话版本同事务提交。
- 回退位置新开 Run 或受理 `rename / handoff / compact / commit / suggest`（含修订 `handoff`）时，同事务删除返回行及脱离当前历史的检查点。发起或修订 `polish` 保留返回行。
- 会话实际改变共享文稿文件时，清除同项目其他会话的全部检查点和返回快照，历史恢复适用同一规则。
- 人工保存共享文稿时，文件实际变化的写事务只记录 `manual` 凭据并更新项目的 `updated_at`。项目内各会话保留原有检查点、返回快照及 `revision`。
- 检查点清理与相关业务变更及文件提交凭据协调提交，只推进受影响会话的版本。
- 同值、失败或回滚的写入保留检查点，之后写回相同内容也不重建已失效的检查点。

## receipts

- 凭据主键为 `(type, owner_id, request_id)`，只记录成功受理或已提交的文件修改。
- `request_hash` 使用规范化原始请求的摘要，重复校验沿用原始参数。

| `type` | `owner_id` | `request_id` | `result` |
|---|---|---|---|
| `create_run` | 会话 ID | 首条消息的提交标识 | JSON 对象 `{run_id}` |
| `create_command` | 会话 ID | 命令请求 ID | JSON 对象 `{id}` |
| `agent` | Run ID | `tool_call_id` | SQL `NULL` |
| `manual` | 项目 ID | 后端操作 ID | SQL `NULL` |

- 文件应用后，`agent` 凭据与配对的 `tool_result`（含实际差异）、检查点失效及受影响会话版本在同一事务提交。
- 人工文件应用后，`manual` 凭据与对应业务变更在同一事务提交。
- `create_run` 和 `create_command` 保留至所属会话删除，回退或返回时均保留原凭据。
- `agent` 凭据随所属 Run 清理，仅随本会话历史恢复。
- `manual` 凭据排除在历史快照之外，文件事务日志移除后可清理，项目删除时一并清理。
- 清理文件提交凭据前，先完成对应遗留日志的恢复。

## assets

- `content` 保存四类资产的正文原文。
- 资产名称按原名称精确比较，在同一 `type` 内唯一，不同类型允许同名。
- 每次成功更新资产均递增 `updated_at`，与正文、元数据和标签关联的写入在同一事务提交；提交 `tag_ids` 数组时替换该资产的全部标签关联。
- 停用或删除主题时，在同一写事务内检查 `projects.theme`。存在引用时拒绝操作并返回 `ASSET_IN_USE`。

## tags

- `id` 和 `type` 创建后固定。
- 标签名称按原名称精确比较，在同一 `type` 内唯一。

## asset_tags

- `(asset_id, tag_id)` 为复合主键，同一标签在同一资产上最多关联一次。
- Service 按资产类型核对提交的标签，确认标签均存在、同类且无重复后写入关联。
- 删除资产或标签时，级联删除对应的关联行。

## settings_fileopen

- 以 `id=1` 保存全局文件打开设置。
- `custom_apps` 保存规范化的应用绝对路径，Windows 使用 `.exe`，其他平台使用 `.app`。重复路径只保留首次出现的一项。
- 指定应用时，保存其在 `custom_apps` 中登记的路径。合法路径未登记时，`default_open` 保存为 `system`，`json_open`、`html_open` 保存为 `inherit`。

## settings_shortcut

- 以 `id=1` 保存全局快捷键设置。
- 操作快捷键按提交值保存，格式为单个修饰键加 `+` 加普通键的 `KeyboardEvent.code`。修饰键可取 `Primary`（Command 或 Ctrl）、`Option` 或 `Alt`。
- `select_suggestion` 只存共用修饰键前缀，默认值为 `Option`。数字 1、2、3 由建议序号决定。检查组合键冲突时，`Option` 与 `Alt` 视为同一修饰键。
- 交换多项快捷键或重置默认值时，在同一事务内更新原记录。

## settings_models

- `id` 为模型配置主键，由服务端生成唯一字符串。每个配置的 ID 固定，`name` 可修改且全局唯一，至少保留一条模型配置。
- `provider` 保存用户选择的模型品牌，取值为deepseek、kimi、mimo、gpt、claude、gemini、qwen、minimax、glm、grok。
- `key` 必须为非空白字符串。

## settings_route

- 以 `id=1` 保存全局路由。一次只更新指定的路由字段，目标模型的存在性校验与路由写入在同一事务完成。
- 各路由字段引用 `settings_models.id`。事务提交时校验外键。模型改名后路由仍指向原配置；删除单个模型时，在同一事务确认它未被使用且至少还剩一个模型。

## settings_behavior

- 以 `id=1` 保存全局行为开关。
- `show_tool_failures` 的取值不影响失败 `tool_result` 的持久化。
