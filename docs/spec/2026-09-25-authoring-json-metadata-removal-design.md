# 创作 JSON 元数据精简

> 后续决定：materialization 的最终边界以 [HTML 生成参考快照与变化上下文](2026-09-25-html-generation-reference-snapshots-design.md) 为准，不再保留独立文件，创作 JSON 的业务字段精简规则继续有效。

## 决定与范围

按用户决定，`manifest.json`、`outline.json`、`design.json` 和 `slides/<slide_id>/spec.json` 删除根层的 `project_id`、`version`、`created_at`、`updated_at`；页面 Spec 另删除根层 `slide_id`。四类文件只记录业务内容及必要的结构引用，不新增替代元数据文件或项目格式字段。

本决定替代此前内容 hash、内容要求、视觉要求及源文件编辑文档中保留这些字段的约定。前后端与 Agent 合同一次性切换到当前 Schema，不提供旧字段兼容、双读双写或历史开发数据迁移。

数据库中的项目、会话、任务时间及历史事件时间保持各自用途。后续按 [持久化辅助字段精简](2026-09-25-persistence-fields-cleanup-design.md) 进一步删除 materialization 的格式版本与 `rendered_at`、附件冗余字段及未使用的 checkpoint 字段；其他有实际用途的协议版本、checkpoint 时间与现场标识保留。

## 数据与写入

- 四类领域结构、JSON Schema、前端类型、初始化与 mutation 写入同步移除上述字段。Schema 继续拒绝未声明的字段。
- 项目身份由请求与项目工作目录确定；页面身份由请求中的页面 ID、大纲引用和 `slides/<slide_id>/` 路径确定。大纲节点的 `slide_id`、章节与子章节的 `id` 继续保留。
- 内容快照在接口外层统一返回 `project_id`，前端据此构建页面模型和预览上下文；`slides_by_id` 继续使用页面 ID 作为键。源文件响应、运行目标、历史记录、数据库和缓存键继续携带各自需要的身份。
- Agent 页面上下文从运行范围传递目标页面 ID，相关页面查询与 HTML 路径不再读取 Spec 正文身份。Polish 上下文在目标外层返回 `slide_id`。
- ResourceHash 对完整 JSON 对象规范化后计算 hash，忽略格式及字段顺序，不再特殊剔除时间字段。内容 hash 不写回创作文件。
- 写入并发继续使用 expected_hash；源文件编辑继续使用字节 hash、项目写锁和历史现场门禁。保存目标仍检查项目归属、大纲成员、沙箱路径和文件存在；取消已不存在的正文身份字段对比。
- 已有 JSON 即使语法或业务内容损坏，也可打开原文修复；只在通过严格 JSON、重复键与 Schema 校验后保存。修复前的内容 hash 可为空，字节 hash 仍用于防止覆盖其他修改。
- 相同内容的 mutation 不写文件、不使页面失效；源文件纯格式保存不改变内容 hash，最终字节相同的保存不推进历史。
- 编辑器删除 `updated_at` 的提取、透传以及撤销/重做补偿，保留正常草稿、格式整理和撤销历史。

相同内容允许具有相同内容 hash，资源身份仍由项目 ID、页面 ID 和资源种类共同确定。把 A 页 Spec 内容复制到 B 页路径表示替换 B 页内容；系统不再通过正文 ID 推断它来自 A 页，也不自动切换写入目标。内容 hash 不能替代身份与归属校验。

删除这些字段会改变现有资源内容 hash；旧渲染证明与旧 checkpoint 不保证适用。此次不自动改写本地开发项目或历史数据，验收使用新建项目。

## 实施与手动验收

已同步修改实现、Agent 提示词与受影响的既有测试。本轮按仓库约定只进行静态复核和源码格式整理，未执行测试、构建或浏览器验证。

用户可在 `backend` 目录运行定向回归：

```sh
go test ./schemas ./internal/spec ./internal/pptmutation ./internal/contextengine ./internal/service ./internal/httpapi
```

用户可在 `frontend` 目录运行类型检查与源文件草稿回归：

```sh
pnpm exec tsc --noEmit
pnpm exec vitest run src/stores/sourceEditorStore.test.ts src/stores/projectStore.test.ts src/lib/sourceFormatting.test.ts src/features/deck/selectors.test.ts src/features/viewer/runtimeFrame.test.ts
```

交互验收：新建项目并生成页面，确认四类 JSON 无根层 `project_id`、`version`、`created_at`、`updated_at`，Spec 无 `slide_id`，大纲页面引用仍在；编辑、保存、撤销与重做内容要求、视觉要求及页面 Spec；确认页面重排后选择与预览仍对应原页面，旧 hash 仍拒绝覆盖，损坏 JSON 可修复，页面过期状态和历史恢复保持预期。
