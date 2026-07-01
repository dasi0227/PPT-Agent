---
id: API-REST
title: REST 端点详述
status: approved
owner: backend
depends_on: [API-OVERVIEW]
verifies: []
---

# REST 端点详述

权威契约见 [openapi.yaml](openapi.yaml)。本文件给出每个端点的语义说明与 `curl` 示例（可执行校验）。

## 项目

### 创建项目（输入主题）

```bash
curl -s -X POST http://127.0.0.1:8787/api/v1/projects \
  -H 'Content-Type: application/json' \
  -d '{"topic":"云原生可观测性实践","language":"zh"}'
```
- `API-PROJECT-001`：创建成功返回 201 + Project，并在 `PPT_WORK_ROOT` 下初始化 work_dir 与 `state.json`。

### 获取 / 删除

```bash
curl -s http://127.0.0.1:8787/api/v1/projects/{id}
curl -s -X DELETE http://127.0.0.1:8787/api/v1/projects/{id}   # 204；级联删除 deck/slides
```

## Deck / Slide

```bash
curl -s http://127.0.0.1:8787/api/v1/projects/{id}/deck
curl -s http://127.0.0.1:8787/api/v1/decks/{deckId}/slides
curl -s http://127.0.0.1:8787/api/v1/slides/{slideId}
curl -s http://127.0.0.1:8787/api/v1/slides/{slideId}/versions
```

### 回滚

```bash
curl -s -X POST http://127.0.0.1:8787/api/v1/slides/{slideId}/rollback \
  -H 'Content-Type: application/json' -d '{"version_no":1}'
```
- `API-SLIDE-001`：回滚产生新版本（见 [versioning](../30-data-model/versioning.md)）。

## 线程（对话，隔离历史，共享 project 产物）

```bash
# 在项目下新建对话线程
curl -s -X POST http://127.0.0.1:8787/api/v1/projects/{id}/threads \
  -H 'Content-Type: application/json' -d '{"title":"内容打磨"}'

# 列出项目的线程
curl -s http://127.0.0.1:8787/api/v1/projects/{id}/threads

# 读取线程历史（恢复：关闭再打开接着聊）
curl -s http://127.0.0.1:8787/api/v1/threads/{threadId}/history

# 归档/删除线程（不影响 PPT 产物）
curl -s -X DELETE http://127.0.0.1:8787/api/v1/threads/{threadId}
```

## Run（生成 / 编辑 / 指令，挂在线程下）

### 发起 Run

```bash
# 生成大纲（在某线程下）
curl -s -X POST http://127.0.0.1:8787/api/v1/threads/{threadId}/runs \
  -H 'Content-Type: application/json' \
  -d '{"kind":"outline","instruction":"做一份8页的技术分享"}'

# 当前页编辑（默认 /current，前端填当前预览页号）
curl -s -X POST http://127.0.0.1:8787/api/v1/threads/{threadId}/runs \
  -H 'Content-Type: application/json' \
  -d '{"kind":"edit","scope":"current","page_index":3,"instruction":"把标题改大一号"}'

# 指定单页编辑（/page 2）
curl -s -X POST http://127.0.0.1:8787/api/v1/threads/{threadId}/runs \
  -H 'Content-Type: application/json' \
  -d '{"kind":"edit","scope":"page","page_index":2,"instruction":"把标题改大一号"}'

# 跨页/全局（/overview）
curl -s -X POST http://127.0.0.1:8787/api/v1/threads/{threadId}/runs \
  -H 'Content-Type: application/json' \
  -d '{"kind":"edit","scope":"overview","instruction":"主色改成品牌蓝"}'

# 改仓库资产（/repo）
curl -s -X POST http://127.0.0.1:8787/api/v1/threads/{threadId}/runs \
  -H 'Content-Type: application/json' \
  -d '{"kind":"edit","scope":"repo","instruction":"把霓虹卡片组件圆角调大"}'

# 只说不做（/talk）
curl -s -X POST http://127.0.0.1:8787/api/v1/threads/{threadId}/runs \
  -H 'Content-Type: application/json' \
  -d '{"kind":"command","command":"talk","mode":"talk","instruction":"分析下整体节奏"}'
```
- 返回 201 + Run（含 `events_url`）。

### 订阅事件（SSE）

```bash
curl -N http://127.0.0.1:8787/api/v1/runs/{runId}/events
# 断线重连续传
curl -N -H 'Last-Event-ID: 42' http://127.0.0.1:8787/api/v1/runs/{runId}/events
```
事件协议见 [sse-events](sse-events.md)。

### 注入控制输入（HITL）

```bash
curl -s -X POST http://127.0.0.1:8787/api/v1/runs/{runId}/input \
  -H 'Content-Type: application/json' \
  -d '{"content":"第3页改成深色背景"}'
# 202 Accepted；若 run 已结束则 409 RUN_NOT_RUNNING

# 回应 Agent 的 needs_input
curl -s -X POST http://127.0.0.1:8787/api/v1/runs/{runId}/input \
  -H 'Content-Type: application/json' \
  -d '{"content":"用方案A","reply_to":"evt_88"}'
```

### 取消

```bash
curl -s -X DELETE http://127.0.0.1:8787/api/v1/runs/{runId}   # 204
```

## 资产（个人仓库）

```bash
# 新增资产（服务端校验 asset-manifest schema）
curl -s -X POST http://127.0.0.1:8787/api/v1/assets \
  -H 'Content-Type: application/json' \
  -d '{"manifest":{"name":"particle-burst","kind":"fx","version":"1.0.0","description":"粒子特效","mount":{"position":"append"},"assets":{"js":"effect.js"}},"payload":{"effect.js":"..."}}'
# 422 VALIDATION_FAILED 当 manifest 不合规

# 列出（可按 kind 过滤）
curl -s 'http://127.0.0.1:8787/api/v1/assets?kind=theme'

# 修改资产（锚定替换载荷，产生资产版本）
curl -s -X PATCH http://127.0.0.1:8787/api/v1/assets/{id} \
  -H 'Content-Type: application/json' \
  -d '{"edits":[{"file":"style.css","old_text":"border-radius: 8px","new_text":"border-radius: 20px"}]}'

# 删除
curl -s -X DELETE http://127.0.0.1:8787/api/v1/assets/{id}
```

## 导出（backlog）

```bash
curl -s -X POST http://127.0.0.1:8787/api/v1/projects/{id}/export   # 501 Not Implemented
```

## 验收标准（Given-When-Then）

- **AC-REST-RUN-001**
  - GIVEN 一个项目下的一条线程
  - WHEN `POST /threads/{id}/runs` kind=outline
  - THEN 201 返回 Run，`status∈{pending,running}`，含 `events_url`

- **AC-REST-INPUT-409**
  - GIVEN 一个 `done` 的 Run
  - WHEN `POST /runs/{id}/input`
  - THEN 409 `RUN_NOT_RUNNING`

## 校验方式

```bash
# 契约测试（实现就绪后）：dredd/schemathesis 跑 openapi.yaml
# 冒烟：上述 curl 期望状态码可写入 scripts/smoke-api.sh
```

## 依赖

- [API-OVERVIEW](api-overview.md)
