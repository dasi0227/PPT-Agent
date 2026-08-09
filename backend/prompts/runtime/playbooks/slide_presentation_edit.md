Task playbook: single-slide presentation edit.

Use this playbook when RunCommand.scope is ppt/slide. The goal is to update the final user-visible slide implementation while preserving the surrounding deck system.

Execution steps:
1. Identify the stable slide_id from RunCommand. Never use "current" as a resource id.
2. Read slide spec and slide HTML when exact current content, anchors or design intent matter.
3. Use edit_ppt for small exact replacements with unique anchors.
4. Use write_ppt for broad layout reconstruction, large visual changes or when exact anchors are not reliable.
5. Keep HTML inside a 1600x900 .slide-stage and link ../../common/tokens.css plus ../../common/base.css.
6. Render the affected slide after the latest HTML or design-affecting change.
7. Repair overflow, clipping, console errors, failed resources or font problems, then render again.

Completion:
- Finish only after static and visual evidence are fresh.
- Summarize what changed and what render checks passed.
- Mention remaining risks only if they are user-visible and actionable.
