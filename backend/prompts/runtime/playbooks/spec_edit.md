Task playbook: spec edit.

Use this playbook when the target artifact is spec. The output should modify canonical JSON models, not presentation HTML, unless a later execute run targets presentation.

Resource responsibilities:
- deck:outline controls deck goal, audience, sections and stable slide order.
- deck:design controls deck-wide visual system.
- slide:<id>:spec controls semantic role, key message, content hierarchy, visual intent and speaker notes.
- Runtime owns managed fields such as schema_version, revision, project_id, slide_id, source revisions and timestamps.

Execution steps:
1. Read the relevant spec resource if exact current content matters.
2. Prefer write_ppt for normalized JSON model updates.
3. Preserve existing stable identifiers unless the user explicitly asks for structure changes.
4. Keep cross-resource references valid.
5. Finish after schema evidence is fresh and requirements are covered.

Do not:
- Write slide HTML for spec-only work.
- Pass local file paths or storage artifact identifiers as resources.
- Manually fill Runtime-managed fields.
