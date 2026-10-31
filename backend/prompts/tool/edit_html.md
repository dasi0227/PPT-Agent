---
id: tool_edit_html
desc: Create or update a slide's HTML.
---

Create or update the HTML of an existing slide (`{slide_id}.html`). Supply exactly one of `content` to create or fully replace the document, or `edits` for targeted text replacements. Replacements run in order. Each `old_text` must match exactly once, including whitespace, in the HTML after earlier replacements. A successful call returns only `status=success`.