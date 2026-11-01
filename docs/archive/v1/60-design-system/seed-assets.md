---
id: DS-SEED
title: 出厂预置资产与冷启动
status: approved
owner: shared
depends_on: [ASSET-PROTOCOL, DS-THEMES, DS-LAYOUTS, DS-COMPONENTS, DS-ANIMATIONS]
verifies: []
---

# 出厂预置资产（seed）与冷启动

## 目的

产品首次运行时，个人仓库已有一批 **preset 资产**，让工具**开箱可用**——用户无需先造资产即可生成 PPT。
preset 与 user 资产同协议、同表，仅 `source` 不同。

## 预置清单（seed）

### theme（5 套，见 [themes](themes.md)）
`swiss-modern`、`tokyo-night`、`editorial-serif`、`warm-pastel`、`bold-signal`

### layout（覆盖核心版式，见 [layouts](layouts.md)）
`cover`、`toc`、`section-divider`、`bullets`、`two-column`、`kpi-grid`、`table`、`code`、`timeline`、`comparison`、`image-hero`、`cta`、`thanks`（其余版式可后续补）

### component（见 [components](components.md)）
`stat-badge`、`quote-block`、`feature-card`、`kv-list`、`svg-bar`

### fx（见 [animations](animations.md)）
`fade-in`、`rise-in`、`stagger-list`、`counter-up`、`particle-burst`、`starfield`

## 冷启动策略

| ID | 策略 |
|---|---|
| `DS-SEED-001` | 首次启动 MUST 把 seed 资产载入个人仓库（`source=preset`），写入 `_assets/` 与 SQLite |
| `DS-SEED-002` | seed 资产 MUST 通过 asset-manifest schema 校验后才载入（与 user 资产同校验） |
| `DS-SEED-003` | 用户 MAY 本地修改 preset 资产；修改后 `source` 仍记 preset 但版本递增，首次修改前保留 factory baseline，可回滚到出厂态；preset 禁止删除，删除仅允许 user 资产 |
| `DS-SEED-004` | 生成时若某 kind 无可用资产，MUST 有兜底：layout 回退 `bullets`，theme 回退首个 preset |
| `DS-SEED-005` | seed 内容版本随产品发布管理；升级产品 SHOULD 幂等更新未被用户改动的 preset |

## seed 存放（仓库内）

```
backend/seed/
├── common/base.css                                  # 主题无关公共基座（16:9 舞台 + 基础样式，只引用 token）
└── assets/
    ├── themes/<name>/{manifest.json, tokens.css}
    ├── layouts/<name>/{manifest.json, template.html, style.css}
    ├── components/<name>/{manifest.json, template.html, style.css}
    └── fx/<name>/{manifest.json, effect.js}
```
启动时由 `internal/asset` 的 seeding 逻辑载入 work_root 的 `_assets/`。运行时仓库与 seed 源**同构**（均按 kind 分 `themes`/`layouts`/`components`/`fx` 子目录），seeding 为 `seed/assets/<kind_dir>/<name>` → `_assets/<kind_dir>/<asset_id>` 的直接映射（见 [filesystem-layout](../30-data-model/filesystem-layout.md)）。`seed/common/` 一并载入 `_assets/common/` 作为公共基座源。

> **公共样式层生成**：整套生成时，`common/base.css` 取自 seed 基座（主题无关）；`common/tokens.css` 为**所选 theme 的 tokens.css 拷贝**——换肤即换 tokens.css 内容来源（[DS-TOKENS-003](design-tokens.md)），base 结构不动。

## 验收标准（Given-When-Then）

- **AC-SEED-001**（`DS-SEED-001/002`）
  - GIVEN 全新环境首次启动
  - WHEN 初始化
  - THEN `_assets/` 含全部 seed 资产，且 `GET /assets` 返回它们（source=preset），全部通过 schema 校验

- **AC-SEED-004**（`DS-SEED-004`）
  - GIVEN 仓库无 theme 资产（异常）
  - WHEN 生成
  - THEN 回退首个可用 preset，不崩溃

## 校验方式

```bash
go test ./internal/asset -run 'TestSeedLoad|TestColdStartFallback'
for f in backend/seed/assets/**/manifest.json; do python -c "..."; done   # 全部 seed manifest 校验
```

## 依赖

- [ASSET-PROTOCOL](asset-protocol.md)、[DS-THEMES](themes.md)、[DS-LAYOUTS](layouts.md)、[DS-COMPONENTS](components.md)、[DS-ANIMATIONS](animations.md)
