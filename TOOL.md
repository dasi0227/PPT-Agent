# 工具反馈、命令提交与成果审查

本文说明实际模型可见的内容预检反馈与命令提交契约；定义由代码构造，经 `llm.ModelToolSchemas` 写入 Responses / Anthropic 工具描述。本文不会自动注入模型。

## JEV 内容预检反馈

`edit_html` 和实际修改单页 HTML 的 `run_command` 在成功持久化后，可在原工具回复中附带 `content_precheck` 对象。批量评分使用各页最终内容，结果附到该页最后一次成功修改的调用；无实际内容变化或同批较早的修改不附评分。命令工具保留原有 stdout、stderr、exit_code。

```json
{
  "summary": "HTML 已保存。",
  "content_precheck": {
    "status": "completed",
    "scores": {
      "content_coverage": 2.0,
      "expression_clarity": 3.0,
      "requirement_adherence": 2.0
    }
  }
}
```

三个数值分别评价当前页面的必需内容覆盖、主旨与表达清晰度、对有效用户要求和限制的遵循程度。范围为 0–3，允许小数，越高越好。完整档位来自 `model.ContentPrecheckRubrics`，同时用于 JEV 问题的 criteria 和工具 OutputSchema 的字段 description，并真正发送给主 Agent；工具回复不重复档位。

`completed` 表示三个评分可用，不代表合格或任务完成。`unavailable` 表示无法得到可用评分，`skipped` 表示未执行，`stale` 表示材料变化使结果失效；后三者只有 status 与 reason，没有 scores。reason 是预检状态的原因码，不是页面具体缺陷。pending 仅用于内部持久化，不作为最终模型回复。

原工具调用标识页面，回复不重复 slide_id、max_score、confidence、概率分布、档位或内部哈希。后台记录和前端事件保留完整数据；独立失效通知携带 slide_id 与 tool_call_id，明确作废的是哪次调用的评分。

JEV 只接收当前 HTML 的完整文本投影与相关背景，不接收修改前或 Run 起点的内容。保留历史内容需要 Reviewer 的基线与差异证据。预检辅助 Agent 判断是否值得修订；低分或执行失败不改变保存成功，分数也不证明视觉质量或事实真伪。

## 共同执行规则

compact、commit、rename、polish、handoff、review 都只披露各自指定的提交工具。正常合法提交立即结束，无成功后的工具确认请求；被拒绝的输出不触发业务效果。

- 整个命令最多追加两次请求。输出协议纠正与明确不支持工具选择／并行限制后的重发共用额度；compact 的必要输入分批共享同一额度。
- 原始输入、assistant 全部内容、tool calls 和适用 continuation 保持原样；每个被拒绝的 call ID 都先收到失败结果，然后才追加有 runtime 来源的具体指导。
- 失败结果为 `{code, field, reason, next_action}`；没有提交值或伪造成功消息。Anthropic 将其序列化为 `is_error: true` 的 `tool_result`；Responses 保留原始 reasoning / phase 与调用对应关系。
- 缺失工具、多调用、错误名称、非法字段及格式可纠正；缺失／重复 call ID、复用已配对 call ID、不能规范化的 provider 响应、取消、超时、过期结果或业务失败直接终止。普通正文不提取为提交结果。
- 所有生成及纠正共享原截止时间、累计输出预算和上下文窗口；无 usage 时用统一估算计费，不在纠正轮重置。必要输入不静默裁剪。
- 支持时指定唯一提交工具并关闭并行调用。未知能力先尝试约束；只有机器错误明确指向不支持的字段才撤掉对应约束。能力判定按实际模型、协议和端点在任务内生效；不改主 Agent 的工具策略。
- 网络重试由既有 HTTP 层负责，与输出纠正单独诊断，共享截止时间。

| 命令 | 提交工具 | 总截止时间 | 累计输出 token 预算 |
| --- | --- | --- | --- |
| compact | compact_context | 45 秒 | 4000（含必要分批） |
| commit | git_commit | 提交信息生成 45 秒 | 1024 |
| rename | rename_thread | 20 秒 | 128 |
| polish | polish_instruction | 12 秒 | 1024 |
| handoff | handoff_thread | 简报生成 45 秒 | 6000 |
| review | submit_review | 证据准备与评审共 5 分钟 | 4096 |

## 提交参数

| 工具 | 参数用途和约束 | 接受与应用 |
| --- | --- | --- |
| compact_context | `title`：任务／阶段标题，1–48 字单行纯文本；`content`：继续原任务所需的完整摘要，按五节约定组织，保留约束、决策、证据和未完成工作；只允许这两个字符串字段 | 必要分批保留先前合法摘要；全命令成功后替换上下文一次，失败保留原始上下文 |
| polish_instruction | `title`：说明表达改善，1–48 字单行纯文本；`content`：完整润色草稿，1–8000 字，保持原意、范围和反馈；只允许这两个字段 | 返回最终合法建议，不执行草稿，也不修改主 Run |
| handoff_thread | `title`：交接任务／阶段标题，1–48 字单行纯文本；`content`：完整独立 Markdown 简报，1–40000 字，保留目标、已确认决定、实际进度、证据限制、剩余工作及下一步；修订返回完整替换内容，遵循原始上下文及全部反馈；只允许这两个字符串字段 | 最终合法结果通过取消检查后保存一个简报版本；纠正不保存版本，不创建会话或启动 Agent，新建会话仍由用户按钮触发 |
| git_commit | `title`：提交标题，1–72 字单行文本，按提交提示词使用类型及中文概述；`items`：1–6 项变动说明，每项 1–160 字；只允许这两个字段 | `/commit` 的模型仅生成信息；校验通过后在业务层执行 Git 一次并保存 intent／回执。主 Agent 的同名工具继续使用原有执行路径和业务结果 |
| rename_thread | `action`：`rename` 或 `keep`；`title`：仅 rename 必填，1–60 字合法会话名，keep 禁止携带；禁止其他字段或普通正文 | 仅最终合法结果进入现有取消、operation version、project generation 与自动命名状态检查，写回一次；keep 是合法成功 |
| submit_review | `decision`：`approve / revise / refuse`；`reasons`：每种结论都必填的非空纯文本字符串数组，说明页面、证据和影响；禁止额外字段 | 唯一工具调用必须是 submit_review；沿用可伴随正文的原规则，正文不构成结论；有效提交且证据仍一致后才接受，执行失败无结论 |

## Reviewer 证据

后端在模型调用前准备完整材料，Reviewer 不再获得 read_resource、read_image、render_slide。

材料包含未受压缩影响的用户原始需求、后续纠正与问答、主 Agent 的 demand、任务范围、当前 manifest / outline / design / spec、全部当前文本源码及二进制哈希、Run 起点以来累计净 diff、当前范围的任务局部 diff、页面目录、所有现存 HTML 页面对应的有效截图，以及相关上传参考图。

图片作为真实图像块发送，明确对应页面或 attachment_id；读取后冻结字节。缺失／过期／损坏截图通过现有后端渲染器补齐，最多 128 次证据渲染，不修改创作权限。渲染产生有效图像与缺陷诊断时，诊断作为证据；读取或渲染执行失败直接失败。

快照保留 evidence_version、源码和渲染依赖哈希。证据准备后、每轮调用前后与接受前校验当前源码、主题／框架依赖及持久化用户要求；变化即失效。读取失败、缺失必要图像、128 MiB 源码／图像容量或模型上下文超限明确失败，不裁剪必要证据，也不默认 approve。approve / revise / refuse 和共享质量标准保持原义。

## 诊断

同一 submission_id 关联命令身份或 review 的 run/call ID；记录 Generate 次数、实际模型 HTTP 请求与底层重试数、协议纠正原因／字段、剩余额度、输出预算、模型及实际工具策略、耗时和终止原因。

HTTP 阶段记录 started、headers_received、body_received 或 transport_failed 及耗时，用于区分本地准备、传输／服务器等待、响应读取；这些时间不能单独证明模型推理慢。model_requests / http_requests 包含实际发送的底层重试，generate_requests 单独计数。命名另记录总体耗时和终止阶段；评审记录准备阶段、证据数量／哈希和一致性失败，结论日志只记枚举与 reasons 数量。日志不保存用户材料、模型正文、工具参数、私有推理或网关 URL。真实模型与界面由用户手动验收。
