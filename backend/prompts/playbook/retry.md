---
id: playbook.retry
description: Guides diagnosis, correction, and retry decisions after tool failures while preserving completed work.
---

## Procedure

1. **Inspect the Failure**: Read the failure reason, affected target, and suggested corrective action. Determine whether the operation ran, whether any changes were saved, and whether dependent calls were skipped.
2. **Choose the Response**: Identify the cause from the feedback and current state. Decide whether to correct the arguments or content, resolve a prerequisite, obtain necessary input or authorization, or stop the affected operation.
3. **Correct and Retry**: Address the identified cause, then resubmit only the affected operation with arguments based on the current state.
4. **Verify and Continue**: Inspect the new tool result and resume from the failure state. If the operation still fails, reassess the cause using the new feedback before choosing another action.
5. **Stop and Report**: If an essential step cannot be completed and no viable alternative exists, stop execution immediately and report the blocker to the user. Do not repeat futile attempts, continue as if the step succeeded, or claim completion.

## Cases「IMPORTANT」

- **Invalid Arguments or Content**: Correct the reported fields according to the tool schema or resource requirements.
- **Failed Prerequisites**: Resolve the failed prerequisite, then regenerate dependent calls from its successful result.
- **Stale Source or Invalid Edit Target**: Use `read_resource` to refresh the source, locate the target again, and regenerate the edit.
- **Missing Resources**: Verify the target and use `edit_manifest`, `edit_outline`, `edit_design`, `edit_spec`, or `edit_html`, as appropriate, to create missing resources only when required by the task and authorized.
- **Render Failures**: Inspect the diagnostics and available screenshots from `render_slide`, correct any identified source defects, and render again.
- **Rejected Completion**: Resolve the missing work or checks reported by `finish_task` before submitting it again.
- **Missing Authorization**: For required page edits outside the current scope, use `request_privilege` and wait for authorization.
- **Rejected Proposals or Actions**: Revise the approach according to the user's decision and feedback, and obtain any required approval before proceeding.

## Error Protocol「IMPORTANT」

Tool execution or validation failure will provide four fields to describe the situation.

- `code`: The machine-readable error code.
- `category`: One of the six failure categories below.
	- `transient`: A temporary failure; Runtime controls automatic retries. Follow `measure` before deciding whether another attempt is permitted.
	- `repairable`: Correct the arguments, content or failed prerequisites before trying again.
	- `interrupted`: Human intervention is required to provide necessary input, authorization or configuration before the operation can continue.
	- `conflict`: Refresh the relevant current state and regenerate the operation before trying again.
	- `canceled`: The operation was canceled; respect the current stop condition.
	- `terminal`: Repeating the call cannot resolve the failure; follow the reported remedy or report the blocker.
- `reason`: The concrete cause of failure. A relevant input or source field path may be included naturally in this explanation.
- `measure`: The suggested remedy. Apply it only within the tools, authorization, behavior and budgets of the current task.

These codes describe shared failure types.

- `RUN_CANCELED`: The run was canceled before the operation completed.
- `DEPENDENCY_FAILED`: Required predecessor operations did not take effect, so the dependent call was not executed.
- `RESOURCE_NOT_DISCLOSED`: The tool or resource is not disclosed in the current turn.
- `TOOL_ARGUMENT_INVALID`: The call arguments do not match the disclosed input schema.
- `INVALID_CONTROL_CALL`: The control action is invalid in the current state, or a call that must execute alone was combined with other calls.
- `IDEMPOTENCY_KEY_REUSED`: The call ID was already used for a different tool or different arguments.
- `CAPABILITY_DENIED`: The current mode, capability or risk policy denies the operation.
- `TARGET_OUT_OF_SCOPE`: The target is outside the effective authorized scope.
- `RUN_SESSION_MISSING`: A write operation has no active Runtime write session.
- `COMMIT_FAILED`: Runtime could not safely persist the changes; the saved outcome may be uncertain.
- `COMMAND_INVARIANT_VIOLATION`: The command's Runtime preflight, approval or execution guard is inconsistent.
- `AGENT_FAILED`: An Agent or Runtime service failure prevents continuation.
- `INTERNAL`: An internal Runtime or persistence failure prevents the operation from completing.
- `TOOL_COUNT`: A tool submission does not contain exactly one required tool call.
- `TOOL_NAME`: A submission calls a tool other than the required submission tool.
- `HAS_TEXT`: A tool-only submission also includes ordinary assistant text.
- `INVALID_SUBMISSION`: A tool submission is invalid without a more specific submission error.

## Rules

- Diagnose failures from the call arguments, `category`, `reason`, `measure` and returned diagnostics before choosing a correction.
- Correct the cause before retrying only the affected operation. Refresh stale or uncertain state before repeating a write.
- Retry unchanged arguments only for a temporary failure when another attempt is permitted. Runtime manages automatic retries.
- For `DEPENDENCY_FAILED`, inspect every reply in `dependency_call_ids` and regenerate the skipped call from current state only after its required predecessors take effect.
- Respect user decisions and revise the approach using `feedback`. Do not repeat denied actions unchanged or treat feedback as authorization.
- Keep repairs and retries within available tools, current authorization and Runtime limits. Do not bypass refusals, cancellations or stop conditions.
- Use `ask_user` for blocking missing facts or decisions, and `request_privilege` for required page authorization. Wait for the required input or approval before proceeding.
- Preserve completed work without counting refused, failed or skipped operations as completed. If an essential step has no viable remedy, stop and report the blocker without claiming success.
