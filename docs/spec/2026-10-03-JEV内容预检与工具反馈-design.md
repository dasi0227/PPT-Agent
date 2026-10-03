# JEV 内容预检与工具反馈设计

日期：2026-10-03。状态：代码已实施，定向测试与类型检查通过；真实模型行为由用户手动验收。

## 1. 目标

JEV 评价当前页面内容，主 Agent 在原编辑工具回复中收到精简的结构化评分。评价方与读分方使用同一份档位定义，避免把预检执行状态、内容质量和 HTML 保存结果混为一谈。

本次改动覆盖内容预检输入、共享评分定义、工具回复与输出契约、失效通知、持久化恢复及前端评分维度类型。工作区已有的审查与命令提交改动继续保留。

## 2. 预检输入与评分

- 共享背景保留有效用户要求、manifest、outline、design；原始用户要求及后续纠正优先于可编辑资源。
- 页面材料保留 slide_id、页面 spec、当前 html_content 及内容投影说明；移除 before_content、run_start_content 和对应状态。
- html_content 沿用当前 HTML 的完整文本投影：保留文本与块边界，排除脚本、样式和显式隐藏节点；不截断超限材料，也不据此声称完成 CSS 可见性、图片或视觉检查。
- 预检不读取 Reviewer 基线，不判断相对旧内容的保留情况；Reviewer 继续使用独立持久化基线与实际差异。
- 批次起点只记录 HTML 哈希，用于排除无实际净变化的页面；Checkpoint 和评分记录不再保存修改前正文。

`model.ContentPrecheckRubrics` 是唯一档位定义，版本为 content-v2。JEV criteria 和模型可见 OutputSchema 的字段 description 都从它生成。

| 字段 | 评价对象 | 0 分 | 1 分 | 2 分 | 3 分 |
| --- | --- | --- | --- | --- | --- |
| content_coverage | 本页核心信息和必要材料 | 内容缺失或偏题 | 缺核心信息或关键材料 | 核心及多数材料齐全，仍有遗漏 | 完整承担本页内容职责 |
| expression_clarity | 主旨、组织、措辞 | 无法理解主旨 | 结构或措辞混乱 | 主旨清楚，少量歧义或冗余 | 主旨、组织和措辞清晰 |
| requirement_adherence | 对有效用户要求与限制的遵循 | 违背关键要求 | 明显偏离 | 核心符合，轻微偏离 | 符合有效要求与限制 |

分数为概率加权期望值，允许 0–3 范围内的小数；不能把小数直接视为精确档位或具体缺陷。评分不提供页面专属理由，不能凭数值生成未经证实的遗漏、位置或修改指令。

## 3. 评分与编辑调用的关联

只在 HTML 成功持久化后评估。收集同批实际变化的页面，去重并评价最终版本；同页多次修改的结果附到最后一次成功修改。多个页面和维度可以合并请求，超过既有容量时拆分，继续共享原批次预算。

当前 edit_html 每次对应一页；run_command 的写入受限于单文件 sed -i，每次 HTML 修改同样对应一页。模型可见 content_precheck 使用单个对象，页面身份由原 ToolCallID 及调用参数确定。

评分完成后重新生成编辑工具已缓存的 Observation，确保普通执行及从持久化回执恢复的路径都能将评分带入下一次模型请求。命令工具的 stdout、stderr、exit_code 保持原值。恢复只消费成功回执，不重放未完成的编辑。

## 4. 模型可见工具回复

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

| status | 含义 | 模型回复中的评分 |
| --- | --- | --- |
| completed | 三个评分可用，任何分值均可 | 返回三个数值 |
| unavailable | 超时、无效返回、材料或记录不可用等 | 无 scores，返回 reason |
| skipped | 未执行，例如 JEV 未启用 | 无 scores，返回 reason |
| stale | 页面、规格或相关要求变化，评价失效 | 无 scores，返回 reason |

pending 只在内部表示未完成；最终投影将不完整评价视为 unavailable。reason 是状态原因码，例如 timeout、invalid_answer、disabled、material_changed，不是质量缺陷。无变化、删除 HTML 或同批较早的编辑不附预检结果。

edit_html 和 run_command 的实际 OutputSchema 包含该可选对象。completed 分支要求完整的三个 number 字段；其他分支要求 status 与 reason，并禁止 scores。每个评分字段说明评价对象、完整档位和小数含义，经 ModelToolSchemas 注入实际 provider 工具描述。系统质量提示词只说明如何使用结果，不重复维护档位。

## 5. 后台记录、事件和失效

后台保留 assessment_id、slide_id、内容／材料哈希、rubric、模型、完整分数、max_score、confidence、legend、probabilities 等数据；前端事件仍携带完整评分记录，并一次性采用三个新维度名。没有旧维度兼容映射、数据迁移或双写。

单页工具回复省略这些后台元数据。独立 runtime 失效通知使用精简对象，额外保留 slide_id 和 tool_call_id，指出应忽略哪次调用的评分。它不重复发送工具终结事件，也不将已保存的编辑变成失败。

HTML、spec、背景要求或 rubric 变化使旧评价失效；无效版本不复用。评分文件和 Checkpoint 的既有持久化与恢复规则继续生效。

## 6. 验证

定向用例覆盖当前最终内容与历史输入移除、同页最后调用归属、多页合并评分与逐调用分配、命令缓存回复刷新、后台完整分布保留、评分不可用不改变写入成功、独立失效通知及回执恢复不重放编辑。工具输出契约检查确认两个相关工具实际披露三个维度及共享档位。

已通过 workflow 相关定向测试、Responses／Anthropic 输出契约边界测试、前端 SSE 的 20 项测试及 TypeScript 检查。契约边界测试使用进程内 HTTP transport，保留实际请求序列化检查并避免监听端口；SSE 既有 diff 用例补齐必填 suggested_next_inputs，以验证其原本目标。默认 Go 缓存受沙箱限制时使用本地临时缓存。

未运行全量测试或浏览器验证。真实模型的修订选择及页面状态由用户手动验收。
