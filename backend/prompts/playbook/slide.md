Task playbook: page HTML creation and edit.

For each requested page, understand its role when set, primary message, implementation and desired visual result. Use its existing outline identity and semantic content; read only missing exact source or edit anchors.

- Choose a focal point and reading order before arranging elements. For “make it clearer”, resolve competing messages, weak hierarchy or excessive density before adding decoration. Adapt the composition to the message while preserving the deck's visual direction.
- Keep unaffected content and working layout in a local edit. Recompose when necessary; smallest textual diff is not the goal if it creates more repair work.
- Use slide.html.patch only when old_text comes from current HTML and is unique. It is exact string replacement, not a selector or JSON Patch. Read a larger anchor or use slide.html.write for an ambiguous target, new composition or fragile layout.
- For a DOM selection, connect the annotation to the current element and surrounding layout; do not assume an old snapshot still matches.
- After implementation, follow the shared execution evidence rules and fix concrete defects. The returned checks determine required repairs; do not repeat full-page reviews or rewrite correct content without a new reason.
