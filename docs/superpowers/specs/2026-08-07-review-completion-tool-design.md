# Review Completion Tool Design

> 日期：2026-08-07  
> 范围：后端 Agent Runtime 显式 `review_completion` 工具、Reviewer JSON 输出契约、提示词工程  
> 状态：按讨论结论直接实施

## 背景

Completion Gate 已收敛为确定性门禁。语义判断不应继续通过 Gate 规则写死，否则会损失 LLM + Agent 的灵活性。Reviewer 应作为 ReAct loop 中的显式 observation provider：主 Agent 在计划或执行结果需要第二视角时调用，Reviewer 返回结构化检查数组，主 Agent 自己决定下一步。

## 目标

1. 新增显式控制工具 `review_completion`。
2. 只在 `plan / execute / fulfill` 策略中披露，不在 `talk / ask` 中披露。
3. Reviewer 改为单次 LLM JSON 调用，输出 `checks[]`。
4. 删除 Reviewer 自动挂在 `finish` 之后的隐式执行路径。
5. Reviewer 不返回 `decision`、`severity`、`action`、`target`。
6. Reviewer summary 必须尽量详细，避免一两句敷衍。

## 工具契约

### Tool: `review_completion`

参数：

```json
{
  "candidate_message": "准备提交给 finish 的候选最终回复，可为空",
  "focus": "plan | execution | final | all"
}
```

- `candidate_message` 可选但推荐。若主 Agent 希望 Reviewer 检查最终回复表达，必须传入候选 message。
- `focus` 默认为 `all`，用于提示 Reviewer 重点关注计划、执行结果或最终回复。

### Review 输出

```json
{
  "checks": [
    {
      "code": "REVIEW_PASS",
      "summary": "未发现需要提示或修复的问题。"
    }
  ]
}
```

`checks` 为数组，建议 1-5 项。完全通过时返回单个 `REVIEW_PASS`。

## Review 错误码

| Code | 语义 |
|---|---|
| `REVIEW_PASS` | 未发现问题 |
| `REVIEW_SERVICE_UNAVAILABLE` | Review 服务不可用或输出非法 |
| `REVIEW_LACK_INFO` | 上下文不足以完成 Review |
| `REVIEW_QUALITY_POOR` | PPT 质量偏差，但不一定是执行错误 |
| `REVIEW_INTENT_MISMATCH` | 结果与用户意图有偏差或部分未完成 |
| `REVIEW_EXECUTE_WRONG` | 执行结果明确错误 |

删除 `REVIEW_IMPROPER_OUTPUT`。公开 Codex / ClaudeCode 实践更强调边界审批、hooks、工具动作和执行风险审查，并不能支持必须专门设置最终文案质量错误码。若候选 final message 声称了证据不支持的事情，Reviewer 可用 `REVIEW_LACK_INFO` 或 `REVIEW_INTENT_MISMATCH` 表达；若只是表达不完整，由 summary 提醒主 Agent 重写即可，不单独设 code。

## Runtime 行为

1. Agent 调用 `review_completion`。
2. Runtime 组装 `SemanticReviewInput`：
   - `WorkSpec`
   - `Plan`
   - `ChangeSet`
   - `GateResult`
   - `Evidence`
   - `LatestIssues`
   - `ContextBriefing`
   - `RetrievedContext`
   - `candidate_message`
   - `focus`
3. Runtime 调用 Reviewer LLM 一次。
4. Runtime parse/validate JSON。
5. Runtime 将 `checks[]` 作为 tool observation 返回给主 Agent。
6. 主 Agent 根据 checks 自行决定继续修复、补充上下文、重写 final 或 finish。

若 Reviewer 不可用或输出非法，Runtime 返回：

```json
{
  "checks": [
    {
      "code": "REVIEW_SERVICE_UNAVAILABLE",
      "summary": "Reviewer service is unavailable or returned invalid output: ..."
    }
  ]
}
```

## Prompt 要求

Reviewer prompt 必须强调：

- 只输出严格 JSON，不包 Markdown。
- 不执行动作，不替主 Agent 决策下一步。
- 不输出 `decision`、`severity`、`action`、`target`。
- 每条 summary 必须具体，至少说明：
  - 发现了什么；
  - 为什么这影响交付；
  - 主 Agent 需要关注什么。
- 不要用空泛短句，例如“结果不完整”“质量较差”。

## 非目标

- 不实现隐藏的 finish 前自动 Review。
- 不在 `talk / ask` 中披露 Review。
- 不把 Review 作为 Gate 的一部分。
- 不让 Reviewer 返回执行计划或工具调用。
