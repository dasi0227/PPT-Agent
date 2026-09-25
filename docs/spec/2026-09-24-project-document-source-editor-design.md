# 内容要求与视觉要求的源文件编辑

## 范围与设计依据

按最新用户决定，将现有「预览 / 源文件」能力扩展至项目级内容要求和视觉要求。本文替代 [源文件编辑设计](2026-09-24-source-preview-editor-design.md) 中仅开放单页 Spec、HTML 的范围限制；其草稿、保存、历史现场、任务互斥和冲突处理规则继续适用。

| 展示对象 | 原始文件 | 范围 |
| --- | --- | --- |
| 内容要求 | `manifest.json` | 项目 |
| 视觉要求 | `design.json` | 项目 |
| 设计稿 | `slides/<slide_id>/spec.json` | 页面 |
| 幻灯片 | `slides/<slide_id>/index.html` | 页面 |

Manifest 遵循 [内容要求边界](2026-09-24-manifest-content-boundary-design.md)，Design 遵循 [视觉要求与主题边界](2026-09-24-visual-requirements-theme-boundary-design.md)。不在 Design 中重新引入主题字段；Manifest 演示标题继续独立于数据库中的项目名称。

## 界面与编辑

- 选择左侧「内容要求」「视觉要求」后，底部「切换形态」可在文档预览与原始 JSON 编辑器之间切换，沿用 AppWindow、FilePen 图标。
- 项目文档不依赖当前页面，没有幻灯片时也能查看、编辑、保存。页面视图、翻页、缩放控件继续保持不可用。
- 共用 CodeMirror、保存按钮及 `⌘S / Ctrl+S`，保存时格式化 JSON；输入、失焦和导航只暂存本地草稿。
- 项目文档草稿按项目和 kind 标识，页面 ID 为空，不绑定当前选中页。切换页面不改变它们的身份。
- 未保存列表、保存全部、Agent 启动检查和历史草稿同时覆盖四种文件。保存全部依次处理内容要求、视觉要求，再按页面顺序处理 Spec、HTML；失败时停下并打开对应源文件。
- 预览只使用已保存内容，有草稿时显示提示及统一保存入口。保存后刷新项目内容快照，沿用既有页面失效和 Runtime 装饰刷新规则，不自动运行 Agent。

## 服务端合同

新增 `GET /api/v1/projects/:id/source?kind=manifest|design` 和相同地址的 `PUT`，复用现有源文件响应及保存请求合同。项目文档返回 `slide_id: ""`、`language: "json"` 和根目录相对路径。

页面端点仍只接受 `spec|html`；项目端点只接受 `manifest|design`。项目文档验证项目存在、目标正式文件存在，不查询或依赖 outline 页面成员；不接受任意路径、不创建缺失文件。

三种 JSON 源文件统一先检查严格 JSON、重复键和原始输入的 Schema，再解码为领域结构，避免缺失字段或 null 在解码后被默认值掩盖。按 [创作 JSON 元数据精简](2026-09-25-authoring-json-metadata-removal-design.md)，文件不再记录 `project_id`、`version`、`created_at` 或 `updated_at`，页面 Spec 也不记录 `slide_id`。身份由请求及文件路径确定，响应外层继续返回身份；已损坏的内容允许打开修复，保存必须满足当前 Schema。

写入继续核对字节 hash、历史现场及项目忙状态，在项目历史门禁和共享写锁内完成原子替换。纯格式保存保持业务 hash，最终字节无变化不推进历史或要求丢弃后续历史。

## 验证范围

定向测试覆盖无页面项目读取和保存、请求前置条件、旧 hash、无变化历史、JSON 与 Schema 校验、缺失文件不可创建、项目文档草稿启动拦截，以及保存全部失败定位。交互和视觉由用户手动验收，不使用浏览器自动化。
