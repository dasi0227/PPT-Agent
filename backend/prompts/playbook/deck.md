Task playbook: deck structure and coordinated change.

Apply this guidance when the requested work involves narrative structure, new pages or shared resources. Permission to edit a resource does not itself create a reason to change it.

- Give each promised page a distinct narrative job and identify what supports its message before expanding the outline. Allocate space to evidence and decisions, not equal-length sections by default.
- When init_outline is disclosed, submit complete initial JSON source with new node IDs omitted. When arrange_outline is disclosed, read the saved source and apply exact replacements. Observe generated IDs in the returned source before dependent page writes. File existence, not section count, controls initialization.
- A representative content-heavy page can establish hierarchy and chart language before extending the deck. This is useful when it prevents repeated layout mistakes, not a mandatory approval checkpoint or all-specs-first sequence.
- Shared design changes update reference requirements for the whole deck. Use html_reference_changes and the requested outcome to decide which HTML pages need edits; reference changes alone do not require synchronized rewrites. Check any necessary page edits against the current editable set and preserve unaffected content.
- Pure reordering/regrouping uses arrange_outline while preserving page IDs; Runtime derives page numbering, so do not rewrite HTML page numbers. For reference changes, use current requirements and html_reference_changes to decide whether the task requires HTML edits; reference changes alone never require rewriting a page.

Examples: “Unify charts on pages 3, 5 and 8” targets those pages, not a whole-deck rewrite. “Add two pages after this section” inserts into existing structure and uses the returned identities. “Move the last page before the conclusion” changes the outline without manually renumbering slides.
