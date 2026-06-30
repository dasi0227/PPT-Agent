---
id: ADR-0001
title: 后端 Go + net/http + chi
status: accepted
owner: backend
depends_on: []
verifies: []
---

# ADR-0001：后端 Go + net/http + chi

## 背景

用户希望借此项目打磨 Go 技术，倾向贴近原生、避免重框架黑盒；同时需要干净的路由与中间件能力。

## 决策

后端以标准库 `net/http`（Go 1.22+ 增强的 `ServeMux`）为基，搭配轻量路由库 **chi** 提供分组路由、URL 参数与中间件链。不引入 Gin/Echo 等全功能框架。

## 选项与权衡

| 选项 | 优点 | 缺点 |
|---|---|---|
| net/http + chi（选中） | 贴近原生、心智负担低、中间件清晰、生产可用 | 需自行组织少量样板 |
| 纯标准库手写 | 学习价值最高 | 样板多、迭代慢 |
| Gin / Echo | 电池齐全、开发快 | 封装多，削弱「打磨原生 Go」目标 |

## 后果

- backend 结构围绕 `net/http` handler 组织（见 [backend-structure](../20-architecture/backend-structure.md)）。
- SSE 用标准 `http.Flusher` 手写，契合 chi 的 `http.Handler` 模型。
- 保留未来替换路由库的低成本（handler 不绑死框架类型）。

## 状态

accepted（2026-06-30）。
