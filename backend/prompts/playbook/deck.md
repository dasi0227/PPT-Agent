Task playbook: deck structure and coordinated change.

Apply this guidance when the requested work involves narrative structure, new pages or shared resources. Permission to edit a resource does not itself create a reason to change it.

- Give each promised page a distinct narrative job and identify what supports its message before expanding the outline. Allocate space to evidence and decisions, not equal-length sections by default.
- With no sections, use outline.init; with existing structure, including empty sections, use outline.insert. Observe returned client_ref-to-ID mappings and hashes before dependent page writes. Never reinitialize an existing outline.
- A representative content-heavy page can establish hierarchy and chart language before extending the deck. This is useful when it prevents repeated layout mistakes, not a mandatory approval checkpoint or all-specs-first sequence.
- Shared design changes update reference requirements for the whole deck. Use html_reference_changes and the requested outcome to decide which HTML pages need edits; reference changes alone do not require synchronized rewrites. Check any necessary page edits against the current editable set and preserve unaffected content.
- Pure reordering/regrouping uses outline.move; Runtime derives page numbering, so do not rewrite HTML page numbers. For reference changes, use current requirements and html_reference_changes to decide whether the task requires HTML edits; reference changes alone never require rewriting a page.

Examples: “Unify charts on pages 3, 5 and 8” targets those pages, not a whole-deck rewrite. “Add two pages after this section” inserts into existing structure and uses the returned identities. “Move the last page before the conclusion” changes the outline without manually renumbering slides.
