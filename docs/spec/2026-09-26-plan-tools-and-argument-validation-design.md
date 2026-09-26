# 计划工具职责与统一参数校验

日期：2026-09-26

依据：本次用户确认——创建与更新计划分开，按当前计划状态暴露工具；所有工具共用明确的参数错误反馈，并允许将数组字段中的 JSON 字符串转换为数组。

## 1. 计划工具

| 状态 | 暴露 | 参数 |
| --- | --- | --- |
| Plan 模式，没有计划 | `create_plan` | `title`、`content`、`steps`；提交后等待用户审批 |
| Execute 模式，没有计划 | `create_plan` | `title`、`content`、`steps`；作为可选执行清单直接进入 active |
| Plan 模式，已有待审批草案且正在修订 | `update_plan` | 完整 `title`、`content`、`steps`；Runtime 更新审批身份 |
| Execute 模式，已有 active 计划 | `update_plan` | `updates`；只更新已有步骤状态 |
| 当前计划 completed / canceled | 不暴露上述两个工具 | 不通过新建计划覆盖本轮计划；新任务按自身状态决定 |

模型每轮只看到适用的单一参数 Schema，不再通过 `oneOf` 同时暴露创建与进度更新。执行入口重新校验当前状态，隐藏工具不能通过直接调用覆盖已有计划。计划审批、范围授权和完成检查规则保持有效。

`create_plan` 的必填参数为 `title: string`、`content: string`（Markdown 正文）、`steps: array`（至少一项）。每个步骤必填 `title: string`，可选 `target_slide_ids: string[]`，只能指定现有且获授权的页面；步骤 ID 与初始状态由 Runtime 生成。

草案修订的 `update_plan` 使用上述完整参数；执行中的 `update_plan` 只接收必填 `updates: array`（至少一项），每项必填 `step_id: string` 和 `status: pending | in_progress | completed | failed`。步骤 ID 必须存在且不能重复，至多一个步骤处于 in_progress，已完成步骤不能回退。

计划未完成时，完成检查返回未完成步骤 ID、状态和 `update_plan` 下一步指引；不替模型把未完成步骤自动标为完成。

## 2. 参数校验与数组兜底

控制工具与普通工具共用 Schema 校验和数组规范化入口，在业务执行与副作用之前处理参数。普通工具在预检和幂等请求指纹计算前完成规范化。原始参数不被部分修改；只有整个参数对象通过校验，才交给业务处理。

兜底限定如下：

- 仅当字段 Schema 要求数组时，尝试把字符串作为 JSON 数组解析；覆盖嵌套数组字段。
- 解析后的数组仍须通过元素类型、必填字段、枚举、数量、重复项等完整校验。
- 不接受 null、对象、畸形 JSON、重复键、双重编码或代码围栏；不猜测或修复内容。
- 不转换字符串字段或对象字段。Outline `content`、HTML、Markdown 和文本替换锚点原样保留。
- 合法的原生数组仍是模型应使用的合同；成功兜底记录 `tool.arguments_normalized`，含工具、调用 ID、字段路径和转换类型。

无法满足 Schema 的请求返回 `TOOL_ARGUMENT_INVALID`，分类为 `agent_repairable`，携带字段路径、具体原因、修正指引；类型错误另含 expected / actual。`PLAN_INVALID` 同样注册为可由 Agent 修正的错误。两者不触发无修改的自动重试；模型应修正后重新调用。

统一入口涵盖计划、提问、权限申请、批量资源加载、JSON 编辑及 HTML / Outline 文本修改等工具。此兜底不改变持久化结构，也不增加旧工具协议兼容入口。

## 3. 验证

针对性测试覆盖字符串数组与嵌套数组转换、非法转换拒绝、源码字符串保留、普通工具执行前校验、计划工具状态切换，以及“参数错误 → 明确反馈 → 字符串数组兜底 → 更新完成 → 正常结束”的完整 Runtime 流程。浏览器与真实模型交互由用户手动验收。

## 4. 工具错误反馈审计与修复

用户追加要求：检查截图中的目录读取／创建失败，并统一排查其他工具的错误提示。

### 日志证据

后端日志中 run `fd6f5adf-38a1-4133-9dbf-94c43376931e` 的开头三次失败为：

| 时间 | 工具 | 原因 |
| --- | --- | --- |
| 18:58:57 | read_resource(outline) | 新项目尚未创建 Outline |
| 18:59:14 | init_outline | 页面节点含不允许的 id、purpose |
| 18:59:18 | init_outline | 删除 purpose 后仍保留了不允许的 id |

18:59:25 的下一次初始化改为仅提供页面 title 后成功。这三次失败源于资源生命周期和 Outline 字段错误，并非数组被编码为字符串，也不是持久化丢失。

### 统一修复

- 错误绑定曾覆盖参数校验的 field / expected / actual 与自定义 next_action。现在普通工具、控制工具、保存和重放共用错误构造逻辑，保留结构化诊断；结果数据不能覆盖已注册的错误码、分类、调用身份和重试策略。
- Outline 缺失返回 OUTLINE_NOT_INITIALIZED，明确提示仅在工具开放且需要创建时初始化。已存在的目录不能重复初始化；保存内容损坏返回 RESOURCE_CONTENT_INVALID，不把读取异常当作文件缺失。
- init_outline 和 arrange_outline 明确区分章节、小节、页面字段，提供最小初始化示例。新页面只含 title，已保存页面只含 slide_id 和 title；页面不允许 id 或 purpose。提示词同步此规则，并避免在未初始化时先读取目录。
- 保留底层校验错误链，返回 JSON Pointer 和具体校验问题。源码节点检查返回明确路径；文本替换区分 EDIT_ANCHOR_NOT_FOUND / EDIT_ANCHOR_AMBIGUOUS，并提供 edits 下标与匹配数量。
- HTML / Spec 缺失、渲染图片过期、组件或技能缺失／禁用／超出加载预算分别提供创建、刷新或缩小批次的指引。资源参数说明切换到已确认的扁平 resource 协议，render_slide 与其他工具统一稳定 slide_id 格式。
- 补全命令错误码及读取／写入错误码。命令语法、路径、参数、退出码、超时和输出限额错误给出对应修正方法，保留 stdout / stderr / exit_code；用户拒绝使用单独错误码，不能引导重试或绕过拒绝。运行不变量和真实 I/O 故障继续与可修正输入错误区分。
- 完成检查不再覆盖已有具体 next_action；语言、页数、阶段等错误补充对应说明。Manifest 读取异常的检查指向 Manifest。扩大范围的引导统一使用 request_privilege，而非更新计划。
- 控制工具拒绝写入 control.rejected 日志，保留调用 ID、错误码及诊断。模型返回的调用身份／原始 JSON 无法解码时仍按协议边界终止，但明确返回 MODEL_TOOL_CALL_INVALID，不笼统描述成 Agent 异常。

### 验证结果

- 已用截图对应的两种非法 Outline 输入复现，校验反馈包含字段位置和新页面规则，失败不留下目录文件，改正后初始化成功。
- 普通工具错误经过真实批量执行与结果序列化／重放后，字段类型详情仍完整；命令错误保留退出码和错误输出。
- 参数、计划、资源、渲染、命令权限、组件／技能、完成检查和幂等性相关针对性回归通过。
- spec、pptmutation、commandexec 包测试通过，后端全部包及测试文件编译检查通过（不等于全量测试通过）。
- 扩展 model 包测试中的 TestPublicTextUsesPageContextAndPreservesTechnicalContent 失败；使用改动前 agent_error.go 的 overlay 复核仍然失败，属于既有文案断言问题。本次错误分类／投影测试通过。
- 未重启用户服务、未修改实际项目资源，真实模型与界面交互由用户手动验收。
