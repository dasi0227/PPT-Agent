---
id: ARCH-FRONTEND
title: 前端结构（React + Vite）
status: approved
owner: frontend
depends_on: [ARCH-SYSTEM, ARCH-PREVIEW, ADR-0003]
verifies: []
---

# 前端结构（React + Vite + TS + Tailwind + shadcn/ui）

## 技术选型

| 维度 | 选型 | 理由 |
|---|---|---|
| 框架 | React 18+ | 生态成熟、组件化 |
| 构建 | Vite | 快、现代、原生 ESM |
| 语言 | TypeScript | 类型安全，契约对齐后端 |
| 样式 | Tailwind CSS | 原子化、与设计系统 token 协同 |
| 组件 | shadcn/ui | 可拥有、可定制、无运行时锁定 |
| 状态 | Zustand | 轻量、适合编辑器状态 |
| 预览 | 沙箱 iframe + postMessage | 隔离、与演示态一致（见 [preview-mechanism](preview-mechanism.md)） |
| SSE | 原生 `EventSource` / fetch-stream | 接收 Run 事件流 |

> 注意：前端 SPA 的技术栈与「产出的 slide」无关。slide 是零依赖纯 HTML/CSS/JS（见 [html-output-spec](../60-design-system/html-output-spec.md)）；React 只用于**编辑器/预览器 App 本身**。

## 目录结构

```
frontend/
├── index.html
├── vite.config.ts
├── tailwind.config.ts
├── src/
│   ├── main.tsx
│   ├── App.tsx
│   ├── api/                   # 后端契约客户端
│   │   ├── client.ts          # fetch 封装、错误码处理
│   │   ├── sse.ts             # SSE 订阅封装
│   │   └── types.ts           # 与 openapi 对齐的 TS 类型
│   ├── stores/                # Zustand
│   │   ├── deckStore.ts       # 当前 deck/slides/当前页
│   │   ├── runStore.ts        # 当前 run 状态、事件流、HITL
│   │   └── uiStore.ts         # 模式（talk/ask）、面板开合
│   ├── features/
│   │   ├── chat/              # 对话/指令输入面板
│   │   ├── viewer/            # 预览器（iframe host + 翻页 + 总览）
│   │   │   ├── PreviewFrame.tsx
│   │   │   ├── OverviewGrid.tsx
│   │   │   └── usePostMessage.ts
│   │   ├── editor/            # 编辑相关 UI（范围选择、版本回滚）
│   │   └── assets/            # 个人仓库 UI（4 类资产管理）
│   ├── components/            # shadcn/ui 衍生通用组件
│   └── lib/                   # 工具（深链解析、key 绑定）
└── public/
    └── slide-runtime/         # 注入 iframe 的预览运行时（切页/分步/缩放）
```

## 关键模块约束

| ID | 约束 |
|---|---|
| `ARCH-FE-001` | 预览 MUST 用 `sandbox` iframe 渲染 slide，宿主与 iframe 经 postMessage 通信 |
| `ARCH-FE-002` | 切页 MUST 通过 postMessage 控制 active，不重载 iframe（[SPEC-VIEWER-006](../10-spec/feat-online-viewer.md)） |
| `ARCH-FE-003` | API 类型 MUST 由 [openapi.yaml](../40-api/openapi.yaml) 生成/对齐，避免手写漂移 |
| `ARCH-FE-004` | Run 事件流 MUST 经 `runStore` 单一来源管理，UI 订阅派生 |
| `ARCH-FE-005` | 指令（`/page` 等）在输入层解析并标注 scope/mode，再发后端 |

## 状态模型（Zustand）

- `runStore`：`{ runId, status, events[], pendingInput, mode }`，承载 SSE 事件与 HITL。
- `deckStore`：`{ deckId, slides[], currentPage, version }`。
- `uiStore`：`{ mode: 'normal'|'talk'|'ask', overviewOpen, panels }`。
- 遵循单一状态游标（参考用户偏好：避免冗余标志位造成「双重真相」）。

## 验收标准（Given-When-Then）

- **AC-ARCH-FE-002**（`ARCH-FE-002`）
  - GIVEN 预览器已加载
  - WHEN 连续翻页 5 次
  - THEN iframe `load` 事件计数不增加（无重载）

## 校验方式

```bash
pnpm tsc --noEmit
pnpm test            # 组件/状态单测
pnpm e2e             # 预览翻页/总览/深链 端到端
```

## 依赖

- [ARCH-SYSTEM](system-overview.md)、[ARCH-PREVIEW](preview-mechanism.md)、[ADR-0003](../90-decisions/0003-frontend-react-vite.md)
