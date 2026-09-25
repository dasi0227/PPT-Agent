# 会话日志、命令与数据库重构总结

日期：2026-09-25。

## 1. 状态与范围

本文总结本轮关于数据库、命令、Timeline 和模型上下文的讨论，并结合当前代码、deepseek-harness 与 zcode 的本地源码给出目标方案。

- **代码已切换，待手动验收**：[数据库结构精简](2026-09-25-database-structure-simplification-design.md)中的删除页面身份表、编号 SQL 迁移及迁移记录表已移除。
- **当前实现**：生产入口采用 12 张表，最新 checkpoint 并入 runs；会话日志、六类命令和前端历史协议已统一。实现边界、未验证风险与手动验收步骤见第 17 节。
- 已实施的资源方案继续遵循 [统一资源登记](2026-09-25-resource-registry-design.md)：`resources + tags + resource_tags` 管元信息，文件管正文。
- 实施期间独立开发，只做静态复核；不执行测试、构建和交互验证，不修改 `DESIGN.md`，不提交或推送。
- 本文以本地第三方源码快照为参考，不声称代表上游所有版本或完整产品实现；第三方项目的兼容层和迁移政策不引入本项目。

**已确认目标：一份会话日志保存完整历史，Timeline 和模型上下文从它生成；SQLite 保存业务当前状态、执行控制和必要的恢复凭据。**

## 2. 重构前的问题

本节保留重构动因，描述切换前的代码；现行入口和文件见第 17 节。重构前同一会话的数据分散在多个位置：

| 当前位置 | 当前职责 | 存在的问题 |
| --- | --- | --- |
| `threads/<id>/user.jsonl` | 部分用户可见历史 | 仅写白名单事件；写失败可忽略，读取又从数据库补齐 |
| `threads/<id>/model.jsonl` | 模型历史消息 | 独立替换、独立压缩，逐渐成为另一份必须保存的历史来源 |
| `run_events` | Run 事件与断线补发 | 与会话 JSONL 重复，序号按 Run 而非整个会话分配 |
| 命令、简报、压缩、命名、Git 专用表 | 请求、状态、正文与结果 | 同一结果出现在活动、业务记录和终态事件中 |
| `ThreadService.History` | 拼装用户时间线 | 读取文件后再合并多个业务表，维护去重和补齐分支 |

问题的重点是历史来源分散。仅把七张表合为一张带大 JSON 的表，仍然无法回答“恢复整个会话应该读取哪里”。

代码依据：

- [Run 事件写入](../../backend/internal/run/bus.go)：先写数据库，再尝试写 JSONL。
- 原 `run/history_writer.go`：逐会话追加，但没有持久化屏障；本轮已删除。
- [历史拼装](../../backend/internal/service/thread.go)：合并日志、Run 事件、补充输入、命令、简报、压缩、Git 提交。
- [模型历史存储](../../backend/internal/contextengine/transcript.go)：替换整个模型消息文件。
- [Runtime](../../backend/internal/workflow/runtime.go)：读取和反复持久化模型历史；压缩后覆盖有效消息。

## 3. 两个参考项目带来的结论

### 3.1 deepseek-harness：日志保存事实，模型消息由日志生成

本地实现中的 `Session` 是只追加的事件序列，`deriveMessages()` 根据事件的消息投影和替换范围生成有效模型历史。压缩遮蔽被替代的历史，原事件仍然存在。

JSONL 后端负责串行写入、序号连续性、写者租约、持久化及尾部恢复。当前默认使用 Zstandard 压缩，也支持纯文本 JSONL。搜索 SQLite 是独立的派生索引。

可借鉴：

1. 原始记录和模型有效上下文有统一来源。
2. 压缩是追加“如何解释历史”的记录。
3. 持久化成功有明确边界；不能把调用文件写入函数等同于已可靠保存。
4. 查询索引可以重建，不承担唯一正文存储职责。

证据：

- [会话模型说明](../thirdparty/deepseek-harness/packages/core/session/README.zh.md)
- [Session.deriveMessages 实现](../thirdparty/deepseek-harness/packages/core/session/src/index.ts)
- [JSONL 后端](../thirdparty/deepseek-harness/packages/session/session-persistence-jsonl/README.zh.md)
- [连续追加与写者租约](../thirdparty/deepseek-harness/packages/session/session-persistence-jsonl/src/storage.ts)
- [派生 SQLite 搜索](../thirdparty/deepseek-harness/packages/session-query/session-query-sqlite/README.zh.md)

本项目先采用普通 UTF-8 JSONL，不引入参考项目的插件化框架、历史格式迁移和压缩世代体系。

### 3.2 zcode：结构化会话记录，分别生成展示与模型视图

本地 `apps/zcode-cli` 主存储实际使用 `SqliteSessionStore`。消息和内容片段写入 `message / part`，另有 `session_entry` 保存会话条目。用户可见 transcript 由消息存储投影，模型恢复由历史 hydrator 处理；内部输入可以参与模型上下文而不显示成用户发言。

它还有受保留策略约束的内存事件流。不能因为源码中出现 JSONL 调试或导入文件，就推断该实现将全部会话历史存入 JSONL。

可借鉴：

1. 内容记录、用户展示、模型消息组装分开建模。
2. 用户输入与运行时内部消息必须能区分。
3. 实时通知与持久会话记录可以有不同粒度。
4. 统一存储接口可以避免业务代码直接依赖文件或 SQL。

证据：

- [SQLite 装配入口](../thirdparty/zcode/apps/zcode-cli/packages/bootstrap/src/app/session-store.ts)
- [消息及片段存储](../thirdparty/zcode/apps/zcode-cli/packages/adapters/src/storage/session-store/repositories/messages.ts)
- [会话条目存储](../thirdparty/zcode/apps/zcode-cli/packages/adapters/src/storage/session-store/repositories/session-entries.ts)
- [用户历史投影](../thirdparty/zcode/apps/zcode-cli/packages/bootstrap/src/session-transcript.ts)
- [模型历史恢复](../thirdparty/zcode/apps/zcode-cli/packages/core/src/agent/session-history-hydrator.ts)
- [内存事件保留策略](../thirdparty/zcode/apps/zcode-cli/packages/contracts/src/events/in-memory-session-event-store.ts)

### 3.3 本项目的选择

采用 deepseek-harness 的日志组织方向，并借鉴 zcode 的展示与模型视图分离方式。PPT-Agent 保留现有 SQLite 业务存储和工具提交事务，避免将整个产品改造成通用事件溯源框架。

## 4. 最终存储边界

| 数据 | 唯一职责明确的存储位置 | 消费方式 |
| --- | --- | --- |
| 完整会话历史、消息、工具事实、问题与回答、命令结果及修订 | `threads/<thread_id>/thread.jsonl` 及其引用的不可变载荷文件 | Timeline、命名取材、简报修订、模型上下文生成 |
| 当前项目、页面、会话、资源元信息 | SQLite 对应业务表 | 管理界面、业务校验、列表查询 |
| Run 与命令当前执行状态、权限、待注入输入、写入去重 | SQLite 运行控制表 | 并发约束、停止、恢复、阻止重复副作用 |
| 当前有效模型消息列表 | 从会话日志派生的内存结果 | 组装模型请求；必要时增量缓存 |
| PPT 正文及资源正文 | 现有 `artifacts/` 与 `assets/` | 编辑、预览、加载、导出 |
| 文件修改恢复与项目回退 | 现有 mutation journal 和项目 checkpoints | 文件／数据库恢复、整项目回退 |
| 待可靠写入 JSONL 的事件 | 有界、可排空的 `thread_event_outbox` | 交付成功后删除，不能用作长期历史查询 |

目标目录：

```text
<project-root>/
  artifacts/                       # 现有 PPT 正文目录
  threads/
    <thread_id>/
      thread.jsonl                 # 完整会话记录
      payloads/                    # 确有需要的大工具输出等不可变载荷
  checkpoints/                     # 现有项目历史
```

SQLite 和全局资源仍使用现有工作根目录布局。`thread.jsonl` 替代 `user.jsonl` 和 `model.jsonl`；路径由会话 ID 推导，删除 `threads.history_path` 的路径副本。新工作目录验收，不转换旧日志。

“完整历史”包含内部类型化记录，不等于将文件原文直接交给前端或模型。附件不内嵌二进制，日志保存已校验的引用；API 只输出适合其消费者的视图。

## 5. 会话日志协议

### 5.1 统一身份和顺序

日志有一条格式及会话身份头记录；后续事件使用统一信封：

```json
{
  "id": "evt_xxx",
  "seq": 42,
  "ts": 1790000000000,
  "type": "command.completed",
  "run_id": "run_xxx",
  "command_id": "cmd_xxx",
  "attempt_id": "attempt_xxx",
  "payload": {}
}
```

- `seq` 在整个会话内严格递增，不再拼接各 Run、命令的局部序号。
- 不适用的关联 ID 省略；`id` 在重试投递时保持不变。
- `command_id` 标识同一张操作卡片；`attempt_id` 标识一次实际执行；工具使用稳定 `call_id`。
- `ts` 统一为毫秒，用于展示，不承担正确排序。
- `payload` 按事件类型校验，展示及模型使用权限由服务端类型规则决定。
- 公共结果不复制原始凭证、访问令牌或供应商请求头；必须保留恢复所需的角色、工具关联、来源与内容结构。

### 5.2 应进入日志的内容

| 事件族 | 需要保存的事实 |
| --- | --- |
| 会话与输入 | 被正式接受的用户输入、正文与引用、输入身份、来源 |
| Run | 开始、等待、暂停、恢复、完成、失败、取消及必要关联 |
| 模型消息 | 有效助手消息、必要的中断部分内容、模型消息关联 |
| 工具 | 调用、参数、成功／失败结果、对应工具调用身份 |
| 交互 | 计划及其更新、提问、用户回答、权限请求、审批与拒绝 |
| 命令 | 开始、重要阶段、结果、失败、取消、中断、基于哪个结果修订 |
| 上下文 | 压缩结果、替换范围、必要的运行时消息及其生效规则 |
| 运行诊断 | 语义检查结论、模型切换、必要用量等类型化记录 |

持久化完整语义内容，不要求持久化每个 token delta、心跳或动画状态。发生取消和中断时，要明确保存已有的部分结果与终态，不能只留下永远运行中的卡片。长输出可以分块持久化，完成事件引用块，避免重复嵌入整份正文。

原始事件允许同时支撑 Timeline 与模型投影；不要分别保存“用户版工具结果”和“模型版工具结果”全文。需要裁剪或脱敏时，由受控投影处理；必要的专用内容片段有明确用途。

### 5.3 读取与实时事件

- `TimelineProjector`：从日志还原可见消息、问题、工具与命令卡片。命令按 `command_id` 原地显示最近尝试，失败修订仍可保留最近成功正文。
- `ModelContextProjector`：从模型相关事件构造有效 `Message[]`，补齐角色、工具调用与结果配对，应用压缩范围。
- `CommandHistoryReader`：按命令身份查找输入、反馈和成功版本；简报不再查询专用版本表。
- `NamingContextReader`：读取已接受的首条及近期用户输入，保留原有首次／每五次触发规则；输入计数和命名开关仍由服务控制。
- 历史与 SSE 使用同一会话事件顺序。所有持久事件在可靠写入后才能广播；临时 delta 明确不作为断线恢复游标。
- 首次可全量回放，随后使用内存索引增量更新；按需增加可删除的文件偏移索引，暂不增加完整全文搜索系统。

## 6. 取消独立的模型历史文件

重构前 `model.jsonl` 保存独立消息副本。当前已移除该文件，模型有效历史改为：

```text
会话事实 + 已生效的压缩记录
    → ModelContextProjector
    → 有效历史消息
    → 当前系统提示词、工具定义、项目上下文组装
    → 模型请求
```

压缩记录至少保存：摘要标题和正文、精确被替换的消息／事件范围、保留边界、生成时的历史水位、Token 变化。它只改变模型视图，不擦除原始会话，也不删除用户历史。

同一次压缩只持久化一条包含结果正文与替换描述的结果事件，Timeline 和模型投影共同消费。命令终态需要单独表达时引用这条事件，不能再把摘要全文复制到另一条完成事件。

约束：

- 压缩只能替换其实际读取的稳定历史范围，生成期间新增的输入不能被覆盖。
- 调用与结果必须作为完整单元处理；未解决的提问、授权和执行中工具不能被压缩丢失。
- 连续压缩要能替换之前摘要及指定近期历史，离线回放和运行中增量应用必须得出同一结果。
- 简报、润色等命令结果默认只用于 Timeline；用户复制、应用或发送后才按正常输入进入主模型。手动与自动压缩则由其类型规则更新有效上下文。
- 模型消息缓存不是恢复必需文件，删除缓存后必须可以仅凭日志及必要的载荷引用重建。
- 运行中的精确续跑仍有 Runtime checkpoint；完整供应商调用追踪如有需要，应作为诊断资料，不再复制一份长期模型历史。

## 7. 六类命令统一执行，正文归日志

统一 `rename / polish / kickoff / handoff / compact / commit` 的接受、状态、阶段、取消、重试与恢复协议。

### 7.1 两种身份

一次独立操作有稳定 `command_id`。每次执行有新 `attempt_id`；修订附 `base_attempt_id`，引用本命令已有的成功结果。

```text
cmd_1
  attempt_1：首次生成成功 → 正文 A
  attempt_2：带反馈生成成功 → 正文 B
  attempt_3：修订失败     → 保留正文 B，显示本次失败
```

输入、反馈、结果和版本均在事件中。一份结果不再同时保存到活动、简报、压缩和完成事件等多个长期位置。

### 7.2 一张精简的执行控制表

新增 `command_executions`，每行对应一次尝试，建议保存：

- `id`（即 attempt ID）、`command_id`、`attempt_no`、项目／会话／可选 Run 归属。
- `kind`、触发来源、执行状态、当前阶段、进程所有者及防迟到写入的执行代次。
- 开始事件和终态事件的序号引用、创建与更新时间。

不保存请求正文、完整结果正文、整份简报版本和独立展示副本。错误的完整内容在终态事件中；确有调度用途的错误码才保留为执行字段。

网络重复请求通过现有 `idempotency_records` 处理，保存请求摘要及 `{command_id, attempt_id}` 回执。主动重试创建新尝试和新请求键。避免命名再维护一张专用幂等表。

该表承担并发、取消、重启识别和项目互斥；Timeline 历史一律从日志读取。历史执行行可按项目生命周期保留，但不能成为结果正文的第二来源。

### 7.3 特殊规则

- 同一命令最多一个活动尝试；旧尝试不能覆盖新尝试。
- 刷新、离开页面和网络断开不取消命令；仅明确停止指定 attempt 才取消。服务重启把普通命令标记为 interrupted，等待用户重试；主 Run 继续使用显式恢复。
- 后台自动命名使用同一执行协议，但沿用不新增可见命令卡片的产品行为。
- 手动命名的当前名称和自动命名开关仍在 `threads` 中原子更新；开关设置可直接更新会话状态并记录相应事实。
- 自动压缩关联主 Run，不能被当作独立任务取消；其结果仍使用统一压缩事件。
- Git 无变更是成功执行的特定结果；取消、失败与不确定中断分别处理。
- 保留现有 Run／提交／压缩的项目写入互斥，不能只限制同一 `command_id` 内并发。

不采用前面讨论过的 `commands + command_attempts + thread_inputs` 三张完整内容表。命令身份和尝试历史由日志承担；保留一张有实际控制用途的精简执行表。

## 8. SQLite 目标清单

按本轮对所有表、SQL 条件、JSON 载荷及前端消费的静态复审，将初版 15 张候选表收敛为 **12 张目标表：保留 10 张现有表，删除／替代 14 张，新增 2 张**。用户已确认此方案；生产启动尚未切换，当前旧表仍在。数量仅用于核对去留，不作为优化目标。

本轮新增结论：删除 `run_contexts`；取消 `context_index_snapshots` 的持久缓存；保留最新 Runtime checkpoint 数据，但并入 `runs`，取消独立 `run_checkpoints` 表及其无限追加历史。后者是存储合并，不能理解为取消续跑恢复。

### 8.1 保留 10 张现有表

| 表名 | 保留理由与调整 |
| --- | --- |
| `projects` | 项目身份、名称、所选主题及时间；删除 work_dir、无有效状态流转的 status 和逐项目 layout_version |
| `slides` | 页面身份、归属和生成时的参考快照；删除没有导出读写用途的 last_export_at，不引入删除身份历史 |
| `threads` | 会话当前名称、归属和命名控制；加入日志投递水位，删除 history_path 和没有归档实现的 status |
| `runs` | 主任务状态机、范围、所有权、停止与恢复；保存最新 checkpoint 及其更新版本，初始输入正文改用日志引用，删除无查询用途的范围字段副本 |
| `steering_inbox` | 运行中输入的待注入／已注入／拒绝控制；正文改引用已可靠记录的输入事件 |
| `idempotency_records` | 创建／取消任务、命令接受及产物提交的去重和恢复凭据；命令正文用事件引用，产物 receipt 按实际恢复需求保留 |
| `resources` | 四类资源元信息 |
| `tags` | 标签定义、名称、排序和系统标记统一由数据库提供；本次不增加标签编辑界面 |
| `resource_tags` | 资源与标签关系 |
| `shortcut_settings` | 用户快捷键设置 |

`slides.generation_inputs_json` 是页面生成时的历史参考快照，继续遵循 [HTML 生成参考快照设计](2026-09-25-html-generation-reference-snapshots-design.md)。当前文件只能说明现在的内容，不能替代生成时的参考状态。`projects.title` 是用户可改的项目名称，也不能直接用 manifest 的演示主题标题替代。

**最新 checkpoint 合并规则：** 当前生产读取使用 LatestCheckpoint，以及接受 steering 时直接读取最新一行；ListCheckpoints 未发现生产调用。目标每个 Run 原子替换一份 checkpoint，使用 checkpoint_revision、Run 所有权及范围版本阻止迟到覆盖；计划批准、范围批准和 steering 接受仍必须与 checkpoint 在同一事务更新。普通 Run 查询显式选择轻量列，恢复时才加载 checkpoint_json。项目回退由项目 checkpoint 捕获当时的这份状态，运行边界诊断进入日志／trace。不能仅因 Run 结束就清空 checkpoint：当前 GetRun 还读取其中的 ModelRoute 展示实际模型，终态信息有可靠替代来源后才能回收。

核对依据：[ResumeRun 与实际模型读取](../../backend/internal/service/run.go)、[索引复用与重建](../../backend/internal/workflow/runtime.go)、[实际检索消费](../../backend/internal/workflow/context_briefing.go)、[上下文清单及 steering 存储](../../backend/internal/store/sqlite/run_store.go)、[checkpoint 事务与读取](../../backend/internal/store/sqlite/runtime_capabilities_store.go)。

### 8.2 删除或替代 14 张

| 旧表 | 当前消费与删除条件／去向 |
| --- | --- |
| `deleted_slides` | 当前用于历史页面引用判断；已确认取消该身份历史，改按当前页面成员校验 |
| `schema_migrations` | 当前启动迁移执行器读取；已确认改成单份最新 schema 初始化和统一格式检查 |
| `run_contexts` | 开始／计划批准时写入，上下文清单 getter 仅有测试调用；恢复实际重新 Assemble，取消表，必要诊断交给 trace／日志 |
| `context_index_snapshots` | 恢复时可能复用，但 RunID／PackHash 不匹配就重建；当前使用本地 HashEmbeddingProvider，推荐总是在内存构建索引 |
| `run_checkpoints` | 续跑、steering 和实际模型查询读取最新一份；合并到 runs，保留恢复能力，取消历史行及专用表 |
| `run_events` | SSE 补发、历史拼装、命名和上下文窗口读取；统一日志具备相同查询与恢复能力后替代 |
| `command_activities` | 刷新恢复命令卡片、失败重试和并发控制；展示投影改读日志，控制进入 command_executions |
| `briefing_versions` | 历史展示和下一次简报修订读取；成功事件保存正文、标题、反馈和修订关系 |
| `context_compactions` | 压缩卡片和自动命名摘要读取；统一压缩事件承担历史、摘要与模型替换范围 |
| `thread_naming_inputs` | 命名读取首条及近期输入，并据此计数；改读已接受的用户输入事件，计数控制另行保留 |
| `thread_naming_operations` | 专用命名入口的幂等、请求状态和回执；通用执行控制与 idempotency_records 替代 |
| `git_commit_operations` | 接受／查询／重启恢复、历史展示和项目互斥；通用控制替代，提交结果进入日志，Git 副作用凭据仍须保存 |
| `git_commit_events` | Git SSE 进度补发、重启恢复事件序号；改成会话日志中的通用命令事件 |
| `semantic_reviews` | 审查后保存，无专用表生产读取消费者；必要审查诊断进入内部事件／trace |

`run_contexts`、`semantic_reviews` 属于没有业务读取的独立存储；事件、命令和简报表均有消费者，属于替换存储后可删除，不能称为当前死表。项目快照整表复制不是业务读取证据。删除 semantic_reviews 不取消语义审查算法及完成门禁。

### 8.3 新增 2 张

| 表名 | 用途 |
| --- | --- |
| `command_executions` | 统一命令的精简执行控制，见第 7 节 |
| `thread_event_outbox` | 数据库事务到会话日志的可靠投递队列，见第 9 节 |

已确认清单：`projects、slides、threads、runs、steering_inbox、idempotency_records、resources、tags、resource_tags、shortcut_settings、command_executions、thread_event_outbox`。最新 checkpoint 已确定并入 runs，不采用独立 checkpoint 表。

## 9. 文件与数据库的一致性

### 9.1 为什么还需要一个 outbox

PPT-Agent 已有依赖 SQLite 事务的执行控制。改名、输入接受、运行终态等操作可能同时改变数据库和会话历史。

直接“更新数据库，再 append 文件”有中断窗口；单纯改成反向顺序同样有窗口。因此推荐暂存待投递事件的 outbox，并在交付后删除。它承担可靠交付，不保存第二份长期历史。

如果未来选择让所有控制状态也从日志重建，可以重新评估取消 outbox；本轮不把整个 Runtime 状态机一并替换。

### 9.2 统一写入协议

1. 校验身份、作用域、请求去重及执行代次。
2. 在一个短 SQLite 事务内，完成相关状态变更、分配会话 `seq`、写入稳定事件 ID 和 outbox 载荷；纯消息同样经过统一入口。
3. 每个会话只有一个写者，按序将待投递事件追加到日志并完成 `fsync`。大载荷先写临时文件、同步并原子发布，再追加引用；首次发布目录项也要有相应持久化处理。
4. 日志确认后，在数据库事务中推进已投递水位、删除对应 outbox 行。
5. 广播持久事件、确认历史可见；接受的输入或命令开始记录未落日志前，不启动后续模型调用或副作用。

`threads` 的序号分配和已投递水位属于运行协调元信息。待投递行不可跳过、不能无限积累；磁盘写入失败时限制新任务接受，给出可恢复错误。执行状态接口可明确报告已提交但等待历史投递，不能冒充完整交付成功。

运行时仅追加选定的持久事件，不把每个 token delta 都写入 SQLite 和磁盘；批量投递有明确上限，关键接受和终态需要等待持久化屏障。

### 9.3 中断恢复

| 中断位置 | 恢复规则 |
| --- | --- |
| 数据库事务前 | 没有接受记录，同请求可正常重试 |
| 数据库已提交、JSONL 尚未完成 | 按原事件 ID 和 seq 重新投递；复用已有命令／Run 身份，不重新接受一次 |
| JSONL 已同步、outbox 尚未清理 | 校验日志现有事件身份和内容，一致则只清理队列，不重复追加 |
| 文件最后一行写了一半 | 独占写入前定位最后完整记录，截去残缺尾部，再投递原事件 |
| 中段损坏、完整行非法、序号缺口或同 ID 内容冲突 | 显式拒绝恢复并报告损坏位置，不静默跳过 |
| 模型调用或普通命令中断 | 标记并追加中断结果；按用户重试策略重新执行，不自动重复生成 |
| 主 Run 中断 | 按现有 Runtime checkpoint 恢复，并核对事件水位与待处理交互 |

网络重复请求保留同一请求键；“投递失败后再试”只重试交付，不再次调用模型。读方只看到完整已验证记录。

写者采用进程所有权或文件租约；现有 Go 实例内 mutex 不能单独承担两个服务进程同时打开工作目录的保护。

### 9.4 Git 与 PPT 文件写入仍有独立恢复边界

- PPT 写工具沿用 mutation journal、文件前后像和 SQLite receipt，成功修改不能因后续日志广播失败而被再次执行。
- 业务提交完成后，应在对应数据库事务中生成可补发的工具完成事件意图；运行恢复从 receipt 和 outbox 确认结果。
- Git 仓库写入无法参加 SQLite 事务。必须保留执行意图及足以核对提交的凭据，例如提交前 HEAD、预期提交对象及稳定操作身份。
- 副作用恢复凭据通过内部准备事件或现有持久操作 journal 在实际修改前保存并同步；仅在内存里记住这些信息不构成恢复方案。
- 若 Git 已完成而终态记录缺失，先核对仓库，再补齐结果。证据不足时报告不确定中断，不能自动再 commit，也不能仅凭“HEAD 变了”推断本次成功。
- Runtime checkpoint 是执行位置记录，目标存于 runs；项目 checkpoints 是整项目回退；两者与聊天日志分别承担不同恢复职责。

## 10. 项目回退、恢复最新与删除

保留“项目和所有对话一起回退”的现有产品行为，不能只恢复内容而留下来自未来的聊天、命令或模型摘要。

1. 获取现有项目独占门禁，阻止新 Run、命令、后台命名及日志追加；按当前规则处理未结束任务。
2. 排空该项目所有会话的 outbox，等待日志及载荷持久化。
3. 以一致水位捕获业务／运行数据库、thread.jsonl 和引用的载荷文件；不捕获待投递行与悬挂的命令尝试。
4. 回退时停止旧写者，使用现有恢复 journal／补偿流程共同恢复数据库和文件；旧未来的投递和迟到响应不得进入恢复后的项目。
5. 恢复日志水位、清除派生缓存并重新生成视图；恢复最新使用同样流程。

日志在正常执行期间仅追加。显式项目回退会恢复选定 checkpoint 中的日志前缀；首次回退前的现场仍由现有“恢复最新”机制保存。不能同时承诺回退删除未来和物理日志永不回退。

SSE 游标使用项目当前 `scene_revision + thread_id + seq`，项目回退／恢复后更新代次；旧游标要求刷新快照，避免新未来重用 seq 导致串接。命令和后台命名回写同时校验项目代次。

项目删除及“丢弃旧未来”的载荷清理必须考虑当前日志和保留 checkpoints 的引用。参考资料或工具输出缺失时明确显示缺失，不能伪造恢复成功。

## 11. 已确认的两项精简如何纳入

### 11.1 删除页面身份表

按 [数据库结构精简](2026-09-25-database-structure-simplification-design.md)执行：

- 当前页面引用只认当前 outline 和页面成员。
- 旧引用显示不可用；发送时要求移除或重选，后端拒绝不存在于项目的引用。
- 删除 deleted_slides 的读写、清理及项目快照关联。
- 历史卡片可显示当时保存的页面摘要；历史存在不赋予当前页面操作权限。
- 页面恢复仍由项目 checkpoint 承担。

### 11.2 取消增量数据库迁移

- 用一份最新 `backend/internal/store/sqlite/schema.sql` 定义完整表、索引和约束。
- 空数据库直接初始化；登记资源的 seed 流程单独调用，保持可重复执行且不覆盖已有修改。
- 删除编号 migrations、迁移执行器和 schema_migrations。
- 用 SQLite 自带 `application_id / user_version` 或等价的最小结构标识识别格式；标识只负责判断匹配，不承载迁移执行记录。
- 已有数据库匹配则正常打开；不匹配时提示用户重建，不自动删除目录或数据。
- 日志格式标识同样只用于校验；本次不写旧日志解码转换、双读或降级分支。
- 新格式前后端、初始化脚本、项目 checkpoint 清单一起切换，用新工作目录验收。

## 12. 建议的代码边界与 API

沿用 Go 服务和前端现有技术栈，不引入参考项目的完整插件系统。

| 模块 | 职责 |
| --- | --- |
| `ThreadJournal` | 追加、读取、格式校验、水位、序号、单写者与损坏恢复 |
| `ThreadEventDelivery` | SQLite outbox 到日志的串行可靠投递与实时广播 |
| `TimelineProjector` | 原始会话事实到用户时间线 |
| `ModelContextProjector` | 日志及压缩记录到模型有效历史 |
| `CommandService` | 六类命令共用的接受、阶段、取消、重试、执行代次与终态 |
| 各命令处理器 | 命名、润色、简报、压缩、Git 的业务算法和结果校验 |
| 现有 Runtime／mutation／projecthistory | 主 Agent 循环、写工具提交、整项目恢复 |

建议统一命令启动／查询／取消接口，所有入口返回稳定 command 与 attempt 身份；必要的内部路由由 kind 分派，不再为命名、简报和 Git 分别制定历史恢复协议。

前端 history 与实时订阅共享同一份时间线模型及 reducer；按项目代次和 seq 去重。一次加载历史不再分别查询命令业务表；重试传 base_attempt_id 和反馈，由后端从日志读取基线，不依赖前端携带完整旧正文。

回放只重建数据与视图，不能重新调用模型、执行工具、写 Git 或再次提交 PPT。所有有副作用的动作都必须由执行控制层显式发起。

## 13. 实施顺序

以下是同一次结构切换的开发步骤，不是线上兼容阶段；全部就绪后使用新工作目录启用新协议。

1. **冻结职责及事件协议**：确定持久事件、模型投影规则、命令身份、压缩范围、游标，并根据实际消费者收敛候选表清单。
2. **建立可靠日志基础**：实现 ThreadJournal、outbox、损坏处理、交付水位和跨进程写者保护。
3. **统一输入和主任务事件**：用户输入、steering、工具、交互、Run 终态进入会话日志；移除 run_events 的历史和补发职责。
4. **替换模型历史**：实现模型投影和压缩重放，切换 Runtime、简报上下文及 Token 窗口读取；移除 model.jsonl。
5. **统一六类命令**：接入 CommandService、精简执行表与日志修订；前端统一恢复和重试。
6. **接通项目恢复**：更新快照清单、日志和载荷捕获、代次隔离、恢复最新、删除及外部副作用对账。
7. **清理存储结构**：按第 8、16 节删除／替代 14 张旧表并清理冗余字段，将最新 Runtime checkpoint 并入 runs，切换单份 schema 和初始化脚本；执行删除页面身份表的决定。
8. **静态复核和人工验收交接**：更新行为测试和最新设计文档，检查不再有旧字段／路径消费者；测试、构建、浏览器操作交由用户手动执行。

只有当统一日志能完整恢复既有可见行为与模型续接语义时，才删除旧来源；开发实现不能通过在运行中保留旧协议兜底来掩盖缺失。

## 14. 聚焦验收

后续实施补充或调整以下行为测试，本次未执行：

- 每一类可见消息、工具、问题、回答、授权和六类命令，刷新前后内容及顺序一致。
- 命令首次生成、修订、失败修订、取消、显式重试、网络重复请求分别符合身份和版本规则。
- 仅用日志与引用载荷重建模型历史；工具调用／结果配对正确，内部输入不伪装成用户发言。
- 多次压缩后完整 Timeline 仍可读，有效模型历史只包含正确摘要和保留消息；压缩期间新增输入不丢失。
- 输入去重、首条自动命名、后续五次触发、关闭／重新开启与迟到结果处理保持原有行为。
- 文件写入失败、fsync 失败、数据库失败以及每个交付中断点，不丢失已接受输入、不重复生成／执行、不误报成功。
- JSONL 末尾残缺可恢复，中段损坏、序号冲突和引用缺失明确报错。
- 服务重启、项目回退、恢复最新、丢弃旧未来和多会话后台活动不混入旧代次事件。
- Git 提交及 PPT 写工具已完成但通知中断时，从凭据恢复结果，保持已提交内容且不重复副作用。
- 数据库初始结构完整；不匹配的工作目录明确拒绝；资源初始化可重复执行且不补回已删除预置。

## 15. 最终结论

最终应形成四条清楚的职责线：

1. **会话发生过什么**：thread.jsonl。
2. **下一次模型应该看到什么**：从日志生成的有效上下文。
3. **产品现在是什么状态、哪些操作还在执行**：SQLite。
4. **PPT／资源正文是什么、如何恢复文件修改**：产物文件与现有恢复机制。

原先为命名、简报、压缩和提交分别建立的完整历史存储可以合并退出；执行控制、去重和跨文件恢复保留明确责任。数据库数量随职责清理自然减少，完整会话的恢复入口最终收敛到一份可靠日志。

## 16. 全表与字段复审补充

本节是实施前的字段审查记录，保留当时的问题与决策依据；其中“当前”“未发现”“待核查”描述审查时状态。实际字段以 `schema.sql` 为准，恢复补齐结果见第 17 节。

### 16.1 审查口径

本轮依据当前 schema 演进结果、PO 映射、服务、Runtime、前端、项目快照及最新设计静态复核，未连接用户数据库，未执行迁移、测试、构建或交互验证。表级完整清单见第 8 节；本节覆盖保留表的全部当前列，并补充被替代表的载荷去向和重要嵌套字段。

- 没有生产读取，且没有仍有效的业务职责：删除存储及专属写入路径。
- 路径、严格确定的计算结果和可重建索引：由权威来源生成。
- 有消费者但重复保存历史：先接通统一日志投影，再在本次结构切换中移除旧来源。
- 用于唯一约束、条件更新、外键、事务互斥的字段：属于真实使用，不能只凭没有 Go／TS 属性读取判死。
- 用户的历史选择、生成时的参考资料、执行时模型配置不能从当前文件或当前设置准确重建。
- 仅被复制进项目快照、PO 来回映射、接口机械透传、测试读取，不足以证明业务必要性。已有设计明确规划的功能则另列待定，避免把未接通误称为无意义。

### 16.2 现有字段中，明确建议删除或改为计算的部分

| 位置 | 结论与证据 |
| --- | --- |
| `projects.work_dir` | 删除持久列，由受校验项目 ID 和工作根目录生成 `projects/<id>/artifacts`。当前 CreateProject 已固定这样构造；领域对象／响应如需路径可以动态填充。项目恢复同时使用当前工作根，避免恢复旧绝对路径 |
| `projects.status` | 删除。创建固定为 draft；SetProjectStatus 没有生产调用，前端未发现业务分支消费项目 status。需要运行状态时读取 Run，内容状态依据文件判断，不能把当前列视为已有可靠聚合状态 |
| `projects.layout_version` | 与迁移执行器一起删除。固定为 6，只有启动格式检查读取；改为工作目录／数据库统一格式标识，不在每个项目重复保存同一版本 |
| `slides.last_export_at` | 删除。仅 PO 映射及元数据 upsert 保留，导出链路不写也不消费；现行导出设计明确不写该字段 |
| `threads.history_path` | 删除持久列，现有路径已完全由 thread ID 生成；新格式统一推导 thread.jsonl 路径 |
| `threads.status` | 删除。只初始化 active 并透传；没有归档写入及列表筛选消费者。以后真正提供归档功能时再引入相应状态 |
| `runs.scope_slide_ids_json` | 删除。与 run_command_json.scope.slide_ids 重复；生产读取从 RunCommand 反序列化，无该列查询用途 |
| `runs.scope_source_json` | 同上，来源信息只保存一份；删除列不删除 scope.source 的业务含义 |
| `runs.scope_include_run_created_slides` | 删除重复列；当前范围校验还强制该值等于 source.kind 是否为 all_pages，嵌套布尔值也可改成派生方法 |
| `idempotency_records.result_json` 中 scope 为 `artifact_commit` 的完整工具结果副本 | 当前产物恢复只读取 request_hash 与 status；同一事务还把工具结果写到 tool_call 凭据。建议取消 artifact_commit 行的正文副本，保留提交回执及 tool_call 的必要重放载荷，不是删除整列 |
| `idempotency_records.created_at / updated_at` | 当前生产路径只写入／映射，无过期回收或按时间读取；时间索引也未发现实际查询。列为可删除辅助字段，连同无用途时间索引退出；若后续增加超时接管／清理，要先定义真实策略再保存对应时间 |

依据：[项目创建](../../backend/internal/service/project.go)、[PO 映射](../../backend/internal/store/sqlite/po.go)、[Run 存储](../../backend/internal/store/sqlite/run_store.go)、[元数据提交](../../backend/internal/store/sqlite/slide_store.go)、原迁移格式检查（`migrate.go` 已删除）、[现行导出设计](2026-09-12-presentation-export-design.md)。本节记录实施前的删除建议；当前代码完成情况见第 17 节。

### 16.3 保留表的其余字段与有条件精简

| 表／字段组 | 决定 |
| --- | --- |
| `projects.id / title / theme / created_at / updated_at` | 保留身份、用户命名、当前主题选择和项目时间。主题来自用户设置；项目列表实际按更新时间展示／排序，不能用某一个文件 mtime 替代 |
| `slides.id / project_id / generation_inputs_json` | 保留。页面身份参与元数据事务；generation_inputs_json 被上下文组装读取，用于比较生成当时与现在的参考内容，不可从当前 outline／manifest／design 反推 |
| `threads.id / project_id / title / auto_rename_enabled / created_at / updated_at` | 保留当前会话归属、用户设置和时间信息。title 可能手动修改，不能总用第一条输入推导 |
| `threads.naming_revision / rename_operation_version` | 保留各自职责：前者防前端旧名称覆盖新名称；后者防旧模型请求结果回写。将来只有统一版本能保持两种行为时才合并，不能认为都是计数器就删除一个 |
| `threads.rename_input_count / rename_first_input_seen` | 有实际控制消费，保留为小型派生状态。可以从完整输入、开关及命名触发日志重建，但直接删列需要重写首次触发、每五次输入触发和重置语义；不是简单 count 用户消息 |
| `runs.id / thread_id / project_id` | 保留。project_id 虽可经 thread 查询，但当前用于项目级锁定、SQL 互斥与恢复筛选，有实际事务用途，不为了消除关联冗余增加不必要改动 |
| `runs.status / mode / scope_revision` | 保留 SQL 控制字段。mode 是计划批准条件更新依据，scope_revision 防并发范围覆盖；scope 的其余有效内容只保留一份 |
| `runs.run_command_json` | 当前恢复确实读取，不能直接删。目标把初始指令、附件、选中资源等快照放开始事件／不可变载荷，Run 保存开始事件引用及当前可变范围；mode、范围版本与 checkpoint 原子更新，避免又保存整份初始请求副本 |
| `runs.client_request_id` | 有唯一索引消费，当前不算死字段。统一接受事务能原子写入 Run、create_run 幂等回执和 outbox 后，可取消本列及重复唯一索引；此前不裸删第二道去重保障 |
| `runs.model_profile_name / model_provider / model_name / model_url` | 保留执行时模型快照。resumeProvider 会与当前配置核对，配置名称相同不代表配置未改变；可改成一个明确模型快照对象，但不能简单改读当前设置 |
| `runs.cancel_requested_at / owner_instance_id / pause_reason / paused_at / created_at / updated_at` | 保留。分别用于取消幂等与 steering 门禁、恢复所有权、暂停说明及执行时间；历史日志能派生不等于应取消事务控制状态 |
| `steering_inbox.run_id / thread_id / client_message_id / request_hash / status` | 保留注入队列的归属、会话级去重与投递控制。接受不等于已经送入模型，仅查看 Timeline 不能替代注入确认 |
| `steering_inbox.content / references_json` | 统一日志可靠记录输入及引用后改为事件引用；附件、DOM 选择、引用顺序和范围内容仍保存一次，不是删除输入信息 |
| `steering_inbox.accepted_at / injected_at / rejection_code` | accepted_at 当前用于顺序，可改用会话输入 seq；注入时间和拒绝原因进入事件，执行表保留所需状态／事件引用。旧状态接口切换前仍有消费者，不当死字段直接删 |
| `idempotency_records.scope / owner_id / key / request_hash / status / result_json` | 保留键、请求一致性和可重放回执；正文按 scope 收敛。创建／取消／命令只需小回执，tool_call 仍需准确结果或可靠载荷引用。不能把实际执行凭据替换成“看到类似日志就算成功” |
| `resources.type / id / name / normalized_name / description / disabled / created_at / updated_at` | 沿用已确认资源结构。normalized_name 虽由 name 计算，却用于 snippet 名称唯一约束，属于有用途的派生列；不要无证明地用 SQLite lower 替代服务的名称规范化语义 |
| `tags.id / scope / key / sort_order` | 当前实际用于合法性检查、标签关系和返回排序，保留 |
| `tags.name / normalized_name / is_system / created_at / updated_at` | 用户已确认全部保留，名称和排序接入统一 GET /api/v1/tags 字典；删除前端独立枚举，不新增自定义标签编辑界面 |
| `resource_tags.resource_type / resource_id / tag_id` | 三列全部保留，承担联合资源身份、标签关联、唯一性与级联删除 |
| `shortcut_settings.id / revision / overrides_json` | 全部保留。id=1 限定单份设置；revision 被条件更新用于防覆盖；overrides_json 保存用户配置，默认快捷键无法反推用户修改 |

标签未接入的设计来源：[仓库统一标签与启停设计](2026-09-02-repository-tags-status-design.md)；当前消费：[标签 Store](../../backend/internal/store/sqlite/repository_metadata_store.go)、[主题筛选标签](../../frontend/src/features/repository/ThemeRepositoryPage.tsx)。本轮不擅自撤销未来自定义标签的已记录目标。

其他依据：[会话命名事务](../../backend/internal/store/sqlite/thread_naming_store.go)、[前端名称版本比较](../../frontend/src/stores/threadStore.ts)、[模型恢复](../../backend/internal/service/resume.go)、[快捷键条件更新](../../backend/internal/store/sqlite/shortcut_settings.go)、[生成参考快照消费](../../backend/internal/contextengine/assembler.go)。

### 16.4 Runtime checkpoint 的列与嵌套字段

不能因为 checkpoint 确实有读者，就默认其中所有字段和所有历史行都值得保留。

| 字段／数据 | 审查结论 |
| --- | --- |
| 表列 `id / loop_id / phase / created_at` | id 未被生产接口外部引用；其余列的有效内容已在 JSON，读取不按这些列筛选。合并最新 checkpoint 后无需保留这些独立列；loop／phase 等真正运行状态仍在载荷内 |
| 表列 `run_id / seq / checkpoint_json` | run_id 改由 runs 主键承担；旧 seq 仅用于最新记录顺序，新建 checkpoint_revision 承担防覆盖版本；checkpoint_json 保留最新恢复载荷，不再另建 checkpoint 表 |
| JSON `run_id / loop_id / phase / resume_phase / mode` | 保留恢复身份与阶段语义。run_id 可由外层带入；mode 可从 runs 单一权威列带入，但要重构当前读取及一致性校验，不能移除执行模式信息 |
| JSON `plan / requirements / work_ledger / changes` | 恢复与提交对账有实际消费，保留；不是从当前页面内容能完整恢复的任务执行状态 |
| JSON `model_route / turns / tool_calls / active_duration_ms / waiting_duration_ms / completion_failures` | 保留模型回退状态、预算与失败计数；删除会使恢复后预算重置或实际模型丢失 |
| JSON `pending_command / pending_scope_expansion` | 保留完整待授权动作、参数、hash、范围及恢复阶段，防止授权与实际执行错配 |
| JSON `active_skills / active_components / dom_selections` | 当前恢复实际消费，保留历史内容语义；大型正文可以引用日志中的不可变载荷。不能只留资源 ID 后读取已被外部编辑的最新文件 |
| JSON `scope` | 当前由 RunCommand 恢复范围，checkpoint 中还用于批准事务一致性与 steering 更新。需收敛单一范围权威并保持同事务；暂不按“恢复没直接读”机械删除，尤其要覆盖 all_pages 运行中新页面的范围变化 |
| JSON `context_index_ref` | 随持久索引表删除。运行内检索缓存仍可有临时索引 ID，不持久化快照引用 |
| JSON `session`，尤其 `artifacts[].before_content / after_content` | 当前有内容的 session 快照在恢复中明确丢弃；只有空 session 被 RestoreRunSession 接受，等价重新创建。因此可删除 checkpoint 的 session 正文副本；实际已提交／未完成文件恢复继续依赖 mutation journal 和 receipt，不能删除后两者 |
| JSON `boundary / created_at` | 当前主要用于追踪／标记快照，非恢复状态。可只在 checkpoint 保存事件／trace 中记录，避免同时存表列及 JSON；若排障需要保留一份也不是恢复所必需 |
| JSON `evidence` | 未发现从 checkpoint 恢复 EvidenceLedger 的路径，但证据影响完成门禁；而 RenderProof 当前还被 json:"-" 排除。列为恢复语义待核查：定义恢复时重新取证，或从可信结果恢复并校验 hash，不能删字段来掩盖可能的恢复缺口 |
| JSON `waiting_question_id` | 写入但未见恢复读取；普通提问与 pending_command 的恢复机制不同。应先核对等待问题如何重建／重新提问、旧问题如何作废，再判断该字段应替换为事件引用还是删除；不直接认定为无意义 |

以上覆盖当前 RuntimeCheckpoint 的全部顶层字段。审批载荷中的 command_hash、preimage_hash、target_paths 等属于已授权动作校验，不作为“能从当前文件重算”而删除；它们记录的是批准当时的前提。

依据：[Runtime 恢复与 checkpoint 构造](../../backend/internal/workflow/runtime.go)、[checkpoint 保存边界](../../backend/internal/workflow/checkpoint.go)、[session 恢复与 mutation journal](../../backend/internal/workflow/session.go)、[Evidence 与 RenderProof](../../backend/internal/workflow/evidence.go)。

### 16.5 被替代表的字段如何处理

| 旧表／载荷 | 必须保留的信息与可消除的重复 |
| --- | --- |
| `run_contexts` 全部列 | 无恢复读取；ContextManifest、token 预算和 pack_hash 已可在上下文构建时产生。必要诊断记一次，不把它改名成另一张同内容的长期表 |
| `context_index_snapshots` 全部列 | 索引身份、运行归属、pack_hash、构建时间及 index_json 均作为可重建缓存退出；运行中的检索算法和素材正文保留 |
| `semantic_reviews` 全部列 | 如需诊断，保留内部审查事件的调用身份、接受与否、置信度、输入 hash 和原始结论；不增加一张专用结果表。完整 prompt_manifest 仅按明确诊断需求留存 |
| `run_events / git_commit_events` 全部列 | 事件归属、type、payload、时间继续进入日志；各自 seq 改成统一 thread seq，run_id／attempt_id 保持关联。不能只保留最终文本而丢掉工具、等待与结果事件 |
| `briefing_versions` 全部列 | briefing_id、kind、version_no、title、content、feedback、归属及时间改由命令身份／修订／成功事件表达；下一次修订仍能找到历史版本和反馈 |
| `context_compactions` 全部列 | 保留压缩身份、触发来源、Run／会话关联、标题、正文、前后 token、窗口和耗时的历史事实；reclaimed_tokens 可直接计算 max(0,before_tokens-after_tokens)，API 投影时生成，事件正文无需重复保存 |
| `thread_naming_inputs` 全部列 | 输入身份、内容、接受顺序由统一输入事件承担；命名首条＋近期输入的读取规则保留 |
| `command_activities.request / result / previous_title` | 请求、结果及修改前名称进入一次命令事件；修订通过 base_attempt_id 找基线，不在新尝试中反复复制整个旧结果 |
| `command_activities` 其余列 | 命令／尝试身份、归属、kind、状态、阶段及时间分别由通用控制表和事件承担；method 表示自动生成／手动命名等业务含义，迁入 rename 的类型化 input；它不是 HTTP 方法 |
| `thread_naming_operations` 全部列 | operation_id／action／request_id 用统一命令与尝试协议表达；request_hash、幂等状态及小回执进入通用幂等机制；不重复建立专用请求缓存 |
| `git_commit_operations` 全部列 | 身份、归属、请求去重、所选模型、状态／phase、结果／错误和时间仍有用途，分别进入通用控制、接受事件和终态事件。当前表有读者且参与 SQL 互斥，须同步替换触发器 |
| `deleted_slides / schema_migrations` 全部列 | 随已确认的产品与初始化规则退出，无替代历史表 |

补充载荷规则：RunCommand、活动资源和公开资源引用中的 `local_path / open_url` 可以按当前资源根目录、类型和 ID 在响应时生成；文件缺失就返回不可用。资源名称、描述、正文、页面 ordinal／title 等历史快照不能用当前资源／大纲覆盖。`ToolResult` 的 observation、evidence、render_proofs 等内部数据采用专用持久载荷，是因为公开 ToolResult 对这些字段标记 json:"-"，并非看见外层与内层同名就必然重复。

### 16.6 实施优先级与限制

1. 先清理无业务消费的字段、重复范围列和路径副本；随清理同步调整 PO、SQL、DTO、前端类型、seed、项目快照与文档。
2. 删除只写不读的独立上下文／审查存储和可重建索引缓存，保留正在执行的算法。
3. 将最新 checkpoint 收敛到 Run，核对计划、scope、steering、恢复模型与迟到写入；对 evidence、等待问题的恢复语义单独处理。
4. 接通可靠日志和通用命令，再移除具有现有消费者的历史表；不建立兼容双读、双写协议。
5. 标签字段保留，统一字典已接入管理页与输入匹配；本次不新增自定义标签编辑界面。

实施进度以第 17 节为准；设计决定不等于已经删除表／字段，也不代表恢复边界已经通过行为测试。验收仍按第 14 节交由用户手动执行，并增加路径迁移、重复请求、命名触发、模型配置变化、checkpoint 并发覆盖和等待问题恢复场景。

## 17. 实施结果与手动验收（2026-09-25）

**后端、前端与初始化入口已切换到新协议；尚未执行测试、构建或交互验证。** 以下是代码实现范围，不代表已经运行验收通过。请使用全新的工作目录，旧数据库会明确拒绝打开。

### 17.1 实现位置与职责

| 范围 | 当前实现 |
| --- | --- |
| 数据库 | [schema.sql](../../backend/internal/store/sqlite/schema.sql) 定义 12 表、索引、约束及 Run／Git 互斥；`schema.go` 检查 application_id、user_version 和表清单。编号迁移、迁移执行器及其旧表消费者已移除。 |
| 启动与写者锁 | [persistence/open.go](../../backend/internal/persistence/open.go) 先恢复未完成项目切换，再恢复 outbox、校验日志、中断普通命令并清理无引用载荷；server 与 init-resources 共用操作系统文件锁。 |
| 可靠日志 | [threadjournal](../../backend/internal/threadjournal/journal.go) 与 [journal_store.go](../../backend/internal/store/sqlite/journal_store.go) 实现身份头、连续序号、毫秒事件、fsync、重复投递核对、残缺尾行修复和完整损坏拒绝。业务事务分配序号并写 outbox，落盘后推进水位。 |
| 不可变正文 | `threadjournal/payload.go` 将长文本提取到 hash 文件，通过 `payload_fields` 的 JSON pointer 引用共享正文；剩余大 JSON 使用 `payload_ref`。引用在事务提交前同步发布，读取校验 hash 并还原。项目检查点和当前日志共同约束启动时的载荷清理。 |
| Run 与输入 | `run_store.go` 同事务创建 Run、接受事件和幂等回执；steering 保留输入事件引用。回答先持久化，重复答案幂等、冲突拒绝，恢复沿用原交互身份。 |
| 检查点 | `runtime_capabilities_store.go` 只更新 runs 中的最新检查点，并检查 owner、execution、scope 和 checkpoint 版本。worker 与 steering 共享写入租约；审批、范围更新和 steering 接受原子衔接检查点。失败时暂停并保留恢复凭据。 |
| 恢复现场 | `workflow/runtime.go` 恢复待处理问题、计划／命令／范围审批、预算、模型路由和活动资源内容快照；批准已提交但展示结果未发布时，检查点保留收尾标记；恢复会补发范围更新与模型观察，展示事件按审批身份去重，不重复批准或扩大范围。旧 waiting_question_id 字段已删除。RenderProof 根据当前内容及依赖 hash 校验。检索索引重建，检查点不保存 session 文件正文。 |
| 模型历史 | [contextengine/transcript.go](../../backend/internal/contextengine/transcript.go) 从日志构造消息，`model.edit` 保存精确消息身份、替换范围和水位。压缩保留完整工具轮次及后来新增输入，可连续替换旧摘要；Timeline 原始历史保留。 |
| 通用命令 | [service/commands.go](../../backend/internal/service/commands.go)、`command_store.go` 和 `httpapi/commands.go` 统一六类命令的请求、尝试、状态、取消及修订；已接受任务脱离 HTTP 生命周期。重启中断普通命令，自动命名仍按首次／每五次输入触发且不显示命令卡片。 |
| 公开历史 | `service/thread.go` 投影内部日志；前端 [threadJournal.ts](../../frontend/src/api/threadJournal.ts)、`historyHydrator.ts`、`eventReducer.ts` 统一历史和会话 SSE，游标包含 scene_revision／thread_id／seq，回退后重载。项目级事件仅更新会话列表。 |
| 副作用与回退 | `projecthistory` 在项目门禁内排空 outbox，保存文件和数据库同一水位；回退推进执行代次，隔离旧 worker。Git 执行前保存意图，按 HEAD／tree／信息摘要和 reflog 尝试标记对账，无法确认则禁止自动重试。PPT 提交回执、工具重放结果及事件意图在业务事务内衔接。 |
| 标签与初始化 | 标签名称、排序、系统标记由 `/api/v1/tags` 返回；资源页和输入匹配使用同一字典。显式初始化登记 seed 标签与资源，已有记录跳过；普通启动不补回预置资源。server 新增 `--work-root`，可在新目录验收。 |

现行接口：

- `POST /api/v1/threads/:id/commands`、`GET /api/v1/commands/:id`、`POST /api/v1/commands/:id/cancel`。
- `GET /api/v1/threads/:id/history`、`GET /api/v1/threads/:id/events`。
- `GET /api/v1/tags?scope=...`；自动命名开关走会话 PATCH，手动命名走 rename 命令。
- Run 创建、停止、恢复与交互接口保留；六类命令旧执行接口、旧 Run／Git SSE 和响应捕获中间件已删除。

### 17.2 本次静态复核及限制

- 已补充／调整日志恢复、载荷损坏、outbox、锁、初始化拒绝、命令断连与取消、尝试去重、检查点防覆盖、回答恢复、计划修订去重、连续压缩、Git 对账、项目回退和标签消费等行为测试源码。
- 已做源码交叉引用、旧表／旧路径残留检查、Go 格式化、TypeScript 语法解析与 diff 空白检查。未运行 Go 编译、TypeScript 类型检查、测试、构建或浏览器验证；依赖装配、类型匹配及运行结果仍须用户验证。
- 日志读取／追加目前会扫描历史，SSE 轮询和前端完整投影在长会话中的成本尚未测量；没有声称实现增量文件索引。
- macOS／Windows 的文件锁、目录同步、磁盘写入失败，以及突然断电后的跨文件恢复，需要在实际目标文件系统验收。突然崩溃仅恢复到最后已持久化边界，token delta 不构成恢复保证。
- 新格式是一次性开发期切换，不支持旧库、旧 JSONL 或旧前端协议。请勿用生产工作目录做故障注入；回退验收应覆盖旧 worker 返回、已接受未消费答案和已完成副作用。

### 17.3 用户手动执行命令

后端聚焦行为测试及构建：

```sh
cd /Users/wyw/Desktop/Projects/PPT-Agent/backend
go test ./internal/threadjournal ./internal/workrootlock ./internal/store/sqlite ./internal/contextengine ./internal/contextcompact ./internal/run ./internal/workflow ./internal/service ./internal/httpapi ./internal/projecthistory ./internal/gitcommit
go build ./cmd/server ./cmd/init-resources
```

前端测试及构建（build 包含 TypeScript 类型检查）：

```sh
cd /Users/wyw/Desktop/Projects/PPT-Agent/frontend
pnpm test
pnpm build
```

新工作目录初始化两次，再启动后端；需要使用项目现有模型配置和渲染运行时：

```sh
cd /Users/wyw/Desktop/Projects/PPT-Agent/backend
PPT_ACCEPTANCE_ROOT="$(mktemp -d /tmp/ppt-agent-persistence.XXXXXX)"
go run ./cmd/init-resources --work-root "$PPT_ACCEPTANCE_ROOT" --seed-root ../seed
go run ./cmd/init-resources --work-root "$PPT_ACCEPTANCE_ROOT" --seed-root ../seed
go run ./cmd/server --work-root "$PPT_ACCEPTANCE_ROOT"
```

另开终端启动现有前端开发入口：

```sh
cd /Users/wyw/Desktop/Projects/PPT-Agent/frontend
pnpm dev
```

### 17.4 手动交互验收清单

1. 在新目录创建项目、会话、主 Run，检查 Timeline、工具结果、页面状态和输入引用；删除被引用页面后前后端均拒绝发送旧引用。
2. 分别执行 rename／polish／kickoff／handoff／compact／commit，刷新或离开页面、断开网络再恢复，确认命令继续执行、历史只出现一次；明确停止只取消对应当前尝试。
3. 同一命令成功后修订失败，确认保留最近成功正文；重复请求键返回原尝试身份，冲突请求拒绝；停止与副作用完成竞争时显示实际结果。
4. 普通提问和三类审批分别在等待、答案已接受但未消费时重启服务，再显式恢复 Run，确认问题身份不变、答案不丢失或重复注入。普通命令显示中断并等待重试。
5. 连续压缩两次，在压缩期间新增输入，核对新增内容保留、工具调用和结果成对、Timeline 原历史仍在；预算无法容纳完整单元时应失败而不丢消息。
6. Run 暂停后修改模型配置和活动资源文件，确认恢复遵守模型快照与历史资源正文；修改渲染依赖后旧 RenderProof 失效，需要重新取证。
7. 项目回退再恢复最新，检查所有会话、页面与执行状态一起变化；旧 SSE 游标触发重载，旧 worker 不能覆盖新状态。
8. 在专用目录注入文件／数据库失败、残缺尾行、完整非法记录及缺失 hash 载荷：可恢复尾行按原事件重投递，其余明确报错；恢复投递不得重复模型调用、PPT 修改或 Git 提交。
9. 在 Git 成功后、终态持久化前中断，再启动检查 reflog 对账；不确定结果显示中断，不自动再次提交。
10. 停止服务后修改数据库标签名称与顺序，再启动检查筛选、名称与输入匹配；再次显式初始化保留已有修改，普通启动不补回删除的资源。第二个服务或写入 CLI 打开同一目录应被锁拒绝。
