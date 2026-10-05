---
id: playbook.deck
description: Guides outline planning, page creation and coordinated changes across presentation resources. Execution tasks involving narrative structure, new pages or shared presentation resources.
---

Task playbook: deck structure and coordinated change.

Permission to edit a resource does not itself create a reason to change it.

- Give each promised page a distinct narrative job and identify what supports its message before expanding the outline. Allocate space to evidence and decisions, not equal-length sections by default.
- When creating or resizing a deck, plan against Manifest.pages and compare the resulting Outline count with that requirement. A range leaves room for content-driven allocation; an approximate count remains approximate. Changing the stored requirement does not create or delete pages. Never overwrite the requested count merely to match the existing outline.
- When outline editing is available and task/outline_exists is false, submit complete initial JSON source to edit_outline with new node IDs omitted. For an existing outline, read the saved source and use edit_outline to apply exact replacements. Observe generated IDs in the returned source before dependent page writes. File existence, not section count, controls initialization.
- A representative content-heavy page can establish hierarchy and chart language before extending the deck. This is useful when it prevents repeated layout mistakes, not a mandatory approval checkpoint or all-specs-first sequence.
- Store content requirements in Manifest, shared visual requirements in Design and a page-specific arrangement in its Spec. Preserve actionable details when separating a mixed request. A local page edit or wording cleanup does not justify replacing the deck's visual requirements; preserve established requirements outside the requested change.
- Changing Manifest or Design updates shared authoring guidance, not existing HTML bodies. Use the requested outcome and actual reference differences to identify affected pages, update the necessary Spec or HTML within the current editable set, and preserve unaffected content. If the outcome needs additional pages outside that set, resolve scope before editing them.
- Runtime derives shared decorations from current resources, so placement or source-text changes may appear without an HTML edit. Distinguish these shared effects from body content that must be updated. Pure reordering or regrouping preserves page identities and HTML filenames; inspect titles or positional references in page content only when the requested result requires synchronization.

Examples: “Unify charts on pages 3, 5 and 8” targets those pages, not a whole-deck rewrite. “Add two pages after this section” inserts into existing structure and uses the returned identities. “Move the last page before the conclusion” changes the outline without manually renumbering slides.
