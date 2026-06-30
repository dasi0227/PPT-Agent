---
id: ARCH-BACKEND
title: 后端结构（Go + chi）
status: approved
owner: backend
depends_on: [ARCH-SYSTEM, ADR-0001, ADR-0002]
verifies: []
---

# 后端结构（Go + net/http + chi）

## 设计原则

- **标准库优先**：以 `net/http` 为基，`chi` 仅做路由与中间件，不引入重框架（打磨原生 Go，见 [ADR-0001](../90-decisions/0001-backend-go-chi.md)）。
- **清晰分层**：handler → service → store，依赖单向向下，禁止反向依赖。
- **接口隔离**：store 与 llm 以 interface 定义，便于替换与测试。
- **串行化共享状态**：写 SQLite / state.json 经单写入通道，避免竞态（[ARCH-SYS-005](system-overview.md)）。

## 目录结构

```
backend/
├── cmd/
│   └── server/
│       └── main.go            # 装配依赖、启动 http server（监听回环）
├── internal/
│   ├── httpapi/               # HTTP 层（handler + 路由 + 中间件）
│   │   ├── router.go          # chi 路由注册
│   │   ├── middleware.go      # 日志、recover、请求 ID、CORS（本地）
│   │   ├── sse.go             # SSE 写出辅助（flush、心跳、事件编码）
│   │   ├── errors.go          # 统一错误响应 + 错误码映射
│   │   ├── project_handler.go
│   │   ├── deck_handler.go
│   │   ├── slide_handler.go
│   │   ├── run_handler.go     # 创建 run / 订阅 events / 注入 input
│   │   └── plugin_handler.go
│   ├── service/               # Service 层（用例编排）
│   │   ├── project.go
│   │   ├── deck.go
│   │   ├── slide.go
│   │   ├── version.go
│   │   └── plugin.go
│   ├── run/                   # Run 引擎（见 agent-runtime.md）
│   │   ├── engine.go          # run 生命周期状态机
│   │   ├── bus.go             # 事件总线（SSE 扇出）
│   │   ├── input.go           # 控制输入队列（HITL）
│   │   └── checkpoint.go
│   ├── agent/                 # Agent 业务逻辑
│   │   ├── outline/           # 大纲生成
│   │   ├── generate/          # slide 生成
│   │   ├── edit/              # 编辑（page/overview）
│   │   ├── command/           # 指令解析与分派（/page /overview ...）
│   │   └── prompt/            # prompt 模板装配
│   ├── llm/                   # LLM 客户端
│   │   ├── client.go          # interface: Chat/Stream
│   │   └── deepseek.go        # DeepSeek 实现
│   ├── store/                 # 持久化
│   │   ├── sqlite/            # SQLite 实现（元数据/版本/插件索引）
│   │   ├── fs/                # 文件系统（slide 产物、work_dir）
│   │   └── store.go           # store interface 定义
│   ├── model/                 # 领域模型（Project/Deck/Slide/Run/Plugin/Version）
│   └── designsystem/          # 设计系统资产加载（主题/版式/动效目录）
├── migrations/                # SQLite 迁移脚本（对应 30-data-model/sqlite-schema.sql）
├── go.mod
└── go.sum
```

## 依赖方向（强约束）

```
httpapi  ──▶ service ──▶ store(interface)
                │            ▲
                ├──▶ run ────┘
                └──▶ agent ──▶ llm(interface) + prompt + designsystem
```

| ID | 规则 |
|---|---|
| `ARCH-BACKEND-001` | `store` 与 `llm` MUST 以 interface 暴露，具体实现可替换 |
| `ARCH-BACKEND-002` | `httpapi` MUST NOT 直接依赖 `store` 具体实现，须经 `service` |
| `ARCH-BACKEND-003` | `model` MUST 无外部依赖（纯领域类型） |
| `ARCH-BACKEND-004` | SQLite 写入 MUST 经单一串行通道（[ARCH-SYS-005](system-overview.md)） |
| `ARCH-BACKEND-005` | SSE 写出 MUST 在每事件后 flush，并周期发送心跳注释行 |

## 路由分组（详见 [40-api](../40-api/rest-endpoints.md)）

```
/api/v1
  /projects            GET POST
  /projects/{id}       GET DELETE
  /projects/{id}/deck  GET
  /decks/{id}/slides   GET
  /slides/{id}         GET
  /slides/{id}/versions GET
  /projects/{id}/runs  POST           # 发起一次 Agent 执行
  /runs/{id}/events    GET (SSE)      # 订阅事件流
  /runs/{id}/input     POST           # HITL 控制输入
  /runs/{id}           DELETE         # 取消 run
  /plugins             GET POST
  /plugins/{id}        GET DELETE
```

## 配置

- 通过环境变量：`DEEPSEEK_API_KEY`、`PPT_WORK_ROOT`（work_dir 根）、`PPT_LISTEN_ADDR`（默认 `127.0.0.1:8787`）。
- 配置加载集中于 `cmd/server/main.go`，不散落到各包。

## 验收标准（Given-When-Then）

- **AC-ARCH-BACKEND-001**（`ARCH-BACKEND-002`）
  - GIVEN 代码库
  - WHEN 运行依赖检查（`go list` / 静态分析）
  - THEN `httpapi` 包未直接 import `store/sqlite` 或 `store/fs` 具体实现

- **AC-ARCH-BACKEND-004**（`ARCH-BACKEND-004`）
  - GIVEN 并发发起多个写操作
  - WHEN 同时写 SQLite
  - THEN 无竞态错误（`go test -race` 通过）

## 校验方式

```bash
go vet ./...
go test -race ./...
# 依赖方向：用 go list -deps 或 import-lint 校验分层
```

## 依赖

- [ARCH-SYSTEM](system-overview.md)、[ADR-0001](../90-decisions/0001-backend-go-chi.md)、[ADR-0002](../90-decisions/0002-persistence-sqlite-fs.md)
