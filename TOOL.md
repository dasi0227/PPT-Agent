# 模型可见工具

本文记录当前代码向 LLM 披露的工具输入，以及执行后写入 LLM 对话的输出。工具会按模式、阶段、项目配置和目录状态动态披露；同一轮不会出现下文所有工具。以下字段以**现有实现**为准。

除特别说明外，以下“输出”指成功时的工具回复。常规失败会向模型返回结构化错误：`ok: false`、`code`、`category`、`reason`、`retryable`、`next_action`，并可能包含 `call_id`、`resource`、`operation` 或问题详情。失败时不应把成功字段当作已返回。`finish` 和 `submit_review` 成功后会终止相应流程，没有后续工具回复。

## read_resource

功能：读取演示文稿的全局资源或指定页面资源。

输入
- `resource`：资源类型，取 `manifest`、`design`、`outline`、`spec`、`html` 之一。
- `slide_id`：页面稳定 ID；读取 `spec` 或 `html` 时必填，其他资源禁止传入。

输出
- `ok`：读取是否成功。
- `resource`：实际读取的资源类型。
- `slide_id`：页面资源对应的页面 ID；仅页面资源返回。
- `content`：资源内容；`manifest`、`design`、`spec` 为 JSON 对象，`outline`、`html` 为原始文本。
- `content_hash`：资源内容的 SHA-256 版本标识。

## render_slide

功能：在隔离 Chromium 中渲染指定页面，返回文字诊断和最新图片引用，不直接返回图片像素。仅在执行模式披露。

输入
- `slide_id`：要渲染的页面稳定 ID，且必须在本次运行授权范围内。

输出
- `ok`：无阻断问题时为 `true`。
- `tool_call_id`：本次工具调用 ID。
- `resource`：被渲染的页面 HTML 资源标识，包含 `type`、`slide_id`、`part`。
- `slide_id`：渲染的页面 ID。
- `image_path`：最新渲染图片引用，可传给 `read_image` 进行视觉检查。
- `source_hash`：本次渲染所用 HTML 源码哈希。
- `diagnostics`：诊断对象，包含 `content_size`（`width`、`height`）、`overflow`（`horizontal`、`vertical`）、`clipping`、`runtime_decorations`、`console_errors`、`failed_resources`、`font_status`。
- `visual_inspection`：提示本次输出不含图片像素，视觉检查需调用 `read_image`。

发现阻断问题时，此工具按失败处理，模型收到统一错误及修正信息。

## read_image

功能：读取用户上传的图片附件，或读取指定页面的最新渲染图像；图像内容会以图片形式提供给模型。

输入
- `attachment_id`：上传附件 ID；读取附件时必填。
- `variant`：附件图像版本，`thumbnail` 或 `original`，默认 `thumbnail`；仅用于附件。
- `image_path`：`render_slide` 返回的最新渲染图片引用；读取渲染图时必填，且不能与附件参数同时使用。

输出
- `image`：实际图像内容块；附件为选定版本，渲染图为 PNG。它不是 JSON 字段。
- `attachment_id`、`name`、`media_type`、`width`、`height`、`variant`、`original_path`：读取附件时，包含在 `<image_attachment>` 文字块中的字段；宽高取自原图元数据。
- `slide_id`、`image_path`、`source_hash`、`rendered_at`、`stale`：读取渲染图时，包含在 `<rendered_image>` 文字块中的字段；`stale` 表示当前渲染依据是否已过期。

## load_component

功能：按稳定 ID 批量加载已启用的组件 HTML 供参考和改编，不会直接写入或修改项目文件。单次最多 8 个组件，总 HTML 不超过 192 KiB。

输入
- `ids`：唯一组件 ID 数组，至少 1 个、最多 8 个。

输出
- `loaded`：本次新加载的组件数量。
- `components`：新加载组件内容，每项包含 `id`、`name`、`description`、`html`。
- `already_available`：此前已在当前上下文中可用、无需重复传输的组件 ID。
- `replaces_previous`：当前实现固定返回 `true`；此前加载的组件仍可在当前上下文中使用。

## load_skill

功能：按 ID 将已启用的仓库技能加入当前运行的活动技能上下文。单次最多 8 个技能。

输入
- `ids`：唯一技能 ID 数组，至少 1 个、最多 8 个。

输出
- `loaded`：本次新加载的技能数量。
- `skills`：新加载技能内容，每项包含 `id`、`name`、`description`、`content`。
- `already_available`：此前已在当前上下文中可用、无需重复传输的技能 ID。
- `replaces_previous`：当前实现固定返回 `true`；此前加载的技能仍可在当前上下文中使用。

## edit_manifest

功能：更新演示文稿需求清单中传入的顶层字段；未传入字段保持不变。仅在执行模式披露。

输入
- `title`：演示文稿标题。
- `language`：演示文稿语言标识。
- `pages`：期望页数或页数范围。
- `audience`：目标受众及其背景。
- `goal`：演示文稿希望受众理解、决定或做到的事项。
- `requirements`：必须覆盖的内容、证据或表述要求；传入数组整体替换原数组。
- `prohibitions`：明确排除的内容；传入数组整体替换原数组。
- `expected_hash`：可选；编辑已有内容时使用的当前版本哈希，用于冲突检测。

以上业务字段至少传一个。

输出
- `ok`：保存是否成功。
- `changed`：内容是否发生变化。
- `content_hash`：保存后的版本哈希。
- `manifest`：保存后的完整需求清单对象。
- `changed_fields`：发生变化的字段。

## edit_design

功能：更新全稿视觉方向、布局偏好或共享装饰的位置。未传入字段保持不变；`decorations` 仅合并传入的位置字段。仅在执行模式披露。

输入
- `direction`：全稿视觉方向描述。
- `layout_preferences`：布局偏好数组；传入时整体替换。
- `decorations`：共享装饰位置设置，可包含 `page_number`、`deck_title`、`section_title`、`key_message`。
- `expected_hash`：可选；编辑已有内容时使用的当前版本哈希，用于冲突检测。

以上业务字段至少传一个。

输出
- `ok`：保存是否成功。
- `changed`：内容是否发生变化。
- `content_hash`：保存后的版本哈希。
- `design`：保存后的完整设计对象。
- `changed_fields`：发生变化的字段。

## edit_spec

功能：更新指定页面的语义设计规格；首次创建页面规格时必须提供 `key_message` 和 `elements`。未传入字段保持不变。仅在执行模式披露。

输入
- `slide_id`：页面稳定 ID。
- `key_message`：本页核心信息。
- `elements`：内容元素数组；元素包括 `type` 和 `intent`，传入数组整体替换。
- `role`：页面语义角色；传 `null` 可移除已有可选角色。
- `layout`：页面布局描述；传 `null` 可移除已有可选布局。
- `expected_hash`：可选；编辑已有内容时使用的当前版本哈希，用于冲突检测。

除 `slide_id` 和 `expected_hash` 外，至少传一个业务字段。

输出
- `ok`：保存是否成功。
- `changed`：内容是否发生变化。
- `content_hash`：保存后的版本哈希。
- `slide_id`：页面 ID。
- `spec`：保存后的完整页面规格对象。
- `changed_fields`：发生变化的字段。

## init_outline

功能：从完整 JSON 文本初始化尚不存在的演示文稿大纲；运行时为大纲节点和页面分配稳定 ID，不覆盖已有大纲。仅在执行模式、大纲不存在时披露。

输入
- `content`：完整大纲 JSON 文本，包含章节、子章节及页面标题；新建时不提供运行时管理的 ID。

输出
- `ok`：初始化是否成功。
- `changed`：内容是否发生变化。
- `content_hash`：保存后的版本哈希。
- `content`：含运行时分配 ID 的完整已保存大纲 JSON 文本。

## arrange_outline

功能：对已有大纲 JSON 源码按顺序执行精确文本替换；每个 `old_text` 必须恰好匹配一次。删除页面会一并删除其规格和 HTML，相关改动原子保存。仅在执行模式、大纲已存在时披露。

输入
- `edits`：必填、非空的有序替换数组，每项包含非空 `old_text` 和 `new_text`。
- `expected_hash`：可选；已有大纲源码的预期哈希，用于冲突检测。

输出
- `ok`：修改是否成功。
- `changed`：内容是否发生变化。
- `content_hash`：保存后的版本哈希。
- `content`：完整已保存大纲 JSON 文本。

## write_html

功能：创建或完整替换指定已有页面的 HTML 源码；保存后需单独调用 `render_slide` 检查外观。仅在执行模式披露。

输入
- `slide_id`：页面稳定 ID。
- `html`：完整页面 HTML 源码。
- `expected_hash`：可选；已有 HTML 的预期哈希，用于冲突检测。

输出
- `ok`：外层成功标志，固定为 `true`。
- `summary`：当前为 `resource saved`。
- `data.ok`：内层成功标志，当前也固定为 `true`。
- `data.changed`：本次是否写入变更。
- `data.content_hash`：保存后的 HTML 源码哈希。
- `data.slide_id`：页面 ID。
- `changed_targets`：仅发生变更时返回；每项包含 `resource`（页面 HTML 资源标识）和 `artifact_hash`。

## patch_html

功能：对指定页面 HTML 按顺序执行精确文本替换；每个 `old_text` 必须恰好匹配一次，全部替换原子保存。仅在执行模式披露。

输入
- `slide_id`：页面稳定 ID。
- `edits`：必填、非空的有序替换数组，每项包含非空 `old_text` 和 `new_text`。
- `expected_hash`：可选；已有 HTML 的预期哈希，用于冲突检测。

输出
- `ok`：外层成功标志，固定为 `true`。
- `summary`：当前为 `resource saved`。
- `data.ok`：内层成功标志，当前也固定为 `true`。
- `data.changed`：本次是否写入变更。
- `data.content_hash`：保存后的 HTML 源码哈希。
- `data.slide_id`：页面 ID。
- `changed_targets`：仅发生变更时返回；每项包含 `resource`（页面 HTML 资源标识）和 `artifact_hash`。

## run_command

功能：在项目目录中运行受限命令，读取项目文件或 Git 状态；唯一写操作是在执行模式下对单个文件执行经确认的 `sed -i` 替换。

输入
- `command`：受限命令文本，最长 4096 字符。可用只读命令包括 `ls`、`cat`、`head`、`tail`、`find`、`grep`、`jq`、`rg`、`pwd`、`stat`、`sed -n`、`wc`、`git status`、`git diff`、`git log`。

输出
- `ok`：外层成功标志，固定为 `true`。
- `summary`：当前为 `command completed`。
- `data.stdout`：标准输出。
- `data.stderr`：标准错误。
- `data.exit_code`：命令退出码。
- `data.duration_ms`：执行耗时（毫秒）。
- `data.output_truncated`：输出是否被截断。
- `changed_targets`：仅写入时返回；每项只包含 `resource`（文件的 `type`、`part`、`path`）和 `artifact_hash`。

## git_commit

功能：将当前项目允许纳入版本管理的源文件提交到本地 Git 仓库；不执行远程推送。仅在提交服务启用的执行模式披露。

输入
- `title`：单行提交标题，最长 72 字符，建议使用 `feat`、`fix`、`refactor`、`perf`、`chore` 或 `docs` 类型前缀。
- `items`：1 至 6 条提交说明，每条最长 160 字符，不加项目符号。

输出
- `ok`：外层成功标志，固定为 `true`。
- `summary`：提交成功或无可提交改动的简短说明。
- `data.empty`：仅无可提交改动时为 `true`；此时不返回提交详情。
- `data.title`、`data.items`、`data.branch`、`data.hash`、`data.committed_at`、`data.files_changed`、`data.insertions`、`data.deletions`：产生提交时返回的提交详情。
- `data.model_profile`、`data.fallback_used`：产生提交时返回；当前工具由 Agent 提交标题和说明，两者分别为空字符串和 `false`。


## create_plan

功能：创建完整计划。规划模式下提交待用户批准的计划；执行模式下创建无需批准的执行清单。

输入
- `title`：必填；计划标题。
- `content`：必填；完整 Markdown 计划。
- `steps`：必填、非空；每项必须含 `title`，可含 `target_slide_ids` 页面 ID 数组。

输出
- `ok`：固定为 `true`。
- `summary`：执行模式为 `execution checklist created`；规划模式为 `plan proposal created; waiting for approval`。
- `data`：空对象；新计划本身不包含在该工具回复中。

## update_plan

功能：规划模式下整体替换待批准计划；执行模式下更新现有步骤状态。同一工具在两种状态下向 LLM 披露不同输入 Schema。

输入
- `title`、`content`、`steps`：仅规划模式使用，格式与 `create_plan` 相同，提交完整替代计划。
- `updates`：仅执行模式使用，必填、非空；每项包含现有 `step_id` 和 `status`，状态为 `pending`、`in_progress`、`completed` 或 `failed`。

输出
- `ok`：固定为 `true`。
- `summary`：规划模式为 `plan revision saved; waiting for approval`；执行模式为 `plan progress accepted`。
- `data`：空对象；更新后的计划不包含在该工具回复中。

## request_privilege

功能：请求用户扩展可编辑页面范围；仅在执行模式披露，必须单独调用。

输入
- `add_slide_ids`：必填、非空；希望加入授权范围的页面 ID 数组。
- `reason`：必填；申请扩展的原因。

输出
- `ok`：固定为 `true`。
- `summary`：用户已回应授权请求，或请求的页面已包含在当前范围内。
- `data.decision`：实际发起用户审批时的决定；已在范围内时不返回。
- `data.scope`：用户回应后的当前页面授权范围；已在范围内时也返回。包含 `slide_ids`、`source`（`kind`，可能还有 `section_ids`）和 `include_run_created_slides`。
- `data.changed`：仅请求页面已在范围内时返回 `false`。

## ask_user

功能：提出一组阻塞性问题，并在用户回答后继续当前运行。

输入
- `questions`：必填、非空的问题数组；每项必须含 `id`、`title`，可含 `description`、`options`（最多三个）、`allow_custom`。
- `questions[].options[]`：选项必须含 `id`、`label`，可含 `description`；无选项的问题由用户填写答案。

输出
- `ok`：用户回答后固定为 `true`。
- `summary`：当前为 `user answered`。
- `data.answers`：答案数组，每项包含 `question_id`，以及所选 `selected_option_id` 或填写的 `custom_text`。
- `data.display_text`：用户答案的显示文本。

## review_task

功能：在执行模式中委派独立的 PPT 成果审查，不审查计划或最终回复，不修改业务文件，也不自动完成主任务。

输入
- `demand`：必填非空字符串，说明待审查的成果、范围、重点和适用要求，不能覆盖用户指令。

输出
- `type`：`approve`（审查通过）、`check`（需要核实）、`refuse`（拒绝交付）。
- `reasons`：非空字符串数组，三种结论都必须给出原因。

审查执行失败时返回统一错误，不伪装为 `check` 或 `refuse`。

## finish

功能：提交完整的最终回复。可在聊天、追问或执行模式的相应阶段披露，必须单独调用。

输入
- `message`：必填；给用户的完整最终回答。
- `suggested_next_inputs`：可选；最多三条、每条不超过 80 字符的后续输入建议。

输出
- 成功：无工具回复；Runtime 直接完成当前运行并将 `message` 用作最终回答。
- 被完成条件拦截：返回包含 `ok: false`、`code`、`category`、`reason`、`retryable`、`next_action` 及可能的 `issues` 的错误回复，供 LLM 修正。

## submit_review

功能：Reviewer 用于提交最终成果审查结论并结束独立审查循环；主 Agent 不会收到这个工具的 Schema。

输入
- `type`：必填，`approve` / `check` / `refuse`。
- `reasons`：必填，至少一个非空字符串。说明批准依据、待核实事项或已确认的问题；无需对象化。

输出
- 成功：无工具回复；Reviewer 循环结束，主 Agent 随后通过 `review_task` 收到 `type` 和 `reasons`。

必须作为最后一次响应中的唯一工具调用；纯文本不作为审查结果。
