---
id: OVERVIEW-BACKLOG
title: Backlog（非 MVP，预留接口）
status: approved
owner: shared
depends_on: [OVERVIEW-SCOPE]
verifies: []
---

# Backlog（非 MVP）

这些能力**不在 MVP 实现**，但文档/数据/接口要**预留扩展点**，避免未来返工。每项标注预留位置。

## B1. 图片导出（PNG / PDF）

- **目标**：把任意单页或整套 slide 导出为 PNG/PDF。
- **计划方案**：服务端 headless Chrome（Go + chromedp），按 16:9 截图；多页合并 PDF。
- **预留接口**：
  - API：`40-api/openapi.yaml` 中预留 `POST /projects/{id}/export`（标注 `x-status: backlog`）。
  - 数据：`Slide` 表预留 `last_export_at`（可空）。
- **暂不实现理由**：用户明确暂不需要；引入无头浏览器增加运行环境依赖。

## B2. 框选编辑（多模态输入）

- **目标**：用户在预览页框选一块区域 → 该区域作为「截图 + DOM 范围」一起作为 LLM 输入，做局部精确编辑。
- **计划方案**：
  - 前端：在 iframe 预览上绘制 marquee，捕获 `bbox`；用 `document.elementsFromPoint` / `getBoundingClientRect` 解析命中的 DOM 节点得到 `dom_selector_range`；canvas 裁剪整页截图得到 `screenshot_crop`。
  - 负载形态：`{ screenshot_crop, dom_selector_range, bbox }`。
- **预留接口**：
  - 上下文装配：`50-agent/context-assembly.md` 预留「选区上下文」字段。
  - 编辑指令：自然语言编辑请求体预留可选 `selection` 字段（见 `40-api/rest-endpoints.md`）。
- **暂不实现理由**：像素↔DOM 在缩放/滚动下的稳定映射复杂；DeepSeek 多模态支持有限。首版不做，结构预留。

## B3. PPTX 导入 / 导出转换

- **目标**：导入既有 .pptx 转为 web slides，或反向导出。
- **状态**：不做。无预留（与产品定位偏离）。

## B4. 演讲者模式

- **目标**：逐字稿 + 计时器 + 双窗同步。
- **状态**：不做。

## B5. 部署成可分享链接

- **目标**：一键部署 slide 到公网可访问 URL。
- **状态**：backlog，低优先级。可在 `80-dev/dev-plan.md` 的「未来里程碑」提及。

## 预留接口一览（供实现期核对）

| Backlog 项 | 预留位置 | 标记 |
|---|---|---|
| 图片导出 | openapi `POST /projects/{id}/export`；`Slide.last_export_at` | `x-status: backlog` |
| 框选编辑 | 编辑请求体 `selection?`；context-assembly 选区字段 | 注释 `// backlog` |

## 依赖

- [OVERVIEW-SCOPE](scope.md)
