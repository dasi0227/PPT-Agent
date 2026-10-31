Create or update the presentation's sections, subsections and slides. Supply exactly one of `content` for a complete outline or `edits` for targeted replacements.

- The root contains an ordered `sections` array. A section may contain direct `slides` or `subsections`, but not both; use `[]` for unused child arrays.
- Set each new node's `id` to `"new"`. Each occurrence receives a distinct stable ID; use returned IDs in subsequent calls.
- Preserve existing IDs when renaming, moving or reordering nodes. Do not duplicate IDs or change their node types.
- Existing slides omitted from the final outline are deleted along with their saved spec and HTML.
- Replacements run in order. Each `old_text` must match exactly once in the JSON after earlier replacements.

Returns the complete saved outline JSON source as a string in `content`, including assigned IDs and any user revisions during approval. A refusal may include the user's reason in `feedback`.

An outline can contain both kinds of section:

```json
{
  "sections": [
    {
      "id": "new", "title": "Introduction",
      "slides": [{ "id": "new", "title": "Overview" }],
      "subsections": []
    },
    {
      "id": "new", "title": "Analysis", "slides": [],
      "subsections": [{
        "id": "new", "title": "Key findings",
        "slides": [{ "id": "new", "title": "Supporting evidence" }]
      }]
    }
  ]
}
```
