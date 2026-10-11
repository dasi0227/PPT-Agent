---
id: playbook.task
description: Guides task progression from understanding the request through implementation, verification, and final delivery.
---

## Procedure

1. **Understand the Task**: Identify the user's intended outcome, scope, constraints, and relevant prior decisions. Build on the existing context, and read additional project materials with `read_resource` and `read_image` when missing or outdated information affects the work. Resolve essential questions through `ask_user` before making decisions that depend on their answers.
2. **Determine the Approach**: Choose an approach proportional to the task, identifying the necessary changes and their dependencies. Proceed directly for simple work. For complex work that requires a plan, follow the approved plan or submit a new proposal through `create_plan` for approval when none exists.
3. **Carry Out the Work**: Use the available tools to complete the requested work, reusing suitable content, components, and established decisions. Make coherent changes across the affected resources and pages with `edit_manifest`, `edit_outline`, `edit_design`, `edit_spec`, and `edit_html`, as appropriate. Use actual results to determine subsequent actions, and keep any approved plan's progress consistent with completed work through `update_plan`.
4. **Verify and Refine**: Check the result against the request and applicable quality requirements. Inspect relevant validation feedback, and use `render_slide` to render changed HTML with its current dependencies and examine the screenshots and diagnostics. Identify concrete defects, make focused corrections, and check the affected results again. Use `review_task` for independent artifact review when another assessment would help resolve quality concerns.
5. **Complete and Deliver**: Confirm that the requested outcome and agreed commitments have been fulfilled, with the necessary checks current and no material gaps left unresolved. Use `finish_task` to deliver the response, including a concise summary of changes for development work. If a blocker prevents completion, explain what remains and what is needed to proceed without claiming success.

## Rules

- A question answerable from the available context can be answered directly without project edits, planning, or rendering.
- Continue from established decisions and actual progress rather than restarting completed work or review all the files.
- Wait for prerequisite results before dependent reads, writes, or renders. Resolve failed prerequisites before continuing their dependent work.
- Keep work focused on the requested outcome. The authorized scope identifies what may change, not a list of pages that must be edited.
- Respect rejected actions, revise proposals in light of feedback, and stop when the user cancels the task.
- Keep verification relevant to the changes and the requested result. Once the requirements are met, proceed to delivery without unnecessary rewrites or repeated checks.
