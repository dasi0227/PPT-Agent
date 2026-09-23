Resource ownership and dependencies.

Resource ownership:
- Manifest owns presentation intent, audience, language, requirements, prohibitions, 16:9 canvas policy and numbering behavior.
- Outline owns the strict section/subsection tree, stable node references, canonical page title, semantic role and the only slide order.
- Design owns direction and shared chrome. The user-selected theme is read-only to the Agent, including design.theme; this is a field-level product rule, independent of page selection.
- Slide Spec owns one page's key_message, ordered elements (type + natural-language intent), and optional layout direction. It never stores title, role, section, subsection, placement, ordinal or page number.
- Slide HTML is the Agent-authored page body. Runtime owns the frame, equal-ratio fitting, shared chrome and derived numbering. Materialization records and render proof are Runtime-managed, never author-written.

Outline structure has exactly two levels: a section is direct (slides, no subsections) or grouped (empty section slides, pages under subsections). Never mix the two. For new nodes submit client_ref, then use returned IDs; never invent formal sec_*, sub_* or sli_* identities.

Use content hashes returned by read_ppt (content_hash) and mutations (hashes) for expected_hash when concurrency protection matters. On a content conflict, read current state and recompute the intended edit. Do not replay an obsolete patch. Runtime owns schema version, identity and timestamps; never copy those fields into writable payloads.

artifact_hash identifies persisted file bytes for change tracking; it is not an expected_hash token.

Tool schemas define operation envelopes, writable fields, enums, limits and examples. Use only the current disclosed schema; no prose contract grants permission. read_ppt returns structured authoring content with its original content_hash, while HTML remains source text. Use fresh hashes and current content when editing.
