---
id: core.error
description: Defines shared tool failure fields, categories and dependency diagnostics.
---

## Error Protocol

Failed tool calls and rejected submissions return JSON text with four required fields:

- `code`: The machine-readable error code.
- `category`: One of the six failure categories below.
- `reason`: The concrete cause of failure. A relevant input or source field path may be included naturally in this explanation.
- `measure`: The suggested remedy. Apply it only within the tools, authorization, behavior and budgets of the current task.

Failure categories:

- `transient`: A temporary failure; Runtime controls automatic retries. Follow `measure` before deciding whether another attempt is permitted.
- `agent_repairable`: Correct the arguments, content or failed prerequisites before trying again.
- `user_action_required`: Necessary input, authorization or configuration requires user intervention.
- `conflict`: Refresh the relevant current state and regenerate the operation before trying again.
- `canceled`: The operation was canceled; respect the current stop condition.
- `terminal`: Repeating the call cannot resolve the failure; follow the reported remedy or report the blocker.

The tool reply's outer association identifies the current call. The error body does not repeat its call ID, operation or resource metadata. Retry eligibility remains a Runtime decision.

Only `DEPENDENCY_FAILED` includes `dependency_call_ids`, a string array listing every unsuccessful required predecessor that blocks this call. Successful predecessors and calls that only need to finish are excluded. The dependent call was not executed. Inspect the listed replies, resolve all blockers and regenerate the dependent call from successful results and current state.

Relevant tool-specific diagnostics may accompany the four fields, such as captured command output, validation findings, completion blockers or render diagnostics. These details explain the failure and do not turn it into a normal business result. Optional `feedback` contains verbatim user feedback and does not authorize another attempt or broader scope. A render failure may retain its screenshot image block alongside the error text; inspect both before making a correction. Business outcomes such as a valid refusal or revision decision remain normal tool results.
