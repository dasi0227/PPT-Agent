---
id: DS-ANIMATIONS
title: 动效库
status: approved
owner: shared
depends_on: [DS-TOKENS]
verifies: []
---

# 动效库

> 动态特效是**资产的一种**（`kind=fx`），遵循 [asset-protocol](asset-protocol.md)，作为 preset 由 [seed-assets](seed-assets.md) 载入，可经 `/repo` 增删改、经 `mount_asset` 挂载进页。

定义可挂载到 slide 的动效及其挂载约定。分两类：轻量 **CSS 动画** 与电影感 **Canvas FX**。
参考项目分别提供 27 CSS + 20 Canvas FX；MVP 精选若干，保证质量。

## 挂载约定

- 动效通过约定属性挂载，运行时在进入/分步/自动时启用：
  - CSS 动画：`data-animate="<name>"`，可选 `data-animate-trigger="enter|step|auto"`。
  - Canvas FX：`data-fx="<name>"`，由预览运行时按需初始化。
- slide-json 的 `animations[]` 描述意图，生成阶段落为上述属性。

## MVP CSS 动画（建议集）

| name | 效果 |
|---|---|
| `fade-in` | 淡入 |
| `rise-in` | 上浮淡入 |
| `zoom-pop` | 放大弹入 |
| `blur-in` | 模糊到清晰 |
| `stagger-list` | 列表逐项进入 |
| `counter-up` | 数字滚动 |
| `typewriter` | 打字机 |
| `path-draw` | SVG 描边 |

## MVP Canvas FX（建议集，按需）

| name | 效果 |
|---|---|
| `particle-burst` | 粒子迸发 |
| `starfield` | 星空 |
| `confetti` | 彩纸 |
| `gradient-blob` | 渐变流体 |

## 约束

| ID | 约束 |
|---|---|
| `DS-ANIM-001` | 动效挂载 MUST 用约定属性（`data-animate`/`data-fx`），不内联散落脚本 |
| `DS-ANIM-002` | 动效 MUST 可被「上一步/下一步」控制（trigger=step 时） |
| `DS-ANIM-003` | Canvas FX MUST 在页离开时清理（停止 RAF、移除监听），避免泄漏 |
| `DS-ANIM-004` | 动效配色 MUST 用 token，随主题变化 |
| `DS-ANIM-005` | 动效 MUST 提供「减弱动效」降级（尊重 `prefers-reduced-motion`） |

## 验收标准（Given-When-Then）

- **AC-ANIM-003**（`DS-ANIM-003`）
  - GIVEN 含 `data-fx="particle-burst"` 的页
  - WHEN 切到其它页
  - THEN 该 FX 的 RAF/监听被清理（无持续 CPU 占用）

- **AC-ANIM-005**（`DS-ANIM-005`）
  - GIVEN 系统开启 reduce motion
  - WHEN 渲染
  - THEN 动效降级为即时呈现

## 校验方式

```bash
go test ./internal/designsystem -run TestAnimationRegistry   # name 合法性
# e2e：切页后断言无残留定时器/监听
```

## 依赖

- [DS-TOKENS](design-tokens.md)
