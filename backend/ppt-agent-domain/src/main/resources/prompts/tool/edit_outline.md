Create or update the presentation outline. Supply exactly one of `content` (full replacement) or `edits` (targeted replacements). Returns the complete saved JSON source as a string in `content`, including assigned IDs and user revisions made during approval. Use the returned IDs in subsequent calls.

- The root requires ordered `slides` and `chapters` arrays. Use `slides` for a flat deck or `chapters` for a grouped deck. At most one array may contain entries. Every chapter, section and slide has an `id` and a `title`. A chapter may contain direct slides or sections. Sections contain slides. Include unused child arrays as `[]`.
- Use `"new"` for new nodes' IDs; each receives a distinct stable ID. Preserve existing IDs and node types.
- Omitted existing slides are deleted with their saved spec and HTML.
- Apply replacements in order; each `old_text` must match exactly once after earlier replacements.

Example `content` source:

```
{
  "slides": [],
  "chapters": [
    {
      "id": "new",
      "title": "Introduction",
      "slides": [
        { "id": "new", "title": "Overview" }
      ],
      "sections": []
    },
    {
      "id": "new",
      "title": "Analysis",
      "slides": [],
      "sections": [
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
