# Contextual Prompt Polish Design

> 日期：2026-08-23  
> 状态：已确认，待实施  
> 范围：Composer 的 `Enhance` 更名并升级为 `Polish`；新增一次无副作用、带项目上下文的 LLM 输入润色能力。

## 1. 决策

`Polish` 是 Composer 的统一输入辅助能力：把用户的自然语言草稿和当前项目的只读上下文交给 LLM，返回一段更清晰、更具体、可以直接发送给 PPT Agent 的指令。

它不是 Run，不进入 Runtime，不创建或取消 Run，不写入 Thread History，不发 SSE，不使用工具，不持有 checkpoint，也不打断正在执行的任务。无论用户最终会创建新 Run，还是把文本作为 active Run 的 steering 发送，Polish 的行为都完全相同。

```text
Composer draft + project/thread/target read-only context
  -> Polish service
  -> one LLM text-generation call
  -> polished draft replaces Composer text
```

## 2. 目标与非目标

目标：

- 将口语化、指代化或感受式需求转写为清晰、具体、可执行的 Agent 指令。
- 使用项目上下文消解“这一页”“延续前文”“更有冲击力”等表达。
- 将视觉、交互、动效意图规范化为有可观察结果的专业设计表达。
- 保留用户原始意图、语言、目标范围和明确限制，不臆造事实或需求。
- 保持一次同步请求和直接替换文本的轻量体验。

非目标：

- 不创建第二套 Agent Harness、Run 生命周期或异步任务。
- 不区分 new-run、steering 或其他业务场景；Polish 只处理 Composer 当前草稿。
- 不读取或改变 active Run 的内存状态、计划、工具轨迹、SSE 或 checkpoint。
- 不做前后对照、优化历史、撤销历史、术语教学卡片或人工确认步骤。
- 不自动决定模型、scope、mode、计划或最终执行方式。
- 不把外部 Skill 全文复制到产品提示词中。

## 3. 产品交互

所有用户与代码语义从 `Enhance` 切换为 `Polish`。中文 UI 使用“润色表达”，忙碌提示使用“正在润色表达”。保留现有 Sparkles 图标、输入框右上角位置、文本内边距和低干扰横向扫光，但动画时长以真实网络请求为准，不再强制一秒。

客户端仅维护局部、非持久化状态：

```text
idle -> polishing -> idle
                  -> failed -> idle
                  -> canceled -> idle
```

- 仅当文本非空且 Composer 可编辑时显示/启用按钮。
- `polishing` 时，文本框为只读，Polish 与发送按钮禁用；卸载或发起更新请求时中止旧请求。
- 响应成功且仍属于最新请求时，以 `polished_instruction` 替换输入文本，恢复焦点并把光标置于末尾。
- 失败、超时或取消时，原始文本及选择范围保持不变；错误使用既有 Composer error 区域展示。
- 正在执行的 Run 不会被暂停、取消、steer 或写入任何消息；用户随后点击发送时沿用既有提交逻辑。

## 4. API 与服务边界

新增唯一端点：

```text
POST /api/v1/projects/:project_id/polish
```

请求：

```json
{
  "instruction": "把这一页做得更有冲击力",
  "thread_id": "optional-active-thread-id",
  "scope": {"artifact": "ppt", "level": "slide", "slide_id": "slide-03"},
  "mode": "execute",
  "model": "Kimi K3"
}
```

响应：

```json
{
  "polished_instruction": "请强化当前页面的核心信息层级……",
  "changed": true,
  "prompt_version": "2026-08-23.v1"
}
```

服务端以 route 的 `project_id` 为项目身份真相；若给出 `thread_id`，必须验证该 Thread 属于该项目。客户端只能提供草稿、当前 Composer 的 target/mode/model 选择，不能提交自拼的项目正文、历史或页面内容。

`PolishService` 只依赖 Store、LLM Registry 和只读 Context Engine 投影。它解析 profile，生成上下文，调用 `profile.Adapter().Generate`，并返回文本。它不依赖 `run.Engine`、EventBus、RunService、锁、幂等表或持久化写操作。

模型只需要能进行文本生成；Polish 不要求 tool calls、vision 或 provider continuation。调用使用短 deadline（建议 12 秒），超时或上游不可用映射到现有 `PROVIDER_UNAVAILABLE`，模型不存在映射到 `MODEL_PROFILE_NOT_FOUND`。空白、过长或无效 scope 使用现有 `BAD_REQUEST` / `INVALID_SCOPE`。模型返回空结果时使用内部 `POLISH_OUTPUT_INVALID`，对外安全投影为可重试的服务错误。

## 5. Polish Context

Context Engine 保持项目上下文的唯一装配入口。新增其内部的、无副作用的 `PolishContext` 投影/装配选项，而不是在 HTTP 或 Service 层重复读取 JSON、HTML 和历史。

`PolishContext` 以信号优先级裁剪，建议总输入预算 4,000 tokens 左右：

1. 当前 Composer scope/mode（不含草稿正文）；
2. 项目标题、outline 的目标/受众/语言/requirements/prohibitions；
3. design 的主题、方向、信息密度；
4. 目标页的 title、role、key message、规格和 HTML 摘要；deck scope 仅带页面/章节摘要；
5. 可选的线程记忆与最近少量已显示的用户决策。

投影不含完整 HTML、资产索引、ContextRef、Provider 私有状态、思维链、工具结果或 active Run 内存状态。所有项目内容在提示词中明确标记为不可信参考数据，不能改变系统规则。用户本次草稿高于可变的当前页面/设计基线；它请求改变基线时，Polish 应清楚表达变更，而非强行保留旧状态。

## 6. Prompt 设计：Design Intent Normalization

Polish 使用独立的文件化 prompt registry，例如 `backend/prompts/polish/`。它遵循 Runtime prompt 的版本与 hash 可追踪原则，但不复用 Runtime 的工具、完成、计划或资源契约模块。

静态提示词包含以下职责：

1. 保留用户明确意图、语言、范围、否定条件与语气；已经清晰时原样返回。
2. 结合上下文解析指代，并消除与项目目标、受众或当前目标页的无意冲突。
3. 将“高级、顺滑、有冲击力、自然、灵动”等感受式描述落实为可观察的视觉、排版、信息层级、交互或动效要求。
4. 仅在有帮助时使用准确术语；每个术语必须对应可感知的效果，不能用于炫技。
5. 不臆造业务事实、数字、受众、品牌规范、调研结论、框架、组件库、动画库、CSS 属性或精确实现参数。
6. 只输出一段可直接发送的最终指令：无 Markdown、标题、解释、术语清单、前后对照或执行承诺。

第 3–4 条定义为 `Design Intent Normalization`。它借鉴两类公开 Skill 的方法而非复制其文字：一类要求保留意图、优先可观察效果、避免擅自指定实现；另一类要求从用户的感受理解意图、映射到精确术语、避免创造不存在的术语。来源：

- https://github.com/oil-oil/vibe-hub-skill/blob/main/skills/vibehub/SKILL.md
- https://github.com/emilkowalski/skills/blob/main/skills/animation-vocabulary/SKILL.md

提示词中的少量自有映射示例：

| 用户表达 | 可输出的规范化意图 |
| --- | --- |
| “一项项出来” | 采用按顺序错峰出现的入场节奏，帮助观众依次阅读信息。 |
| “切换别生硬” | 使用保持空间连续性的过渡，使状态变化可追踪而不突兀。 |
| “点一下有反馈” | 提供克制的按压反馈，明确确认操作但不干扰连续使用。 |
| “高级一点，别花哨” | 采用克制、低饱和、留白充分的视觉方向，以排版和层级而非装饰建立质感。 |

这些例子是 prompt 行为约束，不是供模型硬匹配的完整词典。

## 7. 验收标准

1. UI 与代码术语统一使用 Polish / 润色表达，不再保留 `Enhance` 的产品语义。
2. Polish 成功时以服务端返回文本替换草稿；失败、取消或过期响应绝不覆盖用户原文。
3. 任意一次 Polish 都不会创建/修改 Run、Timeline、SSE、Thread History、checkpoint 或项目内容，也不影响 active Run。
4. 后端仅从权威项目/线程数据装配上下文，拒绝跨项目 Thread 和非法 scope。
5. 输出可消解当前目标页、项目目标和线程已确认决策，但不虚构缺失事实或实现细节。
6. 视觉/交互/动效的口语表达会在适当时转为可观察、可执行的设计语言；清晰文本不会被无意义扩写。
7. 支持文本的模型均可用于 Polish；不因缺少 tool-call 或 vision 能力被拒绝。
8. 所有新 Go、前端和提示词测试通过，已有 Run/steering 测试保持通过。
