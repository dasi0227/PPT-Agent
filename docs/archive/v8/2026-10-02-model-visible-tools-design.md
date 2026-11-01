# 模型可见工具

本文描述当前实现中 LLM 实际看到的工具输入和输出。编辑调度与源码协议对应 [2026-10-02 开发设计](docs/spec/2026-10-02-编辑依赖调度与源码一致性-design.md)。主 Agent 工具按模式、阶段和项目配置披露；Reviewer 和单次模型命令使用独立工具集合，同一轮不包含以下全部工具。

## 定义来源与审查范围

实际发送给模型的是代码构造的 Schema，本文不会自动注入工具定义。当前共 23 个工具名称、24 个定义入口；主 Agent 与单次提交信息生成复用 `GitCommitToolSchema()`，结束流程由各入口控制。

| 调用入口 | 工具 | Schema 来源 |
| --- | --- | --- |
| 主 Agent；部分供 Reviewer 复用 | `read_resource`、`edit_manifest`、`edit_design`、`edit_spec`、`edit_outline`、`edit_html` | `backend/internal/workflow/ppt_tools.go`、`ppt_helpers.go`；Manifest、Design、Spec 字段来自 `backend/schemas/*.schema.json` |
| 主 Agent、Reviewer | `render_slide`、`read_image` | `backend/internal/workflow/render_tool.go`、`image_tools.go` |
| 主 Agent | `load_component`、`load_skill` | `backend/internal/workflow/repository_tools.go` |
| 主 Agent | `run_command`、`git_commit` | `backend/internal/workflow/project_command_tool.go`、`git_commit_tool.go` |
| 主 Agent | `create_plan`、`update_plan`、`request_privilege`、`ask_user`、`review_task`、`finish_task` | `backend/internal/workflow/runtime.go`、`plan_schema.go` |
| Reviewer | `submit_review` | `backend/internal/workflow/reviewer.go` |
| 单次模型命令 | `rename_thread`、`polish_instruction`、`handoff_thread`、`git_commit` | `backend/internal/service/thread_naming.go`、`polish.go`、`briefing.go`、`git_commit.go` |
| 上下文压缩 | `compact_context` | `backend/internal/contextcompact/compactor.go` |

### 输出契约与模型适配

所有 24 份工具定义都声明 `OutputSchema`。主 Agent 与 Reviewer 的输出契约集中在 `backend/internal/workflow/tool_output_schema.go`，定义点显式引用；单次模型命令通过 `llm.NoReplyOutput` 声明不继续工具往返。这里描述的是实际模型 Observation，而不是内部 `ToolResult`、前端卡片或持久化字段。

- 普通 JSON 返回字段使用 JSON Schema 的 `properties`、`items`、`required`、`oneOf` 和 `description`，包含嵌套字段、出现条件及业务语义。Manifest、Design、Spec 复用现有资源 Schema，并在独立副本中调整仅适用于输入的说明。
- `x-content-kind` 是输出 schema 的传输注解，取 `json`、`json+image`、`image` 或 `none`；图片和无回复不会被虚构成 JSON 字段或 `null`。`x-error-schema` 描述错误 JSON，包括常规错误、参数/资源校验、精确替换失败、命令部分输出及完成检查问题。这两个注解不出现在实际工具结果中。
- Responses 和 Anthropic 适配器统一调用 `llm.ModelToolSchemas`，将输出 schema 序列化为工具 `description` 的 Output contract 段落；不向供应商 API 增加不支持的 `output_schema` 字段。业务代码只维护 schema，不手写另一份返回字段说明；重复发送不会修改原定义或重复追加说明。
- 主 Agent、Reviewer、单次命令与上下文压缩均保留输出契约；上下文用量估算计入生成的模型可见说明。输出契约用于模型说明，没有新增通用运行时输出校验。

`backend/internal/llm/tool_output_test.go` 检查两种适配器请求中的嵌套输出说明、图片/结束语义、输入保持不变和重复调用不重复注入。编辑事务、版本、恢复重放及展示缓存使用定向测试验证，界面和真实 Run 交由用户手动验收。

`polish_instruction`、`handoff_thread` 和 `compact_context` 复用 `backend/internal/commandresult/text.go` 构造参数，业务说明由各调用入口提供。Reviewer 仅复用 `read_resource`、`read_image`、`render_slide`，不获得主 Agent 的编辑工具。供应商适配层继续传递这些参数 Schema。

业务参数说明包含嵌套字段、数组条目、互斥输入、清空与保留语义、枚举含义和用户可见内容。`edit_outline.init` 沿用开放对象 Schema，在 description 中解释 `sections`、`subsections`、`slides`、`title`、`purpose`。

通用约定：

- 以下“输出”默认指正常返回；字段按列出的结构组织，不增加通用 `data` 包装。图片以实际图片内容块提供，不是图片路径或 JSON 字符串。
- 所有模型可见结果均不包含 `ok`；工具执行失败返回明确的错误提示。常规结构化错误保留 `code`、`category`、`reason`、`retryable`、`next_action`，按需包含 `call_id`、`resource`、`operation` 或具体问题详情。
- 模型不传递 `expected_hash`，也不接收 `content_hash` 或 `source_hash`。资源版本与冲突校验由后端管理；冲突时工具提示重新读取后修改。Git 提交的 `hash` 仍保留。
- 用户作出的审批决定属于正常业务结果；审批服务异常等执行故障走工具错误处理。
- `finish_task` 和 `submit_review` 成功后结束各自流程，没有后续工具回复。

### 编辑批次、版本与错误

- Runtime 按原始调用顺序分析逻辑资源及物理文件，独立创作调用最多并发 3 项；同一资源的连续编辑要求前项成功提交。不同页面 HTML 互不依赖，Outline、Design、Manifest 之间不因工具名称建立依赖。
- 前序 Manifest、Design 更新是后续 Spec、HTML 的成功依赖；Spec 更新只阻断同页后续 HTML。Outline 按节点身份和实际变更识别影响，未知影响保守等待；不同页面 Spec 共用 `.spec.json`，准备与提交串行，但存储等待不传播失败。
- 每项调用独立暂存，项目提交锁覆盖基线复核、日志、文件、元数据及回滚，与管理编辑共用。无实际变更的成功调用仍复核资源版本。成功回执仅在提交后发布；重放已提交调用返回持久化结果，不重复写入。结果按原始 call ID 和顺序返回。
- 普通读取等待相关写入结束后诊断当前资源，失败的写入不阻断读取。渲染等待对应写入提交；控制、审批、命令写入与 Git 生命周期保留原门禁。
- 只有已返回的真实内容，或已知输入能确定的成功写入，才推进模型版本。HTML 创作需求快照仅包含执行依据和已成功的前序要求，不套用后续或未见的磁盘状态；新增后端 ID 要等返回后在下一轮使用。
- 结构化 Manifest、Design、Spec 使用 JSON 内容哈希，排版和对象键顺序变化不改变版本；数组顺序和字符串内容仍有意义。Outline、HTML 使用原文哈希，文件字节哈希与 JSON 内容哈希分开管理。
- `CONTENT_CONFLICT` 区分尚未取得当前资源与读取后发生变化，`next_action` 包含可直接使用的资源参数；先重新读取并生成编辑，禁止盲重试。`EDIT_ANCHOR_NOT_FOUND` / `EDIT_ANCHOR_AMBIGUOUS` 返回 `edit_index`、`match_count` 及唯一锚点获取方法，不把匹配失败直接等同于并发冲突。缺失页面要求先读 Outline 确认稳定 ID。
- `DEPENDENCY_FAILED` 表示本调用未执行，返回 `dependency_call_id`、`dependency_tool`、`dependency_resources`（资源 `type`、`part` 及可选 `slide_id`）和修复动作。跳过不消耗额外修复预算，不影响独立分支；时间线仅发布阻断结果，不发布开始或成功写入事件。

## read_resource

功能：读取演示文稿的全局资源或指定页面资源。

输入

- `resource`：必填；资源类型，取 `manifest`、`design`、`outline`、`spec`、`html` 之一。
- `slide_id`：页面稳定 ID；读取 `spec` 或 `html` 时必填，其他资源禁止传入。

输出

- `resource`：实际读取的资源类型。
- `slide_id`：页面资源对应的页面 ID；仅页面资源返回。
- `content`：资源内容；`manifest`、`design`、`spec` 为 JSON 对象，`outline`、`html` 为原始文本。

## render_slide

功能：在隔离 Chromium 中渲染指定页面，直接提供截图内容块和文字诊断，供模型检查外观。仅在执行模式披露。

输入

- `slide_id`：必填；要渲染的页面稳定 ID，必须在本次运行授权范围内。

输出

- `image`：实际截图内容块，不是 JSON 字段，也不是路径或 URL。
- `slide_id`：渲染的页面 ID。
- `diagnostics`：诊断对象，仅包含 `overflow`、`out_of_bounds`、`console_errors` 和 `failed_resources` 四项。
- `diagnostics.overflow`：页面整体溢出说明对象，由可滚动内容尺寸与画布对应的视口尺寸比较得出，允许 1px 容差。
- `diagnostics.overflow.horizontal`：字符串，说明是否横向溢出、内容宽度与画布宽度，溢出时给出超出像素数，例如“未溢出：内容宽度 1920px，画布宽度 1920px。”。
- `diagnostics.overflow.vertical`：字符串，说明是否纵向溢出、内容高度与画布高度，溢出时给出超出像素数，例如“溢出：内容高度 1140px > 画布高度 1080px，超出 60px。”。
- `diagnostics.out_of_bounds`：超出 `.slide-stage` 边界的后代元素数组；没有舞台节点时以文档根元素为参照。允许 1px 容差，最多记录 50 项，无越界时为空数组。
- `diagnostics.out_of_bounds[].tag`：元素标签名，小写。
- `diagnostics.out_of_bounds[].id`：元素 ID，没有则省略。
- `diagnostics.out_of_bounds[].class`：元素 class 字符串，可省略，最多 160 个字符。
- `diagnostics.out_of_bounds[].rect`：元素边界坐标，包含 `left`、`top`、`right`、`bottom`，相对于页面 iframe 的视口，单位为 CSS 像素，四舍五入为整数。
- `diagnostics.console_errors`：控制台 error 级别消息及未捕获脚本错误的字符串数组；最多 50 条，每条最多 1000 个字符，无错误时为空数组。
- `diagnostics.failed_resources`：资源请求失败、HTTP 400 及以上响应或外部资源被拦截的说明字符串数组，包含失败原因或状态码及资源地址；去重后最多 50 条，无失败时为空数组。

`out_of_bounds` 检查元素边界，不保证元素实际被裁切，也不覆盖舞台内部的父容器裁切、文字省略或元素互相遮挡；整体 `overflow` 与元素越界不完全等价。后端继续等待字体加载过程结束后截图，字体加载失败信息按采集结果进入资源或控制台错误，不提供独立字体状态字段。

截图有效但诊断发现阻断问题时，仍返回截图和上述诊断，并增加 `code` 与 `reason` 说明问题；渲染本身失败时返回常规工具错误。后续需要重新查看已有截图时，可通过 `read_image(slide_id)` 读取；页面内容变更后必须重新渲染。

## read_image

功能：读取用户上传的图片附件，或读取指定页面最新且有效的渲染截图，直接向模型提供图像内容。

输入

- `attachment_id`：上传图片附件 ID；与 `slide_id` 必须且只能提供一个。
- `slide_id`：要读取截图的页面稳定 ID；与 `attachment_id` 必须且只能提供一个。

输出

- `image`：实际图片内容块，不是 JSON 字段；成功时仅返回图片，不附带元信息文字块。

上传附件以原图为读取来源，必要的格式转换和压缩由后端处理。页面截图不存在或已过期时返回错误，提示先调用 `render_slide`。HTML 嵌入图片所需的附件引用地址由附件上下文提供。

## load_component

功能：按稳定 ID 批量加载已启用的组件 HTML 供参考和改编，不直接写入或修改项目文件。单次最多 8 个组件，总 HTML 不超过 192 KiB。

输入

- `ids`：必填；不重复的组件 ID 数组，至少 1 个、最多 8 个。

输出

- `components`：本次新提供正文的组件数组。
- `components[].id`：组件稳定 ID。
- `components[].content`：组件完整 HTML 源码。
- `already_available`：正文已经在当前模型上下文中、无需重复传输的组件 ID 数组。

组件名称和简介由预置组件列表提供。仅有目录条目而没有完整正文，不算 `already_available`。

## load_skill

功能：按 ID 加载已启用的仓库技能，供当前运行使用。单次最多 8 个技能。

输入

- `ids`：必填；不重复的技能 ID 数组，至少 1 个、最多 8 个。

输出

- `skills`：本次新提供正文的技能数组。
- `skills[].id`：技能稳定 ID。
- `skills[].content`：技能完整正文。
- `already_available`：正文已经在当前模型上下文中、无需重复传输的技能 ID 数组。

技能名称和简介由预置技能列表提供。仅有目录条目而没有完整正文，不算 `already_available`。

## edit_manifest

功能：更新演示文稿需求清单中传入的顶层字段；未传入字段保持不变。仅在执行模式披露。

输入

- `title`：演示文稿标题。
- `language`：演示文稿语言标识。
- `pages`：期望页数或页数范围。
- `audience`：目标受众及其背景。
- `goal`：演示文稿希望受众理解、决定或做到的事项。
- `requirements`：必须覆盖的内容、证据或表述要求数组；传入时整体替换。
- `prohibitions`：明确排除的内容数组；传入时整体替换。

以上字段至少传一个。

输出

- `content`：保存后的完整需求清单 JSON 对象。
- `changed_fields`：实际发生变化的字段数组；没有变化时为空数组。

## edit_design

功能：更新全稿设计要求或共享装饰的位置。未传入字段保持不变；`decorations` 仅合并传入的位置字段。仅在执行模式披露。

输入

- `requirements`：整套演示在页面创作时持续遵循的视觉设计要求数组，界面展示为「设计要求」；传入时整体替换，`[]` 表示清空。
- `decorations`：共享装饰位置设置，仅更新传入的子字段；`left-edge`、`right-edge` 表示左右边缘的垂直中点。文字来自以下对应资源，外观由 Runtime 和主题控制。
- `decorations.page_number`：大纲顺序生成的数字页码位置；封面和结尾页也显示，不允许 `none`。
- `decorations.deck_title`：Manifest 标题的位置。
- `decorations.section_title`：当前一级章节标题的位置，不是子章节标题。
- `decorations.key_message`：当前页 Spec 核心信息的位置。后三项允许 `none` 表示隐藏，缺少对应文字时不渲染。

以上字段至少传一个。

每条设计要求应独立、明确、可执行，可涉及整体观感、信息密度、排版与层级、图文表达、视觉元素、参考与品牌约束或动效，允许适用条件、偏好和明确禁令。只记录本项目实际要求，无需填满所有维度；内容要求放在 Manifest，单页布局放在该页 Spec，不重复通用质量规则或所选主题的现有配置。配色、字体等明确要求由相应主题能力落实，当前不支持的能力应说明。更新要求不代表已有 HTML 已同步修改。

装饰合并后的配置中，每个非 `none` 位置只能分配给一个装饰，缺少文字也仍占用位置；重复位置会返回包含冲突双方及位置的错误，且不保存。`none` 可以重复，页码不能使用 `none`。调整占用者或交换位置时，可以在同一次编辑中提交多个位置字段。

输出

- `content`：保存后的完整设计 JSON 对象。
- `changed_fields`：实际发生变化的字段数组；没有变化时为空数组。

## edit_spec

功能：更新指定页面的语义设计规格；首次创建页面规格时必须提供 `key_message` 和 `elements`。未传入字段保持不变。仅在执行模式披露。

输入

- `slide_id`：必填；页面稳定 ID。
- `key_message`：本页核心信息。
- `elements`：按顺序组织的内容元素数组，传入时整体替换，`[]` 清空。
- `elements[].type`：元素表达形式，使用 `text`、`list`、`metric`、`quote`、`table`、`chart`、`diagram`、`code` 或 `asset`；比较属于页面角色，可用图表、表格或关系图表达。
- `elements[].intent`：该元素要传达的内容、证据或关系及其在页面中的作用，使用自然语言，不编造缺失事实。
- `role`：页面语义角色；传 `null` 可移除已有可选角色。
- `layout`：内容排布和阅读顺序的自然语言建议，不是模板编号或 CSS；传 `null` 可移除已有可选布局。

除 `slide_id` 外，至少传一个业务字段。

输出

- `content`：保存后的完整页面规格 JSON 对象。
- `changed_fields`：实际发生变化的字段数组；没有变化时为空数组。

## edit_outline

功能：初始化尚不存在的大纲，或对已有大纲 JSON 源码执行精确文本替换。仅在执行模式披露。

输入

- `init`：完整初始大纲 JSON 对象，包含章节、子章节和页面；仅在大纲不存在时使用，与 `edits` 必须且只能提供一个。
- `edits`：非空的有序替换数组，仅用于已有大纲；与 `init` 必须且只能提供一个。
- `edits[].old_text`：必填、非空；待替换的原文，在该项执行时必须恰好匹配一次。
- `edits[].new_text`：必填；替换后的文本。

新节点省略 ID，由后端生成；已有节点改名、移动或编辑时保留原 ID。已有大纲即使为空也不能再次初始化。删除页面会一并清理其 Spec 和 HTML，全部相关修改原子保存。

输出

- `content`：实际保存的完整大纲 JSON 源码字符串，包含后端生成的稳定 ID，可直接用于后续精确文本编辑。

## edit_html

功能：创建、完整替换或精确修改指定已有页面的 HTML 源码。仅在执行模式披露；保存后需调用 `render_slide` 检查外观。

输入

- `slide_id`：必填；页面稳定 ID。
- `content`：完整 HTML 源码字符串；页面 HTML 不存在时创建，存在时完整替换。与 `edits` 必须且只能提供一个。
- `edits`：非空的有序替换数组，仅用于已有 HTML；与 `content` 必须且只能提供一个。
- `edits[].old_text`：必填、非空；待替换的原文，在该项执行时必须恰好匹配一次。
- `edits[].new_text`：必填；替换后的文本。

全部替换原子保存；匹配失败时返回对应替换项及具体问题。保存相同内容也视为成功。

HTML 输入和精确替换结果原样保存，不改缩进、换行符、引号或 CSS。首次生成清晰可读的源码，局部修改沿用原排版；成功原样写入且未变化时可直接继续精确编辑。尚未读取、版本已变化或上下文缺少源码时先用 `read_resource` 读取，不凭记忆构造锚点。结构化 JSON 工具继续统一序列化为两空格缩进并带末尾换行，大纲返回实际完整源码。

源码视图和消息源码卡片共用展示 Worker 与缓存，相同版本的并发请求合并。展示配置为 HTML parser、两空格、print width 100、strict 空白敏感、嵌入语言格式化和 LF；失败或 Worker 不可用时显示原文。接口、哈希、Agent 读取、页面渲染、源码复制和导出继续使用保存原文。

输出

- `summary`：保存结果说明，例如“HTML 已保存。”；不回传整份 HTML。

保存成功不表示视觉检查通过。

## run_command

功能：在项目目录中运行受限命令，读取项目文件或 Git 状态；唯一写操作是在执行模式下对单个文件执行经确认的 `sed -i` 替换。编辑前需完整单文件 `cat` 取得真实文本；Outline、HTML 的 `read_resource` 原文也可复用。Manifest、Design、Spec 的对象读取不能授权文本修改；部分读取或原文变化后需重读。

输入

- `command`：必填；受限命令文本，最长 4096 字符。可用只读命令包括 `ls`、`cat`、`head`、`tail`、`find`、`grep`、`jq`、`rg`、`pwd`、`stat`、`sed -n`、`wc`、`git status`、`git diff`、`git log`。

输出

- `stdout`：已捕获的标准输出。
- `stderr`：已捕获的标准错误。
- `exit_code`：命令退出码。
- `output_truncated`：仅输出被截断时返回，固定为 `true`；未截断时省略。

输出超限时终止命令，返回明确的超限错误提示，并保留已捕获的输出、退出码和截断标志；不提供完整输出文件或分段读取接口。

需要用户审批的命令必须单独调用，审批期间挂起；批准后执行并返回执行结果，拒绝时通过本次工具调用返回拒绝错误，不生成虚假的命令输出。

命令产生的 HTML、JSON 原文直接暂存，保留语法、Schema、页面身份与作用范围校验，不额外格式化。文本版本按真实文件字节校验，不能用等价 JSON 内容哈希替代；下一次结构化工具更新可以重新统一序列化。

## git_commit

功能：将当前项目允许纳入版本管理的源文件提交到本地 Git 仓库；不执行远程推送。主 Agent 在提交服务启用的执行模式披露此工具，`/commit` 命令复用同一份工具定义和提交服务。根据已提供的暂存改动填写提交信息；未提供改动证据时，先用 `run_command` 查看 Git 状态和差异。

输入

- `title`：必填；根据实际改动生成单行中文 `<类型>: <核心改动概述>`，类型为 `feat`、`fix`、`refactor`、`perf`、`chore` 或 `docs`，最长 72 字符，无句末句号。
- `items`：必填；1 至 6 条中文实际变动，每条最长 160 字符，每项一句，不加项目符号，不编造未来工作、测试结论或完成情况。

输出

- `summary`：提交结果说明；无可提交改动时仅返回此字段，例如“当前项目没有可提交的变更。”。
- `hash`：实际产生的 Git 提交标识；仅产生提交时返回。
- `branch`：实际提交所在分支；仅产生提交时返回。

以上是主 Agent 循环中的工具回复。`/commit` 仅允许一次此工具调用，不附普通文本；后端执行提交并发布命令结果后结束，不再向模型发送工具回复或发起下一轮请求。

## create_plan

功能：创建完整计划草案并提交用户审批；计划尚未获批时可反复调用，完整替换当前草案并重新提交审批。调用挂起等待用户决定，再返回审批结果。计划获批后不可通过此工具覆盖当前计划。

输入

- `title`：必填；计划标题。
- `content`：必填；完整 Markdown 计划正文。
- `steps`：必填、非空；计划步骤数组。
- `steps[].title`：必填；步骤标题。
- `steps[].target_slide_ids`：可选；该步骤关联的已有页面 ID 数组，使用当前大纲和授权范围内的稳定 ID；不创建未来页面身份，也不授予编辑权限。无关联已有页面时省略或传 `[]`。

计划与步骤 ID 由后端管理，无需模型传入。

输出

- `decision`：`approve`、`revise` 或 `refuse`。
- `summary`：与审批决定对应的说明或修改建议，规则如下。

审批结果：

- `approve`：固定为“用户已批准计划，可以开始执行。”；冻结计划结构并进入执行。
- `revise`：返回用户修改建议原文；未填写时为“用户要求修改计划，但未提供具体建议。”。计划保持未获批，修改后再次调用 `create_plan` 提交审批。
- `refuse`：固定为“用户已拒绝计划，不得执行该计划。”；不得执行或自动重新提交该计划。

审批结果通过原工具调用返回，不额外注入审批结果消息。

## update_plan

功能：仅更新当前已获批计划中已有步骤的执行状态；不修改计划标题、正文或步骤结构。

输入

- `updates`：必填、非空；步骤状态更新数组。
- `updates[].step_id`：必填；当前计划中已有步骤的 ID，使用计划上下文提供的准确 ID。
- `updates[].status`：必填；`pending`、`processing`、`completed` 或 `failed`。

无需传入 `plan_id`。同时至多一个步骤处于 `processing`，已完成步骤不得回退。

输出

- `summary`：步骤状态更新结果说明。

## request_privilege

功能：请求用户扩大可编辑页面范围；仅在执行模式披露，必须单独调用。需要审批时挂起等待，并通过原工具调用返回用户决定。

输入

- `slide_ids`：必填、非空；希望追加授权的页面 ID 数组，不表示替换整个授权范围。
- `reason`：必填；申请扩展的原因。

输出

- `decision`：`approve`、`revise` 或 `refuse`。
- `summary`：与授权决定对应的说明，规则如下。

审批结果：

- `approve`：批准所申请页面，固定为“用户已批准所请求页面的编辑权限。”。
- `revise`：用户选择“允许全部项”，直接授权所有页面，包括本次运行中新建的页面；固定为“用户已将编辑范围扩大至所有页面，包括本次运行中新建的页面，可以继续执行。”。授权立即生效，无需修改申请后再次审批。
- `refuse`：维持原授权范围，固定为“用户已拒绝扩展权限，请在原有授权范围内继续。”。

所请求页面已全部获授权时不重复审批，返回 `decision: "approve"` 和 `summary: "所请求页面已具备编辑权限，无需再次审批。"`。

## ask_user

功能：提出一组需要用户回答的问题，挂起等待，并通过原工具调用返回用户答案后继续当前运行。

输入

- `questions`：必填、非空；问题数组。
- `questions[].question`：必填；直接展示给用户的完整、明确疑问句，包含回答所需上下文，必须以 `?` 或 `？` 结尾；禁止分类标题、陈述句和指令句。这些句式要求仅通过 Tool Definition 的 description 引导模型，前后端不校验句式或问号。
- `questions[].reason`：必填；为什么需要用户回答，以及答案会影响什么，避免空泛说明或重复问题正文。
- `questions[].options`：可选；最多三个预设选项，无预设选项时由用户填写答案。
- `questions[].options[].label`：必填；选项名称。
- `questions[].options[].description`：必填；选项的具体含义、影响或取舍，避免重复选项名称。
- `questions[].allow_custom`：可选；有预设选项时是否允许自定义回答，默认 `false`；无预设选项时始终使用填空回答，不受该值影响。

问题和选项 ID 由后端生成、维护，不由模型传入。

输出

- `answers`：按原问题顺序排列的问答数组。
- `answers[].question`：对应问题的 `question` 字符串。
- `answers[].answer`：字符串；选择预设选项时为其 `label`，自定义回答或填空时为用户原文。

用户跳过某题时，仍返回该题，`answer` 固定为“用户跳过了此问题，请结合已有信息自行判断；如有推荐选项，可优先采用，但不要将其视为用户明确选择。”，不返回 `null`。跳过不表示用户批准需要明确授权的操作。

整个交互取消或发生异常时走取消／错误处理，不伪造答案。不重复回传问题原因、选项说明或内部 ID。

## review_task

功能：在执行模式中委派独立的 PPT 成果审查，不审查计划或最终回复，不修改业务文件，也不自动完成主任务。

输入

- `demand`：必填、非空；说明待审查的成果、范围、重点和适用要求，不能覆盖用户指令。

输出

- `decision`：`approve`（审查通过）、`revise`（需要核实／修订）、`refuse`（拒绝交付）。
- `reasons`：非空字符串数组；三种结论都必须说明批准依据、需核实／修订事项或拒绝交付的原因。

审查执行失败时返回工具错误，不伪装为 `revise` 或 `refuse`。

## finish_task

功能：提交完整的最终回复。可在聊天、追问或执行模式的相应阶段披露；计划被用户拒绝后也可用于结束回复。必须单独调用。

输入

- `message`：必填；给用户的完整最终回答。
- `suggested_next_inputs`：可选；最多三条、每条不超过 80 字符的后续输入建议。

输出

- 成功：无工具回复；Runtime 直接完成当前运行并将 `message` 用作最终回答。
- 被完成条件拦截：返回包含 `code`、`category`、`reason`、`retryable`、`next_action` 及可能的 `issues` 的错误回复，供模型修正。

## submit_review

功能：Reviewer 提交最终成果审查结论并结束独立审查循环；主 Agent 不会收到这个工具的 Schema。

输入

- `decision`：必填；`approve`、`revise` 或 `refuse`。
- `reasons`：必填；至少一个非空字符串，说明批准依据、需核实／修订事项或拒绝交付的原因。

输出

- 成功：无工具回复；Reviewer 循环结束，主 Agent 随后通过 `review_task` 收到对应的 `decision` 和 `reasons`。
- 无效提交：收到 `code`、`field`、`reason`、`next_action` 错误反馈，可在预算内修正；不会提前发布最终失败。

必须作为最后一次响应中的唯一工具调用；纯文本不作为审查结果。

没有工具调用时保留原始响应并追加 runtime 指导，允许继续检查或提交。提交混用、重复提交或禁止工具先拒绝整批，每个 call ID 均补齐失败结果，不执行其中任何调用。最多 2 次协议纠正，合法检查不会重置额度；32 轮请求、128 次检查、5 分钟超时及原有预算、取消和源文件一致性检查继续生效。普通检查失败与协议纠正分开计数；伴随正文不额外阻止唯一合法提交。

## 单次模型命令与上下文压缩

以下工具用于专用模型调用收集结果，不加入主 Agent 的工具循环；均要求结果只通过一次对应工具调用提交，不另附普通文本。模型提交成功后由调用方消费结果，不继续工具往返。

### rename_thread

- `action`：必填；`rename` 表示近期工作已有明确主题且现名不能准确表示，`keep` 表示现名仍合适或信息不足。
- `title`：`rename` 时必填，`keep` 时禁止传入；使用对话语言概括近期主要任务或阶段，1–60 字符的单行纯文本，不含 Markdown、HTML、状态前缀或句末句号。

成功无回复，合法 rename/keep 只接受一次；无效提交返回 `code`、`field`、`reason`、`next_action` 并要求修正。每条提交响应须恰好一个 `rename_thread`，无普通正文。命名首次请求之外最多追加 2 次 Generate，协议纠正与明确不支持约束后的重发共享额度；沿用同一 20 秒超时、128 输出 token、上下文与 command execution，不重新运行 JEV。

命名独立传递指定工具和单次调用约束：Responses 使用 `tool_choice={type:function,name:rename_thread}`、`parallel_tool_calls=false`；Anthropic 使用 `tool_choice={type:tool,name:rename_thread,disable_parallel_tool_use:true}`。未设置约束的调用方保持原行为。任务内按实际 provider、模型、协议和端点记录能力；仅机器错误码明确不支持且字段精确指向约束时撤去相应限制，普通 400、鉴权、超时和服务错误不会触发撤限。能保留单次调用限制时继续保留，备用路由重新核对策略。provider 解析失败没有可安全回放的标准响应时不伪造消息。

诊断记录请求身份、实际策略、工具数量/名称、正文存在性、字段和校验原因、内部请求序号及剩余额度，不保存正文或参数。中间纠正保持活动运行；写回继续检查取消、项目 generation、operation version 和自动命名状态，旧结果不得覆盖新状态。

### polish_instruction

- `title`：必填；最多 48 字符的单行纯文本，使用用户语言说明表达改善方向，不复述任务或声称任务完成；无需修改时说明保持原文。
- `content`：必填；用户可直接发送的完整润色草稿，结合本次修订反馈保留原意、语言和范围；无需修改时返回原文。不回答或执行草稿，不补写需求或附加解释。

### handoff_thread

- `title`：必填；最多 48 字符的单行纯文本，使用用户语言说明交接的工作或阶段。
- `content`：必填；面向接手 Agent 的完整独立 Markdown 交接正文，保留目标、已确认决定、实际进展、证据局限、剩余工作与下一步。修订时返回完整替代正文；已完成时不编造未完成事项。

此工具仅提交交接正文，不创建会话或启动 Agent。

### compact_context

- `title`：必填；最多 48 字符的单行纯文本，使用对话语言说明任务、阶段或关键决定。
- `content`：必填；供同一任务继续使用的完整 Markdown 工作摘要，按提示词包含“目标与意图”“已完成改动”“关键决策”“未决问题”“下一步”五个二级标题。保留约束、决定、相关页面和附件 ID、实际进展、证据局限与剩余工作；区分完成、尝试和计划，历史授权记录不构成新授权。

以上三个 `title + content` 工具的标题均不含 Markdown、HTML、命令前缀或句末句号；正文长度限制沿用各调用方配置。
