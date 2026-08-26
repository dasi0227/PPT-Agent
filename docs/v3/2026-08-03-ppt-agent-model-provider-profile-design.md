# PPT Agent 模型 Profile 与多 Provider Adapter 设计

> 日期：2026-08-03
>
> 状态：设计确认稿
>
> 范围：用户按 Run 选择模型、统一模型调用协议、DeepSeek/Kimi/OpenAI Adapter、模型能力校验与安全配置投影
>
> 文档性质：后续实现与验收的权威增量规格

## 1. 结论

PPT Agent 要支持多个模型服务，但不能让 Runtime 直接依赖任一供应商的 HTTP 协议，也不能在一个
Run 中自动改用另一个模型。

本设计确认：

1. 普通用户为**每个新 Run**主动选择模型；后端不自动路由或自动降级。
2. 配置以 `llm.profiles[]` 表达可选模型 Profile；同一 Provider 可以拥有多个 Profile。
3. Profile 的 `name` 既是前端展示名，也是配置、API、Run 与幂等请求使用的稳定唯一引用；本设计
   **不引入额外 `id`**。
4. `provider` 是固定 Adapter 枚举：`deepseek | kimi | openai`；它决定协议转换实现。
5. `url`、下游 `model` 和明文 `key` 属于 Profile 配置。`key` 只在内存中的 Adapter 使用，绝不进入
   API、数据库、History、SSE、Trace 或日志。
6. Runtime 只依赖统一的 `Generate` 协议、规范化 Message/ToolCall、Capability 和统一错误；Adapter
   承担各 Provider 的图片格式、工具协议、reasoning continuation、流式与错误细节。
7. 每个 Run 创建时锁定一个 Profile 并持久化非敏感选择快照。Steering 必须沿用同一 Profile；Retry
   默认预选原 Profile，但用户可以在创建新 Run 前改选模型。
8. 所选模型能力不足时明确拒绝；绝不静默丢弃截图、绝不自动切换为其他 Provider。

这个设计仅扩展模型接入层。它不改变 HTML PPT 业务 Agent 的公开任务模型、五个业务工具、三个
Runtime 控制动作、连续 ReAct Loop、Completion Gate 或 11 种公共前端事件。

## 2. 问题与边界

当前核心 Runtime 只使用 `llm.Client.CallTool()`；`DeepSeek` 是唯一真实 Adapter。该实现已经有
内部多模态 ContentPart、图片引用解析和工具调用基础，但把 Provider、模型和视觉能力配置为一个
单一的 DeepSeek 实例，无法让用户选择模型，也无法可靠解决 DeepSeek V4 的文本模型限制。

各服务协议看似都能采用 OpenAI 风格，但不能简单共用一个 HTTP Client：

- DeepSeek V4 支持工具调用；Thinking + Tool Call 后续请求必须带回 `reasoning_content`。
- Kimi API 兼容 OpenAI 格式，支持图片输入与工具调用，但模型参数、推理和能力集需要独立声明。
- OpenAI 推荐的 Responses API 采用 items、function call output 与 response continuation，不应被
  Runtime 误认为普通 Chat Completions 的细节。

因此，统一的是**项目内部协议**，不是供应商请求 JSON 的最低公分母。

## 3. 不变的产品约束

```text
target.artifact = spec | presentation
target.level    = deck | slide
interaction     = talk | ask | execute
strategy        = chat | simple | complex
```

继续保持：

- 所有策略在一个连续 ReAct Loop 内运行；Complex 只额外拥有动态 Plan；
- Plan 不是 Workflow DAG，不增加逐 Step 子 Loop 或独立 Verify/Repair Stage；
- 正常成功退出必须显式调用 `finish` 并通过 Completion Gate；
- 模型业务工具固定为 `read_ppt/mutate_ppt/mutate_ppt/search_refs/render_slide`；
- Runtime 控制动作固定为 `update_plan/ask_user/finish`；
- 公共事件仍严格只有 11 种，不能为模型选择新增 SSE 事件；
- staging、evidence、context、strategy、phase、trace、Provider continuation 与 Provider 原始响应
  均为内部能力；
- 前端保持顶部 Tab、左中右三栏、侧栏伸缩、右侧 Thread Tabs 与底部 Composer，只做保留式完善；
- 不引入多 Agent、通用 shell、Repo 资产写入、进程级恢复、Eval 平台或复杂 verifier。

## 4. 概念模型

### 4.1 Provider、Adapter、Profile 与模型

| 名称 | 定义 | 对用户可见 |
| --- | --- | --- |
| Provider | 下游服务协议家族，如 DeepSeek、Kimi、OpenAI | 否 |
| Adapter | 把内部协议转换为 Provider 协议的代码实现 | 否 |
| Profile | 一条可配置、可选择的模型入口 | 是 |
| Profile `name` | Profile 稳定唯一身份，同时作为前端展示名称 | 是 |
| 下游 `model` | 实际发送给 Provider 的模型字符串 | 可只读展示 |

`name` 不只是可以随意改的文案。它是 Run 请求、Run 持久化、Retry 默认值和 Create Run 幂等哈希的
稳定键。Profile 改名等同删除旧 Profile 并新增新 Profile；旧名称不会自动迁移。

同一公司可以配置多个模型或多个网关：

```text
Kimi K3            → provider=kimi, model=kimi-k3
Kimi K2.6          → provider=kimi, model=kimi-k2.6
OpenAI GPT-5       → provider=openai, model=gpt-5
OpenAI Vision Proxy → provider=openai, model=<another model>
```

### 4.2 Profile 配置

应用配置使用如下 YAML。`key` 明文保存是本项目当前明确接受的部署决策；示例必须使用占位符，真实
密钥文件不得提交到仓库。

```yaml
llm:
  default: Kimi K3

  profiles:
    - name: DeepSeek V4 Pro
      provider: deepseek
      url: https://api.deepseek.com
      model: deepseek-v4-pro
      key: sk-xxxxxxxx

    - name: Kimi K3
      provider: kimi
      url: https://api.moonshot.cn/v1
      model: kimi-k3
      key: sk-xxxxxxxx

    - name: GPT-5
      provider: openai
      url: https://api.openai.com/v1
      model: gpt-5
      key: sk-xxxxxxxx
```

Profile YAML 的来源为 `LLM_CONFIG_PATH` 指向的文件；未设置时读取进程工作目录的 `config.yaml`。
仓库只提交不含真实 Key 的 `config.example.yaml`，本地 `config.yaml` 必须在 `.gitignore` 中。已有 `.env`
可继续承载其他应用配置，但不再作为 LLM Profile 的隐式来源：迁移后没有有效 `llm.profiles` 时服务应在
启动阶段失败，而不是悄悄退回单一 DeepSeek。

启动校验：

1. `llm.profiles` 至少有一项；
2. `name` 去除首尾空格后不能为空，长度 1–80 个 Unicode code point，不能含控制字符；
3. 所有 `name` 精确唯一；
4. `provider` 必须为当前编译版本支持的 Adapter 枚举；
5. `url` 必须是绝对 `https` 或在本地开发允许的 `http` URL；
6. `model`、`key` 不能为空；
7. `llm.default` 必须精确匹配一条 Profile `name`；
8. 任何配置错误都在启动失败前给出不含 Key 的错误信息。

只有启动时构造 Adapter 的配置对象可读取 `key`。以下投影一律不得包含 `key`、Authorization header
或 Provider 原始 body：配置日志、Trace、HTTP API、SSE、Thread History、SQLite Run、错误 Cause 和
测试失败输出。包含实际 Key 的本地配置文件必须被 `.gitignore` 覆盖，并按部署环境限制文件权限。

### 4.3 用户模型选择

前端从安全模型列表读取可选项：

```http
GET /api/v1/llm/profiles
```

```json
{
  "default": "Kimi K3",
  "profiles": [
    {
      "name": "DeepSeek V4 Pro",
      "model": "deepseek-v4-pro",
      "capabilities": {
        "vision": false,
        "tool_calls": true,
        "multiple_tool_calls": true
      }
    },
    {
      "name": "Kimi K3",
      "model": "kimi-k3",
      "capabilities": {
        "vision": true,
        "tool_calls": true,
        "multiple_tool_calls": false
      }
    }
  ]
}
```

这个 API 不返回 `provider`、`url`、`key`、上下文窗口、reasoning continuation 或内部限流设置。
`multiple_tool_calls` 是 Adapter 当前确认的能力；示例中的值不构成某个真实模型的永久承诺。Runtime
是否并发执行一批已返回的调用，仍只由现有“独立只读/渲染可限流并发、写操作按序执行”的业务规则决定。

Create Run 请求新增可选 `model`：

```json
{
  "client_request_id": "req_123",
  "model": "Kimi K3",
  "target": {"artifact": "presentation", "level": "deck"},
  "interaction": {"intent": "execute"},
  "instruction": "生成一份产品介绍 PPT"
}
```

- `model` 表示 Profile `name`，不是直接透传给 Provider 的原始模型字符串；
- 空值使用 `llm.default`；
- Composer 保存用户最近一次选择作为本地默认值，但后端配置的 `llm.default` 是新用户与无本地状态时的
  默认值；
- 前端可以根据当前 target 提前禁用不兼容项并说明原因，但后端能力校验始终是权威；
- Run 创建成功后，不允许在 Run 内切换模型；Steering 也不带模型字段；
- Retry 预选失败 Run 的 `model`，用户可在发送新 Run 前更改它。

### 4.4 Run 快照与幂等

Run 创建时将选择持久化为非敏感快照：

```go
type ModelSelection struct {
    Name     string // Profile name，例如 Kimi K3
    Provider string // kimi
    Model    string // kimi-k3
    URL      string // 内部审计使用；绝不投影到普通用户 API
}
```

Key 永不进入该快照。Create Run 幂等请求哈希必须包含 Profile `name`；同一 `client_request_id` 改用
其他模型属于不同请求，返回 `IDEMPOTENCY_KEY_REUSED`。历史 Run 永远解释为创建时的快照，即使之后
Profile 的 URL、模型或 Key 被改动。

若 Retry 的原 Profile 已不存在，Create Run 返回 `MODEL_PROFILE_NOT_FOUND`，要求用户重新选择；不得
猜测替换项或自动迁移旧名称。

## 5. Runtime 统一模型协议

### 5.1 Runtime 面向的接口

Runtime 不直接依赖 `DeepSeek`、`Kimi`、`OpenAI` 或 Chat Completions/Responses 请求体。核心调用收敛为：

```go
type Provider interface {
    Name() string
    Model() string
    Capabilities() Capabilities
    Generate(context.Context, GenerateRequest) (GenerateResponse, error)
}
```

```go
type GenerateRequest struct {
    Messages      []Message
    Tools         []ToolSchema
    ImageResolver ImageRefResolver
    Continuation  *ProviderContinuation
    OnRetry       func(attempt int)
}

type GenerateResponse struct {
    Content       []ContentPart
    ToolCalls     []ToolCall
    Continuation  *ProviderContinuation
    Usage         Usage
}
```

`Message` 使用现有规范化角色、文本和 Runtime 控制的 `ImageRef`。Provider 只能经由
`ImageResolver` 获取当前 Run 授权的截图字节，不能接收模型填写的 URL 或本地路径。

当前 `Chat`、`Stream`、`CallTool` 不是 Runtime 的三个平行协议：核心 Runtime 迁移到 `Generate`。
如果产品未来需要普通聊天流式显示，可在同一 Provider 协议之上增加输出流能力，但不能让其绕过
统一 Message、Capability、错误或 continuation 模型。

### 5.2 Capability

```go
type Capabilities struct {
    Vision                  bool
    ToolCalls               bool
    MultipleToolCalls       bool
    Reasoning               bool
    RequiresReasoningReplay bool
    ImageInputMIMEs         []string
    MaxImageBytes           int
}
```

Capability 是 `provider + model` 的代码知识，由 Adapter 内部 Model Capability Registry 提供；不是
管理员可用 YAML 任意宣称的布尔开关。未知模型默认保守：不允许视觉输入、多工具调用或其他受限能力。

所有正常 Run 都依赖工具调用，因为 `finish` 是 Runtime 控制工具。`presentation + execute` 额外要求：

```text
ToolCalls = true
Vision = true
```

当前 HTML PPT 业务中，视觉是“渲染截图作为下一轮 Observation 进入同一 ReAct Loop”的能力，不是一个
独立视觉 Agent 或单独 Verify Stage。

Capability 缺失时：

```json
{
  "error": {
    "code": "MODEL_CAPABILITY_MISMATCH",
    "message": "所选模型不支持页面图片观察，请选择支持视觉输入的模型。",
    "retryable": false,
    "details": {"required_capability": "vision"}
  }
}
```

不自动切换 Kimi/OpenAI，不将截图改写为无保证的文字描述，也不向模型静默省略图片。

### 5.3 Provider continuation

`ProviderContinuation` 是 Adapter 私有的、只存在 Run 内存和必要内部运行记录中的不透明状态：

```go
type ProviderContinuation struct {
    Provider string
    Model    string
    Opaque   json.RawMessage
}
```

它可承载 DeepSeek 必须回传的 `reasoning_content`，或 OpenAI Responses 的 response/item continuation。
Runtime 只负责把同一 Profile 的 continuation 原样带入下一轮 `Generate`；不解析、不写入普通 Thread
History、不发送给前端，也不能跨 Provider 使用。

每个 Run 固定一个 Profile 是该机制的前提。运行中自动故障切换会让 continuation、工具 call ID、提示词
语义、幂等和证据归属不再可靠，因此不属于本设计。

## 6. Adapter 责任

```text
Composer model selection
        │
        ▼
RunService resolves Profile by name
        │
        ├── validates task capabilities
        ├── snapshots non-sensitive selection
        └── pins Provider for the Run
                    │
                    ▼
            Single ReAct Runtime
                    │ Generate(Messages, Tools, ImageRefs, Continuation)
                    ▼
              Provider Adapter
       ┌────────────┼────────────┐
       ▼            ▼            ▼
  DeepSeek      Kimi         OpenAI
```

每个 Adapter 负责：

1. 将统一 Message/ToolSchema 转换为真实请求；
2. 将 Runtime ImageRef 解析、压缩并转换为 Provider 接受的图片输入；
3. 将真实工具调用、文本与 continuation 转换回规范化响应；
4. 遵守该 Provider 的工具调用和 reasoning 回放规则；
5. 映射为统一 `AgentError` 事实，避免泄露 Provider body、URL 或 Key；
6. 尊重 `context.Context` 取消并清理请求资源；
7. 在 Capability 不支持时 fail closed。

Adapter 不得：

- 注册、隐藏或替换 PPT 业务工具；
- 改变工具 Schema、Completion Gate、Plan、Strategy 或工具并发业务规则；
- 直接读写项目文件、staging 或数据库；
- 自动更换用户选择的 Profile；
- 向公共事件注入 Provider 原始 token、reasoning 或内部请求数据。

首批实现：

| Adapter | 下游协议 | MVP 用途 |
| --- | --- | --- |
| `DeepSeekAdapter` | DeepSeek Chat Completions / 官方规则 | 文本、工具调用、Thinking continuation；不承担视觉 Run |
| `KimiAdapter` | Kimi OpenAI-compatible Chat Completions | 支持图片 Observation 的 PPT Run |
| `OpenAIAdapter` | OpenAI Responses API | 支持图片、工具与 Provider-native continuation 的 PPT Run |

Kimi 与 DeepSeek 虽均兼容 OpenAI 格式，也必须保留独立 Adapter：二者的模型能力、reasoning 参数和多轮
工具上下文约束不是稳定的同一契约。

## 7. 错误、取消、重试与安全

沿用统一 `AgentError`。新增稳定错误码：

| 错误码 | HTTP | 类别 | 用户动作 |
| --- | --- | --- | --- |
| `MODEL_PROFILE_NOT_FOUND` | 422 | user_action_required | 重新选择已配置模型 |
| `MODEL_CAPABILITY_MISMATCH` | 422 | user_action_required | 选择满足能力的模型 |
| `MODEL_PROVIDER_UNSUPPORTED` | 500 | terminal | 管理员修正启动配置或部署版本 |

Provider 网络、限流、5xx 与超时继续映射为 `PROVIDER_UNAVAILABLE`；它是唯一允许自动重试的 Provider
错误。Provider 4xx、Capability 错误、取消和幂等冲突不得被自动重试。

用户取消 Run 时，`context.Context` 必须传入 Adapter 的 HTTP 请求、图片读取和流式读取。取消结果是
`RUN_CANCELED`，不能被 Adapter 错误包装成 `PROVIDER_UNAVAILABLE`。

模型列表 API、Create Run API、Run 查询、SSE、History 和 Public Error 都不能泄露明文 Key。开发和测试
日志也必须对 `Authorization` 与 Profile Key 做脱敏。

## 8. 前端体验

在现有 Composer 的保留式布局中增加模型选择器：

- 从 `/api/v1/llm/profiles` 加载；
- 显示 Profile `name`；可辅以 `model` 和“支持页面观察”标签；
- 初始值为本地最近选择，其次为服务端 `default`；
- 用户发送时把当前 `model` 带入 Create Run；
- Run 运行期间选择器随其他新建 Run 配置一同锁定，Steering 不可更改模型；
- 当前 target 需要 Vision 时，不兼容项显示原因并禁止选择或提交；后端仍重复校验；
- Retry 新建 Run 时默认恢复原 Run 的 Profile `name`，让用户在发送前可改选。

不增加模型选择专用 SSE 事件，不把 Provider、URL、Key、Capability 原始细节放入 Timeline。

## 9. 数据、API 与迁移

### 9.1 Create Run 与查询

`POST /api/v1/threads/{thread_id}/runs` 的请求和 `GET /api/v1/runs/{run_id}` 响应增加：

```json
{"model": "Kimi K3"}
```

这里 `model` 始终表示 Profile `name`。下游真实模型字符串仅在 `/llm/profiles` 的只读字段与 Run
内部审计快照中存在。

### 9.2 持久化

`runs` 增加非敏感快照字段或一个等价 JSON 字段：

```text
model_profile_name
model_provider
model_name
model_url
```

迁移应让历史 Run 保持可读：它们的模型选择字段为空，API 返回 `model: null`；历史 Run 不伪造为当前
默认模型。所有新 Run 必须拥有快照。

### 9.3 无自动路由

RunService 的选择顺序固定：

```text
request.model
→ llm.default（request.model 为空时）
→ exact Profile name
→ capabilities check
→ pin Provider
→ create Run
```

没有“可用模型替换”“视觉模型备用”“最便宜模型”或“失败自动转 Provider”分支。Profile 选择属于用户
意图与一次 Run 的执行输入，不属于 Runtime strategy。

## 10. 测试与完成定义

### 10.1 配置与 Registry

- 解析一个 Provider 多 Profile；
- 同名、空 name、未知 provider、空 key/model、非法 URL、缺失 default 均启动失败且不泄露 Key；
- 模型列表只返回安全字段与默认 Profile；
- Capability 来自 Adapter/Model Registry，不能由 YAML 虚构。

### 10.2 Adapter 合约

- 以 fake HTTP Server 测试 DeepSeek、Kimi、OpenAI 的请求映射；
- 文本、工具调用、多个 Tool Calls、工具结果回传、ImageRef 图片输入与图片上限均有覆盖；
- DeepSeek Thinking 工具轮次正确回传 continuation；
- Kimi/OpenAI 视觉请求真实包含由受控 ImageRef 转换出的图片；
- 不支持 Vision 的 Profile 对 `presentation + execute` 返回 `MODEL_CAPABILITY_MISMATCH`；
- 所有 Adapter 都响应 context cancel；
- 请求、错误、测试输出不泄露 Key。

### 10.3 Run 与前端

- Create Run 使用显式 `model` 并在 Run 查询中返回同一 Profile name；
- 缺省模型采用 `llm.default`；
- Profile 不存在、能力不匹配和相同幂等键改模型均符合错误契约；
- Run 持久化保存创建时快照，改动配置不改变历史 Run；
- Steering 不能更改模型；
- Retry 默认原 Profile，但允许用户改选；
- 模型选择器不改变既有三栏和 Composer 交互；
- 公共事件仍为 11 种，SSE 续传保持可用。

### 10.4 最终验收

至少通过：

```bash
cd backend
go test -count=1 ./...
go test -race -count=1 ./internal/llm ./internal/workflow ./internal/run ./internal/service ./internal/store/sqlite
PPT_RUN_CHROMIUM_TEST=1 go test -count=1 -run TestNodeSlideRendererWithRealChromium -v ./internal/workflow

cd ../frontend
pnpm test -- --run
pnpm tsc
pnpm lint
pnpm build

cd ..
git diff --check
```

真实收费 Provider 的 smoke test 不应使用测试中的真实 Key 自动执行。若当前任务所有者明确授权，可额外
分别验证：DeepSeek 工具调用、Kimi 或 OpenAI 的“生成 → render_slide → 图片 Observation → 修正 →
finish”闭环；报告必须明确区分 fake-wire 测试与真实 Provider 结果。

## 11. 实施顺序

1. 定义 Profile 配置、启动校验、安全投影与 Provider Registry；
2. 将核心 Runtime 从 `CallTool` 收敛到统一 `Generate`，保持现有 DeepSeek 行为与测试不回归；
3. 在 Run 数据模型、迁移、Create Run 幂等哈希和 API 中加入 Profile 选择与快照；
4. 实现 DeepSeekAdapter 的 continuation 与 Capability 适配；
5. 实现 KimiAdapter，并以受控截图完成视觉 Observation wire 测试；
6. 实现 OpenAIAdapter（Responses API）；
7. 增加模型列表 API、Composer 选择器和 Retry 默认选择；
8. 完成取消、错误、安全、并发工具与 Chromium 回归测试。

不得在第 5、6 步之前把 `presentation + execute` 的视觉约束放松为文本模型可运行。

## 12. 非目标

- 不让用户输入任意 Provider URL、模型名或 API Key；只能选择预配置 Profile；
- 不做运行中换模型、自动路由、自动故障切换或文本/视觉模型拼接；
- 不做模型价格比较、token 账单、限额控制或评测平台；
- 不暴露 Provider、URL、Key、原始 reasoning 或 Provider 请求/响应给普通用户；
- 不把 OpenAI-compatible 当作不同 Provider 的正确性保证；
- 不修改 PPT 资源模型、工具 Schema、业务 Prompt、公共事件数量或前端主布局。

## 13. 设计参考

- OpenAI Codex App Server 的 `model/list` 与 turn-scoped `model`：
  https://github.com/openai/codex/blob/main/codex-rs/app-server/README.md
- OpenAI Responses API 的图片输入和 function call output：
  https://platform.openai.com/docs/api-reference/responses-streaming/response/web_search_call?lang=curl
- Kimi API 的 OpenAI compatibility、图片输入和工具调用：
  https://platform.kimi.com/docs/overview
- DeepSeek Tool Calls：
  https://api-docs.deepseek.com/guides/tool_calls
- DeepSeek Thinking Mode 的 `reasoning_content` 回放要求：
  https://api-docs.deepseek.com/guides/thinking_mode
- DeepSeek V4 文本模型与视觉代理说明：
  https://api-docs.deepseek.com/quick_start/agent_integrations/github_copilot/
