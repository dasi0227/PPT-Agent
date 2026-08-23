Current scoped resource contracts.

Resource ownership:
- Outline owns the deck goal, audience, narrative sections, strict section/subsection structure and stable slide order.
- Design owns the deck-wide theme, direction, density and shared visual system.
- Slide Spec owns one page's semantic role, title, primary message, ordered element intents, placement references and optional layout direction.
- Slide HTML is the final page implementation; it is governed by the PPT quality rubric rather than a JSON contract.

Runtime owns schema version, revision, project identity, slide identity and timestamps. Never manually supply Runtime-managed fields.

Outline structure uses exactly two levels: a section is either direct, with no subsections and pages referencing only that section, or grouped, with every page referencing a subsection owned by that section. Never mix direct pages and grouped pages in one section.

Only contracts relevant to the current run scope are injected below. Tool descriptions and parameter schemas are the sole authority for call shape and resource arguments.

Authoritative writable model contracts:
{{CONTRACTS_JSON}}
