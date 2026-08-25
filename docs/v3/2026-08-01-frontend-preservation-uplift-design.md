# HTML PPT Agent 前端保留式完善规格

> 状态：待实施
> 日期：2026-08-01
> 适用范围：`frontend/`，以及为前端运行状态恢复新增的最小后端查询接口
> 产品阶段：0 到 1，不保留已废弃的前端命名和兼容层

## 1. 目标

本次不是页面重构，而是在现有工作区骨架上完成一次产品化完善，使界面更像成熟的 HTML PPT Agent 工作台，而不是由若干功能组件拼接成的开发界面。

本次同时解决四类问题：

1. 保持现有 PowerPoint 式工作区组织不变，提高视觉一致性和信息密度。
2. 让 Agent 运行过程对业务用户可读，同时保留可下钻的工程细节。
3. 完善 API、SSE、运行状态和预览加载的错误处理，消除静默失败与错误状态。
4. 补齐可访问性、键盘操作、加载态、空态、失败态和回归测试。

## 2. 设计判断

### 2.1 Design Read

这是一个面向业务用户的专业 HTML PPT Agent 工作区。采用克制、精密、偏生产力工具的视觉语言，继续使用现有 React、Tailwind、Radix 和 Lucide 技术栈，只吸收 Fluent 类办公软件的秩序感，不引入新的 UI 框架。

### 2.2 设计参数

- `DESIGN_VARIANCE: 3`：低变化，优先统一和可预测。
- `MOTION_INTENSITY: 2`：仅保留 hover、focus、展开收起、面板调整等功能性动效。
- `VISUAL_DENSITY: 7`：中高密度，适合桌面生产力工作区。

### 2.3 视觉原则

- 一个主强调色：蓝色只用于选中、主操作、焦点和当前执行。
- 语义色只用于成功、警告、错误和等待输入。
- 大部分分组使用间距、分割线和背景层级，不把每个内容块都做成卡片。
- 圆角按对象类型固定，不允许每个组件自行决定。
- 不使用紫色 AI 渐变、霓虹色、玻璃拟态、大面积渐变和装饰性动画。
- 不引入新的图标库，统一使用现有 Lucide 图标。
- 不新增字体依赖，使用系统 UI 字体栈。

## 3. 不可破坏的产品契约

以下内容属于验收级约束，任何实现不得改变：

1. 保留顶部工作区项目 Tab，并继续支持项目切换、新建和项目菜单。
2. 保留左侧幻灯片导航、中间预览、右侧 Agent 对话的三栏结构。
3. 保留左栏和右栏的独立隐藏、展开效果。
4. 保留左右面板拖拽调整宽度的能力。
5. 保留面板宽度和显示状态的持久化。
6. 保留中间区域的单页预览、整份概览、蓝图视图、HTML 视图和前后翻页。
7. 保留右栏的多 Thread Tab、时间线、输入框和需要用户输入时的交互。
8. 保留主预览 iframe 的隔离策略，不恢复 `allow-same-origin`。
9. 不增加 Office 式顶部工具栏，不改变对话栏位于右侧的产品方向。

建议保持当前面板尺寸约束：

| 区域 | 默认宽度 | 最小宽度 | 最大宽度 |
|---|---:|---:|---:|
| 左栏 | 22% | 16% | 32% |
| 中栏 | 自适应 | 30% | 自适应 |
| 右栏 | 28% | 20% | 40% |

如果现有 `react-resizable-panels` 的持久化 key 需要升级，只允许进行一次明确的版本升级，并补充迁移说明。不得因为样式调整而丢失用户已保存的左右栏显示状态。

## 4. 当前主要问题

### 4.1 API 与运行状态

- `APIError` 只显示 HTTP 状态码，丢失后端错误码、可读消息、详情和 `X-Request-ID`。
- `fetchClient` 没有统一超时策略，也没有把外部 `AbortSignal` 与内部超时组合。
- SSE 数据仍以 `any` 贯穿事件解析、Store 和 UI，事件字段错误只能在运行时暴露。
- EventSource 临时断线会触发 `onerror`，当前实现会把可恢复断线直接呈现为运行失败。
- `lastEventId` 没有按运行持久化，刷新页面后无法续接仍在运行的 SSE。
- 取消请求成功后，Store 把状态写为 `done`，这是明确的状态错误。
- 创建 Run 失败时只写入 `console.error`，用户已经发送的消息也没有明确失败反馈。
- 运行终止后刷新项目资源时依赖当前激活项目，可能错误刷新另一个已经切换到的项目。
- 多个 API 操作只在控制台报告失败，页面缺少上下文内反馈。

### 4.2 预览与资源加载

- 进入项目后使用 `Promise.all` 拉取全部幻灯片 HTML，页数增加后首屏成本线性增长。
- HTML 加载失败会静默回落到蓝图或空状态，用户无法区分未物化、正在加载和加载失败。
- 概览与单页预览共享过于粗糙的加载方式，没有按当前页、相邻页和可见缩略图分级加载。
- 页面切换时缺少稳定的预览缓存 key，版本更新后可能出现旧 HTML 短暂闪现。
- 部分按钮存在展示但没有完整行为，必须接通真实行为或移除，不得保留假按钮。

### 4.3 视觉与组件

- 旧的 `mode-outline`、`mode-page`、`mode-normal` 等 Token 仍对应历史 kind 设计，与当前 `artifact + level + interaction + strategy` 模型不一致。
- 颜色过多承担结构区分，造成 Agent 模式像多个独立产品。
- 圆角、边框、阴影、按钮尺寸和图标线宽缺少统一规则。
- 左栏主要依赖文字行，章节、子章节、当前页和物化状态的层级不够清楚。
- 右栏直接展示 `Strategy`、`Context ready`、工具名和英文状态，业务用户理解成本高。
- Plan、工具调用、验证和产物事件权重接近，导致时间线噪声较大。
- 多处 hover 才出现的操作对键盘用户不可发现。
- 中英文混排和状态文案不统一。

## 5. 目标视觉系统

### 5.1 色彩 Token

建议在 `frontend/tailwind.config.js` 和 `frontend/src/index.css` 中统一为语义 Token，具体命名可根据现有工程稍作调整：

| Token | 建议值 | 用途 |
|---|---|---|
| `workspace` | `#E9EDF2` | 应用背景 |
| `panel` | `#F8F9FB` | 左右面板和顶部区域 |
| `panel-muted` | `#F0F3F7` | 次级工具条和悬停背景 |
| `canvas` | `#DFE5EC` | 中间预览画布 |
| `surface` | `#FFFFFF` | 幻灯片、输入框、真实内容卡片 |
| `ink` | `#17202B` | 主文字 |
| `muted` | `#5F6B7A` | 次级文字 |
| `subtle` | `#95A0AE` | 辅助文字和禁用态 |
| `border` | `#D5DBE3` | 常规分割线 |
| `border-strong` | `#BCC5D0` | 面板边界和选中边界 |
| `accent` | `#2F67F6` | 主操作、选中、焦点 |
| `accent-soft` | `#E8EFFF` | 轻量选中背景 |
| `success` | `#2F7D65` | 成功和验证通过 |
| `success-soft` | `#E7F3EE` | 成功背景 |
| `warning` | `#B96D1F` | 等待输入和非阻塞警告 |
| `warning-soft` | `#FFF3DF` | 警告背景 |
| `danger` | `#C84953` | 失败和破坏性操作 |
| `danger-soft` | `#FCECEF` | 失败背景 |

实施要求：

- 删除没有调用者的旧 mode Token。
- 将运行状态、目标范围和交互策略通过文字、图标和少量语义色表达，不再为每种组合分配独立颜色。
- `blueprint` 与 `presentation` 可以使用相同主色，通过标签文案区分。
- 仅实际状态使用圆点或 Badge，不使用装饰性彩色圆点。

### 5.2 字体、间距和形状

- UI 字体：`Aptos, Inter, -apple-system, BlinkMacSystemFont, "SF Pro Text", "PingFang SC", "Microsoft YaHei", sans-serif`。
- 等宽字体仅用于请求 ID、事件 ID、耗时和技术详情。
- 基础间距继续使用 4px 网格。
- 常规控件高度统一为 28px 或 32px。
- 圆角规则：
  - 画布和幻灯片：4px。
  - 按钮、输入框、菜单项：6px。
  - 内容卡片和提示：8px。
  - Dialog：10px。
  - 完全圆角只用于状态 Badge 和头像类对象。
- Lucide 默认尺寸 16px，默认 `strokeWidth={1.75}`，小型辅助图标可使用 14px。
- 焦点环统一为 2px `accent` 外环，必须使用 `focus-visible`。

### 5.3 阴影和动效

- 面板分组优先使用边框和背景，不添加悬浮卡片阴影。
- 幻灯片画布允许一层低对比冷灰阴影。
- 菜单和 Dialog 使用统一浮层阴影。
- 动画时长控制在 120ms 到 180ms。
- 面板展开收起沿用现有行为，不增加弹性或大位移动画。
- 尊重 `prefers-reduced-motion`。

## 6. 信息架构与页面完善

### 6.1 顶部项目 Tab

保留现有位置、高度和切换模型，只做以下完善：

- 当前 Tab 使用白色或 `panel` 背景、清晰边框和底边连接感，不使用强阴影。
- 非当前 Tab 使用轻量 hover 背景。
- 项目运行状态只在状态真实存在时展示，运行中、等待输入、失败分别使用语义色。
- 项目菜单按钮在 hover 和 `focus-within` 时都可见。
- `Loading...`、`Untitled Project` 等文案统一为中文。
- 保持新增项目入口和水平滚动，不改变 Tab 数据结构。

### 6.2 左侧幻灯片导航

保留章节列表、页面选择、新增、删除和重排能力，增强层级：

- 章节号和章节名作为一级分组标题。
- 子章节号和子章节名作为二级辅助标题。
- 每个页面行包含紧凑缩略图或版式占位、页码、标题和物化状态。
- 当前页使用 3px 主色侧边条与 `accent-soft` 背景，避免整块高饱和颜色。
- 状态只显示业务可读文案，例如“未生成”“蓝图已更新”“设计已更新”“已同步”。
- 删除操作在 hover 与键盘 focus 时均可发现。
- 运行期间继续禁用结构编辑，并在 tooltip 中解释原因。
- 拖拽排序必须保留，同时提供键盘可用的上移、下移入口。
- 新增和删除失败必须在左栏就地反馈，不得只写控制台。

### 6.3 中间预览区

保留现有视图切换和导航，只做局部强化：

- 工具条统一控件高度、图标尺寸、tooltip 和中文文案。
- 画布使用 `canvas` 背景，可增加极低对比度网格，但不得影响幻灯片内容判断。
- 当前幻灯片使用固定比例容器，加载中显示不跳动的 Skeleton。
- 明确区分以下状态：
  - 未选择项目。
  - 项目没有页面。
  - 页面未物化。
  - HTML 正在加载。
  - HTML 加载失败。
  - HTML 已加载但 iframe 运行异常。
  - 蓝图正在加载或加载失败。
- 每个失败态提供就地“重试”，不自动重放有副作用的请求。
- “放映”按钮必须连接真实全屏预览和键盘翻页。如果本迭代无法完成真实行为，则从界面移除，禁止保留无效按钮。
- 支持预览区键盘操作：
  - 左右方向键切换页面。
  - `O` 切换单页与概览。
  - `Escape` 退出全屏或关闭当前轻量浮层。
  - 输入框聚焦时不得劫持快捷键。
- 保留 `IsolatedSlidePreview` 的 iframe 安全边界和运行错误提示。

### 6.4 右侧 Agent 对话栏

保留 `Header + ThreadTabs + Timeline + Composer` 结构，不改造成后台 Dashboard。

#### 运行摘要

在 ThreadTabs 下方、Timeline 上方增加紧凑的 `RunSummary`。仅在运行中、等待输入或最近一次终止结果需要展示时出现。

摘要包含：

- 目标：当前页蓝图、整份蓝图、当前页 HTML 或整份 HTML。
- 策略：业务可读名称。
- 阶段：业务可读名称。
- 进度：优先从 Plan 步骤计算完成数和总数，不再长期显示 `0 / 0`。
- 操作：运行中提供“停止”。

策略文案：

| 内部值 | 界面文案 |
|---|---|
| `respond` | 直接回答 |
| `direct_action` | 直接修改 |
| `compact_workflow` | 轻量工作流 |
| `full_pev` | 完整工作流 |

阶段文案：

| 内部值 | 界面文案 |
|---|---|
| `context` | 准备上下文 |
| `plan` | 制定执行方案 |
| `execute` | 执行修改 |
| `verify` | 验证结果 |
| `repair` | 修复问题 |
| `commit` | 提交版本 |
| `deliver` | 完成交付 |

#### Timeline 层级

时间线分成三层：

1. 业务叙事：用户指令、Agent 回复、等待输入、最终结果和错误。
2. 执行摘要：Plan、验证总结、产物状态和运行摘要。
3. 技术细节：工具参数、工具结果、上下文详情和事件原始字段。

实施规则：

- `context_status` 与 `strategy_status` 合并为紧凑的 `ExecutionMetaRow`。
- 执行开始后，元信息默认收起，只显示“上下文已准备”和策略名称。
- `PlanCard` 可展开收起。运行中默认展开，运行结束后默认收起。
- `ToolCallCard` 默认只显示业务动作、目标、耗时和状态，原始工具名与 JSON 放在 `Disclosure` 内。
- 连续的验证事件合并为一个 `VerificationSummary`，展示通过数、失败数和问题入口。
- `artifact.staged` 到 `artifact.committed` 应更新同一产物状态，不重复堆叠。
- 最终结果按“完成内容、影响范围、验证结果、可继续操作”展示，不直接显示原始对象。
- 请求 ID 放在错误详情的可展开区域，不占据主界面。
- 所有状态改为中文，保留 HTML、PPT、JSON 等必要技术名词。

#### 输入框

保留当前 ModeSwitcher 和快捷提交逻辑，优化为更易懂的上下文栏：

`本次作用于 当前页 HTML | 自动执行 | 仅阻塞时询问`

要求：

- 低频选项可以通过紧凑菜单展开，但不得删除现有能力。
- 输入框边框只使用主强调色，不再根据 artifact 切换橙色、蓝色等多套 ring。
- 运行中禁用输入时展示原因，并保留明确的“停止”操作。
- 新建项目、创建 Thread、创建 Run 任一步失败时，输入内容不得丢失。
- 创建 Run 成功后再清空输入内容。
- 需要用户输入的卡片提交后必须进入已回答状态，防止重复提交。
- 保留中文输入法组合输入保护和 `Cmd/Ctrl + Enter`。

## 7. API 与状态模型

### 7.1 类型化错误

新增或重构以下类型：

```ts
interface APIErrorPayload {
  error?: {
    code?: string;
    message?: string;
    details?: unknown;
  };
}

class APIError extends Error {
  status: number;
  code?: string;
  details?: unknown;
  requestId?: string;
  retryable: boolean;
}
```

`fetchClient` 必须：

- 解析后端 `{ error: { code, message, details } }`。
- 读取响应头 `X-Request-ID`。
- 保留无法解析为 JSON 的响应文本。
- 支持调用方 `signal`。
- 支持默认超时，建议 15 秒，渲染 HTML 可单独配置更长超时。
- 区分用户取消、超时、网络错误和业务错误。
- 不对 `POST`、`PATCH`、`DELETE` 自动重试。
- 不新增全局 toast 依赖，错误由最接近操作的组件就地展示。

### 7.2 SSE 类型

把 `SSEEvent` 改为由事件名称判别的联合类型，至少覆盖：

- Run 开始、完成、失败、取消。
- Context、Strategy、Plan、Stage、Step。
- Tool call 与 Tool result。
- Verification 与 Repair。
- Artifact staged 与 committed。
- Needs input。
- Status summary。

每种事件拥有独立 payload 类型。解析器在运行时验证最小必需字段，对未知事件记录诊断但不破坏当前运行。

### 7.3 SSE 连接状态

前端连接状态与业务运行状态分离：

```ts
type StreamStatus =
  | 'idle'
  | 'connecting'
  | 'open'
  | 'reconnecting'
  | 'closed';
```

规则：

- EventSource 的临时 `error` 先进入 `reconnecting`，不得立即把 Run 标记为失败。
- 只有收到 `run.failed`、无法恢复的协议错误，或查询后端确认失败，才进入业务错误态。
- 每次事件保存 `lastEventId`。
- 终态事件到达后主动关闭 EventSource。
- 页面卸载、Thread 关闭和 Run 被替换时清理连接。
- 重连时不得重复插入相同事件。事件 reducer 以事件 ID 或稳定业务 ID 去重。

### 7.4 活跃 Run 恢复

增加最小只读接口：

```http
GET /api/v1/runs/:id
```

响应复用现有 `runResponse`，包含 `id`、`thread_id`、`project_id`、`status`、`target`、`interaction`、`events_url`。

前端在 `sessionStorage` 按 Thread 保存：

```ts
interface PersistedActiveRun {
  runId: string;
  threadId: string;
  projectId: string;
  lastEventId?: string;
}
```

刷新页面后的行为：

1. 查询 Run。
2. 如果状态为 `pending`、`running` 或 `waiting`，使用保存的 `lastEventId` 重新订阅。
3. 如果已经终止，恢复对应终态并清理活跃 Run 记录。
4. 如果 Run 不存在，清理无效记录并给出一次非阻塞提示。

这不是 Durable Run，也不恢复后端进程重启后丢失的执行任务。本次只解决浏览器刷新和短暂网络中断后的前端续接。

### 7.5 Run Store 修正

- Run 状态至少区分 `creating`、`running`、`needs_input`、`done`、`error`、`canceled`。
- 取消成功后必须写入 `canceled`，不得写入 `done`。
- 创建失败时保留用户输入，在 Timeline 插入可重试的失败项。
- Session 保存自己的 `projectId`，终止后只刷新该项目，不读取可能已经变化的全局激活项目。
- `progress.current` 与 `progress.total` 从 Plan 步骤或明确事件计算。如果没有可信数据，只展示阶段，不展示虚假数字。
- 终止状态关闭连接并清理持久化。
- Thread 和项目切换不得串写 Session。

## 8. 预览加载模型

建立按页面和版本缓存的资源状态：

```ts
type ResourceState<T> =
  | { status: 'idle' }
  | { status: 'loading'; previous?: T }
  | { status: 'ready'; data: T }
  | { status: 'error'; error: APIError; previous?: T };
```

建议缓存 key：

```text
projectId:slideId:presentationRevision
```

加载策略：

- 单页视图优先加载当前页。
- 当前页稳定后预取前一页和后一页。
- 概览视图只加载进入可视区域的缩略图，可使用 `IntersectionObserver`。
- 蓝图视图独立缓存，不因 HTML 失败而丢失蓝图。
- 页面 revision 变化时立即使用新 key，请求成功前可保留上一版本并显示“正在更新”。
- 请求切页后取消已经无意义的高优先级请求。
- 不新增后端缩略图服务，本次继续复用安全 iframe。

## 9. 组件边界

优先复用现有组件，不进行目录级大迁移。允许新增以下小型组件：

### 通用 UI

- `Button`：primary、secondary、ghost、danger。
- `IconButton`：统一尺寸、tooltip、aria-label 和 focus。
- `Badge`：仅状态和紧凑元信息。
- `Disclosure`：技术详情展开收起。
- `InlineNotice`：就地成功、警告和错误。
- `Skeleton`：预览和列表加载占位。

### Agent

- `RunSummary`。
- `ExecutionMetaRow`。
- `VerificationSummary`。
- `runtimeLabels.ts`：集中维护 strategy、stage、tool、artifact、level 和状态的业务文案。

### Viewer

- `useSlideRenderCache` 或等价的局部资源层。
- `PreviewState`：统一未物化、加载、错误和重试展示。

组件要求：

- 不引入 Fluent UI、shadcn、Carbon 或新的 Design System。
- 不把所有 Timeline 项统一包成相同 Card。
- 保持现有 Zustand Store 边界，只有跨组件共享且确实需要持久化的状态进入 Store。
- 派生数据使用 selector 或纯函数，不重复存储。

## 10. 文件级实施建议

以下为预期触达范围，不要求为了匹配清单而机械修改：

| 范围 | 文件 |
|---|---|
| 全局 Token | `frontend/tailwind.config.js`、`frontend/src/index.css` |
| API 错误与类型 | `frontend/src/api/client.ts`、`frontend/src/api/types.ts`、可新增 `frontend/src/api/errors.ts` |
| SSE | `frontend/src/api/sse.ts` |
| Run 查询 | `frontend/src/api/runs.ts`、`backend/internal/httpapi/router.go`、`backend/internal/httpapi/run_handler.go` |
| Run 状态 | `frontend/src/stores/runStore.ts`、`frontend/src/features/agent/useActiveSession.ts` |
| 工作区契约 | `frontend/src/features/workspace/AppShell.tsx`、`ProjectTabs.tsx`、`PanelToggleButtons.tsx` |
| 左栏 | `frontend/src/features/deck/DeckNavigator.tsx` |
| 中栏 | `frontend/src/features/viewer/PreviewWorkspace.tsx`、`IsolatedSlidePreview.tsx` |
| 右栏 | `frontend/src/features/agent/AgentPanel.tsx`、`Timeline.tsx`、`CommandComposer.tsx` 和相关 Card |
| 通用组件 | `frontend/src/components/ui/` |
| 测试 | 对应的 `*.test.ts`、`*.test.tsx` 和后端 handler 测试 |

删除无调用者且已经过时的前端接口、Token 和字段，例如确认无调用者后的 `activeModeColor`、旧 mode 色彩映射或后端没有实现的陈旧 client 方法。项目处于 0 到 1 阶段，不为这些旧设计增加兼容别名。

## 11. 可访问性和文案

- 所有纯图标按钮有明确的中文 `aria-label`。
- hover 才出现的内容必须同时支持 `focus-within`。
- 所有菜单、Dialog 和 Disclosure 可用键盘操作。
- 页面排序提供非拖拽替代操作。
- 颜色对比满足 WCAG AA。
- 状态不能只靠颜色表达，必须有文字或图标。
- 错误文案包含用户可以执行的下一步。
- 技术详情默认收起，不隐藏排障所需的错误码和请求 ID。
- 全站清理以下英文用户文案：
  - `Loading...`
  - `Untitled Project`
  - `Thinking...`
  - `Needs Input`
  - `Failed`
  - `Canceled`
  - `Context ready`
  - `Strategy`
  - `verified`
  - `No slides yet`

## 12. 测试与验证

### 12.1 单元测试

必须覆盖：

- API 错误体、非 JSON 错误、请求 ID、超时和用户取消。
- 每种 SSE 事件的解析与 reducer。
- SSE 临时断线进入重连，不进入业务失败。
- last event ID 保存与恢复。
- 取消 Run 后状态为 `canceled`。
- 创建 Run 失败保留输入并展示失败。
- Artifact staged 到 committed 不重复。
- 当前页和相邻页预取。
- 概览只加载可见页面。
- revision 变化不会错误复用旧缓存。
- 未物化、加载、失败、iframe 运行错误的展示。
- strategy、stage 和 tool 的中文映射。

### 12.2 集成测试

必须把以下工作区行为作为回归测试：

- 顶部项目 Tab 存在且可以切换项目。
- 左、中、右三栏同时存在。
- 左栏可以隐藏和恢复。
- 右栏可以隐藏和恢复。
- 面板可以拖拽调整，状态持久化后可恢复。
- 切换项目和 Thread 不会串写运行状态。
- 运行中、等待输入、完成、失败、取消都能正确显示。
- 键盘翻页、概览切换和全屏退出有效。
- hover 操作可通过键盘 focus 发现。

### 12.3 浏览器验收

至少在以下视口检查：

- 1440 x 900。
- 1280 x 800。

每个视口检查：

- 三栏展开。
- 左栏隐藏。
- 右栏隐藏。
- 两侧都隐藏。
- 左右栏调整到最小和最大宽度。
- 项目名称、页面标题和消息很长时不溢出。
- 真实 HTML iframe 加载、报错和切页。
- SSE 运行、断线重连、等待输入、取消和完成。

### 12.4 命令

```bash
cd backend && go test ./...
cd frontend && pnpm test
cd frontend && pnpm tsc --noEmit
cd frontend && pnpm lint
cd frontend && pnpm build
```

如果项目没有独立 `tsc` 或 `lint` script，使用现有等价命令。所有新增测试通过，现有 React `act(...)` 警告也应清理，不把警告视为可忽略。

## 13. 分阶段实施顺序

1. 建立视觉基线截图和工作区契约测试。
2. 类型化 API 错误和 SSE 事件。
3. 修正 Run Store、取消状态、断线重连和活跃 Run 恢复。
4. 改造预览资源加载与明确状态。
5. 收敛 Token 和基础 UI 组件。
6. 完善顶部 Tab、左栏和中间预览。
7. 完善右栏运行摘要、Timeline 层级和 Composer。
8. 清理文案、可访问性和废弃代码。
9. 完成单元、集成、浏览器和构建验证。

接口和状态模型优先于视觉修改。不得用 UI 条件判断掩盖状态错误。

## 14. 非目标

- 不改变三栏信息架构。
- 不改变顶部项目 Tab 组织。
- 不取消左右栏隐藏、展开和缩放。
- 不增加传统 Office 顶部工具栏。
- 不做全新品牌或大面积视觉重构。
- 不引入暗色主题。
- 不引入新的 UI 框架、图标库、动画库或状态管理库。
- 不修改 Agent Runtime、Context Engineering 或 PEV 的后端执行语义。
- 不实现后端进程重启后的 Durable Run。
- 不实现多人协同编辑。
- 不实现移动端完整编辑体验。
- 不新增 repo 管理或素材库业务面板。
- 不建设新的缩略图渲染服务。

## 15. 完成定义

只有同时满足以下条件才算完成：

- 三栏、顶部 Tab、左右栏隐藏和缩放全部保留并有回归测试。
- 旧 mode 视觉语义被当前产品语义替代，没有为了兼容而残留两套系统。
- 前端所有主要异步操作具有 loading、empty、error 和 retry 表达。
- SSE 临时断线不再误报 Run 失败，刷新页面可以续接仍存活的 Run。
- 取消、失败、等待输入和完成状态准确。
- 预览不再在首屏请求整份 HTML。
- Agent 时间线以业务摘要为主，技术详情可下钻。
- 用户界面文案统一为中文。
- 键盘操作、focus-visible、aria-label 和状态对比度达标。
- 前后端测试、类型检查、lint 和构建通过。
- 浏览器验收覆盖指定视口和面板组合。
