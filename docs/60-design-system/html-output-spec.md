---
id: DS-HTML-OUTPUT
title: slide HTML 产出规范
status: approved
owner: shared
depends_on: [DS-TOKENS, DS-LAYOUTS]
verifies: []
---

# slide HTML 产出规范

定义 Agent 生成的 slide html 必须满足的工程约束。是生成/编辑产物落盘前的**校验清单来源**。

## 硬性约束

| ID | 约束 |
|---|---|
| `DS-HTML-001` | 固定 16:9 舞台（基于 `--stage-w`/`--stage-h`，1920×1080 基准），自适应缩放到视口 |
| `DS-HTML-002` | 零运行时强依赖：单页可独立渲染；仅允许 CDN webfont / highlight.js / chart.js 等可选增强 |
| `DS-HTML-003` | 主题相关视觉值 MUST 用 `var(--token)`，禁止硬编码主题色/字号（见 [DS-TOKENS-001](design-tokens.md)） |
| `DS-HTML-004` | MUST 引用公共样式层（`common/tokens.css` + `common/base.css`） |
| `DS-HTML-005` | 基本可访问性：语义标签、足够对比度、图片 `alt`、标题层级合理 |
| `DS-HTML-006` | 中英文一等公民：字体含中文回退，排版兼顾 CJK |
| `DS-HTML-007` | 动效用约定属性挂载（见 [animations](animations.md)），不内联散落脚本逻辑 |
| `DS-HTML-008` | 页内分步元素 MUST 用约定标记（如 `data-step`），供 viewer「上一步/下一步」 |
| `DS-HTML-009` | 单页 html MUST 可被 `?preview=N` 单独渲染（与预览机制契合） |
| `DS-HTML-010` | 代码可读、必要处注释（参考偏好：注释克制，仅解释「为什么」） |

## 校验清单（lint-slide）

生成/编辑产物落盘前 MUST 通过 `scripts/lint-slide.mjs`，检查项：

1. 含 16:9 舞台容器与缩放逻辑。
2. 引用公共层（tokens + base）。
3. 无硬编码主题色（`#rrggbb` 主题色）/硬编码字号。
4. 图片含 `alt`。
5. 动效用约定属性。
6. 无被禁的重运行时依赖（白名单外的 `<script src>`）。
7. 结构可被 `?preview=N` 渲染。

## 验收标准（Given-When-Then）

- **AC-HTML-001**（`DS-HTML-001/003/004`）
  - GIVEN 一个生成的 slide html
  - WHEN 运行 lint-slide
  - THEN 全部检查项通过

- **AC-HTML-002**（`DS-HTML-002`）
  - GIVEN 断网环境（无 CDN）
  - WHEN 打开 slide
  - THEN 结构与排版仍可用（增强降级，不白屏）

## 校验方式

```bash
node scripts/lint-slide.mjs slides/*/index.html
```

## 依赖

- [DS-TOKENS](design-tokens.md)、[DS-LAYOUTS](layouts.md)
