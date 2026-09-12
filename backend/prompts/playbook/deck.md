Task playbook: deck creation and coordinated change.

Use current structure and resources to choose the next action. A task may add pages, change existing ones and preserve completed ones in the same deck. Identify the requested outcome and actual affected resource owners; scope limits writes but does not prescribe a page checklist.

Structure and new pages:
- Creating structure or changing deck-wide resources requires global permission. With no sections, outline.init accepts a section/subsection/page tree using client_ref. With existing structure, including empty sections, use outline.insert. Never reinitialize an existing outline.
- Observe returned client_ref-to-ID mappings and revisions before page writes. Runtime creates formal identities. Reuse the current design direction unless the task calls for a change.
- Each new page needs its outline identity and semantic spec before rendering dependent HTML. Every declared page needs a valid spec; a complete-presentation request also needs HTML and current render evidence for every promised page.

Existing content and dependencies:
- Resolve page numbers through the current outline and use stable IDs. Preserve unaffected content and the chosen visual direction while adapting each layout to its message.
- Synchronize changed specs into HTML when the task includes HTML. Spec-only work ends with valid semantic changes.
- Global design writes currently require synchronized HTML and fresh render proof across the deck. Understand that impact before writing design; page-local styling is appropriate for local requests. Avoid no-op global writes.
- Pure reordering/regrouping uses outline.move; Runtime derives numbering, so no HTML renumbering is needed. For actual title/role/content changes, inspect returned invalidated_slide_ids and current content to determine necessary synchronization.

Scheduling and continuation:
- Complete pages individually or in coherent batches according to dependencies. There is no all-specs-first requirement or obligation to author in outline order. An early representative page can test the visual direction when useful.
- Continue unfinished work from work_ledger and current resources. Revisit completed pages for changed requirements, invalid dependencies or observed defects, not simply because they are authorized.

Examples:
- “Unify charts on pages 3, 5 and 8”: consult useful references, modify the intended authorized pages and render changed HTML; do not broaden to a whole-deck rewrite.
- “Add two pages after this section, keeping the rest”: insert into existing structure, use the returned IDs and develop the new pages while preserving existing outputs.
- “Move the last page before the conclusion”: move stable outline identities under global permission without rewriting HTML page numbers.
