# 工具说明集中配置与装配设计

日期：2026-10-12

状态：工具 JSON 加载与装配方案已于 2026-10-14 移除。完整文案转存 `backend/prompts/tools.md` 供审阅，运行时代码中的工具及参数描述暂为空字符串，待用户确认后直接写入代码。下文保留原方案记录。

## 设计理由

改动前，工具功能、输入说明和输出说明分散在工具实现、资源 Schema、辅助调用和 LLM 适配代码中。原 `backend/prompts/TOOLS.md` review 稿已经包含描述去重、输入与输出说明分离，以及 `edit_outline.init` Schema 补齐方案；这些内容现已落实到 JSON 说明和后端契约。

原 `ModelToolSchemas` 将完整 OutputSchema 序列化后附加到工具 description。该行为只提供文本说明，不执行工具输出验证，且带入了大量 Schema 结构、分支编号和重复业务字段说明；此路径现已删除。

将模型可见的工具说明集中到 `backend/prompts/tools.json`，使用户能够直接 review 和调整文案。后端继续维护结构与运行规则，统一校验并装配说明，避免文案与代码长期双份维护。

## 参考文件

以下路径相对于项目根目录：

- `AGENTS.md`：项目约束与文件保护范围。
- `backend/prompts/tools.json`：工具说明的唯一维护入口；原 TOOLS.md 的最新 review 内容已迁移，原文件删除。
- `backend/prompts/playbook/tool.md`：已确定的公共错误结构、拒绝语义和重试规则，仅作为参考。
- `backend/embedded_prompts.go`、`backend/internal/prompt/loader.go`、`backend/internal/prompt/catalog.go`：生产提示词嵌入与加载入口。
- `backend/internal/workflow/default_tools.go`、`tools.go`、`runtime.go`、`reviewer.go`、`context_window.go`：工具注册、披露和不同角色入口。
- `backend/internal/prompt/tools.go`：JSON 解析、字段路径校验、说明装配与 Schema 副本处理。
- `backend/internal/tooldefinition/catalog.go`、`inputs.go`、`outputs.go`、`errors.go`：完整工具目录、输入／输出契约与入口错误集合。
- `backend/internal/workflow/ppt_tools.go`、`tool_arguments.go`：实际工具执行与参数校验。
- `backend/internal/llm/token_estimate.go`、`client.go`、`responses.go`、`anthropic.go`：最终定义的长度统计和供应商适配。
- `backend/internal/model/agent_error.go`、`backend/internal/workflow/resource_errors.go`、`tool_dependencies.go`：公共错误定义与工具错误路径。
- `backend/internal/commandresult/text.go`、`backend/internal/service/git_commit.go`、`thread_naming.go`：辅助提交工具与同名工具的上下文差异。
- `backend/schemas/`：共享资源结构及其其他使用场景。

## 范围与边界

- 迁移全部模型可见工具，包括主 Agent、Reviewer 和辅助调用；不能只接入业务工具注册器。
- 当前 review 稿包含 23 个工具：`read_resource`、`render_slide`、`read_image`、`load_component`、`load_skill`、`edit_manifest`、`edit_design`、`edit_spec`、`edit_html`、`edit_outline`、`git_commit`、`run_command`、`create_plan`、`update_plan`、`request_privilege`、`ask_user`、`review_task`、`finish_task`、`submit_review`、`rename_thread`、`compact_context`、`polish_instruction`、`handoff_thread`。实现时仍须核对最新代码的完整目录。
- JSON 维护说明文字，不维护执行实现、权限、风险、可用阶段或重试策略。
- 后端继续维护输入／输出结构、必填字段、枚举、格式、长度和数量约束，以及 Runtime 行为。
- 保持现有工具名称、正常拒绝语义及供应商调用类型；除下述 `read_resource` 正常返回精简外，不改变其他工具的实际返回结构。本次不新增前端功能，也不为辅助调用额外注入公共错误提示词。
- 将 review 稿中已确认的 `edit_outline.init` 完整结构补齐到真实输入 Schema，使嵌套参数校验有结构依据；保持 Runtime 分配 ID 和现有业务限制。
- 直接切换到新实现，禁止保留旧文案回退、双来源、双拼接或兼容加载分支。

## tools.json 格式

顶层为说明条目 ID 到工具说明的映射。每个条目保持以下六个字段，不增加 `returns.description`、`returns.fields` 等包装层：

```json
{
  "tool_name": {
    "function": "工具功能及主要效果。",
    "cases": [
      "适用场景。"
    ],
    "rules": [
      "重要前置条件或行为限制。"
    ],
    "args": {
      "arg1": "参数含义。",
      "questions.question": "嵌套参数含义。"
    },
    "returns": {
      "field": "返回字段含义。"
    },
    "errors": {
      "RESOURCE_NOT_FOUND": "该工具中此错误的具体触发情况。"
    }
  }
}
```

- `function` 是非空字符串；其余字段分别为字符串数组或字符串映射，没有内容时使用 `[]` 或 `{}`。
- 映射中的值及数组中的条目必须是非空说明。JSON 不允许注释、重复键、尾逗号或未知配置字段。
- 保留现有说明语言；本次不额外增加翻译。
- `args`、`returns` 的键是实际参数／返回字段的语义路径，不是 Schema 关键字路径。
- `errors` 的键是该入口实际业务失败使用的公共错误 `code`，不是数字、category、原因文本或 HTTP 状态；通用框架错误不作为工具说明条目。

## 说明与结构的职责

| 内容 | 维护来源 | 模型侧处理 |
| --- | --- | --- |
| 功能、场景、规则 | tools.json | 拼入工具 description |
| 输入字段说明 | tools.json 的 args | 注入输入 Schema 对应字段的 description |
| 输出字段说明 | tools.json 的 returns | 拼成 Returns，不输出完整 OutputSchema |
| 工具错误说明 | tools.json 的 errors | 拼成 Errors |
| 输入／输出结构和约束 | 后端 Schema | 输入结构正常发送；输出结构仅作为内部依据 |
| 执行、权限、阶段、风险、重试限制 | 后端 Runtime | 沿用实际运行规则 |

输入说明不重复拼成工具描述中的 Args。`function` 直接作为首段，也不额外添加 Function 标题。

工具说明只在 JSON 中维护。共享资源 Schema 若仍承担其他用途，其说明可以继续服务那些用途；构造工具视图时应使用独立副本，去除旧工具视图描述后从 JSON 注入，不得修改共享 Schema 或其他已审定提示词的实际内容。

结构性信息不能随旧说明一起删除，例如 `type`、`required`、`enum`、`oneOf`、`anyOf`、`$ref` 和实际字段结构。现有输出传输注解也不是返回字段，不能伪造成 Returns 条目。

## 字段路径与校验

### 路径约定

- 顶层字段使用字段名，如 `resource`、`content`、`decision`。
- 嵌套字段使用点路径，如 `questions.question`、`questions.options.label`、`diagnostics.overflow`。
- 数组不写 `[]`，不写数字下标；`questions.question` 表示 questions 数组每个对象的 question 字段。
- 数组字段自身的说明使用 `questions`。原有数组元素说明应按含义合并到数组字段或真实子字段，不保留单独的 `questions[]` 条目。
- 禁止使用 `properties`、`items`、`oneOf[0]`、`anyOf[0]`、`$defs` 等结构路径作为字段路径。

### 匹配规则

- `args` 对照当前入口的输入 Schema；`returns` 对照当前入口的输出 Schema。
- 解析器需要穿透数组 items、本地引用及组合分支，不能只检查第一层 properties。
- 输入分支中的同一语义字段可以出现在多个结构节点；相同 args 路径应注入所有对应的有效字段节点，不向 null 分支或无关字段注入。
- 多个路径复用同一个引用字段时，说明必须一致；矛盾文案直接报错，不能按遍历顺序覆盖。
- 输出各正常分支可以含不同字段。某字段只存在于一个分支时仍是合法路径，其说明应交代适用条件；不用分支编号表达字段名称。
- 一个路径无法匹配真实字段时必须报错，并指出说明条目、args／returns、错误路径和匹配失败原因。禁止忽略、猜测或自动创建字段。
- 缺失工具条目、未绑定说明条目、同一条目的重复键或已删除字段残留同样必须报错。
- 校验针对后端声明的完整工具与上下文目录，不以当前请求中被权限或功能开关筛选后的工具子集判定配置无效。
- 每个顶层输入参数应有说明；有意义的嵌套输入按实际结构补齐。输出只校验已声明的说明路径，不强制逐个描述全部内部叶子字段。

## 输出说明精简

- 完整资源内容使用 `content` 描述其完整返回格式，不再展开 Manifest、Design、Spec 等内部业务字段。
- `read_resource` 的资源选择输入键统一为 `type`，取值仍为 manifest、design、outline、spec、html；不接受旧 `resource` 参数。Spec／HTML 必须带 `slide_id`，全局资源禁止带 `slide_id`。该外部 `type` 表示资源内容类型，与内部 `Resource.Type` 的 deck／slide 定位类型不同，内部结构和其他工具／事件的 `resource` 字段保持不变。
- `read_resource` 正常返回仅含 `content`，不回显输入的 `type`、`slide_id`，也不返回 `resource` 或 `status`。实际内容继续保持完整解析后的 JSON 对象或完整原始源码；tools.json 的 content 文案保持简洁。全局资源未创建，或当前 Outline 中存在的页面尚未创建 Spec／HTML，均返回 `{"content":null}`，仍为正常结果。页面读取先严格读取并验证当前会话／磁盘的 Outline；无效或已删除的页面、Outline 未初始化、授权不足、已有内容损坏及实际读取失败继续返回公共错误，不吞成 null，空内容处理不变。
- 正常未创建结果不带源码版本戳或验证 Evidence，不计入工具失败；调用正常完成不代表源码存在。依赖图对先前读取仅安排等待，不将读取列为写入成功前置条件；源码可用性继续由实际文件和独立版本戳检查，缺失结果不会授予精确编辑或命令修改已有源码的权限，首次创建仍沿用既有业务逻辑。
- `read_resource` 的事件目标、展示和依赖判断从对应调用输入的 `type`、`slide_id` 取得资源身份；Runtime 活动分类和 HTML 错误绑定也使用 `type`，错误 measure 的读取调用示例同步切换。前端和历史回放消费公共事件的 `target`。调用 ID 关联、独立持久化的 `ObservationMetadata.Resources` 版本戳及 Runtime 元数据继续保留，不受模型返回精简影响。
- 其他工具保留外层身份、状态、变更结果、决策和反馈等影响后续操作的字段说明。例如 resource、slide_id、status、changed、changed_fields、decision、feedback。
- 渲染诊断、内容预检、用户答案和加载结果用真实字段或必要的点路径集中解释。可以在父字段说明中概括嵌套结构，不把每个数组层、矩形边或样式属性都拆成独立段落。
- 精简不能丢失重要语义：诊断坐标单位、评分范围及局限、可选字段条件、跳过问题不代表批准、拒绝操作未生效等需要保留。
- 图片工具没有 JSON 返回字段时，returns 使用 `{}`，在 function 或 rules 中说明图片内容块的返回形式。
- 成功后无工具回复的提交工具也使用空 returns，在 function 或 rules 中明确消费提交和结束行为，不能写成 JSON null 或伪造 summary 字段。
- JSON 加图片的工具同时说明真实 JSON 字段和独立图片内容块，不能将图片说成 JSON 中的路径或 URL。

## 公共错误协议与业务 Errors

公共错误仍保持既有必填字段 `code`、`category`、`reason`、`measure`。仅 `DEPENDENCY_FAILED` 包含 `dependency_call_ids`，本次不重新设计错误体、失败计数或重试行为。

当前 code 已是字符串。本次统一核对和使用现有字符串值，不能把 HTTP 数字状态混入工具错误 code，也不能为展示文案另造一套错误名称。

- 每个工具说明中的 errors 仅说明该入口实际业务失败条件；业务码可以由多个工具复用，不要求全项目唯一。
- 后端为每个工具／入口明确声明 `businessErrorContracts`，通过 `ToolContract.BusinessErrorCodes` 绑定说明。该集合只用于说明加载与校验，不是运行时错误白名单，不限制 Runtime 实际返回。
- JSON errors 必须与声明的业务集合严格一致；保留未知／未声明 code、漏写业务 code、空描述、重复键、重复声明及无绑定条目的拒绝，不能放宽为任意子集。仅产生通用框架错误的工具使用空 errors，装配时省略 Errors 段落。
- 公共工具错误统一为 RUN_CANCELED、DEPENDENCY_FAILED、RESOURCE_NOT_DISCLOSED、TOOL_ARGUMENT_INVALID、TOOL_CALL_INVALID、IDEMPOTENCY_KEY_REUSED、OUT_OF_SCOPE、PERSISTENCE_FAILED、INTERNAL 九个码，由 `model/common_errors.go` 集中声明。工具业务说明禁止声明这些公共码。
- 调用数量、指定工具、独立执行及普通正文限制统一返回 TOOL_CALL_INVALID；参数问题返回 TOOL_ARGUMENT_INVALID。缺失内部服务、写入会话、配置或命令执行状态异常统一返回 INTERNAL，并停止当前任务。
- PERSISTENCE_FAILED 表示文件变更或执行记录无法安全保存，不承接写入前的内容校验失败；Git 提交失败继续使用工具专属 GIT_COMMIT_FAILED。内部故障及无法确认的保存结果不交给模型盲目重试。
- `retry.md` 维护九个公共码的说明和处理要求，加载器不自动向各工具 description 拼接公共清单，辅助调用也不额外注入公共错误提示词。辅助提交仍通过具体 reason、measure 和字段路径提供纠错反馈。
- 同一业务 code 可以用于多个工具，各工具的说明解释自己的触发条件。此处的一一对应发生在每个工具／入口内部，不表示全项目一个 code 只能由一个工具使用。
- 不把仅用于 HTTP 返回、Provider 请求失败或整轮终止且不会成为该工具回复的错误强行列入该工具 Errors。
- errors 是调用前的说明，不替代运行时针对实际失败生成的 reason、measure 或诊断信息。
- 人工拒绝及正常 approve／revise／refuse 决策放在 Returns，不伪装成错误 code；继续区分调用正常完成与操作生效。
- 不在每个工具中重复公共错误体和重试流程，也不为本次配置加载增加独立错误协议提取逻辑。
- 工具参数及辅助提交字段约束统一使用 TOOL_ARGUMENT_INVALID；工具调用方式约束使用 TOOL_CALL_INVALID。资源读取和执行环境故障使用 INTERNAL，真实文件／索引／执行记录保存失败使用 PERSISTENCE_FAILED，内存暂存失败使用 INTERNAL。
- RESOURCE_CONTENT_INVALID 仅描述已保存的创作源码；运行时截图索引损坏属于 INTERNAL。RENDER_FAILED 仅描述 HTML 或视觉诊断缺陷，源内容变化使用 CONTENT_CONFLICT；渲染 Worker 故障允许一次内部恢复尝试，仍失败后返回 INTERNAL。
- REVIEW_FAILED 仅描述审查材料缺失、不可支持、超限或过期；审查服务、读写和辅助提交故障保留对应公共错误。人工判断仍使用 approve／revise／refuse。
- 全局 manifest／design 缺失可以首次创建，outline 缺失由 OUTLINE_NOT_INITIALIZED 描述，不能为这些入口保留无独立触发条件的 RESOURCE_NOT_FOUND。

当前业务说明集合如下；已按真实执行与转换路径核对，未直接复用旧全量资源编辑或写入集合：

| 工具／入口 | 业务错误 code |
| --- | --- |
| read_resource | CONTENT_TOO_LARGE、OUTLINE_NOT_INITIALIZED、RESOURCE_CONTENT_INVALID、RESOURCE_NOT_FOUND |
| read_image | ATTACHMENT_DIMENSIONS_EXCEEDED、ATTACHMENT_INVALID、ATTACHMENT_NOT_FOUND、ATTACHMENT_TOO_LARGE、RESOURCE_CONTENT_INVALID、RESOURCE_NOT_FOUND |
| render_slide | CONTENT_CONFLICT、OUTLINE_NOT_INITIALIZED、RENDER_FAILED、RESOURCE_CONTENT_INVALID、RESOURCE_NOT_FOUND |
| load_component | CONTENT_TOO_LARGE、RESOURCE_NOT_FOUND |
| load_skill | CONTENT_TOO_LARGE、RESOURCE_NOT_FOUND |
| edit_manifest | CONTENT_CONFLICT、CONTENT_INVALID、CONTENT_TOO_LARGE、RESOURCE_CONTENT_INVALID |
| edit_design | CONTENT_CONFLICT、CONTENT_INVALID、CONTENT_TOO_LARGE、RESOURCE_CONTENT_INVALID |
| edit_spec | CONTENT_CONFLICT、CONTENT_INVALID、CONTENT_TOO_LARGE、RESOURCE_CONTENT_INVALID、RESOURCE_NOT_FOUND |
| edit_html | CONTENT_CONFLICT、CONTENT_INVALID、CONTENT_TOO_LARGE、RESOURCE_CONTENT_INVALID、RESOURCE_NOT_FOUND |
| edit_outline | CONTENT_CONFLICT、CONTENT_INVALID、CONTENT_TOO_LARGE、OUTLINE_NOT_INITIALIZED、RESOURCE_CONTENT_INVALID、TARGET_ALREADY_EXISTS |
| git_commit | GIT_COMMIT_FAILED |
| run_command（真实 shell） | COMMAND_INVALID、COMMAND_NOT_ALLOWED、COMMAND_NOT_AVAILABLE、COMMAND_EXIT_NONZERO、COMMAND_TIMEOUT；其余使用公共错误码 |
| create_plan | PLAN_INVALID |
| update_plan | PLAN_INVALID |
| request_privilege | 无，使用公共错误码 |
| ask_user | 无，使用公共错误码 |
| review_task | REVIEW_FAILED |
| finish_task | COMPLETION_GATE_BLOCK |
| submit_review | 无，使用公共错误码 |
| rename_thread | 无，使用公共错误码 |
| compact_context | 无，使用公共错误码 |
| polish_instruction | 无，使用公共错误码 |
| handoff_thread | 无，使用公共错误码 |
| git_commit.command | 无，使用公共错误码 |

技能加载中 SKILL_NOT_FOUND／SKILL_DISABLED 已转换为 RESOURCE_NOT_FOUND，动态容量失败转换为 CONTENT_TOO_LARGE，选择校验转换为通用 TOOL_ARGUMENT_INVALID；不将转换前的服务错误伪装成模型收到的业务码。辅助提交保留与结果字段、标题、内容、审查决策或重命名动作相关的失败，删除提交数量、工具名称、普通文本及通用提交信封说明。原全量说明集合与 union 拼接没有其他生产用途，已删除；执行实现、真实错误输出、分类、measure、失败计数和重试均不变。

## 同名工具与上下文

不能假定工具名能唯一确定所有入口的说明。当前 `git_commit` 在主 Agent 中返回提交结果，在 `/commit` 辅助提交入口成功后没有模型工具回复。

- 默认说明条目 ID 等于工具名。
- 确有说明差异的入口使用显式说明 ID，例如 `git_commit.command`；每个条目仍保持上述六字段结构。
- 工具入口在代码中绑定说明 ID 及当前输入／输出契约。API 中实际工具名保持 `git_commit`，不发送说明 ID。
- 变体采用独立完整条目，不设计继承、合并覆盖或旧配置回退。只有存在真实语义差异时才建立变体。
- 各入口按自己的输入、输出与错误集合校验，禁止用所有上下文的字段并集掩盖某一入口的错误说明。
- 原有同名上下文差异不得继续以入口中硬编码的说明字符串覆盖 JSON。

## 加载、装配与披露

### 配置加载

- 在现有 prompt 模块范围内增加工具 JSON 的加载与校验能力，不复用 Markdown front matter 解析器硬读 JSON。
- 将 `prompts/tools.json` 明确加入 `backend/embedded_prompts.go` 的生产嵌入范围；更新相关资源目录检查，不能把 JSON 当成未注册 Markdown 或遗漏于生产包。
- 沿用编译嵌入方式，本次不引入热加载、文件监听或文件系统回退。修改说明后通过重新编译生效。
- 配置错误在应用初始化或工具目录构建阶段明确失败，不等待模型调用后才静默失去说明。

### 统一装配

使用同一套装配规则输出：

```text
<function>

Cases:
- <case>

Rules:
- <rule>

Returns:
- <field>: <description>

Errors:
- <code>: <description>
```

- 空分段整体省略，args 不出现在上述文本中。
- cases、rules 保持配置数组顺序；returns、errors 使用固定且稳定的排序，不能依赖 Go map 遍历顺序。
- 输入结构副本注入 args 后，作为 OpenAI 的 parameters 或 Anthropic 的 input_schema 正常发送。
- 装配结果必须使用独立副本，不修改原始注册契约、共享 Schema 或已缓存说明；重复请求不能继续追加文本。
- 主 Agent、Reviewer、上下文压缩和各辅助提交入口均调用共同装配能力，不能绕过它自行拼文案。
- 各模型请求只披露当前可用工具和当前入口正确的一份说明，不把整份 JSON 作为系统提示词重复注入。
- 供应商适配器消费装配完成的工具定义，不再二次投影输出 Schema。
- 上下文预算统计以实际发送的 description 和输入 Schema 为依据，不计入未发送的 OutputSchema，也不能在统计时额外拼一次说明。

## 删除旧实现

- 删除 `ModelToolSchemas` 当前完整 OutputSchema 序列化与 description 拼接路径，以及为该投影服务的废弃辅助代码。
- 不改成另一套 OutputSchema description 提取器；Returns 和 Errors 直接来自 tools.json。
- 清理工具功能、输入和输出说明的旧硬编码来源，包括辅助提交入口的说明覆盖和仅为输出文案创建的辅助函数。
- 输出结构保留为 returns 字段校验依据。已有函数若仍用于构造结构，应去除说明职责后保留；不能因为原文件与文案有关就把结构校验能力一起删除。
- 清理旧说明专用注解、过时注释、调用入口和测试断言；仍参与传输或结构识别的元数据按实际用途保留。
- 将最新 TOOLS.md 的用户修改全部迁移并核对后删除该文件。最终不保留自动生成的第二份 TOOLS.md 或重新生成 Markdown 文案的维护流程。
- 不新增兼容层、旧配置回退或历史开发数据迁移。

## 验收要求

- 所有模型可见工具与上下文变体都有明确的 JSON 说明绑定，不遗漏主 Agent、Reviewer、Git 提交、重命名、压缩、润色或移交入口。
- 所有实际输入说明均来自 args；function／cases／rules／returns／errors 按约定形成工具 description。
- 真实模型请求不包含完整 OutputSchema、旧 Output contract 段落、Args 重复段落、虚假图片字段或成功后的虚假回复。
- 点路径支持对象、数组、本地引用和组合分支；不存在的字段、重复键、未知条目及错误 code 不对应都能明确报错。
- Errors 只包含入口声明的业务失败，空集合不披露 Errors 段落；通用框架错误仍按原公共协议真实返回，不因说明精简而改变运行规则。
- 无论选择 OpenAI 或 Anthropic，都披露同一入口相同的语义说明，保留现有输入 Schema 和实际运行行为。
- 连续请求与上下文预算统计不改变契约或重复拼接；同名工具的不同入口只展示正确的返回约定。
- 完整资源的输出说明已经收敛到完整内容格式，必要决策、状态和诊断解释仍可见。
- `edit_outline.init` 的 review 稿结构补齐已落实到后端，嵌套 args 可以实际匹配；Runtime 管理字段和现有业务限制没有被放宽。
- `TOOLS.md`、旧文案投影、废弃提取及重复硬编码清理完成，JSON 为工具说明的唯一维护入口。
- 保留实施开始时的用户已有修改；本次不修改受保护的 core／mode／playbook 提示词正文，不牵连前端 UI 或改变公共错误协议。
- 遵守 AGENTS.md：独立完成，不使用 SubAgent、Computer Use 或 Browser 执行验证；验证交由用户手动完成。必要时调整与本次变更直接相关的现有校验用例，不为覆盖率增加大量测试。
- 交付时列出改动文件、旧代码清理情况、实际完成的静态检查，以及需要用户手动验证的加载、错误配置、两个供应商请求和上下文差异场景。不把未执行的验证报告为通过。

## 实现与手动验证

实现集中在 prompt 的工具说明加载器与 tooldefinition 完整目录。供应商侧 ToolSchema 仅包含 name、description、parameters，内部输出契约不会进入模型请求。原输出文案投影、NoReplyOutput、旧输入构造和文案覆盖已删除；仅用于合成测试工具的结构辅助函数移入测试文件。

复查已清理资源 Schema 中不再披露的 Agent 示例及其转换、过滤代码，删除没有对应资源标记的只读和 Runtime 字段过滤分支，保留真实字段与引用解析。工具长度统计并入现有 token_estimate.go，主 Agent 与上下文预算直接使用统一工具定义，错误集合去除重复项。HTTP 错误详情白名单移除没有生产写入路径的 current_hash、json_pointer、rejection_code、next_action、required_capability，仅保留实际使用的 component_name；其他协议仍在使用的同名字段不受影响。前端不参与工具说明装配，现有 HTTP／运行事件错误与操作决策协议仍有生产用途，未作为旧工具定义残留删除。

已做源码静态审查与 Go 格式化，未运行构建、测试、模型请求或浏览器验证。按项目约定，由用户手动执行以下验证：

1. 在 backend 目录运行相关包检查：`go test ./schemas ./internal/prompt ./internal/tooldefinition ./internal/llm ./internal/workflow ./internal/commandresult ./internal/contextcompact ./internal/service ./internal/httpapi`。现有断言已调整，并补充了复杂字段路径与配置拒绝用例；此命令尚未执行。
2. 重新编译并启动后端，确认生产包能加载 tools.json。修改 JSON 说明后需要重新编译，现有运行中的二进制不会热加载文件。
3. 分别在 OpenAI 和 Anthropic 的请求记录中检查：description 分段一致，参数说明在对应字段上，无完整输出 Schema 或重复 Args 段落；Errors 只含业务码，ask_user／request_privilege 不含 Errors；重复调用不追加说明，辅助提交不额外收到公共错误清单。
4. 在本地副本中分别加入未知 args／returns 路径、未声明的错误 code、通用框架 code、空错误描述、重复键、漏写业务 code 或缺失条目，重新编译启动，确认初始化报出明确错误。验证后恢复配置；实际调用中的授权、取消、参数校验、依赖、幂等和提交信封错误仍按原协议返回。
5. 对比主 Agent 的 git_commit 和 /commit：前者描述正常提交返回字段，后者实际工具名仍为 git_commit，成功提交后无模型工具回复。确认图片、Reviewer 和辅助提交工具的返回形式也正确。
6. 手动检查 Outline 初始化、资源完整读取、正常拒绝及依赖阻断：init 节点不接受自填 ID；`read_resource` 使用 `type` 选择五类资源并拒绝旧 `resource` 参数，`slide_id` 要求不变，正常回复仅含完整 `content`；未创建的全局资源、有效页面未创建的 Spec／HTML 返回 `{"content":null}` 且不计入工具失败，无效／已删除页面、损坏内容和实际读取失败仍返回公共错误；事件和历史回放保留资源定位，null 不授予源码版本或满足内容依赖，实际读取后的编辑版本检查仍生效；拒绝不计入工具失败，未生效操作不满足依赖。
