---
id: ADR-0006
title: 统一 manifest 插件协议
status: accepted
owner: shared
depends_on: [ADR-0005]
verifies: []
---

# ADR-0006：统一 manifest 插件协议

## 背景

用户希望「个人仓库」收藏的 html 样式 / js 特效能被 AI 理解并作为插件移植进 slide。需要一个统一、可机器校验、可参数化、可预测挂载的协议，避免裸片段不可控。

## 决策

定义**统一 manifest 协议**：每个插件含 `manifest.json`（符合 [plugin-manifest.schema.json](../70-plugins/plugin-manifest.schema.json)）+ 隔离的 html/css/js 资源。manifest 声明 `kind`、`description`（供 AI 检索）、`mount`（挂载点/方式）、`params`（参数 schema）、`assets`、`requires`。

## 选项与权衡

| 选项 | 优点 | 缺点 |
|---|---|---|
| 统一 manifest（选中） | 机器可校验、参数化、挂载可预测、AI 可检索移植 | 需用户/系统按协议封装 |
| 无协议裸片段 | 收藏最简单 | AI 不可控、移植易破坏、不可参数化 |

## 后果

- 协议与流程见 [plugin-protocol](../70-plugins/plugin-protocol.md) 与 [authoring-and-mount](../70-plugins/authoring-and-mount.md)。
- 插件视觉默认引用 token，随主题换肤；fx 插件遵循动效清理约定。
- 移植后产物仍须通过 html-output-spec 校验。
- MVP 仅本地信任、不远程加载；安全审查列为未来增强。
- 与设计系统的动效/版式形成一致的「可拼装资产」生态。

## 状态

accepted（2026-06-30）。
