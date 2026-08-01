# HTML PPT Agent 前端保留式完善一键执行 Prompt

将下面整段 Prompt 交给负责开发的 Agent。该 Agent 应在项目根目录执行，一次性完成设计、实现、测试和交付，中途不向用户确认普通实现细节。

```text
你现在是本项目的高级前端工程师，同时负责本次变更涉及的最小 Go API 适配。请在当前代码库中一次性完成 HTML PPT Agent 前端保留式完善。

唯一规格文件：
docs/superpowers/specs/2026-08-01-frontend-preservation-uplift-design.md

开始前必须完整阅读该规格，以及当前代码、现有测试和最近相关 commit。规格是本次实现的单一事实来源。如果代码现状与规格冲突，以规格为准；如果普通建议与“不可破坏的产品契约”冲突，以不可破坏契约为最高优先级。

一、不可违反的边界

1. 这不是视觉重构，不得改变现有信息架构。
2. 必须保留顶部工作区项目 Tab 切换。
3. 必须保留左侧幻灯片导航、中间预览、右侧 Agent 对话的三栏结构。
4. 必须保留左右侧栏独立隐藏、展开和拖拽缩放。
5. 必须保留面板显示状态和宽度持久化。
6. 必须保留单页、概览、蓝图、HTML、前后翻页、Thread Tabs、Timeline、Composer 和 Needs Input。
7. 不增加 Office 式顶部工具栏。
8. 不引入新的 UI 框架、图标库、动画库、状态管理库或字体依赖。
9. 不恢复 iframe 的同源权限，不降低当前预览安全隔离。
10. 不修改 Agent Runtime、Context Engineering、PEV 语义，不实现 Durable Run。
11. 项目处于 0 到 1 阶段，不为旧 mode Token、旧 kind 命名或无调用者接口增加兼容层。
12. 不做与本规格无关的重构，不覆盖用户已有的无关修改。

二、执行方式

你需要自主完成以下过程，中途不要向用户询问普通设计和实现选择：

1. 检查 git 状态、最近 commit、项目说明、前后端脚本和测试结构。
2. 读取规格后制定一次内部实施计划，然后连续执行到完成。
3. 在修改前运行相关测试并记录基线。
4. 如本地服务可启动，使用浏览器记录 1440 x 900 的当前界面基线。
5. 按下面阶段实现，每一阶段完成后运行最小相关测试。
6. 最后运行完整测试、类型检查、lint、构建和浏览器验收。
7. 只有遇到会破坏用户数据、需要外部凭证或超出规格授权的操作时才停止。
8. 不得因为个别测试困难而跳过验证，也不得把 console error 或 React act 警告留作“已知问题”。

三、实现阶段

阶段 0：建立保护网

- 为 AppShell 增加或完善回归测试，锁定顶部项目 Tab、三栏结构、左右栏隐藏恢复和面板缩放持久化。
- 为当前关键 Run reducer、SSE、PreviewWorkspace 建立可验证基线。
- 不通过快照掩盖行为问题，优先测试用户可观察行为。

阶段 1：API 错误与 SSE 类型

- 将 API 错误实现为结构化错误，保留 status、code、message、details、requestId 和 retryable。
- 解析后端 `{ error: { code, message, details } }` 和 `X-Request-ID`。
- fetchClient 支持外部 AbortSignal 和默认超时，区分超时、用户取消、网络错误和业务错误。
- 不对 POST、PATCH、DELETE 自动重试。
- 把 SSEEvent 从 `data: any` 改为按 event 判别的联合类型。
- 为未知事件保留诊断，不让未知事件破坏当前 Run。

阶段 2：Run、SSE 和刷新续接

- 将 SSE 连接状态与 Run 业务状态分开。
- 临时 EventSource error 进入 reconnecting，不得直接进入 Run error。
- 保存 lastEventId，重连去重，终态事件后主动关闭连接。
- 修复取消成功后被写成 done 的错误，必须为 canceled。
- 创建 Run 失败时保留输入，在正确上下文显示可重试错误。
- Session 保存自己的 projectId，终态只刷新对应项目。
- progress 只展示可信数据，优先从 Plan 步骤计算。
- 新增最小只读接口 `GET /api/v1/runs/:id`，复用 runResponse。
- 在 sessionStorage 保存活跃 Run 标识和 lastEventId，浏览器刷新后查询 Run 并续接。
- 明确声明这只是前端刷新和网络断线续接，不实现后端进程重启后的任务恢复。

阶段 3：预览资源加载

- 不再在进入项目时使用 Promise.all 拉取整份 HTML。
- 单页优先加载当前页，随后预取相邻页。
- 概览使用 IntersectionObserver 或等价方式，只加载可见缩略图。
- 以 projectId、slideId、presentationRevision 作为稳定缓存维度。
- revision 变化时不错误复用旧 HTML，可在更新期间保留旧画面并明确显示“正在更新”。
- 区分未物化、HTML 加载中、HTML 加载失败、iframe 运行异常、蓝图加载失败等状态。
- 每个可恢复失败态提供就地重试。
- 保留 iframe sandbox，不添加 allow-same-origin。

阶段 4：视觉 Token 和小型 UI 原语

- 按规格收敛 Tailwind Token 和 index.css。
- 删除确认无调用者的旧 mode Token 与 activeModeColor 等历史残留。
- 主强调色只使用蓝色，状态使用 success、warning、danger。
- 统一字体、间距、圆角、边框、阴影、控件高度、Lucide 尺寸和线宽。
- 只在确有复用价值时新增 Button、IconButton、Badge、Disclosure、InlineNotice、Skeleton。
- 不迁移到 Fluent UI、shadcn、Carbon 或其他 Design System。
- 不把所有内容都套进卡片。

阶段 5：三栏局部完善

- 顶部 Tab：保留尺寸、位置和交互，完善 active、hover、focus、状态点和中文文案。
- 左栏：强化章节、子章节、当前页、页码、紧凑缩略图和物化状态层级。
- 保留拖拽排序，增加键盘可用的上移、下移入口。
- 新增、删除、重排失败在左栏就地反馈。
- 中栏：统一工具条，补充 tooltip、aria-label、中文文案和清晰画布背景。
- 连接真实的全屏放映和键盘翻页。如果无法提供真实行为，移除无效按钮。
- 输入框聚焦时不得劫持预览快捷键。
- 不移动或重组三栏，不改变顶部项目 Tab 为其他导航形式。

阶段 6：Agent 信息层级

- 在 ThreadTabs 和 Timeline 之间增加紧凑 RunSummary。
- 集中维护 strategy、stage、tool、artifact、level 和状态的中文映射。
- 将 respond、direct_action、compact_workflow、full_pev 映射为规格中的业务名称。
- 将 context、plan、execute、verify、repair、commit、deliver 映射为规格中的业务阶段。
- 合并 context_status 与 strategy_status 为可收起的 ExecutionMetaRow。
- PlanCard 运行中默认展开，终态默认收起。
- ToolCallCard 默认展示业务动作、目标、耗时和状态，原始工具名与 JSON 放到 Disclosure。
- 多个 verification 事件合并为 VerificationSummary。
- Artifact staged 到 committed 更新同一项，不产生重复。
- FinalResult 按完成内容、影响范围、验证和下一步展示。
- Error 主文案可行动，错误码和 requestId 放到可展开详情。
- Timeline 以业务叙事为主，不能做成满屏 Dashboard 卡片。

阶段 7：Composer、文案和可访问性

- 保留 ModeSwitcher 和现有能力，用紧凑上下文栏表达“作用对象、执行方式、询问策略”。
- 输入框统一使用主强调色边框，不根据 artifact 改变整套颜色。
- 创建项目、Thread 或 Run 失败时不清空用户输入，成功后再清空。
- Needs Input 提交后进入已回答状态，防止重复。
- 保留中文输入法保护和 Cmd/Ctrl + Enter。
- 所有纯图标按钮提供中文 aria-label。
- hover 操作同时支持 focus-within。
- 状态不只依赖颜色。
- 清理规格列出的英文用户文案。
- 满足 WCAG AA，补齐 focus-visible 和 reduced motion。

阶段 8：验证和收尾

- 运行所有新增与现有前端测试。
- 运行后端 go test。
- 运行 TypeScript 类型检查、lint 和生产构建。
- 修复所有由本次变更产生或暴露的测试失败、console error、act warning 和类型问题。
- 在 1440 x 900 与 1280 x 800 验收。
- 每个视口验证三栏、隐藏左栏、隐藏右栏、两侧隐藏、左右最小和最大宽度。
- 验证长项目名、长页面标题、长消息、空态、加载态、错误态、重试、SSE 重连、等待输入、取消和完成。
- 验证真实 iframe 预览、全屏和键盘操作。
- 最后检查 git diff，确认没有大范围视觉重构、没有旧兼容层、没有无关文件和运行产物。

四、工程约束

- 使用现有包管理器和现有依赖。
- 除非规格无法在现有依赖下完成，否则不安装新包。
- API 与 Store 使用明确 TypeScript 类型，不使用 any 绕过。
- 状态修正应发生在 API、parser 或 Store 层，不用组件条件分支掩盖错误。
- 异步请求应清理 AbortController、EventSource、observer 和订阅。
- 不重复存储可以派生的数据。
- 组件保持单一职责，但不要为每个小样式创建文件。
- 保留现有测试风格和目录组织。
- 后端只新增 Run GET 查询和对应测试，不扩张 Runtime。
- 不提交日志、SQLite、构建目录、截图临时文件或本地运行产物。

五、最低测试清单

1. API error body、非 JSON error、X-Request-ID、timeout、abort。
2. 所有 SSE 事件 payload 和 reducer。
3. 临时断线为 reconnecting，终态才关闭。
4. lastEventId 保存、恢复和事件去重。
5. cancel 后为 canceled。
6. create run 失败保留输入并展示失败。
7. project/thread 切换不串写。
8. artifact staged/committed 不重复。
9. 当前页优先和相邻页预取。
10. 概览可见项按需加载。
11. revision 缓存失效。
12. 未物化、loading、render error、iframe runtime error。
13. 顶部 Tab、三栏、左右隐藏恢复、调整宽度和持久化。
14. strategy、stage、tool 中文映射。
15. keyboard navigation、focus-visible、aria-label。
16. Run running、needs input、done、failed、canceled。

六、完成汇报格式

完成后一次性向用户汇报：

1. 最终实现了什么，按接口与状态、预览、三栏 UI、Agent 交互、可访问性分组。
2. 明确说明三栏、顶部 Tab、左右栏隐藏和缩放均被保留。
3. 列出主要修改文件。
4. 列出所有实际执行的测试和结果。
5. 提供 1440 x 900 与 1280 x 800 的最终截图路径。
6. 说明是否增加依赖。如果没有，明确写“未增加依赖”。
7. 列出仍然存在但明确属于规格非目标的事项，不得把未完成的规格内容列为后续事项。

不要只输出分析或计划。请直接完成实现、测试、浏览器验收和最终交付。
```
