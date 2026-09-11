# 独立 HTML、PNG 与 PDF 导出设计

- 状态：已实现
- 日期：2026-09-12
- 对应 TODO：【七、】导出可独立使用的 HTML、图片和 PDF
- 关联设计：
  - `2026-09-08-unified-html-canvas-design.md`
  - `2026-09-09-image-attachments-design.md`
  - `2026-08-26-deck-outline-mutate-ppt-architecture-design.md`

## 1. 需求解释

当前产品已经可以在编辑器中预览、全屏展示 HTML 幻灯片，并通过 Chromium 对单页执行 1920×1080 截图，但尚无面向用户的整套交付入口。项目页面仍依赖应用提供的 Base CSS、Theme CSS、附件访问接口和外层 Runtime；直接复制 `slides/<slideID>/index.html` 无法离开应用独立播放。

本次能力增加一个简洁、前台、整套导出的交付流程：用户从预览区的单一“导出”入口选择 PNG、PDF 或 HTML，等待不可退出的进度过程，生成完成后主动点击下载，由浏览器保存最终文件。

三种格式都只导出创建任务时的完整页面集合，不提供当前页、选定页、章节、分辨率、纸张或质量选项。导出任务必须绑定一次冻结内容快照；快照完成后的项目修改不能混入本次产物。

## 2. 已确认的产品决策

### 2.1 入口与交互

- 预览区顶栏只增加一个“导出”按钮，位置紧邻全屏入口。
- 点击按钮展示极简下拉菜单：导出 PNG、导出 PDF、导出 HTML。
- 点击任一格式后立即创建导出，不再展示配置、范围选择或预览确认页面。
- 生成过程使用不可关闭的模态层，只展示格式、进度条、阶段和页面进度。
- 生成期间不提供取消按钮；`Esc`、点击遮罩、项目切换均不能关闭模态层。
- 刷新、关闭标签页或离开页面时使用浏览器离开确认；用户仍强制离开时取消导出并清理临时内容。
- 生成完成后不自动下载，模态层显示“下载”按钮。
- 用户点击下载后，保存位置完全交给浏览器：浏览器可以使用 Downloads，也可以按自身设置询问保存位置。
- 文件成功传输后关闭模态层，删除后端临时文件和一次性任务。
- 不提供导出历史、刷新恢复、跨会话恢复或长期产物管理。

### 2.2 页面范围与资格

- PNG、PDF、HTML 均始终导出 Outline 中的全部页面。
- 页面顺序只来自冻结 Outline 的 `FlattenOutline` 结果，使用稳定 `slide_id` 标识页面。
- 任意页面缺少 HTML 时完全阻止导出，列出全部缺失页面，不生成空白占位，也不静默跳过。
- `fresh`、`spec_stale`、`design_stale`、`frame_stale`、`unknown` 均允许导出；只要 HTML 存在，就使用冻结时的 HTML 与当前 Runtime Frame。
- 导出不修改 materialization 状态，不生成页面版本，不写 `last_export_at`。
- 任意页面在渲染或打包阶段失败时，整单失败，不发布残缺 ZIP 或 PDF。

### 2.3 并发与版本

- 存在 active、paused 或 recovering Agent Run 时禁止开始导出。
- 存在项目 Git 提交操作时禁止开始导出。
- 同一个项目同一时间只允许一个导出任务。
- 创建导出时短暂取得项目锁，完成快照后立即释放，不在整个渲染期间占用项目锁。
- 快照释放锁后，后端允许项目继续修改；其他窗口或后续请求产生的变化不进入本次导出。
- 每次用户选择格式都创建新的导出和新的内容快照，不复用已有图片、PDF、HTML 包或缓存结果。
- 请求级 `client_request_id` 只用于消除一次点击产生的网络重试，不构成跨导出缓存。

### 2.4 字体与外部资源

- 本次不引入字体二进制文件，不做字体下载、授权管理或字形子集化。
- HTML 包只携带 Theme CSS 中的字体栈声明；目标电脑缺少首选字体时允许使用系统 fallback。
- PNG/PDF 以导出机器当时实际可用的字体为准。
- 产品不承诺 HTML 在另一台电脑上与 PNG/PDF 像素级一致。
- 普通 `<a href>` 外部超链接允许保留。
- HTML 页面中的外部图片、样式、脚本和字体 URL 保留，导出过程给出“播放时需要联网”的警告。
- PNG/PDF 渲染禁止访问外网；检测到外部渲染资源或运行时实际请求外网时，整单失败并指出受影响页面。

### 2.5 正式兼容范围

- HTML ZIP 解压后通过 `file://` 双击 `index.html` 打开。
- 第一版正式支持当前主流 Chrome 与 Edge。
- Safari、Firefox 和其他浏览器只做尽力兼容，不作为验收条件。

## 3. 交付物契约

### 3.1 PNG

- 一个 ZIP 文件，包含整套页面的 PNG。
- 每页固定 1920×1080、device scale factor 1、不透明背景。
- 页面使用冻结 Runtime Frame 注入页码、章节标识和公共装饰。
- 文件按冻结页面顺序命名：`001-<安全化页面标题>.png`、`002-<安全化页面标题>.png`。
- 序号固定三位，标题移除路径分隔符、控制字符和平台非法字符；序号保证重名标题仍不冲突。
- 不提供 2×、透明背景或其他尺寸。

### 3.2 PDF

- 一个完整 PDF 文件，每张幻灯片对应一页。
- PDF 内容由与 PNG 导出相同的 1920×1080 栅格页组成，优先保证静态视觉一致。
- 页面固定为 PowerPoint 宽屏比例 13.333333×7.5 英寸，无页边距、无额外打印页眉页脚。
- PDF 文字不可选择，不承诺矢量文字或可编辑性。
- 任意页面 PNG 渲染失败时不生成 PDF。

### 3.3 HTML

- 一个 ZIP 文件，解压后直接打开根目录 `index.html`。
- 只包含播放所需内容，不包含 Manifest、Outline、Design、Slide Spec、Materialization、附件元数据、缩略图或其他开发文件。
- 所有项目图片附件的原始文件均进入包内；不只复制静态扫描命中的附件，避免页面脚本动态引用时遗漏正式 PPT 素材。
- 不包含字体文件。

建议目录结构：

~~~text
<deck-name>-html/
├── index.html
├── runtime/
│   ├── player.css
│   └── player.js
├── assets/
│   ├── base.css
│   └── theme.css
├── slides/
│   ├── 001.html
│   ├── 002.html
│   └── ...
└── attachments/
    └── <attachmentID>/
        └── original.<ext>
~~~

播放器提供：

- 上一页、下一页按钮；
- `←`、`→`、Space、Home、End 键盘导航；
- 当前页数与总页数；
- 浏览器全屏；
- Runtime Frame 公共装饰；
- 页面自身 JavaScript 执行。

播放器不提供：

- 编辑器 DOM 选择桥接；
- 页面编辑、设计稿或 Agent 能力；
- Overview；
- 讲稿；
- 进入、离开、暂停、重播等统一动画生命周期；
- Presenter View；
- 导出源数据清单。

这些播放增强能力仍属于 TODO 第九项。

## 4. 当前实现约束

### 4.1 内容与版本不是单一数据库版本

当前 `ProjectContentSnapshot` 由 Manifest、Outline、Design 和逐页内容组合，没有全项目 version ID。`versions` 表按 target 分散记录版本，Git HEAD 也可能落后于当前项目工作树。因此导出不能使用 Slide `current_version`、单个 revision 或 Git hash 代表整套内容。

本次为每个导出独立计算 `source_digest`。它仅用于证明一次任务内部读取的是同一快照，不创建新的用户可见版本体系，也不落为导出历史。

### 4.2 预览与 Chromium Runtime 尚未共享单一实现

前端 `slide-runtime/index.html` 和后端 `render-worker/worker.mjs` 分别实现 Runtime Frame 公共装饰。导出不得再创建第三套不同规则。实现时应抽出可复用的 Runtime chrome 规范或至少建立共享 fixture/契约测试，确保三处对以下字段解释一致：

- canvas；
- ordinal / total；
- numbering visibility；
- section / subsection；
- deck title；
- chrome type / placement / style。

本次不借机扩展未完整实现的 chrome 语义；保持现有受支持集合。

### 4.3 项目正式素材范围

当前正式项目素材仅为图片附件：PNG、JPEG、WebP 原图。通用 CSS、JavaScript、SVG、字体和数据文件的项目级创建与依赖图仍属于 TODO 第十项。

HTML 包复制全部正式附件原图，但不扫描和交付任意项目文件。页面引用未知项目相对路径时视为不受支持资源：PNG/PDF 失败；HTML 打包失败并指出页面与路径，避免产生打开后静默缺失的演示包。

## 5. 总体架构

~~~text
ExportButton
  └─ 选择 format
      └─ POST /projects/:id/exports
          ├─ 检查 active Run / Git commit / existing export
          ├─ 取得 project lock
          ├─ 校验整套页面完整性
          ├─ 冻结 source snapshot + source_digest
          ├─ 释放 project lock
          └─ 启动一次性 ExportOperation
              ├─ SSE 推送阶段与逐页进度
              ├─ PNG：render all → zip
              ├─ PDF：render all → raster PDF
              └─ HTML：rewrite/package → zip
                  └─ ready
                      └─ 用户点击 GET /download
                          ├─ 浏览器接收 Content-Disposition attachment
                          └─ 成功传输后删除 artifact、snapshot、operation
~~~

导出不是 Agent Run，不写入 Thread 时间线，不使用 Run scope，也不生成版本记录。它是一个短生命周期、项目级、一次性派生产物任务。

## 6. 冻结快照

### 6.1 快照内容

在持有项目锁期间，后端读取并复制：

1. Manifest 原始内容；
2. Outline 原始内容与展平后的稳定 slide ID 顺序；
3. Design 原始内容；
4. 每页现有 `index.html`；
5. 每页由冻结 Manifest、Outline、Design 计算的 Runtime Frame；
6. Base CSS；
7. 当前 Theme CSS；
8. 全部项目附件原图；
9. Export Runtime / Player 版本标识。

快照不需要 materialization 作为资格条件。可以读取其 hash 辅助诊断，但不能因 stale 或 unknown 阻止导出。

### 6.2 一致性校验

创建快照时必须一次性验证：

- Outline 至少有一页；
- 每个 Outline slide ID 都存在对应 HTML；
- 不存在 Outline 外目录页被误纳入；
- Theme 和 Base CSS 可读取；
- 所有附件路径位于项目附件白名单且原图存在；
- Runtime Frame 对每个 slide ID 都可构建；
- 画布严格为 1920×1080 / 16:9。

存在缺失 HTML 时返回结构化错误，包含全部缺失 slide ID、ordinal 和 title。不得创建后台任务后才逐页发现普通缺页。

### 6.3 source_digest

`source_digest` 使用规范序列和 SHA-256 计算，至少覆盖：

- Manifest、Outline、Design 字节；
- 有序 slide ID 列表；
- 每页 HTML 字节；
- 每页 Runtime Frame；
- Base CSS 与 Theme CSS；
- 每个附件相对路径和原图 hash；
- Export Runtime 版本。

该 digest 只存在于任务内存状态、日志和诊断事件，不写入交付包，也不对用户形成可管理的版本对象。

### 6.4 临时目录

使用受控目录：

~~~text
<projectRoot>/.runtime/exports/<exportID>/
├── snapshot/
├── work/
└── artifact/
~~~

`.runtime` 必须保持 Git ignore。所有路径使用后端生成的 export ID 和固定子目录，不接受客户端路径、文件名或输出目录。

清理条件：

- 下载响应成功传输；
- 任务失败；
- 用户强制离开并发出取消；
- 项目删除；
- 后端关闭；
- 兜底清理器发现超出短期 TTL 的孤立目录。

TTL 仅是异常兜底，不构成导出历史；默认值作为内部配置管理，不在 UI 暴露。

## 7. 一次性任务模型

### 7.1 状态

~~~text
accepted
  → running
      → ready
          → delivering
              → consumed
      → failed
      → canceled
~~~

- `accepted`：请求已通过入口校验，等待快照或执行。
- `running`：正在 snapshotting、rendering 或 packaging。
- `ready`：成品已生成，等待用户点击下载。
- `delivering`：正在向浏览器传输。
- `consumed`：HTTP 字节传输成功，立即清理；只短暂存在以完成事件通知。
- `failed`：整单失败，无可下载成品。
- `canceled`：页面离开、项目删除或服务关闭导致取消。

`ready` 不是自动下载。前端只能在用户点击“下载”后请求 artifact。

### 7.2 阶段与进度

运行阶段：

- `snapshotting`；
- `rendering`；
- `packaging`。

公开进度字段：

~~~json
{
  "phase": "rendering",
  "completed_pages": 4,
  "total_pages": 12,
  "current_slide_id": "sli_xxxxxxxx",
  "current_ordinal": 5,
  "warnings": []
}
~~~

PNG/PDF 在每页渲染完成后推进页面进度。HTML 在校验、重写并写入每页文件后推进页面进度。进度只能单调增加。

### 7.3 生命周期边界

- Operation 使用内存管理，不新增长期导出表或历史查询接口。
- 前端当前页面持有 export ID，并通过 SSE 获取状态。
- SSE 暂时断线时允许同一页面重连；刷新页面不恢复任务。
- 页面 `beforeunload` 使用 keepalive 请求尽力取消；后端 TTL 负责清理未收到取消的孤立任务。
- 后端进程重启时不恢复任务，启动清理遗留导出目录。
- 同项目第二次创建在前一任务未清理前返回 `EXPORT_ALREADY_ACTIVE`。

## 8. 后端设计

### 8.1 组件职责

建议新增：

~~~text
backend/internal/export/
├── manager.go          # 每项目单任务、生命周期、订阅与取消
├── snapshot.go         # 受锁快照、完整性验证、source_digest
├── html_package.go     # HTML 重写、播放器与 ZIP
├── png_package.go      # 整套渲染与 PNG ZIP
├── pdf_package.go      # PNG 到栅格 PDF
├── resources.go        # 附件和外部资源策略
├── filename.go         # 安全文件名
└── cleanup.go          # 成功、失败、离开和孤儿清理
~~~

`service.ExportService` 负责项目权限、active operation 检查、项目锁和 HTTP 层可见错误；底层 `export.Manager` 不读取请求参数中的任意路径。

导出复用 `NodeSlideRenderer` 的底层渲染接口，但不复用 Agent `render_slide` tool、evidence ledger 或 materialization 写入逻辑。

### 8.2 项目锁

创建顺序固定为：

1. 检查同项目 active export；
2. 检查 active Run 与 active Git commit；
3. 获取项目锁；
4. 在锁内重新检查互斥状态，避免 TOCTOU；
5. 完成完整性校验与快照复制；
6. 注册 operation；
7. 释放项目锁；
8. 异步执行格式生成。

不允许为整个 Chromium 渲染过程持有项目锁。

### 8.3 PNG 渲染

- Renderer 的 `project_dir` 指向冻结 snapshot，而不是仍在变化的项目目录。
- HTML、Base CSS、Theme CSS 和 Runtime Frame 均来自同一 snapshot。
- 沿用 Chromium 的网络阻断、font ready、双 RAF、静态动画状态、overflow/clipping 和资源诊断。
- 最大并发沿用 Renderer 当前全局限制，不为导出创建另一套无界 browser pool。
- 任何页面 diagnostics 含资源失败、页面错误或渲染失败时，任务停止发布；已生成 PNG 只作为内部临时数据并在失败后删除。

### 8.4 PDF 组装

先完成整套 PNG 渲染，再由 Render Worker 增加受控的 PDF 组装操作：

1. 创建只包含冻结 PNG 的内部打印文档；
2. 每页一个固定 16:9 page box；
3. 图片铺满页面，无 margin、header、footer；
4. 使用 Chromium `page.pdf` 输出单个 PDF；
5. 校验 PDF 页数等于 Outline 页面数后原子发布。

不得把原始页面 HTML 拼入同一个打印 DOM，避免跨页 CSS、ID 和脚本污染。

### 8.5 HTML 打包

对每页 HTML 执行确定性转换：

- 移除应用 API 形式的 Base CSS / Theme CSS 链接；
- 保留包内 `../assets/base.css` 与 `../assets/theme.css`，并在每页内联同一份样式作为 `file://` + sandbox 环境的可靠加载路径；
- 将静态 `/attachments/<id>/original.<ext>` 引用重写为原图 Data URL，同时注入仅映射已验证附件的轻量 Runtime，支持页面脚本动态设置附件 URL；附件原图仍完整保留在包内；
- 移除 DOM selection bridge；
- 保留页面本身的 inline style、inline script、data/blob URL 和普通超链接；
- 保留外部资源 URL，并把检测结果加入 warning；
- 拒绝指向附件白名单之外的项目本地绝对或相对资源。

每页由播放器放入独立 iframe：

~~~html
<iframe sandbox="allow-scripts" src="slides/001.html"></iframe>
~~~

不要加入 `allow-same-origin`、`allow-popups`、`allow-top-navigation` 或 `allow-downloads`。播放器拥有导航、全屏和公共装饰；页面脚本不能访问或替换播放器父文档。

播放器的页面列表直接内联在 `index.html` 或 `player.js`，不得在 `file://` 环境通过 `fetch(deck.json)` 获取，否则会引入本地文件 CORS 差异。

### 8.6 外部资源检测

静态检查覆盖：

- `script[src]`；
- `link[href]` 中的 stylesheet、preload、modulepreload；
- `img[src/srcset]`、`source[src/srcset]`、`video[poster/src]`；
- inline style 和 style block 中的 `url()`、`@import`；
- iframe、object、embed 等可加载文档的元素；
- CSS 中的远程字体 URL。

普通可点击超链接不算渲染资源。

PNG/PDF 即使静态检查未发现动态 URL，也继续依赖 Chromium route interception 阻止网络；实际外部请求进入失败 diagnostics。HTML 只记录 warning，不下载、不代理、不内联外部资源。

### 8.7 下载与一次性消费

- 下载文件只能通过 export ID 映射，不接受文件路径。
- 响应设置准确 MIME、`Content-Length`、`X-Content-Type-Options: nosniff` 和 `Content-Disposition: attachment`。
- HTML 与 PNG 文件名带格式后缀，避免两个 ZIP 混淆：
  - `<项目名>-<yyyyMMdd-HHmm>-html.zip`
  - `<项目名>-<yyyyMMdd-HHmm>-png.zip`
  - `<项目名>-<yyyyMMdd-HHmm>.pdf`
- 同一个 ready artifact 同时只允许一个 delivering 请求。
- `io.Copy` 成功且响应连接未报错后，删除 artifact 和 snapshot，并发出 consumed 事件。
- 传输中断时回到 ready，保留同一个一次性文件供当前模态层再次点击；兜底 TTL 最终清理。

## 9. HTTP API

### 9.1 创建导出

~~~http
POST /api/v1/projects/:id/exports
Content-Type: application/json
~~~

~~~json
{
  "format": "png",
  "client_request_id": "exp_req_xxxxxxxx"
}
~~~

`format` 只允许 `png | pdf | html`。页面范围、分辨率、字体和输出目录均不进入请求。

成功返回 `202 Accepted`：

~~~json
{
  "id": "exp_xxxxxxxx",
  "project_id": "pro_xxxxxxxx",
  "format": "png",
  "status": "accepted",
  "phase": "snapshotting",
  "completed_pages": 0,
  "total_pages": 12,
  "events_url": "/api/v1/exports/exp_xxxxxxxx/events"
}
~~~

创建阶段可返回：

- `EXPORT_RUN_ACTIVE`；
- `EXPORT_GIT_COMMIT_ACTIVE`；
- `EXPORT_ALREADY_ACTIVE`；
- `EXPORT_SLIDES_MISSING`；
- `EXPORT_THEME_UNAVAILABLE`；
- `EXPORT_RESOURCE_INVALID`。

`EXPORT_SLIDES_MISSING` 的 details 必须返回全部缺失页：

~~~json
{
  "code": "EXPORT_SLIDES_MISSING",
  "message": "有 2 页尚未生成，无法导出完整演示文稿。",
  "details": {
    "slides": [
      {"slide_id": "sli_a", "ordinal": 3, "title": "市场分析"},
      {"slide_id": "sli_b", "ordinal": 7, "title": "下一步"}
    ]
  }
}
~~~

### 9.2 读取当前任务

~~~http
GET /api/v1/exports/:id
~~~

只用于当前页面在 SSE 断线后读取最新状态，不提供项目级 list endpoint，不形成历史列表。任务 consumed、canceled 或清理后返回 `410 Gone`。

### 9.3 进度事件

~~~http
GET /api/v1/exports/:id/events
Accept: text/event-stream
~~~

事件类型：

- `export.progress`；
- `export.ready`；
- `export.delivery_started`；
- `export.download_failed`；
- `export.consumed`；
- `export.failed`；
- `export.canceled`。

事件包含递增 `seq`，SSE 支持 `Last-Event-ID`，使当前页面的短暂网络断线可以继续。`export.ready` 携带一次性 `download_url`、安全文件名、文件大小和 warnings。

### 9.4 下载

~~~http
GET /api/v1/exports/:id/download
~~~

- 只有 ready 状态允许下载。
- delivering 返回 `409 EXPORT_DOWNLOAD_IN_PROGRESS`。
- consumed 或已清理返回 `410 EXPORT_CONSUMED`。
- 传输失败回到 ready，并发送 `export.download_failed`。

### 9.5 页面离开取消

~~~http
DELETE /api/v1/exports/:id
~~~

该接口主要由 `beforeunload` 的 keepalive 请求和项目删除流程调用，不在生成中 UI 暴露取消按钮。ready 状态也可取消并删除尚未下载的文件。

## 10. 前端设计

### 10.1 组件与状态

建议新增：

~~~text
frontend/src/features/export/
├── ExportButton.tsx
├── ExportProgressDialog.tsx
├── ExportFailureList.tsx
└── exportPresentation.ts

frontend/src/api/exports.ts
frontend/src/stores/exportStore.ts
~~~

`exportStore` 只保存当前页面生命周期内的一个 project export，不持久化到 localStorage。它不能复用 Composer 的多页 scope，也不能进入 Run store。

### 10.2 ExportButton

- 放在 `PreviewWorkspace` 顶栏全屏入口旁。
- 菜单固定三项，不显示设置图标或二级面板。
- active Run、active Git commit 或同项目 active export 时按钮保持可见但禁用，并通过 tooltip 解释原因。
- 选择菜单项后立即调用创建 API；成功后打开 Progress Dialog。
- 缺页等创建失败直接打开终止态 Dialog，列出具体页面，不先显示虚假进度。

### 10.3 Progress Dialog

运行态：

- 无关闭按钮；
- 拦截 Escape 和 outside interaction；
- 显示格式名称、进度条、`已处理 x / y 页` 和当前阶段；
- warning 可显示但不阻塞 HTML；
- 不展示日志、技术堆栈或内部绝对路径。

ready 态：

- 进度为 100%；
- 显示安全文件名和大小；
- 主按钮为“下载”；
- 不自动触发下载；
- 下载传输中按钮禁用并显示“正在交给浏览器…”；
- 传输失败时恢复“再次下载”。

failed 态：

- 显示人类可读原因；
- 页面错误按 ordinal/title 列出；
- 不展示残缺产物；
- 提供“重新导出”和“关闭”；
- “重新导出”使用当前选择的格式重新 POST，并重新冻结项目。

### 10.4 离开保护

- accepted、running、ready、delivering 状态注册 `beforeunload`。
- 浏览器确认文案由平台控制，前端不能自定义。
- 用户确认离开后尽力发送 DELETE keepalive。
- 普通路由项目切换在应用内直接阻止，不弹出第二套自定义确认。
- failed、canceled、consumed 状态解除离开保护。

### 10.5 浏览器下载

用户点击后使用同源 download URL 和 `Content-Disposition`，不把大文件先读入 JavaScript Blob。前端保持 SSE 连接以接收 delivery/consumed 或 download_failed 状态；服务器确认传输后关闭 Dialog。

浏览器最终是否显示保存位置、保存到哪个目录以及本地磁盘写入结果不属于应用可控范围。产品文案使用“已交给浏览器下载”，不声称已经写入某个本地路径。

## 11. 错误与安全

### 11.1 公开错误

建议公开错误码：

| Code | 阶段 | 用户行为 |
| --- | --- | --- |
| EXPORT_RUN_ACTIVE | 创建 | 等 Agent 任务结束后重试 |
| EXPORT_GIT_COMMIT_ACTIVE | 创建 | 等提交结束后重试 |
| EXPORT_ALREADY_ACTIVE | 创建 | 返回当前进度 |
| EXPORT_SLIDES_MISSING | 创建 | 按列表补齐页面 |
| EXPORT_RESOURCE_INVALID | 快照/打包 | 修正项目资源 |
| EXPORT_EXTERNAL_RESOURCE | PNG/PDF | 将外部资源转为项目附件 |
| EXPORT_RENDER_FAILED | 渲染 | 查看失败页并重新导出 |
| EXPORT_PACKAGE_FAILED | 打包 | 重新导出 |
| EXPORT_DOWNLOAD_IN_PROGRESS | 下载 | 等待当前传输 |
| EXPORT_DOWNLOAD_FAILED | 下载 | 再次点击下载 |
| EXPORT_CONSUMED | 下载 | 重新导出 |

公开错误不得包含 work root、临时目录、系统用户名、原始堆栈或外部资源中的敏感 query/fragment。外部 URL 诊断只展示安全化 origin/path。

### 11.2 路径与 ZIP 安全

- ZIP entry 名称全部由服务端生成或严格安全化。
- 禁止绝对路径、`..`、NUL、驱动器前缀和符号链接 entry。
- 附件复制继续使用 artifactfs/project sandbox 规则。
- 解压后的所有播放资源必须位于包根目录内。
- 下载接口不接受 `file_id`、文件名或路径参数。

### 11.3 HTML 安全

- 每页 iframe 只开放 `allow-scripts`。
- 页面不能获得 `allow-same-origin` 后与父页面脚本组合逃逸 sandbox。
- 播放器自身不执行来自 Manifest、Outline、标题或页面内容的拼接脚本；文本使用 DOM textContent 或构建期安全转义。
- 外部资源保留是已确认的交付策略，不代表应用后端在导出时访问这些 URL。

## 12. 项目删除与服务关闭

- 删除项目之前取消关联 export 并等待临时写入停止，再删除项目目录。
- Export worker 发布 artifact 前再次确认 operation 仍有效，不能在项目删除后重建 authoring root。
- 服务关闭时停止接收新导出、取消运行任务、关闭 SSE，并清理所有一次性 export 目录。
- 后端启动时清理遗留 `.runtime/exports/` 目录，不尝试恢复任务。

## 13. 测试计划

### 13.1 后端单元测试

- Outline 顺序是唯一导出顺序。
- 任意缺页返回全部缺失页面并且不创建 operation。
- stale/unknown 页面只要 HTML 存在即可导出。
- active Run、Git commit 和同项目 export 正确互斥。
- 快照后修改 Manifest、Outline、Design、HTML、Theme 或附件不影响本次 source digest 与产物。
- source digest 对任一快照输入变化敏感且计算稳定。
- 安全文件名处理控制字符、路径字符、空标题和重名。
- ZIP entry 无路径逃逸。
- HTML 重写后不残留 `/api/v1/runtime`、`/api/v1/themes` 或附件 API URL。
- HTML 包含全部附件原图，不包含 thumbnail/meta/spec/outline/design/materialization。
- 外部普通链接允许；PNG/PDF 外部渲染资源失败；HTML 外部资源产生 warning。
- 取消、失败、成功下载和 TTL 均清理临时目录。
- 下载只允许 ready、单并发，并在成功传输后消费 artifact。

### 13.2 Render Worker 集成测试

- PNG 数量、顺序和尺寸为 1920×1080。
- Runtime chrome 与现有截图结果一致。
- PDF 页数等于 Outline 页面数，page box 为 16:9 且无 margin。
- PDF 每页使用相同 PNG，不重新解释页面 HTML。
- 外部网络请求被阻止并映射到具体 slide ID。
- 页面脚本、资源失败和 Chromium 崩溃不会发布部分产物。
- 取消后 Renderer 可继续服务下一次导出。

### 13.3 前端测试

- 顶栏只有一个导出入口，菜单包含三种格式。
- 选择格式立即创建，没有配置或预览页。
- active Run/Git commit/export 时按钮禁用并说明原因。
- 创建成功后 Dialog 不可 Escape、遮罩或按钮关闭。
- SSE 进度单调更新页数、阶段和进度条。
- 缺页与运行失败展示页面列表且无下载按钮。
- ready 状态不会自动下载，必须点击“下载”。
- delivering 状态按钮禁用；失败后可再次点击。
- 重新导出发送新的 client request ID。
- beforeunload 只在非终止态注册，并尽力发送取消。
- consumed 后关闭 Dialog 并清空 export store。

### 13.4 HTML 包验收

在当前 Chrome 与 Edge 中验证：

1. ZIP 解压后双击 `index.html`，无需本地 HTTP server；
2. 所有页面可按顺序展示；
3. 按钮、方向键、Space、Home、End 可导航；
4. 页码、章节和公共装饰与编辑器/PNG 一致；
5. 全屏可进入和退出；
6. 项目附件离线加载；
7. 页面 inline JavaScript 可运行但不能控制父播放器；
8. 包内没有应用 API 地址、开发源文件、缩略图或附件元数据；
9. 外部资源存在时明确提示需要联网；
10. 系统字体不同导致的 fallback 差异符合已声明限制。

## 14. 实施顺序

1. 建立 export model、内存 Manager、错误码与项目级互斥。
2. 实现受锁快照、完整性验证、source digest 和临时目录清理。
3. 抽出可验证的 Runtime Frame/chrome 契约，避免导出产生第三套解释。
4. 接入整套 PNG 渲染与 ZIP 原子发布。
5. 增加栅格 PDF 组装与页数校验。
6. 实现 HTML 重写、独立播放器、附件复制与 ZIP。
7. 增加创建、状态、SSE、下载和离开取消 API。
8. 实现单按钮菜单、不可关闭进度 Dialog 和手动浏览器下载。
9. 完成后端、前端、Chromium 和离线 HTML 集成测试。

不保留旧导出协议或历史 501 接口兼容层；当前没有有效导出协议，直接以本文档为唯一新契约。

## 15. 非目标

- 不导出 PPTX。
- 不支持单页、选定页、章节或自定义范围。
- 不支持 A4、讲义、打印备注、质量或分辨率选项。
- 不支持透明 PNG、2× 图片或矢量 PDF。
- 不支持自动下载。
- 不支持导出历史、长期产物、刷新恢复或后台任务中心。
- 不支持字体文件打包或跨机器像素级字体一致。
- 不下载或代理外部资源。
- 不支持任意项目文件作为通用素材。
- 不实现讲稿、概览、Presenter View 或统一动画生命周期。
- 不把导出加入 Agent Thread、Run scope、版本历史或 Git 提交流程。
