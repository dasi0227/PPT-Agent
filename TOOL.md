# 模型可见工具

本文描述当前实现中 LLM 实际看到的工具输入和输出，与 `TODO.md` 已确认需求对应。工具按模式、阶段和项目配置披露，同一轮不一定包含以下全部工具。

通用约定：

- 以下“输出”默认指正常返回；字段按列出的结构组织，不增加通用 `data` 包装。图片以实际图片内容块提供，不是图片路径或 JSON 字符串。
- 所有模型可见结果均不包含 `ok`；工具执行失败返回明确的错误提示。常规结构化错误保留 `code`、`category`、`reason`、`retryable`、`next_action`，按需包含 `call_id`、`resource`、`operation` 或具体问题详情。
- 模型不传递 `expected_hash`，也不接收 `content_hash` 或 `source_hash`。资源版本与冲突校验由后端管理；冲突时工具提示重新读取后修改。Git 提交的 `hash` 仍保留。
- 用户作出的审批决定属于正常业务结果；审批服务异常等执行故障走工具错误处理。
- `finish_task` 和 `submit_review` 成功后结束各自流程，没有后续工具回复。

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

功能：更新全稿视觉方向、布局偏好或共享装饰的位置。未传入字段保持不变；`decorations` 仅合并传入的位置字段。仅在执行模式披露。

输入

- `direction`：全稿视觉方向描述。
- `layout_preferences`：布局偏好数组；传入时整体替换。
- `decorations`：共享装饰位置设置，可包含 `page_number`、`deck_title`、`section_title`、`key_message`。

以上字段至少传一个。

输出

- `content`：保存后的完整设计 JSON 对象。
- `changed_fields`：实际发生变化的字段数组；没有变化时为空数组。

## edit_spec

功能：更新指定页面的语义设计规格；首次创建页面规格时必须提供 `key_message` 和 `elements`。未传入字段保持不变。仅在执行模式披露。

输入

- `slide_id`：必填；页面稳定 ID。
- `key_message`：本页核心信息。
- `elements`：内容元素数组；元素包括 `type` 和 `intent`，传入时整体替换。
- `role`：页面语义角色；传 `null` 可移除已有可选角色。
- `layout`：页面布局描述；传 `null` 可移除已有可选布局。

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

输出

- `summary`：保存结果说明，例如“HTML 已保存。”；不回传整份 HTML。

保存成功不表示视觉检查通过。

## run_command

功能：在项目目录中运行受限命令，读取项目文件或 Git 状态；唯一写操作是在执行模式下对单个文件执行经确认的 `sed -i` 替换。编辑前需通过完整单文件 `cat`（PPT 资源也可通过 `read_resource`）取得当前内容；仅部分读取或内容已变化时需要重新读取。

输入

- `command`：必填；受限命令文本，最长 4096 字符。可用只读命令包括 `ls`、`cat`、`head`、`tail`、`find`、`grep`、`jq`、`rg`、`pwd`、`stat`、`sed -n`、`wc`、`git status`、`git diff`、`git log`。

输出

- `stdout`：已捕获的标准输出。
- `stderr`：已捕获的标准错误。
- `exit_code`：命令退出码。
- `output_truncated`：仅输出被截断时返回，固定为 `true`；未截断时省略。

输出超限时终止命令，返回明确的超限错误提示，并保留已捕获的输出、退出码和截断标志；不提供完整输出文件或分段读取接口。

需要用户审批的命令必须单独调用，审批期间挂起；批准后执行并返回执行结果，拒绝时通过本次工具调用返回拒绝错误，不生成虚假的命令输出。

## git_commit

功能：将当前项目允许纳入版本管理的源文件提交到本地 Git 仓库；不执行远程推送。仅在提交服务启用的执行模式披露。

输入

- `title`：必填；单行提交标题，最长 72 字符，建议使用 `feat`、`fix`、`refactor`、`perf`、`chore` 或 `docs` 类型前缀。
- `items`：必填；1 至 6 条提交说明，每条最长 160 字符，不加项目符号。

输出

- `summary`：提交结果说明；无可提交改动时仅返回此字段，例如“当前项目没有可提交的变更。”。
- `hash`：实际产生的 Git 提交标识；仅产生提交时返回。
- `branch`：实际提交所在分支；仅产生提交时返回。

## create_plan

功能：创建完整计划草案并提交用户审批；计划尚未获批时可反复调用，完整替换当前草案并重新提交审批。调用挂起等待用户决定，再返回审批结果。计划获批后不可通过此工具覆盖当前计划。

输入

- `title`：必填；计划标题。
- `content`：必填；完整 Markdown 计划正文。
- `steps`：必填、非空；计划步骤数组。
- `steps[].title`：必填；步骤标题。
- `steps[].target_slide_ids`：可选；该步骤关联的页面 ID 数组。

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
- `questions[].title`：必填；完整、明确的问题。
- `questions[].reason`：必填；为什么需要用户回答，以及答案会影响什么，避免空泛说明或重复问题正文。
- `questions[].options`：可选；最多三个预设选项，无预设选项时由用户填写答案。
- `questions[].options[].label`：必填；选项名称。
- `questions[].options[].description`：必填；选项的具体含义、影响或取舍，避免重复选项名称。
- `questions[].allow_custom`：可选；有预设选项时是否允许自定义回答；无预设选项时使用填空回答。

问题和选项 ID 由后端生成、维护，不由模型传入。

输出

- `answers`：按原问题顺序排列的问答数组。
- `answers[].question`：对应问题的 `title` 字符串。
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

必须作为最后一次响应中的唯一工具调用；纯文本不作为审查结果。
