# Run 内已读图片保留

> 工具输入输出、审批状态、资源版本与图片恢复规则已由 [2026-09-30 协议统一实现](2026-09-30-model-tool-protocol-design.md) 更新；冲突时以该文档和根目录 TOOL.md 为准。本文保留设计演进记录。

## 决定与范围

以本轮用户决定为准：同一 Run 通过 `read_image` 成功读取的全部图片持续提供给主 Agent，不按 Turn 数量、是否写出文字结论或固定图片张数释放。覆盖渲染截图和读图工具读取的上传附件。

本文取代 `2026-09-22-render-image-on-demand-design.md` 中“仅下一次响应可见”的生命周期约定。此次仅修改后端图片生命周期与相关提示词，不增加前端 timeline，也不修改工作账本或计划推进逻辑。

## 数据结构与存储

- PNG 和上传附件继续存放在项目 artifacts 下现有位置，不复制图片文件。
- `RunState.readImages` 使用 `[]RunReadImage`，记录 `image_ref / mime_type / detail / observation`。`observation` 是读图工具当时返回的元信息，不是模型视觉判断。
- 按 `image_ref` 去重并保留首次读取顺序。同一截图重复读不会添加第二份像素输入；同页不同截图 ID 和附件不同 variant 分别保留。
- `RuntimeCheckpoint.read_images` 持久化相同结构，由现有 SQLite `runs.checkpoint_json` 保存。新图片成功读入后保存 `after_image_read` 检查点；常规、等待、终止检查点也保留集合。
- 检查点只存引用与文字元信息，不存 PNG 字节或 Base64。请求供应商时通过现有 resolver 校验引用、读取文件、编码图片。

## 模型上下文

1. 工具结果保留读图描述及调用对应关系；图片由 Run 集合统一提供，避免重复调用不断叠加像素。
2. `prepareAgentRequest` 生成 `run_read_image` 消息，包含已读图片与描述。已有图片消息保留原位置，新图片追加，避免每轮移动图片而破坏供应商历史回放。已有用户附件图片若与集合引用相同，当前请求只保留一份。
3. 这些消息带有内部 `runtime/run_image` 标记，属于派生上下文，不写入普通线程历史。每次请求由当前 Run 集合重建；文字压缩只处理历史文字，压缩完成后自动恢复全部已读图片。
4. 移除成功模型响应后的图片清理，以及由图片单轮释放引起的 continuation 清理。图片集合或最新渲染索引变化、压缩和恢复仍按实际上下文变化处理续接。
5. 同一 Run 恢复时加载 `read_images`；新 Run 从空集合开始，不从历史已读工具描述推断或自动恢复旧 Run 的全部图片。原有用户直接上传/选择附件的普通消息生命周期保持不变。

## 版本与预算

- `latest_rendered_images` 仍表示每页最新可用截图；`read_images` 表示本 Run 实际读过的全部图片，两者不能替代。
- 旧截图保持可见。每次请求根据最新索引给出 `render_state`：`current`、`stale`、`superseded`、`not_in_current_index` 或 `unknown`。读入时的描述保留为 `observation_at_read`，避免将旧的 `stale:false` 当作当前状态。
- `current` 只表示当前版本有新鲜渲染依据，不表示模型已确认视觉质量。所有其他状态的截图只能作为历史参照，不能证明当前页面效果。
- 图片计入现有上下文估算的 `read_image` 项和模型窗口预算。没有额外的 4 张上限，也不在超限时静默丢图；文字压缩后仍超过窗口时，走既有明确失败路径。
- 重复读取无进展保护继续生效。保留图片不会自动更新 Plan 或将页面标记为已完成。

## 验证范围

定向用例覆盖多轮保留、重复读去重、附件读图、检查点 JSON 往返与同 Run 恢复、新 Run 隔离、连续两次文字压缩及新旧截图状态变化。供应商适配器继续使用已有的图片引用解析与图片内容块编码。

遵循仓库 AGENTS.md，本次不运行测试、构建、浏览器或真实模型验证。可由用户在 `backend` 目录执行：

```sh
go test ./internal/workflow -run 'TestRunReadImages|TestReadImageAllowsOnlyLatestImageOfAnExistingPage|TestReadLoop'
```

更新代码后需重启后端。历史运行的检查点没有 `read_images`，不会迁移或从旧文本补造图片集合；新读取会按新结构保存。
