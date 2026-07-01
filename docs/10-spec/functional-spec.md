---
id: SPEC-FUNCTIONAL
title: 功能总规格
status: approved
owner: shared
depends_on: [OVERVIEW-SCOPE, OVERVIEW-FLOWS]
verifies: []
---

# 功能总规格

本文件给出能力全景与产品级约束，细节下沉到各 `feat-*.md`。

## 能力全景

```
┌─────────────────────────────────────────────────────────────┐
│                     AI PPT Builder                          │
├─────────────┬─────────────┬─────────────┬───────────────────┤
│  生成        │  编辑        │  查看        │  沉淀             │
├─────────────┼─────────────┼─────────────┼───────────────────┤
│ 大纲生成     │ 自然语言编辑 │ 上下页/步骤  │ 个人仓库          │
│ 整套生成     │ 指令系统     │ 缩放总览     │ manifest 插件     │
│ 风格选择     │ 单页/全局    │ 点击跳转     │ AI 移植进 slide   │
└─────────────┴─────────────┴─────────────┴───────────────────┘
        └──────── 设计系统（token/版式/图表/动效） ────────┘
        └──────── Run 引擎（SSE 流式 + HITL） ─────────────┘
```

## 产品能力表（参考项目提炼 → 本项目落点）

| 能力 | 说明 | 落点 |
|---|---|---|
| token 驱动换肤 | 一套 design token，换一处全局换肤 | [60/design-tokens](../60-design-system/design-tokens.md)、`/overview` |
| 版式库 | cover/toc/bullets/two-column/kpi-grid/table/code/timeline/mindmap/comparison… | [60/layouts](../60-design-system/layouts.md) |
| 图表样式 | bar/line/pie/radar/arch/flow/gantt… | [60/charts](../60-design-system/charts.md) |
| 动效库 | CSS 动画 + Canvas FX | [60/animations](../60-design-system/animations.md) |
| iframe 隔离预览 | 每页独立 iframe，`?preview=N` 单页渲染 | [20/preview-mechanism](../20-architecture/preview-mechanism.md) |
| postMessage 切页不刷新 | 切页只 toggle active，无重载无闪烁 | [20/preview-mechanism](../20-architecture/preview-mechanism.md) |
| 总览网格 + 深链跳页 | 缩略全部页、点击跳转 | [feat-online-viewer](feat-online-viewer.md) |
| 整套生成 | 主题/简报 → 整套精美 HTML | [feat-slide-generation](feat-slide-generation.md) |
| Show-don't-tell 风格发现 | 生成若干风格预览供选择 | [feat-outline-generation](feat-outline-generation.md) |
| 零依赖单 HTML / 16:9 / 中英文 | 产出物工程约束 | [60/html-output-spec](../60-design-system/html-output-spec.md) |

## 全局功能性约束

| ID | 约束 | 优先级 |
|---|---|---|
| `SPEC-GLOBAL-001` | 所有生成与编辑操作通过 Run 外壳 + Harness（ReAct 循环 + 工具化）执行，输出经 SSE 流式返回 | P0 |
| `SPEC-GLOBAL-002` | Run 执行中途可接受控制输入（HITL），在 checkpoint 处消费 | P0 |
| `SPEC-GLOBAL-003` | 所有 slide 产出遵循 [html-output-spec](../60-design-system/html-output-spec.md)（16:9、零运行时依赖、可访问性、中英文） | P0 |
| `SPEC-GLOBAL-004` | 任意编辑须保留可回滚版本（见 [versioning](../30-data-model/versioning.md)） | P0 |
| `SPEC-GLOBAL-005` | 无登录注册；单用户单机 | P0 |
| `SPEC-GLOBAL-006` | 全局样式与单页样式分层；scope 隔离由 harness **工具门控机制**保证（见 [agent-harness](../20-architecture/agent-harness.md)） | P0 |
| `SPEC-GLOBAL-007` | 编辑采用**锚定局部 patch**（edit_file 同构），非整页重写（[ARCH-TOOLS](../20-architecture/tools.md)） | P0 |
| `SPEC-GLOBAL-008` | 个人仓库为 4 类统一资产，preset 与 user 同协议（[asset-protocol](../60-design-system/asset-protocol.md)） | P0 |

## 验收标准（Given-When-Then）

- **AC-GLOBAL-001**（覆盖 `SPEC-GLOBAL-001`）
  - GIVEN 用户发起一次生成请求
  - WHEN 后端创建 Run 并执行
  - THEN 客户端通过 SSE 收到至少一个 `progress` 事件与一个 `done` 事件

- **AC-GLOBAL-002**（覆盖 `SPEC-GLOBAL-002`）
  - GIVEN 一个执行中的 Run
  - WHEN 客户端 `POST /runs/{id}/input` 注入消息
  - THEN 该消息在下一个 checkpoint 被纳入上下文，且影响后续输出

- **AC-GLOBAL-006**（覆盖 `SPEC-GLOBAL-006`）
  - GIVEN 一个多页演示文稿
  - WHEN 执行 `/page 2` 编辑第 2 页
  - THEN 仅第 2 页的 slide html 变更，公共样式层与其它页文件 hash 不变

## 校验方式

- `SPEC-GLOBAL-001/002`：集成测试 `go test ./internal/run -run TestRunSSEAndInput`。
- `SPEC-GLOBAL-003`：lint 脚本校验产出 html（见 [html-output-spec](../60-design-system/html-output-spec.md) 校验清单）。
- `SPEC-GLOBAL-006`：编辑前后对相关文件做 hash diff 断言。

## 依赖

- [OVERVIEW-SCOPE](../00-overview/scope.md)
- [OVERVIEW-FLOWS](../00-overview/personas-and-flows.md)
