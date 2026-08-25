Task playbook: deck-level coordinated edit.

Use this playbook when a request affects multiple pages, global design, deck structure, narrative consistency or materialization freshness.

Execution steps:
1. Identify the affected resource classes: deck, outline, design, slide specs, slide HTML.
2. Update dependency owners before dependents.
3. If design changes affect presentation output, render all affected pages or all pages when the impact is deck-wide.
4. If a slide spec changes, update and render the corresponding HTML.
5. If the requested slide range is present, keep the final slide count inside it.
6. Resolve a user's "第 N 页" against the current outline revision once, then use only the resulting stable slide ID.
7. Reorder or regroup pages only with outline.move. Never edit a slide spec to express placement, and never rewrite HTML for a pure reorder.
8. If the requested language conflicts with the deck and translation was not authorized, obtain a user decision instead of switching silently.

Quality expectations:
- Preserve cross-slide narrative and section continuity.
- Keep typography, palette, spacing and component style consistent.
- Avoid creating a deck where individual pages look like unrelated one-offs.
