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
- **Missing Resources**: Identify the exact missing target from `reason`. For PPT sources, use the appropriate edit tool only when creation is needed and authorized. For missing components or skills, choose an existing repository entry or restore its body; for attachments, select an existing original or obtain an upload. For missing screenshots of existing pages, render the page. Creating PPT content does not repair missing repository entries or attachments.
- **Render Failures**: Inspect the diagnostics and available screenshots from `render_slide`, correct any identified source defects, and render again.
- **Rejected Completion**: Resolve the missing work or checks reported by `finish_task` before submitting it again.
- **Missing Authorization**: For required page edits outside the current scope, use `request_privilege` and wait for authorization.
- **Rejected Proposals or Actions**: Revise the approach according to the user's decision and feedback, and obtain any required approval before proceeding.

## Tool Result Protocol「IMPORTANT」

Every tool reply includes a JSON text block with one shared `status`:

- `success`: The operation completed normally, including a valid no-op or an already loaded resource. It does not guarantee changed content or a passing `review_ppt` assessment.
- `refuse`: The user declined the proposed operation or authorization request. Do not treat it as granted authorization, an applied operation or an execution error to retry automatically or unconditionally. Existing drafts or interaction state may remain, including an unapproved `create_plan` draft.
- `error`: Validation, execution or persistence failed. Handle the reported error as described below.

Business fields remain alongside `status`; only `review_ppt` independently retains a tool-result `decision` (`approve`, `revise` or `refuse`) and `reasons`. A completed review returns `success` for every valid assessment; inability to complete it returns `error`. User `feedback` guides the next approach but grants no authorization. Image content remains in separate blocks, and submission tools that end without a reply on success retain that behavior. Loading an already active skill or component returns `success`, with its body in the corresponding `active_*` context.

An `error` result includes these four fields and any relevant diagnostics:

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

These thirteen codes describe shared failure types; tool-specific errors describe domain failures.

- `RUN_CANCELED`: The run was canceled before the operation completed.
- `DEPENDENCY_FAILED`: Required predecessor operations did not take effect, so the dependent call was not executed.
- `RESOURCE_NOT_DISCLOSED`: The tool or resource is not disclosed in the current turn.
- `TOOL_ARGUMENT_INVALID`: The call arguments do not match the disclosed input schema.
- `TOOL_CALL_INVALID`: The call format violates tool selection, call count, execution order, isolated-call requirements or restrictions on ordinary assistant text.
- `IDEMPOTENCY_KEY_REUSED`: The call ID was already used for a different tool or different arguments.
- `TARGET_OUT_OF_SCOPE`: The target is outside the effective authorized scope.
- `NOT_FOUND`: The requested target or required resource does not exist. Resolve the specific missing entry, body file, page, source or screenshot before retrying. If the outline exists but has no matching page (including an empty outline), read that outline, confirm a current page ID or create the needed page within authorization; never reinitialize the existing outline. Directly reading a missing outline also uses this code.
- `CONTENT_CONFLICT`: The current content version has not been confirmed, or content, relevant dependencies or an approval baseline changed. Refresh the authoritative state identified by `reason` and `measure`, then regenerate the edit, command, approval or render. Do not reuse stale approval or screenshot evidence; the correct refresh is not always an outline read.
- `CONTENT_INVALID`: Proposed content or saved content required by the operation violates syntax, structure or business constraints. Use `reason`, `measure` and diagnostics to identify the resource and defect. Rejected proposed content was not saved; correct it before resubmitting. Invalid saved content requires a safe, authorized repair; if none is available, report the blocker and request help. Do not treat invalid content as missing, reinitialize it, or repeatedly read unchanged content.
- `OUTLINE_NOT_EXIST`: A page operation or an edit requiring an existing outline cannot locate valid page IDs because the effective workspace has no outline source. Create it with `edit_outline(content: <complete JSON source string>)` only when disclosed, authorized and needed, using `id: "new"` for every node, then use its returned page IDs. Do not invent IDs or blindly reread missing resources. Invalid outline content and an absent page within an existing outline are different failures.
- `PERSISTENCE_FAILED`: Runtime could not safely save file changes or execution records; the saved outcome may be uncertain.
- `INTERNAL`: An internal service, configuration or execution-state failure prevents safe continuation.

## Rules

- Diagnose failures from the call arguments, `category`, `reason`, `measure` and returned diagnostics before choosing a correction.
- Correct the cause before retrying only the affected operation. Refresh stale or uncertain state before repeating a write.
- Retry unchanged arguments only for a temporary failure when another attempt is permitted. Runtime manages automatic retries.
- For `NOT_FOUND` and `OUTLINE_NOT_EXIST`, repair the reported prerequisite before resubmitting; Runtime does not automatically retry these failures. For `CONTENT_CONFLICT`, refresh the relevant current state before regenerating the affected operation.
- For `CONTENT_INVALID`, fix the rejected proposal or invalid saved source identified by the feedback before retrying; Runtime does not automatically retry this failure. Existing source damage does not authorize replacing or reinitializing the resource.
- For `DEPENDENCY_FAILED`, inspect every reply in `dependency_call_ids` and regenerate the skipped call from current state only after its required predecessors take effect.
- For `INTERNAL`, stop and report the failure; another model call cannot repair Runtime services or execution state.
- For `PERSISTENCE_FAILED`, stop and verify saved state and the operation receipt before any new write. Do not blindly repeat an operation whose saved outcome is uncertain.
- Respect user decisions and revise the approach using `feedback`. Do not repeat denied actions unchanged or treat feedback as authorization.
- Keep repairs and retries within available tools, current authorization and Runtime limits. Do not bypass refusals, cancellations or stop conditions.
- Use `ask_user` for blocking missing facts or decisions, and `request_privilege` for required page authorization. Wait for the required input or approval before proceeding.
- Preserve completed work without counting refused, failed or skipped operations as completed. If an essential step has no viable remedy, stop and report the blocker without claiming success.
