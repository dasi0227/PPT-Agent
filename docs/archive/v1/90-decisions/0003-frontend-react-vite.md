---
id: ADR-0003
title: 前端 React + Vite + TS + Tailwind + shadcn/ui
status: accepted
owner: frontend
depends_on: []
verifies: []
---

# ADR-0003：前端 React + Vite + TS + Tailwind + shadcn/ui

## 背景

需要网页形式（非终端）的编辑器/预览器；用户授权由 AI 选定前端栈，要求先进、主流。需良好支持沙箱 iframe 预览、流式 SSE、复杂编辑器状态。

## 决策

前端 SPA 采用 **React + Vite + TypeScript + Tailwind CSS + shadcn/ui**，状态用 **Zustand**，slide 渲染于**沙箱 iframe** 并经 postMessage 通信。

## 选项与权衡

| 选项 | 优点 | 缺点 |
|---|---|---|
| React+Vite+TS+Tailwind+shadcn（选中） | 生态成熟、构建快、类型安全、组件可拥有、与 token 协同好 | 需自行集成各部分 |
| Vue + Vite | 同样现代、上手快 | 团队/生态偏好不一 |
| Svelte/SvelteKit | 轻、性能好 | 生态相对小，复杂编辑器组件少 |
| 原生 + Web Components | 零框架锁定 | 复杂编辑器状态开发成本高 |

## 关键澄清

- 前端 SPA 技术栈与**产出的 slide** 无关：slide 是零依赖纯 HTML/CSS/JS（[html-output-spec](../60-design-system/html-output-spec.md)）；React 仅用于编辑器/预览器 App 本身。

## 后果

- 目录与状态模型见 [frontend-structure](../20-architecture/frontend-structure.md)。
- API 类型对齐 [openapi.yaml](../40-api/openapi.yaml)，避免手写漂移。
- 预览采用 iframe + postMessage（[preview-mechanism](../20-architecture/preview-mechanism.md)）。

## 状态

accepted（2026-06-30）。
