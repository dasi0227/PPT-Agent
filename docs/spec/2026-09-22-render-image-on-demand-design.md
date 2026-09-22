# 页面渲染图片按需读取设计

## 1. 需求与问题

页面渲染图片是 Agent 检查布局的临时视觉输入，不应作为常驻聊天图片反复发送。Runtime 只注入当前项目每页最新渲染图片的路径及必要的版本、过期状态；Agent 需要观察时，主动调用 `read_image`。DOM 标记继续只携带结构化 DOM 快照，不生成截图；用户主动上传的图片保持附件协议和生命周期不变。

此前 `render_slide` 自动返回图片 content part，图片引用被写入聊天历史；下一次 Run 重放旧 Run 图片时，resolver 仅授权当前 Run，导致请求在本地组装阶段失败，却被误报为模型服务拒绝。这不是重启旧后端就能解决的问题。

本文取代此前设计中关于运行截图自动进入上下文、使用 `run:…/screenshot:…` 引用、直到 Compact 才释放像素的约定。附件、DOM 标记和 UI 历史截图展示不受影响，不提供新旧协议兼容分支或历史索引回填。

## 2. 工具与 Runtime 协议

### 2.1 render_slide

- 入参只保留 `slide_id`，删除 `visual_review`。
- 渲染仍生成 PNG、执行布局诊断并产生 materialization evidence。
- 模型 observation 只包含诊断、`image_path`、来源 hash 等文字信息，不自动附带图片，即使诊断失败也不附带。
- 截图文件成功生成、来源校验完成后，原子更新该页最新图片索引。失败不会覆盖之前成功生成的索引。
- UI evidence 继续保留 screenshot URL；模型 Runtime evidence 不重复注入历史截图 URL、引用或路径。

### 2.2 latest_rendered_images

每次调用模型前，从当前 outline 和项目索引重新生成列表，每个尚存在且有可读渲染结果的页面最多一项：

```json
{
  "slide_id": "sli_example",
  "image_path": ".runtime/renders/<run_id>/<screenshot_id>.png",
  "source_hash": "<HTML hash>",
  "rendered_at": 1790000000,
  "stale": false
}
```

列表按 outline 顺序注入 Runtime，不包含图片字节或 base64。索引不存在或图片缺失时不列出；页面删除后不再列出。HTML、manifest、页面 outline 节点、slide spec、design 或运行时页框依赖变化后标记 `stale`；旧图可用于对照，但不能据此判断当前页面效果，需要重新渲染。该状态不是对任意外部资源变化的监测承诺。

### 2.3 read_image

两种互斥调用：

```json
{"image_path": ".runtime/renders/<run_id>/<screenshot_id>.png"}
```

```json
{"attachment_id": "att_example", "variant": "thumbnail"}
```

`variant` 仅用于附件。`image_path` 必须精确匹配当前仍存在页面的最新索引；禁止任意路径、越界路径和已被替代的路径。渲染图不是可以嵌入页面的用户素材。

## 3. 图片生命周期与费用边界

1. 普通模型请求只携带路径索引，不自动携带页面截图。
2. 显式 `read_image` 后，图片仅进入接下来一次成功的模型响应上下文；Agent 应把视觉发现保存为文字。
3. 成功响应后清除渲染图片 content part 和 provider continuation，防止服务端续接状态继续携带图片。模型重试或切换备用模型尚未成功时，仍保留待观察图片。
4. 聊天历史写入、恢复和 Compact 均移除渲染图片 content part，保留文字观察、诊断、工具调用对应关系和用户附件。
5. 如果读取图片后立即触发 Compact，压缩输入只有文字，尚未观察的图片以临时视觉输入保留到接下来一次响应，仍不持久化。
6. 图片索引发生变化时清除 continuation，确保模型拿到最新 Runtime 信息。

读取图片的那次模型请求仍会传输图片并消耗视觉 token；本设计减少的是无需求情况下的重复传输，不意味着视觉分析免费。旧历史中的文字、路径和视觉结论仍可能存在，但不再自动触发图片解析。

## 4. 存储与授权

- PNG：项目 artifacts 下 `.runtime/renders/<run_id>/<screenshot_id>.png`。
- 最新页索引：`.runtime/render-index/<slide_id>.json`。
- 单张图片注册记录：`.runtime/render-index/refs/<screenshot_id>.json`。
- provider 内部引用：`project:<project_id>/render:<slide_id>/<screenshot_id>`，不接受旧 Run 引用。
- `read_image` 只允许读取最新页索引；resolver 按项目和注册记录解析不可变快照，使同批工具先读取、后重新渲染时，已读取的图片不会在发送前突然失效。
- 校验项目归属、标识符、精确派生路径、沙箱边界、普通文件、非空、10 MiB 上限和 PNG 文件头；不以模型提供的路径直接读取文件。
- 注册和索引通过临时文件及 rename 发布。原始图片与历史注册记录不主动删除，避免破坏 UI 历史截图展示；磁盘清理不是本次范围。
- 缺失、损坏或未注册图片返回可行动的读取错误；本地 resolver 失败使用独立的 `IMAGE_REFERENCE_UNAVAILABLE`，不再归类为 `PROVIDER_BAD_REQUEST`。

## 5. 验收与交付

- 旧聊天包含其他 Run 截图时，新 Run 不再因旧截图授权失败而被拒绝；原文字记录与用户上传图片保留。
- 连续渲染同一页，Runtime 始终只列最新路径；其他页索引不受影响；修改页面后标记过期；删除页面后移除索引展示。
- 不调用 `read_image` 时请求没有渲染图片；调用后下一次请求有图片，再下一次请求只保留文字。
- 读取相同项目、前一次 Run 的最新图成功；越权项目、未登记图片、任意路径、过期路径、越界 symlink、已删除页均被拒绝。
- Compact、聊天持久化和 provider continuation 不使旧图片重新进入后续请求。
- 用户附件读取与页面嵌入方式不变；UI 历史截图 URL 不变。

对应回归用例位于 renderimage、contextengine、workflow、service 和 llm 的测试中。依照项目要求，验证由用户手动执行；此次开发不自动运行测试或浏览器验证。更新后需重启后端加载新代码，再在原对话重试；已有历史无需手动删除，未登记的旧截图需要重新渲染后才会出现在最新索引中。
