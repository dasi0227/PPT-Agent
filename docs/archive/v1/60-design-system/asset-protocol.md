---
id: ASSET-PROTOCOL
title: 资产协议（统一信封 + 4 类载荷）
status: approved
owner: shared
depends_on: [DS-TOKENS, DS-HTML-OUTPUT, ADR-0009]
verifies: []
---

# 资产协议（个人仓库统一协议）

## 核心理念：一切皆资产

**没有「自带 vs 收藏」的二分**。个人仓库是唯一中心概念，持有 **4 类资产**，共享一个**统一信封 + 类型化载荷**。
产品出厂**预置（preset）**一批资产开箱可用，用户后续**新增（user）**同类资产——同一套协议、同一套数据结构。

```
统一信封（4 类共有）: id, name, version, kind, source, description, tags, preview, mount
   ├── kind=layout      整页板式   载荷: html(整页盒子骨架) + css
   ├── kind=component   组件样式   载荷: html(页内小块) + css + params
   ├── kind=theme       配色主题   载荷: tokens(必需 token 全集) + 字体引入
   └── kind=fx          动态特效   载荷: js(init/cleanup) + params
```

**扩展性**：未来加第 5 类资产，只需加一个载荷 schema，信封与仓库 CRUD 不变。

## 四类资产对比

| kind | 粒度 | 载荷 | 应用方式 | 典型 |
|---|---|---|---|---|
| `layout` | 整页 | html 骨架 + css | 作为新页骨架 / `mount_asset` 替换页结构 | cover、two-column、kpi-grid |
| `component` | 页内小块 | html 片段 + css + params | `mount_asset` 注入页内某处 | 霓虹卡片、标题徽标、引用块、SVG 图表 |
| `theme` | 全局 | tokens 全集 + 字体 | `apply_theme` 写入公共层，作用所有页 | tokyo-night、warm-pastel |
| `fx` | 页内/页级 | js + params | `mount_asset` 挂载 + init/cleanup | 粒子、星空、canvas 图表 |

> 图表不独立成类：静态 SVG/HTML 图表 = `component`；动态 canvas/chart.js 图表 = `fx`。

## 统一信封字段

| 字段 | 说明 |
|---|---|
| `id` / `name` / `version` | 标识与语义化版本 |
| `kind` | layout / component / theme / fx |
| `source` | preset（出厂）/ user（新增） |
| `description` | 一句话用途（供 AI 语义检索匹配） |
| `tags` | 检索标签 |
| `preview` | 预览图/示例引用（可选） |
| `mount` | 挂载约定：`target`（选择器/语义位）+ `position`（append/prepend/replace/wrap）；theme 无需 mount |
| `params` | 参数 schema（component/fx 用）；AI 据此填参 |

详见机器可校验的 [asset-manifest.schema.json](asset-manifest.schema.json)。

## 应用 / 移植流程（经 harness 工具）

| 操作 | 工具 | scope |
|---|---|---|
| 检索资产 | `search_assets(kind, query)` | all |
| 把 layout/component/fx 移植进页 | `mount_asset(slide_idx, asset_id, params)` | current/page/overview |
| 应用 theme 到公共层 | `apply_theme(theme_asset_id)` | overview |
| 增删改资产本身 | `create/patch/delete_asset` | repo |

```
AI 检索仓库（按 description/tags 匹配意图）
  → 读 manifest → 校验 → 解析 mount + params
  → 按 params schema 填参（缺参走默认 / ask 模式提问）
  → 按 mount.position 注入目标页 / 或 apply_theme 写公共层
  → validate_slide 校验 → 落盘 + 版本
```

## 约束

| ID | 约束 |
|---|---|
| `ASSET-001` | manifest MUST 符合 [asset-manifest.schema.json](asset-manifest.schema.json) |
| `ASSET-002` | `theme` 资产 MUST 提供必需 token 全集（[design-tokens](design-tokens.md)），不残缺 |
| `ASSET-003` | 移植/应用后产物 MUST 仍满足 [html-output-spec](html-output-spec.md) |
| `ASSET-004` | 资产视觉 SHOULD 用 token，以随主题换肤 |
| `ASSET-005` | `fx` 资产 MUST 遵循 init/cleanup 契约（页离开清理，见 [animations](animations.md)） |
| `ASSET-006` | `component`/`fx` 的 params MUST 有完备默认值，无参也能渲染 |
| `ASSET-007` | preset 与 user 资产同协议、同表、同校验；仅 `source` 不同 |
| `ASSET-008` | MVP 仅本地信任，不远程加载；资产资源 MUST 为本地文件 |

## 验收标准（Given-When-Then）

- **AC-ASSET-001**（`ASSET-001`）
  - GIVEN 任一资产 manifest（任一 kind）
  - WHEN 用 asset-manifest schema 校验
  - THEN 通过

- **AC-ASSET-002**（`ASSET-002`）
  - GIVEN 一个 theme 资产缺少某必需 token
  - WHEN 校验
  - THEN 失败并指出缺失 token

- **AC-ASSET-003**（`ASSET-003`）
  - GIVEN 把组件移植进封面
  - WHEN lint-slide
  - THEN 封面仍通过 html-output-spec

## 校验方式

```bash
# manifest schema 校验
python -c "import json,jsonschema,sys; s=json.load(open('docs/60-design-system/asset-manifest.schema.json')); jsonschema.validate(json.load(open(sys.argv[1])),s)" <manifest.json>
node scripts/lint-slide.mjs <target-slide.html>
node scripts/lint-tokens.mjs <theme-asset>/tokens.css   # theme 资产 token 全集
```

## 依赖

- [DS-TOKENS](design-tokens.md)、[DS-HTML-OUTPUT](html-output-spec.md)、[ADR-0009](../90-decisions/0009-unified-asset-protocol.md)
