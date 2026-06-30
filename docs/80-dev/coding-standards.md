---
id: DEV-CODING
title: 编码规范
status: approved
owner: shared
depends_on: [DEV-RULES, ARCH-BACKEND, ARCH-FRONTEND]
verifies: []
---

# 编码规范

## 通用

- 命名表意优先；标识符英文。文档中文。
- 函数/文件保持单一职责；文件过大是「职责过多」的信号，应拆分。
- 错误处理在边界，内部信任。

## Go（后端）

| 项 | 规范 |
|---|---|
| 格式化 | `gofmt` / `goimports` 必过 |
| 静态检查 | `go vet` + golangci-lint（无新增告警） |
| 包结构 | 遵循 [backend-structure](../20-architecture/backend-structure.md) 分层与依赖方向 |
| 错误 | 用 `errors.Is/As` + 包装；对外映射为错误码表 |
| 接口 | store/llm 以 interface 暴露；接口定义在消费方包，避免反向依赖 |
| 并发 | 共享状态串行化；`go test -race` 必过 |
| 上下文 | 所有 IO/LLM 调用接受 `context.Context`，支持取消 |
| 命名 | 导出符号有文档注释（但克制，解释「为什么/约束」） |
| 测试 | 表驱动测试；关键路径含 `-race` |

## TypeScript / React（前端）

| 项 | 规范 |
|---|---|
| 类型 | `strict` 开启；`tsc --noEmit` 必过；API 类型对齐 openapi |
| 格式化/lint | Prettier + ESLint，无新增告警 |
| 组件 | 函数组件 + hooks；单一职责；shadcn/ui 衍生组件集中 `components/` |
| 状态 | Zustand store 单一来源；避免冗余派生状态 |
| 副作用 | 数据请求经 `api/client`；SSE 经 `api/sse` 封装 |
| 预览 | 不直接操作 iframe 内 DOM，经 postMessage |

## 产出 slide（生成物，非应用代码）

- 遵循 [html-output-spec](../60-design-system/html-output-spec.md)：零依赖、token、16:9、可访问性、中英文。
- 与应用源码标准不同：slide 是「数据/产物」，强调可移植与渲染稳定。

## 注释与文档

- 默认不写注释；仅在「为什么」非显而易见处写一行。
- 不写解释「做什么」的注释（命名已说明）。
- 不在代码注释里引用当前任务/调用方/issue 号。

## 校验方式

```bash
# 后端
gofmt -l . ; go vet ./... ; go test -race ./...
# 前端
pnpm lint && pnpm tsc --noEmit && pnpm test
```

## 依赖

- [DEV-RULES](dev-rules.md)、[ARCH-BACKEND](../20-architecture/backend-structure.md)、[ARCH-FRONTEND](../20-architecture/frontend-structure.md)
