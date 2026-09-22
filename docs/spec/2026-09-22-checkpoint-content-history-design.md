# Checkpoint 内容历史统一设计

## 回退确认

前端以 [精简确认原型](../demo/2026-09-22-checkpoint-rollback-refined-demo.html) 为准，沿用公共 Dialog、Button。弹窗宽 460px，目标时间和输入摘要置于白底细线分隔区；摘要最多三行，超出省略，不提供展开操作。撤回任务数量放在底部操作区左侧，仍包含目标任务，但不展示「含本次」。说明缩短为「项目和所有对话将一并回退。」，删除草稿替换与恢复最新的提示文字；对应业务行为保持不变。保留确认、取消、执行中禁用和错误重试。

`history/preview` 只返回 `revision`、`time`、`input`、`runs`，删除文件变更清单和会话数量及其计算。任务数量以当前与目标数据库中的 Run 身份差异计算；连续回退只计算当前仍存在的任务。恢复最新沿用同一预览协议，`input` 取首次回退前保存现场中当前会话的草稿，不取回退任务输入或当前编辑草稿。两类弹窗均展示输入摘要，空白输入显示「（暂无输入）」。

## 历史与当前元数据

- 移除 `GET /slides/:id/versions`、`POST /slides/:id/rollback`、版本模型及存储读写接口；当前 HTML 预览继续使用原端点。
- 工具提交不再复制 HTML、spec、outline、manifest、design 到 `artifacts/versions`，也不再写入旧 `versions` 表。编号 SQL 变更直接移除该表，不回填历史数据。
- manifest、outline、design、spec 和 materialization 采用[内容指纹与计划审批简化设计](2026-09-22-content-hash-and-plan-approval-design.md)，内容身份由 hash 表达，渲染证明继续保留。
- 删除 `slides.current_version`；HTML 缓存与 DOM 引用使用实际文件 hash。
- `artifact_commit` receipt 继续以 `run_id + operation_id` 去重，并与页面元数据、工具结果在同一 SQLite 事务中保存。检查 receipt 先于任何元数据写入：重复操作直接返回，冲突摘要拒绝，迟到重放不得覆盖后续页面元数据或成员。
- 新增 `deleted_slides(project_id, slide_id)` 保存已删除页面身份。删除前从真实页面记录写入；重建时清除。覆盖工具提交和大纲成员同步，也支持从未生成 HTML 的页面；DOM 引用验证按项目查询该元数据。
- 简报版本及其它仍承担业务语义的版本字段保持原有能力。

## Checkpoint 边界

项目快照清单由 `versions` 切换为 `deleted_slides`，页面身份和提交 receipt 继续纳入同一快照，确保回退、连续回退、恢复最新与丢弃后续历史时共同恢复。

`versions` 目录同时退出文件捕获、恢复删除和目录同步范围。遗留目录不进入新 checkpoint，也不会被回退操作删除或重建。不迁移旧 checkpoint、不保留双协议、不扫描清理本地项目数据。

## 验证

- 工作流提交覆盖五类内容，无历史副本生成；内容 hash 标识实际变化，相同内容不重复失效，迟到重放不覆盖新状态。
- 删除身份元数据覆盖失败回滚、项目隔离、未渲染页面、重建与重复提交。
- 完整项目测试覆盖连续回退、目标包含计数、唯一最新现场、页面身份与删除记录、提交凭据恢复及遗留版本目录保留。
- 沿用项目写门禁、恢复失败补偿、崩溃恢复和丢弃旧未来测试；前端检查预览、取消、Escape 和草稿恢复。
- 不使用浏览器自动验证；视觉效果与手动交互由用户验收。
