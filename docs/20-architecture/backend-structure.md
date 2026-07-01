---
id: ARCH-BACKEND
title: 后端结构（Go + Gin）
status: approved
owner: backend
depends_on: [ARCH-SYSTEM, ADR-0001, ADR-0002]
verifies: []
---

# 后端结构（Go + Gin）

## 设计原则

- **拥抱主流框架**：HTTP 用 **Gin**、ORM 用 **GORM**（驱动 `modernc.org/sqlite` 纯 Go）、配置 **Viper**、日志 **zap**、依赖注入 **google/wire**，面向企业实践编码（见 [ADR-0001](../90-decisions/0001-backend-go-gin.md)、[ADR-0012](../90-decisions/0012-backend-lib-stack.md)）。
- **清晰分层**：handler → service → store，依赖单向向下，禁止反向依赖。
- **框架不渗透**：`gin.Context` 仅在 handler 层；GORM tag 仅在 `store/sqlite` 的 PO；`model/` 与 service 签名只用纯领域类型（PO↔model 在 store 边界互转）。
- **接口隔离**：store 与 llm 以 interface 定义，GORM 仅为 store 的一种实现，可替换、好测试。
- **串行化共享状态**：写 SQLite / state.json 经单写入通道，避免竞态（[ARCH-SYS-005](system-overview.md)）。

## 目录结构

```
backend/
├── cmd/
│   └── server/
│       ├── main.go             # 入口：调 wire 生成的 injector 装配并启动
│       ├── wire.go             # wire provider set 声明（+build wireinject）
│       └── wire_gen.go         # wire 生成的装配代码（勿手改）
├── internal/
│   ├── config/                 # 配置（Viper：env + 可选配置文件 → 强类型 config）
│   │   └── config.go
│   ├── logger/                 # 日志（zap 封装：结构化、带 run_id 等字段）
│   │   └── logger.go
│   ├── httpapi/                # HTTP 层（gin handler + 路由 + 中间件）
│   │   ├── router.go          # gin 引擎与路由分组注册（RouterGroup）
│   │   ├── middleware.go      # gin 中间件：日志、recover、请求 ID、CORS（本地）
│   │   ├── sse.go             # SSE 写出辅助（c.Stream/Flusher、心跳、事件编码）
│   │   ├── errors.go          # 统一错误响应 + 错误码映射
│   │   ├── project_handler.go  # 项目 CRUD（含原 deck 的主题/状态/design 字段）
│   │   ├── thread_handler.go  # 对话线程 CRUD + 历史
│   │   ├── slide_handler.go   # 项目 slides 列表 + 单页 + 版本 + 回滚
│   │   ├── run_handler.go     # 在 thread 下创建 run / 订阅 events / 注入 input
│   │   └── asset_handler.go   # 个人仓库资产 CRUD
│   ├── service/               # Service 层（用例编排）
│   │   ├── project.go         # 项目用例（合并原 deck：主题/状态/公共样式层）
│   │   ├── thread.go          # 线程创建/列表/历史读写
│   │   ├── slide.go
│   │   ├── version.go
│   │   └── asset.go
│   ├── run/                   # Run 外壳（见 agent-runtime.md）
│   │   ├── engine.go          # run 生命周期状态机
│   │   ├── lock.go            # 每 project 一把执行锁（同项目串行，跨项目并行）
│   │   ├── bus.go             # 事件总线（SSE 扇出）
│   │   ├── input.go           # 控制输入队列（HITL）
│   │   └── checkpoint.go
│   ├── harness/               # ★ Agent Harness（见 agent-harness.md / tools.md）
│   │   ├── loop.go            # ReAct 主循环（thought→tool_call→observation）
│   │   ├── gate.go            # 动态工具门控（按 scope/mode 裁剪工具集）
│   │   ├── subagent.go        # 子代理委派（逐页生成）
│   │   ├── stop.go            # 停止条件（max_turns/finish/熔断/取消）
│   │   ├── context.go         # 上下文预算/压缩/渐进披露
│   │   └── tools/             # 工具实现（确定性脚本，带 schema）
│   │       ├── registry.go    # 工具注册与 function schema
│   │       ├── slide_tools.go # read/patch/write_slide、validate_slide、mount_asset
│   │       ├── style_tools.go # read/patch_design、apply_theme
│   │       ├── asset_tools.go # search/read/create/patch/delete_asset
│   │       └── finish.go
│   ├── agent/                 # Agent 业务编排（构造 harness 配置）
│   │   ├── outline/           # 大纲生成（两阶段第一步）
│   │   ├── generate/          # slide 生成（子代理逐页）
│   │   ├── edit/              # 编辑（current/page/overview 局部 patch）
│   │   ├── command/           # 指令解析（scope 四件套 + mode）
│   │   └── prompt/            # prompt 模板装配（system/tools/context/user）
│   ├── llm/                   # LLM 客户端
│   │   ├── client.go          # interface: Chat/Stream/CallTool
│   │   └── deepseek.go        # DeepSeek 实现
│   ├── store/                 # 持久化
│   │   ├── sqlite/            # GORM 实现（元数据/版本/资产索引/run_events）
│   │   │   ├── db.go          # GORM+modernc 初始化（WAL/busy_timeout/单一 *sql.DB）
│   │   │   ├── po.go          # 持久化对象（带 GORM tag，仅本包内）+ PO↔model 互转
│   │   │   ├── project.go     # projects 表读写
│   │   │   ├── thread.go      # threads 表读写
│   │   │   ├── slide.go       # slides 表读写
│   │   │   ├── version.go     # versions 表读写
│   │   │   ├── run.go         # runs + run_events 表读写
│   │   │   └── asset.go       # assets 表读写
│   │   ├── fs/                # 文件系统（slide 产物、work_dir、_assets）
│   │   └── store.go           # store interface 定义
│   ├── asset/                 # 资产协议：校验、seed 载入、移植
│   ├── model/                 # 领域模型（纯类型，无 ORM/框架依赖）
│   └── designsystem/          # 公共层/产出规范辅助（tokens 校验、lint）
├── seed/assets/               # 出厂预置资产（themes/layouts/components/fx）
├── migrations/                # SQLite 迁移脚本（对应 30-data-model/sqlite-schema.sql）
├── go.mod
└── go.sum
```

## 依赖方向（强约束）

```
httpapi ──▶ service ──▶ store(interface)
               │           ▲
               ├──▶ run ───┘         （Run 外壳：状态机/SSE/输入队列）
               │     └──▶ harness ──▶ harness/tools ──▶ store + asset
               │                └──▶ llm(interface)  （CallTool / Stream）
               └──▶ agent ──▶ prompt + designsystem  （构造 harness 配置）
```

> Run 外壳驱动 Harness；Harness 经工具改产物；工具调 store/asset 落盘。LLM 只被 harness 通过 `CallTool` 使用。

| ID | 规则 |
|---|---|
| `ARCH-BACKEND-001` | `store` 与 `llm` MUST 以 interface 暴露，具体实现可替换 |
| `ARCH-BACKEND-002` | `httpapi` MUST NOT 直接依赖 `store` 具体实现，须经 `service` |
| `ARCH-BACKEND-003` | `model` MUST 无外部依赖（纯领域类型） |
| `ARCH-BACKEND-004` | SQLite 写入 MUST 经单一串行通道（[ARCH-SYS-005](system-overview.md)） |
| `ARCH-BACKEND-005` | SSE 写出 MUST 在每事件后 flush，并周期发送心跳注释行 |
| `ARCH-BACKEND-006` | ORM/框架类型（GORM tag、`gin.Context` 等）MUST NOT 出现在 `model/` 与 service 层签名；GORM tag 仅限 `store/sqlite` 的 PO（[ADR-0012](../90-decisions/0012-backend-lib-stack.md)） |

## 路由分组（详见 [40-api](../40-api/rest-endpoints.md)）

```
/api/v1
  /projects              GET POST
  /projects/{id}         GET DELETE
  /projects/{id}/threads GET POST       # 对话线程
  /threads/{id}          GET DELETE
  /threads/{id}/history  GET            # 线程历史（恢复）
  /projects/{id}/slides  GET            # 项目的 slides（原 /decks/{id}/slides）
  /slides/{id}           GET
  /slides/{id}/versions  GET
  /threads/{id}/runs     POST           # 在线程下发起一次 Agent 执行
  /runs/{id}/events      GET (SSE)      # 订阅事件流
  /runs/{id}/input       POST           # HITL 控制输入
  /runs/{id}             DELETE         # 取消 run
  /assets                GET POST
  /assets/{id}           GET PATCH DELETE
```

## 配置

- 由 **Viper** 统一加载（env + 可选配置文件），装配为强类型 config 结构体，集中于 `internal/config`，不散落到各包。
- 关键项：`DEEPSEEK_API_KEY`、`PPT_WORK_ROOT`（work_dir 根）、`PPT_LISTEN_ADDR`（默认 `127.0.0.1:8787`）。

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

- [ARCH-SYSTEM](system-overview.md)、[ADR-0001](../90-decisions/0001-backend-go-gin.md)、[ADR-0002](../90-decisions/0002-persistence-sqlite-fs.md)
