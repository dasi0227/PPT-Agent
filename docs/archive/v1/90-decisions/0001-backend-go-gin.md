---
id: ADR-0001
title: 后端 Go + Gin
status: accepted
owner: backend
depends_on: []
verifies: []
---

# ADR-0001：后端 Go + Gin

## 背景

用户希望借此项目学习 Go 的**工程化编码方式**（面向工作、贴近企业实践），而非从底层一点点手写标准库。国内企业后端大量直接采用成熟 Web 框架，Gin 是事实上的主流选型之一。因此后端应选一个电池齐全、生态成熟、简历/面试常见的框架，以最大化「面向工作学习」的价值。

> 本 ADR 曾选 `net/http + chi`（理由为「打磨原生 Go、避免框架黑盒」）。该前提已被新的学习目标取代——目标从「贴近原生」转为「掌握企业级框架编码方式」，故改用 Gin。

## 决策

后端采用 **Gin** 作为 Web 框架：路由分组、URL 参数、中间件链、请求绑定与校验（binding/validator）、统一错误处理均走 Gin 生态。不再以标准库手写路由样板。

- SSE 流式输出用 Gin 的 `c.Stream()` / 直接持有 `http.Flusher` 手动 flush；MUST 关闭会缓冲响应的中间件（如 gzip）对 SSE 端点的作用，保证事件即时下发（契合 [ADR-0004](0004-transport-sse-hitl.md)）。
- handler 依赖 `*gin.Context`，但业务逻辑经 service 层解耦，不把框架类型渗透到 service/store（见 [backend-structure](../20-architecture/backend-structure.md) 依赖方向约束）。

## 选项与权衡

| 选项 | 优点 | 缺点 |
|---|---|---|
| Gin（选中） | 国内主流、生态齐全、binding/validator/中间件开箱即用、面向工作学习价值高 | 封装较多，需理解其 Context 模型；SSE 需注意缓冲 |
| net/http + chi | 贴近原生、心智负担低 | 与「学习企业级框架编码方式」目标不符，样板偏多 |
| Echo | 与 Gin 类似、性能好 | 国内普及度略低于 Gin |
| 纯标准库手写 | 学习价值最高 | 偏底层，非本项目学习目标；迭代慢 |

## 后果

- backend 结构围绕 Gin handler 组织（见 [backend-structure](../20-architecture/backend-structure.md)）。
- 请求校验优先用 Gin 的 struct binding + validator tag，减少手写解析。
- SSE 端点需专门处理 flush 与中间件缓冲，作为一处「企业级流式接口」实战点。
- service/store 层不依赖 Gin 类型，保留未来替换框架的低成本。

## 状态

accepted（2026-07-01，取代原 chi 选型）。
