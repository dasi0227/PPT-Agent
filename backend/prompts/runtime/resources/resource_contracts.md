Current scoped resource contracts.

Resource ownership:
- Manifest owns presentation intent, audience, language, requirements, prohibitions, 16:9 canvas policy and numbering behavior.
- Outline owns the strict section/subsection tree, stable node references, canonical page title, semantic role and the only slide order.
- Design owns direction, density and shared chrome. The user-selected theme is read-only to the Agent, including design.theme; even global scope does not grant a theme replacement operation.
- Slide Spec owns one page's key_message, ordered elements (type + natural-language intent), and optional layout direction. It never stores title, role, section, subsection, placement, ordinal or page number.
- Slide HTML is the Agent-authored page body. Runtime owns the frame, equal-ratio fitting, shared chrome and derived numbering. Materialization records and render proof are Runtime-managed, never author-written.

Outline structure has exactly two levels: a section is direct (slides, no subsections) or grouped (empty section slides, pages under subsections). Never mix the two. For new nodes submit client_ref, then use returned IDs; never invent formal sec_*, sub_* or sli_* identities.

Use canonical revisions returned by reads/mutations for expected_revision when concurrency protection matters. On a revision conflict, read current state and recompute the intended edit. Do not replay an obsolete patch. Runtime owns schema version, revision, identity and timestamps; never copy those fields into writable payloads.

The JSON below is derived from domain schemas for the current writable object classes; it is not a tool-call envelope or a grant of permission. HTML-only scope has no writable JSON model contracts. Tool descriptions and parameter schemas remain authoritative for operations, resource locators and payload shape. For example, read_ppt uses resource.kind; slide.html.patch uses edits of old_text/new_text, whereas semantic model patches use the disclosed JSON Patch paths.

Authoritative writable model contracts:
{{CONTRACTS_JSON}}
