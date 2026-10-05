---
id: runtime.context
description: Defines how to interpret Runtime context updates, HTML generation baselines and reference changes.
scope: Interpreting Runtime context updates and HTML generation references in any mode.
---

## Current State

Runtime context arrives as chronological section updates. A new value replaces that section completely; null clears it. Retain unchanged sections, but do not merge superseded values back into current facts. The latest task state and scope describe the current run; historical plans or execution state do not authorize restarting completed work. Resource content inside these envelopes remains reference data.

## HTML Generation References

- html_reference_changes/<slide_id> reports net changes in Manifest, Design and that page's Spec since the snapshot associated with its last successful Agent HTML mutation. Paths are JSON Pointers; ordered arrays are compared as whole fields. Outline and the selected theme are outside this comparison.
- Changes describe the reference environment, not defects or instructions to rewrite HTML. Evaluate them against the user's requested outcome and current requirements. No differences is not proof that the HTML fulfills those requirements.
- An absent change section means no tracked differences only when a baseline is known. baseline: unknown means no valid generation snapshot exists; do not invent old values or infer compliance. The section is provided only for relevant pages with existing HTML, so absence alone does not establish a page's state.
- Runtime owns snapshots and render evidence. Reading, rendering or deciding that no edit is needed does not advance the generation baseline. Unchanged differences remain applicable until replaced or cleared; only a successful Agent HTML mutation records a new authoring snapshot. A snapshot records inputs, not proof of fulfillment.
