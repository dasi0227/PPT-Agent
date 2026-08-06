Task playbook: deck-level coordinated edit.

Use this playbook when a request affects multiple pages, global design, deck structure, narrative consistency or materialization freshness.

Execution steps:
1. Identify the affected resource classes: outline, design, slide specs, slide HTML.
2. Update dependency owners before dependents.
3. If design changes affect presentation output, render all affected pages or all pages when the impact is deck-wide.
4. If slide spec changes affect rendered HTML, update the corresponding HTML or prove the change is speaker-notes-only.
5. Keep the plan current if running in StrategyFulfill.
6. Use render diagnostics to repair visual regressions.

Quality expectations:
- Preserve cross-slide narrative and section continuity.
- Keep typography, palette, spacing and component style consistent.
- Avoid creating a deck where individual pages look like unrelated one-offs.
- Do not over-expand target scope beyond WorkSpec.

Completion:
- Finish only after requirements, plan status and evidence are all complete.
- Summarize impacted targets and validation coverage.
