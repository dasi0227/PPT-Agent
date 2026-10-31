---
id: tool_read_resource
desc: Read a saved presentation resource.
---

Read the complete saved source of a presentation resource. Use `<project_state>` within `<runtime_context>` to reuse fresh content and avoid reading missing resources. Reread stale content when needed or when a content conflict requires a new baseline. Provide `slide_id` for `spec` and `html` and omit it for other resource types. A successful call returns the original JSON or HTML source as a string in `content`.