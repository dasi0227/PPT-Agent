Task playbook: single-slide HTML creation and edit.

Understand the page's role, primary message, current implementation and requested visual change. Read only the missing exact content needed to make the edit safely.

- Preserve the surrounding deck's visual direction and unaffected content. An HTML-only visual change need not rewrite the semantic spec; a changed message may require spec permission before synchronizing it.
- Use slide.html.patch when old_text comes from current HTML and is unique. Patches are exact string replacements, not selectors or JSON Patch. If the target is ambiguous, read a larger anchor or rewrite the page coherently.
- A new composition or a fragile existing layout can justify slide.html.write; smallest textual diff is not the goal when it creates more repair work.
- For a DOM selection, connect the user's annotation to the current element and related layout. Do not assume an old snapshot still matches.
- Render the changed page; request visual_review when judging composition, typography, image matching or an ambiguous clipping warning. Repair identified issues and render the latest version before finishing.

For a new page, use its Runtime-issued outline identity and existing semantic spec before implementing dependent HTML. If required semantic content is missing, obtain the necessary spec permission rather than inventing persistent fields in HTML-only scope.
