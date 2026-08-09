Current resource contracts.

Resource display keys are:
- deck:outline
- deck:design
- slide:<slide_id>:spec
- slide:<slide_id>:html

Display keys are labels for discussion, evidence and summaries. They are not tool arguments.

Tool resource arguments must be objects:
- deck:outline -> {"type":"deck","part":"outline"}
- deck:design -> {"type":"deck","part":"design"}
- slide:<slide_id>:spec -> {"type":"slide","slide_id":"<stable slide_id>","part":"spec"}
- slide:<slide_id>:html -> {"type":"slide","slide_id":"<stable slide_id>","part":"html"}

Never pass resource as a string. Never pass disk paths, project paths, runtime paths, database IDs, storage artifact kinds, absolute paths, display keys, "current" or opaque internal handles as resource arguments.

Tool contracts:
- read_ppt(resource_object) returns the complete saved JSON or HTML string for an authorized resource.
- write_ppt(resource_object, content) receives content as a string. Runtime owns version, revision, project, slide_id, section_id, subsection_id and timestamps.
- edit_ppt(resource_object, edits) applies ordered exact replacements. Each old_text must match exactly once.
- search_refs(query, kinds, limit) searches only authorized current-run context, references, memory, history and project material.
- render_slide(slide_id, visual_review) renders one authorized slide in isolated Chromium and returns diagnostics plus screenshot-bound evidence when needed.

Authoritative schema contracts:
{{CONTRACTS_JSON}}
