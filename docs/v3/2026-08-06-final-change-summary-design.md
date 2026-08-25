# 最终回复变更汇总设计

## 目标

最终回复不再只渲染普通 Markdown 文本，而是在存在产物变更时展示一个可折叠的变更汇总卡片：

- 折叠态：`N 个页面已经变更`，右侧显示总 `+insertions -deletions`。
- 展开态：每行一个变更目标，如 `第 1 页设计稿`、`第 1 页幻灯片`、`全局视觉设计`。
- 每行右侧提供两个动作：应用内跳转、外部打开文件。

## 后端契约

扩展 `PublicTarget`：

- `display_name?: string`：安全展示名，如 `第 1 页`。
- `insertions?: number`
- `deletions?: number`

行数统计由后端在写入工具成功时基于 before/after 内容计算，不依赖 Git 仓库。算法目标是 Git stat 风格的新增/删除行数，不需要输出完整 diff。

`RunSession` 已保存首次写入前的 `BeforeContent`，写入工具也持有本次 `raw` 内容，因此可在 `mutationResult` 阶段计算当前工具变更的行数。`run.finished.affected_targets` 也沿用同一 stat 字段，供最终回复聚合展示。

## 前端交互

当 `FinalMessageItem.affectedTargets.length > 0` 时：

- 渲染 `FinalChangeSummary`。
- 继续保留 `MarkdownMessage`，但它作为解释文本位于变更卡片下方。
- 变更标记 icon：`Sparkle`。
- 应用内跳转 icon：`Crosshair`。
- 外部打开 icon：`ExternalLink`。

跳转规则：

- slide spec：`setCurrentPage(index)` + `setGlobalView('outline')`
- slide html：`setCurrentPage(index)` + `setGlobalView('html')`
- deck outline/design：切到设计稿视图；后续可细化到全局区域。

外部打开：

- 使用 `vscode://file/...` 深链作为开发环境能力。
- Web 端不能强制默认 VS Code；如果系统注册了协议，浏览器会弹出确认并打开。

## 边界

- 不实现完整 diff 内容视图。
- 不改 Runtime 总体架构。
- 不强制外部编辑器打开。
- 若后端没有 file path，前端隐藏外部打开按钮。
