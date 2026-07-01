---
id: ARCH-SYSTEM
title: 系统架构总览
status: approved
owner: shared
depends_on: [SPEC-FUNCTIONAL]
verifies: []
---

# 系统架构总览

## 组件总图

```
┌──────────────────────────── 浏览器（前端 SPA） ────────────────────────────┐
│  React + Vite + TS + Tailwind + shadcn/ui + Zustand                        │
│  ┌────────────┐  ┌─────────────┐  ┌──────────────────────────────────┐    │
│  │ 对话/指令面板 │  │ 编辑器状态   │  │ 预览层（沙箱 iframe + postMessage） │   │
│  └─────┬──────┘  └──────┬──────┘  └─────────────┬────────────────────┘    │
└────────┼────────────────┼───────────────────────┼─────────────────────────┘
         │ REST (JSON)     │ SSE (事件流)           │ 静态 slide 资源
         ▼                 ▼                        ▼
┌──────────────────────────── 后端（Go + net/http + chi） ───────────────────┐
│  ┌──────────┐   ┌───────────────┐   ┌────────────┐   ┌──────────────────┐  │
│  │ HTTP 层   │──▶│ Service 层     │──▶│ Run+Harness │──▶│ LLM 客户端(DeepSeek)│ │
│  │(handler) │   │(project/slide/│   │(SSE+HITL +  │   │  CallTool/Stream  │ │
│  │          │   │ asset/version)│   │ ReAct+工具)  │   └──────────────────┘  │
│  └──────────┘   └───────┬───────┘   └─────┬──────┘                         │
│                         │                  │                                │
│                         ▼                  ▼                                │
│                 ┌──────────────┐   ┌────────────────┐                       │
│                 │ Store 层      │   │ 文件系统(work_dir)│                      │
│                 │ SQLite(元数据) │   │ slide html/css/js│                     │
│                 └──────────────┘   └────────────────┘                       │
└─────────────────────────────────────────────────────────────────────────────┘
```

## 分层职责

| 层 | 职责 | 文档 |
|---|---|---|
| 前端 SPA | 对话交互、指令输入、编辑器状态、沙箱预览 | [frontend-structure](frontend-structure.md) |
| HTTP 层 | 路由、请求校验、SSE 写出、错误码 | [backend-structure](backend-structure.md) |
| Service 层 | 业务用例编排（项目/Slide/资产/版本） | [backend-structure](backend-structure.md) |
| Run 引擎 | Agent 执行单元、SSE 流、HITL 控制输入、checkpoint | [agent-runtime](agent-runtime.md) |
| LLM 客户端 | DeepSeek 接入、流式、重试/超时 | [llm-integration](llm-integration.md) |
| Store 层 | SQLite 持久化（元数据/版本/资产索引） | [30-data-model](../30-data-model/data-model.md) |
| 文件系统 | slide html/css/js 等大文本产物 | [filesystem-layout](../30-data-model/filesystem-layout.md) |

## 关键架构约束

| ID | 约束 |
|---|---|
| `ARCH-SYS-001` | 前后端通过 REST（控制/查询）+ SSE（流式输出）通信；不引入 WebSocket |
| `ARCH-SYS-002` | 所有 Agent 行为收敛到 Run 引擎，统一生命周期与事件协议 |
| `ARCH-SYS-003` | 元数据入 SQLite，slide 大文本入文件系统（混合持久化） |
| `ARCH-SYS-004` | 公共样式层与单页 html 物理分离，支撑 `/page` 与 `/overview` 的隔离语义 |
| `ARCH-SYS-005` | 访问共享状态（state.json/SQLite 写）MUST 串行化，避免竞态 |
| `ARCH-SYS-006` | 单用户单机、无鉴权；后端默认仅监听本地回环 |

## 数据流（生成一次的典型路径）

1. 前端 `POST /threads/{id}/runs`（含指令，挂在某对话线程下）。
2. HTTP 层校验 → Service 取 project 锁 → 创建 Run → 返回 `run_id` + SSE 订阅地址。
3. 前端 `GET /runs/{id}/events`（SSE）订阅。
4. Run 外壳驱动 Harness（ReAct 循环）：调 LLM、经工具产出，写文件 + 落版本，发 `thought`/`tool_call`/`progress` 事件。
5. 用户可 `POST /runs/{id}/input` 注入控制输入；Run 在 checkpoint 消费。
6. 完成发 `done`；前端刷新预览。

## 依赖

- [SPEC-FUNCTIONAL](../10-spec/functional-spec.md)
