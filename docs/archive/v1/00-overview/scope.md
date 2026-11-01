---
id: OVERVIEW-SCOPE
title: 范围与 MVP 边界
status: approved
owner: shared
depends_on: [OVERVIEW-VISION]
verifies: []
---

# 范围与 MVP 边界

## MVP 必做（In Scope）

| 能力 | 对应规格 |
|---|---|
| 输入主题 → 生成 PPT 大纲 slide-json | [feat-outline-generation](../10-spec/feat-outline-generation.md) |
| 按设计系统（主题/版式/图表/动效）生成 slide html | [feat-slide-generation](../10-spec/feat-slide-generation.md) |
| 自然语言引导编辑 slide html | [feat-nl-editing](../10-spec/feat-nl-editing.md) |
| 在线查看：上下页、上下步骤、缩放总览后点击跳转 | [feat-online-viewer](../10-spec/feat-online-viewer.md) |
| 个人仓库（4 类统一资产协议：layout/component/theme/fx） | [feat-personal-repo](../10-spec/feat-personal-repo.md) |
| 自定义指令 scope：`/current`(默认) `/page` `/overview` `/repo`；mode：`/prompt` `/recap` `/talk` `/ask` | [50-agent/commands](../50-agent/commands/) |
| 流式生成 + Human-in-the-loop（SSE + 控制输入） | [agent-runtime](../20-architecture/agent-runtime.md) |
| 无登录注册 | [api-overview](../40-api/api-overview.md) |

## 非目标（Out of Scope，列入 backlog）

详见 [backlog.md](backlog.md)。简述：

- **图片导出（PNG/PDF）**：计划用服务端 headless Chrome（chromedp），MVP 不实现，数据/接口预留。
- **框选编辑（多模态）**：marquee → `{screenshot_crop, dom_selector_range, bbox}` 多模态输入，MVP 不实现，数据结构预留。
- **PPTX 导入/导出转换**：不做。
- **演讲者模式**：不做。
- **多用户 / 登录 / 协作**：不做（单用户本地工具定位）。

## MVP 的明确边界（避免范围蔓延）

- 单用户、单机运行，无鉴权。
- LLM 仅接入 DeepSeek API；不做多模型路由。
- 产出 slide 为固定 16:9、零运行时依赖（webfont/highlight.js/chart.js 等 CDN 资源除外）。
- 持久化用本机 SQLite + 文件系统，不引入外部数据库服务。

## 依赖

- [OVERVIEW-VISION](vision.md)
