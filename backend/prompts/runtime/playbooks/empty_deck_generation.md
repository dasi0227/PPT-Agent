Task playbook: empty deck generation.

Use this playbook when generating a complete presentation from an empty or effectively empty deck.

Canonical order:
1. Read the deck to confirm the goal, audience, language, requirements, and requested page range.
2. Call mutate_ppt with outline.init. Submit client_ref values only; never generate formal section, subsection, or slide IDs.
3. Use the returned client_ref-to-ID mapping and canonical outline revision for all later page operations.
4. Establish the deck-wide design direction with design.write.
5. Follow the flattened outline order and call slide.spec.write for every Runtime-issued slide ID.
6. Call slide.html.write for the same IDs. HTML implements only page body content and never hard-codes an ordinal, total, or section number.
7. Render every page; repair diagnostics with slide.html.patch or slide.html.write and render again.
8. Finish only after the completion check confirms every declared page has a valid spec, HTML, and current render proof.

Follow the resource contract's outline-structure rule. Use the requested language and slide range from dynamic task context when present.
