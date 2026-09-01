# 给前端 Agent 的 M7 开发 Prompt

你现在接手 ByteDance/PPT_Agent 项目的 M7 前端开发任务。你的目标是基于现有后端 M0-M6 能力，完成一个可真实体验的 AI PPT Agent 前端工作台。

## 必读文档

先阅读以下文档，不要跳过：

1. `docs/70-frontend/m7-design-spec.md`
2. `docs/70-frontend/prototypes/m7-mvp-core-page-v3.html`
3. `docs/20-architecture/frontend-structure.md`
4. `docs/20-architecture/preview-mechanism.md`
5. `docs/40-api/sse-events.md`
6. `docs/40-api/openapi.yaml`
7. `docs/50-agent/commands/README.md`
8. `docs/50-agent/modes.md`

如果文档存在冲突，以架构/API 事实源为准；M7 UI 设计以 `docs/70-frontend/m7-design-spec.md` 为准。

## 项目背景

这是一个 AI PPT 生成与编辑产品。后端已经具备 Project / Thread / Run / SSE / Slide / Asset 等能力。前端 M7 需要让用户能够真正体验：

- 多 Project 切换。
- PPT 在线预览。
- 页目录导航。
- Agent 对话与指令输入。
- Run 事件流。
- 工具调用、思考、产物、最终结果等卡片化展示。

前端 SPA 使用 React + Vite + TypeScript。当前 `frontend/` 只有脚手架，不要假设已有复杂 UI。

## 总体功能概述

实现一个三栏工作台：

```text
顶部：产品标识 + 多 Project 标签页 + Run 状态 + 高频入口

左侧：当前 Project 摘要 + Slide 垂直列表
中间：sandbox iframe PPT 预览 + 翻页 / 总览 / 单页预览
右侧：Agent 面板 + Markdown 输出 + 工具/思考/计划/产物/最终结果卡片 + 输入区
```

## 开发范围

### 必须完成

1. 多 Project 顶部标签页
   - 数据来自 `GET /projects`。
   - 点击标签立即切换 `activeProjectId`。
   - 切换后加载对应 slides / threads。
   - 标签过多时横向滚动。

2. 左侧目录区
   - 展示项目标题、主题、状态、页数。
   - 展示 slide 垂直列表。
   - 点击 slide 切换当前页。
   - 不放 scope/mode 切换组件。

3. 中央预览区
   - 使用 sandbox iframe。
   - 翻页通过 postMessage，不通过改 iframe src。
   - 支持主预览、总览、单页预览的 UI 结构。
   - 保持 16:9 视觉比例。
   - 移除重阴影，保持干净专业。

4. 右侧 Agent 面板
   - Markdown 渲染。
   - 工具调用卡。
   - 思考卡，默认折叠。
   - 计划卡组件，第一版可接前端类型/模拟数据，后端真实事件后续扩展。
   - 中间产物卡。
   - 最终交付卡。
   - needs_input 卡和回答入口。

5. SSE 对接
   - 订阅 `GET /runs/{id}/events`。
   - 支持 Last-Event-ID 重连。
   - 消费现有事件：`run.started`、`thought`、`tool_call`、`tool_result`、`progress`、`token`、`artifact`、`needs_input`、`info`、`done`、`error`。
   - 按 `call_id` 合并 `tool_call` 和 `tool_result`。

6. 指令输入
   - 支持创建 Run：`POST /threads/{id}/runs`。
   - 默认 scope 为 `/current`，必须携带当前 `page_index`。
   - needs_input 回答调用 `POST /runs/{id}/input`，带 `reply_to`。

7. 视觉系统
   - 使用低饱和专业配色。
   - 避免廉价科技感和大面积高饱和蓝。
   - 模式色只用于小面积状态识别。

### 不要做

- 不实现独立 Workspace 概念。当前后端只有多 Project。
- 不修改后端 Run/Harness 语义。
- 不强行新增真实 `plan` / `plan.update` 后端事件。
- 不把 Agent 输出做成纯文本列表。
- 不让 `/talk` 模式展示 artifact。
- 不通过直接操作 iframe DOM 来翻页。

## 推荐前端目录结构

```text
frontend/src/
├── api/
│   ├── client.ts
│   ├── projects.ts
│   ├── runs.ts
│   ├── slides.ts
│   └── sse.ts
├── stores/
│   ├── projectStore.ts
│   ├── deckStore.ts
│   ├── runStore.ts
│   └── uiStore.ts
├── features/
│   ├── workspace/
│   │   ├── AppShell.tsx
│   │   ├── AppHeader.tsx
│   │   └── ProjectTabs.tsx
│   ├── deck/
│   │   ├── DeckNavigator.tsx
│   │   └── SlideList.tsx
│   ├── viewer/
│   │   ├── PreviewFrame.tsx
│   │   ├── PreviewToolbar.tsx
│   │   ├── OverviewGrid.tsx
│   │   └── usePostMessage.ts
│   └── agent/
│       ├── AgentPanel.tsx
│       ├── MarkdownMessage.tsx
│       ├── Timeline.tsx
│       ├── ThoughtCard.tsx
│       ├── ToolCallCard.tsx
│       ├── PlanCard.tsx
│       ├── ArtifactCard.tsx
│       ├── FinalResultCard.tsx
│       ├── NeedsInputCard.tsx
│       └── eventReducer.ts
├── components/
└── lib/
```

## 核心数据设计

### TimelineItem

前端将 SSE 事件转换为 timeline item：

```ts
type TimelineItem =
  | MarkdownMessageItem
  | ThoughtItem
  | ToolCallItem
  | PlanItem
  | ArtifactItem
  | FinalResultItem
  | NeedsInputItem
  | ErrorItem;
```

### 工具卡归并

```ts
tool_call   -> create ToolCallItem(status="running")
tool_result -> update ToolCallItem by call_id
artifact    -> attach to nearest related ToolCallItem or render ArtifactItem
```

### 产物区分

```ts
artifact -> delivery="intermediate"
done     -> delivery="final"
```

## UI 规范

### 配色

- 背景：warm graphite / cool gray。
- 面板：warm paper。
- 文本：graphite。
- 边框：低透明 graphite。
- 默认模式：sage。
- talk：indigo。
- ask：amber。
- repo：teal。
- overview：violet。
- error：muted red。
- final：green。

模式色只用在状态点、标签、卡片左边界，不要大面积铺色。

### 卡片

- MarkdownMessage：普通 Agent 文本。
- ThoughtCard：默认折叠，标题为“执行思路”。
- ToolCallCard：展示工具名、参数摘要、状态、observation、artifact。
- PlanCard：展示步骤和状态，支持折叠。
- ArtifactCard：标签“中间产物”。
- FinalResultCard：标签“最终交付”，视觉层级最高。
- NeedsInputCard：amber 提醒，支持选择项和文本回答。

## 交互要求

- Project 标签点击后立即高亮，不等待网络完成。
- 数据未加载时只在局部区域显示 skeleton。
- 当前页变化要同步：
  - 左侧 slide active
  - 中间 iframe
  - 创建 Run 时的 `page_index`
- Run 开始后：
  - 右侧显示 RunSummary。
  - SSE 事件进入 Timeline。
  - 终态 done/error 后关闭 EventSource。
- needs_input 出现后：
  - 输入区切换为回答模式。
  - 提交时调用 `/runs/{id}/input`。

## API 要求

至少实现：

- `GET /projects`
- `POST /projects`
- `GET /projects/{id}/slides`
- `GET /projects/{id}/threads`
- `POST /projects/{id}/threads`
- `POST /threads/{id}/runs`
- `GET /runs/{id}/events`
- `POST /runs/{id}/input`
- `DELETE /runs/{id}`

API 类型必须与 `docs/40-api/openapi.yaml` 对齐。

## Markdown 要求

使用 `react-markdown` + `remark-gfm` 或等价方案。

必须支持：

- 表格。
- 列表。
- 代码块。
- 链接。
- 图片。
- 行内代码。

链接新窗口打开，图片限制最大宽度并懒加载。

## 测试要求

必须补测试：

1. event reducer
   - tool_call + tool_result 合并。
   - artifact 和 done 区分。
   - thought 转 ThoughtCard。

2. Markdown 渲染
   - 表格。
   - 代码块。
   - 链接。
   - 图片。

3. 组件渲染
   - ToolCallCard running/success/failed。
   - ThoughtCard 折叠。
   - PlanCard 状态。
   - FinalResultCard。
   - NeedsInputCard。

4. App 级测试
   - Project 标签切换。
   - 当前页切换。
   - Run 事件展示。

## 验收标准

执行：

```bash
cd frontend
pnpm test
pnpm tsc
pnpm build
```

若引入浏览器 e2e，再补：

- 项目标签切换。
- iframe 翻页无重载。
- needs_input 提交。
- SSE 事件驱动工具卡状态变化。

## 交付说明

完成后输出：

- 修改文件清单。
- 实现了哪些 M7 模块。
- 新增依赖。
- 新增测试。
- 验证命令和结果。
- 未完成或后续扩展项，尤其是后端 `plan` / `plan.update` 事件。
