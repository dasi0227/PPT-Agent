---
id: playbook.tool
description: Defines tool result statuses and guides failure diagnosis, correction, and retry decisions while preserving completed work.
---

## Procedure

1. **Inspect the Failure**: Read the failure reason, affected target, and suggested corrective action. Determine whether the operation ran, whether any changes were saved, and whether dependent calls were skipped.
2. **Choose the Response**: Identify the cause from the feedback and current state. Decide whether to correct the arguments or content, resolve a prerequisite, obtain necessary input or authorization, or stop the affected operation.
3. **Correct and Retry**: Address the identified cause, then resubmit only the affected operation with arguments based on the current state.
4. **Verify and Continue**: Inspect the new tool result and resume from the failure state. If the operation still fails, reassess the cause using the new feedback before choosing another action.
5. **Stop and Report**: If an essential step cannot be completed and no viable alternative exists, stop execution immediately and report the blocker to the user. Do not repeat futile attempts, continue as if the step succeeded, or claim completion.

## Cases「IMPORTANT」

- **Invalid Arguments or Content**: Correct invalid arguments according to the tool schema. For rejected proposed content, fix the reported resource constraints before resubmitting. For invalid saved source, repair the identified resource within available tools and authorization; report the blocker when no safe repair is available.
- **Failed Prerequisites**: Resolve the failed prerequisite, then regenerate dependent calls from its successful result.
- **Stale Source or Invalid Edit Target**: Use `read_resource` to refresh the source, locate the target again, and regenerate the edit.
- **Missing Resources**: Identify the exact missing target from `reason`. For PPT sources, use the appropriate edit tool only when creation is needed and authorized. For missing components or skills, choose an existing repository entry or restore its body; for attachments, select an existing original or obtain an upload. For missing screenshots of existing slides, render the slide. Creating PPT content does not repair missing repository entries or attachments.
- **Render Failures**: Inspect the diagnostics and available screenshots from `render_slide`, correct any identified source defects, and render again.
- **Rejected Completion**: Resolve the missing work or checks reported by the runtime completion check before replying with the final answer again.
- **Missing Authorization**: For required slide edits outside the current scope, use `request_privilege` and wait for authorization.
- **Rejected Proposals or Actions**: Revise the approach according to the user's decision and feedback, and obtain any required approval before proceeding.

## Tool Result Protocol「IMPORTANT」

### Result Status

Every tool reply includes a JSON text block with one shared `status`.
- `success`: The operation has been authorized and completed successfully.
- `refuse`: The user declined the proposed operation or authorization request.
- `error`: Validation, execution or persistence failed.

### Error Structure

An `error` result includes these four fields and any relevant diagnostics.
- `code`: The readable error code that reflects a certain error.
- `category`: Error types that guides the direction of remedy.
	- `repairable`: The failure can be resolved by repairing or retrying.
	- `canceled`: The operation was compulsorily canceled by runtime.
	- `terminal`: The failure cannot be resolved by the current effective means.
- `reason`: The concrete cause of failure.
- `measure`: The suggested remedy or the next step suggestions.

### Shared Error Codes

These codes describe shared failure types and tool-specific errors describe domain failures.
- `INTERNAL`: An internal service or failure prevents safe continuation.
- `RUN_CANCELED`: The run was canceled before the operation completed.
- `NOT_FOUND`: The requested target or required resource does not exist.
- `TOOL_ARGUMENT_INVALID`: The call arguments do not match the input schema.
- `TOOL_CALL_INVALID`: The call format violates the usage rules.
- `OUT_OF_SCOPE`: The target is outside the authorized scope.
- `DEPENDENCY_FAILED`: Required predecessor operations did not take effect.
- `RESOURCE_NOT_DISCLOSED`: The tool or resource is not disclosed in the current turn.
- `IDEMPOTENCY_KEY_REUSED`: The call ID was already used by a different request.
- `EDIT_AMBIGUOUS`: `old_text` matches more than once after earlier replacements.
- `EDIT_NOT_FOUND`: `old_text` has no match after earlier replacements.
- `CONTENT_INVALID`: Content violates syntax, structure or business constraints.
- `CONTENT_CONFLICT`: The current content being used does not match the latest content.
- `OUTLINE_NOT_EXIST`: The project lacks an outline and cannot obtain the slide ID.
- `PERSISTENCE_FAILED`: Runtime could not safely save file changes or execution records.

## Rules

- Diagnose failures from the call arguments, `category`, `reason`, `measure` and returned diagnostics before choosing a correction.
- Correct the cause before retrying only the affected operation. Refresh stale or uncertain state before repeating a write.
- Runtime alone controls automatic retries. Do not restart an exhausted retry budget by repeating the same call; follow the final failure's `category` and `measure`.
- Respect user decisions and revise the approach using `feedback`. Do not repeat denied actions unchanged or treat feedback as authorization.
- Keep repairs and retries within available tools, current authorization and Runtime limits. Do not bypass refusals, cancellations or stop conditions.
- Preserve completed work without counting refused, failed or skipped operations as completed. If an essential step has no viable remedy, stop and report the blocker without claiming success.
