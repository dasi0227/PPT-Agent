Task playbook: coordinated multi-page or global edit.

Identify the actual requested pages and resource owners, then choose the smallest coherent change that satisfies the request across them.

- Resolve page numbers against the current outline once and keep the stable IDs. Use the active scope as the write boundary, not as a mandate to edit all pages.
- Update dependencies before their implementations. When a semantic spec changes and HTML is authorized, synchronize that page's HTML and render it. A spec-only task ends with valid specs and does not require HTML writes or render proof.
- Global design writes currently require synchronized HTML and fresh render proof across the deck. Inspect this impact before changing design; use page-local styling for a local request. Do not perform a no-op global write for bookkeeping.
- Pure reordering/regrouping uses outline.move and does not require HTML rewrites for page numbers. Runtime derives the frame. For actual title/role/content changes, inspect returned invalidated_slide_ids and current content to decide which page bodies need synchronization.
- Preserve narrative continuity, palette, typography and spacing across affected pages, while adapting layout to each message. Meet explicit language and page-count constraints without silently broadening the task.

Examples:
- “Unify the charts on pages 3, 5 and 8”: read these pages and useful neighboring references, change only the intended authorized pages, and render each changed HTML. Do not rewrite the whole deck just because it is readable.
- “Move the last page before the conclusion”: use the current stable identities and outline.move under global permission. Do not renumber HTML.
- “Continue the remaining pages”: inspect work_ledger and current outputs, continue unfinished work and repair stale dependencies; do not regenerate completed pages without a reason.
