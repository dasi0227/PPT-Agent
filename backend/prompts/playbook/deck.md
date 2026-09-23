Task playbook: deck structure and coordinated change.

Apply this guidance when the requested work involves narrative structure, new pages or shared resources. Permission to edit a resource does not itself create a reason to change it.

- Give each promised page a distinct narrative job and identify what supports its message before expanding the outline. Allocate space to evidence and decisions, not equal-length sections by default.
- With no sections, use outline.init; with existing structure, including empty sections, use outline.insert. Observe returned client_ref-to-ID mappings and hashes before dependent page writes. Never reinitialize an existing outline.
- A representative content-heavy page can establish hierarchy and chart language before extending the deck. This is useful when it prevents repeated layout mistakes, not a mandatory approval checkpoint or all-specs-first sequence.
- Shared design changes affect the whole deck and require synchronized page outputs and current evidence. Check the affected pages against the current editable set before choosing that approach; a local request usually needs a local change. Preserve unaffected content and avoid no-op shared writes.
- Pure reordering/regrouping uses outline.move; Runtime derives page numbering, so do not rewrite HTML page numbers. For title, role or content changes, use returned invalidated_slide_ids and current content to identify necessary synchronization.

Examples: “Unify charts on pages 3, 5 and 8” targets those pages, not a whole-deck rewrite. “Add two pages after this section” inserts into existing structure and uses the returned identities. “Move the last page before the conclusion” changes the outline without manually renumbering slides.
