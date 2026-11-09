Create or update the presentation outline. Supply exactly one of `content` (full replacement) or `edits` (targeted replacements). Returns the complete saved JSON source as a string in `content`, including assigned IDs and user revisions made during approval. Use the returned IDs in subsequent calls.

- The root has an ordered `sections` array. Every node has `id` and `title`. A section may contain direct `slides` or `subsections`, never both; subsections contain only `slides`. Keep unused child arrays empty.
- Use `"new"` for new nodes' IDs; each receives a distinct stable ID. Preserve existing IDs and node types.
- Omitted existing slides are deleted with their saved spec and HTML.
- Apply replacements in order; each `old_text` must match exactly once after earlier replacements.

Example `content` source:

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
