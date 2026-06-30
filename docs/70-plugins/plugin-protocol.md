---
id: PLUGIN-PROTOCOL
title: 插件协议（统一 manifest）
status: approved
owner: shared
depends_on: [ADR-0006, DS-HTML-OUTPUT]
verifies: []
---

# 插件协议（统一 manifest）

## 目标

把用户收藏的 html 样式片段 / js 动效，统一为遵循 **manifest 协议**的插件，使 AI 能理解并把它们
**移植进 slide**。统一协议 = 可被机器校验、可参数化、可预测挂载。

## 插件构成

```
_plugins/<plugin_id>/
├── manifest.json     # 元数据 + 参数 schema + 挂载约定（符合 plugin-manifest.schema.json）
├── template.html     # 样式型：要注入的 html 结构片段（可选）
├── style.css         # 可选样式
└── effect.js         # 特效型：Canvas/JS 特效（可选）
```

## manifest 关键字段（详见 [schema](plugin-manifest.schema.json)）

| 字段 | 说明 |
|---|---|
| `name` / `version` | 名称与版本 |
| `kind` | `style`（html/css 片段）或 `fx`（js 特效） |
| `description` | 一句话用途（供 AI 检索/匹配） |
| `mount` | 挂载约定：`target`（选择器/语义位）、`position`（`append`/`prepend`/`replace`/`wrap`） |
| `params` | 参数 schema（JSON Schema 子集），AI 据此填参 |
| `assets` | 资源文件清单（html/css/js） |
| `requires` | 可选依赖声明（如需某 token、某 CDN） |

## 挂载与移植流程

```
AI 检索个人仓库（按 description 语义匹配用户意图）
  │
  ▼
读取 manifest → 校验 schema → 解析 mount + params
  │
  ▼
按 params schema 填参（缺参走默认/在 ask 模式提问）
  │
  ▼
按 mount.position 注入 template.html / 挂载 effect.js 到目标 slide
  │
  ▼
校验结果仍满足 html-output-spec → 落盘 + 版本
```

## 约束

| ID | 约束 |
|---|---|
| `PLUGIN-001` | manifest MUST 符合 [plugin-manifest.schema.json](plugin-manifest.schema.json) |
| `PLUGIN-002` | 移植后目标 slide MUST 仍满足 [html-output-spec](../60-design-system/html-output-spec.md) |
| `PLUGIN-003` | 插件视觉 SHOULD 用 token，以随主题换肤 |
| `PLUGIN-004` | `fx` 插件 MUST 遵循动效清理约定（页离开清理，见 [DS-ANIM-003](../60-design-system/animations.md)） |
| `PLUGIN-005` | params 缺省值 MUST 完备，使无参移植也能渲染 |
| `PLUGIN-006` | MVP 仅本地信任，不执行远程加载；插件资源 MUST 为本地文件 |

## 验收标准（Given-When-Then）

- **AC-PLUGIN-001**（`PLUGIN-001`）
  - GIVEN 一个插件 manifest
  - WHEN 用 schema 校验
  - THEN 通过

- **AC-PLUGIN-002**（`PLUGIN-002`）
  - GIVEN 把 `particle-burst` 移植进封面
  - WHEN lint-slide
  - THEN 封面仍通过 html-output-spec

## 校验方式

```bash
# manifest schema 校验
python -c "import json,jsonschema,sys; s=json.load(open('docs/70-plugins/plugin-manifest.schema.json')); jsonschema.validate(json.load(open(sys.argv[1])),s)" <manifest.json>
node scripts/lint-slide.mjs <target-slide.html>
```

## 依赖

- [ADR-0006](../90-decisions/0006-plugin-manifest-protocol.md)、[DS-HTML-OUTPUT](../60-design-system/html-output-spec.md)
