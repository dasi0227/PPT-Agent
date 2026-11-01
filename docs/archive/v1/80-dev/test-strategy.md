---
id: DEV-TEST
title: 测试与验收策略
status: approved
owner: shared
depends_on: [SPEC-ACCEPTANCE, DEV-PLAN]
verifies: []
---

# 测试与验收策略

「完成」必须有客观证据。本策略定义测试分层与各类 AC 的校验手段。

## 测试金字塔

| 层 | 范围 | 工具 |
|---|---|---|
| 单元测试 | 纯逻辑：解析、状态机、版本、装配 | `go test` / vitest |
| 契约测试 | API 与 openapi、schema 一致性 | schemathesis/dredd、JSON Schema 校验 |
| 集成测试 | Run + LLM(mock) + SSE + 文件/SQLite | `go test`（mock LLM server） |
| 端到端 | 浏览器：预览/翻页/总览/编辑 | Playwright |
| 静态校验 | DDL/OpenAPI/Schema/产出 html | sqlite3、redocly、lint-slide、lint-tokens |

## 各类验收的校验手段映射

| AC 类别 | 手段 |
|---|---|
| scope 隔离（page/overview） | 编辑前后 **hash diff**，断言受影响文件集合 |
| 流式/HITL | 集成测试 + mock LLM，断言事件序列与输入消费时机 |
| SSE 续传 | 断线重连 + `Last-Event-ID`，断言 seq 续发无重复 |
| slide 产出合规 | `lint-slide.mjs` 校验清单 |
| token 完整性 | `lint-tokens.mjs` 必需 token |
| 枚举一致性 | layout/chart enum 与 schema 比对脚本 |
| 数据/契约 | `sqlite3` 执行 DDL、`redocly lint` openapi、jsonschema 校验 |
| 预览无刷新 | e2e 监听 iframe `load` 次数 |

## 关键校验脚本（约定）

| 脚本 | 作用 |
|---|---|
| `scripts/lint-slide.mjs` | 校验 slide html 是否满足 html-output-spec |
| `scripts/lint-tokens.mjs` | 校验公共层/主题必需 token |
| `scripts/check-layout-enum.mjs` | layout 文档 ↔ schema enum 一致 |
| `scripts/check-chart-enum.mjs` | chart 文档 ↔ schema enum 一致 |
| `scripts/check-workdir.mjs` | work_dir 结构（state.json 位置、公共层分离） |
| `scripts/smoke-api.sh` | curl 冒烟，验证状态码 |

## LLM 测试策略

- 默认用 **mock LLM server** 跑确定性测试（固定响应/流式分片）。
- 真实 DeepSeek 仅在「质量评估」中按需手动跑，不进必经 CI（避免不稳定与成本）。

## CI 必经检查

```
gofmt -l . == 空
go vet ./... && go test -race ./...
pnpm lint && pnpm tsc --noEmit && pnpm test
sqlite3 :memory: < docs/30-data-model/sqlite-schema.sql
npx @redocly/cli lint docs/40-api/openapi.yaml
node scripts/check-layout-enum.mjs && node scripts/check-chart-enum.mjs
```

## 验收标准（Given-When-Then）

- **AC-TEST-CI**
  - GIVEN 一次提交
  - WHEN CI 运行上述必经检查
  - THEN 全绿才允许合并

## 校验方式

- CI 配置执行本文件「CI 必经检查」全部命令。

## 依赖

- [SPEC-ACCEPTANCE](../10-spec/acceptance-criteria.md)、[DEV-PLAN](dev-plan.md)
