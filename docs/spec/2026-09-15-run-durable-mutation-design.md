# Run 工具级持久化设计

> 后续决定：[HTML 生成参考快照与变化上下文](2026-09-25-html-generation-reference-snapshots-design.md) 将生成参考快照纳入 mutation journal、请求摘要、SQLite 提交与 receipt；渲染只产生独立证据，不写 materialization 文件。

> 2026-09-22 更新：内容历史统一由项目 checkpoints 承担；旧单页版本、确认弹窗与元数据的最新约定见 [Checkpoint 内容历史统一设计](2026-09-22-checkpoint-content-history-design.md)。

## 背景

当前执行模式把所有 PPT 与项目文件修改保存在 `RunSession` 内存 overlay 中，只有 Completion Gate 接受最终回复后才一次性写入项目目录。这个模型会造成三个直接问题：

- 成功工具调用产生的目录和文件对后续 `run_command` 不可见。
- Run 失败或被用户取消时，已经完成且消耗大量模型 Token 的成果会被整体丢弃。
- Runtime checkpoint 虽然携带 overlay 快照，但终止后的项目内容仍回到 Run 前状态，和用户对 Agent 工作区的预期不一致。

本设计以最新用户决定替代整 Run 原子 staging：执行模式按成功工具调用持久化，取消只停止后续工作，不自动撤销已经成功的修改。

## 核心语义

1. 每个会修改项目的工具调用是一个持久化事务边界。
2. 工具只有在文件和数据库元数据均成功提交后，才对 Runtime 返回成功并发布 `tool.completed`。
3. 同一工具调用涉及的多个文件要么全部生效，要么全部回到调用前状态。
4. 已成功工具调用的结果立即出现在项目正式目录，结构化读取与 shell 命令读取同一份磁盘状态。
5. Run 失败或取消只丢弃正在执行但尚未提交的工具调用；此前成功调用的结果继续保留。
6. Completion Gate 只判断任务是否达到交付标准，不再决定修改是否落盘。
7. Run 开始前的 Project Checkpoint 继续作为用户主动回退整次任务的基线；Runtime checkpoint 只负责续跑控制状态和记录已提交变更。

## 提交协议

每次写工具调用使用稳定的 `run_id + call_id` 作为操作身份：

1. 工具先在当前 `RunSession` 中构造候选文件和变更集。
2. Runtime 为候选内容写入持久化 mutation journal，journal 包含操作身份、请求摘要、文件前像、文件后像和 Hash。
3. RunSession 使用同目录临时文件、`fsync` 与原子 rename 应用正式文件。
4. 元数据提交在 SQLite 事务内同步页面身份、版本和工具调用结果，并写入同一操作的持久化 receipt。
5. 提交成功后删除 journal，并清空本次工具调用的候选状态；下一次写工具调用使用新的空事务。
6. 任一步骤返回错误时恢复文件前像，工具调用返回失败。

启动或新 Run 使用项目前必须恢复遗留 journal：存在匹配的已完成 receipt 时重放后像，否则恢复前像。这样可以区分“数据库已经提交但进程尚未来得及清理 journal”和“文件只写了一半、数据库尚未提交”两种崩溃窗口。

## 增量项目状态

- `outline.init` 或结构写入成功后立即同步正式 slide identity；允许页面 spec/HTML 处于 pending。
- spec 写入成功后立即创建该次工具调用对应的版本。
- HTML 写入成功后立即创建 HTML 版本；没有新渲染证明时允许 materialization 处于 stale/unknown。
- `render_slide` 成功后单独持久化 materialization proof，不要求等待 Run 完成。
- 同一 Run 多次修改同一目标时，每个不同 `call_id` 产生独立版本；同一 `call_id` 重放不得重复创建版本。

## 取消、失败与恢复

- 取消发生在工具提交前：回滚该工具调用，保留更早的已提交调用。
- 取消发生在工具提交后：该工具调用视为已完成并保留。
- Agent、Completion Gate 或后续工具失败：不回滚已经提交的修改。
- 用户不满意已保留结果时，通过项目历史入口恢复到 Run 开始前的 Project Checkpoint。
- 终态事件的 `affected_targets` 与 Runtime checkpoint 使用整个 Run 的累计已提交变更，而不是当前空事务。

## 验证要求

- 成功写工具返回后，正式路径可被 `os.ReadFile` 和 `run_command ls/cat` 立即读取。
- 后续 Agent 错误或用户取消后，先前成功写入仍存在。
- 单次多文件提交或元数据提交失败时恢复全部前像。
- 遗留 journal 在 receipt 已完成时前滚，在 receipt 不存在时回滚。
- outline 先落盘、spec/HTML 后补齐时，项目快照和下一次 Run 均可读取 pending 状态。
- 取消后的权威事件、Checkpoint 和项目快照报告已经保留的 affected targets。
