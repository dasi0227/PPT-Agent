# 工具执行前拒绝的事件生命周期修复

## 问题

2026-09-27 17:42:20，`edit_spec` 的 `role` 参数不符合枚举约束，参数预检返回 `TOOL_ARGUMENT_INVALID`。调用尚未执行，没有发送 `tool.started`，但 Runtime 仍发送 `tool.completed(status=failed)`。事件总线因缺少开始事件而拒绝该终态，上层将其误记为 `journal_write_failed` 并暂停任务。调度器退出时再次暂停同一任务，产生 `run: not running` 次生错误。

## 事件约定

延续 `2026-08-29-run-command-security-and-ui-design.md` 对执行前拒绝的约定，并统一应用于业务工具：

- 参数预检失败、策略拒绝、审批失败或用户拒绝等执行前失败：只发送 `tool.completed(status=blocked)`，附带错误，不发送 `tool.started`。
- 工具实际开始执行：发送 `tool.started`，随后以 `completed` 或 `failed` 结束，严格匹配相同的调用 ID 和工具名。
- `blocked` 必须附带错误；命令不得携带退出码或执行耗时，避免暗示已经执行。
- 独立的 `blocked` 终态占用调用 ID，禁止再次开始或再次结束；历史事件恢复后保持相同约束。

Agent 仍接收原有结构化工具错误及修正信息。公开事件的合法性不应阻断 Agent 修正参数后的继续执行。

## 错误分类与暂停

- 公开事件载荷、身份或顺序错误标记为 `event_protocol_invalid`。
- 事件持久化失败保留 `journal_write_failed`。
- 调度器记录原始事件投递错误，避免只显示泛化的暂停原因。
- 同一事件错误的暂停成功后不再重复暂停；首次暂停写入失败时，允许调度器退出阶段重试。

## 同类路径修复

原审批批次逻辑用一个索引表示待审批命令，多条待审批命令会覆盖索引，使拒绝分支失效。改为计数：响应只要包含待审批命令且工具调用数大于一，就整批拒绝并提示单独提交。

## 验证

- 真实 Runtime → workflowEmitter → Bus 回归覆盖：非法 `edit_spec.role` 后修正并完成、缺少页面 ID 的读取、命令缺参及类型错误、命令策略拒绝、同批两条或三条待审批命令。
- 验证拒绝反馈到达 Agent、不触发暂停或上下文取消、无错误的开始事件、命令没有虚构执行信息，且事件历史可以恢复。
- 验证事件协议错误与存储错误分别记录正确的暂停原因，暂停成功不重复写入，暂停失败可以重试。
- `internal/run` 全部测试通过；新增回归的 race 检测通过；后端所有包编译检查通过。
- 相关 workflow/model 筛选回归中，`TestSteeringWaitsUntilCompleteToolBatchObservation` 仍失败；用 HEAD 版本的 workflow 文件进行 Go overlay 对照后，同一测试同样失败，确认为本次修改前已存在的消息顺序断言问题。

修改不重写已暂停任务的数据；重启后端后可由用户继续任务。
