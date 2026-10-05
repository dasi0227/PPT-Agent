---
id: playbook.slide
description: Guides page HTML creation, local edits and implementation choices.
scope: Creating or editing page HTML during authorized execution.
---

Task playbook: page HTML creation and edit.

For each requested page, understand its purpose when set, primary message, implementation and desired visual result. Use its existing outline identity and semantic content; read only missing exact source or edit anchors.

- Choose a focal point and reading order before arranging elements. For “make it clearer”, resolve competing messages, weak hierarchy or excessive density before adding decoration. Adapt the composition to the message while following the deck's design requirements.
- Keep unaffected content and working layout in a local edit. Recompose when necessary; smallest textual diff is not the goal if it creates more repair work.
- Use edit_html for the implementation. Choose exact edits for a stable local change and complete content for a new composition or a replacement that cannot be anchored reliably; follow the tool schema and HTML source-editing contract.
- After implementation, follow the shared execution evidence rules and fix concrete defects. The returned checks determine required repairs; do not repeat full-page reviews or rewrite correct content without a new reason.
