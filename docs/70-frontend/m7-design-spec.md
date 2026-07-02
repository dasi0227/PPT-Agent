# M7 前端完整设计规格

## 1. 目标与范围

M7 的目标是交付一个可真实体验 M0-M6 后端能力的前端工作台：用户可以在同一页面完成项目切换、PPT 在线预览、页目录导航、Agent 对话交互、Run 事件查看、HITL 输入和最终产物识别。

本阶段不是普通 Dashboard，也不是只展示静态文本的聊天页。它是面向「PPT Coding Agent」的专业工作台，核心体验是：左侧理解 Deck，中央验证最终视觉，右侧与 Agent 协作。

## 2. 需求整合清单

### 2.1 已确认框架

- 保持三栏工作台：左侧目录区、中间预览区、右侧 Agent 对话区。
- 顶部采用类似 JetBrains IDE / Microsoft Office 的多项目标签页，不使用下拉式项目切换。
- 点击项目标签必须立即切换当前项目上下文：目录、预览、线程、Run 状态、输入上下文同步更新。
- 左侧目录区保持「上方项目标题 + 下方垂直页面列表」结构。
- 左侧不再放 scope/mode 切换组件，只做信息展示和页面导航。
- 移除灰色格子胶片孔装饰。
- 中央 PPT 预览区域保持干净，移除中间内容区域重阴影。
- 右侧 Agent 面板避免蓝色科技感，改为专业 AI Agent 工具风格。

### 2.2 Agent 对话能力

- Agent 输出必须支持 Markdown 渲染：代码块、表格、列表、链接、图片等。
- 工具调用必须以卡片展示，不能只显示纯文本。
- 深度思考 / thought 必须以可折叠卡片展示。
- 计划模式卡片需要设计并预留组件能力；第一版不强制后端真实发 `plan` 事件。
- UI 必须区分中间产物与最终交付结果。
- 不同工作模式需要有区分色，但第一版只用于标签、边界和状态点，不做大面积色块。

### 2.3 后端能力边界

当前后端没有独立 Workspace 实体，也没有 `/workspaces` API。前端 M7 实现的是多 Project 标签切换，数据来自：

- `GET /projects`
- `POST /projects`
- `GET /projects/{id}`
- `GET /projects/{id}/slides`
- `GET /projects/{id}/threads`
- `POST /projects/{id}/threads`
- `POST /threads/{id}/runs`
- `GET /runs/{id}/events`
- `POST /runs/{id}/input`
- `DELETE /runs/{id}`

## 3. 页面结构与组件划分

### 3.1 顶部栏 `TopProjectTabs`

职责：

- 展示产品标识。
- 横向展示多个已打开项目标签。
- 显示当前后端连接 / Run 状态。
- 提供高频入口：资产、历史、演示。

组件：

- `AppHeader`
- `ProjectTabs`
- `ProjectTab`
- `RunStatusBadge`
- `HeaderActions`

交互：

- 点击项目标签后，立即更新 `activeProjectId`。
- 切换项目时不展示加载动画；若数据未就绪，局部区域显示 skeleton。
- 标签过多时横向滚动，当前标签保持可见。

### 3.2 左侧目录区 `DeckNavigator`

职责：

- 展示当前项目标题、状态、主题、页数。
- 展示 slide 垂直列表。
- 点击 slide 后切换当前页，并同步预览 iframe。

组件：

- `DeckSummary`
- `SlideList`
- `SlideListItem`

非目标：

- 不放 scope/mode 切换。
- 不放复杂编辑控件。

### 3.3 中央预览区 `PreviewWorkspace`

职责：

- 用 sandbox iframe 渲染 PPT。
- 提供主预览、总览、单页预览视图切换。
- 支持键盘翻页、总览点击跳转、深链定位。
- 编辑完成后只刷新目标页，不重载整套 iframe。

组件：

- `PreviewToolbar`
- `PreviewFrame`
- `PreviewFooter`
- `OverviewGrid`
- `usePostMessage`

约束：

- iframe 必须设置 sandbox。
- 翻页使用 postMessage `goto`，不能通过改 iframe `src` 翻页。
- 主视图和总览必须使用同一主题、字体和 16:9 视口。

### 3.4 右侧 Agent 面板 `AgentPanel`

职责：

- 展示当前线程的对话和 Run 事件。
- 支持 Markdown、工具卡、思考卡、计划卡、产物卡、最终结果卡。
- 支持 needs_input 的回答。
- 支持用户输入并创建 Run。

组件：

- `AgentPanel`
- `RunSummary`
- `Timeline`
- `MarkdownMessage`
- `ThoughtCard`
- `ToolCallCard`
- `PlanCard`
- `ArtifactCard`
- `FinalResultCard`
- `NeedsInputCard`
- `CommandComposer`

## 4. Agent Timeline 数据设计

前端以 SSE 事件为事实源，将事件转换为 UI Timeline。

### 4.1 输入事件

来自 [../40-api/sse-events.md](../40-api/sse-events.md)：

| SSE event | UI 表现 |
|---|---|
| `run.started` | 更新 RunSummary |
| `thought` | `ThoughtCard`，默认折叠 |
| `tool_call` | 创建/更新 `ToolCallCard` 为 running |
| `tool_result` | 按 `call_id` 合并到对应工具卡 |
| `artifact` | 若紧随工具调用，则挂到工具卡；否则展示 `ArtifactCard` |
| `progress` | 更新 RunSummary 或页进度 |
| `token` | 聚合为 Markdown 流式消息 |
| `info` | Markdown 消息 |
| `needs_input` | `NeedsInputCard` |
| `done` | `FinalResultCard` |
| `error` | 错误卡 |

### 4.2 工具卡归并规则

- `tool_call.call_id` 是工具卡主键。
- `tool_result.call_id` 必须归并到对应工具卡。
- `artifact` 若在工具卡后到达且业务上属于该工具结果，可挂到最近未挂 artifact 的工具卡；否则作为独立中间产物卡展示。
- 工具卡状态：
  - `running`：已收到 `tool_call`，未收到结果。
  - `success`：收到 `tool_result.ok=true`。
  - `failed`：收到 `tool_result.ok=false` 或 error。

### 4.3 计划卡协议预留

第一版前端实现 PlanCard 组件，但后端不强制发 plan 事件。未来扩展协议：

```text
event: plan
data: { id, title, steps:[{ id, title, status, detail? }] }

event: plan.update
data: { id, step_id, status, detail? }
```

状态枚举：

- `pending`
- `in_progress`
- `completed`
- `failed`
- `skipped`

## 5. 状态管理设计

推荐 Zustand store：

### 5.1 `projectStore`

状态：

- `projects`
- `activeProjectId`
- `slidesByProjectId`
- `threadsByProjectId`
- `loadingProjects`

动作：

- `loadProjects()`
- `selectProject(projectId)`
- `loadProjectSlides(projectId)`
- `loadProjectThreads(projectId)`
- `createProject(params)`

### 5.2 `deckStore`

状态：

- `currentPage`
- `overviewOpen`
- `previewMode: "main" | "overview" | "single"`
- `iframeReady`
- `loadCount`

动作：

- `setCurrentPage(index)`
- `goNext()`
- `goPrev()`
- `enterOverview()`
- `exitOverview()`
- `reloadSlide(index)`

### 5.3 `runStore`

状态：

- `activeRunId`
- `status`
- `mode`
- `scope`
- `events`
- `timelineItems`
- `pendingInput`
- `eventSourceState`

动作：

- `createRun(threadId, payload)`
- `subscribeRun(runId, lastEventId?)`
- `appendEvent(event)`
- `replyNeedsInput(runId, replyTo, content)`
- `cancelRun(runId)`

### 5.4 `uiStore`

状态：

- `rightPanelOpen`
- `leftPanelCollapsed`
- `themeDensity`
- `activeModeColor`

## 6. API 调用设计

### 6.1 项目与线程

启动工作台：

1. `GET /projects`
2. 若为空，显示创建项目入口。
3. 选中第一个项目或上次项目。
4. 并行加载：
   - `GET /projects/{id}/slides`
   - `GET /projects/{id}/threads`

项目标签切换：

1. 更新 `activeProjectId`。
2. 从缓存取 slides/threads。
3. 缓存缺失时局部加载，不阻塞标签高亮。
4. 取消旧项目的非当前 SSE 订阅，避免事件串线。

### 6.2 Run 与 SSE

创建 Run：

```http
POST /api/v1/threads/{id}/runs
```

payload 必须包含：

- `kind`
- `instruction`
- `scope`
- `mode`
- `page_index`，`/current` 必填

订阅：

```http
GET /api/v1/runs/{id}/events
```

断线重连带：

```http
Last-Event-ID: <last_seq>
```

HITL：

```http
POST /api/v1/runs/{id}/input
{ "content": "...", "reply_to": "evt_x" }
```

## 7. UI 与视觉规范

### 7.1 总体风格

关键词：专业、克制、工作台、低饱和、高信息密度。

避免：

- 大面积高饱和蓝色科技感。
- 重阴影和玻璃拟态堆叠。
- 过多胶囊按钮。
- 把工具日志和最终结果混成同一种文本气泡。

### 7.2 色彩

基础：

- background: warm graphite / cool gray
- surface: warm paper
- border: graphite 12%-24%
- text: graphite 900 / 600 / 400

模式色：

| 模式 / Scope | 用途 | 色彩方向 |
|---|---|---|
| normal / edit | 默认编辑 | graphite + sage |
| talk | 只说不做 | indigo |
| ask | 等待澄清 | amber |
| repo | 资产仓库 | teal |
| overview | 全局修改 | violet |
| error | 错误 | muted red |
| final | 最终交付 | green |

模式色只用于状态点、标签、卡片左边界和小面积强调。

### 7.3 卡片层级

- MarkdownMessage：普通文本卡。
- ThoughtCard：折叠，低强调。
- ToolCallCard：中等强调，有状态边界。
- ArtifactCard：中间产物，弱强调。
- FinalResultCard：强强调，带主要操作入口。
- NeedsInputCard：amber 强提示，必须清楚展示可回答选项。

### 7.4 Markdown 样式

支持：

- 表格横向滚动。
- 代码块等宽字体、深浅适配。
- 图片最大宽度 100%，懒加载。
- 链接新窗口打开。
- 长文本保留段落节奏，避免贴边。

建议依赖：

- `react-markdown`
- `remark-gfm`

代码高亮第一版可不引入额外库，后续再接 Shiki。

## 8. 响应式设计

桌面宽屏：

- 顶部项目标签横向展示。
- 三栏布局：左 280px，中间自适应，右 380px。

中等屏幕：

- 左侧可折叠。
- 右侧可切换为抽屉或下方区域。
- 项目标签横向滚动。

移动端：

- 顶部保留项目标签。
- 主体改为分段 Tabs：目录 / 预览 / Agent。
- 预览保持 16:9，宽度优先。

## 9. 性能优化

- SSE 事件进入 store 后做增量 reducer，避免每次全量重算。
- token 流式输出节流合并，例如 50-100ms 批量刷新一次。
- Markdown 渲染对稳定消息使用 memo。
- slide 缩略图懒加载。
- 总览缩略图按需渲染，避免一次性创建大量 iframe。
- 项目切换使用缓存，标签高亮立即生效，数据区局部刷新。
- EventSource 离开项目或 Run 结束后及时关闭。

## 10. 验收标准

### P0

- 项目标签可展示多个 Project，点击标签立即切换上下文。
- 预览区使用 sandbox iframe，翻页不改变 iframe src。
- 左侧目录只展示项目摘要和页列表。
- 右侧 Agent 输出支持 Markdown。
- `thought` 渲染为可折叠思考卡。
- `tool_call` + `tool_result` 合并为工具卡。
- `artifact` 与 `done` 明确区分为中间产物和最终交付。
- `/talk` 模式不展示产物卡。
- `needs_input` 可提交回答到 `/runs/{id}/input`。

### P1

- 计划卡组件完成，但真实后端计划事件可后续接入。
- token 流式 Markdown 平滑刷新。
- 总览缩略图与主预览视觉一致。
- 左右栏支持折叠。

### 验证命令

```bash
cd frontend
pnpm test
pnpm tsc
pnpm build
```

浏览器 e2e 后续补充：

- 项目标签切换。
- iframe 翻页无重载。
- 工具卡状态流转。
- needs_input 回答。
