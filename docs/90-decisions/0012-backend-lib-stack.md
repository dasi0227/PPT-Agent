---
id: ADR-0012
title: 后端库栈 GORM + modernc + Viper + zap + wire
status: accepted
owner: backend
depends_on: [ADR-0001, ADR-0002]
verifies: []
---

# ADR-0012：后端库栈（GORM + modernc + Viper + zap + wire）

## 背景

[ADR-0001](0001-backend-go-gin.md) 定了 Gin，[ADR-0002](0002-persistence-sqlite-fs.md) 定了 SQLite+文件系统，但未定：SQLite 怎么访问、配置/日志/依赖装配用什么。用户为资深 Java 工程师，目标是**快速掌握 Go 的企业级框架用法与语法**（非从零学后端），故选一套企业普及度高、与 Java 生态可类比的「全家桶」，以最大化迁移效率与简历价值。

## 决策

| 关注点 | 选型 | Java 类比 |
|---|---|---|
| ORM | **GORM** | Hibernate/JPA |
| SQLite 驱动 | **`modernc.org/sqlite`**（纯 Go，无 cgo）经 **`github.com/glebarez/sqlite`** dialector 接入 GORM | 纯 JDBC 驱动 |
| 配置 | **Viper** | `@ConfigurationProperties` / `application.yml` |
| 日志 | **zap**（Uber，结构化） | SLF4J + Logback |
| 依赖注入 | **google/wire**（编译期生成，无反射） | Spring IoC（但编译期、无反射） |

细则：
- **GORM + PO/model 分离**：GORM tag 只出现在 `store/sqlite/` 内部的持久化对象（PO，如 `projectPO`），`model/` 保持纯领域类型、零外部依赖；store 在边界做 PO↔`model` 互转（≈ Java 的 Entity 与 Domain 分离）。
- **驱动**：GORM 官方 `gorm.io/driver/sqlite` **硬依赖 cgo（mattn/go-sqlite3）**，无法直接替换为纯 Go 引擎。故用 **`github.com/glebarez/sqlite`**（GORM dialector，内部封装 `modernc.org/sqlite` 纯 Go 引擎）接入——`modernc.org/sqlite` 仍是真实引擎，全程免 cgo，便于交叉编译与部署。这是「GORM + 纯 Go SQLite」的标准组合。
- **迁移**：以 `migrations/` 的 SQL（对应 [sqlite-schema.sql](../30-data-model/sqlite-schema.sql)）为 schema 权威源；GORM AutoMigrate 至多用于开发期校对，不作事实源（避免「两份真相」）。
- **配置**：Viper 统一读取 env + 可选配置文件，装配为强类型 config 结构体，集中于 `cmd/server` 装配、经 `internal/config` 暴露。
- **日志**：zap 结构化输出；harness 每轮 thought/tool_call/observation 带 `run_id` 等字段，便于按执行追溯。
- **DI**：wire 在编译期生成装配代码（provider set），`cmd/server/main.go` 调生成的 injector；接线错误编译即报错。
- **串行写**：仍遵循 [ARCH-SYS-005](../20-architecture/system-overview.md) 单写通道；SQLite 开 WAL + `busy_timeout`，复用单一 `*sql.DB`。

## 选项与权衡

| 关注点 | 选中 | 备选 | 落选原因 |
|---|---|---|---|
| ORM | GORM | sqlc / sqlx / 纯 database/sql | GORM 企业普及最高、与 JPA 心智直接迁移；用 PO 隔离化解 tag 渗透 |
| 驱动 | modernc（经 glebarez/sqlite dialector） | mattn/go-sqlite3、gorm 官方 driver | 官方 driver/mattn 依赖 cgo，交叉编译/部署麻烦 |
| 配置 | Viper | os.Getenv / 自写 | 配置虽少，但为练企业用法与多来源/热加载能力统一采用 |
| 日志 | zap | 标准库 slog / logrus | zap 企业高频、性能强、生态成熟 |
| DI | wire | 手写装配 / dig(运行时反射) | wire 编译期、无反射，错误早暴露；契合分层多的装配 |

## 后果

- 新增包：`internal/config`（Viper 装配）、`internal/logger`（zap 封装）、`wire.go`/`wire_gen.go`（DI）。
- `store/sqlite/` 各表 PO 带 GORM tag；`model/` 与 service 签名 MUST NOT 出现 ORM/框架类型（新约束 `ARCH-BACKEND-006`）。
- store/llm 仍以 interface 暴露（`ARCH-BACKEND-001`），GORM 仅为其一种实现，可替换。
- 代价：PO↔model 转换样板、wire 生成步骤、多几个依赖；换取分层纯净、企业栈练手与简历广度。
- go.mod 引入：`gorm.io/gorm`、`github.com/glebarez/sqlite`（GORM dialector，封装 modernc）、`modernc.org/sqlite`、`github.com/spf13/viper`、`go.uber.org/zap`、`github.com/google/wire`。

## 状态

accepted（2026-07-01）。
