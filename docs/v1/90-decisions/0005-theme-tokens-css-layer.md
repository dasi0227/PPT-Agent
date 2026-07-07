---
id: ADR-0005
title: design tokens + 公共 CSS 层
status: accepted
owner: shared
depends_on: []
verifies: []
---

# ADR-0005：design tokens + 公共 CSS 层

## 背景

需要支持全局换肤与一处改全局风格（`/overview`），同时保证单页可独立编辑（`/page`）而不互相污染。参考项目（html-ppt-skill）验证了「token 驱动设计系统、换一个文件即换肤」的有效性。

## 决策

采用**集中 design tokens + 公共 CSS 层**：所有主题相关视觉值以 CSS 自定义属性（token）承载于 `common/tokens.css`；所有 slide 引用 token 而非硬编码。`/overview` 修改 tokens.css 实现全局换肤；`/page` 改单页 html，不碰公共层。

## 选项与权衡

| 选项 | 优点 | 缺点 |
|---|---|---|
| tokens + 公共层（选中） | 一处换肤、全局一致、page/overview 隔离自然、机器可校验 | 需约束生成器不硬编码 |
| 每页自带完整样式 | 单页灵活 | 全局一致难、/overview 需逐页改、易漂移 |

## 后果

- 规范见 [design-tokens](../60-design-system/design-tokens.md)；主题清单见 [themes](../60-design-system/themes.md)。
- 生成产物必须引用 token（[html-output-spec](../60-design-system/html-output-spec.md) 强制），并由 lint-slide/lint-tokens 校验。
- 公共层独立成文件（[filesystem-layout](../30-data-model/filesystem-layout.md)），与 `/page`/`/overview` 隔离语义对齐。
- 定义「必需 token 清单」作为换肤兼容基线。

## 状态

accepted（2026-06-30）。
