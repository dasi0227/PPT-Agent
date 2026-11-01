# Workspace Shell Uplift Design (v6)

> **Date**: 2026-07-13
> **Scope**: 前端 (`frontend/`) 全量 + 后端 (`backend/internal/httpapi`, `service`, `store/sqlite`) 少量补丁
> **Commit prefix**: `feat(ux-consistency):`
> **Naming**: 本轮统一命名为 **v6**（前 5 期已交付）
> **Focus**: 输入快捷键 / finish 交付气泡化 / ⋯ 菜单化 / 项目开启方式 / 通用弹层 / 面板拖拽伸缩

---

## 1. 背景与目标

Studio 前端在 5 期 UX 迭代后仍有若干"感受不到位"的位置：

1. `CommandComposer` 用 `Enter` 提交，误触率高 — 用户期望 `⌘/Ctrl + Enter` 才发送。
2. `finish` 工具当前既产出 `tool_call/tool_result` 卡，又产出 `FinalResultCard`，非结构化 done 用重卡包一句总结话读起来"生硬"，缺乏对话感。
3. `ProjectTabs` / `ThreadTabs` 上"删除 / 关闭"分成两个 icon 按钮，交互噪声大，也不支持"重命名"。
4. `ProjectTabs` 的 `+` 直接建草稿 project，用户无法"从已有项目里挑一个继续开发"；且首次进 App 会自动选中第一个项目 + 后台拉 slides / threads，与"启动即空、按需打开"的心智不符。
5. 缺一套统一的弹层组件；现在散布着 `window.confirm` 与手写下拉，视觉/无障碍/键盘导航参差不齐。
6. 主布局左/中/右三栏宽度写死（`w-[280px]` / `w-[380px]`），无法拖拽也不方便临时隐藏。

### 目标（DoD）

- **R1** 快捷键：`Enter` 换行，`⌘+Enter`（macOS）/ `Ctrl+Enter`（Win/Linux）发送；输入区底部提示文案自适应平台。
- **R2** 交付气泡化：非结构化 `done` 以 **左对齐 Markdown 气泡** 呈现；结构化 `done`（含 `slide_count`）保留 `FinalResultCard`；隐藏 `tool=finish` 的 `tool_call`/`tool_result` 卡片以避免双重展示。
- **R3** ⋯ 菜单化：`ProjectTabs` / `ThreadTabs` 每一项只保留一个 `MoreHorizontal` 图标，点击弹出 `DropdownMenu`；项目菜单为 `重命名 / 关闭项目 / 删除项目`，会话菜单为 `重命名 / 删除会话`。所有"删除 / 重命名"改走通用 Modal。
- **R4** 项目开启改造：
  - `+` 改为弹 `ProjectPickerModal`，二选一：`新建项目` / `打开已有项目`。
  - `打开已有项目` 展示后端 `GET /projects` 的列表（**不是** OS 目录选择器），支持搜索、项目卡片；选中一个即"打开为 Tab"。
  - 首次进入 App 不再自动打开首个项目；`ProjectTabs` 为空态时中间区域显示 `WorkspaceEmptyState`（引导用户点 `+`）。
  - 移除 project / thread 双侧的草稿态与相关 flush 逻辑。
- **R5** 通用弹层：新增 `frontend/src/components/ui/` 目录，基于 Radix Primitives 封装：
  - `dialog.tsx`（`Dialog` / `DialogContent` / `DialogHeader` 等 headless 组合）
  - `dropdown-menu.tsx`（同上）
  - `modal-confirm.tsx`（危险 / 常规二次确认）
  - `modal-form.tsx`（表单式弹层，本轮驱动"重命名"）
  - `modal-picker.tsx`（列表选择弹层，本轮驱动"打开已有项目"）
  - 全部使用现有 Tailwind tokens（`bg-surface` / `border-border` / `text-text-{400,600,900}` / `mode-*`）
- **R6** 面板伸缩：主布局改为 `react-resizable-panels`；左/右两栏各有 `[min, max]` 宽度区间；左右两栏均可整体隐藏；两个 `PanelToggleButton`（`PanelLeft`/`PanelRight`）放到 `ProjectTabs` 右侧；宽度与隐藏态持久化到 `localStorage`。

### 非目标（YAGNI）

- 项目 / 会话的多选、拖拽排序、批量删除。
- 弹层动画曲线定制、暗色模式适配（已由主题 tokens 承载）。
- 项目从任意 OS 目录导入 / 打开（不做 File System Access API）。
- 面板上下拖拽、四象限自由布局。
- 后端历史项目自动清理任务。

---

## 2. 总体架构

```
┌─────────────────────────────────────────────────────────────────┐
│ ProjectTabs (顶栏)                                              │
│ ├─ M7 Studio                                                    │
│ ├─ [Tab: Project ⋯]   [Tab: Project ⋯]   [+]                    │  ← 空态时只有 [+]
│ └─ (右侧)  [PanelLeftToggle]  [PanelRightToggle]                │
├─────────────────────────────────────────────────────────────────┤
│ PanelGroup  (react-resizable-panels, direction=horizontal)      │
│                                                                 │
│ ┌── Panel ────┐ Handle ┌── Panel ─────────────┐ Handle ┌── Panel│
│ │ DeckNavigator│         │ PreviewWorkspace    │         │ Agent │
│ │ (左, 可隐藏) │         │ (中, 不可隐藏)      │         │ Panel │
│ │              │         │                     │         │ (右, │
│ │              │         │                     │         │ 可隐)│
│ └──────────────┘        └──────────────────────┘        └──────┘│
└─────────────────────────────────────────────────────────────────┘

弹层层 (Portal → document.body)：
- ProjectPickerModal          触发：ProjectTabs [+]
- ProjectMenu (Dropdown)      触发：ProjectTabs Tab ⋯
- ThreadMenu (Dropdown)       触发：ThreadTabs   Tab ⋯
- RenameProjectModal          由 ProjectMenu 触发
- ConfirmDeleteProjectModal   由 ProjectMenu 触发
- CloseProjectModal (可选)    由 ProjectMenu 触发
- RenameThreadModal           由 ThreadMenu  触发
- ConfirmDeleteThreadModal    由 ThreadMenu  触发
```

### 组件分层与文件清单

```
frontend/src/
├── components/ui/                        ← 新增：主题化 Radix 包装层
│   ├── dialog.tsx
│   ├── dropdown-menu.tsx
│   ├── modal-confirm.tsx
│   ├── modal-form.tsx
│   ├── modal-picker.tsx
│   └── modal-picker.test.tsx
│
├── features/workspace/
│   ├── AppShell.tsx                      ← 改造：react-resizable-panels
│   ├── ProjectTabs.tsx                   ← 改造：⋯ 菜单 + PanelToggleButtons
│   ├── ProjectMenu.tsx                   ← 新增
│   ├── ProjectPickerModal.tsx            ← 新增
│   ├── OpenExistingProjectModal.tsx      ← 新增
│   ├── PanelToggleButtons.tsx            ← 新增
│   ├── NewProjectHint.tsx                ← 新增（new-pending 时中央引导）
│   └── WorkspaceEmptyState.tsx           ← 新增
│
├── features/agent/
│   ├── CommandComposer.tsx               ← 改造：⌘/Ctrl+Enter
│   ├── ThreadTabs.tsx                    ← 改造：⋯ 菜单
│   ├── ThreadMenu.tsx                    ← 新增
│   ├── Timeline.tsx                      ← 改造：隐藏 finish tool_call、气泡渲染
│   ├── FinalResultCard.tsx               ← 改造：仅结构化 result 渲染
│   ├── FinishBubble.tsx                  ← 新增（承载非结构化 done）
│   └── eventReducer.ts                   ← 改造：隐藏 finish tool_call
│
├── stores/
│   ├── uiStore.ts                        ← 改造：宽度 + 隐藏 + 持久化
│   ├── projectStore.ts                   ← 改造：去草稿态 + rename + 首次不选中
│   └── threadStore.ts                    ← 改造：去草稿态 + rename
│
├── lib/
│   ├── platform.ts                       ← 新增：isMac()
│   └── draft.ts                          ← 删除（草稿态移除）
│
└── api/
    ├── projects.ts                       ← 改造：新增 patch()
    └── threads.ts                        ← 改造：新增 patch()

backend/internal/
├── httpapi/
│   ├── project_handler.go                ← 改造：新增 Patch handler
│   ├── thread_handler.go                 ← 改造：新增 Patch handler
│   └── router.go                         ← 改造：注册 PATCH 路由
├── service/
│   ├── project.go                        ← 改造：新增 RenameProject
│   └── thread.go                         ← 改造：新增 RenameThread
└── store/sqlite/
    └── run_store.go                      ← 改造：新增 UpdateProjectTitle / UpdateThreadTitle
```

---

## 3. 需求 1：发送快捷键（R1）

### 3.1 行为

| 按键 | 行为 |
|---|---|
| `Enter` | **换行**（在 `textarea` 里插入 `\n`；原生行为，无需 preventDefault） |
| `Shift + Enter` | 换行（保持一致，与许多聊天工具一致） |
| `⌘ + Enter` （macOS）| **发送**（`e.preventDefault()` + `handleSubmit()`） |
| `Ctrl + Enter` （非 macOS）| **发送**（同上） |

### 3.2 平台判定

`frontend/src/lib/platform.ts`：

```ts
export function isMac(): boolean {
  if (typeof navigator === 'undefined') return false;
  return /Mac|iPhone|iPad|iPod/.test(navigator.platform);
}

export function submitShortcutLabel(): string {
  return isMac() ? '⌘ + Enter' : 'Ctrl + Enter';
}
```

### 3.3 `CommandComposer` 改造点

- `onKeyDown`：`e.key === 'Enter' && (isMac() ? e.metaKey : e.ctrlKey) → 发送`。移除现有的 `Enter && !shiftKey → 发送` 逻辑。
- 底部提示条追加 `<kbd>{submitShortcutLabel()}</kbd> 发送`。
- IME composition 期间不响应：加 `e.nativeEvent.isComposing` guard，避免中文选词时误发。

### 3.4 单测

`CommandComposer.test.tsx` 追加：

- Enter 只是换行，不触发 submit（值应变为原文 + `\n`）。
- Meta + Enter（mac 模拟）触发 submit。
- Ctrl + Enter（win 模拟）触发 submit。
- IME 输入中的 Meta + Enter 不触发。

---

## 4. 需求 2：finish 交付气泡化（R2）

### 4.1 现状

- `finish` 工具触发时后端会同时发：`tool_call {tool: "finish"}` → `tool_result` → `done {result}`。
- 前端 Timeline 会渲染 `ToolCallCard`（finish 卡）+ `FinalResultCard`（final 卡）。
- `done.result` 有两种形态：
  - **结构化**：`{slide_count, theme, warnings, signature, design_spec_ref}` — 用于从零全量生成。
  - **非结构化**：字符串 或 `{summary: string}` — 用于 edit / talk / outline / overview / repo。

### 4.2 目标

- 隐藏 `tool=finish` 的 `tool_call` 与 `tool_result`（**不再产生一张"finish 工具卡"**）。
- 非结构化 `done` 改渲染为 `FinishBubble`（**左对齐 Markdown 气泡**，样式与已有的 `MarkdownMessage` 气泡略作强调，例如底部一行 `已完成` 徽标）。
- 结构化 `done` 保留现 `FinalResultCard`。

### 4.3 实现

**`eventReducer.ts`：**

在 `case 'tool_call'` 中，若 `event.data.tool === 'finish'`，构造 item 时打标 `hiddenFromTimeline: true` 塞进 state（不外泄到渲染树，但仍在 state 中保留以便 `tool_result` 事件能通过 `call_id` 匹配到并更新 status）。`case 'tool_result'` 无需特殊分支：仍按 `call_id` 匹配对应 `tool_call`，若该 tool_call 已带 `hiddenFromTimeline`，更新后依然被过滤。这样 reducer 保持纯函数、无外部状态。

`ToolCallItem` 类型增加可选字段：

```ts
export interface ToolCallItem extends BaseTimelineItem {
  // ...existing
  hiddenFromTimeline?: boolean;   // finish 工具专用
}
```

**`Timeline.tsx`：**

- `case 'tool_call'`：`if (item.hiddenFromTimeline) return null;`
- `case 'final_result'`：
  - 结构化（`typeof result === 'object' && typeof result.slide_count === 'number'`）→ 现 `FinalResultCard`。
  - 否则 → `FinishBubble`。
- `AGENT_CONTENT_TYPES` 保持不变（`final_result` 仍算 agent 内容，撤下 ThinkingBubble）。

**`FinishBubble.tsx` 新组件：**

- 左对齐（`justify-start`）；宽度上限 85%。
- 卡片背景 `bg-surface`，边框 `border-border`。
- 内容用 `MarkdownMessage` 渲染 `result.summary || (typeof result === 'string' ? result : '已完成')`。
- 右下角一小行 `已完成` 徽标（`text-mode-final`）。

### 4.4 单测

- `Cards.test.tsx`（或 `Timeline.test.tsx`）追加：
  - `tool_call {tool: "finish"}` 不出现在渲染树里。
  - `done {result: "..."}` 渲染为 `FinishBubble` + Markdown。
  - `done {result: {slide_count: 12, ...}}` 渲染为 `FinalResultCard`。

---

## 5. 需求 3：⋯ 菜单化 + 重命名（R3）

### 5.1 交互

**项目 Tab（`ProjectTabs`）：**

- Tab 右侧 `hover` 时显示一个 `MoreHorizontal`（14px）图标。点击弹 `ProjectMenu`。
- 菜单项：
  1. `重命名` → 弹 `RenameProjectModal`（下方 §5.4）
  2. `关闭项目` → 从"打开态"移除（**不删数据**），焦点跳到相邻 tab
  3. `删除项目` → 弹 `ConfirmDeleteProjectModal`，二次确认后 DELETE

**会话 Tab（`ThreadTabs`）：**

- Tab 右侧 `hover` 时显示 `MoreHorizontal`。点击弹 `ThreadMenu`。
- 菜单项：
  1. `重命名` → 弹 `RenameThreadModal`
  2. `删除会话` → 弹 `ConfirmDeleteThreadModal`

> ✅ 由 R4 决定：草稿态取消，`ThreadTabs` 不再有 `× 关闭标签` 逻辑；一个 thread 一旦创建就存在于后端，"关闭"由项目层 tab 机制处理，不下发到 thread 层。

### 5.2 Dropdown 组件

使用 `@radix-ui/react-dropdown-menu`，包装为 `components/ui/dropdown-menu.tsx`：

```tsx
export const DropdownMenu = DropdownMenuPrimitive.Root;
export const DropdownMenuTrigger = DropdownMenuPrimitive.Trigger;
// Content, Item, Separator, Label 全部主题化：
// Content:  className="min-w-[160px] bg-surface border border-border rounded-md shadow-md py-1 z-50"
// Item:     className="w-full px-3 py-1.5 text-xs text-text-600 hover:bg-black/5 cursor-pointer outline-none data-[highlighted]:bg-black/5"
// Item.destructive: 追加 "text-mode-error hover:bg-mode-error/10"
```

### 5.3 `ProjectMenu.tsx` / `ThreadMenu.tsx`

薄组件：接收 `project` / `thread` 与 `onRename` / `onClose` / `onDelete` 回调；渲染 `DropdownMenuTrigger` + `DropdownMenuContent` + 若干 `Item`。触发器由父组件（Tab）传入（`asChild` 模式）。

### 5.4 重命名 Modal

**`RenameProjectModal` / `RenameThreadModal`**：使用 `components/ui/modal-form.tsx`：

- 标题：`重命名项目` / `重命名会话`
- 单行 `Input`：默认值 = 当前 title
- 校验：`title.trim().length >= 1 && length <= 60`；否则 `确认` 按钮 disabled。
- `确认` → 调 `projectStore.renameProject(id, title)` / `threadStore.renameThread(id, title)`（下方 §5.6）。
- `Enter` = 确认；`Esc` = 取消（Radix Dialog 默认）。
- 提交中 → `Confirm` 按钮 spinner；失败 → 内嵌错误文案。

### 5.5 删除 Modal

**`ConfirmDeleteProjectModal` / `ConfirmDeleteThreadModal`**：使用 `components/ui/modal-confirm.tsx`：

- 标题：`删除项目「{title}」？` / `删除会话「{title}」？`
- 正文：`此操作不可撤销，历史记录将永久丢失。`（thread 版）；`此操作不可撤销，项目的所有会话、切片、历史记录都会被移除。`（project 版）
- 取消：`取消`（默认 focus）
- 确认：`删除`（`variant="danger"`，`text-white bg-mode-error`）
- Confirm click → 调 store delete → 成功后关闭。

### 5.6 Store API 变更

**`projectStore.ts`：**

```ts
interface ProjectState {
  // ...
  renameProject: (projectId: string, title: string) => Promise<void>;
  closeProject: (projectId: string) => void;   // 仅从"打开态"移除
  // deleteProject 已存在，保留
}
```

- `renameProject`：调用 `projectsApi.patch(id, {title})`；成功后 in-place 更新 `projects[]` 对应项的 `title` 与 `updated_at`。
- `closeProject`：新增；将该 project 从 `openProjectIds`（新加的持久化字段，见 §7.2）里移除；若 `activeProjectId === id`，切到相邻打开的 project 或 `null`。
- 移除 `createDraftProject` / `flushProject` / `discardDraftProject`（草稿态取消）。
- 新增 `createProject(topic, brief, slide_count, language)`：直接 POST；成功后加入 `projects[]` + 加入 `openProjectIds` + `selectProject(newId)`。
- 新增 `openProject(id: string)`：把已存在项目加入 `openProjectIds`（打开态）+ selectProject；用于 `OpenExistingProjectModal`。
- `loadProjects` 修改：**不再自动 `selectProject(projects[0].id)`**；仅拉列表，供 picker 使用。

**`threadStore.ts`：**

```ts
interface ThreadState {
  // ...
  renameThread: (projectId: string, threadId: string, title: string) => Promise<void>;
}
```

- 移除 `createDraftThread` / `flushThread` / `nextUntitledName`（草稿态取消）。
- `ensureActiveThread` 简化：若有 active → 直接返回；否则若 `threadsByProjectId[projectId].length > 0` → 用第一个；否则 **直接** POST 建 thread 并设为 active（title 留空 `""`，由后端 `CreateThread` 决定默认标题）。
- 新增 `createThread(projectId, title?)`：直接调 `threadsApi.create`，加入 `threadsByProjectId` + `openThreadIdsByProjectId` + 设为 active。
- `renameThread`：`threadsApi.patch(threadId, {title})` → in-place 更新。

### 5.7 后端 PATCH 接口（Rename）

**路由（`router.go`）：**

```go
v1.PATCH("/projects/:id", r.project.Patch)
v1.PATCH("/threads/:id",  r.thread.Patch)
```

**Handler（`project_handler.go`）：**

```go
type patchProjectRequest struct {
    Title *string `json:"title"`
}

func (h *ProjectHandler) Patch(c *gin.Context) {
    var req patchProjectRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        AbortWithError(c, ErrBadRequest("invalid request body"))
        return
    }
    if req.Title == nil {
        AbortWithError(c, ErrBadRequest("no fields to update"))
        return
    }
    title := strings.TrimSpace(*req.Title)
    if title == "" || utf8.RuneCountInString(title) > 60 {
        AbortWithError(c, ErrBadRequest("title length must be 1..60"))
        return
    }
    p, err := h.svc.RenameProject(c.Request.Context(), c.Param("id"), title)
    switch {
    case err == nil:
        c.JSON(http.StatusOK, toProjectResponse(p))
    case errors.Is(err, run.ErrRunNotFound):
        AbortWithError(c, ErrNotFound("project not found"))
    default:
        AbortWithError(c, ErrInternal(err.Error()))
    }
}
```

`thread_handler.go` 完全同构；`title` 上限沿用 60。

**Service（`service/project.go`）：**

```go
func (svc *ProjectService) RenameProject(ctx context.Context, id, title string) (model.Project, error) {
    p, err := svc.store.GetProject(ctx, id)
    if err != nil {
        return model.Project{}, err
    }
    p.Title = title
    p.UpdatedAt = time.Now().Unix()
    if err := svc.store.UpdateProjectTitle(ctx, p.ID, p.Title, p.UpdatedAt); err != nil {
        return model.Project{}, err
    }
    return p, nil
}
```

同构 `ThreadService.RenameThread`。

**Store（`store/sqlite/run_store.go`）：**

```go
func (s *Store) UpdateProjectTitle(ctx context.Context, id, title string, updatedAt int64) error {
    _, err := s.db.ExecContext(ctx,
        "UPDATE projects SET title=?, updated_at=? WHERE id=?", title, updatedAt, id)
    return err
}
func (s *Store) UpdateThreadTitle(ctx context.Context, id, title string, updatedAt int64) error {
    _, err := s.db.ExecContext(ctx,
        "UPDATE threads SET title=?, updated_at=? WHERE id=?", title, updatedAt, id)
    return err
}
```

**e2e 单测：** 至少覆盖成功、空 title、超长 title、不存在的 id 四种。

### 5.8 前端 API

`api/projects.ts`：

```ts
patch: (id: string, patch: {title?: string}) =>
  fetchClient<Project>(`/projects/${id}`, { method: 'PATCH', body: JSON.stringify(patch) }),
```

`api/threads.ts` 同构。

---

## 6. 需求 4：项目开启方式（R4） + 去草稿态

### 6.1 交互

- `ProjectTabs` 的 `+` 触发 `ProjectPickerModal`：
  - 主内容有两张大卡片：
    - **新建项目**：icon `FilePlus`，标题 `新建项目`，副文本 `从零开始一个新的演示文稿`。
    - **打开已有项目**：icon `FolderOpen`，标题 `打开已有项目`，副文本 `从最近的项目里继续`。
  - 点击 `新建项目`：**关闭当前 modal，直接**在 `ProjectTabs` 上插入一个"临时占位 Tab"（**非草稿态**，标题 `新建中…`，无 id）— 具体处理见 §6.2。
  - 点击 `打开已有项目`：**切到 `OpenExistingProjectModal`**（同一层 modal 内 route 切换，不新开一层）。
- `OpenExistingProjectModal`：
  - 顶部搜索框（按 `title` 模糊匹配）。
  - 列表：项目卡片（`title` / `slide_count?` / `updated_at 相对时间`）。
  - 空态：`还没有历史项目，去新建一个`（次要按钮 `返回`）。
  - 点击项目卡 → `projectStore.openProject(id)` → close modal。

### 6.2 「新建项目」的落地：**Composer 引导，不预建后端 project**

关键决定：**"新建项目" 不再前端软创建 draft，也不立即 POST 后端**。改为：

- 用户点击 `新建项目` → `ProjectPickerModal` 关闭 → `ProjectTabs` 上出现一个 `Tab (title="新建中…", id="new-pending")`（仅一份，若已存在则直接 `selectProject("new-pending")`，不叠加也不重建）；`activeProjectId = "new-pending"`；中间区域展示"输入首条消息即开始"引导态（**不复用** `WorkspaceEmptyState` — 那是"无任何 activeProject"的空态；这里已选中 pending tab，中央应显示 `NewProjectHint`：`在右侧输入你想做的 PPT 主题，⌘/Ctrl+Enter 发送即可开始`）。
- Composer 状态：`activeProjectId === "new-pending"` 视为草稿态，允许 `send`；`handleSubmit` 走：
  1. `projectStore.createProject({topic: raw, brief, slide_count, language})` → 返回真实 project。
  2. 把 `new-pending` tab 替换为真实 project tab（`replacePendingTab(realId)`），并 `selectProject(realId)`。
  3. `threadStore.ensureActiveThread(realId)` → 拿真实 thread id。
  4. `runStore.createRun(threadId, payload)`。
- 若用户在 `new-pending` 状态下切换到别的 tab / 关闭 tab，`new-pending` 直接从 `openProjectIds` 里移除（零残留、无后端调用）。

> 说明：这个 `new-pending` 与之前的 `draft_*` 草稿态**在语义上不同**。草稿态是"完整的 project 对象副本，含 draft:true 标记，全 store 各分支要处理"；`new-pending` 是"UI 层单一占位符（sentinel id），只影响 `openProjectIds` 与 `activeProjectId`，不进 `projects[]` 主数组"。

对应 `projectStore` 字段：

```ts
interface ProjectState {
  projects: Project[];                   // 全部来自后端，无 draft
  openProjectIds: string[];              // 打开态（含 "new-pending"）；持久化
  activeProjectId: string | null;
  pendingNewProject: boolean;            // 是否有 new-pending tab
  slidesByProjectId: Record<string, Slide[]>;
  loadingProjects: boolean;

  loadProjects: () => Promise<void>;              // 不自动 select
  openProject: (id: string) => void;              // 已有 → 打开
  createProject: (opts) => Promise<Project>;      // 直接 POST；不入 openProjectIds（外部决定）
  startPendingNewProject: () => void;             // 打开 new-pending 占位 tab
  finalizePendingNewProject: (realId: string) => void;
  cancelPendingNewProject: () => void;
  closeProject: (id: string) => void;
  renameProject: (id: string, title: string) => Promise<void>;
  deleteProject: (id: string) => Promise<void>;
  selectProject: (id: string) => void;
}
```

### 6.3 首次进入 App

- `AppShell` 挂载时 `loadProjects()`（拉取列表供 picker 使用），但**不 `selectProject`**。
- `openProjectIds` 从 `localStorage` 恢复；过滤掉列表里已不存在的 id（防止后端删除后残留）；如果结果为空，`activeProjectId = null`。
- 中间区域 & 右栏在 `activeProjectId === null` 时展示 `WorkspaceEmptyState`：

```
┌───────────────────────────────────┐
│                                   │
│         [大 logo]                 │
│                                   │
│   Welcome back                    │
│   点击顶栏「+」新建或打开项目     │
│                                   │
│   [ + 新建 / 打开项目 ]           │
│                                   │
└───────────────────────────────────┘
```

按钮点击 = 打开 `ProjectPickerModal`（等价于顶栏 `+`）。

### 6.4 去草稿态：删除清单

- `frontend/src/lib/draft.ts` — 整文件删除。
- `projectStore`：删除 `createDraftProject` / `flushProject` / `discardDraftProject` / `isDraftId` 分支；`Project.draft?: boolean` 字段从 `api/types.ts` 移除。
- `threadStore`：删除 `draftThreadsByProjectId` / `createDraftThread` / `flushThread` / `nextUntitledName`；`Thread.draft?: boolean` 字段移除；`displayThreads` 直接返回 `threadsByProjectId[projectId]`。
- `ProjectTabs` / `ThreadTabs` / `CommandComposer` 中所有 `isDraftId(...)` / `draft:true` 逻辑删除。
- 测试：`projectStore.test.ts`、`threadStore.test.ts`、`runStore.multithread.test.ts` 中相关用例改写或删除。

### 6.5 用户流程（关键 E2E）

**Flow A — 全新用户第一次进 App：**

1. 打开 App → `ProjectTabs` 只有 `[+]`，中间是 `WorkspaceEmptyState`。
2. 点 `+` → `ProjectPickerModal`。
3. 选 `新建项目` → modal 关闭 → 出现 `new-pending` tab（标题"新建中…"）→ Composer 可输入。
4. 输入 `给投资人讲我们的 AI 产品，8 页` → Enter 换行；`⌘+Enter` 发送。
5. Composer 内部 `createProject` → 拿到真实 id → 替换 pending tab → `ensureActiveThread` → `createRun` → SSE 开始。

**Flow B — 老用户点 `+` 想继续之前项目：**

1. `+` → `ProjectPickerModal` → `打开已有项目` → 搜索 or 直接选中 → 卡片点击。
2. `openProject(id)` → tab 出现在顶栏 → 拉 slides + threads → 中间 & 右栏渲染。

**Flow C — 重命名 / 删除：**

- 项目 tab 上 hover → `⋯` → 选 `重命名` → 表单 modal → 提交 → 顶栏 tab 标题实时更新。
- `删除项目` → 二次确认 modal → DELETE → tab 消失 → 若为 active，切到相邻 openProject 或 null。

---

## 7. 需求 5：通用弹层（R5）

### 7.1 依赖新增

```json
"@radix-ui/react-dialog": "^1.1.6",
"@radix-ui/react-dropdown-menu": "^2.1.6"
```

（版本号以执行时最新 minor 为准；本 spec 数字仅供参考。）

### 7.2 `components/ui/dialog.tsx`

薄封装，导出：

- `Dialog`（Root）
- `DialogTrigger`
- `DialogContent`（内含 `Overlay` + `Content` + 关闭按钮；Portal 到 body）
  - 主题：`fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 z-50`
  - 内容盒：`bg-surface rounded-lg shadow-xl border border-border-strong min-w-[360px] max-w-[560px] p-6`
  - Overlay：`fixed inset-0 z-40 bg-black/40 backdrop-blur-[2px]`
- `DialogHeader` / `DialogTitle` / `DialogDescription`
- `DialogFooter`（右对齐按钮组）
- `DialogClose`

### 7.3 `components/ui/modal-confirm.tsx`

```tsx
interface ConfirmModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: React.ReactNode;
  confirmLabel?: string;    // 默认 '确认'
  cancelLabel?: string;     // 默认 '取消'
  variant?: 'default' | 'danger';   // 影响 Confirm 按钮配色
  onConfirm: () => void | Promise<void>;
}
```

- `Confirm` 支持 `Promise`：点击后 disabled + spinner，直到 resolve/reject。
- 失败时不自动关闭，弹一个内嵌错误行 `无法完成，请稍后重试`（可通过 prop 覆盖）。

### 7.4 `components/ui/modal-form.tsx`

```tsx
interface FormModalProps<T> {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  initialValue: T;
  validate: (value: T) => string | null;  // 返回错误文案 or null
  onSubmit: (value: T) => Promise<void>;
  renderField: (value: T, setValue: (v: T) => void, error: string | null) => React.ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
}
```

`RenameProjectModal` 用它包一个单 Input；未来能扩展为多字段表单。

### 7.5 `components/ui/modal-picker.tsx`

用于 `打开已有项目`：

```tsx
interface PickerModalProps<T> {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  items: T[];
  keyOf: (item: T) => string;
  searchOf: (item: T) => string;                  // 用于过滤
  renderItem: (item: T) => React.ReactNode;
  onPick: (item: T) => void;
  emptyState?: React.ReactNode;
}
```

内部维护搜索框（受控）与 `↑/↓` 键盘导航（借助 Radix DropdownMenu 的 arrow-nav 无法用在 Dialog 里，需要手写 focus 环形逻辑；参见 §11 验收）。

### 7.6 `components/ui/dropdown-menu.tsx`

- 只暴露 `DropdownMenu` / `DropdownMenuTrigger` / `DropdownMenuContent` / `DropdownMenuItem` / `DropdownMenuSeparator` / `DropdownMenuLabel`。
- `DropdownMenuItem` 支持 `destructive?: boolean`，加红色主题。
- 内容位置：`side="bottom" align="end" sideOffset={4}`；碰撞时 Radix 自动翻转。

### 7.7 主题 tokens 引用规范

**MUST**：一律用 `bg-surface / bg-background / border-border / border-border-strong / text-text-{400,600,900} / mode-normal / mode-ask / mode-error / mode-final`；**禁止**引入新 hex。

---

## 8. 需求 6：面板拖拽伸缩 + 隐藏（R6）

### 8.1 依赖新增

```json
"react-resizable-panels": "^2.1.4"
```

### 8.2 `AppShell` 改造

```tsx
<PanelGroup direction="horizontal" autoSaveId="workspace-shell-v6">
  {!leftPanelHidden && (
    <>
      <Panel id="left" order={1} defaultSize={22} minSize={16} maxSize={32} collapsible={false}>
        <DeckNavigator />
      </Panel>
      <PanelResizeHandle className="w-[3px] bg-border hover:bg-mode-normal transition-colors" />
    </>
  )}
  <Panel id="center" order={2} minSize={30}>
    <PreviewWorkspace />
  </Panel>
  {!rightPanelHidden && (
    <>
      <PanelResizeHandle className="w-[3px] bg-border hover:bg-mode-normal transition-colors" />
      <Panel id="right" order={3} defaultSize={28} minSize={20} maxSize={40} collapsible={false}>
        <AgentPanel />
      </Panel>
    </>
  )}
</PanelGroup>
```

- `defaultSize` / `minSize` / `maxSize` 单位是百分比（相对 `PanelGroup` 宽度）。
- 起始值：左 22% / 中剩 / 右 28%；对 1440 屏近似 316 / 720 / 404 px，稍宽于现在的 280 / auto / 380。
- `autoSaveId` 由 `react-resizable-panels` 内建 localStorage 持久化拖拽比例。

### 8.3 隐藏面板

- `useUIStore` 追加 `leftPanelHidden` / `rightPanelHidden`（**独立于宽度**）。
- Hidden 时把对应 `<Panel>` 与其 `<PanelResizeHandle>` **不渲染**（而不是 width=0）；展开时恢复。这样 `react-resizable-panels` 的 autoSave 只在"两侧都可见"状态下累积比例，不会被 0-width 污染。
- 展开时使用**上次可见比例**（由 `autoSaveId` 自动恢复）。

### 8.4 顶栏切换按钮

`PanelToggleButtons.tsx`：

```tsx
<div className="flex items-center gap-1 ml-auto pr-2">
  <button aria-label={leftPanelHidden ? '展开左侧目录' : '隐藏左侧目录'}
          onClick={toggleLeftPanel}
          className="p-1.5 rounded hover:bg-black/5 text-text-600">
    <PanelLeft className="w-4 h-4" />          {/* lucide icon */}
  </button>
  <button aria-label={rightPanelHidden ? '展开右侧对话' : '隐藏右侧对话'}
          onClick={toggleRightPanel}
          className="p-1.5 rounded hover:bg-black/5 text-text-600">
    <PanelRight className="w-4 h-4" />
  </button>
</div>
```

放到 `ProjectTabs` 的最右侧（在项目 tab 列表之后、`ml-auto` 靠右）。

### 8.5 持久化

`useUIStore` 改用 zustand `persist` middleware，仅持久化面板隐藏态：

```ts
import { persist } from 'zustand/middleware';

export const useUIStore = create<UIState>()(
  persist(
    (set) => ({
      leftPanelHidden: false,
      rightPanelHidden: false,
      // ... 其它字段
      toggleLeftPanel: () => set((s) => ({ leftPanelHidden: !s.leftPanelHidden })),
      toggleRightPanel: () => set((s) => ({ rightPanelHidden: !s.rightPanelHidden })),
    }),
    { name: 'ppt-agent-ui-v6', partialize: (s) => ({ leftPanelHidden: s.leftPanelHidden, rightPanelHidden: s.rightPanelHidden }) },
  ),
);
```

`projectStore` 单独 persist：

```ts
persist(
  storeCreator,
  { name: 'ppt-agent-project-v6', partialize: (s) => ({ openProjectIds: s.openProjectIds, activeProjectId: s.activeProjectId }) },
)
```

（活跃 project 也持久化，让"关掉再回来"体验连贯；但 `activeProjectId` 若已不在 `projects[]` 里，`loadProjects` 完成后回落到 `openProjectIds` 头元素或 `null`。`new-pending` 不持久化：`partialize` 前需先把 `new-pending` 从 `openProjectIds` 里剔除。）

宽度比例由 `react-resizable-panels` 的 `autoSaveId` 自行 localStorage 持久化，key 为 `react-resizable-panels:workspace-shell-v6`；无需 zustand 承接。

### 8.6 中间面板"不可隐藏"

- 中央 `PreviewWorkspace` 始终可见。若两侧都隐藏，中央面板占满 100%。
- 中央面板本身不放拖拽把手（拖拽只在两条邻界上）。

---

## 9. 数据流

### 9.1 打开项目

```
User click Project card in OpenExistingProjectModal
     │
     ├─▶ projectStore.openProject(id)
     │     ├─ ensure id ∈ openProjectIds
     │     ├─ selectProject(id)
     │     │     ├─ close previous project's SSE (closePreviousProjectSessions)
     │     │     ├─ loadProjectSlides(id)   [projectsApi.getSlides]
     │     │     └─ threadStore.loadThreads(id)  [threadsApi.list]
     │     └─ persist openProjectIds + activeProjectId
     │
     └─▶ Right panel: 若 threadsByProjectId[id] 非空，ensureActiveThread 选中首个；否则 keep null（Composer 会在首发时建 thread）
```

### 9.2 重命名项目

```
Rename click in ProjectMenu
     │
     ├─▶ open RenameProjectModal (initialValue = current title)
     │
     └─▶ submit
           ├─ projectStore.renameProject(id, newTitle)
           │     ├─ projectsApi.patch(id, {title: newTitle})
           │     ├─ set state: projects[i].title = newTitle
           │     └─ resolve()
           └─ modal 关闭
```

### 9.3 finish → 气泡

```
SSE stream:
  event: tool_call        {call_id: c1, tool: "finish", args: {summary:"..."}}
  event: tool_result      {call_id: c1, ok: true}
  event: done             {result: {summary: "..."}}
              or
  event: done             {result: {slide_count: 12, ...}}

reduceSSEEvent:
  ── tool_call: 若 tool==="finish" → push {hiddenFromTimeline:true}
  ── tool_result: 更新 status; hiddenFromTimeline 继承
  ── done: 无变化（照旧 push final_result）

Timeline render:
  ── tool_call.hiddenFromTimeline → skip
  ── final_result:
       structured?  →  FinalResultCard
       else         →  FinishBubble  ← 左对齐 markdown 气泡
```

---

## 10. 错误处理

| 场景 | 处理 |
|---|---|
| Rename title 为空 / 超长 | 前端 validate → 按钮 disabled；同时后端返 400 `INVALID_TITLE`，兜底文案 `名称长度需在 1–60 之间` |
| Rename 后端 5xx | Modal 内嵌错误 `重命名失败，请稍后重试`；输入框保留 |
| Delete 后端 5xx | Modal 内嵌错误；不自动关闭 |
| PATCH 冲突（并发编辑） | 视为覆盖写；后端不做 optimistic lock；前端最终一致，无需特殊处理 |
| `openProject(id)` 时 id 不在 `projects[]`（比如后端删了） | `loadProjects()` 后自愈；`openProjectIds` 里过滤掉不存在的 id |
| `new-pending` 提交前用户切走 tab | `cancelPendingNewProject()` 清理；再点 `+ 新建项目` 会重建 pending |
| Radix Dialog 在 SSR / test 环境找不到 Portal target | `jsdom` 已内建 `document`；`setupTests.ts` 里 mock `matchMedia` 与 `IntersectionObserver`（如果 Radix 内部需要） |
| Panel autoSave 数据被人手编辑破坏 | `react-resizable-panels` 有 fallback 到 defaultSize；无需干预 |
| Enter/⌘Enter 在 IME 组合期误发 | `isComposing` guard |
| `finish` tool_call 隐藏后 progress 计数偏差 | progress 事件独立于 tool_call；不受影响 |

---

## 11. 验收清单（DoD）

### 功能

- [ ] **R1** `Enter` 换行；`⌘+Enter`(Mac) / `Ctrl+Enter`(Win/Linux) 发送；IME 期间不发。
- [ ] **R2** 全量生成场景 → `FinalResultCard`；edit/talk/outline/overview/repo 场景 → `FinishBubble` 左对齐 Markdown 气泡；Timeline 中不再出现 `tool=finish` 工具卡。
- [ ] **R3a** 项目 tab hover 显示 `⋯`；菜单三项：重命名 / 关闭项目 / 删除项目；重命名走 Modal；删除二次确认。
- [ ] **R3b** 会话 tab hover 显示 `⋯`；菜单两项：重命名 / 删除会话；同上走 Modal。
- [ ] **R4** 顶栏 `+` 弹 `ProjectPickerModal`；`新建项目` 走 `new-pending` 占位，首发消息时才 POST；`打开已有项目` 走列表选择；首次进 App 不自动选中项目、中间显示 `WorkspaceEmptyState`。
- [ ] **R5** 全部 Modal / Dropdown 走 `components/ui/`；无 `window.confirm`；旧手写下拉（`ThreadTabs` 的"历史"下拉）迁移到 `DropdownMenu`。
- [ ] **R6** 左右两侧宽度可拖拽、区间生效；两侧可整体隐藏；`ProjectTabs` 右侧有两个 toggle icon；宽度与隐藏态跨刷新保持。

### 无障碍

- [ ] 所有交互式元素有 `aria-label` 或可读文本。
- [ ] Modal 打开时焦点自动落到内部第一个可 focus 元素；`Tab` 循环在 Modal 内部（Radix 提供）；ESC 关闭；关闭后焦点回落到 trigger。
- [ ] DropdownMenu 支持 `↑/↓/Home/End/Type-ahead/Enter/Esc`（Radix 提供）。
- [ ] PickerModal 搜索框输入时可用 `↑/↓` 遍历列表，`Enter` 选中；`Esc` 关闭。

### 后端

- [ ] `PATCH /projects/:id` 支持 `{title}`；空 / 超长 → 400；不存在 → 404；成功 → 200 + 完整 project。
- [ ] `PATCH /threads/:id` 同上。
- [ ] `service.RenameProject` / `service.RenameThread` 单测覆盖。
- [ ] `Store.UpdateProjectTitle` / `Store.UpdateThreadTitle` 单测覆盖。
- [ ] `internal/httpapi` e2e 追加 rename 用例。

### 前端测试

- [ ] `CommandComposer.test.tsx` 追加 4 个用例（Enter 换行 / Mac Meta+Enter / Win Ctrl+Enter / IME 抑制）。
- [ ] `Timeline.test.tsx` 追加：finish tool_call 隐藏、结构化 done → FinalResultCard、非结构化 done → FinishBubble。
- [ ] `ProjectTabs.test.tsx`（新）：⋯ 菜单渲染 / 触发重命名 modal / 触发删除 modal / 触发关闭。
- [ ] `ThreadTabs.test.tsx`（新）：⋯ 菜单渲染 / 触发重命名 / 触发删除。
- [ ] `ProjectPickerModal.test.tsx`：新建 vs 打开已有分支。
- [ ] `OpenExistingProjectModal.test.tsx`：搜索过滤、空态、选中。
- [ ] `projectStore.test.ts` / `threadStore.test.ts`：去 draft 用例改写；新增 rename、new-pending 生命周期用例。
- [ ] `AppShell.test.tsx`（新）：默认宽度 / 拖拽后持久化 / 隐藏切换 / 三栏组合渲染。

### 提交规范

- [ ] Commit prefix：`feat(ux-consistency):`
- [ ] Pangu 排版（中英文 / 数字之间有空格）
- [ ] 不修改用户已定稿的中文文案（本 spec 中的文案属新写，可修改）

---

## 12. 里程碑（供实施 agent 拆解）

1. **M1 — 基础设施**
   - 新增 npm 依赖：`@radix-ui/react-dialog`、`@radix-ui/react-dropdown-menu`、`react-resizable-panels`
   - 新增 `components/ui/*` 五件套（含单测）
   - 新增 `lib/platform.ts`
2. **M2 — 后端 rename**
   - Store / Service / Handler / Router / e2e
3. **M3 — 前端 store 重构**
   - `projectStore` / `threadStore` 去草稿态 + rename + new-pending
   - `uiStore` 持久化
4. **M4 — 布局 & 顶栏**
   - `AppShell` 三栏拖拽 + 隐藏
   - `ProjectTabs` ⋯ 菜单 + PanelToggleButtons
   - `WorkspaceEmptyState`
5. **M5 — Picker & Rename & Delete Modal 接线**
   - `ProjectPickerModal` / `OpenExistingProjectModal`
   - `ProjectMenu` / `ThreadMenu`
   - Rename / Delete modal 接入
6. **M6 — 交付气泡 & 快捷键**
   - `CommandComposer` 快捷键与提示
   - `eventReducer` `finish` 隐藏
   - `FinishBubble` + `Timeline` 分支
7. **M7 — 单测 & 验收**
   - 补齐所有测试；`npm run tsc && npm run lint && npm test` 全绿
   - 手动跑 Flow A / B / C 三条 E2E

---

## 13. 附录 — 与既有约定的对齐

- **UX 一致性 commit 前缀**：本轮共约 10~15 个 commit，全部 `feat(ux-consistency):`。可选前缀细化：`feat(ux-consistency): rename projects/threads via PATCH`、`feat(ux-consistency): finish delivery as chat bubble` 等。
- **History JSONL Schema**：本轮不改后端历史事件格式；`{seq, ts, run_id, turn, type, data}` 保持不变。
- **LLM 超时配置**：不动。
- **Thinking Bubble**：不动；`FinishBubble` 与之并列存在，语义正交。
- **draft.ts 移除**：v2 项目里 `isDraftId` / `newDraftId` 是当时的产物，本次一并清理；引用点走全局搜索删净。
