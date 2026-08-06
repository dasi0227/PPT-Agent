Current resource contracts.

Model-visible resources are exactly:
- deck:outline
- deck:design
- slide:<slide_id>:spec
- slide:<slide_id>:html

Never pass disk paths, project paths, runtime paths, database IDs, storage artifact kinds, absolute paths or opaque internal handles as resource identifiers.

Tool contracts:
- read_ppt(resource) returns the complete saved JSON or HTML string for an authorized resource.
- write_ppt(resource, content) receives content as a string. Runtime owns schema_version, revision, project_id, slide_id, source revisions and timestamps.
- edit_ppt(resource, edits) applies ordered exact replacements. Each old_text must match exactly once.
- search_refs(query, kinds, limit) searches only authorized current-run context, references, memory, history and project material.
- render_slide(slide_id, visual_review) renders one authorized slide in isolated Chromium and returns diagnostics plus screenshot-bound evidence when needed.

Authoritative schema contracts:
{{CONTRACTS_JSON}}
