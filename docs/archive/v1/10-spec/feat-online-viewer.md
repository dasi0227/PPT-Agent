---
id: SPEC-VIEWER
title: 在线查看
status: approved
owner: frontend
depends_on: [ARCH-PREVIEW, SPEC-GEN]
verifies: []
---

# 功能规格：在线查看

## 目标

在浏览器中以沙箱 iframe 渲染 slide，提供翻页、分步、缩放总览、点击跳转的演示与浏览体验。

## 范围与非目标

- **范围**：上下页、上下步骤（页内分步动画）、缩放总览网格、点击缩略图跳转、深链定位。
- **非目标**：演讲者模式（不做）；导出（backlog）。

## 规格正文

| ID | 需求 | 优先级 |
|---|---|---|
| `SPEC-VIEWER-001` | slide MUST 在沙箱 iframe 中渲染，使用与最终演示一致的 CSS/主题/字体/视口 | P0 |
| `SPEC-VIEWER-002` | MUST 支持上一页/下一页（键盘 ←/→/Space + UI 按钮） | P0 |
| `SPEC-VIEWER-003` | MUST 支持页内「上一步/下一步」（逐步展开页内分步动画元素） | P1 |
| `SPEC-VIEWER-004` | MUST 提供缩放总览网格，缩略显示全部页 | P0 |
| `SPEC-VIEWER-005` | 在总览中点击任一缩略图 MUST 跳转到该页 | P0 |
| `SPEC-VIEWER-006` | 切页 MUST 平滑无刷新（postMessage 切换 active，不重载 iframe） | P1 |
| `SPEC-VIEWER-007` | MUST 支持深链 `#/<page>` 直接定位到指定页 | P1 |
| `SPEC-VIEWER-008` | 编辑某页后预览 MUST 能刷新该页而不重载整套 | P1 |

## 交互与按键

| 操作 | 触发 |
|---|---|
| 上/下页 | `←` `→` `Space` / UI 箭头 |
| 上/下步骤 | `↑` `↓` / UI 步进 |
| 总览网格 | `O` / UI 网格按钮 |
| 跳转 | 总览中点击缩略图 / 深链 `#/N` |

## 验收标准（Given-When-Then）

- **AC-VIEWER-001**（`SPEC-VIEWER-001`）
  - GIVEN 一份已生成演示文稿
  - WHEN 打开预览
  - THEN 每页在独立 iframe 渲染，视觉与导出/演示态一致（同 CSS/主题/视口）

- **AC-VIEWER-005**（`SPEC-VIEWER-004/005`）
  - GIVEN 8 页演示文稿
  - WHEN 打开总览并点击第 6 张缩略图
  - THEN 主视图跳转到第 6 页

- **AC-VIEWER-006**（`SPEC-VIEWER-006`）
  - GIVEN 正在第 2 页
  - WHEN 点下一页
  - THEN 切到第 3 页且 iframe 未发生整页重载（无白屏闪烁）

## 校验方式

- 端到端：Playwright/浏览器自动化脚本验证翻页、总览点击跳转、深链定位、无刷新切页。
- `SPEC-VIEWER-006`：监听 iframe `load` 事件次数，切页不应触发新的 `load`。

## 依赖

- [ARCH-PREVIEW](../20-architecture/preview-mechanism.md)
- [SPEC-GEN](feat-slide-generation.md)
