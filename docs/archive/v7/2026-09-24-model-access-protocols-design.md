# 模型接入：协议、品牌与地址分离

日期：2026-09-24。依据用户本次确认，覆盖此前模型设置设计中固定供应商适配器、固定地址及 OpenAI 服务端续接的约定。

## 配置合同

每条 `llm` 配置包含 `name / protocol / base_url / model / key`。`protocol` 只允许 `responses`、`anthropic`；协议与地址必须显式填写，不推断旧配置，不保留 Chat Completions 实现。

```yaml
llm:
  - name: 中转 GPT
    protocol: responses
    base_url: https://gateway.example.com/v1
    model: gpt-5.6
    key: your-api-key
main-road:
  default: 中转 GPT
side-road:
  default: 中转 GPT
```

`provider` 不再是用户填写或写入 YAML 的配置项，而是后端从 `model` 推断的只读展示信息。`protocol` 决定请求结构、响应解析和鉴权头；`base_url` 表示真正的接入地址；`model` 原样传给该接口。品牌识别不会改动协议、地址或模型标识，也不限制模型的协议组合。

识别规则忽略大小写，取命名空间后的模型名，再按有边界的模型系列前缀匹配：DeepSeek、Kimi/Moonshot、MiMo、GPT/ChatGPT/o1/o3/o4、Claude、Gemini、Qwen、MiniMax、GLM/ZAI。识别不到时返回 `custom`，显示通用图标，仍可正常配置和调用。后端统一推断，设置读取和模型列表共用同一套规则，前端不重复实现正则。

`base_url` 是完整 API 前缀，协议适配器只追加 `/responses` 或 `/messages`。例如 `https://gateway.example.com/team/v1` 会产生 `/team/v1/responses`，不会插入或重复 `/v1`。接受 HTTP(S)，拒绝内嵌凭证、查询参数、fragment 和已包含完整接口路径的地址。尾部斜线统一移除。

## 协议实现

后端只保留 `ResponsesAdapter`、`AnthropicAdapter`；共用 HTTP 重试、超时、错误脱敏、图片解析和工具参数处理。原 Kimi、DeepSeek Chat Completions 适配器及 OpenAI 原生 response ID 续接删除。

- Responses：调用 `/responses`，Bearer 鉴权；显式 `store: false`，不传 `previous_response_id` 或 `conversation`。每次重发当前上下文，工具调用和图片结果使用 Responses item。服务端输出中的 reasoning/phase 等协议信息保留为客户端不透明回放数据，并校验历史、工具及接入地址；历史改写或压缩后丢弃失配的回放。它不是远端会话引用，也不进入公开回复。
- Anthropic：调用 `/messages`，使用 `x-api-key` 和 `anthropic-version: 2023-06-01`；系统提示独立，工具采用 `tool_use / tool_result`，同批工具结果合并为紧随 assistant 的 user 消息，截图保留在对应 `tool_result`。继续沿用产品关闭 thinking 的策略；未指定输出预算时提供必需的 `max_tokens: 4096`。
- 返回输出截断、空响应或缺少有效文本/工具调用时，不将其视为成功完成；工具参数仍按现有错误分类处理。
- HTTP 客户端不跟随重定向，防止中转地址把带密钥的请求转发到其他位置。

主旁路选择、重试时长及故障接替策略保持既有规则。协议和地址进入不可变配置快照及其指纹；运行中使用既有快照，新任务使用新配置，恢复时校验配置指纹。路由检查点增加协议标识。

## 设置与图标

设置接口返回两个协议选项，界面显示 `Responses` 和 `Anthropic`。模型卡片仅保留名称、模型、协议、地址和密钥五项。新卡片默认 Responses，地址由用户填写；不再通过品牌预设覆盖用户的接入地址。保存成功保持编辑面，仅点击卡片外部或取消时翻回正面。

更换协议或地址时必须重新输入密钥，前后端均校验。单纯更改名称或模型标识可以保留原密钥，包括在同一个中转地址下切换不同品牌的模型；密钥不通过读取接口返回。

模型选择列表公开自动识别的 `provider`、配置的 `protocol`，不公开实际地址和密钥。设置卡片与聊天模型选择器复用 `ModelProviderIcon`，保存或刷新后采用后端推断的品牌图标，包括 MiMo。

实际开发配置和示例配置均移除手填 provider 字段，保留协议、地址、模型标识和密钥。按开发期约定直接切换结构，不保留旧 provider 字段的兼容解析。`DESIGN.md` 不属于本次修改范围。

## 验收

已补充或更新协议 payload、多工具结果、图片授权、完整上下文回放、历史失配、配置保存及密钥隔离的定向用例；原 HTTP 重试、取消和错误脱敏测试保留到公共传输测试文件。

按仓库约定，本次不执行测试、构建、浏览器自动化或真实模型请求。以下由用户手动运行：

```sh
cd backend
go test ./internal/config ./internal/llm
go test ./internal/httpapi -run 'TestLLMProfilesEndpointIsSafe|TestCreateRunSelectsExplicitAndDefaultProfiles'
```

```sh
cd frontend
npx vitest run src/features/settings/modelSettingsDraft.test.ts src/features/agent/ModelSelector.test.tsx
npm run build
```

交互验收包括：表单只显示五项、保存后品牌图标自动更新、未知模型显示通用图标、第三方路径保存及刷新、同地址与协议下跨品牌换模型保留密钥、换地址或协议后要求输入新密钥、保存和内部点击不翻回正面，以及两个协议分别完成带工具/截图的多轮对话。

协议依据：[OpenAI 会话状态](https://developers.openai.com/api/docs/guides/conversation-state)、[Anthropic Messages](https://platform.claude.com/docs/en/api/messages/create)、[DeepSeek Anthropic 接入](https://api-docs.deepseek.com/guides/anthropic_api/)、[Kimi Claude Code 接入](https://platform.kimi.com/docs/guide/claude-code-kimi)。
