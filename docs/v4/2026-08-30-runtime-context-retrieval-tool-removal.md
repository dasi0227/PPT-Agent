# Runtime Context Retrieval Tool Removal

## Decision

`search_refs` is removed from the model-visible Runtime tool surface. Context
selection remains an internal Runtime responsibility and does not emit business
tool lifecycle events.

The model uses:

- `read_ppt` for exact authorized PPT resources;
- `run_command` for permitted project-local inspection;
- Runtime-supplied context for presentation summaries, memory, recent turns and
  authorized references.

No compatibility alias, fallback tool or legacy event migration is provided.

## Runtime contract

Runtime owns context indexing, scope and freshness filtering, retrieval budgets,
and prompt injection. Retrieval is recomputed only when its index, phase or
query-driving state changes. A ContextBriefing contains the selected summaries,
not only reference identifiers and ranking metadata.

The internal retrieval implementation may be replaced without changing the
model-visible tool contract. A future external source must use a source-specific
tool such as `web_search` or `knowledge_search`, rather than restoring the
generic `search_refs` name.

## Removed surface

- `referenceSearchTool` and the `context.search` capability.
- `search_refs` tool disclosure in every mode and phase.
- `search_refs` public event validation, progress copy and timeline projection.
- Runtime Prompt wording that instructs the model to search authorized context.

Historical v3 design documents retain the earlier decision for design history.
This document and the v4 canonical architecture supersede those references.
