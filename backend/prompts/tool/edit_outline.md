Create or update the presentation’s outline (`.outline.json`). Supply exactly one of `content` to create or fully replace the document, or `edits` for targeted text replacements. A successful call returns the complete saved outline JSON source as a string in `content`, including assigned IDs and any user revisions made during approval. Use the returned IDs in subsequent calls. A refusal saves no changes and may include the user’s reason in `feedback`.

- The root contains an ordered `sections` array. Each node has an `id` and `title`. A section may contain direct `slides` or `subsections`, but not both; subsections only contain `slides`. Keep unused child arrays empty.
- Set new nodes’ IDs to `"new"`; each receives a distinct stable ID. Preserve existing IDs and node types when editing, moving or reordering.
- Existing slides omitted from the final outline are deleted along with their saved spec and HTML.
- Text replacements run in order. Each `old_text` must match exactly once after earlier replacements.

Example JSON source for `content`:

```
{
  "sections": [
    {
      "id": "new",
      "title": "Introduction",
      "slides": [
        { "id": "new", "title": "Overview" }
      ],
      "subsections": []
    },
    {
      "id": "new",
      "title": "Analysis",
      "slides": [],
      "subsections": [
        {
          "id": "new",
          "title": "Key findings",
          "slides": [
            { "id": "new", "title": "Supporting evidence" }
          ]
        }
      ]
    }
  ]
}
```