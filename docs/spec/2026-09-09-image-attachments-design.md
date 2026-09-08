# 图片附件与视觉参考上下文设计

- 状态：设计定稿，待实现
- 日期：2026-09-09
- 对应 TODO：【四、】上传图片与参考图，让 Agent 基于视觉素材创作
- 关联设计：2026-09-04-context-window-compaction-design.md
- 本次交付：本文档与 docs/demo/2026-09-09-image-attachments-demo.html

## 1. 需求解释

当前右栏输入框可以输入文字并引用页面、组件和 Prompt，但用户无法把截图、品牌图、产品图、手绘草图或视觉参考直接交给 Agent。要求用户用文字转述视觉信息，会丢失布局、风格、色彩、尺寸和图像细节。

本能力要建立一条完整链路：

1. 用户通过右栏回形针入口选择图片，或在输入框粘贴图片文件。
2. 图片选中后立即上传到当前项目，上传与发送消息解耦。
3. 附件在输入框内显示缩略图、文件名和状态，发送前可解除当前消息引用。
4. 发送时把稳定附件 ID 和图片 Item 一起放入对话上下文，使视觉模型直接看到图片。
5. 只要图片 Item 仍处于有效 Conversation Context，后续 Model Call 就继续可见；compact 或裁剪可以移除图片 Item，但不删除项目原图。
6. 图片不再位于当前 context 时，Agent 可通过 read_image 按附件 ID 重新读取缩略图或原图。
7. Agent 既可以把图片作为风格、内容或布局参考，也可以把原图直接用于幻灯片。

本阶段只处理静态单图，不扩展到 PDF、Office、表格、文本附件、GIF 或通用项目素材创建。

## 2. 已确认的产品决策

### 2.1 持久化与归属

- 图片是项目级资产，跨 thread、run 和 context compact 持久化保存。
- 附件根目录为项目工作目录内的 attachments/，与 manifest.json、outline.json、design.json 和 slides/ 同级。
- 项目删除时，附件随整个项目工作目录删除。
- 附件进入项目 Git 版本历史，为未来 checkpoint、rollback 和导出保留完整依赖。
- 从当前消息移除附件只解除消息引用，不删除项目文件。

### 2.2 格式、数量与安全限制

- 允许：PNG、JPG/JPEG、WebP。
- 禁止：GIF、SVG、其他动图、文档和多页容器。
- 单文件最大 10 MiB。
- 解码后像素数不得超过 40 MP。
- 每条用户消息最多引用 8 张图片。
- 文件选择和剪贴板粘贴都允许一次加入多张。
- 后端以 magic bytes 和实际解码结果为准，不信任扩展名和客户端 MIME。
- 上传后生成 WebP 缩略图，原图字节保持不变。

### 2.3 用户交互

- 输入框控制栏最左侧新增 icon-only 回形针按钮。
- 点击调用浏览器原生多文件选择，accept 只允许 image/png、image/jpeg、image/webp。
- 输入框接收剪贴板中的图片 File Item；同次粘贴的文字与图片分别进入文本和附件流程。
- 选择或粘贴后立即上传，而不是等发送消息时上传。
- 附件临时区位于文本编辑区下方、底部控制栏上方，仍属于 composer。
- 每张附件展示缩略图、截断文件名、大小和 uploading / ready / failed 状态。
- 失败项保留在 composer 内，提供重试；移除只解除本条消息引用。
- 不提供“仅参考 / 可用于页面”选择器，由用户提示词表达用途，Agent 判断。
- 有附件时要求 vision 模型；不支持时阻止发送并保留草稿与附件。
- 新 run 和运行中的 steering 都支持图片附件。
- 不实现拖放、项目素材库或附件管理页。

### 2.4 图片用途

- 视觉参考：理解内容、提取风格、分析布局、采样色彩、核对细节。
- 页面使用：将品牌图、产品图或截图直接嵌入页面。
- 本阶段不新增版权工作流。

## 3. 数据与文件模型

### 3.1 项目目录

~~~text
<workRoot>/projects/<projectID>/
├── attachments/
│   └── <attachmentID>/
│       ├── original.<ext>
│       ├── thumbnail.webp
│       └── meta.json
├── design.json
├── manifest.json
├── outline.json
├── slides/
└── threads/
~~~

attachmentID 由后端生成，不从文件名推导。文件名只作展示与来源信息，不参与路径解析。重复上传同一文件默认生成不同 ID，不做用户可见的跨消息去重。

### 3.2 meta.json

~~~json
{
  "version": "1.0",
  "id": "att_xxxxxxxx",
  "project_id": "pro_xxxxxxxx",
  "original_name": "brand-reference.webp",
  "media_type": "image/webp",
  "extension": "webp",
  "size_bytes": 1843200,
  "width": 2400,
  "height": 1600,
  "sha256": "<hex>",
  "original_path": "attachments/att_xxxxxxxx/original.webp",
  "thumbnail_path": "attachments/att_xxxxxxxx/thumbnail.webp",
  "created_at": 1788940800
}
~~~

文件系统是附件内容和元数据的事实来源；Run/Thread 持久化只记录稳定附件 ID 和当次引用关系。这样项目 Git 版本可独立还原原图与元数据。

### 3.3 消息附件引用

新 run 的 CreateRunRequest 以 attachment_ids 携带当前消息引用；steering 请求使用同一字段。后端必须在创建或注入消息前验证：

- ID 存在且属于当前 project。
- 元数据、原图与缩略图一致。
- 引用数量不超过 8。
- 当前模型支持 vision 和附件实际 MIME。

RunCommand 保存附件引用快照，至少包含 ID、原始文件名、尺寸、格式和消息内顺序，以供历史展示、重放和核对。

## 4. 后端设计

### 4.1 附件服务

新增项目图片附件服务，所有路径通过 artifactfs 沙箱解析，拒绝绝对路径、..、符号链接逃逸和跨项目访问。

写入流程：

1. 限流读取 multipart 字节，超过 10 MiB 立即失败。
2. 识别 magic bytes 并解码图像配置，验证格式和 40 MP 上限。
3. 在项目临时目录完成原图、缩略图和 meta.json 写入与 fsync。
4. 原子重命名为 attachments/<id>/；任一步失败均清理未发布临时目录。
5. 返回公开元数据，不暴露服务器绝对路径。

上传与 run 发送解耦：先上传得到附件 ID，随后的 run/steering 仍使用 JSON，不改成 multipart。

### 4.2 API 契约

| Method | Path | 用途 |
| --- | --- | --- |
| POST | /api/v1/projects/:id/attachments | multipart 上传一张图并返回元数据 |
| GET | /api/v1/projects/:id/attachments/:attachment_id | 获取元数据 |
| GET | /api/v1/projects/:id/attachments/:attachment_id/content?variant=thumbnail或original | 读取授权图片字节 |

本阶段不提供 DELETE API，因为移除 composer 引用不等于删除项目资产，且没有项目附件管理界面。

### 4.3 模型图片解析

现有 provider-neutral llm.ContentPart 已支持 image、ImageRef、MIMEType 和 Detail。实现时把仅支持运行截图的 runImageResolver 扩展为组合 resolver：

- run:<runID>/screenshot:<screenshotID>：现有运行截图。
- project:<projectID>/attachment:<attachmentID>/<variant>：项目附件。

resolver 必须校验 ref 内 project ID 与当前 run 一致，并再次验证字节上限与 MIME。各 provider 的 ImageInputMIMEs 仍是最终能力事实来源。

### 4.4 read_image 工具

~~~json
{
  "name": "read_image",
  "arguments": {
    "attachment_id": "att_xxxxxxxx",
    "variant": "thumbnail"
  }
}
~~~

- attachment_id 必填，只允许当前项目附件。
- variant 为 thumbnail 或 original，默认 thumbnail。
- 缩略图用于风格、构图与内容理解；原图用于像素细节、文字核对或直接使用素材。
- 返回模型可见 image content part 与简短元数据，不把 base64 文本写入公开时间线。
- 不并入 read_ppt：两者安全边界、返回类型和 context 分类不同。

### 4.5 页面嵌入

Agent 在幻灯片 HTML 中使用项目相对路径 /attachments/<attachmentID>/original.<ext>。后端 render worker 已能安全读取项目根目录资源，但前端 slide runtime 的 srcdoc 当前只重写 base/theme CSS，因此实现时必须补充附件 URL 桥接，让编辑器预览与 render worker 使用同一份项目文件。

该桥接只放行 attachments/ 白名单下的已校验图片，不扩张为 TODO 十的通用素材创建与任意文件服务。

## 5. Conversation Context 与进度条

### 5.1 图片 Item 生命周期

1. 仅上传：文件已在项目磁盘中，但未被消息引用；不进入 LLM，不占 context。
2. 发送消息：用户文本和被引用图片一起进入 provider-neutral message。
3. 有效 Conversation Context：图片 Item 保留在持久 transcript，后续 Model Call 继续可见并持续占用 context。
4. compact / 裁剪：可以移除图片 Item，但摘要必须保留附件 ID、文件名、尺寸、格式、所属消息和已知用途。
5. 按需恢复：Agent 用 read_image 重新读取缩略图或原图，返回的 image content part 再次进入有效 context。

项目持久化回答“原图是否仍存在”，Conversation Context 回答“下一次模型调用实际看到什么”，两者是独立生命周期。

参考事实：[OpenAI Responses API](https://developers.openai.com/api/reference/cli/resources/responses/methods/create) 说明 Conversation 中的既有 Item 会加入后续请求输入，自动截断会在超出模型 context window 时从对话开头移除 Item。本项目不依赖单一 provider 的服务端 Conversation，而由 provider-neutral transcript 实现等价且可测试的生命周期。

### 5.2 新增 uploaded_file 分段

现有 context bucket 从六类扩展为七类：

~~~text
read_ppt | run_command | system_prompt | user_prompt |
chat_history | uploaded_file | other
~~~

前端文案为“上传文件”。该分段仍遵守进度条原定义：只统计下一次 Model Call 实际可见的上传文件内容，不统计项目磁盘占用。

| 状态 | 是否计入 uploaded_file | 说明 |
| --- | --- | --- |
| 只上传，未引用 | 否 | 没有进入下一次 prompt |
| 当前消息的图片 Item | 是 | 图片 token 和附件描述均进入该桶 |
| 历史中仍有效的图片 Item | 是 | 不转入 chat_history |
| read_image 返回的图片 | 是 | 不进入 read_ppt |
| compact 后的轻量附件引用 | 是 | 只计描述文本的少量 token |
| 已裁掉且无引用 | 否 | 必要时可从项目重读 |

一个 token 只能属于一个 bucket。图片 Item 和附件描述不再重复计入 user_prompt、chat_history 或 read_ppt；同一消息的普通文本仍按原分类计算。

ContextWindowDetail 以原始文件名展示每张图的估算占用。source 可区分 message_attachment、read_image、compacted_reference，layer 仍为 seed 或 transcript。

### 5.3 估算与 compact

- 估算器拆分消息 content parts，把上传图片与附件描述归入 uploaded_file。
- 初版可沿用每张图片 1024 token 的 provider-neutral 稳定估算，再用 provider input usage 校准总量。
- compact 把图片 content part 转换为结构化附件引用摘要，不把 base64 或原图字节写入文本摘要。
- compact 前后 snapshot 应体现 uploaded_file 被释放的图片 token，避免把“原图仍在项目”误解为“原图仍在 context”。

## 6. 前端实现契约

### 6.1 状态模型

附件草稿按 thread 隔离，并与文本草稿一起在切换 thread 时恢复：

~~~ts
type ComposerAttachmentStatus = 'uploading' | 'ready' | 'failed';

interface ComposerAttachment {
  localId: string;
  attachmentId?: string;
  file: File;
  previewURL: string;
  name: string;
  size: number;
  status: ComposerAttachmentStatus;
  error?: string;
}
~~~

File 和 object URL 只用于当前浏览器上传过程；成功后以后端 ID 为准。移除、切换项目或销毁草稿时必须 revokeObjectURL。

### 6.2 发送条件

- 文本非空或至少一张 ready 附件时可发送，允许纯图片消息。
- 存在 uploading 附件时禁用发送并提示“正在上传图片”。
- 存在 failed 附件时不静默丢弃，用户必须重试或移除。
- 有附件且模型不支持 vision 时禁用发送，提示“当前模型不支持图片，请更换模型后发送”。
- 发送成功后清空当前消息引用和预览 URL，不删除项目文件。

### 6.3 可访问性与密度

- 回形针按钮使用 aria-label 和 title“选择图片”。
- 隐藏 file input 可由键盘触发按钮访问。
- 状态变化经 aria-live=polite 通知；错误说明具体文件和修复方式。
- 回形针属于 composer start control group，必须被现有控制栏宽度测量逻辑纳入。
- 窄屏附件区横向滚动，不展开成独立面板。

## 7. 需要同步切换的代码面

实现时一次性切换下列边界，不保留新旧协议并存：

- 后端：attachment model/service/handler/router、RunCommand 与 steering、transcript、image resolver、read_image、context estimator/compactor/public event 校验。
- 前端：API types/client、composer thread draft、file input、paste、上传状态、vision gating、run 与 steering、context bucket 类型与面板。
- 预览：附件 content endpoint、slide runtime URL 桥接、render worker 对齐。
- 文档：context window 六分类契约更新为七分类。

## 8. 错误契约

| 场景 | 错误码建议 | 用户文案 |
| --- | --- | --- |
| 格式不支持 | ATTACHMENT_TYPE_UNSUPPORTED | 仅支持 PNG、JPG 和 WebP 图片 |
| 文件超过 10 MiB | ATTACHMENT_TOO_LARGE | 图片超过 10 MiB，请压缩后重试 |
| 像素超过 40 MP | ATTACHMENT_DIMENSIONS_EXCEEDED | 图片分辨率过高，请缩小后重试 |
| 内容损坏或 MIME 不一致 | ATTACHMENT_INVALID | 无法读取这张图片，请更换文件 |
| 附件不属于项目 | ATTACHMENT_NOT_FOUND | 附件已不可用，请重新选择 |
| 超过每条 8 张 | ATTACHMENT_LIMIT_EXCEEDED | 每条消息最多添加 8 张图片 |
| 模型无 vision | MODEL_VISION_REQUIRED | 当前模型不支持图片，请更换模型后发送 |

## 9. 验收标准

### 9.1 上传与持久化

- 回形针选择和剪贴板粘贴都能一次上传多张静态图片。
- 成功图片形成独立 attachments/<id>/，包含原图、缩略图和元数据。
- 前端友好提示限制，后端独立强制限制。
- 移除 composer 附件不删除文件，删除项目则删除附件。
- 附件进入项目 Git 版本并可供未来 checkpoint 恢复。

### 9.2 模型与对话

- 第一轮及后续调用都能看到仍处于有效 Conversation Context 的图片。
- compact 可移除图片 Item，但保留稳定引用；read_image 可重新获取。
- read_image 不能读取其他项目、运行临时目录或任意服务器路径。
- 附件可用于视觉参考和页面嵌入，前端预览与后端渲染一致。
- 无 vision 模型在新 run 与 steering 均被阻止。

### 9.3 Context Window

- snapshot 固定包含 uploaded_file 桶与 details，即使为 0 也不缺字段。
- 进度条新增“上传文件”色段，不与其他桶重复计数。
- 只上传未引用的项目文件不影响 context 比例。
- 图片 Item 有效时持续占用 uploaded_file；compact 移除后该桶下降。
- read_image 返回图片进入 uploaded_file，而非 read_ppt。

## 10. 测试建议

- 附件服务：magic 校验、格式/字节/像素上限、原子发布、项目隔离、符号链接防护。
- API：multipart 上传、内容授权、run/steering 校验、稳定错误码。
- Provider：PNG/JPEG/WebP、无 vision 拦截、不支持 MIME 拒绝。
- Transcript/compact：图片跨轮保留、compact 后保留引用、read_image 恢复。
- Context：七桶总和、图片拆分归类、details 文件名、compact 前后差值。
- Composer：file picker、多图粘贴、状态、移除、纯图发送、vision 拦截、steering。
- Slide runtime：附件在编辑器预览与 render worker 中一致可见，路径逃逸被拒绝。

## 11. 本次不做

- 不实现生产前端、后端、Schema 或测试代码；本次只交付开发设计与视觉原型。
- 不支持拖放。
- 不支持 GIF、SVG、PDF、Office、文本或表格文件。
- 不提供项目附件库、附件删除或未引用文件清理。
- 原型不展示 read_image、context 进度条、模型能力拦截或页面嵌入。

## 12. Grilling QA 决策记录

以下保留本次需求分析中的问题、建议、用户决定和被修正的结论。

### Q1：图片是否持久化？

- 问题：图片作为项目资产长期保存，还是仅在当前消息/会话临时保存？
- 建议：项目级持久化，消息只保存稳定引用。
- 用户决定：同意项目级持久化。

### Q2：附件目录如何组织？

- 问题：是否放在项目目录的 attachments/ 并用稳定 ID 隔离？
- 建议：attachments/<attachment_id>/original.<ext> + 缩略图 + 元数据。
- 用户决定：同意。

### Q3：附件属于哪个 context 分类？

- 初始问题：是否新增 context 进度条分段？
- 初始建议：不新增，根据消息、历史或工具读取分入现有 bucket。
- 用户修正：必须新增“上传文件”分段。
- 最终决定：新增 uploaded_file；只统计下一次 Model Call 实际可见的上传文件，不统计仅存磁盘但未引用的图片。

### Q4：是否进入项目版本历史？

- 问题：附件是否由项目 Git/checkpoint 跟踪？
- 建议：进入，避免回滚后页面依赖丢失。
- 用户决定：进入。

### Q5：前端入口范围？

- 问题：实现拖放、粘贴和选择，还是保留轻量入口？
- 用户决定：支持粘贴图片文件和浏览器原生文件选择；只支持图片，不做拖放。

### Q6 / Q12：后续 Model Call 如何看到图片？

- 初始问题：图片是否首轮后立刻从 context 移除并依靠工具重读？
- 初始建议：首轮后仅保留轻量引用。
- 用户补充：只要 Item 仍处于有效 Conversation Context，后续 Model Call 就能看到；裁剪或压缩后原图才可以被移除。
- 最终决定：图片 Item 在有效 context 中持续保留；compact/裁剪后保留引用，必要时用 read_image 恢复。

### Q7 / Q13：只作参考还是可直接使用？

- 问题：图片是否同时支持视觉参考与页面嵌入，是否需要用途选择器？
- 建议：两种用途都支持，不新增选择器，以提示词表意。
- 用户决定：同意。

### Q8：移除时是否删除文件？

- 用户决定：只解除当前消息引用，文件继续保留在项目中。

### Q9 / Q14：格式和上限？

- 用户决定：不支持 GIF，只支持 PNG、JPG/JPEG、WebP。
- 推荐且已确认：单文件 10 MiB、最高 40 MP、每条消息最多 8 张。

### Q10：模型不支持 vision 怎么办？

- 建议：阻止发送并保留草稿。
- 用户决定：阻止发送。

### Q11：运行中的 steering 是否支持图片？

- 建议：新 run 与 steering 都支持。
- 用户决定：支持。

### Q15：选择和粘贴是否支持多图？

- 建议：两者都支持，共用每条消息 8 张上限。
- 用户决定：同意。

### Q16：最终统计边界

- 用户决定：所有边界已确定。
- 最终落点：uploaded_file 独立统计当前有效 prompt 中的上传文件；仅上传未引用为 0，消息图片、历史有效图片、read_image 结果和 compact 后轻量引用按实际内容计入。

## 13. 代码现状与实现注意点

- 项目工作目录为 <workRoot>/projects/<projectID>，初始化时建立独立 Git；attachments/ 不在现有 ignore 列表中，会进入项目版本。
- 当前没有 attachment model/service/route，run 与 steering 只接收文本和现有引用。
- llm.ContentPart 和 provider adapter 已具备 image ref 基础，但 resolver 只接收当前 run screenshot。
- transcript 已可持久化 ContentPart，可承载有效 context 中的图片 Item。
- context bucket 在 Go 枚举、估算器、SSE 校验、TypeScript union 和右栏面板均硬编码为六类，新增 uploaded_file 必须全链路同步切换。
- composer 当前 paste handler 只取纯文本并 preventDefault，需要为图片 File Item 新增显式分支。
- slide runtime 是 sandboxed srcdoc，前端附件预览需要受限 URL 桥接，不能只依赖 render worker 的项目文件服务。
