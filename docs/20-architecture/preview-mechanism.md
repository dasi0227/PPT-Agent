---
id: ARCH-PREVIEW
title: 预览机制（沙箱 iframe + postMessage）
status: approved
owner: frontend
depends_on: [ARCH-FRONTEND, SPEC-VIEWER, DS-HTML-OUTPUT]
verifies: []
---

# 预览机制（沙箱 iframe + postMessage）

## 设计目标

让在线预览与最终演示态**像素一致**，同时支持平滑翻页、页内分步、缩放总览、点击跳转——且不污染宿主 SPA、不重载 iframe。

借鉴参考项目（html-ppt-skill）的两个关键做法：**iframe 隔离预览**与 **postMessage 切页不刷新**。

## 渲染模型

- 每份 Deck 在预览时加载到一个**沙箱 iframe**。
- iframe 内运行轻量预览运行时 `public/slide-runtime/`，负责：切换当前页 active、页内分步、键盘绑定、上报状态。
- 宿主 SPA 与 iframe 经 `postMessage` 双向通信，宿主不直接操作 iframe 内 DOM。

### 一致性保证

- iframe 内使用**与导出/演示完全相同**的公共样式层 + 主题 + 字体 + 16:9 视口。
- 单页预览模式 `?preview=N`：仅渲染第 N 页、无导航 chrome，用于总览缩略图，保证缩略图与大图同源同样式。

## postMessage 协议

### 宿主 → iframe

| type | payload | 作用 |
|---|---|---|
| `goto` | `{ index }` | 切换到第 index 页（toggle active，不重载） |
| `step` | `{ dir: 'next'\|'prev' }` | 页内上一步/下一步 |
| `set-theme` | `{ theme }` | 切主题（换公共样式层 link） |
| `reload-slide` | `{ index }` | 仅重渲染某页（编辑后刷新） |
| `enter-overview` / `exit-overview` | — | 进入/退出总览网格 |

### iframe → 宿主

| type | payload | 作用 |
|---|---|---|
| `ready` | `{ total }` | 运行时就绪，上报总页数 |
| `page-changed` | `{ index }` | 当前页变化（同步深链与 UI） |
| `step-changed` | `{ index, step, steps }` | 分步状态 |
| `request-jump` | `{ index }` | 总览缩略图点击请求跳转 |

## 总览网格

- 方式：宿主渲染 N 个小 iframe（各 `?preview=k`）作为缩略图；或单 iframe 进入 overview 模式由运行时缩放平铺。
- 默认采用**运行时缩放平铺**（性能更好），缩略图点击 → iframe 发 `request-jump` → 宿主发 `goto`。

## 深链

- 宿主监听 `hashchange`，`#/<n>` → 发 `goto`；iframe `page-changed` → 回写 hash。

| ID | 约束 |
|---|---|
| `ARCH-PREVIEW-001` | iframe MUST 带 `sandbox`（允许脚本，禁止 top 导航） |
| `ARCH-PREVIEW-002` | 切页 MUST 经 `goto`，禁止通过改 iframe `src` 实现翻页（避免重载） |
| `ARCH-PREVIEW-003` | 预览所用样式/主题/视口 MUST 与演示/导出态一致 |
| `ARCH-PREVIEW-004` | postMessage MUST 校验 `origin`，忽略非预期来源消息 |

## 验收标准（Given-When-Then）

- **AC-PREVIEW-002**（`ARCH-PREVIEW-002`）
  - GIVEN 预览运行
  - WHEN 翻页
  - THEN iframe `src` 不变、无 `load` 重触发

- **AC-PREVIEW-003**（`ARCH-PREVIEW-003`）
  - GIVEN 总览缩略图（`?preview=N`）与主视图第 N 页
  - WHEN 对比渲染
  - THEN 二者使用相同 CSS/主题/视口（视觉一致）

## 校验方式

- e2e：监听 `load` 次数验证无重载；快照对比缩略图与大图一致性。

## 依赖

- [ARCH-FRONTEND](frontend-structure.md)、[SPEC-VIEWER](../10-spec/feat-online-viewer.md)、[DS-HTML-OUTPUT](../60-design-system/html-output-spec.md)
