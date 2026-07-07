# M7 前端设计文档

本目录承载 M7 前端产品设计、交互设计、视觉规范、组件设计和交付给前端开发 Agent 的执行 Prompt。

## 文档索引

| 文档 | 作用 |
|---|---|
| [m7-design-spec.md](m7-design-spec.md) | M7 前端完整设计规格，覆盖需求整合、模块设计、状态/API/样式/响应式/性能/验收 |
| [frontend-agent-prompt.md](frontend-agent-prompt.md) | 交付给前端 Agent 的开发 Prompt |
| [prototypes/m7-mvp-core-page-v3.html](prototypes/m7-mvp-core-page-v3.html) | 当前 M7 MVP 核心页参考设计稿 |

## 事实源关系

本目录只定义 M7 前端产品与 UI 设计，不重复替代以下事实源：

- 前端架构总览：[../20-architecture/frontend-structure.md](../20-architecture/frontend-structure.md)
- iframe 预览机制：[../20-architecture/preview-mechanism.md](../20-architecture/preview-mechanism.md)
- SSE 事件协议：[../40-api/sse-events.md](../40-api/sse-events.md)
- REST API 契约：[../40-api/openapi.yaml](../40-api/openapi.yaml)
- 指令与 mode/scope：[../50-agent/commands/README.md](../50-agent/commands/README.md)

若实现需要调整协议，先更新对应事实源，再同步本目录。
