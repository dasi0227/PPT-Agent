---
id: ARCH-LLM
title: LLM 接入（DeepSeek）
status: approved
owner: backend
depends_on: [ARCH-RUNTIME, ARCH-HARNESS, AGENT-PROMPTS, ADR-0008]
verifies: []
---

# LLM 接入（DeepSeek）

## 设计目标

以可替换的 interface 接入 DeepSeek，支持流式输出，统一重试/超时/错误映射，明确 prompt 注入边界与安全约束。

## 客户端接口

```go
// internal/llm/client.go
type Client interface {
    // 一次性补全
    Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
    // 流式补全：通过 channel 推送增量 token
    Stream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error)
    // function calling：带工具集，返回 LLM 选择的工具调用（或文本/finish）
    CallTool(ctx context.Context, req ToolCallRequest) (ToolCallResponse, error)
}
```

`deepseek.go` 实现该接口，对接 DeepSeek 的 OpenAI 兼容 Chat Completions + tools 端点。

> **接口抽象不等于现在做多供应商**：当前仅 DeepSeek 一个实现（[ADR](../90-decisions/0008-llm-interface-abstraction.md)）。以 interface 暴露只是「不焊死」——未来加 OpenAI/Claude 仅需新增实现类，不改 harness。这与「严格专用」不冲突：专用的是 PPT 业务，不是 LLM 厂商。

## function calling 与 harness

- `CallTool` 接收 harness 动态裁剪后的工具集（function schema），返回 LLM 选择的 `tool_call`（工具名 + JSON 参数）或 `finish`。
- harness 据此执行工具、回灌 observation（见 [agent-harness](agent-harness.md)）。

### function call 容错（最小策略，按 ADR-0008）

当前**假设 DeepSeek function calling 表现良好**，不做复杂纠偏，但守住一条底线：

| ID | 约束 |
|---|---|
| `ARCH-LLM-FC-001` | function call 返回**无法解析**（非法 JSON / 缺必需参数）时，MUST 走「最小失败退出」：记录错误 → 发 `error` 事件 → Run 转 `failed`，**不进入重试死循环** |
| `ARCH-LLM-FC-002` | 智能纠偏（JSON 修复 / 结构化重提示）列为**后续增强**，不在 MVP 实现 |

> 多 LLM 切换与高级容错：见 backlog，本期不做。

## 配置

| 项 | 来源 | 默认 |
|---|---|---|
| API Key | env `DEEPSEEK_API_KEY` | 必填 |
| Base URL | env `DEEPSEEK_BASE_URL` | DeepSeek 默认端点 |
| Model | env `DEEPSEEK_MODEL` | `deepseek-chat` |
| 超时 | 配置 | 单次请求 60s（流式按首 token + 空闲超时） |

## 流式与 Run 引擎对接

- `Stream` 产出的 chunk 由 Run 引擎转译为 SSE `token` 事件。
- 流式过程中 Run 仍可在 checkpoint 消费控制输入（输入不打断当前 LLM 调用，见 [agent-runtime](agent-runtime.md)）。

## 重试与错误

| 情况 | 策略 |
|---|---|
| 429 / 限流 | 指数退避重试（上限 3 次） |
| 5xx / 网络抖动 | 退避重试（上限 3 次） |
| 4xx（鉴权/参数） | 不重试，映射为 `LLM_BAD_REQUEST` 错误码 |
| 超时 | 取消上下文，发 `error` 事件，Run 转 `failed` |

错误码映射见 [api-overview](../40-api/api-overview.md) 错误码表。

| ID | 约束 |
|---|---|
| `ARCH-LLM-001` | LLM 客户端 MUST 以 interface 暴露，DeepSeek 为其一实现 |
| `ARCH-LLM-002` | 流式调用 MUST 可被 ctx 取消（支持 Run 取消） |
| `ARCH-LLM-003` | API Key MUST 仅来自环境变量，禁止落库/落日志 |
| `ARCH-LLM-004` | prompt 装配 MUST 经 [prompt-templates](../50-agent/prompt-templates.md)，禁止在 llm 包内硬编码业务 prompt |
| `ARCH-LLM-005` | 用户输入注入 prompt 前 MUST 经边界处理（角色隔离、长度上限），防止提示注入越权 |

## prompt 注入边界

- 系统约束（设计系统规则、输出格式、安全规则）放 system 消息，用户内容放 user 消息，二者不混淆。
- 用户输入不得改变系统级约束（如「忽略前面的规则」类注入须被系统 prompt 抵御）。

## 验收标准（Given-When-Then）

- **AC-LLM-002**（`ARCH-LLM-002`）
  - GIVEN 一个流式调用
  - WHEN 取消其 ctx（Run 被取消）
  - THEN 流式 goroutine 及时退出，无泄漏（`go test -race` + goroutine 计数）

- **AC-LLM-003**（`ARCH-LLM-003`）
  - GIVEN 运行日志与数据库
  - WHEN 检索 API Key
  - THEN 任何日志/表中不出现明文 Key

## 校验方式

```bash
go test ./internal/llm -run 'TestStreamCancel|TestRetryPolicy'
# 用 mock server 模拟 429/5xx/超时
```

## 依赖

- [ARCH-RUNTIME](agent-runtime.md)、[AGENT-PROMPTS](../50-agent/prompt-templates.md)
