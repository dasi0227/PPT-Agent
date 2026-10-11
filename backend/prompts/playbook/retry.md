---
id: playbook.retry
description: Guides diagnosis, correction, and retry decisions after tool failures while preserving completed work.
---

## Procedure

1. **Inspect the Failure.** Read the failure reason, affected target, and suggested corrective action. Determine whether the operation ran, whether any changes were saved, and whether dependent calls were skipped.
2. **Choose the Response.** Identify the cause from the feedback and current state. Decide whether to correct the arguments or content, resolve a prerequisite, obtain necessary input or authorization, or stop the affected operation.
3. **Correct and Retry.** Address the identified cause, then resubmit only the affected operation with arguments based on the current state.
4. **Verify and Continue.** Inspect the new tool result and resume from the failure state. If the operation still fails, reassess the cause using the new feedback before choosing another action.
5. **Stop and Report.** If an essential step cannot be completed and no viable alternative exists, stop execution immediately and report the blocker to the user. Do not repeat futile attempts, continue as if the step succeeded, or claim completion.

## Rules

- Retry unchanged arguments only for a temporary failure when the tool feedback permits another attempt.
- Keep retries within the available tools, current authorization, and runtime limits. Do not bypass rejected actions, cancellations, or stop conditions by changing tools or repeating calls.
- Use `ask_user` when progress requires a missing fact or user decision that cannot be established from available information.
- If no supported action can resolve the blocker, explain what prevents progress and preserve completed work without claiming success.

## Cases

- **Invalid Arguments or Content** → Correct the reported fields according to the tool schema or resource requirements.
- **Failed Prerequisites** → Resolve the failed prerequisite, then regenerate dependent calls from its successful result.
- **Stale Source or Invalid Edit Target** → Use `read_resource` to refresh the source, locate the target again, and regenerate the edit.
- **Missing Resources** → Verify the target and use `edit_manifest`, `edit_outline`, `edit_design`, `edit_spec`, or `edit_html`, as appropriate, to create missing resources only when required by the task and authorized.
- **Render Failures** → Inspect the diagnostics and available screenshots from `render_slide`, correct any identified source defects, and render again.
- **Rejected Completion** → Resolve the missing work or checks reported by `finish_task` before submitting it again.
- **Missing Authorization** → For required page edits outside the current scope, use `request_privilege` and wait for authorization.
- **Rejected Proposals or Actions** → Revise the approach according to the user's decision and feedback, and obtain any required approval before proceeding.
