---
id: DS-TOKENS
title: 设计令牌与公共样式层
status: approved
owner: shared
depends_on: [ADR-0005]
verifies: []
---

# 设计令牌（Design Tokens）与公共样式层

## 核心理念

一套**集中的 design tokens** 驱动整份 PPT 的视觉。换一组 token 即全局换肤。
tokens 承载于**公共样式层**（`common/tokens.css`），所有 slide 引用它而非硬编码——
这是 `/overview` 指令的根基，也是参考项目「token 驱动设计系统」的产品化。

## token 分类

| 类别 | 变量前缀 | 示例 |
|---|---|---|
| 颜色 | `--color-*` | `--color-bg`、`--color-fg`、`--color-primary`、`--color-accent`、`--color-muted` |
| 字体 | `--font-*` | `--font-sans`、`--font-serif`、`--font-mono` |
| 字号 | `--text-*` | `--text-title`、`--text-h1`、`--text-body`、`--text-caption` |
| 间距 | `--space-*` | `--space-1`…`--space-8` |
| 圆角 | `--radius-*` | `--radius-sm`、`--radius-md`、`--radius-lg` |
| 阴影 | `--shadow-*` | `--shadow-card`、`--shadow-pop` |
| 舞台 | `--stage-*` | `--stage-w`(1920)、`--stage-h`(1080)、16:9 |

## 公共样式层结构

```css
/* common/tokens.css —— /overview 修改此文件 */
:root {
  /* 颜色 */
  --color-bg: #0f1117;
  --color-fg: #e6e6e6;
  --color-primary: #6aa3ff;
  --color-accent: #ffd166;
  --color-muted: #8a8f98;
  /* 字体 */
  --font-sans: "Inter","Noto Sans SC",system-ui,sans-serif;
  --font-serif: "Noto Serif SC",Georgia,serif;
  --font-mono: "JetBrains Mono",monospace;
  /* 字号 / 间距 / 圆角 / 阴影 / 舞台 ... */
}
```

`common/base.css` 提供 16:9 舞台与共享基元（不含主题色值，只引用 token）。

## 约束

| ID | 约束 |
|---|---|
| `DS-TOKENS-001` | 所有主题相关视觉值 MUST 经 token，slide html 禁止硬编码十六进制色/像素字号 |
| `DS-TOKENS-002` | 公共样式层 MUST 定义全部「必需 token」集合；主题切换只改取值不删 key |
| `DS-TOKENS-003` | `/overview` MUST 只改 `tokens.css`，不动 base.css 结构与单页 html |
| `DS-TOKENS-004` | 必需 token 清单变更 MUST 同步更新本文件与校验脚本 |

## 必需 token 清单（校验基线）

`--color-bg`、`--color-fg`、`--color-primary`、`--color-accent`、`--color-muted`、
`--font-sans`、`--text-title`、`--text-body`、`--space-4`、`--radius-md`、`--shadow-card`、
`--stage-w`、`--stage-h`。

## 验收标准（Given-When-Then）

- **AC-TOKENS-001**（`DS-TOKENS-001`）
  - GIVEN 任一生成的 slide html
  - WHEN 扫描样式
  - THEN 无硬编码主题色（`#rrggbb` 主题色）/ 硬编码字号，均为 `var(--token)`

- **AC-TOKENS-002**（`DS-TOKENS-002`）
  - GIVEN 切换主题
  - WHEN 校验 tokens.css
  - THEN 必需 token 清单全部存在（仅取值不同）

## 校验方式

```bash
node scripts/lint-tokens.mjs common/tokens.css     # 必需 token 完整性
node scripts/lint-slide.mjs slides/*/index.html    # 无硬编码主题值
```

## 依赖

- [ADR-0005](../90-decisions/0005-theme-tokens-css-layer.md)
