---
id: DS-THEMES
title: 预置主题目录
status: approved
owner: shared
depends_on: [DS-TOKENS]
verifies: []
---

# 预置主题目录

> 主题是**资产的一种**（`kind=theme`），遵循 [asset-protocol](asset-protocol.md)。本文件定义主题的语义与预置清单；
> 主题作为 preset 资产由 [seed-assets](seed-assets.md) 载入个人仓库，可经 `/repo` 增删改、经 `apply_theme` 应用到公共层。

主题 = 一组 design token 取值。本文件维护 MVP 预置主题清单及其语义（何时用）。
参考项目有 12–36 套主题；MVP 先精选若干，保证质量与可维护，后续可扩。

## MVP 预置主题（建议 5 套）

| 主题 id | 名称 | 风格 | 适用场景 |
|---|---|---|---|
| `swiss-modern` | 瑞士现代 | 极简、网格、无衬线 | 商务、产品、通用 |
| `tokyo-night` | 东京夜 | 深色、冷色霓虹 | 技术分享、开发者 |
| `editorial-serif` | 杂志衬线 | 浅色、衬线、编辑感 | 内容、观点、文化 |
| `warm-pastel` | 暖柔马卡龙 | 浅色、柔和高饱和 | 教育、图文、亲和 |
| `bold-signal` | 高对比信号 | 强对比、单一强调色 | 路演、发布、冲击力 |

> 每个主题以一个 token 取值文件承载（如 `themes/tokyo-night.css`），切换即换 `tokens.css` 内容来源。

## 主题文件约定

| ID | 约束 |
|---|---|
| `DS-THEMES-001` | 每个主题 MUST 提供 [design-tokens](design-tokens.md) 的全部必需 token |
| `DS-THEMES-002` | 主题 MUST 自包含、可独立预览（不依赖其它主题） |
| `DS-THEMES-003` | 新增主题 MUST 在本表登记 id/名称/风格/适用场景 |
| `DS-THEMES-004` | 主题 MUST 兼顾中英文排版（字体含中文回退） |

## 风格发现（Show-don't-tell）

大纲完成后，Agent SHOULD 推荐若干主题候选，用真实标题页预览供用户挑选（见 [SPEC-OUTLINE-005](../10-spec/feat-outline-generation.md)）。

## 验收标准（Given-When-Then）

- **AC-THEMES-001**（`DS-THEMES-001`）
  - GIVEN 任一预置主题
  - WHEN 校验其 token
  - THEN 必需 token 全部齐备

## 校验方式

```bash
for f in themes/*.css; do node scripts/lint-tokens.mjs "$f"; done
```

## 依赖

- [DS-TOKENS](design-tokens.md)
