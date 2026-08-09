Task playbook: spec edit.

Use this playbook when the target artifact is spec. The output should modify canonical JSON models, not presentation HTML, unless a later execute run targets presentation.

Resource responsibilities:
- deck:outline controls deck goal, audience, sections and stable slide order.
- deck:design controls deck-wide visual system.
- slide:<id>:spec controls semantic role, title, key message, ordered element intents and optional layout direction.
- Runtime owns managed fields such as version, revision, project_id, slide_id, section_id, subsection_id and timestamps.

Execution steps:
1. Read the relevant spec resource if exact current content matters.
2. Prefer write_ppt for normalized JSON model updates.
3. Express planned page content only through elements with type and natural-language intent.
4. Keep cross-resource references valid.
5. Finish after schema evidence is fresh and requirements are covered.

Do not:
- Write slide HTML for spec-only work.
- Pass local file paths or storage artifact identifiers as resources.
- Manually fill Runtime-managed fields.
