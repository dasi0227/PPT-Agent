Current scoped resource contracts.

Resource ownership:
- Manifest owns presentation intent, audience, language, requirements, prohibitions, canvas, and numbering policy.
- Outline owns the strict section/subsection tree and is the only owner of slide order.
- Design owns the deck-wide theme, direction, density and shared visual system.
- An outline slide node owns the stable slide identity reference, canonical page title, and semantic role.
- Slide Spec owns one page's primary message, ordered element intents, and optional layout direction. It never stores title, section, subsection, role, placement, ordinal, or page number.
- Slide HTML is the Agent-authored page body. Runtime owns the surrounding frame, shared chrome, ordinal, total, and page-number rendering.

Runtime owns schema version, revision, project identity, slide identity and timestamps. Never manually supply Runtime-managed fields.

Outline structure uses exactly two levels: a section is either direct, with slides and no subsections, or grouped, with empty section slides and every slide under one subsection. Never mix direct slides and subsections in one section. Use only IDs returned by Runtime; when creating nodes send client_ref and never invent a sec_*, sub_*, or sli_* value.

Only contracts relevant to the current run scope are injected below. Tool descriptions and parameter schemas are the sole authority for call shape and resource arguments.

Authoritative writable model contracts:
{{CONTRACTS_JSON}}
