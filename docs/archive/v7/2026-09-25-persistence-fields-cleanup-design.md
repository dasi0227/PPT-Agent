# 持久化辅助字段精简

> 2026-09-25 持久化切换说明：本文中的旧数据库表、独立 user/model JSONL、命令专用执行接口及事件流描述已由[会话日志、命令与数据库重构总结](2026-09-25-session-storage-refactor-design.md)替代。其他产品行为仍按最新用户决定执行；新实现仅做静态复核，手动验收见新文档第 17 节。

> 后续决定：materialization 已按 [HTML 生成参考快照与变化上下文](2026-09-25-html-generation-reference-snapshots-design.md) 整体退出，改为 SQLite 按页生成参考快照及独立渲染证据，本文其他辅助字段清理规则继续有效。

## 最新决定

继创作 JSON 精简后，按用户决定删除其他持久化记录中无业务消费者的字段，以及能够从已校验身份和固定目录规则推导的路径副本。前后端直接切换，不增加兼容层或迁移流程，不批量改写既有项目、数据库记录和历史快照。

## 删除与替代

| 记录 | 删除的持久化字段 | 处理方式 |
| --- | --- | --- |
| `attachments/<id>/meta.json` | `version`、`created_at` | 删除固定版本校验及接口中未使用的创建时间 |
| 同上 | `original_path`、`thumbnail_path` | 由校验后的附件 ID 和扩展名生成；读取、Agent 引用与导出共用路径方法 |
| `slides/<id>/materialization.json` | 整份文件及其 Schema | 已由按页数据库生成参考快照与独立渲染证据替代，不迁移旧记录 |
| `.runtime/render-index/<id>.json` 和 `refs/<id>.json` | `image_path` | 由校验后的 Run ID 和截图 ID 生成，工具与上下文仍输出可读取路径 |
| `checkpoints/state.json` 及快照内的 state | 根层 `sequence`、`checkpoints[].sequence` | 使用 checkpoint 数组顺序及 Run ID，删除无消费者的计数与接口透传 |
| 项目 checkpoint 快照根层 | `work_dir` | 从当前工作根目录和项目 ID 生成恢复位置；项目数据库行缺失时同样可恢复 |
| SQLite `run_checkpoints.checkpoint_json` | `message_summary`、`latest_tool_results`、`provider_continuation`、`context_briefing` | 删除恢复流程未消费的诊断副本及专属生成逻辑，保留实际运行中的消息、上下文和 provider continuation |

删除 Runtime checkpoint 诊断副本会减少直接查看该 JSON 时的冗余信息；正式会话记录、工具事件与运行 trace 继续承担各自用途。`context_briefing` 仍按当前运行上下文生成，模型请求和语义审查继续使用它。

## 保留的职责

- 附件身份、项目归属、类型、尺寸、大小和 SHA-256；类型与扩展名先校验，再生成路径。
- 截图的项目、页面、Run、截图身份，来源和依赖 hash，以及发送给 Agent 的 `rendered_at`。截图索引中的时间与已删除的 materialization 时间是不同字段。
- 页面内容和渲染证明的全部 hash，既有过期状态判定。
- 项目历史的 revision、scene_revision、时间、输入、快照引用和操作幂等标识；保留快照完整性、项目归属与文件路径边界检查。
- Runtime checkpoint 的恢复状态、计划、范围、审批现场、模型路由及预算用量。
- 聊天事件序号和时间、草稿更新时间，以及其他有实际消费者的协议字段。

项目快照的 `database` 仍是完整数据库记录；本次删除的是快照根层重复的 `work_dir`，不调整数据库项目模型或恢复清单。

## 实施与验收

已同步修改实现、接口类型、既有回归用例与相关设计文档。本轮仅进行静态阅读和源码格式整理；按仓库约定，不执行测试、构建或浏览器验证。

用户可运行以下相关回归：

```sh
cd backend
go test ./internal/attachment ./internal/renderimage ./internal/spec ./internal/projecthistory ./internal/export ./internal/service ./internal/workflow ./internal/httpapi ./schemas
```

```sh
cd frontend
pnpm exec tsc --noEmit
pnpm exec vitest run src/features/agent/ProjectHistoryControls.test.tsx
```

手动验收使用新记录：上传并引用图片、预览缩略图、导出含附件的 PPT；生成页面并渲染、检查过期状态、读取最新截图；创建多个任务并回退、恢复最新。崩溃恢复重点关注项目删除中断且数据库项目行已不存在的场景。检查新生成文件仅含当前结构，不以旧历史快照仍带有旧字段判断新写入行为。
