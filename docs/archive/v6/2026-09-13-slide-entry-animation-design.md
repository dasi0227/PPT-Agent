# 页内动画按页面展示时机播放设计

- 状态：已实现
- 日期：2026-09-13
- 对应 TODO：九、让页内动画在页面展示时播放
- 关联设计：
  - 2026-09-08-unified-html-canvas-design.md
  - 2026-09-10-dom-selection-reference-design.md
  - 2026-09-12-presentation-export-design.md

## 1. 需求解释

当前编辑器会预取当前页附近的 HTML，并在 Slide Runtime 中为已加载页面分别创建 iframe。非当前页只在外层通过 display: none 隐藏，其 HTML、CSS 和 JavaScript 已经开始执行。

这会使依赖页面加载触发的入场效果在后台提前运行。例如标题淡入、数字增长、SVG 绘制、Canvas 初始化和页面脚本计时都可能在用户翻到该页前开始或结束。再次返回页面时，原 iframe 仍被复用，页面状态和动画也不会从头开始。

本需求只修复应用内普通预览与全屏放映的页面执行时机：

> Runtime 可以继续预取和缓存页面 HTML 数据，但只有当前页可以创建并执行 iframe。页面离开时销毁执行环境，再次进入时重新创建，使现有基于页面加载触发的 CSS 和 JavaScript 从头运行。

本需求不是页与页之间的转场动画，也不建立页面可感知的 enter、leave、pause 或 replay 公共生命周期协议。

## 2. 产品目标

1. 未成为当前页的 HTML 不执行页面 CSS 动画或 JavaScript。
2. 页面首次成为当前页时，从新的文档执行环境开始运行。
3. 离开页面后释放原 iframe；再次进入时获得全新文档并从头运行。
4. 点击应用的“全屏放映”入口后，在全屏已经建立的前提下重播当前页。
5. 页面内容或主题真正变化时重建当前页；无关状态变化不得误触发重播。
6. 不改变静态页面、缩略图、概览、导出及既有加载错误交互。

## 3. 已确认范围

### 3.1 纳入范围

- 编辑器主区域的单页 HTML 预览。
- 应用内全屏放映。
- 现有页面加载即执行的 CSS 动画。
- 现有页面加载即执行的内联 JavaScript、Canvas 和其他脚本初始化。
- 页面离开后的整页执行环境释放。
- HTML 或主题变化后的当前页重建。
- DOM 选择桥接和草稿标记在页面重建后的恢复。
- 操作系统 prefers-reduced-motion 设置的既有行为。

### 3.2 不纳入范围

- 演讲者视图、讲稿或备注。
- 页间淡入、滑动、缩放等转场。
- 播放、暂停或手动重播控件。
- 向页面 HTML 暴露统一生命周期事件。
- 分析、改写或选择性重播页面内部动画。
- 绕过浏览器音视频自动播放限制。
- 重新设计页面加载、失败或重试界面。
- 改变概览和缩略图的现有行为。
- 改变独立 HTML、PNG 或 PDF 导出。
- 后端 API、Schema、数据库、项目文件格式、Agent 工具或生产 Prompt 改造。

## 4. 当前实现与问题定位

### 4.1 HTML 数据预取

PreviewWorkspace 读取当前页 HTML，并预取前后相邻页。useSlideRenderCache 已按项目、页面和 HTML revision 缓存字符串。

数据预取本身没有问题，应继续保留。问题在于 Slide Runtime 收到已缓存的页面集合后，会立即为集合中的每一页创建 iframe 和设置 srcdoc。

### 4.2 页面切换

当前 Runtime 为所有页面保留 runtime-slide 容器，setActive 仅切换 data-active。非当前容器使用 display: none，但其内层文档已经加载，页面脚本仍可能执行。

因此：

- 隐藏不等于暂停；
- CSS 动画时间不会等用户翻页；
- JavaScript 定时器和持续效果可能在后台运行；
- 返回页面不会得到新的初始状态。

### 4.3 全屏

当前全屏只是对主画布调用 Fullscreen API，复用同一个 Runtime 和当前 iframe。若当前页动画已在普通预览中结束，点击“全屏放映”不会自动重新开始。

### 4.4 导出

独立 HTML 播放器当前通过修改单一 iframe 的 src 进行翻页，页面进入时会重新加载。PNG 和 PDF 继续由 Chromium 生成静态页面。本需求不改变任何导出行为。

## 5. 核心设计

### 5.1 分离“数据已预取”和“页面正在执行”

Runtime 的 state.slides 继续保存父页面传入的 RuntimeSlide 描述，包括 HTML 字符串和 Runtime Frame。

DOM 中只允许存在一个带 data-slide-frame 的执行容器，即当前页容器。非当前页可以存在于 state.slides 数据中，但不得存在 iframe。

由此形成两层状态：

~~~text
HTML 数据层
  ├─ 当前页 HTML
  ├─ 前一页 HTML（可预取）
  └─ 后一页 HTML（可预取）

执行层
  └─ 当前页 iframe（唯一）
~~~

预取仍减少网络等待，但不会提前执行页面。

### 5.2 当前页执行实例

Runtime 维护当前页面索引、当前 slide_id 和当前执行身份。

当前页发生以下变化时，销毁旧执行容器并创建新容器：

- 当前 slide_id 变化；
- 当前页 HTML 字符串变化；
- 当前页 theme_id 变化；
- 收到针对当前 slide_id 的显式重播命令。

创建顺序：

1. 根据当前 RuntimeSlide 规范化 HTML；
2. 注入 Base CSS、Theme CSS 和 DOM selection bridge；
3. 创建新的 runtime-slide、runtime-canvas 与 iframe；
4. 写入 srcdoc，使页面从新的 Window 和 Document 环境执行；
5. 注入当前 Runtime Frame 公共装饰；
6. iframe load 后重新发送选择模式并绘制当前页草稿标记；
7. 根据当前容器尺寸执行 1920×1080 等比适配。

离开页面时直接移除原执行容器。iframe 被销毁后，其 Document、Window、计时器、requestAnimationFrame、Canvas 上下文和媒体实例由浏览器释放；Runtime 不尝试理解或逐项清理任意页面脚本。

### 5.3 执行身份与装饰更新

页面执行身份由以下内容决定：

- slide_id；
- 实际传入的 HTML 字符串；
- theme_id。

不把 ordinal、total、section、subsection、deck title、numbering 或 chrome 纳入执行身份。这些 Runtime Frame 字段变化时，只重新生成外层公共装饰并重新适配画布，不重建页面 iframe。

使用实际 HTML 字符串而不是仅依赖 Slide revision，原因是 HTML 新 revision 加载期间，前端可能暂时展示上一 revision 的 previous 内容。只有真正传入 Runtime 的 HTML 发生变化时才应重建；新内容到达后再重建一次。

state.slides 中其他页面的加入、移除、HTML 更新或预取完成，不得改变当前执行实例。

### 5.4 导航语义

gotoSlide 处理规则：

- 目标索引对应不同 slide_id：销毁旧实例并创建目标页实例；
- 目标索引仍对应同一 slide_id：不重播；
- 目标不存在：拒绝命令并保留当前有效实例；
- 页面重排但当前 slide_id 未变化：更新索引与公共装饰，不重播；
- 当前页被删除并由父组件选择另一页：按新 slide_id 创建实例。

快速连续导航时，以最后一次合法命令对应的 slide_id 为准。旧 iframe 的迟到 load 或 selection 消息不得覆盖新当前页状态。

### 5.5 全屏重播

点击“全屏放映”时：

1. PreviewWorkspace 请求主画布进入全屏；
2. 只有 Fullscreen API 成功后，才向 IsolatedSlidePreview 发出一次重播请求；
3. IsolatedSlidePreview 向 Runtime 发送内部命令 replayCurrentSlide，并携带预期 slide_id；
4. Runtime 仅在该 slide_id 仍是当前页时重建当前执行实例；
5. 全屏请求失败时不重播。

replayCurrentSlide 是应用外层与 Slide Runtime 之间的内部命令，不转发给页面 HTML，也不构成页面作者可依赖的生命周期 API。

仅以下动作触发该命令：

- 用户通过产品“全屏放映”按钮成功进入全屏。

窗口尺寸变化、fullscreenchange 的重复通知、浏览器标签页切换、选择工具开关和左右面板开关不发送重播命令。

### 5.6 离开其他视图后返回

从以下位置返回主单页 HTML 视图时，IsolatedSlidePreview 会重新挂载，因此当前页自然获得新执行实例：

- 概览视图；
- 设计稿视图；
- 其他项目。

浏览器标签页进入后台不卸载组件，返回时不主动重播。浏览器可以按自身策略暂停或节流后台动画，本产品不覆盖浏览器行为。

### 5.7 DOM 选择兼容

当前 DOM 选择数据保存在外层 Composer Store，而不是页面 iframe 内。页面实例重建后应：

- 重新注入 selection-bridge.js；
- 重新发送当前 selection session、slide_id、HTML revision/hash 和选择模式；
- 重新绘制属于当前页的草稿选择框；
- 继续拒绝来自旧 iframe Window 的迟到消息；
- 保持切页时取消当前选择模式的既有行为。

公共装饰仍位于内层页面 iframe 之外的 runtime-canvas 中，不因页面脚本重建而进入页面所有权。

### 5.8 减少动态效果

Runtime 继续加载现有 Base CSS。操作系统开启 prefers-reduced-motion 时，现有规则会把动画和 transition 压缩为近乎瞬时完成。

本需求不增加产品级动画开关，也不强制覆盖用户的系统无障碍设置。

### 5.9 音视频限制

整页重建会使页面内音视频回到初始状态，但是否允许自动播放仍由浏览器策略决定。本需求不增加用户手势代理、静音降级或权限提示，也不承诺有声媒体自动播放。

## 6. 前端协议调整

PreviewCommand 增加一个封闭命令：

~~~text
{
  type: "replayCurrentSlide",
  slide_id: "<当前稳定页面 ID>"
}
~~~

校验规则：

- type 必须精确匹配；
- slide_id 必须为非空字符串；
- 消息必须来自 Runtime 的直接父窗口；
- Runtime 当前页 ID 必须等于请求 slide_id；
- 不接受回调、脚本文本、URL 或任意执行参数。

updateDeck 和 gotoSlide 保持现有外部结构，避免为页面数据协议引入无关版本迁移。

## 7. 前端组件职责

### 7.1 PreviewWorkspace

- 保留当前页加载及相邻页 HTML 数据预取。
- 在全屏请求成功后产生一次重播请求。
- 全屏失败、退出全屏和普通 fullscreenchange 不产生重播。

### 7.2 IsolatedSlidePreview

- 继续发送 updateDeck、gotoSlide 和 DOM 选择命令。
- 接收外层一次性 replay 请求，并发送带当前 slide_id 的 replayCurrentSlide。
- 组件初次挂载不得因为 replay 请求默认值而造成双重加载。

### 7.3 Slide Runtime

- state.slides 只保存页面描述数据。
- DOM 中只维护当前页执行容器。
- 导航、内容变化、主题变化和显式重播按本设计重建。
- Runtime Frame 元数据变化只更新外层装饰。
- 保持父窗口来源校验、命令白名单和 iframe sandbox="allow-scripts"。

### 7.4 Preview Protocol

- 为 replayCurrentSlide 增加 TypeScript 类型和运行时校验。
- 不增加从页面 iframe 发出的生命周期事件。
- 不扩大既有 RuntimeEvent 白名单。

## 8. 预计改动文件

- frontend/public/slide-runtime/index.html
- frontend/src/features/viewer/previewProtocol.ts
- frontend/src/features/viewer/previewProtocol.test.ts
- frontend/src/features/viewer/IsolatedSlidePreview.tsx
- frontend/src/features/viewer/PreviewWorkspace.tsx
- frontend/src/features/viewer/slideRuntime.test.ts
- 必要时增加 PreviewWorkspace 或 IsolatedSlidePreview 的聚焦交互测试

不修改：

- backend/render-worker/worker.mjs
- backend/internal/export
- backend/prompts
- backend/internal/spec
- frontend/src/api 的项目数据结构

## 9. 状态与并发保护

### 9.1 重复 updateDeck

Runtime 初始化、iframe onLoad、React effect 和数据预取可能重复发送内容相同的 updateDeck。Runtime 必须比较当前执行身份，相同时复用当前 iframe，不能重播。

### 9.2 无关页面预取

相邻页预取完成会改变 slides 数组，但如果当前页 slide_id、HTML 和 theme_id 没有变化，只更新 state.slides，不重建当前实例。

### 9.3 迟到事件

切页或重播后，旧 iframe 可能在销毁边界产生迟到 load、selection 或 error 消息。所有内层消息继续以 event.source === 当前 iframe.contentWindow 为前提；旧 Window 的消息被忽略。

### 9.4 快速切页

连续切换 A → B → C 时，最终 DOM 只能存在 C 的一个执行容器。A、B 不得在后台残留 iframe，B 的迟到事件不得覆盖 C。

### 9.5 全屏竞争

发起全屏后用户可能立即切页。重播命令携带发起时预期 slide_id；若 Runtime 当前页已经变化，则忽略该命令，避免错误重播新页面。

## 10. 验收标准

- AC-ANIM-001：GIVEN 当前页为 A 且 B 的 HTML 已预取，WHEN 尚未导航到 B，THEN DOM 中不存在 B 的页面 iframe，B 的脚本未执行。
- AC-ANIM-002：GIVEN 页面 B 含加载触发的 CSS 动画和内联脚本，WHEN 首次导航到 B，THEN B 在新的 iframe 中从初始状态执行。
- AC-ANIM-003：GIVEN 已从 B 导航到 C，WHEN 检查 Runtime，THEN B 的 iframe 已移除，持续计时器和 Canvas 不再拥有活动文档。
- AC-ANIM-004：GIVEN B 已播放完成并离开，WHEN 再次导航到 B，THEN 新 iframe 与旧实例不同，CSS 和 JavaScript 从头执行。
- AC-ANIM-005：GIVEN 当前页 HTML 发生变化，WHEN 新 HTML 实际进入 Runtime，THEN 当前页重建一次并展示新内容。
- AC-ANIM-006：GIVEN 当前 theme_id 发生变化，WHEN Runtime 更新，THEN 当前页重建并使用新主题。
- AC-ANIM-007：GIVEN 只改变页序、页码、章节或公共装饰，WHEN updateDeck 到达，THEN iframe 实例保持不变，外层装饰更新。
- AC-ANIM-008：GIVEN 只有相邻页预取完成或其他页更新，WHEN updateDeck 到达，THEN 当前 iframe 实例保持不变。
- AC-ANIM-009：GIVEN 当前页已在普通预览中播放完成，WHEN 用户成功点击全屏放映，THEN 全屏建立后当前页使用新 iframe 重播。
- AC-ANIM-010：GIVEN 全屏请求失败，WHEN Promise 拒绝，THEN 当前页不重播。
- AC-ANIM-011：GIVEN 用户调整窗口、开关面板、开关选择工具或再次点击当前页，WHEN 相关状态变化，THEN 当前页不重播。
- AC-ANIM-012：GIVEN 用户从概览、设计稿或其他项目返回主 HTML 视图，WHEN 主预览重新挂载，THEN 当前页从新实例开始播放。
- AC-ANIM-013：GIVEN 浏览器标签进入后台再恢复，WHEN 当前预览组件始终挂载，THEN Runtime 不主动重播。
- AC-ANIM-014：GIVEN 系统启用 prefers-reduced-motion，WHEN 页面进入，THEN 继续使用现有近乎瞬时动画规则。
- AC-ANIM-015：GIVEN 页面实例被重建，WHEN DOM 选择状态重新发送，THEN 当前页草稿标记恢复且旧 iframe 消息被拒绝。
- AC-ANIM-016：GIVEN 页面为完全静态 HTML，WHEN 进入、离开和返回，THEN 页面视觉与现有行为一致。
- AC-ANIM-017：GIVEN 用户导出 HTML、PNG 或 PDF，WHEN 导出完成，THEN 播放和静态渲染行为与本需求实施前一致。

## 11. 测试方案

### 11.1 Runtime 单元测试

调整 slideRuntime.test.ts，覆盖：

- updateDeck 含多页时只创建当前页一个 iframe；
- gotoSlide 到不同页面时旧 iframe 被替换；
- 返回页面时创建新的 iframe；
- 相同 updateDeck 和相同页 gotoSlide 不重建；
- 其他页面加入或更新不重建当前页；
- 当前 HTML 或 theme_id 变化会重建；
- Runtime Frame 装饰变化不重建 iframe；
- replayCurrentSlide 匹配当前 slide_id 时重建，不匹配时忽略；
- 快速连续导航后只剩最终页面；
- 草稿标记在重建后恢复；
- 非父窗口和未声明命令继续被拒绝。

### 11.2 React 交互测试

覆盖：

- 全屏 Promise 成功后发送一次重播；
- 全屏 Promise 失败时不发送；
- 初次挂载不误发重播；
- fullscreenchange、resize、面板和选择状态变化不误发；
- HTML 数据预取逻辑保持不变。

### 11.3 真实浏览器验证（不纳入本轮）

JSDOM 只能证明 iframe DOM 的创建与替换，不能证明 srcdoc 内 CSS 动画、Window 销毁和 requestAnimationFrame 的真实执行时机。根据本轮最终开发要求，不执行真实 Chromium 验收；如后续需要端到端回归，可复用以下场景：

1. A 页显示，B 页已完成 HTML 数据预取；
2. B 页脚本在 window 上记录初始化次数，并包含可观察的 CSS 入场动画；
3. 未进入 B 时初始化次数为零；
4. 首次进入 B 时初始化一次且动画从起点播放；
5. 离开 B 后旧实例不再递增计时；
6. 返回 B 时产生新实例并从头播放；
7. 点击全屏后当前页产生新实例并从头播放；
8. 开启 prefers-reduced-motion 后页面直接接近最终状态；
9. 完成 DOM 选择标记回归；
10. 抽样确认独立 HTML、PNG 与 PDF 导出未变化。

## 12. 实施顺序

1. 扩展 PreviewCommand 类型、校验和协议测试。
2. 重构 Slide Runtime，只保留当前页执行 iframe，并建立执行身份比较。
3. 更新 Runtime 单元测试，覆盖导航、重播、元数据更新和迟到消息。
4. 在 IsolatedSlidePreview 接入一次性重播请求。
5. 在 PreviewWorkspace 全屏成功后触发当前页重播。
6. 补充 React 交互测试。
7. 执行 TypeScript、Vitest 和构建验证；本轮不执行真实 Chromium 验收。
8. 核对导出代码和测试无改动、导出行为无回归。

## 13. 风险与约束

### 13.1 页面交互状态丢失

返回页面会清空页内点击状态、滚动状态、视频进度和脚本变量。这是已确认的产品语义，不视为回归。

### 13.2 iframe 重建开销

HTML 字符串仍由现有缓存预取，但图片解析、样式计算和脚本执行会在进入页面时发生。该开销是确保加载动画时机正确的必要代价。本次不增加新的等待或转场界面。

### 13.3 任意页面脚本

Runtime 通过销毁文档保证停止环境，不尝试审计页面脚本。浏览器外部副作用，例如已经发出的网络请求，不能通过销毁 iframe 撤销；现有资源与 sandbox 策略保持不变。

### 13.4 全屏权限

Fullscreen API 依赖用户手势和浏览器权限。请求失败沿用当前行为，不通过页面重播伪装成功。

## 14. 数据与兼容策略

本需求没有持久化结构变化，不需要迁移历史项目，也不增加兼容层。

现有静态页面无需修改。现有依赖 DOMContentLoaded、load、CSS animation 或普通内联脚本初始化的页面自动获得新行为。页面不得依赖未声明的 Runtime 生命周期事件；生产 Prompt 中“不假定存在页面生命周期 API”的现有约束继续成立。

## 15. 决策 Q&A

### Q1：兼容哪些动画？

**问题：** 是让现有“页面加载即执行”的 CSS 动画和 JavaScript 动效自动获得正确行为，还是只支持未来遵循新 Runtime 协议的页面？

**决定：** 兼容现有页面。用户和 Agent 不需要接入新动画协议；页面成为当前页时再运行整份 HTML。

### Q2：再次进入时重置什么？

**问题：** 返回页面时，是重置整张页面，还是只重播 CSS 动画并保留交互状态、视频进度和计数器？

**决定：** 重置整张页面。返回页面等于重新开始，原执行环境整体销毁。

### Q3：哪些基础操作算重新进入？

**决定：**

- 首次打开当前页时播放；
- 翻到其他页再返回时重新播放；
- 当前页 HTML 更新后重新播放；
- 浏览器尺寸变化不重播；
- 不增加手动重播按钮。

本轮最初曾暂定“进入全屏不重播”，后续经 Q8 压力测试发现这会使第一页动画在正式放映前已经结束，因此该项被 Q8 的最终决定取代。

### Q4：什么是减少动态效果？

**问题：** 当操作系统开启 prefers-reduced-motion 时，是否继续把动画近乎瞬时完成？

**决定：** 保留现状并尊重系统无障碍设置，不增加产品级开关。

### Q5：是否包含页面切换转场？

**决定：** 不包含，也不需要。本次只处理单页内部动画的启动时机。

### Q6：音视频是否纳入保证？

**决定：** 不解决浏览器音视频自动播放限制。整页重建会重置媒体，但不承诺自动播放有声内容。

### Q7：是否改动加载与错误界面？

**决定：** 不改，也不作为 TODO 9 的关联需求。

### Q8：开始全屏放映时是否重播当前页？

**决定：** 重播。产品“全屏放映”按钮成功进入全屏后，重建当前页，使第一页或当前页从头播放。

### Q9：离开 HTML 视图后返回是否重播？

**决定：**

- 从概览返回单页时重播；
- 从设计稿返回幻灯片时重播；
- 切换其他项目再返回时重播；
- 浏览器标签页进入后台再返回时不主动重播。

### Q10：哪些内容变化触发当前页重播？

**决定：**

- 当前页 HTML 或 theme_id 变化时重播；
- 页面排序、页码、章节名或公共装饰变化时不重播；
- 其他页面加载或更新时不重播；
- 再次点击已选中的当前页时不重播；
- 开关元素选择、框选或右侧面板时不重播。

## 16. 实施记录

- 实施日期：2026-09-14
- Slide Runtime 继续接收并保存已预取的多页 HTML 描述，但 DOM 中只创建当前页的唯一 iframe。
- 当前页以 slide_id、实际 HTML 字符串和 theme_id 作为执行身份；页码、章节、标题等 Runtime Frame 装饰更新只原位刷新外层 DOM。
- 切换到其他页会销毁旧 iframe，返回时创建新 iframe；当前 HTML、主题或匹配当前 slide_id 的显式重播命令会重建执行实例。
- 应用全屏请求成功后发送一次 replayCurrentSlide；请求失败、退出全屏及普通 fullscreenchange 不触发重播，导航竞争由预期 slide_id 校验隔离。
- DOM 选择桥接、草稿标记、iframe sandbox、父窗口来源校验和 Base CSS 注入保持不变。
- 未修改后端、导出、加载/错误 UI、讲稿、页间转场、缩略图行为或 HTML 数据预取逻辑。
- 验证通过：前端完整 Vitest 59 个文件 / 256 个测试、聚焦 Vitest 3 个文件 / 10 个测试、TypeScript、ESLint（0 error）及生产构建；按最终要求未执行真实 Chromium 验收。
