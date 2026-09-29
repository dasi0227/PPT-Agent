# 已确认下来的更改需求

- [ ] 将资源版本管理收回后端：去除所有模型可见的 `content_hash` 输出（包括嵌套字段），以及所有工具 Schema 中由模型传递的 `expected_hash` 输入。
  - 后端记录当前运行中实际提供给模型的资源版本，编辑时自动使用对应版本进行冲突检查；不能用写入前读取的最新版本替代模型所见版本。
  - 保留后端版本校验，并保证校验与写入之间不会被其他写操作插入；冲突时向模型返回重新读取后修改的提示。
  - 实现完成后同步更新 `TOOL.md` 中的模型可见输入和输出。

- [ ] 去除所有模型可见工具输出中的 `ok`，包括顶层、嵌套字段，以及成功和失败返回中的该字段。
  - 正常返回即表示工具执行成功；执行失败时返回明确的错误提示消息，无需额外传递 `ok`。
  - 后端内部的执行状态管理保留；本项仅调整模型可见输出，并在实现后同步更新 `TOOL.md`。

- [ ] 删除 `render_slide` 模型可见输出中的 `tool_call_id`、`resource` 和 `source_hash`。
  - 工具调用关联由协议层维护；后端保留资源定位、源版本和渲染有效性校验所需的信息。
  - `source_hash` 按前述版本信息由后端管理的原则处理；实现后同步更新 `TOOL.md`。

- [ ] 删除 `render_slide` 输出中的 `visual_inspection`，改为直接返回截图内容块和诊断信息，不再向模型返回 `image_path`。
  - 渲染返回的截图接入现有图片保留、上下文恢复和去重机制。
  - 保留 `read_image`，用于读取已有截图或上传附件；页面内容变化后仍需重新渲染才能得到新截图。
  - 实现后同步更新工具描述及 `TOOL.md`。

- [ ] 将图片定位交给后端，简化 `read_image` 的模型可见输入和输出。
  - 后端维护“项目＋`slide_id` → 最新截图”的映射，截图路径作为内部信息管理。
  - `read_image` 输入改为 `attachment_id` 或 `slide_id`，必须且只能提供其中一个；移除 `image_path` 和 `variant` 输入，上传附件统一从原图读取，必要的格式转换和压缩由后端适配层处理。
  - `read_image` 成功时只返回图片内容块，移除现有 `<image_attachment>`、`<rendered_image>` 文字块中的整组元信息；失败时返回明确的错误提示。
  - 按 `slide_id` 读取时，若截图不存在或已过期，提示先调用 `render_slide`，避免把旧截图当作当前页面。
  - 图片保留、上下文压缩与恢复过程中，由 Runtime 保持图片与原调用及页面或附件的关联。
  - 将 HTML 嵌入所需的附件引用地址移到附件上下文统一提供，保证移除 `original_path` 工具输出后仍能正确引用上传图片。
  - 实现后同步更新相关工具描述、提示词及 `TOOL.md`。

- [ ] 简化 `load_component` 和 `load_skill` 的模型可见输出，加载正文统一使用 `id` 和 `content`。
  - 删除 `replaces_previous`、`loaded`，以及每个正文项中的 `name`、`description`；组件正文的 `html` 字段改为 `content`，技能正文继续使用 `content`。
  - 保留 `components` / `skills` 数组及 `already_available`，后者列出正文已在当前模型上下文中、无需重复返回的资源 ID。
  - `name`、`description` 等选择资源所需的信息继续由预置资源列表提供；加载正文、预置正文及上下文恢复正文统一采用 `id + content`。
  - 保留后端资源快照和去重机制，确保上下文压缩后能恢复必要的资源信息；实现后同步更新相关工具描述、提示词及 `TOOL.md`。

- [ ] 统一 `edit_manifest`、`edit_design`、`edit_spec` 的模型可见输出。
  - 删除 `changed`，保留 `changed_fields` 表达实际变更字段；没有内容变化时返回空列表。
  - 将 `manifest`、`design`、`spec` 输出字段统一改为 `content`，返回保存后的完整 JSON 对象。
  - 删除 `edit_spec` 模型可见输出中的 `slide_id`；由工具调用关联及后端元数据维护结果的页面归属，并在上下文压缩与恢复时保留该关联。
  - 实现后同步更新相关工具描述、提示词及 `TOOL.md`。

- [ ] 将 `init_outline` 和 `arrange_outline` 合并为 `edit_outline`，不保留旧工具别名。
  - 输入包含互斥的 `init` 和 `edits`，必须且只能提供一个：`init` 为完整初始大纲 JSON 对象，`edits` 为非空的 `old_text` / `new_text` 有序替换数组。
  - `init` 仅允许大纲不存在时使用；已有大纲即使为空也不能重新初始化。`edits` 仅允许编辑已有大纲，每个 `old_text` 必须恰好匹配一次。
  - 新节点省略 ID，由后端沿用现有生成规则分配章节、子章节和页面的稳定 ID；已有节点编辑、改名或移动时保留 ID。
  - 保留结构校验、页面删除时关联清理 Spec / HTML，以及全部相关改动的原子保存。
  - 成功后通过 `content` 返回实际保存的完整 JSON 源码字符串，包含后端生成的 ID，可直接用于后续精确文本编辑。
  - 同步调整工具注册、能力分组、调用分发、错误提示和相关提示词，并在实现后更新 `TOOL.md`。

- [ ] 将 `write_html` 和 `patch_html` 合并为 `edit_html`，统一输入和精简输出，不保留旧工具别名。
  - 输入必须包含 `slide_id`，并在 `content` 和 `edits` 中必须且只能提供一个，无需额外的 `mode` 参数。
  - `content` 表示完整 HTML 源码，替代原 `html` 字段；目标页面 HTML 不存在时创建，存在时完整替换。
  - `edits` 为非空的 `old_text` / `new_text` 有序替换数组，仅用于已有 HTML；每个 `old_text` 必须非空且恰好匹配一次，全部修改原子保存。
  - 成功输出仅保留 `summary`，例如 `{"summary":"HTML 已保存。"}`；删除 `data` 包装及其中的字段、`changed_targets` 等其他成功输出，不回传整份 HTML。
  - 保存相同内容也视为成功；失败时返回具体错误原因及必要的修正提示，文本匹配失败应指出对应替换项和匹配问题。
  - 后端继续维护改动记录、页面状态和截图失效信息；保存成功不表示视觉检查通过。
  - 同步调整工具注册、能力分组、调用分发、工具描述及相关提示词，并在实现后更新 `TOOL.md`。

- [ ] 精简 `run_command` 的模型可见输出。
  - 普通结果直接返回 `stdout`、`stderr` 和 `exit_code`，移除 `data` 包装、固定的 `summary`、`duration_ms` 和 `changed_targets` 等冗余输出。
  - 输出超限时额外返回 `output_truncated: true`，并保留已捕获的输出、退出码和明确的超限错误提示；未截断时不返回该布尔字段。
  - 暂时沿用现有输出上限和超限终止策略，不增加完整输出落盘及分段读取机制；后端继续维护必要的执行统计和变更记录。
  - 实现后同步更新工具描述、相关提示词及 `TOOL.md`。

- [ ] 精简 `git_commit` 的模型可见输出。
  - 产生提交时直接返回 `summary`、`hash` 和 `branch`，分别说明提交结果、Git 提交标识和实际提交分支。
  - 没有可提交改动时仅返回 `summary`，例如 `{"summary":"当前项目没有可提交的变更。"}`，不返回 `empty` 或提交详情。
  - 移除 `data` 包装，以及 `title`、`items`、`model_profile`、`fallback_used`、`committed_at`、`files_changed`、`insertions`、`deletions` 等模型输出字段；后端按需保留记录。
  - 保留的 `hash` 用于定位 Git 提交，不属于此前移除的资源并发校验哈希。
  - 失败时返回具体错误原因及必要的修正提示；实现后同步更新工具描述、相关提示词及 `TOOL.md`。

- [ ] 收窄计划工具职责，并精简模型可见输出。
  - `create_plan` 负责首次创建计划，并允许在计划尚未获批时反复调用，以完整替换当前草案的标题、正文和步骤，重新提交用户审批；保留审批前的草案修订能力。
  - `update_plan` 仅通过 `updates` 更新当前计划中已有步骤的状态，每项包含 `step_id` 和 `status`，无需模型传递 `plan_id`；删除其 `title`、`content`、`steps` 输入及整份草案修订分支，审批前修订统一使用 `create_plan`。
  - 计划获批后不允许通过 `create_plan` 覆盖当前计划，也不提供执行途中修改计划正文或步骤结构的能力；执行期间只通过 `update_plan` 更新步骤状态。
  - 将计划步骤状态 `in_progress` 统一改为 `processing`；可用状态为 `pending`、`processing`、`completed`、`failed`，继续限制同时至多一个步骤处于进行中，已完成步骤不得回退。
  - `create_plan` 和 `update_plan` 的成功输出仅保留 `summary`，去除空的 `data`。
  - 前后端、工具 Schema、状态校验、上下文及恢复逻辑一次性切换到新步骤状态，不保留旧值兼容分支；同步更新工具描述、提示词及 `TOOL.md`。
