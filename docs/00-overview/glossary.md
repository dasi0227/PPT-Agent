---
id: OVERVIEW-GLOSSARY
title: 术语表
status: approved
owner: shared
depends_on: []
verifies: []
---

# 术语表

全项目术语的唯一定义来源。其它文档引用术语时以本表为准。

| 术语 | 英文 / 标识 | 定义 |
|---|---|---|
| 项目 | Project | 一份 PPT 工程的顶层容器，对应一个 `work_dir`。一个项目含一个 Deck，可含多条 Thread。是隔离与执行锁的单元。 |
| 线程 | Thread | 一条可恢复的对话历史（对齐 Codex thread）。一个 Project 可有多条 Thread，**共享该 Project 产物**，仅隔离对话历史。 |
| 演示文稿 | Deck | 一组有序 slide 的集合，归属一个 Project。 |
| 幻灯片 | Slide | 单页，含 slide-json（结构化元数据）与 slide html/css/js（产出物）。 |
| 大纲 | Outline | Deck 的结构化骨架，即 slide-json 数组，描述每页的标题、要点、版式、图表意图。 |
| slide-json | slide-json | 单页的结构化描述（schema 见 30-data-model）。是「内容与意图」，区别于最终渲染的 html。 |
| 设计系统 | Design System | 主题 token + 版式库 + 图表样式 + 动效库 + HTML 产出规范的总称（见 60-design-system）。 |
| 设计令牌 | Design Token | 颜色/字体/间距/圆角/阴影等原子设计变量，以 CSS 自定义属性承载。 |
| 公共样式层 | Common CSS Layer | 所有 slide 共享的全局样式文件，承载 design tokens。`/overview` 修改的就是它。 |
| 主题 | Theme | 一套 design token 取值组合，换一个主题即全局换肤。 |
| 版式 | Layout | 单页的结构模板（cover/toc/bullets/two-column/kpi-grid/table/code/timeline 等）。 |
| 动效 | Animation / FX | CSS 动画或 Canvas 特效，按约定挂载到 slide 元素。 |
| 运行 | Run | 一次 Agent 执行单元（turn），挂在某 Thread 下，从用户指令开始，经 harness 循环到产出结束。有生命周期状态机。 |
| 检查点 | Checkpoint | Run 执行过程中可暂停、可排空输入队列、可发起提问的节点。 |
| 人在环 | Human-in-the-loop (HITL) | 用户在 Run 执行中途注入输入、回答 Agent 提问、影响后续执行的能力。 |
| 控制输入 | Control Input | 通过 `POST /runs/{id}/input` 向运行中的 Run 注入的消息。 |
| needs_input | needs_input | Run 暂停并等待用户输入时发出的 SSE 事件。 |
| 指令 | Command | 以 `/` 开头的用户指令。scope：`/current` `/page` `/overview` `/repo`；mode：`/prompt` `/recap` `/talk` `/ask`。 |
| 模式 | Mode | Agent 的行为模式（如 `/talk` 只说不做、`/ask` 必须提问）。 |
| 资产 | Asset | 个人仓库中可复用的单元，遵循统一资产协议；4 类：layout/component/theme/fx。 |
| 整页板式 | Layout | 整页盒子布局骨架（粒度=一整页）。资产 kind 之一。 |
| 组件样式 | Component | 页内可复用的小块（粒度<一页，如卡片/徽标）。资产 kind 之一。 |
| 动态特效 | FX | js 驱动的动效（含 init/cleanup 契约）。资产 kind 之一。 |
| 清单 | Manifest | 资产的元数据描述文件（manifest.json），统一信封 + 定义参数、挂载点、载荷。 |
| 挂载点 | Mount Point | 资产被注入 slide 时的目标位置约定。 |
| 个人仓库 | Personal Repo | 4 类资产的本地仓库（出厂预置 + 用户新增，同协议）。 |
| 版本 | Version | Slide / Deck 的一次快照，支持回滚。 |
| 工作目录 | work_dir | 单个 Project 的根目录，`state.json` 位于其根。 |
| 预览 | Preview | 在沙箱 iframe 中渲染 slide 的在线查看能力。 |
| 总览 | Overview Grid | 缩略所有页、点击跳转的网格视图。 |

## 依赖

无。
