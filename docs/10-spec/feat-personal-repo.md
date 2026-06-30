---
id: SPEC-REPO
title: 个人仓库（插件）
status: approved
owner: shared
depends_on: [PLUGIN-PROTOCOL, AGENT-CONTEXT]
verifies: []
---

# 功能规格：个人仓库（收藏样式/特效作为插件）

## 目标

用户把满意的 html 样式片段或 js 动效收藏为**插件**，沉淀进个人仓库；这些插件遵循统一 manifest 协议，
便于 Agent 理解并在后续生成/编辑时把插件**移植进 slide**。

## 范围与非目标

- **范围**：插件的收藏/列表/查看/删除、manifest 校验、Agent 检索与移植。
- **非目标**：插件市场/分享/远程仓库（不做）；插件沙箱安全审查（MVP 仅本地信任）。

## 规格正文

| ID | 需求 | 优先级 |
|---|---|---|
| `SPEC-REPO-001` | 用户 MUST 能把一段 html/css/js 片段收藏为插件，系统据 [plugin protocol](../70-plugins/plugin-protocol.md) 生成/校验 manifest | P0 |
| `SPEC-REPO-002` | 插件 manifest MUST 通过 [plugin-manifest schema](../70-plugins/plugin-manifest.schema.json) 校验 | P0 |
| `SPEC-REPO-003` | MUST 能列出、查看、删除个人仓库中的插件 | P0 |
| `SPEC-REPO-004` | Agent 在生成/编辑时 MUST 能检索个人仓库，并按 manifest 的挂载约定把插件移植进目标 slide | P1 |
| `SPEC-REPO-005` | 插件含参数 schema 时，移植 MUST 按参数填充；缺参 MUST 走默认值或在 `/ask` 模式提问 | P1 |
| `SPEC-REPO-006` | 移植插件 MUST 不破坏目标 slide 的 html-output-spec 合规性 | P0 |

## 验收标准（Given-When-Then）

- **AC-REPO-001**（`SPEC-REPO-001/002`）
  - GIVEN 一段 Canvas 粒子特效 js
  - WHEN 用户收藏为插件
  - THEN 生成的 manifest 通过 schema 校验，且插件出现在仓库列表

- **AC-REPO-004**（`SPEC-REPO-004`）
  - GIVEN 仓库中有插件 `particle-burst`
  - WHEN 用户说「给封面加上我收藏的粒子特效」
  - THEN Agent 检索到该插件并按挂载约定注入封面页，渲染生效

## 校验方式

```bash
go test ./internal/plugin -run TestPluginCRUDAndValidate
# manifest schema 校验：validate against plugin-manifest.schema.json
```

## 依赖

- [PLUGIN-PROTOCOL](../70-plugins/plugin-protocol.md)
- [AGENT-CONTEXT](../50-agent/context-assembly.md)
