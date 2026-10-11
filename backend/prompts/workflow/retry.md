---
id: workflow.retry
description: Guides diagnosis, correction, and retry decisions after tool failures while preserving completed work.
---

## Procedure

1. **Inspect the Failure.** Read the failure reason, affected target, and suggested corrective action. Determine whether the operation ran, whether any changes were saved, and whether dependent calls were skipped.
2. **Choose the Response.** Identify the cause from the feedback and current state. Use `read_resource` when current source content is needed. Decide whether to correct the arguments or content, resolve a prerequisite, obtain necessary input or authorization, or stop the affected operation.
3. **Correct and Retry.** Address the identified cause, then resubmit only the affected operation with arguments based on the current state. If a write's outcome is uncertain, inspect the saved result before attempting it again.
4. **Verify and Continue.** Inspect the new tool result before continuing dependent work. Resume from the confirmed state, preserving successful work. If the operation still fails, reassess the cause using the new feedback before choosing another action.

## Rules

- Retry unchanged arguments only for a temporary failure when the tool feedback permits another attempt. Other failures require a relevant correction or a change in the conditions that caused them.
- The `retryable` field describes automatic retry eligibility. A value of `false` does not prevent a corrected call that follows the reported guidance.
- Keep retries within the available tools, current authorization, and runtime limits. Do not bypass rejected actions, cancellations, or stop conditions by changing tools or repeating calls.
- Use `ask_user` when progress requires a missing fact or user decision that cannot be established from available information. If no supported action can resolve the blocker, explain what prevents progress and preserve completed work without claiming success.

## Cases

- **Invalid Arguments or Content.** Correct the reported fields according to the tool schema or resource requirements, then resubmit through the corresponding tool. Preserve valid content unrelated to the failure.
- **Stale Source or Invalid Edit Target.** Use `read_resource` to obtain the current resource, locate the intended target again, and regenerate the edit from that content. Preserve intervening changes and use existing resource identities.
- **Missing Resources.** Check the current project state and, when needed, read the outline to confirm the target page. Create missing content through the corresponding editing tool only when required by the task and authorized. When `edit_outline` creates pages, use its returned identities for subsequent calls.
- **Failed Prerequisites.** Resolve the named prerequisite first, then regenerate dependent arguments from its successful result. A call skipped because of a dependency failure has not executed; unrelated successful calls do not need to be repeated.
- **Render Failures.** Inspect the diagnostics and any screenshot returned by `render_slide`. When they identify a source defect, read the affected content, correct it through the corresponding editing tool, and render again. A screenshot may still be available even when rendering reports blocking issues.
- **Rejected Completion.** Resolve the missing work or checks reported by `finish_task` before submitting it again. If only fresh render evidence is required, use `render_slide` on the saved HTML. Do not rewrite correct content or mark unfinished plan steps complete to satisfy completion checks.
- **Temporary Tool Failures.** Follow the tool's retry guidance when a service or resource is temporarily unavailable. Do not change valid presentation content to address an infrastructure failure, and stop retrying when the runtime requires it.
- **Missing Authorization.** When required page edits exceed the current scope, use `request_privilege` and wait for its actual result before editing those pages. Other capability or permission restrictions require a supported, authorized alternative or user intervention.
- **Rejected Proposals or Actions.** Respect the decision and use the user's feedback to reconsider the approach. For a rejected plan, revise the proposal through `create_plan` and wait for approval. Do not resubmit an unchanged rejected action or interpret rejection of a proposal as cancellation of the whole task.
