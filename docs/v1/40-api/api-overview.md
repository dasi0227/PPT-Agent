---
id: API-OVERVIEW
title: API 总览与约定
status: approved
owner: backend
depends_on: [ARCH-BACKEND, ARCH-RUNTIME]
verifies: []
---

# API 总览与约定

## 基本约定

| 项 | 约定 |
|---|---|
| 基址 | `/api/v1` |
| 传输 | REST（控制/查询，JSON）+ SSE（流式输出） |
| 鉴权 | 无（单用户单机，默认监听 `127.0.0.1`） |
| 编码 | UTF-8；请求/响应 `application/json`；SSE `text/event-stream` |
| 时间 | unix 秒（INTEGER） |
| ID | UUID 字符串 |
| 错误体 | 统一 `{ "error": { "code": "...", "message": "...", "details": {} } }` |

## 资源与端点（详见 [rest-endpoints](rest-endpoints.md)）

| 资源 | 端点 |
|---|---|
| 项目（PPT） | `GET/POST /projects`，`GET/DELETE /projects/{id}` |
| 线程（对话） | `GET/POST /projects/{id}/threads`，`GET/DELETE /threads/{id}`，`GET /threads/{id}/history` |
| Slide | `GET /projects/{id}/slides`，`GET /slides/{id}`，`GET /slides/{id}/versions`，`POST /slides/{id}/rollback` |
| Run | `POST /threads/{id}/runs`，`GET /runs/{id}/events`(SSE)，`POST /runs/{id}/input`，`DELETE /runs/{id}` |
| 资产（个人仓库） | `GET/POST /assets`，`GET/PATCH/DELETE /assets/{id}`，`POST /assets/{id}/rollback` |
| 导出（backlog） | `POST /projects/{id}/export`（`x-status: backlog`） |

## 错误码表

| code | HTTP | 含义 |
|---|---|---|
| `BAD_REQUEST` | 400 | 请求体/参数非法 |
| `NOT_FOUND` | 404 | 资源不存在 |
| `CONFLICT` | 409 | 状态冲突（如对已结束 run 注入输入） |
| `RUN_NOT_RUNNING` | 409 | run 非可注入状态 |
| `VALIDATION_FAILED` | 422 | schema 校验失败（slide-json/manifest） |
| `LLM_BAD_REQUEST` | 502 | LLM 上游 4xx |
| `LLM_UNAVAILABLE` | 503 | LLM 限流/超时/5xx 重试耗尽 |
| `INTERNAL` | 500 | 未分类内部错误 |

| ID | 约束 |
|---|---|
| `API-OVERVIEW-001` | 所有端点 MUST 在 [openapi.yaml](openapi.yaml) 中有定义，文档与契约一致 |
| `API-OVERVIEW-002` | 错误响应 MUST 使用统一错误体与上表 code |
| `API-OVERVIEW-003` | 流式输出 MUST 经 SSE（[sse-events](sse-events.md)），非 WebSocket |
| `API-OVERVIEW-004` | 写操作 MUST 幂等性可控（创建返回资源 id，重复创建不静默合并） |

## 验收标准（Given-When-Then）

- **AC-API-001**（`API-OVERVIEW-001`）
  - GIVEN [openapi.yaml](openapi.yaml)
  - WHEN 用 OpenAPI 校验器加载
  - THEN 为合法 OpenAPI 3.1，且包含上表全部端点

- **AC-API-002**（`API-OVERVIEW-002`）
  - GIVEN 请求不存在的项目
  - WHEN `GET /projects/{badid}`
  - THEN 返回 404 + `{ "error": { "code": "NOT_FOUND", ... } }`

## 校验方式

```bash
# OpenAPI 合法性
npx @redocly/cli lint docs/40-api/openapi.yaml
# 契约测试：用 schemathesis / dredd 对实现做契约校验
```

> **lint 例外**：`recommended` 规则集的 `security-defined`（要求每个 operation 声明 `security`）与本 API 的无鉴权设计（[ARCH-SYS-006](../20-architecture/system-overview.md)：单机回环、无鉴权）相抵触。故在 `redocly.yaml` 中**仅关闭该条规则**，不在 `openapi.yaml` 添加虚假的 `security` 声明（契约须诚实）。lint 例外应带注释指向 `ARCH-SYS-006` 以自解释。

## 依赖

- [ARCH-BACKEND](../20-architecture/backend-structure.md)、[ARCH-RUNTIME](../20-architecture/agent-runtime.md)
