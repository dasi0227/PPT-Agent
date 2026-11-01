# 下一步输入建议生命周期简化设计

- 日期：2026-09-23
- 替代范围：2026-09-14 下一步输入预测设计中的项目版本绑定、刷新校验、Run 版本字段及相关 QA。

## 产品决定

建议是用户下一条输入的参考文本，不是必须与项目当前内容保持一致的执行操作。用户选择后只形成普通草稿；实际发送时，Agent 根据当时的上下文理解和处理。

- Chat、Grill、Execute 成功完成时，通过现有 `finish.suggested_next_inputs` 生成零至三个候选，不增加 Model Call 或工具。
- 不校验项目 revision、资源 hash 或其它上下文标识，不自动刷新候选。
- 其他会话执行任务、手动修改项目、内容读取失败，都不使当前会话已有建议失效。
- 候选随 `message.final` 持久化；刷新、会话切换、Checkpoint 回退及恢复最新后，按对应事件历史恢复。
- 同一会话的新任务被服务端可靠接受后消费旧候选；新任务失败、取消或返回零候选，都不复活此前候选。
- 创建请求尚未被接受、创建失败或用户取消确认时，不消费旧候选。请求进行中暂时隐藏；失败后仍保留普通草稿规则。
- 输入草稿完全空白时展示；点击、Tab/Enter/空格、Option/Alt + 数字仅填入，不发送。清空草稿后重新显示仍存在的候选。

## 后端与协议

公共 Run 事件直接从 v5 升为 v6。`message.final` 保留 `message_id`、`text`、`affected_targets`、`suggested_next_inputs` 及公共事件头，删除 `project_history_revision`。空候选仍为 `[]`。

删除仅为候选校验增加的字段及传递链路：

- `Run.ProjectHistoryRevision`、SQLite Run 映射和 `runs.project_history_revision`。
- RunService 到 RuntimeInput、RunState、StructuredOutcome 的历史版本传递。
- Runtime 完成事件和 Engine fallback 中的版本字段及默认值。
- 前端事件类型、解析器及候选派生状态中的版本字段。

编号 SQL 变更 `0017_remove_suggestion_history_revision.sql` 删除废弃列；不改写已应用的历史 SQL。候选仍以事件 JSON 为唯一持久化来源，不新建候选表，不复制到 Composer 草稿或 scene。

项目历史自身的 revision、Baseline、启动屏障、回退确认和 journal 恢复流程保持原有职责。候选不再消费这些版本信息，因此无需为候选修补历史计数方式。

不保留旧事件协议兼容层，不迁移旧事件或旧开发 Checkpoint。v6 历史与 Checkpoint 正常恢复；旧开发历史遵循项目的一次性协议切换约定。

候选规范化、Prompt、Completion Gate、纯文本呈现与防止自动执行的约束继续沿用原设计。

## 前端生命周期

候选仅包含 `runId`、`messageId`、`items`、`status: staged | eligible`。

1. `message.final` 暂存候选；同一 Run 的 `run.completed` 激活候选。
2. 创建接口确认接受新 Run 时立即消费当前会话旧候选，无需等待 SSE；`run.started` 同样清除，支持其他入口与重连。
3. 历史 hydration 中，持久化 `user_turn` 已表明 Run 被接受，也应消费旧候选，覆盖尚未发出 started 就启动失败的场景。
4. 同一 Run 失败或取消清除其暂存候选；旧 Run 迟到事件不能覆盖新 Run。复用既有事件幂等和会话隔离机制。
5. Checkpoint 恢复之后，用恢复的事件序列重建候选。后续 Run 被回退掉后，之前成功 Run 的候选可再次出现；若恢复现场带有草稿或引用，则先隐藏。

Composer 删除 `verifiedSuggestionKey`、专门为建议发起的项目内容/history 查询以及版本匹配要求。候选可见性只由当前会话候选、运行/输入可用状态、完全空白的草稿和临时 Escape 隐藏状态派生，不依赖项目内容加载成功。

建议不会跨会话复制。同一项目会话 B 的新请求只消费 B 的候选，A 的候选继续保留。项目/会话切换继续重置本地隐藏状态，防止填入其它会话的候选。

### Placeholder 呈现与焦点

建议与 contenteditable 位于同一网格单元，建议层按完整换行内容撑开输入区，真实编辑器从同一左上起点开始，不在列表下方追加空输入行。建议文字不进入可编辑 DOM。只有候选按钮接收点击，标题及空白区域透传到编辑器。

建议出现时，若焦点停留在页面 body、没有选中文本及打开的弹窗/菜单，则聚焦真实编辑器并显示光标；不抢占其它控件焦点。普通输入、粘贴及中文输入法 composition 开始后隐藏建议；删除全部文字或取消空白组词后重新展示，编辑器始终保持同一 DOM 节点和原焦点。点击、Tab 选择及 Option/Alt 快捷填入继续沿用原行为。

## 验证重点

- Runtime/Engine 发布规范化候选，v6 解析与数据库 Run 写入不需要历史版本字段。
- Composer 在 history 缺失/失败、revision 改变、内容加载失败及其它会话完成任务时，仍展示自己的候选。
- 点击只写草稿，清空后恢复；会话切换展示各自候选；恢复的事件历史能重建候选。
- HTTP 接受后、SSE 到达前已消费；未接受的请求不消费；已接受但启动失败的历史不复活旧候选。
- 沿用既有快捷键、事件顺序、迟到事件、失败终态及历史恢复测试，不做真实浏览器自动验收。

## 实施验证记录

- 后端 `migrations`、`model`、`store/sqlite`、`run`、`workflow`、`service`、`projecthistory` 七个相关包测试通过。
- 前端 SSE、Run Store、多会话、事件归约、历史 hydration、快捷键、候选按钮和真实 Composer 组件共 8 个测试文件、85 项测试通过。
- `pnpm tsc --noEmit`、改动源码及新增 Composer 测试的 ESLint 检查、`git diff --check` 通过。
- 未启动真实浏览器，也未对本地运行数据库或旧 Checkpoint 执行转换。

## 本轮确认 QA

| 问题 | 最新决定 |
| --- | --- |
| 建议需要始终匹配项目最新状态吗？ | 不需要，仅作为输入参考。 |
| 项目被其他会话或用户手动修改怎么办？ | 保留原建议，不校验版本、不重新生成。 |
| Checkpoint 已保存候选是否恢复展示？ | 恢复，按对应历史事件重建。 |
| 新任务失败后旧建议是否回来？ | 已被接受的新任务会消费旧建议，失败或取消不复活。 |
| 选择建议会执行任务吗？ | 不会，只填入草稿；清空后可以重新展示。 |
